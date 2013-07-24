#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <math.h>
#include <sys/time.h>

#include "xiwilib.h"
#include "common.h"
#include "layout.h"
#include "map.h"
#include "mysql.h"

static bool map_env_update(map_env_t *map_env, int num_iterations, bool boost_step_size) {
    struct timeval tp;
    gettimeofday(&tp, NULL);
    int start_time = tp.tv_sec * 1000 + tp.tv_usec / 1000;
    bool converged = false;
    for (int i = 0; i < num_iterations; i++) {
        converged = map_env_iterate(map_env, NULL, boost_step_size);
        boost_step_size = false;
    }
    gettimeofday(&tp, NULL);
    int end_time = tp.tv_sec * 1000 + tp.tv_usec / 1000;
    printf("did %d iterations, %.2f seconds per iteration\n", num_iterations, (end_time - start_time) / 1000.0 / num_iterations);
    return converged;
}

static int usage(const char *progname) {
    printf("\n");
    printf("usage: %s [options]\n", progname);
    printf("\n");
    printf("options:\n");
    printf("    --start-afresh      start the graph layout afresh (default is to process only new papers); enabling this enables --write-json\n");
    printf("    --whole-arxiv       process all papers from the arxiv (default is to process only a small, test subset)\n");
    printf("    --write-db          write positions to DB (default is not to)\n");
    printf("    --write-json        write positions to json file (default is not to)\n");
    printf("\n");
    return 1;
}

int main(int argc, char *argv[]) {

    // parse command line arguments
    bool arg_start_afresh = false;
    const char *where_clause = "(arxiv IS NOT NULL AND status != 'WDN' AND id > 2130000000 AND maincat='hep-th')";
    bool arg_write_db = false;
    bool arg_write_json = false;
    for (int a = 1; a < argc; a++) {
        if (streq(argv[a], "--start-afresh")) {
            arg_start_afresh = true;
            arg_write_json = true;
        } else if (streq(argv[a], "--whole-arxiv")) {
            where_clause = "(arxiv IS NOT NULL AND status != 'WDN')";
        } else if (streq(argv[a], "--write-db")) {
            arg_write_db = true;
        } else if (streq(argv[a], "--write-json")) {
            arg_write_json = true;
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
    if (!mysql_load_papers(where_clause, false, &num_papers, &papers, &keyword_set)) {
        return 1;
    }

    // create the map object
    map_env_t *map_env = map_env_new();

    // set the papers
    map_env_set_papers(map_env, num_papers, papers, keyword_set);

    // select the date range
    {
        int id_min;
        int id_max;
        map_env_get_max_id_range(map_env, &id_min, &id_max);
        map_env_select_date_range(map_env, id_min, id_max);
    }

    if (arg_start_afresh) {
        // create a new layout with 10 levels of coarsening
        map_env_layout_new(map_env, 10);
        map_env_set_do_close_repulsion(map_env, false);

        printf("iterating from the start to build entire graph\n");

        bool boost_step_size = false;
        bool refining_stage = true;
        int iterate_counter = 0;
        int iterate_counter_wait_until = 0;
        while (true) {
            bool converged = map_env_update(map_env, 50, boost_step_size);
            boost_step_size = false;
            iterate_counter += 50;
            if (refining_stage) {
                if (iterate_counter_wait_until > 0 && iterate_counter > iterate_counter_wait_until) {
                    printf("refining to finest layout\n");
                    map_env_refine_layout(map_env);
                    boost_step_size = true;
                    refining_stage = false;
                    iterate_counter_wait_until = iterate_counter + 6000;
                } else if (converged) {
                    int num_finer = map_env_number_of_finer_layouts(map_env);
                    if (num_finer > 1) {
                        printf("refining layout; %d to go\n", num_finer - 1);
                        map_env_refine_layout(map_env);
                        boost_step_size = true;
                    } else if (num_finer == 1) {
                        printf("doing close repulsion\n");
                        map_env_set_do_close_repulsion(map_env, true);
                        boost_step_size = true;
                        iterate_counter_wait_until = iterate_counter + 2000;
                    }
                }
            } else {
                // final stage at full refinement
                if (iterate_counter > iterate_counter_wait_until) {
                    // finished!
                    printf("finished layout\n");
                    break;
                }
            }
        }

    } else {
        // load existing positions from DB
        map_env_layout_load_from_db(map_env);

        // rotate the entire map by a random amount, to reduce quad-tree-force artifacts
        struct timeval tp;
        gettimeofday(&tp, NULL);
        srandom(tp.tv_sec * 1000000 + tp.tv_usec);
        double angle = 6.28 * (double)random() / (double)RAND_MAX;
        map_env_rotate_all(map_env, angle);
        printf("rotated graph by %.2f rad to eliminate quad-tree-force artifacts\n", angle);

        // assign positions to new papers
        int n_new = map_env_layout_place_new_papers(map_env);
        if (n_new > 0) {
            printf("iterating to place new papers\n");
            map_env_set_do_close_repulsion(map_env, false);
            map_env_update(map_env, 250, false);
        }
        map_env_layout_finish_placing_new_papers(map_env);

        // iterate to adjust whole graph
        printf("iterating to adjust entire graph\n");
        map_env_set_do_close_repulsion(map_env, true);
        map_env_update(map_env, 100, false);
    }

    // align the map in a fixed direction
    map_env_orient(map_env, CAT_hep_ph, 4.2);

    // write the new positions to the DB
    if (arg_write_db) {
        map_env_layout_save_to_db(map_env);
    }

    // write map to JSON
    if (arg_write_json) {
        vstr_t *vstr = vstr_new();
        vstr_reset(vstr);
        vstr_printf(vstr, "map-%06u.json", map_env_get_num_papers(map_env));
        map_env_layout_save_to_json(map_env, vstr_str(vstr));
    }

    return 0;
}
