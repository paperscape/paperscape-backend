#ifndef _INCLUDED_MAP2_H
#define _INCLUDED_MAP2_H

#include "map.h"

// high-level map layout functions

bool Mapauto_env_do_iterations(map_env_t *map_env, int num_iterations, bool boost_step_size, bool very_fine_steps);

void Mapauto_env_do_complete_layout(map_env_t *map_env, int num_iterations_close_repulsion, int num_iterations_finest_layout);

#endif // _INCLUDED_MAP2_H
