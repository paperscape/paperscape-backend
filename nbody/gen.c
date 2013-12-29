#include <stdio.h>
#include <math.h>

#include "xiwilib.h"
#include "Common.h"
#include "map.h"
#include "gen.h"
#include "tile.h"

static vstr_t *vstr;
static map_env_t *map_env_global;
static const char *included_papers_string = NULL;
static bool boost_step_size = false;
static int id_range_start = 2050000000;
static int id_range_end = 2060000000;

static int add_counter = -200;
static void map_env_update(map_env_t *map_env) {
    for (int i = 0; i < 2; i++) {
        if (map_env_iterate(map_env, NULL, boost_step_size)) {
            break;
        }
        boost_step_size = false;
    }

    if (add_counter++ > 50) {
        add_counter = 0;

        vstr_reset(vstr);
        vstr_t *vstr_info = vstr_new();
        int y, m, d;

        Common_unique_id_to_date(id_range_start, &y, &m, &d);
        vstr_printf(vstr, "map-%04u-%02u-%02u.png", y, m, d);
        vstr_printf(vstr_info, "date: %02u-%02u-%04u to ", d, m, y);
        Common_unique_id_to_date(id_range_end, &y, &m, &d);
        vstr_printf(vstr_info, "%02u-%02u-%04u\n%d papers", d, m, y, map_env_get_num_papers(map_env));
        write_tiles(map_env, 1000, 1000, vstr_str(vstr), vstr_info);
        vstr_free(vstr_info);

        while (true) {
            id_range_start += 200000;
            id_range_end += 200000;
            Common_unique_id_to_date(id_range_start, &y, &m, &d);
            if (m <= 12) {
                break;
            }
        }
        map_env_select_date_range(map_env, id_range_start, id_range_end);
        boost_step_size = true;
    }
}

void main_loop() {
    int id_min;
    int id_max;
    map_env_get_max_id_range(map_env_global, &id_min, &id_max);
    while (id_range_end < id_max) {
        map_env_update(map_env_global);
    }
}

void build_gen(map_env_t *map_env, const char *papers_string) {
    vstr = vstr_new();
    map_env_global = map_env;
    included_papers_string = papers_string;

    int id_min;
    int id_max;
    map_env_get_max_id_range(map_env, &id_min, &id_max);
    id_range_start = id_min;
    id_range_end = id_min + 20000000; // plus 2 years
    map_env_select_date_range(map_env, id_range_start, id_range_end);

    // for now
    id_range_start = 1952278129;
    id_range_end = id_range_start + 20000000; // plus 2 years
    map_env_select_date_range(map_env, id_range_start, id_range_end);
}
