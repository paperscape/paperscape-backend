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

static int boost_step_size = 0;
static bool auto_refine = true;
static int iterate_counter_full_refine = 0;
static int iterate_counter = 0;

static bool map_env_update(map_env_t *map_env) {
    bool converged = false;
    struct timeval tp;
    gettimeofday(&tp, NULL);
    int start_time = tp.tv_sec * 1000 + tp.tv_usec / 1000;
    for (int i = 0; i < 10; i++) {
        iterate_counter += 1;
        /*
        if (iterate_counter == 500) {
            map_env_toggle_do_close_repulsion(map_env);
        }
        */
        printf("nbody iteration %d\n", iterate_counter);
        if (map_env_iterate(map_env, NULL, boost_step_size > 0)) {
            converged = true;
            break;
        }
        if (boost_step_size > 0) {
            boost_step_size -= 1;
        }
    }
    gettimeofday(&tp, NULL);
    int end_time = tp.tv_sec * 1000 + tp.tv_usec / 1000;
    printf("%f seconds per iteration\n", (end_time - start_time) / 10.0 / 1000.0);

    if (auto_refine) {
        if (iterate_counter_full_refine > 0 && iterate_counter > iterate_counter_full_refine) {
            map_env_refine_layout(map_env);
            boost_step_size = 1;
            auto_refine = false;
        } else if (converged) {
            if (map_env_number_of_finer_layouts(map_env) > 1) {
                map_env_refine_layout(map_env);
                boost_step_size = 1;
            } else if (map_env_number_of_finer_layouts(map_env) == 1) {
                map_env_set_do_close_repulsion(map_env, true);
                boost_step_size = 1;
                iterate_counter_full_refine = iterate_counter + 2000;
            }
        }
    }

    return true; // yes, we want to be called again
}

static int usage(const char *progname) {
    printf("\n");
    printf("usage: %s [options]\n", progname);
    printf("\n");
    printf("options:\n");
    printf("    -write-db           write positions back to DB\n");
    printf("\n");
    return 1;
}

int main(int argc, char *argv[]) {

    // parse command line arguments
    bool arg_write_db = false;
    for (int a = 1; a < argc; a++) {
        if (streq(argv[a], "-write-db")) {
            arg_write_db = true;
        } else {
            return usage(argv[0]);
        }
    }

    //const char *where_clause = "(arxiv IS NOT NULL AND status != 'WDN')";
    const char *where_clause = "(arxiv IS NOT NULL AND status != 'WDN' AND id > 2130000000 AND maincat='hep-th')";

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

    // load existing positions from DB
    map_env_layout_load_from_db(map_env);

    // assign positions to new papers
    map_env_layout_XX(map_env);

    // print some help text :)
    printf("running headless, only ctrl-C can kill me now!\n");

    // run the nbody algo until it's done
    while (map_env_update(map_env)) {
        break;
    }

    // write the new positions to the DB
    if (arg_write_db) {
        map_env_layout_save_to_db(map_env);
    }

    return 0;
}
