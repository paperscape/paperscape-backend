#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <math.h>
#include <sys/time.h>
#include <gtk/gtk.h>
#include <cairo.h>

#include "util/xiwilib.h"
#include "common.h"
#include "layout.h"
#include "map.h"
#include "mysql.h"
#include "json.h"
#include "mapmysql.h"
#include "mapcairo.h"
#include "cairohelper.h"

vstr_t *vstr;
GtkWidget *window;
GtkTextBuffer *text_buf;
GtkWidget *statusbar;
guint statusbar_context_id;

const char *included_papers_string = NULL;
bool update_running = true;
int boost_step_size = 0;
bool mouse_held = false;
bool mouse_dragged;
bool auto_refine = true;
static int iterate_counter_full_refine = 0;
bool lock_view_all = true;
double mouse_last_x = 0, mouse_last_y = 0;
layout_node_t *mouse_layout_node_held = NULL;
layout_node_t *mouse_layout_node_prev = NULL;


static int iterate_counter = 0;
static double iters_per_sec = 1.; 
static int converged_counter = 0;
static gboolean map_env_update(map_env_t *map_env) {
    struct timeval tp;
    gettimeofday(&tp, NULL);
    int start_time = tp.tv_sec * 1000 + tp.tv_usec / 1000;
    for (int i = 1; i <= 5; i++) {
        iterate_counter += 1;
        if (map_env_iterate(map_env, mouse_layout_node_held, boost_step_size > 0, false)) {
            converged_counter += 1;
        } else {
            converged_counter = 0;
        }
        if (boost_step_size > 0) {
            boost_step_size -= 1;
        }
        if (i == 5) {
            gettimeofday(&tp, NULL);
            int end_time = tp.tv_sec * 1000 + tp.tv_usec / 1000;
            iters_per_sec = 5.0 * 1000.0 / (end_time - start_time);
        }
    }

    if (auto_refine) {
        if (iterate_counter_full_refine > 0 && iterate_counter > iterate_counter_full_refine) {
            if (map_env_number_of_finer_layouts(map_env) > 1) {
                printf("auto refine: refine with close repulsion\n");
                map_env_refine_layout(map_env);
                boost_step_size = 1;
                iterate_counter_full_refine = iterate_counter + 2000;
            } else {
                printf("auto refine: refine with close repulsion final\n");
                map_env_refine_layout(map_env);
                auto_refine = false;
            }
        } else if (converged_counter > 100) {
            if (map_env_number_of_finer_layouts(map_env) > 1) {
                printf("auto refine: refine\n");
                map_env_refine_layout(map_env);
                boost_step_size = 1;
            } else {
                printf("auto refine: do close repulsion\n");
                map_env_set_do_close_repulsion(map_env, true);
                boost_step_size = 1;
                iterate_counter_full_refine = iterate_counter + 2000;
            }
        }
    }

    if (iterate_counter % 50 == 0) {
        // force a redraw
        gtk_widget_queue_draw(window);
    }

    return TRUE; // yes, we want to be called again
}

static void draw_to_png(map_env_t *map_env, int width, int height, const char *file, vstr_t *vstr_info) {
    cairo_surface_t *surface = cairo_image_surface_create(CAIRO_FORMAT_RGB24, width, height);
    cairo_t *cr = cairo_create(surface);
    map_env_draw(map_env, cr, width, height, NULL);

    if (vstr_info != NULL) {
        cairo_identity_matrix(cr);
        cairo_set_source_rgb(cr, 0, 0, 0);
        cairo_set_font_size(cr, 10);
        cairo_helper_draw_text_lines(cr, 10, 20, vstr_info);
    }

    cairo_status_t status = cairo_surface_write_to_png(surface, file);
    cairo_destroy(cr);
    cairo_surface_destroy(surface);
    if (status != CAIRO_STATUS_SUCCESS) {
        printf("ERROR: cannot write PNG to file %s\n", file);
    } else {
        printf("wrote PNG to file %s\n", file);
    }
}

static gboolean draw_callback(GtkWidget *widget, cairo_t *cr, map_env_t *map_env) {
    guint width = gtk_widget_get_allocated_width(widget);
    guint height = gtk_widget_get_allocated_height(widget);

    vstr_reset(vstr);
    vstr_printf(vstr, "included papers: %s\n", included_papers_string);
    if (iterate_counter > 0) {
        if (lock_view_all) {
            map_env_centre_view(map_env);
            map_env_set_zoom_to_fit_n_standard_deviations(map_env, 3.0, width, height);
        }
        map_env_draw(map_env, cr, width, height, vstr);
    }
    vstr_printf(vstr, "\n");
    vstr_printf(vstr, "(A) auto refine: %d\n", auto_refine);
    vstr_printf(vstr, "(V) lock view: %d\n", lock_view_all);
    vstr_printf(vstr, "\n");
    vstr_printf(vstr, "number of iterations: %d (%.1f/sec)\n", iterate_counter,iters_per_sec);

    // draw a scale for anti-gravity radius r*
    double rstar = sqrt(map_env_get_anti_gravity(map_env)), dummy_y = 0;
    map_env_world_to_screen(map_env, &rstar, &dummy_y);
    cairo_identity_matrix(cr);
    cairo_set_source_rgb(cr, 1, 1, 1);
    cairo_set_line_width (cr, 1.5);
    cairo_helper_draw_horizontal_scale(cr,width-20,height-20,rstar,"r* scale",true);

    // draw info to canvas
    cairo_identity_matrix(cr);
    cairo_set_source_rgb(cr, 1, 1, 1);
    cairo_set_font_size(cr, 10);
    cairo_helper_draw_text_lines(cr, 10, 20, vstr);

    return FALSE;
}

static gboolean key_press_event_callback(GtkWidget *widget, GdkEventKey *event, map_env_t *map_env) {
    //printf("here %d %d\n", event->state, event->keyval);

    if (event->keyval == GDK_KEY_space) {
        if (update_running) {
            g_idle_remove_by_data(map_env);
            update_running = false;
            printf("update not running\n");
        } else {
            g_idle_add((GSourceFunc)map_env_update, map_env);
            update_running = true;
            printf("update running\n");
        }

    } else if (event->keyval == GDK_KEY_Return) {
        gtk_widget_queue_draw(window);

    } else if (event->keyval == GDK_KEY_Tab) {
        boost_step_size += 1;

        /* obsolete code to adjust date range
    } else if (event->keyval >= GDK_KEY_a && event->keyval <= GDK_KEY_f) {

               if (event->keyval == GDK_KEY_a) {
            id_range_start -= 100000;
            id_range_end -= 100000;
        } else if (event->keyval == GDK_KEY_b) {
            id_range_start += 100000;
            id_range_end += 100000;
        } else if (event->keyval == GDK_KEY_c) {
            id_range_end -= 100000;
        } else if (event->keyval == GDK_KEY_d) {
            id_range_end += 100000;
        } else if (event->keyval == GDK_KEY_e) {
        } else if (event->keyval == GDK_KEY_f) {
        }

        map_env_select_date_range(map_env, id_range_start, id_range_end, true);
        */

        /*
    } else if (event->keyval == GDK_KEY_a) {
        map_env_inc_num_papers(map_env, 1);
    } else if (event->keyval == GDK_KEY_b) {
        map_env_inc_num_papers(map_env, 10);
    } else if (event->keyval == GDK_KEY_c) {
        map_env_inc_num_papers(map_env, 100);
    } else if (event->keyval == GDK_KEY_d) {
        map_env_inc_num_papers(map_env, 1000);
    } else if (event->keyval == GDK_KEY_e) {
        map_env_inc_num_papers(map_env, 10000);
        */

    } else if (event->keyval == GDK_KEY_g) {
        map_env_toggle_draw_grid(map_env);

    } else if (event->keyval == GDK_KEY_J) {
        // write map to JSON
        vstr_reset(vstr);
        vstr_printf(vstr, "out-map_%06u.json", map_env_get_num_papers(map_env));
        map_env_layout_pos_save_to_json(map_env, vstr_str(vstr));

    } else if (event->keyval == GDK_KEY_j) {
        map_env_jolt(map_env, 0.5);
    } else if (event->keyval == GDK_KEY_k) {
        map_env_jolt(map_env, 2.5);

    } else if (event->keyval == GDK_KEY_l) {
        map_env_toggle_draw_paper_links(map_env);

    } else if (event->keyval == GDK_KEY_q) {
        gtk_main_quit();

    } else if (event->keyval == GDK_KEY_r) {
        map_env_toggle_do_close_repulsion(map_env);

    } else if (event->keyval == GDK_KEY_c) {
        map_env_toggle_draw_categories(map_env);

    } else if (event->keyval == GDK_KEY_t) {
        map_env_toggle_do_tred(map_env);

    } else if (event->keyval == GDK_KEY_v) {
        map_env_toggle_use_ref_freq(map_env);

    } else if (event->keyval == GDK_KEY_w) {
        vstr_reset(vstr);
        vstr_printf(vstr, "out-map_%06u.png", map_env_get_num_papers(map_env));
        draw_to_png(map_env, 1000, 1000, vstr_str(vstr), NULL);

    } else if (event->keyval == GDK_KEY_z) {
        map_env_centre_view(map_env);
        map_env_set_zoom_to_fit_n_standard_deviations(map_env, 3.0, 1000, 1000);

    } else if (event->keyval == GDK_KEY_A) {
        auto_refine = !auto_refine;
    } else if (event->keyval == GDK_KEY_V) {
        lock_view_all = !lock_view_all;

    } else if (event->keyval == GDK_KEY_1) {
        map_env_adjust_anti_gravity(map_env, 0.9);
    } else if (event->keyval == GDK_KEY_exclam) {
        map_env_adjust_anti_gravity(map_env, 1.1);

    } else if (event->keyval == GDK_KEY_2) {
        map_env_adjust_link_strength(map_env, 0.9);
    } else if (event->keyval == GDK_KEY_at) {
        map_env_adjust_link_strength(map_env, 1.1);

    } else if (event->keyval == GDK_KEY_3) {
        map_env_adjust_close_repulsion(map_env, 0.7, 1.0);
    } else if (event->keyval == GDK_KEY_numbersign) {
        map_env_adjust_close_repulsion(map_env, 1.5, 1.0);
    } else if (event->keyval == GDK_KEY_4) {
        map_env_adjust_close_repulsion(map_env, 1.0, 0.7);
    } else if (event->keyval == GDK_KEY_dollar) {
        map_env_adjust_close_repulsion(map_env, 1.0, 1.5);

    } else if (event->keyval == GDK_KEY_5) {
        map_env_adjust_close_repulsion2(map_env, 0.95, 0.0);
    } else if (event->keyval == GDK_KEY_percent) {
        map_env_adjust_close_repulsion2(map_env, 1.05, 0.0);
    } else if (event->keyval == GDK_KEY_6) {
        map_env_adjust_close_repulsion2(map_env, 1.0, -0.05);
    } else if (event->keyval == GDK_KEY_asciicircum) {
        map_env_adjust_close_repulsion2(map_env, 1.0, 0.05);

    } else if (event->keyval == GDK_KEY_9) {
        map_env_coarsen_layout(map_env);
    } else if (event->keyval == GDK_KEY_0) {
        map_env_refine_layout(map_env);

    } else if (event->keyval == GDK_KEY_plus || event->keyval == GDK_KEY_equal) {
        map_env_zoom(map_env, 0, 0, 1.2);
    } else if (event->keyval == GDK_KEY_minus) {
        map_env_zoom(map_env, 0, 0, 0.8);

    } else if (event->keyval == GDK_KEY_Left) {
        map_env_rotate_all(map_env, 0.1);
    } else if (event->keyval == GDK_KEY_Right) {
        map_env_rotate_all(map_env, -0.1);
    } else if (event->keyval == GDK_KEY_Up) {
        map_env_flip_x(map_env);
    }

    if (!update_running) {
        gtk_widget_queue_draw(window);
    }

    return TRUE; // we handled the event, stop processing
}

static gboolean button_press_event_callback(GtkWidget *widget, GdkEventButton *event, map_env_t *map_env) {
    if (event->button == GDK_BUTTON_PRIMARY) {
        mouse_held = TRUE;
        mouse_dragged = FALSE;
        mouse_last_x = event->x;
        mouse_last_y = event->y;
        mouse_layout_node_held = map_env_get_layout_node_at(map_env, gtk_widget_get_allocated_width(widget), gtk_widget_get_allocated_height(widget), event->x, event->y);
        mouse_layout_node_prev = mouse_layout_node_held;
    }

    return TRUE; // we handled the event, stop processing
}

static gboolean button_release_event_callback(GtkWidget *widget, GdkEventButton *event, map_env_t *map_env) {
    if (event->button == GDK_BUTTON_PRIMARY) {
        mouse_held = FALSE;
        if (!mouse_dragged) {
            if (mouse_layout_node_held != NULL && map_env_number_of_finer_layouts(map_env) == 0) {
                paper_t *p = mouse_layout_node_held->paper;
                vstr_reset(vstr);
                if (p->authors == NULL || p->title == NULL) {
                    vstr_printf(vstr, "paper[%d] = %u (%d refs, %d cites)", p->index, p->id, p->num_refs, p->num_cites);
                } else {
                    vstr_printf(vstr, "paper[%d] = %u (%d refs, %d cites) %s -- %s", p->index, p->id, p->num_refs, p->num_cites, p->title, p->authors);
                }
                gtk_statusbar_push(GTK_STATUSBAR(statusbar), statusbar_context_id, vstr_str(vstr));
                printf("%s\n", vstr_str(vstr));
            }
        }
        mouse_layout_node_held = NULL;
    } else if (event->button == GDK_BUTTON_SECONDARY) {
        if (mouse_layout_node_prev != NULL && map_env_number_of_finer_layouts(map_env) == 0) {
            // right click move previously clicked paper to this location
            double x = event->x;
            double y = event->y;
            map_env_screen_to_world(map_env, gtk_widget_get_allocated_width(widget), gtk_widget_get_allocated_height(widget), &x, &y);
            mouse_layout_node_prev->x = x;
            mouse_layout_node_prev->y = y;
            printf("moved paper %u to (%.2f,%.2f)\n", ((paper_t*)mouse_layout_node_prev->paper)->id, x, y);
            if (!update_running) {
                gtk_widget_queue_draw(window);
            }
        }
    }

    return TRUE; // we handled the event, stop processing
}

static gboolean scroll_event_callback(GtkWidget *widget, GdkEventScroll *event, map_env_t *map_env) {
    guint width = gtk_widget_get_allocated_width(widget);
    guint height = gtk_widget_get_allocated_height(widget);
    if (event->direction == GDK_SCROLL_UP) {
        map_env_zoom(map_env, event->x - 0.5 * width, event->y - 0.5 * height, 1.2);
    } else if (event->direction == GDK_SCROLL_DOWN) {
        map_env_zoom(map_env, event->x - 0.5 * width, event->y - 0.5 * height, 0.8);
    }

    if (!update_running) {
        gtk_widget_queue_draw(window);
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
            if (mouse_layout_node_held == NULL) {
                // mouse dragged on background
                double dx = event->x - mouse_last_x;
                double dy = event->y - mouse_last_y;
                map_env_scroll(map_env, dx, dy);
            } else {
                // mouse dragged on paper
                double x = event->x;
                double y = event->y;
                map_env_screen_to_world(map_env, gtk_widget_get_allocated_width(widget), gtk_widget_get_allocated_height(widget), &x, &y);
                mouse_layout_node_held->x = x;
                mouse_layout_node_held->y = y;
            }
            mouse_last_x = event->x;
            mouse_last_y = event->y;
        }

        if (!update_running) {
            gtk_widget_queue_draw(window);
        }
    }
    return TRUE;
}

// for a gtk example, see: http://git.gnome.org/browse/gtk+/tree/demos/gtk-demo/drawingarea.c

void build_gui(map_env_t *map_env, const char *papers_string) {
    vstr = vstr_new();
    included_papers_string = papers_string;

    // create the main, top level window
    window = gtk_window_new(GTK_WINDOW_TOPLEVEL);

    // give it a title
    gtk_window_set_title(GTK_WINDOW(window), "Papercape map generator");

    // create the outer box
    GtkWidget *box_outer = gtk_box_new(GTK_ORIENTATION_VERTICAL, 0);
    gtk_container_add(GTK_CONTAINER(window), box_outer);

    // create the controls
    /*
    GtkWidget *box = gtk_box_new(GTK_ORIENTATION_HORIZONTAL, 2);
    GtkWidget *ctrl_tred = gtk_toggle_button_new_with_label("transitive reduction");
    GtkWidget *ctrl_draw_links = gtk_toggle_button_new_with_label("draw links");
    GtkWidget *ctrl_draw_grid = gtk_toggle_button_new_with_label("draw grid");
    GtkWidget *xxx = gtk_scale_new_with_range(GTK_ORIENTATION_HORIZONTAL, 0.1, 10, 0.1);
    gtk_toggle_button_set_active(GTK_TOGGLE_BUTTON(ctrl_tred), TRUE);
    gtk_toggle_button_set_active(GTK_TOGGLE_BUTTON(ctrl_draw_links), TRUE);
    gtk_toggle_button_set_active(GTK_TOGGLE_BUTTON(ctrl_draw_grid), FALSE);
    gtk_scale_set_digits(GTK_SCALE(xxx), 3);
    gtk_widget_set_size_request(xxx, 80, -1);
    gtk_box_pack_start(GTK_BOX(box), ctrl_tred, FALSE, FALSE, 0);
    gtk_box_pack_start(GTK_BOX(box), ctrl_draw_links, FALSE, FALSE, 0);
    gtk_box_pack_start(GTK_BOX(box), ctrl_draw_grid, FALSE, FALSE, 0);
    gtk_box_pack_start(GTK_BOX(box), xxx, FALSE, FALSE, 0);
    gtk_box_pack_start(GTK_BOX(box_outer), box, FALSE, FALSE, 0);
    */

    // create the drawing area
    GtkWidget *drawing_area = gtk_drawing_area_new();
    GtkWidget *drawing_area_frame = gtk_frame_new(NULL);
    gtk_container_add(GTK_CONTAINER(drawing_area_frame), drawing_area);
    gtk_frame_set_shadow_type(GTK_FRAME(drawing_area_frame), GTK_SHADOW_IN);
    //gtk_widget_set_size_request(drawing_area_frame, 50, -1);
    gtk_box_pack_start(GTK_BOX(box_outer), drawing_area_frame, TRUE, TRUE, 0);

    // create the console box
    /*
    GtkWidget *text_area = gtk_text_view_new();
    GtkWidget *scrolled_text_area = gtk_scrolled_window_new(NULL, NULL);
    GtkWidget *console_frame = gtk_frame_new("Output console");
    gtk_container_add(GTK_CONTAINER(scrolled_text_area), text_area);
    gtk_container_add(GTK_CONTAINER(console_frame), scrolled_text_area);
    gtk_frame_set_shadow_type(GTK_FRAME(console_frame), GTK_SHADOW_IN);
    gtk_widget_set_size_request(console_frame, 50, -1);
    gtk_paned_pack2(GTK_PANED(paned), console_frame, FALSE, FALSE);

    text_buf = gtk_text_view_get_buffer(GTK_TEXT_VIEW(text_area));
    */

    // create the status bar
    statusbar = gtk_statusbar_new();
    statusbar_context_id = gtk_statusbar_get_context_id(GTK_STATUSBAR(statusbar), "status");
    gtk_box_pack_end(GTK_BOX(box_outer), statusbar, FALSE, FALSE, 0);

    /*
    ** Map the destroy signal of the window to gtk_main_quit;
    ** When the window is about to be destroyed, we get a notification and
    ** stop the main GTK+ loop by returning 0
    */
    g_signal_connect(window, "destroy", G_CALLBACK(gtk_main_quit), NULL);

    g_signal_connect(window, "key-press-event", G_CALLBACK(key_press_event_callback), map_env);

    //g_timeout_add(100 /* milliseconds */, (GSourceFunc)map_env_update, map_env);
    g_idle_add((GSourceFunc)map_env_update, map_env);
    g_signal_connect(drawing_area, "draw", G_CALLBACK(draw_callback), map_env);
    g_signal_connect(drawing_area, "button-press-event", G_CALLBACK(button_press_event_callback), map_env);
    g_signal_connect(drawing_area, "button-release-event", G_CALLBACK(button_release_event_callback), map_env);
    g_signal_connect(drawing_area, "scroll-event", G_CALLBACK(scroll_event_callback), map_env);
    g_signal_connect(drawing_area, "motion-notify-event", G_CALLBACK(pointer_motion_event_callback), map_env);

    /* Ask to receive events the drawing area doesn't normally subscribe to
    */
    gtk_widget_set_events(drawing_area, gtk_widget_get_events(drawing_area)
        | GDK_LEAVE_NOTIFY_MASK
        | GDK_BUTTON_PRESS_MASK
        | GDK_BUTTON_RELEASE_MASK
        | GDK_SCROLL_MASK
        | GDK_POINTER_MOTION_MASK
        | GDK_POINTER_MOTION_HINT_MASK);

    /* Make sure that everything, window and label, are visible */
    gtk_widget_show_all(window);

    // print some help text
    printf(
        "key bindings\n"
        " space- play/pause the physics update\n"
        /*
        "    a - decrease whole id range by 1/10 of a year\n"
        "    b - increase whole id range by 1/10 of a year\n"
        "    c - decrease end of id range by 1/10 of a year\n"
        "    d - increase end of id range by 1/10 of a year\n"
        */
        "    t - turn tred on/off\n"
        "    l - turn links on/off\n"
        "    j - make a small jolt\n"
        "    k - make a large jolt\n"
        "    q - quit\n"
        "mouse bindings\n"
        "    left click - show info about a paper\n"
        "    left drag - move a paper around / pan the view\n"
        "       scroll - zoom in/out\n"
    );
}

static int usage(const char *progname) {
    printf("\n");
    printf("usage: %s [options]\n", progname);
    printf("\n");
    printf("options:\n");
    printf("    --settings, -s <file>   load settings from given JSON file\n");
    printf("    --layout-db             load layout from DB\n");
    printf("    --layout-json <file>    load layout from given JSON file\n");
    printf("    --refs-json <file>      load reference data from JSON file (default is from DB)\n");
    printf("                            this omits author and title data\n");
    printf("    --no-fake-links, -nf    don't create fake links\n");
    printf("    --link <num>            link strength\n");
    printf("    --rsq <num>             r-star squared distance for anti-gravity\n");
    printf("\n");
    return 1;
}

int main(int argc, char *argv[]) {

    // parse command line arguments
    double arg_anti_grav_rsq    = -1;
    double arg_link_strength    = -1;
    bool arg_layout_db          = false;
    bool arg_no_fake_links      = false;
    const char *arg_settings    = NULL;
    const char *arg_layout_json = NULL;
    const char *arg_refs_json   = NULL;
    for (int a = 1; a < argc; a++) {
        if (streq(argv[a], "--settings") || streq(argv[a], "-s")) {
            a += 1;
            if (a >= argc) {
                return usage(argv[0]);
            }
            arg_settings = argv[a];
        } else if (streq(argv[a], "--rsq")) {
            a += 1;
            if (a >= argc) {
                return usage(argv[0]);
            }
            arg_anti_grav_rsq = strtod(argv[a], NULL);
        } else if (streq(argv[a], "--link")) {
            a += 1;
            if (a >= argc) {
                return usage(argv[0]);
            }
            arg_link_strength = strtod(argv[a], NULL);
        } else if (streq(argv[a], "--layout-db")) {
            arg_layout_db = true;
        } else if (streq(argv[a], "--layout-json")) {
            a += 1;
            if (a >= argc) {
                return usage(argv[0]);
            }
            arg_layout_json = argv[a];
        } else if (streq(argv[a], "--refs-json")) {
            a += 1;
            if (a >= argc) {
                return usage(argv[0]);
            }
            arg_refs_json = argv[a];
        } else if (streq(argv[a], "--no-fake-links") || streq(argv[a], "-nf")) {
            arg_no_fake_links = true;
        } else {
            return usage(argv[0]);
        }
    }

    // load settings from json file
    const char *settings_file = "settings.json";
    if (arg_settings != NULL) {
        settings_file = arg_settings;
    }
    init_config_t *init_config = NULL;
    if (!init_config_new(settings_file,&init_config)) {
        return 1;
    }

    //const char *where_clause = "(maincat='hep-th' OR maincat='hep-ph') AND id >= 2100000000";
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
    //const char *where_clause = "(arxiv IS NOT NULL AND status != 'WDN')";

    // load the papers from the DB
    int num_papers;
    paper_t *papers;
    hashmap_t *keyword_set;
    if (arg_refs_json == NULL) {
        // load the papers from the DB
        if (!mysql_load_papers(init_config, true, &num_papers, &papers, &keyword_set)) {
            return 1;
        }
    } else {
        // load the papers from a JSON file
        // NOTE: this does not load authors and titles
        if (!json_load_papers(arg_refs_json, &num_papers, &papers, &keyword_set)) {
            return 1;
        }
    }

    // create the map object
    map_env_t *map_env = map_env_new();

    // set initial configuration
    if (init_config != NULL) {
        map_env_set_init_config(map_env,init_config);
    }

    // whether to create fake links for disconnected papers
    map_env_set_make_fake_links(map_env,!arg_no_fake_links);

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
        unsigned int id_min;
        unsigned int id_max;
        map_env_get_max_id_range(map_env, &id_min, &id_max);
        unsigned int id_range_start = id_min;
        unsigned int id_range_end   = id_max;

        // for starting part way through
        id_range_start = date_to_unique_id(2012, 3, 0);
        id_range_end = id_range_start + 20000000; // plus 2 years
        id_range_end = id_range_start +  3000000; // plus 0.5 year
        id_range_start = id_min; id_range_end = id_max; // full range

        //id_range_end = id_max - 120000000; // minus 2 years
        map_env_select_date_range(map_env, id_range_start, id_range_end);
    }

    if (arg_layout_db) {
        map_env_layout_pos_load_from_db(map_env);
    } else if (arg_layout_json != NULL) {
        map_env_layout_pos_load_from_json(map_env, arg_layout_json);
    } else {
        map_env_layout_new(map_env, 10, 1, 0);
    }

    // init gtk
    gtk_init(&argc, &argv);

    build_gui(map_env, init_config->sql_extra_clause);

    // start the main loop and block until the application is closed
    gtk_main();

    return 0;
}
