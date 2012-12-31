#include <stdlib.h>
#include <assert.h>
#include <string.h>
#include <math.h>
#include <gtk/gtk.h>

#include "xiwilib.h"
#include "common.h"
#include "quadtree.h"
#include "map.h"

static double anti_gravity_strength = 0.2;
static double link_strength = 0.03;

struct _map_env_t {
    vstr_t *vstr;

    // loaded
    int cur_num_papers;
    int max_num_papers;
    paper_t *all_papers;

    // currently in the graph
    int num_papers;
    paper_t **papers;

    quad_tree_t *quad_tree;

    int grid_w;
    int grid_h;
    int grid_d;
    paper_t **grid;

    bool draw_grid;
    bool draw_paper_links;

    cairo_matrix_t tr_matrix;
};

map_env_t *map_env_new() {
    map_env_t *map_env = m_new(map_env_t, 1);
    map_env->vstr = vstr_new();
    map_env->cur_num_papers = 0;
    map_env->max_num_papers = 0;
    map_env->all_papers = NULL;
    map_env->num_papers = 0;
    map_env->papers = NULL;
    map_env->quad_tree = m_new(quad_tree_t, 1);
    map_env->grid_w = 160;
    map_env->grid_h = 160;
    map_env->grid_d = 20;
    map_env->grid = m_new0(paper_t*, map_env->grid_w * map_env->grid_h * map_env->grid_d);

    map_env->draw_grid = false;
    map_env->draw_paper_links = true;

    cairo_matrix_init_identity(&map_env->tr_matrix);
    map_env->tr_matrix.xx = 8;
    map_env->tr_matrix.yy = 8;

    return map_env;
}

void map_env_world_to_screen(map_env_t *map_env, double *x, double *y) {
    *x = map_env->tr_matrix.xx * (*x) + map_env->tr_matrix.x0;
    *y = map_env->tr_matrix.yy * (*y) + map_env->tr_matrix.y0;
}

void map_env_screen_to_world(map_env_t *map_env, double *x, double *y) {
    *x = ((*x) - map_env->tr_matrix.x0) / map_env->tr_matrix.xx;
    *y = ((*y) - map_env->tr_matrix.y0) / map_env->tr_matrix.yy;
}

paper_t *map_env_get_paper_at(map_env_t *map_env, double x, double y) {
    map_env_screen_to_world(map_env, &x, &y);
    for (int i = 0; i < map_env->num_papers; i++) {
        paper_t *p = map_env->papers[i];
        double dx = p->x - x;
        double dy = p->y - y;
        double r = dx*dx + dy*dy;
        if (r < p->r*p->r) {
            return p;
        }
    }
    return NULL;
}

void map_env_set_papers(map_env_t *map_env, int num_papers, paper_t *papers) {
    map_env->max_num_papers = num_papers;
    map_env->all_papers = papers;
    map_env->papers = m_renew(paper_t*, map_env->papers, map_env->max_num_papers);
    for (int i = 0; i < map_env->max_num_papers; i++) {
        paper_t *p = &map_env->all_papers[i];
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
    map_env->all_papers = m_renew(paper_t, map_env->all_papers, n);
    map_env->papers = m_renew(paper_t*, map_env->papers, map_env->max_num_papers);
    for (int i = 0; i < n; i++) {
        paper_t *p = &map_env->all_papers[i];
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
    map_env->all_papers = m_renew(paper_t, map_env->all_papers, n);
    map_env->papers = m_renew(paper_t*, map_env->papers, map_env->max_num_papers);
    for (int i = 0; i < n; i++) {
        paper_t *p = &map_env->all_papers[i];
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
            p->refs[0] = &map_env->all_papers[0];
        }
    }
}

void map_env_papers_test2(map_env_t *map_env, int n) {
    // the first 2 papers are cited both by the rest
    map_env->max_num_papers = n;
    map_env->all_papers = m_renew(paper_t, map_env->all_papers, n);
    map_env->papers = m_renew(paper_t*, map_env->papers, map_env->max_num_papers);
    for (int i = 0; i < n; i++) {
        paper_t *p = &map_env->all_papers[i];
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
            p->refs[0] = &map_env->all_papers[0];
            p->refs[1] = &map_env->all_papers[1];
        }
    }
}

void map_env_scroll(map_env_t *map_env, double dx, double dy) {
    map_env->tr_matrix.x0 += dx;
    map_env->tr_matrix.y0 += dy;
}

void map_env_zoom(map_env_t *map_env, double screen_x, double screen_y, double amt) {
    map_env->tr_matrix.xx *= amt;
    map_env->tr_matrix.yy *= amt;
    map_env->tr_matrix.x0 = map_env->tr_matrix.x0 * amt + screen_x * (1.0 - amt);
    map_env->tr_matrix.y0 = map_env->tr_matrix.y0 * amt + screen_y * (1.0 - amt);
}

void map_env_toggle_draw_grid(map_env_t *map_env) {
    map_env->draw_grid = !map_env->draw_grid;
}

void map_env_toggle_draw_paper_links(map_env_t *map_env) {
    map_env->draw_paper_links = !map_env->draw_paper_links;
}

void map_env_adjust_anti_gravity(map_env_t *map_env, double amt) {
    anti_gravity_strength *= amt;
}

void map_env_adjust_link_strength(map_env_t *map_env, double amt) {
    link_strength *= amt;
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
    double x = p->x;
    double y = p->y;
    double w = p->r;
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

void quad_tree_draw_grid(cairo_t *cr, quad_tree_node_t *q, double min_x, double min_y, double max_x, double max_y) {
    if (q != NULL) {
        if (q->num_papers == 1) {
            cairo_rectangle(cr, min_x, min_y, max_x - min_x, max_y - min_y);
            cairo_fill(cr);
        } else if (q->num_papers > 1) {
            double mid_x = 0.5 * (min_x + max_x);
            double mid_y = 0.5 * (min_y + max_y);
            cairo_move_to(cr, min_x, mid_y);
            cairo_line_to(cr, max_x, mid_y);
            cairo_move_to(cr, mid_x, min_y);
            cairo_line_to(cr, mid_x, max_y);
            cairo_stroke(cr);
            quad_tree_draw_grid(cr, q->q0, min_x, min_y, mid_x, mid_y);
            quad_tree_draw_grid(cr, q->q1, mid_x, min_y, max_x, mid_y);
            quad_tree_draw_grid(cr, q->q2, min_x, mid_y, mid_x, max_y);
            quad_tree_draw_grid(cr, q->q3, mid_x, mid_y, max_x, max_y);
        }
    }
}

void map_env_draw(map_env_t *map_env, cairo_t *cr, guint width, guint height, bool do_tred) {
    double line_width_1px = 1.0 / map_env->tr_matrix.xx;
    cairo_set_matrix(cr, &map_env->tr_matrix);

    if (map_env->draw_grid) {
        cairo_set_line_width(cr, line_width_1px);
        cairo_set_source_rgba(cr, 0, 0, 0, 0.3);
        quad_tree_draw_grid(cr, map_env->quad_tree->root, map_env->quad_tree->min_x, map_env->quad_tree->min_y, map_env->quad_tree->max_x, map_env->quad_tree->max_y);

        /*
        // grid density
        for (int j = 0; j < map_env->grid_h; j++) {
            for (int i = 0; i < map_env->grid_w; i++) {
                paper_t **g = &map_env->grid[(j * map_env->grid_w + i) * map_env->grid_d];
                int d = 0;
                for (; *g != NULL; g++, d++) { }
                cairo_set_source_rgba(cr, 0, 0, 0, 1.0 * d / map_env->grid_d);
                cairo_rectangle(cr, i, j, 1, 1);
                cairo_fill(cr);
            }
        }
        */

        // grid lines
        /*
        cairo_set_line_width(cr, line_width_1px);
        cairo_set_source_rgba(cr, 0, 0, 0, 0.5);
        for (int i = 0; i <= map_env->grid_w; i++) {
            cairo_move_to(cr, i, 0);
            cairo_line_to(cr, i, map_env->grid_h);
        }
        for (int i = 0; i <= map_env->grid_h; i++) {
            cairo_move_to(cr, 0, i);
            cairo_line_to(cr, map_env->grid_w, i);
        }
        cairo_stroke(cr);
        */
    } else {
        // just draw the border of the grid
        /*
        cairo_set_line_width(cr, line_width_1px);
        cairo_set_source_rgba(cr, 0, 0, 0, 0.5);
        cairo_rectangle(cr, 0, 0, map_env->grid_w, map_env->grid_h);
        cairo_stroke(cr);
        */
    }

    // paper links
    if (map_env->draw_paper_links) {
        cairo_set_source_rgba(cr, 0, 0, 0, 0.3);
        if (do_tred) {
            for (int i = 0; i < map_env->num_papers; i++) {
                paper_t *p = map_env->papers[i];
                for (int j = 0; j < p->num_refs; j++) {
                    paper_t *p2 = p->refs[j];
                    if (p->refs_tred_computed[j] && p2->index < map_env->cur_num_papers) {
                        cairo_set_line_width(cr, 0.1 * p->refs_tred_computed[j]);
                        cairo_move_to(cr, p->x, p->y);
                        cairo_line_to(cr, p2->x, p2->y);
                        cairo_stroke(cr);
                    }
                }
            }
        } else {
            cairo_set_line_width(cr, 0.1);
            for (int i = 0; i < map_env->num_papers; i++) {
                paper_t *p = map_env->papers[i];
                for (int j = 0; j < p->num_refs; j++) {
                    paper_t *p2 = p->refs[j];
                    if ((!do_tred || p->refs_tred_computed[j]) && p2->index < map_env->cur_num_papers) {
                        cairo_move_to(cr, p->x, p->y);
                        cairo_line_to(cr, p2->x, p2->y);
                    }
                }
            }
            cairo_stroke(cr);
        }
    }

    // papers
    for (int i = 0; i < map_env->num_papers; i++) {
        paper_t *p = map_env->papers[i];
        draw_paper(cr, map_env, p);
    }

    // info

    vstr_reset(map_env->vstr);
    vstr_printf(map_env->vstr, "anti-gravity strength: %.3f\n", anti_gravity_strength);
    vstr_printf(map_env->vstr, "link strength: %.3f\n", link_strength);
    vstr_printf(map_env->vstr, "transitive reduction: %d\n", do_tred);
    vstr_printf(map_env->vstr, "have %d papers, %d connected and included in graph\n", map_env->cur_num_papers, map_env->num_papers);

    cairo_identity_matrix(cr);
    cairo_set_source_rgb(cr, 0, 0, 0);
    int y = 20;
    char *s1 = vstr_str(map_env->vstr);
    while (*s1 != '\0') {
        char *s2 = s1;
        while (*s2 != '\0' && *s2 != '\n') {
            s2 += 1;
        }
        int old_c = *s2;
        *s2 = '\0';
        cairo_move_to(cr, 10, y);
        cairo_show_text(cr, s1);
        *s2 = old_c;
        y += 12;
        s1 = s2;
        if (*s1 == '\n') {
            s1 += 1;
        }
    }
}

// reset the forces and compute the grid
static void map_env_init_forces(map_env_t *map_env) {
    int grid_depth_overflow = 0;
    memset(map_env->grid, 0, map_env->grid_w * map_env->grid_h * map_env->grid_d * sizeof(paper_t*));
    for (int i = 0; i < map_env->num_papers; i++) {
        paper_t *p = map_env->papers[i];

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

// q1 is a leaf against which we check q2
void quad_tree_node_forces2(quad_tree_node_t *q1, quad_tree_node_t *q2, double q2_cell_side_length) {
    if (q2 == NULL) {
        // q2 is empty node
    } else {
        // q2 is leaf or internal node

        // compute distance from q1 to centroid of q2
        double dx = q1->x - q2->x;
        double dy = q1->y - q2->y;
        double r = sqrt(dx * dx + dy * dy);
        if (r < 1e-3) {
            // minimum distance cut-off
            r = 1e-3;
        }

        if (q2->num_papers == 1) {
            // q2 is leaf node
            double fac = q1->mass * q2->mass * anti_gravity_strength / (r*r*r);
            double fx = dx * fac;
            double fy = dy * fac;
            q1->fx += fx;
            q1->fy += fy;
            q2->fx -= fx;
            q2->fy -= fy;

        } else {
            // q2 is internal node
            if (q2_cell_side_length / r < 0.7) {
                // q1 and the cell q2 are "well separated"
                // approximate force by centroid of q2
                double fac = q1->mass * q2->mass * anti_gravity_strength / (r*r*r);
                double fx = dx * fac;
                double fy = dy * fac;
                q1->fx += fx;
                q1->fy += fy;
                q2->fx -= fx;
                q2->fy -= fy;

            } else {
                // q1 and q2 are not "well separated"
                // descend into children of q2
                q2_cell_side_length *= 0.5;
                quad_tree_node_forces2(q1, q2->q0, q2_cell_side_length);
                quad_tree_node_forces2(q1, q2->q1, q2_cell_side_length);
                quad_tree_node_forces2(q1, q2->q2, q2_cell_side_length);
                quad_tree_node_forces2(q1, q2->q3, q2_cell_side_length);
            }
        }
    }
}

void quad_tree_node_forces1(quad_tree_node_t *q, double cell_side_length) {
    assert(q->num_papers == 1); // must be a leaf node
    for (quad_tree_node_t *q2 = q; q2->parent != NULL; q2 = q2->parent) {
        quad_tree_node_t *parent = q2->parent;
        assert(parent->num_papers > 1); // all parents should be internal nodes
        if (parent->q0 != q2) { quad_tree_node_forces2(q, parent->q0, cell_side_length); }
        if (parent->q1 != q2) { quad_tree_node_forces2(q, parent->q1, cell_side_length); }
        if (parent->q2 != q2) { quad_tree_node_forces2(q, parent->q2, cell_side_length); }
        if (parent->q3 != q2) { quad_tree_node_forces2(q, parent->q3, cell_side_length); }
        cell_side_length *= 2;
    }
}

void quad_tree_node_forces0(quad_tree_node_t *q, double cell_side_length) {
    if (q == NULL) {
    } else if (q->num_papers == 1) {
        quad_tree_node_forces1(q, cell_side_length);
    } else {
        cell_side_length *= 0.5;
        quad_tree_node_forces0(q->q0, cell_side_length);
        quad_tree_node_forces0(q->q1, cell_side_length);
        quad_tree_node_forces0(q->q2, cell_side_length);
        quad_tree_node_forces0(q->q3, cell_side_length);
    }
}

void quad_tree_node_forces_propagate(quad_tree_node_t *q, double fx, double fy) {
    if (q == NULL) {
    } else {
        fx *= q->mass;
        fy *= q->mass;
        fx += q->fx;
        fy += q->fy;

        if (q->num_papers == 1) {
            q->paper->fx += fx;
            q->paper->fy += fy;
        } else {
            fx /= q->mass;
            fy /= q->mass;
            quad_tree_node_forces_propagate(q->q0, fx, fy);
            quad_tree_node_forces_propagate(q->q1, fx, fy);
            quad_tree_node_forces_propagate(q->q2, fx, fy);
            quad_tree_node_forces_propagate(q->q3, fx, fy);
        }
    }
}

void quad_tree_forces(quad_tree_t *qt) {
    double cell_side_length = 0.5 * ((qt->max_x - qt->min_x) + (qt->max_y - qt->min_y));
    quad_tree_node_forces0(qt->root, cell_side_length);
    quad_tree_node_forces_propagate(qt->root, 0, 0);
}

bool map_env_forces(map_env_t *map_env, bool do_touch, bool do_attr, bool do_tred, paper_t *hold_still) {
    for (int i = 0; i < map_env->num_papers; i++) {
        paper_t *p = map_env->papers[i];

        // reset the forces
        p->fx = 0;
        p->fy = 0;
    }

    //map_env_init_forces(map_env);

    quad_tree_build(map_env->num_papers, map_env->papers, map_env->quad_tree);
    quad_tree_forces(map_env->quad_tree);

    #if 0
    // repulsion from touching
    for (int i = 0; i < map_env->num_papers; i++) {
        paper_t *p = map_env->papers[i];
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
                    if (r > 0 && overlap > 0) {
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
    if (0 && do_attr) {
    for (int i = 0; i < map_env->num_papers; i++) {
        paper_t *p1 = map_env->papers[i];
        for (int j = i + 1; j < map_env->num_papers; j++) {
            paper_t *p2 = map_env->papers[j];
            double dx = p1->x - p2->x;
            double dy = p1->y - p2->y;
            double r = sqrt(dx*dx + dy*dy);
            if (r > 1e-2) {
                //double fac = 0.5 * p1->mass * p2->mass / (r*r*r*r);
                double fac = anti_gravity_strength / (r*r*r);
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
    }
    #endif

    /*
    // attraction to correct y location
    for (int i = 0; i < map_env->num_papers; i++) {
        paper_t *p = map_env->papers[i];
        double dy = (0.02 * p->index) - p->y;
        double fac = 0.2 * p->mass;
        double fy = dy * fac;
        p->fy += fy;
    }
    */

    // attraction due to links
    if (do_attr) {
    for (int i = 0; i < map_env->num_papers; i++) {
        paper_t *p1 = map_env->papers[i];
        for (int j = 0; j < p1->num_refs; j++) {
            paper_t *p2 = p1->refs[j];
            if ((!do_tred || p1->refs_tred_computed[j]) && p2->index < map_env->cur_num_papers) {
                double dx = p1->x - p2->x;
                double dy = p1->y - p2->y;
                double r = sqrt(dx*dx + dy*dy);
                double overlap = p1->r + p2->r - r + 0.1;
                if (overlap < 0) {
                    double fac = 2.4 * link_strength;

                    if (do_tred) {
                        fac = link_strength * p1->refs_tred_computed[j];
                        //fac *= p1->refs_tred_computed[j];
                    }

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
    for (int i = 0; i < map_env->num_papers; i++) {
        paper_t *p1 = map_env->papers[i];
        for (int j = i + 1; j < map_env->num_papers; j++) {
            paper_t *p2 = map_env->papers[j];
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
    for (int i = 0; i < map_env->num_papers; i++) {
        paper_t *p = map_env->papers[i];
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
    if (fmax > 2) {
        fmult = 1.0 / fmax;
    } else {
        fmult = 0.5;
    }
    for (int i = 0; i < map_env->num_papers; i++) {
        paper_t *p = map_env->papers[i];
        if (p == hold_still) {
            continue;
        }

        p->x += fmult * p->fx;
        p->y += fmult * p->fy;

        // apply boundary conditions
        /*
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
        */

        // force y-position
        //p->y = 1 + 0.05 * p->index;
    }

    return false;
}

void map_env_grow(map_env_t *map_env, double amt) {
    for (int i = 0; i < map_env->num_papers; i++) {
        paper_t *p = map_env->papers[i];
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
    recompute_num_cites(map_env->cur_num_papers, map_env->all_papers);
    recompute_colours(map_env->cur_num_papers, map_env->all_papers, true);
    compute_tred(map_env->cur_num_papers, map_env->all_papers);
    for (int i = 0; i < map_env->cur_num_papers; i++) {
        paper_t *p = &map_env->all_papers[i];
        p->mass = 0.05 + 0.2 * p->num_cites;
        p->r = sqrt(p->mass / M_PI);
    }
    // compute initial position for newly added papers (average of all its references)
    for (int i = old_num_papers; i < map_env->cur_num_papers; i++) {
        paper_t *p = &map_env->all_papers[i];
        double x = 0;
        double y = 0;
        int n = 0;
        // average x- and y-pos of references
        for (int j = 0; j < p->num_refs; j++) {
            paper_t *p2 = p->refs[j];
            if (p2->index < map_env->cur_num_papers) {
                x += p2->x;
                y += p2->y;
                n += 1;
            }
        }
        if (n == 0) {
            p->x = map_env->grid_w * 1.0 * random() / RAND_MAX;
            p->y = map_env->grid_h * 1.0 * random() / RAND_MAX;
        } else {
            // add some random element to average, mainly so we don't put it at the same pos for n=1
            p->x = x / n + 1.0 * random() / RAND_MAX;
            p->y = y / n + 1.0 * random() / RAND_MAX;
        }
    }

    // make array of papers that we want to include (exclude non-connected papers)
    map_env->num_papers = 0;
    for (int i = 0; i < map_env->cur_num_papers; i++) {
        paper_t *p = &map_env->all_papers[i];
        if (p->num_with_my_colour > 100) {
            map_env->papers[map_env->num_papers++] = p;
        }
    }

    printf("now have %d papers, %d connected and included in graph, maximum id is %d\n", map_env->cur_num_papers, map_env->num_papers, map_env->all_papers[map_env->cur_num_papers - 1].id);
}

void map_env_jolt(map_env_t *map_env, double amt) {
    for (int i = 0; i < map_env->num_papers; i++) {
        paper_t *p = map_env->papers[i];
        p->x += amt * (-0.5 + 1.0 * random() / RAND_MAX);
        p->y += amt * (-0.5 + 1.0 * random() / RAND_MAX);
    }
}
