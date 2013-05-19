#include <stdio.h>
#include <math.h>
#include <gtk/gtk.h>

#include "xiwilib.h"
#include "common.h"
#include "map.h"
#include "mysql.h"
#include "gui.h"

int main(int argc, char *argv[]) {
    // create the map object
    map_env_t *map_env = map_env_new();
    int num_papers;
    paper_t *papers;

    //const char *where_clause = NULL;
    const char *where_clause = "(maincat='hep-th' OR maincat='hep-ph') AND id >= 2120000000";
    //const char *where_clause = "(maincat='hep-th' OR maincat='hep-ph' OR maincat='gr-qc' OR maincat='hep-ex' OR arxiv IS NULL)";
    //const char *where_clause = "(maincat='hep-th' OR maincat='hep-ph' OR maincat='gr-qc') AND id >= 2115000000";
    //const char *where_clause = "(maincat='hep-th' OR maincat='hep-ph' OR maincat='hep-ex' OR maincat='hep-lat' OR maincat='gr-qc') AND id >= 2110000000";
    //const char *where_clause = "(maincat='hep-th' OR maincat='hep-ph' OR maincat='hep-ex' OR maincat='hep-lat' OR maincat='gr-qc' OR maincat='astro-ph') AND id >= 2120000000";
    //const char *where_clause = "(maincat='hep-lat') AND id >= 1910000000";
    //const char *where_clause = "(maincat='cond-mat' OR maincat='quant-ph') AND id >= 2110000000";
    //const char *where_clause = "(maincat='hep-th' OR maincat='hep-ph' OR maincat='gr-qc' OR maincat='hep-ex' OR maincat='astro-ph' OR maincat='math-ph') AND id >= 2110000000";
    //const char *where_clause = "(maincat='astro-ph' OR maincat='cond-mat' OR maincat='gr-qc' OR maincat='hep-ex' OR maincat='hep-lat' OR maincat='hep-ph' OR maincat='hep-th' OR maincat='math-ph' OR maincat='nlin' OR maincat='nucl-ex' OR maincat='nucl-th' OR maincat='physics' OR maincat='quant-ph') AND id >= 1900000000";
    //const char *where_clause = "(maincat='cs') AND id >= 2122000000";

    if (!mysql_load_papers(where_clause, &num_papers, &papers)) {
        return 1;
    }
    map_env_set_papers(map_env, num_papers, papers);
    //map_env_random_papers(map_env, 1000);
    //map_env_papers_test2(map_env, 100);

    // init gtk
    gtk_init(&argc, &argv);

    // build the gui elements
    build_gui(map_env, where_clause);

    // start the main loop and block until the application is closed
    gtk_main();

    //mysql_save_paper_positions(num_papers, papers);

    return 0;
}
