#include <stdio.h>
#include <gtk/gtk.h>

#include "xiwilib.h"
#include "common.h"
#include "map.h"
#include "mysql.h"

GtkWidget *window;

static gboolean draw_callback(GtkWidget *widget, cairo_t *cr, map_env_t *map_env) {
    guint width = gtk_widget_get_allocated_width(widget);
    guint height = gtk_widget_get_allocated_height(widget);
    map_env_draw(map_env, cr, width, height);
    return FALSE;
}

static gboolean button_press_event_callback(GtkWidget *widget, GdkEventButton *event, map_env_t *map_env) {
    if (event->button == GDK_BUTTON_PRIMARY) {
    } else if (event->button == GDK_BUTTON_SECONDARY) {
        gtk_main_quit();
    }

    return TRUE; // we handled the event, stop processing
}

static gboolean map_env_update(map_env_t *map_env) {
    map_env_forces(map_env, 1);
    //map_env_grow(map_env, 1.001);
    for (int i = 0; i < 10; i++) {
        if (map_env_forces(map_env, 0)) {
            break;
        }
    }
    // force a redraw
    gtk_widget_queue_draw(window);
    return TRUE; // yes, we want to be called again
}

/****************************************************************/

// for a gtk example, see: http://git.gnome.org/browse/gtk+/tree/demos/gtk-demo/drawingarea.c

int main(int argc, char *argv[]) {
    map_env_t *map_env = map_env_new();

    /*
    int num_papers;
    paper_t *papers;
    if (!load_papers_from_mysql("hep-th", &num_papers, &papers)) {
        return 1;
    }
    map_env_set_papers(map_env, 1000, papers);
    map_env_random_papers(map_env, 1000);
    */
    map_env_papers_test2(map_env, 100);

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
    GtkWidget *drawing_area = gtk_drawing_area_new();
    gtk_container_add(GTK_CONTAINER(window), drawing_area);
    gtk_window_set_position(GTK_WINDOW(window), GTK_WIN_POS_CENTER);
    gtk_window_set_default_size(GTK_WINDOW(window), 300, 300);

    g_timeout_add(50 /* milliseconds */, (GSourceFunc)map_env_update, map_env);
    g_signal_connect(drawing_area, "draw", G_CALLBACK(draw_callback), map_env);
    g_signal_connect(drawing_area, "button-press-event", G_CALLBACK(button_press_event_callback), map_env);

    /* Ask to receive events the drawing area doesn't normally
    * subscribe to
    */
    gtk_widget_set_events(drawing_area, gtk_widget_get_events(drawing_area)
        | GDK_LEAVE_NOTIFY_MASK
        | GDK_BUTTON_PRESS_MASK
        | GDK_POINTER_MOTION_MASK
        | GDK_POINTER_MOTION_HINT_MASK);

    /* Make sure that everything, window and label, are visible */
    gtk_widget_show_all(window);

    /*
    ** Start the main loop, and do nothing (block) until
    ** the application is closed
    */
    gtk_main();

    return 0;
}
