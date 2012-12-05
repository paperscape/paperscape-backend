#include <unistd.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <ctype.h>
#include <mysql/mysql.h>
#include <math.h>
#include <stdarg.h>
#include <gtk/gtk.h>

#include "xiwilib.h"

GtkWidget *window;

typedef struct _paper_t {
    int kind;
    double x;
    double y;
    double r;
} paper_t;

typedef struct _map_env_t {
    int num_papers;
    paper_t *papers;
    int grid_w;
    int grid_h;
    int grid_d;
    paper_t **grid;
} map_env_t;

map_env_t *map_env_new() {
    map_env_t *map_env = m_new(map_env_t, 1);
    map_env->num_papers = 0;
    map_env->papers = NULL;
    map_env->grid_w = 30;
    map_env->grid_h = 30;
    map_env->grid_d = 5;
    map_env->grid = m_new0(paper_t*, map_env->grid_w * map_env->grid_h * map_env->grid_d);
    return map_env;
}

void map_env_random_papers(map_env_t *map_env, int n) {
    map_env->num_papers = n;
    map_env->papers = m_renew(paper_t, map_env->papers, n);
    for (int i = 0; i < n; i++) {
        paper_t *p = &map_env->papers[i];
        p->kind = 2.5 * random() / RAND_MAX;
        p->x = map_env->grid_w * 1.0 * random() / RAND_MAX;
        p->y = map_env->grid_h * 1.0 * random() / RAND_MAX;
        p->r = 0.1 + 0.05 / (1.0 * random() / RAND_MAX);
        if (p->r > 4) { p->r = 4; }
    }
}

void draw_paper(cairo_t *cr, map_env_t *map_env, double x, double y, double w, unsigned int kind) {
    /*
    double h = w * 1.41;
    cairo_set_source_rgba(cr, 0.9, 0.9, 0.8, 0.9);
    cairo_rectangle(cr, x-0.5*w, y-0.5*h, w, h);
    cairo_fill(cr);
    cairo_set_source_rgba(cr, 0, 0, 0, 0.5);
    cairo_rectangle(cr, x-0.5*w, y-0.5*h, w, h);
    cairo_stroke(cr);
    */
    x = 600.0 * x / map_env->grid_w;
    y = 600.0 * y / map_env->grid_h;
    w *= 600.0 / map_env->grid_w;
    if (kind == 1) {
        cairo_set_source_rgba(cr, 0, 0, 0.8, 0.5);
    } else if (kind == 2) {
        cairo_set_source_rgba(cr, 0.8, 0, 0, 0.5);
    } else {
        cairo_set_source_rgba(cr, 0, 0.8, 0, 0.5);
    }
    cairo_arc(cr, x, y, w, 0, 2 * M_PI);
    cairo_fill(cr);
}

void map_env_draw(cairo_t *cr, map_env_t *map_env) {
    for (int i = 0; i < map_env->num_papers; i++) {
        paper_t *p = &map_env->papers[i];
        draw_paper(cr, map_env, p->x, p->y, p->r, p->kind);
    }
}

void map_env_forces(map_env_t *map_env) {
    // compute grid
    memset(map_env->grid, 0, map_env->grid_w * map_env->grid_h * map_env->grid_d * sizeof(paper_t*));
    int overflow = 0;
    for (int i = 0; i < map_env->num_papers; i++) {
        paper_t *p = &map_env->papers[i];
        int x1 = floor(p->x - p->r);
        int y1 = floor(p->y - p->r);
        int x2 = floor(p->x + p->r);
        int y2 = floor(p->y + p->r);
        if (x1 < 0) { x1 = 0; } else if (x1 > map_env->grid_w - 1) { x1 = map_env->grid_w - 1; }
        if (y1 < 0) { y1 = 0; } else if (y1 > map_env->grid_h - 1) { y1 = map_env->grid_h - 1; }
        if (x2 < 0) { x2 = 0; } else if (x2 > map_env->grid_w - 1) { x2 = map_env->grid_w - 1; }
        if (y2 < 0) { y2 = 0; } else if (y2 > map_env->grid_h - 1) { y2 = map_env->grid_h - 1; }
        for (int y = y1; y <= y2; y++) {
            for (int x = x1; x <= x2; x++) {
                paper_t **g = &map_env->grid[(y * map_env->grid_w + x) * map_env->grid_d];
                int d;
                for (d = 0; d < map_env->grid_d - 1 && *g != NULL; d++, g++) { }
                if (d < map_env->grid_d) {
                    //printf("paper %d at %d %d %d\n", i, x, y, d);
                    *g = p;
                } else {
                    //printf("paper %d at %d %d overflow\n", i, x, y);
                    overflow = 1;
                }
            }
        }
    }
    if (overflow) {
        printf("grid depth overflow\n");
    }

    // work out forces
    for (int i = 0; i < map_env->num_papers; i++) {
        paper_t *p = &map_env->papers[i];
        int x1 = floor(p->x - p->r);
        int y1 = floor(p->y - p->r);
        int x2 = floor(p->x + p->r);
        int y2 = floor(p->y + p->r);
        if (x1 < 0) { x1 = 0; } else if (x1 > map_env->grid_w - 1) { x1 = map_env->grid_w - 1; }
        if (y1 < 0) { y1 = 0; } else if (y1 > map_env->grid_h - 1) { y1 = map_env->grid_h - 1; }
        if (x2 < 0) { x2 = 0; } else if (x2 > map_env->grid_w - 1) { x2 = map_env->grid_w - 1; }
        if (y2 < 0) { y2 = 0; } else if (y2 > map_env->grid_h - 1) { y2 = map_env->grid_h - 1; }
        double fx = 0;
        double fy = 0;
        for (int y = y1; y <= y2; y++) {
            for (int x = x1; x <= x2; x++) {
                paper_t **g = &map_env->grid[(y * map_env->grid_w + x) * map_env->grid_d];
                for (; *g != NULL; g++) {
                    paper_t *p2 = *g;
                    if (p != p2) {
                        double dx = p->x - p2->x;
                        double dy = p->y - p2->y;
                        double r = sqrt(dx*dx + dy*dy);
                        if (r < p->r + p2->r) {
                            fx += dx / r / r;
                            fy += dy / r / r;
                        }
                    }
                }
            }
        }
        if (p->y + p->r < map_env->grid_h) {
            fy += 0.7; // gravity!
        }
        p->x += 0.005 * fx / p->r;
        p->y += 0.005 * fy / p->r;
        if (p->x - p->r < 0) {
            p->x = p->r;
        }
        if (p->x + p->r > map_env->grid_w) {
            p->x = map_env->grid_w - p->r;
        }
        if (p->y + p->r > map_env->grid_h) {
            p->y = map_env->grid_h - p->r;
        }
    }
}

static gboolean on_expose_event(GtkWidget *widget, cairo_t *cr, map_env_t *map_env) {
    map_env_draw(cr, map_env);
    return TRUE;
}

static gboolean map_env_update(map_env_t *map_env) {
    map_env_forces(map_env);
    // force a redraw
    gtk_widget_queue_draw(window);
    return TRUE;
}

/****************************************************************/

int main(int argc, char *argv[]) {
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

    map_env_t *map_env = map_env_new();
    map_env_random_papers(map_env, 100);

    g_timeout_add(20 /* milliseconds */, (GSourceFunc)map_env_update, map_env);
    g_signal_connect(darea, "draw", G_CALLBACK(on_expose_event), map_env);

    /*
    ** Start the main loop, and do nothing (block) until
    ** the application is closed
    */
    gtk_main();

    return 0;
}
