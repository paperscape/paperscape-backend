// high-level map layout functions

#include <stdio.h>
#include <sys/time.h>

#include "xiwilib.h"
#include "Common.h"
#include "layout.h"
#include "Map.h"
#include "Mapauto.h"

bool Mapauto_env_do_iterations(Map_env_t *map_env, int num_iterations, bool boost_step_size, bool very_fine_steps) {
    struct timeval tp;
    gettimeofday(&tp, NULL);
    int start_time = tp.tv_sec * 1000 + tp.tv_usec / 1000;
    bool converged = false;
    for (int i = 0; i < num_iterations; i++) {
        converged = Map_env_iterate(map_env, NULL, boost_step_size, very_fine_steps);
        boost_step_size = false;
    }
    gettimeofday(&tp, NULL);
    int end_time = tp.tv_sec * 1000 + tp.tv_usec / 1000;
    printf("did %d iterations, %.2f seconds per iteration, %.2f step size\n", num_iterations, (end_time - start_time) / 1000.0 / num_iterations,Map_env_get_step_size(map_env));
    return converged;
}

void Mapauto_env_do_complete_layout(Map_env_t *map_env, int num_iterations_close_repulsion, int num_iterations_finest_layout) {

    printf("iterating from the start to build entire graph\n");

    Map_env_set_do_close_repulsion(map_env, false);

    bool boost_step_size = false;
    bool refining_stage = true;
    bool started_close_repulsion = false;
    int iterate_counter = 0;
    int iterate_counter_wait_until = 0;
    while (true) {
        bool converged = Mapauto_env_do_iterations(map_env, 50, boost_step_size, false);
        boost_step_size = false;
        iterate_counter += 50;
        if (refining_stage) {
            if (iterate_counter_wait_until > 0 && iterate_counter > iterate_counter_wait_until) {
                printf("refining to finest layout\n");
                Map_env_refine_layout(map_env);
                boost_step_size = true;
                refining_stage = false;
                iterate_counter_wait_until = iterate_counter + num_iterations_finest_layout;
            } else if (converged) {
                int num_finer = Map_env_number_of_finer_layouts(map_env);
                if (num_finer > 1) {
                    printf("refining layout; %d to go\n", num_finer - 1);
                    Map_env_refine_layout(map_env);
                    boost_step_size = true;
                //} else if (num_finer == 1) {
                } else if (num_finer <= 1 && !started_close_repulsion) {
                    printf("doing close repulsion\n");
                    started_close_repulsion = true;
                    Map_env_set_do_close_repulsion(map_env, true);
                    boost_step_size = true;
                    iterate_counter_wait_until = iterate_counter + num_iterations_close_repulsion;
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
}
