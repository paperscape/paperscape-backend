#include <stdlib.h>
#include <stdio.h>
#include <math.h>
#include <string.h>
#include <gtk/gtk.h>

#include "xiwilib.h"
#include "common.h"
#include "map.h"
#include "mysql.h"
#include "gui.h"

int usage(const char *progname) {
    printf("\n");
    printf("usage: %s [options]\n", progname);
    printf("\n");
    printf("options:\n");
    printf("    -rsq <num>      r-star squared distance\n");
    printf("    -link <num>     link strength\n");
    printf("\n");
    return 1;
}

int main(int argc, char *argv[]) {
    // parse command line arguments
    double arg_anti_grav_rsq = -1;
    double arg_link_strength = -1;
    for (int a = 1; a < argc; a++) {
        if (streq(argv[a], "-rsq")) {
            a += 1;
            if (a >= argc) {
                return usage(argv[0]);
            }
            arg_anti_grav_rsq = strtod(argv[a], NULL);
        } else if (streq(argv[a], "-link")) {
            a += 1;
            if (a >= argc) {
                return usage(argv[0]);
            }
            arg_link_strength = strtod(argv[a], NULL);
        } else {
            return usage(argv[0]);
        }
    }

    //const char *where_clause = NULL;
    const char *where_clause = "(maincat='hep-th' OR maincat='hep-ph') AND id >= 2100000000";
    //const char *where_clause = "(maincat='hep-th' OR maincat='hep-ph' OR maincat='gr-qc' OR maincat='hep-ex' OR arxiv IS NULL)";
    //const char *where_clause = "(maincat='hep-th' OR maincat='hep-ph' OR maincat='gr-qc') AND id >= 2115000000";
    //const char *where_clause = "(maincat='hep-th' OR maincat='hep-ph' OR maincat='hep-ex' OR maincat='hep-lat' OR maincat='gr-qc') AND id >= 2110000000";
    //const char *where_clause = "(maincat='hep-th' OR maincat='hep-ph' OR maincat='hep-ex' OR maincat='hep-lat' OR maincat='gr-qc' OR maincat='astro-ph') AND id >= 2120000000";
    //const char *where_clause = "(maincat='hep-lat') AND id >= 1910000000";
    //const char *where_clause = "(maincat='cond-mat' OR maincat='quant-ph') AND id >= 2110000000";
    //const char *where_clause = "(maincat='hep-th' OR maincat='hep-ph' OR maincat='gr-qc' OR maincat='hep-ex' OR maincat='astro-ph' OR maincat='math-ph') AND id >= 2110000000";
    //const char *where_clause = "(maincat='astro-ph' OR maincat='cond-mat' OR maincat='gr-qc' OR maincat='hep-ex' OR maincat='hep-lat' OR maincat='hep-ph' OR maincat='hep-th' OR maincat='math-ph' OR maincat='nlin' OR maincat='nucl-ex' OR maincat='nucl-th' OR maincat='physics' OR maincat='quant-ph') AND id >= 1900000000";
    //const char *where_clause = "(maincat='cs') AND id >= 2090000000";
    //const char *where_clause = "(maincat='math') AND id >= 1900000000";
    //const char *where_clause = "(arxiv IS NOT NULL)";

    // load the papers from the DB
    int num_papers;
    paper_t *papers;
    keyword_set_t *keyword_set;
    if (!mysql_load_papers(where_clause, &num_papers, &papers, &keyword_set)) {
        return 1;
    }

    // create the map object
    map_env_t *map_env = map_env_new();

    // set parameters
    if (arg_anti_grav_rsq > 0) {
        map_env_set_anti_gravity(map_env, arg_anti_grav_rsq);
    }
    if (arg_link_strength > 0) {
        map_env_set_link_strength(map_env, arg_link_strength);
    }

    // set the papers
    map_env_set_papers(map_env, num_papers, papers, keyword_set);
    //map_env_random_papers(map_env, 1000);
    //map_env_papers_test2(map_env, 100);

    // select the date range
    {
        int id_min;
        int id_max;
        map_env_get_max_id_range(map_env, &id_min, &id_max);
        int id_range_start = id_min;
        int id_range_end = id_min + 20000000; // plus 2 years

        // for starting part way through
        id_range_start = date_to_unique_id(2012, 3, 0);
        id_range_end = id_range_start + 20000000; // plus 2 years
        id_range_end = id_range_start +  3000000; // plus 0.5 year
        id_range_start = id_min; id_range_end = id_max; // full range

        map_env_select_date_range(map_env, id_range_start, id_range_end, false);
    }

    // init gtk
    gtk_init(&argc, &argv);

    // build the gui elements
    build_gui(map_env, where_clause);

    // start the main loop and block until the application is closed
    gtk_main();

    //mysql_save_paper_positions(num_papers, papers);

    return 0;
}
