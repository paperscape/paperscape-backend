#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/time.h>

#include "xiwilib.h"
#include "common.h"
#include "layout.h"
#include "Map.h"
#include "Mapmysql.h"
#include "Mapauto.h"
#include "Mysql.h"

static int usage(const char *progname) {
    printf("\n");
    printf("usage: %s [options]\n", progname);
    printf("\n");
    printf("options:\n");
    printf("    --start-afresh       start the graph layout afresh (default is to process only new papers); enabling this enables --write-json\n");
    printf("    --layout-json <file> load layout from given JSON file\n");
    printf("    --whole-arxiv        process all papers from the arxiv (default is to process only a small, test subset)\n");
    printf("    --write-db           write positions to DB (default is not to)\n");
    printf("    --write-json         write positions to json file (default is not to)\n");
    printf("    --years-ago <num>    perform timelapse (writes positions to json)\n");
    printf("    --link <num>         link strength\n");
    printf("\n");
    return 1;
}

int main(int argc, char *argv[]) {

    // parse command line arguments
    bool arg_start_afresh = false;
    const char *where_clause = "(arxiv IS NOT NULL AND status != 'WDN' AND id > 2130000000 AND maincat='hep-th')";
    bool arg_write_db = false;
    bool arg_write_json = false;
    double arg_link_strength = -1;
    int arg_yearsago = -1; // for timelapse (-1 means no timelapse)
    const char *arg_layout_json = NULL;
    for (int a = 1; a < argc; a++) {
        if (streq(argv[a], "--start-afresh")) {
            arg_start_afresh = true;
            arg_write_json = true;
        } else if (streq(argv[a], "--layout-json")) {
            a += 1;
            if (a >= argc) {
                return usage(argv[0]);
            }
            arg_layout_json = argv[a];
        } else if (streq(argv[a], "--whole-arxiv")) {
            where_clause = "(arxiv IS NOT NULL AND status != 'WDN')";
        } else if (streq(argv[a], "--write-db")) {
            arg_write_db = true;
        } else if (streq(argv[a], "--write-json")) {
            arg_write_json = true;
        } else if (streq(argv[a], "--years-ago")) {
            a += 1;
            if (a >= argc) {
                return usage(argv[0]);
            }
            arg_yearsago = atoi(argv[a]);
            arg_write_json = true;
        } else if (streq(argv[a], "--link")) {
            a += 1;
            if (a >= argc) {
                return usage(argv[0]);
            }
            arg_link_strength = strtod(argv[a], NULL);
        } else {
            return usage(argv[0]);
        }
    }

    // print info about the where clause being used
    printf("using where clause: %s\n", where_clause);

    // load the papers from the DB
    int num_papers;
    paper_t *papers;
    keyword_set_t *keyword_set;
    if (!Mysql_load_papers(where_clause, false, &num_papers, &papers, &keyword_set)) {
        return 1;
    }

    // create the map object
    Map_env_t *map_env = Map_env_new();

    // set parameters
    if (arg_link_strength > 0) {
        Map_env_set_link_strength(map_env, arg_link_strength);
    } else if (arg_yearsago >= 0) {
        double link_strength = 1.17 + 0.014*((double)arg_yearsago);
        Map_env_set_link_strength(map_env, link_strength);
    }
    printf("using a link strength of: %.3f\n",Map_env_get_link_strength(map_env));

    // set the papers
    Map_env_set_papers(map_env, num_papers, papers, keyword_set);

    // select the date range
    {
        unsigned int id_min;
        unsigned int id_max;
        Map_env_get_max_id_range(map_env, &id_min, &id_max);

        if (arg_yearsago > 0) {
            id_max = (unsigned int)2150000000 - arg_yearsago * 10000000;
        }

        Map_env_select_date_range(map_env, id_min, id_max);
    }

    if (arg_start_afresh) {
        // create a new layout with 10 levels of coarsening, using only ref_freq as weight
        Map_env_layout_new(map_env, 10, 1, 0);

        // do the layout
        Mapauto_env_do_complete_layout(map_env, 2000, 6000);

    } else {

        if (arg_layout_json == NULL) {
            // load existing positions from DB
            Mapmysql_env_layout_pos_load_from_db(map_env);
        } else {
            // load existing positions from json file
            Map_env_layout_pos_load_from_json(map_env, arg_layout_json);
        }

        // rotate the entire map by a random amount, to reduce quad-tree-force artifacts
        struct timeval tp;
        gettimeofday(&tp, NULL);
        srandom(tp.tv_sec * 1000000 + tp.tv_usec);
        double angle = 6.28 * (double)random() / (double)RAND_MAX;
        Map_env_rotate_all(map_env, angle);
        printf("rotated graph by %.2f rad to eliminate quad-tree-force artifacts\n", angle);

        if (arg_yearsago < 0) {
            // assign positions to new papers
            int n_new = Map_env_layout_place_new_papers(map_env);
            if (n_new > 0) {
                printf("iterating to place new papers\n");
                Map_env_set_do_close_repulsion(map_env, false);
                Mapauto_env_do_iterations(map_env, 250, false, false);
            }
            Map_env_layout_finish_placing_new_papers(map_env);

            // iterate to adjust whole graph
            printf("iterating to adjust entire graph\n");
            Map_env_set_do_close_repulsion(map_env, true);
            Mapauto_env_do_iterations(map_env, 80, false, false);
        
            // iterate for final, very fine steps
            printf("iterating final, very fine steps\n");
            Map_env_set_do_close_repulsion(map_env, true);
            Mapauto_env_do_iterations(map_env, 30, false, true);
        
        } else {
            Map_env_set_step_size(map_env,0.5);
            Mapauto_env_do_complete_layout(map_env, 4000, 10000);
        }

    }

    // align the map in a fixed direction
    Map_env_orient_using_category(map_env, CAT_hep_ph, 4.2);

    // write the new positions to the DB (never do this for timelapse)
    if (arg_write_db && arg_yearsago < 0) {
        Mapmysql_env_layout_pos_save_to_db(map_env);
    }

    // write map to JSON (always do this for timelapse)
    if (arg_write_json) {
        vstr_t *vstr = vstr_new();
        vstr_reset(vstr);
        if (arg_yearsago < 0) {
            vstr_printf(vstr, "map-%06u.json", Map_env_get_num_papers(map_env));
        } else {
            vstr_printf(vstr, "map-%d_L%04d.json", 2014-arg_yearsago,(int)(1000.*Map_env_get_link_strength(map_env)));
        }
        Map_env_layout_pos_save_to_json(map_env, vstr_str(vstr));
    }

    return 0;
}
