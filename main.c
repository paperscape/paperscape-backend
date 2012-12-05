#include <stdio.h>
#include <gtk/gtk.h>

#include "xiwilib.h"
#include "common.h"
#include "map.h"
#include "mysql.h"

GtkWidget *window;

static gboolean on_expose_event(GtkWidget *widget, cairo_t *cr, map_env_t *map_env) {
    map_env_draw(cr, map_env);
    return TRUE;
}

static gboolean map_env_update(map_env_t *map_env) {
    map_env_forces(map_env, 1);
    map_env_forces(map_env, 0);
    map_env_forces(map_env, 0);
    map_env_forces(map_env, 0);
    map_env_forces(map_env, 0);
    map_env_forces(map_env, 0);
    // force a redraw
    gtk_widget_queue_draw(window);
    return TRUE;
}

/****************************************************************/

int main(int argc, char *argv[]) {
    /*
    int num_papers;
    paper_t *papers;
    if (!load_papers_from_mysql("hep-th", &num_papers, &papers)) {
        return 1;
    }
    */
    map_env_t *map_env = map_env_new();
    //map_env_set_papers(map_env, 1000, papers);
    map_env_random_papers(map_env, 1000);

    gtk_init(&argc, &argv);

    /* Create the main, top level window */
    window = gtk_window_new(GTK_WINDOW_TOPLEVEL);

    /* Give it the title */
    gtk_window_set_title(GTK_WINDOW(window), "Map generator");

    /*
    ** Map the destroy signal of the window to gtk_main_quit;
    ** When the window is about to be destroyed, we get a notification and
    ** stop the main GTK+ loop by returning 0
    */
    g_signal_connect(window, "destroy", G_CALLBACK(gtk_main_quit), NULL);

    // create the drawing area
    GtkWidget *darea = gtk_drawing_area_new();
    gtk_container_add(GTK_CONTAINER(window), darea);
    gtk_window_set_position(GTK_WINDOW(window), GTK_WIN_POS_CENTER);
    gtk_window_set_default_size(GTK_WINDOW(window), 390, 240);

    /* Make sure that everything, window and label, are visible */
    gtk_widget_show_all(window);

    g_timeout_add(20 /* milliseconds */, (GSourceFunc)map_env_update, map_env);
    g_signal_connect(darea, "draw", G_CALLBACK(on_expose_event), map_env);

    /*
    ** Start the main loop, and do nothing (block) until
    ** the application is closed
    */
    gtk_main();

    return 0;
}
