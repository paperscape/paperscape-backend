#ifndef _INCLUDED_MAPMYSQL_H
#define _INCLUDED_MAPMYSQL_H

#include "map.h"

void map_env_layout_pos_load_from_db(map_env_t *map_env);
void map_env_layout_pos_save_to_db(map_env_t *map_env);

#endif // _INCLUDED_MAPMYSQL_H

