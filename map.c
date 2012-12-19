#include <stdlib.h>
#include <string.h>
#include <math.h>
#include <gtk/gtk.h>

#include "xiwilib.h"
#include "common.h"
#include "map.h"

struct _map_env_t {
    int cur_num_papers;
    int max_num_papers;
    paper_t *papers;

    int grid_w;
    int grid_h;
    int grid_d;
    paper_t **grid;
    double scale;
};

map_env_t *map_env_new() {
    map_env_t *map_env = m_new(map_env_t, 1);
    map_env->cur_num_papers = 0;
    map_env->max_num_papers = 0;
    map_env->papers = NULL;
    map_env->grid_w = 80;
    map_env->grid_h = 50;
    map_env->grid_d = 20;
    map_env->grid = m_new0(paper_t*, map_env->grid_w * map_env->grid_h * map_env->grid_d);
    map_env->scale = 600.0/map_env->grid_w;
    return map_env;
}

void map_env_world_to_screen(map_env_t *map_env, double *x, double *y) {
    *x *= map_env->scale;
    *y *= map_env->scale;
}

void map_env_screen_to_world(map_env_t *map_env, double *x, double *y) {
    *x /= map_env->scale;
    *y /= map_env->scale;
}

void map_env_set_papers(map_env_t *map_env, int num_papers, paper_t *papers) {
    map_env->max_num_papers = num_papers;
    map_env->papers = papers;
    for (int i = 0; i < map_env->max_num_papers; i++) {
        paper_t *p = &map_env->papers[i];
        p->refs_tred_computed = m_new(int, p->num_refs);
        //p->kind = 2.5 * random() / RAND_MAX;
        p->kind = p->maincat;
        p->mass = 0.05 + 0.2 * p->num_cites;
        p->r = sqrt(p->mass / M_PI);
        p->x = map_env->grid_w * 1.0 * random() / RAND_MAX;
        p->y = map_env->grid_h * 1.0 * random() / RAND_MAX;
    }
}

void map_env_random_papers(map_env_t *map_env, int n) {
    map_env->max_num_papers = n;
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

void map_env_papers_test1(map_env_t *map_env, int n) {
    // the first paper is cited by the rest
    map_env->max_num_papers = n;
    map_env->papers = m_renew(paper_t, map_env->papers, n);
    for (int i = 0; i < n; i++) {
        paper_t *p = &map_env->papers[i];
        p->kind = 1;
        if (i == 0) {
            p->mass = 0.05 + 0.1 * (n - 1);
        } else {
            p->mass = 0.05;
        }
        p->r = sqrt(p->mass / M_PI);
        p->x = map_env->grid_w * 1.0 * random() / RAND_MAX;
        p->y = map_env->grid_h * 1.0 * random() / RAND_MAX;
        p->index = i;
        if (i == 0) {
            p->num_refs = 0;
        } else {
            p->num_refs = 1;
            p->refs = m_new(paper_t*, 1);
            p->refs[0] = &map_env->papers[0];
        }
    }
}

void map_env_papers_test2(map_env_t *map_env, int n) {
    // the first 2 papers are cited both by the rest
    map_env->max_num_papers = n;
    map_env->papers = m_renew(paper_t, map_env->papers, n);
    for (int i = 0; i < n; i++) {
        paper_t *p = &map_env->papers[i];
        p->kind = 1;
        if (i < 2) {
            p->mass = 0.05 + 0.1 * (n - 2);
        } else {
            p->mass = 0.05;
        }
        p->r = sqrt(p->mass / M_PI);
        p->x = map_env->grid_w * 1.0 * random() / RAND_MAX;
        p->y = map_env->grid_h * 1.0 * random() / RAND_MAX;
        p->index = i;
        if (i < 2) {
            p->num_refs = 0;
        } else {
            p->num_refs = 2;
            p->refs = m_new(paper_t*, 2);
            p->refs[0] = &map_env->papers[0];
            p->refs[1] = &map_env->papers[1];
        }
    }
}

void draw_paper(cairo_t *cr, map_env_t *map_env, paper_t *p) {
    /*
    double h = w * 1.41;
    cairo_set_source_rgba(cr, 0.9, 0.9, 0.8, 0.9);
    cairo_rectangle(cr, x-0.5*w, y-0.5*h, w, h);
    cairo_fill(cr);
    cairo_set_source_rgba(cr, 0, 0, 0, 0.5);
    cairo_rectangle(cr, x-0.5*w, y-0.5*h, w, h);
    cairo_stroke(cr);
    */
    double x = p->x * map_env->scale;
    double y = p->y * map_env->scale;
    double w = p->r * map_env->scale;
    if (p->id == 1992546899 || p->id == 1993234723) {
        cairo_set_source_rgba(cr, 0.8, 0.8, 0, 0.7);
    } else if (p->kind == 1) {
        cairo_set_source_rgba(cr, 0, 0, 0.8, 0.7);
    } else if (p->kind == 2) {
        cairo_set_source_rgba(cr, 0.8, 0, 0, 0.7);
    } else {
        cairo_set_source_rgba(cr, 0, 0.8, 0, 0.7);
    }

    cairo_arc(cr, x, y, w, 0, 2 * M_PI);
    cairo_fill(cr);
}

static bool paper_include(paper_t *p) {
    return p->num_with_my_colour > 2;
}

void map_env_draw(map_env_t *map_env, cairo_t *cr, guint width, guint height, bool do_tred) {
    double scale_w = 1.0 * width / map_env->grid_w;
    double scale_h = 1.0 * height / map_env->grid_h;
    double scale;
    if (scale_w < scale_h) {
        scale = scale_w;
    } else {
        scale = scale_h;
    }
    map_env->scale = scale;

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

    // paper links
    cairo_set_line_width(cr, 0.5);
    cairo_set_source_rgba(cr, 0, 0, 0, 0.7);
    for (int i = 0; i < map_env->cur_num_papers; i++) {
        paper_t *p = &map_env->papers[i];
        if (!paper_include(p)) {
            continue;
        }
        for (int j = 0; j < p->num_refs; j++) {
            paper_t *p2 = p->refs[j];
            if (!paper_include(p2)) {
                continue;
            }
            if ((!do_tred || p->refs_tred_computed[j]) && p2->index < map_env->cur_num_papers) {
                cairo_move_to(cr, scale * p->x, scale * p->y);
                cairo_line_to(cr, scale * p2->x, scale * p2->y);
            }
        }
    }
    cairo_stroke(cr);

    // papers
    for (int i = 0; i < map_env->cur_num_papers; i++) {
        paper_t *p = &map_env->papers[i];
        if (!paper_include(p)) {
            continue;
        }
        draw_paper(cr, map_env, p);
    }
}

// reset the forces and compute the grid
static void map_env_init_forces(map_env_t *map_env) {
    int grid_depth_overflow = 0;
    memset(map_env->grid, 0, map_env->grid_w * map_env->grid_h * map_env->grid_d * sizeof(paper_t*));
    for (int i = 0; i < map_env->cur_num_papers; i++) {
        paper_t *p = &map_env->papers[i];
        if (!paper_include(p)) {
            continue;
        }

        // reset the forces
        p->fx = 0;
        p->fy = 0;

        // fill in the grid where this paper is located
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
                    grid_depth_overflow = 1;
                }
            }
        }
    }
    if (grid_depth_overflow) {
        printf("grid depth overflow\n");
    }
}

bool map_env_forces(map_env_t *map_env, bool do_touch, bool do_attr, bool do_tred, paper_t *hold_still) {
    map_env_init_forces(map_env);

    // repulsion from touching
    for (int i = 0; i < map_env->cur_num_papers; i++) {
        paper_t *p = &map_env->papers[i];
        if (!paper_include(p)) {
            continue;
        }
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
                // tricky!
                // the papers at a given depth are sorted, smallest pointer first, and end in a NULL pointer
                // we only need to check pairs of papers (no double counting), so keep checking until our pointer p is bigger than in g
                for (; *g != NULL && p > *g; g++) {
                    paper_t *p2 = *g;
                    double dx = p->x - p2->x;
                    double dy = p->y - p2->y;
                    double r = sqrt(dx*dx + dy*dy);
                    double overlap = p->r + p2->r - r + 0.1;
                    if (overlap > 0) {
                        // p and p2 are overlapping
                        overlap = 0.5 * overlap*overlap*overlap;
                        double fx = overlap * dx / r;
                        double fy = overlap * dy / r;
                        p->fx += fx;
                        p->fy += fy;
                        p2->fx -= fx;
                        p2->fy -= fy;
                    }
                }
            }
        }
    }

    // repulsion from all others
    for (int i = 0; i < map_env->cur_num_papers; i++) {
        paper_t *p1 = &map_env->papers[i];
        if (!paper_include(p1)) {
            continue;
        }
        for (int j = i + 1; j < map_env->cur_num_papers; j++) {
            paper_t *p2 = &map_env->papers[j];
            if (!paper_include(p2)) {
                continue;
            }
            double dx = p1->x - p2->x;
            double dy = p1->y - p2->y;
            double r = sqrt(dx*dx + dy*dy);
            if (r > 1e-2) {
                //double fac = 0.5 * p1->mass * p2->mass / (r*r*r*r);
                double fac = 0.1 / (r*r*r);
                if (p1->colour != p2->colour) {
                    fac = 0.8 / (r*r*r*r);
                }
                double fx = dx * fac;
                double fy = dy * fac;
                p1->fx += fx;
                p1->fy += fy;
                p2->fx -= fx;
                p2->fy -= fy;
            }
        }
    }

    /*
    // attraction to correct y location
    for (int i = 0; i < map_env->cur_num_papers; i++) {
        paper_t *p = &map_env->papers[i];
        if (!paper_include(p)) {
            continue;
        }
        double dy = (0.02 * p->index) - p->y;
        double fac = 0.2 * p->mass;
        double fy = dy * fac;
        p->fy += fy;
    }
    */

    // attraction due to links
    if (do_attr) {
    for (int i = 0; i < map_env->cur_num_papers; i++) {
        paper_t *p1 = &map_env->papers[i];
        if (!paper_include(p1)) {
            continue;
        }
        for (int j = 0; j < p1->num_refs; j++) {
            paper_t *p2 = p1->refs[j];
            if (!paper_include(p2)) {
                continue;
            }
            if ((!do_tred || p1->refs_tred_computed[j]) && p2->index < map_env->cur_num_papers) {
                double dx = p1->x - p2->x;
                double dy = p1->y - p2->y;
                double r = sqrt(dx*dx + dy*dy);
                double overlap = p1->r + p2->r - r + 0.1;
                if (overlap < 0) {
                    double fac = 0.12;
                    double fx = dx*r * fac;
                    double fy = dy*r * fac;

                    /*
                    // rotate pairs so the earliest one is above the other
                    dy /= r;
                    dx /= r;
                    if (p1->index < p2->index) {
                        fac = -0.1 * dx;
                    } else {
                        fac = 0.1 * dx;
                    }
                    fx += fac * dy;
                    fy -= fac * dx;
                    */

                    p1->fx -= fx;
                    p1->fy -= fy;
                    p2->fx += fx;
                    p2->fy += fy;
                }
            }
        }
    }
    }

    /*
    // gravity!
    for (int i = 0; i < map_env->cur_num_papers; i++) {
        paper_t *p1 = &map_env->papers[i];
        for (int j = i + 1; j < map_env->cur_num_papers; j++) {
            paper_t *p2 = &map_env->papers[j];
            double dx = p1->x - p2->x;
            double dy = p1->y - p2->y;
            double r = sqrt(dx*dx + dy*dy);
            if (r > p1->r + p2->r) {
                double fac = 0.8 * p1->mass * p2->mass / (r*r*r);
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

    // work out maximum force
    double fmax = 0;
    for (int i = 0; i < map_env->cur_num_papers; i++) {
        paper_t *p = &map_env->papers[i];
        if (!paper_include(p)) {
            continue;
        }
        p->fx /= p->mass;
        p->fy /= p->mass;
        if (fabs(p->fx) > fmax) {
            fmax = fabs(p->fx);
        }
        if (fabs(p->fy) > fmax) {
            fmax = fabs(p->fy);
        }
    }

    if (fmax < 1e-3) {
        // we have converged
        return true;
    }

    // apply forces
    double fmult;
    if (fmax > 1) {
        fmult = 0.25 / fmax;
    } else {
        fmult = 0.25;
    }
    for (int i = 0; i < map_env->cur_num_papers; i++) {
        paper_t *p = &map_env->papers[i];
        if (!paper_include(p)) {
            continue;
        }
        if (p == hold_still) {
            continue;
        }

        p->x += fmult * p->fx;
        p->y += fmult * p->fy;

        // apply boundary conditions
        if (p->x - p->r < 0) {
            p->x = p->r;
        } else if (p->x + p->r > map_env->grid_w) {
            p->x = map_env->grid_w - p->r;
        }
        if (p->y - p->r < 0) {
            p->y = p->r;
        } else if (p->y + p->r > map_env->grid_h) {
            p->y = map_env->grid_h - p->r;
        }

        // force y-position
        //p->y = 1 + 0.05 * p->index;
    }

    return false;
}

void map_env_grow(map_env_t *map_env, double amt) {
    for (int i = 0; i < map_env->cur_num_papers; i++) {
        paper_t *p = &map_env->papers[i];
        p->mass *= amt;
        p->r = sqrt(p->mass / M_PI);
    }
}

void map_env_inc_num_papers(map_env_t *map_env, int amt) {
    int old_num_papers = map_env->cur_num_papers;
    map_env->cur_num_papers += amt;
    if (map_env->cur_num_papers > map_env->max_num_papers) {
        map_env->cur_num_papers = map_env->max_num_papers;
    }
    recompute_num_cites(map_env->cur_num_papers, map_env->papers);
    recompute_colours(map_env->cur_num_papers, map_env->papers, true);
    compute_tred(map_env->cur_num_papers, map_env->papers);
    for (int i = 0; i < map_env->cur_num_papers; i++) {
        paper_t *p = &map_env->papers[i];
        p->mass = 0.05 + 0.2 * p->num_cites;
        p->r = sqrt(p->mass / M_PI);
    }
    for (int i = old_num_papers; i < map_env->cur_num_papers; i++) {
        paper_t *p = &map_env->papers[i];
        double x = 0;
        int n = 0;
        // add x-pos of references
        for (int j = 0; j < p->num_refs; j++) {
            paper_t *p2 = p->refs[j];
            if (p2->index < map_env->cur_num_papers) {
                x += p2->x;
                n += 1;
            }
        }
        // add x-pos of any references coming from the past
        for (int j = 0; j < old_num_papers; j++) {
            paper_t *p2 = &map_env->papers[j];
            for (int k = 0; k < p2->num_refs; k++) {
                if (p2->refs[k] == p) {
                    x += p2->x;
                    n += 1;
                    break;
                }
            }
        }
        if (n > 0) {
            p->x = x / n;
        }
        //p->y = 1 + 0.01 * p->index;
        p->y = map_env->grid_h * 1.0 * random() / RAND_MAX;
    }
    int num_included = 0;
    for (int i = 0; i < map_env->cur_num_papers; i++) {
        paper_t *p = &map_env->papers[i];
        if (paper_include(p)) {
            num_included += 1;
        }
    }
    printf("now have %d papers, %d connected and included in graph, maximum id is %d\n", map_env->cur_num_papers, num_included, map_env->papers[map_env->cur_num_papers - 1].id);
}

void map_env_jolt(map_env_t *map_env, double amt) {
    for (int i = 0; i < map_env->cur_num_papers; i++) {
        paper_t *p = &map_env->papers[i];
        p->x += amt * (-0.5 + 1.0 * random() / RAND_MAX);
        p->y += amt * (-0.5 + 1.0 * random() / RAND_MAX);
    }
}

paper_t *map_env_get_paper_at(map_env_t *map_env, int mx, int my) {
    double x = mx / map_env->scale;
    double y = my / map_env->scale;
    for (int i = 0; i < map_env->cur_num_papers; i++) {
        paper_t *p = &map_env->papers[i];
        double dx = p->x - x;
        double dy = p->y - y;
        double r = dx*dx + dy*dy;
        if (r < p->r*p->r) {
            return p;
        }
    }
    return NULL;
}
