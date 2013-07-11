#include <stdio.h>
#include <math.h>
#include <sys/time.h>

#include "xiwilib.h"
#include "common.h"
#include "map.h"
#include "mysql.h"
#include "init.h"

const char *included_papers_string = NULL;
bool update_running = true;
int boost_step_size = 0;
bool mouse_held = false;
bool mouse_dragged;
bool auto_refine = true;
static int iterate_counter_full_refine = 0;
bool lock_view_all = true;
double mouse_last_x = 0, mouse_last_y = 0;
paper_t *mouse_paper = NULL;

/* obsolete
int id_range_start = 2050000000;
int id_range_end = 2060000000;
*/

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
        if (map_env_iterate(map_env, mouse_paper, boost_step_size > 0)) {
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

void run_headless(map_env_t *map_env, const char *papers_string) {
    included_papers_string = papers_string;

    // print some help text :)
    printf("running headless, only ctrl-C can kill me now!\n");

    // run the nbody algo until it's done
    while (map_env_update(map_env)) {
    }
}

int main(int argc, char *argv[]) {
    map_env_t *map_env;
    const char *where_clause;

    int ret = init(argc, argv, 0, &map_env, &where_clause);
    if (ret) {
        return ret;
    }

    run_headless(map_env, where_clause);

    //mysql_save_paper_positions(num_papers, papers);

    return 0;
}
