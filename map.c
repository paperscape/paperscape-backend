#include <stdlib.h>
#include <string.h>
#include <math.h>
#include <gtk/gtk.h>

#include "xiwilib.h"
#include "common.h"
#include "map.h"

struct _map_env_t {
    int num_papers;
    paper_t *papers;

    int grid_w;
    int grid_h;
    int grid_d;
    paper_t **grid;
    double scale;
};

map_env_t *map_env_new() {
    map_env_t *map_env = m_new(map_env_t, 1);
    map_env->num_papers = 0;
    map_env->papers = NULL;
    map_env->grid_w = 50;
    map_env->grid_h = 50;
    map_env->grid_d = 20;
    map_env->grid = m_new0(paper_t*, map_env->grid_w * map_env->grid_h * map_env->grid_d);
    map_env->scale = 600.0/map_env->grid_w;
    return map_env;
}

void map_env_set_papers(map_env_t *map_env, int num_papers, paper_t *papers) {
    map_env->num_papers = num_papers;
    map_env->papers = papers;
    for (int i = 0; i < map_env->num_papers; i++) {
        paper_t *p = &map_env->papers[i];
        p->kind = 2.5 * random() / RAND_MAX;
        p->mass = 0.05 + 0.1 * p->num_cites;
        p->r = sqrt(p->mass / M_PI);
        p->x = map_env->grid_w * 1.0 * random() / RAND_MAX;
        p->y = map_env->grid_h * 1.0 * random() / RAND_MAX;
    }
}

void map_env_random_papers(map_env_t *map_env, int n) {
    map_env->num_papers = n;
    map_env->papers = m_renew(paper_t, map_env->papers, n);
    for (int i = 0; i < n; i++) {
        paper_t *p = &map_env->papers[i];
        p->kind = 2.5 * random() / RAND_MAX;
        p->r = 0.1 + 0.05 / (0.01 + 1.0 * random() / RAND_MAX);
        if (p->r > 4) { p->r = 4; }
        p->mass = M_PI * p->r * p->r;
        p->x = map_env->grid_w * 1.0 * random() / RAND_MAX;
        p->y = map_env->grid_h * 1.0 * random() / RAND_MAX;
        p->index = i;
        p->num_refs = 0;
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
    x *= map_env->scale;
    y *= map_env->scale;
    w *= map_env->scale;
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
    double scale = map_env->scale;

    /*
    // grid density
    for (int j = 0; j < map_env->grid_h; j++) {
        for (int i = 0; i < map_env->grid_w; i++) {
            paper_t **g = &map_env->grid[(j * map_env->grid_w + i) * map_env->grid_d];
            int d = 0;
            for (; *g != NULL; g++, d++) { }
            cairo_set_source_rgba(cr, 0, 0, 0, 1.0 * d / map_env->grid_d);
            cairo_rectangle(cr, scale * i, scale * j, scale, scale);
            cairo_fill(cr);
        }
    }
    */

    // grid lines
    cairo_set_line_width(cr, 0.5);
    cairo_set_source_rgba(cr, 0, 0, 0, 0.5);
    for (int i = 0; i <= map_env->grid_w; i++) {
        cairo_move_to(cr, scale*i, 0);
        cairo_line_to(cr, scale*i, scale*map_env->grid_h);
    }
    for (int i = 0; i <= map_env->grid_h; i++) {
        cairo_move_to(cr, 0, scale*i);
        cairo_line_to(cr, scale*map_env->grid_w, scale*i);
    }
    cairo_stroke(cr);

    // papers
    for (int i = 0; i < map_env->num_papers; i++) {
        paper_t *p = &map_env->papers[i];
        draw_paper(cr, map_env, p->x, p->y, p->r, p->kind);
    }
}

void map_env_forces(map_env_t *map_env, bool do_attr) {
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
                if (d < map_env->grid_d - 1) {
                    //printf("paper %d at %d %d %d\n", i, x, y, d);
                    *g = p;
                } else {
                    //printf("paper %d at %d %d overflow\n", i, x, y);
                    overflow = 1;
                }
            }
        }
        p->fx = 0;
        p->fy = 0;
    }
    if (overflow) {
        printf("grid depth overflow\n");
    }

    // repulsion from touching
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
                for (; *g != NULL; g++) {
                    paper_t *p2 = *g;
                    if (p < p2) {
                        double dx = p->x - p2->x;
                        double dy = p->y - p2->y;
                        double r = sqrt(dx*dx + dy*dy);
                        if (r < p->r + p2->r) {
                            double fx = dx / r;
                            double fy = dy / r;
                            p->fx += fx;
                            p->fy += fy;
                            p2->fx -= fx;
                            p2->fy -= fy;
                        }
                    }
                }
            }
        }
    }

    // attraction due to links
    if (do_attr) {
    for (int i = 0; i < map_env->num_papers; i++) {
        paper_t *p1 = &map_env->papers[i];
        for (int j = 0; j < p1->num_refs; j++) {
            paper_t *p2 = p1->refs[j];
            double dx = p1->x - p2->x;
            double dy = p1->y - p2->y;
            double r = sqrt(dx*dx + dy*dy);
            if (r > 1.1 * (p1->r + p2->r)) {
                double fac = 0.1;
                double fx = dx * fac;
                double fy = dy * fac;
                p1->fx -= fx;
                p1->fy -= fy;
                p2->fx += fx;
                p2->fy += fy;
            }
        }
    }
    }


    /*
    // gravity!
    for (int i = 0; i < map_env->num_papers; i++) {
        paper_t *p1 = &map_env->papers[i];
        for (int j = i + 1; j < map_env->num_papers; j++) {
            paper_t *p2 = &map_env->papers[j];
            double dx = p1->x - p2->x;
            double dy = p1->y - p2->y;
            double r = sqrt(dx*dx + dy*dy);
            if (r > p1->r + p2->r) {
                double fac = 0.8 * p1->mass * p2->mass / pow(r, 3);
                double fx = dx * fac;
                double fy = dy * fac;
                p1->fx -= fx;
                p1->fy -= fy;
                p2->fx += fx;
                p2->fy += fy;
            }
        }
    }
    */

    // apply forces
    for (int i = 0; i < map_env->num_papers; i++) {
        paper_t *p = &map_env->papers[i];

        p->x += 0.005 * p->fx / p->mass;
        p->y += 0.005 * p->fy / p->mass;

        // apply boundary conditions
        if (p->x - p->r < 0) {
            p->x = p->r;
        }
        if (p->x + p->r > map_env->grid_w) {
            p->x = map_env->grid_w - p->r;
        }
        if (p->y + p->r > map_env->grid_h) {
            p->y = map_env->grid_h - p->r;
        }

        // force y-position
        //p->y = 0.05 * p->index;
    }
}
