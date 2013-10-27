#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/time.h>

#include "xiwilib.h"
#include "common.h"
#include "layout.h"
#include "map.h"
#include "map2.h"
#include "json.h"

static int usage(const char *progname) {
    printf("\n");
    printf("usage: %s [options]\n", progname);
    printf("\n");
    printf("options:\n");
    printf("    --write-json        write positions to json file (default is not to)\n");
    printf("\n");
    return 1;
}

int main(int argc, char *argv[]) {

    // parse command line arguments
    bool arg_write_json = false;
    for (int a = 1; a < argc; a++) {
        if (streq(argv[a], "--start-afresh")) {
            arg_write_json = true;
        } else if (streq(argv[a], "--write-json")) {
            arg_write_json = true;
        } else {
            return usage(argv[0]);
        }
    }

    // load the papers from the DB
    int num_papers;
    paper_t *papers;
    keyword_set_t *keyword_set;
    if (!json_load_papers("sample.json", &num_papers, &papers, &keyword_set)) {
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

    // do the layout
    map_env_do_complete_layout(map_env);

    // align the map in a fixed direction
    map_env_orient(map_env, CAT_hep_ph, 4.2);

    // write map to JSON
    if (arg_write_json) {
        vstr_t *vstr = vstr_new();
        vstr_reset(vstr);
        vstr_printf(vstr, "map-%06u.json", map_env_get_num_papers(map_env));
        map_env_layout_save_to_json(map_env, vstr_str(vstr));
    }

    return 0;
}
