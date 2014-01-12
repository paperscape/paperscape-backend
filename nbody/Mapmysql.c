// map functions that rely on mysql

#include <stdio.h>
#include <stdlib.h>

#include "xiwilib.h"
#include "Common.h"
#include "Layout.h"
#include "Force.h"
#include "Quadtree.h"
#include "Mysql.h"
#include "Mapmysql.h"
#include "map.h"
#include "mapprivate.h"

void Mapmysql_env_layout_pos_load_from_db(map_env_t *map_env) {
    // make a single layout
    Layout_t *l = Layout_build_from_papers(map_env->num_papers, map_env->papers, false, 1, 0);
    map_env->layout = l;

    // print info about the layout
    Layout_print(l);

    // initialise random positions, in case we can't/don't load a position for a given paper
    for (int i = 0; i < l->num_nodes; i++) {
        l->nodes[i].x = 100.0 * random() / RAND_MAX;
        l->nodes[i].y = 100.0 * random() / RAND_MAX;
    }

    // load the layout using MySQL
    Mysql_load_paper_positions(l);

    // set do_close_repulsion, since we are loading a layout that was saved this way
    map_env->force_params.do_close_repulsion = true;

    // small step size for the next force iteration
    map_env->step_size = 0.1;
}

void Mapmysql_env_layout_pos_save_to_db(map_env_t *map_env) {
    // get the finest layout, corresponding to one layout_node per paper
    Layout_t *l = map_env->layout;
    while (l->child_layout != NULL) {
        l = l->child_layout;
    }

    // save the layout using MySQL
    Mysql_save_paper_positions(l);
}
