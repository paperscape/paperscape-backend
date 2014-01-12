#ifndef _INCLUDED_MAPMYSQL_H
#define _INCLUDED_MAPMYSQL_H

#include "Map.h"

void Mapmysql_env_layout_pos_load_from_db(Map_env_t *map_env);
void Mapmysql_env_layout_pos_save_to_db(Map_env_t *map_env);

#endif // _INCLUDED_MAPMYSQL_H

