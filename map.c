#include <stdlib.h>
#include <assert.h>
#include <string.h>
#include <math.h>
#include <gtk/gtk.h>

#include "xiwilib.h"
#include "common.h"
#include "quadtree.h"
#include "map.h"

static double anti_gravity_strength = 0.4;
static double link_strength = 0.03;

struct _map_env_t {
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

    bool do_tred;
    bool draw_grid;
    bool draw_paper_links;

    cairo_matrix_t tr_matrix;

    double energy;
    int progress;
    double step_size;
};

map_env_t *map_env_new() {
    map_env_t *map_env = m_new(map_env_t, 1);
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

    map_env->do_tred = false;
    map_env->draw_grid = false;
    map_env->draw_paper_links = false;

    cairo_matrix_init_identity(&map_env->tr_matrix);
    map_env->tr_matrix.xx = 8;
    map_env->tr_matrix.yy = 8;

    map_env->energy = 0;
    map_env->progress = 0;
    map_env->step_size = 0.1;

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

void map_env_toggle_do_tred(map_env_t *map_env) {
    map_env->do_tred = !map_env->do_tred;
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

void draw_paper_bg(cairo_t *cr, map_env_t *map_env, paper_t *p) {
    double x = p->x;
    double y = p->y;
    double w = 2*p->r;
    if (p->kind == 1) {
        cairo_set_source_rgba(cr, 0.85, 0.85, 1, 1);
    } else if (p->kind == 2) {
        cairo_set_source_rgba(cr, 0.85, 1, 0.85, 1);
    } else if (p->kind == 3) {
        cairo_set_source_rgba(cr, 1, 1, 0.85, 1);
    } else if (p->kind == 4) {
        cairo_set_source_rgba(cr, 0.85, 1, 1, 1);
    } else {
        cairo_set_source_rgba(cr, 1, 0.85, 1, 1);
    }
    cairo_arc(cr, x, y, w, 0, 2 * M_PI);
    cairo_fill(cr);
}

void draw_paper(cairo_t *cr, map_env_t *map_env, paper_t *p, double age) {
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
    /*
    if (p->id == 1992546899 || p->id == 1993234723) {
        cairo_set_source_rgba(cr, 0.8, 0.8, 0, 0.7);
    } else if (p->kind == 1) {
        cairo_set_source_rgba(cr, 0, 0, 0.8, 0.7);
    } else if (p->kind == 2) {
        cairo_set_source_rgba(cr, 0.8, 0, 0, 0.7);
    } else {
        cairo_set_source_rgba(cr, 0, 0.8, 0, 0.7);
    }
    */

    // basic colour of paper
    double r, g, b;
    if (p->kind == 1) {
        r = 0;
        g = 0;
        b = 1;
    } else if (p->kind == 2) {
        r = 0;
        g = 1;
        b = 0;
    } else if (p->kind == 3) {
        r = 1;
        g = 1;
        b = 0;
    } else if (p->kind == 4) {
        r = 0;
        g = 1;
        b = 1;
    } else {
        r = 1;
        g = 0;
        b = 1;
    }

    // older papers are more saturated in colour
    double saturation = 0.6 * (1 - age);

    // compute and set final colour; newer papers tend towards red
    age = age * age;
    r = saturation + (r * (1 - age) + age) * (1 - saturation);
    g = saturation + (g * (1 - age)      ) * (1 - saturation);
    b = saturation + (b * (1 - age)      ) * (1 - saturation);
    cairo_set_source_rgb(cr, r, g, b);

    cairo_arc(cr, x, y, w, 0, 2 * M_PI);
    cairo_fill(cr);
}

void draw_paper_text(cairo_t *cr, map_env_t *map_env, paper_t *p) {
    if (p->r * map_env->tr_matrix.xx > 20) {
        double x = p->x;
        double y = p->y;
        map_env_world_to_screen(map_env, &x, &y);
        cairo_text_extents_t extents;
        cairo_text_extents(cr, p->title, &extents);
        cairo_move_to(cr, x - 0.5 * extents.width, y + 0.5 * extents.height);
        cairo_show_text(cr, p->title);
    }
}

void draw_big_labels(cairo_t *cr, map_env_t *map_env) {
    for (int i = 0; i < map_env->num_papers; i++) {
        paper_t *p = map_env->papers[i];
        const char *str = NULL;
             if (p->id == 2071594354) { str = "unparticles"; }
        else if (p->id == 2076328973) { str = "M2-branes"; }
        else if (p->id == 2070391225) { str = "black hole mergers"; }
        else if (p->id == 2082673143) { str = "f(R) gravity"; }
        else if (p->id == 2085375036) { str = "Kerr/CFT"; }
        else if (p->id == 2090390629) { str = "Horava-Lifshitz"; }
        else if (p->id == 2100078229) { str = "entropic gravity"; }
        else if (p->id == 2110390945) { str = "TMD PDFs"; }
        else if (p->id == 2113360267) { str = "massive gravity"; }
        else if (p->id == 2115329009) { str = "superluminal neutrinos"; }
        else if (p->id == 2123937504) { str = "firewalls"; }
        else if (p->id == 2124219058) { str = "Higgs"; }
        //else if (p->id == ) { str = ""; }
        if (str != NULL) {
            double x = p->x;
            double y = p->y;
            map_env_world_to_screen(map_env, &x, &y);
            cairo_text_extents_t extents;
            cairo_text_extents(cr, str, &extents);
            cairo_move_to(cr, x - 0.5 * extents.width, y + 0.5 * extents.height);
            cairo_show_text(cr, str);
        }
    }
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

void map_env_draw(map_env_t *map_env, cairo_t *cr, guint width, guint height, vstr_t* vstr_info) {
    // clear bg
    cairo_set_source_rgb(cr, 1, 1, 1);
    cairo_rectangle(cr, 0, 0, width, height);
    cairo_fill(cr);

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
        if (map_env->do_tred) {
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
                    if (p2->index < map_env->cur_num_papers) {
                        cairo_move_to(cr, p->x, p->y);
                        cairo_line_to(cr, p2->x, p2->y);
                        cairo_stroke(cr);
                    }
                }
            }
        }
    }

    // papers background halo
    for (int i = 0; i < map_env->num_papers; i++) {
        paper_t *p = map_env->papers[i];
        draw_paper_bg(cr, map_env, p);
    }

    // papers
    for (int i = 0; i < map_env->num_papers; i++) {
        paper_t *p = map_env->papers[i];
        draw_paper(cr, map_env, p, 1.0 * i / map_env->num_papers);
    }

    // paper text
    cairo_identity_matrix(cr);
    cairo_set_source_rgb(cr, 0, 0, 0);
    cairo_set_font_size(cr, 10);
    for (int i = 0; i < map_env->num_papers; i++) {
        paper_t *p = map_env->papers[i];
        draw_paper_text(cr, map_env, p);
    }

    // big labels
    cairo_set_source_rgb(cr, 0, 0, 0);
    cairo_set_font_size(cr, 16);
    draw_big_labels(cr, map_env);

    // create info string to return
    vstr_printf(vstr_info, "have %d papers, %d connected and included in graph\n", map_env->cur_num_papers, map_env->num_papers);
    if (map_env->num_papers > 0) {
        int id0 = map_env->papers[0]->id;
        int id1 = map_env->papers[map_env->num_papers - 1]->id;
        int y0 = id0 / 10000000 + 1800;
        int m0 = ((id0 % 10000000) / 625000) + 1;
        int d0 = ((id0 % 625000) / 15625) + 1;
        int y1 = id1 / 10000000 + 1800;
        int m1 = ((id1 % 10000000) / 625000) + 1;
        int d1 = ((id1 % 625000) / 15625) + 1;
        vstr_printf(vstr_info, "date range is %d/%d/%d -- %d/%d/%d\n", d0, m0, y0, d1, m1, y1);
    }
    vstr_printf(vstr_info, "energy: %.3g\n", map_env->energy);
    vstr_printf(vstr_info, "step size: %.3g\n", map_env->step_size);
    vstr_printf(vstr_info, "anti-gravity strength: %.3f\n", anti_gravity_strength);
    vstr_printf(vstr_info, "link strength: %.3f\n", link_strength);
    vstr_printf(vstr_info, "transitive reduction: %d\n", map_env->do_tred);
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
            double fac = q1->mass * q2->mass * anti_gravity_strength / (r*r);
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
                double fac = q1->mass * q2->mass * anti_gravity_strength / (r*r);
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

void map_env_compute_forces(map_env_t *map_env) {
    for (int i = 0; i < map_env->num_papers; i++) {
        paper_t *p = map_env->papers[i];

        // reset the forces
        p->fx = 0;
        p->fy = 0;
    }

    // compute paper-paper forces using a quad tree
    quad_tree_build(map_env->num_papers, map_env->papers, map_env->quad_tree);
    quad_tree_forces(map_env->quad_tree);

    /*
    // naive gravity/anti-gravity
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

    #if 0
    // repulsion from touching
    map_env_init_forces(map_env);
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
    #endif

    /*
    // attraction to correct y location
    for (int i = 0; i < map_env->num_papers; i++) {
        paper_t *p = map_env->papers[i];
        double dy = (0.02 * p->index) - p->y;
        double fac = 9.5 * p->mass;
        double fy = dy * fac;
        p->fy += fy;
    }
    */

    // attraction due to links
    for (int i = 0; i < map_env->num_papers; i++) {
        paper_t *p1 = map_env->papers[i];
        for (int j = 0; j < p1->num_refs; j++) {
            paper_t *p2 = p1->refs[j];
            if ((!map_env->do_tred || p1->refs_tred_computed[j]) && p2->index < map_env->cur_num_papers) {
                double dx = p1->x - p2->x;
                double dy = p1->y - p2->y;
                double r = sqrt(dx*dx + dy*dy);
                double rest_len = 1.1 * (p1->r + p2->r);

                double fac = 2.4 * link_strength;

                if (map_env->do_tred) {
                    fac = link_strength * p1->refs_tred_computed[j];
                    //fac *= p1->refs_tred_computed[j];
                }

                if (r > 1e-2) {
                    fac *= (r - rest_len) * fabs(r - rest_len) / r;
                    double fx = dx * fac;
                    double fy = dy * fac;

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

bool map_env_iterate(map_env_t *map_env, paper_t *hold_still) {
    map_env_compute_forces(map_env);

    // use the computed forces to update the (x,y) positions of the papers
    double energy = 0;
    for (int i = 0; i < map_env->num_papers; i++) {
        paper_t *p = map_env->papers[i];
        if (p == hold_still) {
            continue;
        }

        p->fx /= p->mass;
        p->fy /= p->mass;

        double fmagsq = p->fx * p->fx + p->fy * p->fy;
        energy += fmagsq;

        double dt = map_env->step_size / sqrt(fmagsq);

        p->x += dt * p->fx;
        p->y += dt * p->fy;
    }

    // adjust the step size
    if (energy < map_env->energy) {
        // energy went down
        if (map_env->progress < 3) {
            map_env->progress += 1;
        } else {
            if (map_env->step_size < 5) {
                map_env->step_size *= 1.1;
            }
        }
    } else {
        // energy went up
        map_env->progress = 0;
        if (map_env->step_size > 1e-1) {
            map_env->step_size *= 0.9;
        }
    }
    map_env->energy = energy;

    return map_env->step_size <= 1e-1;

    #if 0
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
    #endif
}

void map_env_grow(map_env_t *map_env, double amt) {
    for (int i = 0; i < map_env->num_papers; i++) {
        paper_t *p = map_env->papers[i];
        p->mass *= amt;
        p->r = sqrt(p->mass / M_PI);
    }
}

void map_env_inc_num_papers(map_env_t *map_env, int amt) {
    if (map_env->cur_num_papers >= map_env->max_num_papers) {
        // already have maximum number of papers in graph
        return;
    }
    int old_num_papers = map_env->cur_num_papers;
    map_env->cur_num_papers += amt;
    if (map_env->cur_num_papers > map_env->max_num_papers) {
        map_env->cur_num_papers = map_env->max_num_papers;
    }
    recompute_num_cites(map_env->cur_num_papers, map_env->all_papers);
    recompute_colours(map_env->cur_num_papers, map_env->all_papers, false);
    //compute_tred(map_env->cur_num_papers, map_env->all_papers);
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
        if (p->num_with_my_colour > 10) {
            map_env->papers[map_env->num_papers++] = p;
        }
    }

    if (amt > 10) {
        map_env->step_size = 1;
    }

    //printf("now have %d papers, %d connected and included in graph, maximum id is %d\n", map_env->cur_num_papers, map_env->num_papers, map_env->all_papers[map_env->cur_num_papers - 1].id);
}

void map_env_jolt(map_env_t *map_env, double amt) {
    for (int i = 0; i < map_env->num_papers; i++) {
        paper_t *p = map_env->papers[i];
        p->x += amt * (-0.5 + 1.0 * random() / RAND_MAX);
        p->y += amt * (-0.5 + 1.0 * random() / RAND_MAX);
    }
}

void map_env_rotate_all(map_env_t *map_env, double angle) {
    double s_angle = sin(angle);
    double c_angle = cos(angle);
    for (int i = 0; i < map_env->num_papers; i++) {
        paper_t *p = map_env->papers[i];
        double x = p->x;
        double y = p->y;
        p->x = c_angle * x - s_angle * y;
        p->y = s_angle * x + c_angle * y;
    }
}
