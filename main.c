#include <stdio.h>
#include <math.h>
#include <gtk/gtk.h>

#include "xiwilib.h"
#include "common.h"
#include "map.h"
#include "mysql.h"

GtkWidget *window;
int do_tred = 1;

bool mouse_held = FALSE;
bool mouse_dragged;
double mouse_last_x = 0, mouse_last_y = 0;
paper_t *mouse_paper = NULL;

static gboolean draw_callback(GtkWidget *widget, cairo_t *cr, map_env_t *map_env) {
    guint width = gtk_widget_get_allocated_width(widget);
    guint height = gtk_widget_get_allocated_height(widget);
    map_env_draw(map_env, cr, width, height, do_tred);
    if (mouse_paper != NULL) {
        cairo_set_line_width(cr, 1);
        cairo_set_source_rgba(cr, 0, 0, 0, 0.8);
        double x = 0;
        double y = 0.02 * mouse_paper->index;
        map_env_world_to_screen(map_env, &x, &y);
        cairo_move_to(cr, 0, y);
        cairo_line_to(cr, width, y);
        cairo_stroke(cr);
    }
    return FALSE;
}

static gboolean key_press_event_callback(GtkWidget *widget, GdkEventKey *event, map_env_t *map_env) {
    //printf("here %d %d\n", event->state, event->keyval);
    if (event->keyval == GDK_KEY_a) {
        map_env_inc_num_papers(map_env, 1);
    } else if (event->keyval == GDK_KEY_b) {
        map_env_inc_num_papers(map_env, 10);
    } else if (event->keyval == GDK_KEY_c) {
        map_env_inc_num_papers(map_env, 100);
    } else if (event->keyval == GDK_KEY_j) {
        map_env_jolt(map_env, 0.5);
    } else if (event->keyval == GDK_KEY_k) {
        map_env_jolt(map_env, 2.5);
    } else if (event->keyval == GDK_KEY_t) {
        do_tred = 1 - do_tred;
        if (do_tred) {
            printf("transitive reduction turned on\n");
        } else {
            printf("transitive reduction turned off\n");
        }
    } else if (event->keyval == GDK_KEY_q) {
        gtk_main_quit();
    }

    return TRUE; // we handled the event, stop processing
}

static gboolean button_press_event_callback(GtkWidget *widget, GdkEventButton *event, map_env_t *map_env) {
    if (event->button == GDK_BUTTON_PRIMARY) {
        mouse_held = TRUE;
        mouse_dragged = FALSE;
        mouse_last_x = event->x;
        mouse_last_y = event->y;
        mouse_paper = map_env_get_paper_at(map_env, event->x, event->y);
    }

    return TRUE; // we handled the event, stop processing
}

static gboolean button_release_event_callback(GtkWidget *widget, GdkEventButton *event, map_env_t *map_env) {
    if (event->button == GDK_BUTTON_PRIMARY) {
        mouse_held = FALSE;
        if (!mouse_dragged) {
            if (mouse_paper != NULL) {
                paper_t *p = mouse_paper;
                printf("paper[%d] = %d (%d refs, %d cites) %s -- %s\n", p->index, p->id, p->num_refs, p->num_cites, p->authors, p->title);
            }
        }
        mouse_paper = NULL;
    }

    return TRUE; // we handled the event, stop processing
}

static gboolean pointer_motion_event_callback(GtkWidget *widget, GdkEventMotion *event, map_env_t *map_env) {
    if (mouse_held) {
        if (!mouse_dragged) {
            if (fabs(mouse_last_x - event->x) > 4 || fabs(mouse_last_y - event->y) > 4) {
                mouse_dragged = TRUE;
            }
        } else {
            if (mouse_paper != NULL) {
                double x = event->x;
                double y = event->y;
                map_env_screen_to_world(map_env, &x, &y);
                mouse_paper->x = x;
                mouse_paper->y = y;
            }
        }
    }
    return TRUE;
}

static gboolean map_env_update(map_env_t *map_env) {
    //map_env_forces(map_env, 0, 1);
    //map_env_grow(map_env, 1.001);
    for (int i = 0; i < 5; i++) {
        if (map_env_forces(map_env, 1, 1, do_tred, mouse_paper)) {
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

    int num_papers;
    paper_t *papers;
    if (!load_papers_from_mysql("hep-th", &num_papers, &papers)) {
        return 1;
    }
    map_env_set_papers(map_env, num_papers, papers);
    //map_env_random_papers(map_env, 1000);
    //map_env_papers_test2(map_env, 100);

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

    g_signal_connect(window, "key-press-event", G_CALLBACK(key_press_event_callback), map_env);

    // create the drawing area
    GtkWidget *drawing_area = gtk_drawing_area_new();
    gtk_container_add(GTK_CONTAINER(window), drawing_area);
    gtk_window_set_position(GTK_WINDOW(window), GTK_WIN_POS_CENTER);
    gtk_window_set_default_size(GTK_WINDOW(window), 300, 300);

    g_timeout_add(100 /* milliseconds */, (GSourceFunc)map_env_update, map_env);
    g_signal_connect(drawing_area, "draw", G_CALLBACK(draw_callback), map_env);
    g_signal_connect(drawing_area, "button-press-event", G_CALLBACK(button_press_event_callback), map_env);
    g_signal_connect(drawing_area, "button-release-event", G_CALLBACK(button_release_event_callback), map_env);
    g_signal_connect(drawing_area, "motion-notify-event", G_CALLBACK(pointer_motion_event_callback), map_env);

    /* Ask to receive events the drawing area doesn't normally subscribe to
    */
    gtk_widget_set_events(drawing_area, gtk_widget_get_events(drawing_area)
        | GDK_LEAVE_NOTIFY_MASK
        | GDK_BUTTON_PRESS_MASK
        | GDK_BUTTON_RELEASE_MASK
        | GDK_POINTER_MOTION_MASK
        | GDK_POINTER_MOTION_HINT_MASK);

    /* Make sure that everything, window and label, are visible */
    gtk_widget_show_all(window);

    // print some help text
    printf(
        "key bindings\n"
        "    a - add 1 paper\n"
        "    b - add 10 papers\n"
        "    c - add 100 papers\n"
        "    t - turn tred on/off\n"
        "    j - make a small jolt\n"
        "    k - make a large jolt\n"
        "    q - quit\n"
        "mouse bindings\n"
        "    left click - show info about a paper\n"
        "    left drag - move a paper around\n"
    );

    /*
    ** Start the main loop, and do nothing (block) until
    ** the application is closed
    */
    gtk_main();

    return 0;
}
