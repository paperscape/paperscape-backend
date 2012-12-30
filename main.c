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
    if (!load_papers_from_mysql("hep-th", &num_papers, &papers)) {
        return 1;
    }
    map_env_set_papers(map_env, num_papers, papers);
    //map_env_random_papers(map_env, 1000);
    //map_env_papers_test2(map_env, 100);

    // init gtk
    gtk_init(&argc, &argv);

    // build the gui elements
    build_gui(map_env);

    // start the main loop and block until the application is closed
    gtk_main();

    return 0;
}
