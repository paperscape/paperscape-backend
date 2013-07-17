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

static void map_env_update(map_env_t *map_env, int num_iterations) {
    struct timeval tp;
    gettimeofday(&tp, NULL);
    int start_time = tp.tv_sec * 1000 + tp.tv_usec / 1000;
    for (int i = 0; i < num_iterations; i++) {
        map_env_iterate(map_env, NULL, false);
    }
    gettimeofday(&tp, NULL);
    int end_time = tp.tv_sec * 1000 + tp.tv_usec / 1000;
    printf("did %d iterations, %.2f seconds per iteration\n", num_iterations, (end_time - start_time) / 1000.0 / num_iterations);
}

static int usage(const char *progname) {
    printf("\n");
    printf("usage: %s [options]\n", progname);
    printf("\n");
    printf("options:\n");
    printf("    --whole-arxiv       process all papers from the arxiv\n");
    printf("    --write-db          write positions back to DB\n");
    printf("\n");
    return 1;
}

int main(int argc, char *argv[]) {

    // parse command line arguments
    const char *where_clause = "(arxiv IS NOT NULL AND status != 'WDN' AND id > 2130000000 AND maincat='hep-th')";
    bool arg_write_db = false;
    for (int a = 1; a < argc; a++) {
        if (streq(argv[a], "--whole-arxiv")) {
            where_clause = "(arxiv IS NOT NULL AND status != 'WDN')";
        } else if (streq(argv[a], "--write-db")) {
            arg_write_db = true;
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

    // load existing positions from DB
    map_env_layout_load_from_db(map_env);

    // assign positions to new papers
    int n_new = map_env_layout_place_new_papers(map_env);
    if (n_new > 0) {
        printf("iterating to place new papers\n");
        map_env_set_do_close_repulsion(map_env, false);
        map_env_update(map_env, 30);
    }
    map_env_layout_finish_placing_new_papers(map_env);

    // iterate to adjust whole graph
    printf("iterating to adjust entire graph\n");
    map_env_set_do_close_repulsion(map_env, true);
    map_env_update(map_env, 100);

    // write the new positions to the DB
    if (arg_write_db) {
        map_env_layout_save_to_db(map_env);
    }

    return 0;
}
