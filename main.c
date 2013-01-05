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
    //const char *where_clause = "(maincat='hep-th' OR maincat='hep-ph' OR maincat='gr-qc' OR maincat='hep-ex' OR arxiv IS NULL)";
    //const char *where_clause = "(maincat='hep-th' OR maincat='hep-ph' OR maincat='gr-qc' OR maincat='hep-ex') AND id >= 1992500000 AND id < 2000000000";
    const char *where_clause = "(maincat='hep-th' OR maincat='hep-ph' OR maincat='gr-qc' OR maincat='hep-ex') AND id >= 2090000000";

    if (!load_papers_from_mysql(where_clause, &num_papers, &papers)) {
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

    return 0;
}
