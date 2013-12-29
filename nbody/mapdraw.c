// in here is everything to do with drawing map_env to a cairo canvas

#include <stdlib.h>
#include <math.h>
#include <cairo.h>

#include "xiwilib.h"
#include "Common.h"
#include "layout.h"
#include "force.h"
#include "quadtree.h"
#include "map.h"
#include "mapdraw.h"
#include "mapprivate.h"

static void paper_colour(Common_paper_t *p, double *r, double *g, double *b) {
    Common_category_t c = p->allcats[0];
    if (c == CAT_hep_th) { *r = 0.0; *g = 0.0; *b = 1.0; } // blue
    else if (c == CAT_hep_ph) { *r = 0.0; *g = 1.0; *b = 0.0; } // green
    else if (c == CAT_hep_ex) { *r = 1.0; *g = 1.0; *b = 0.0; } // yellow
    else if (c == CAT_gr_qc || c == CAT_INSPIRE) { *r = 0.0; *g = 1.0; *b = 1.0; } // cyan
    else if (c == CAT_astro_ph_GA) { *r = 1.0; *g = 0.0; *b = 1.0; } // purple
    else if (c == CAT_hep_lat) { *r = 0.70; *g = 0.36; *b = 0.20; } // tan brown
    else if (c == CAT_astro_ph_HE) { *r = 0.62; *g = 0.86; *b = 0.24; } // lime green
    else if (CAT_astro_ph <= c && c <= CAT_astro_ph_SR) { *r = 0.89; *g = 0.53; *b = 0.60; } // skin pink
    else if (c == CAT_cond_mat) { *r = 0.6; *g = 0.4; *b = 0.4; }
    else if (c == CAT_quant_ph) { *r = 0.4; *g = 0.7; *b = 0.7; }
    else if (CAT_physics_acc_ph <= c && c <= CAT_physics_space_ph) { *r = 0.0; *g = 0.5; *b = 0.0; } // dark green
    else { *r = 0.7; *g = 1.0; *b = 0.3; }
}

/* unused function
static void draw_paper_bg(cairo_t *cr, map_env_t *map_env, Common_paper_t *p) {
    layout_node_t *l = p->layout_node;
    double x = l->x;
    double y = l->y;
    double w = 2*p->r;
    double r, g, b;
    paper_colour(p, &r, &g, &b);
    cairo_set_source_rgba(cr, 0.75 + 0.349 * r, 0.75 + 0.349 * g, 0.75 + 0.349 * b, 1);
    //cairo_rectangle(cr, x - 2*w, y - w, 4*w, 2*w);
    cairo_arc(cr, x, y, w, 0, 2 * M_PI);
    cairo_fill(cr);
}
*/

static void draw_paper(cairo_t *cr, map_env_t *map_env, Common_paper_t *p) {
    /*
    double h = w * 1.41;
    cairo_set_source_rgba(cr, 0.9, 0.9, 0.8, 0.9);
    cairo_rectangle(cr, x-0.5*w, y-0.5*h, w, h);
    cairo_fill(cr);
    cairo_set_source_rgba(cr, 0, 0, 0, 0.5);
    cairo_rectangle(cr, x-0.5*w, y-0.5*h, w, h);
    cairo_stroke(cr);
    */
    layout_node_t *l = p->layout_node;
    double x = l->x;
    double y = l->y;
    double w = p->radius;
    double age = p->age;
    /*
    if (p->id == 1992546899 || p->id == 1993234723) {
        cairo_set_source_rgba(cr, 0.8, 0.8, 0, 0.7);
    } else if (p->allcats[0] == 1) {
        cairo_set_source_rgba(cr, 0, 0, 0.8, 0.7);
    } else if (p->allcats[0] == 2) {
        cairo_set_source_rgba(cr, 0.8, 0, 0, 0.7);
    } else {
        cairo_set_source_rgba(cr, 0, 0.8, 0, 0.7);
    }
    */

    // basic colour of paper
    double r, g, b;
    paper_colour(p, &r, &g, &b);

    // older papers are more saturated in colour
    double saturation = 0.6 * (1 - age);

    // compute and set final colour; newer papers tend towards red
    age = age * age * age * age;
    r = saturation + (r * (1 - age) + age) * (1 - saturation);
    g = saturation + (g * (1 - age)      ) * (1 - saturation);
    b = saturation + (b * (1 - age)      ) * (1 - saturation);
    cairo_set_source_rgb(cr, r, g, b);

    cairo_arc(cr, x, y, w, 0, 2 * M_PI);
    cairo_fill(cr);
}

static void draw_paper_text(cairo_t *cr, map_env_t *map_env, Common_paper_t *p) {
    if (p->title != NULL && p->radius * map_env->tr_scale > 20) {
        double x = p->layout_node->x;
        double y = p->layout_node->y;
        map_env_world_to_screen(map_env, &x, &y);
        cairo_text_extents_t extents;
        cairo_text_extents(cr, p->title, &extents);
        cairo_move_to(cr, x - 0.5 * extents.width, y + 0.5 * extents.height);
        cairo_show_text(cr, p->title);
    }
}

static void draw_big_labels(cairo_t *cr, map_env_t *map_env) {
    for (int i = 0; i < map_env->num_papers; i++) {
        Common_paper_t *p = map_env->papers[i];
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
        else if (p->id == 2127218782) { str = "amplitudes"; }
        //else if (p->id == ) { str = ""; }
        if (str != NULL) {
            double x = p->layout_node->x;
            double y = p->layout_node->y;
            map_env_world_to_screen(map_env, &x, &y);
            cairo_text_extents_t extents;
            cairo_text_extents(cr, str, &extents);
            cairo_move_to(cr, x - 0.5 * extents.width, y + 0.5 * extents.height);
            cairo_show_text(cr, str);
        }
    }
}

static void draw_category_labels(cairo_t *cr, map_env_t *map_env) {
    for (int i = 0; i < CAT_NUMBER_OF; i++) {
        category_info_t *cat = &map_env->category_info[i];
        if (cat->num > 0) {
            const char *str = Common_category_enum_to_str(i);
            double x = cat->x;
            double y = cat->y;
            map_env_world_to_screen(map_env, &x, &y);
            cairo_text_extents_t extents;
            cairo_text_extents(cr, str, &extents);
            cairo_move_to(cr, x - 0.5 * extents.width, y + 0.5 * extents.height);
            cairo_show_text(cr, str);
        }
    }
}

static void quad_tree_draw_grid(cairo_t *cr, quad_tree_node_t *q, double min_x, double min_y, double max_x, double max_y) {
    if (q != NULL) {
        if (q->num_items == 1) {
            cairo_rectangle(cr, min_x, min_y, max_x - min_x, max_y - min_y);
            cairo_fill(cr);
        } else if (q->num_items > 1) {
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

static int paper_cmp_id(const void *in1, const void *in2) {
    Common_paper_t *p1 = *(Common_paper_t **)in1;
    Common_paper_t *p2 = *(Common_paper_t **)in2;
    return p1->id - p2->id;
}

static int paper_cmp_radius(const void *in1, const void *in2) {
    Common_paper_t *p1 = *(Common_paper_t **)in1;
    Common_paper_t *p2 = *(Common_paper_t **)in2;
    if (p1->radius < p2->radius) {
        return -1;
    } else if (p1->radius > p2->radius) {
        return 1;
    } else {
        return 0;
    }
}

static void map_env_draw_all(map_env_t *map_env, cairo_t *cr, int width, int height) {
    // clear bg
    //cairo_set_source_rgb(cr, 0.133, 0.267, 0.4);
    cairo_set_source_rgb(cr, 0, 0, 0);
    cairo_rectangle(cr, 0, 0, width, height);
    cairo_fill(cr);

    double line_width_1px = 1.0 / map_env->tr_scale;
    cairo_matrix_t tr_matrix;
    cairo_matrix_init_identity(&tr_matrix);
    tr_matrix.xx = map_env->tr_scale;
    tr_matrix.yy = map_env->tr_scale;
    tr_matrix.x0 = map_env->tr_x0;
    tr_matrix.y0 = map_env->tr_y0;
    cairo_set_matrix(cr, &tr_matrix);
    cairo_translate(cr, 0.5 * width / map_env->tr_scale, 0.5 * height / map_env->tr_scale);

    if (map_env->draw_grid) {
        // the origin/axis
        cairo_set_line_width(cr, line_width_1px);
        cairo_set_source_rgba(cr, 0, 0, 0, 1);
        cairo_move_to(cr, 0, -100);
        cairo_line_to(cr, 0, 100);
        cairo_stroke(cr);
        cairo_move_to(cr, -100, 0);
        cairo_line_to(cr, 100, 0);
        cairo_stroke(cr);

        // the quad tree grid
        cairo_set_line_width(cr, line_width_1px);
        cairo_set_source_rgba(cr, 0, 0, 0, 0.3);
        quad_tree_draw_grid(cr, map_env->quad_tree->root, map_env->quad_tree->min_x, map_env->quad_tree->min_y, map_env->quad_tree->max_x, map_env->quad_tree->max_y);
    }

    // links
    if (map_env->draw_paper_links) {
        cairo_set_source_rgba(cr, 0, 0, 0, 0.3);
        layout_t *l = map_env->layout;
        if (map_env->do_tred) {
#ifdef ENABLE_TRED
            for (int i = 0; i < map_env->num_papers; i++) {
                Common_paper_t *p = map_env->papers[i];
                for (int j = 0; j < p->num_refs; j++) {
                    Common_paper_t *p2 = p->refs[j];
                    if (p->refs_tred_computed[j] && p2->included) {
                        cairo_set_line_width(cr, 0.1 * p->refs_tred_computed[j]);
                        cairo_move_to(cr, p->layout_node->x, p->layout_node->y);
                        cairo_line_to(cr, p2->layout_node->x, p2->layout_node->y);
                        cairo_stroke(cr);
                    }
                }
            }
#endif
        } else {
            for (int i = 0; i < l->num_nodes; i++) {
                layout_node_t *n = &l->nodes[i];
                for (int j = 0; j < n->num_links; j++) {
                    layout_link_t *n2 = &n->links[j];
                    cairo_move_to(cr, n->x, n->y);
                    cairo_line_to(cr, n2->node->x, n2->node->y);
                    cairo_set_line_width(cr, 0.1 * n2->weight);
                    cairo_stroke(cr);
                }
            }
        }
    }

    // nodes
    cairo_set_line_width(cr, line_width_1px);
    if (map_env->layout->child_layout == NULL) {
        // at the finest layout, so draw individual papers

        // sort the papers array by radius, smallest first
        qsort(map_env->papers, map_env->num_papers, sizeof(Common_paper_t*), paper_cmp_radius);

        // papers background halo (smallest first, so big ones take over the bg)
        /*
        for (int i = 0; i < map_env->num_papers; i++) {
            Common_paper_t *p = map_env->papers[i];
            draw_paper_bg(cr, map_env, p);
        }
        */

        // papers (biggest first, so small ones are drawn over the top)
        for (int i = map_env->num_papers - 1; i >= 0; i--) {
            Common_paper_t *p = map_env->papers[i];
            draw_paper(cr, map_env, p);
        }

        // sort the papers array by id, to put it back the way it was
        qsort(map_env->papers, map_env->num_papers, sizeof(Common_paper_t*), paper_cmp_id);
    } else {
        // draw the layout-nodes
        for (int i = 0; i < map_env->layout->num_nodes; i++) {
            layout_node_t *n = &map_env->layout->nodes[i];
            cairo_set_source_rgb(cr, 0.7, 0.7, 0.5);
            cairo_arc(cr, n->x, n->y, n->radius, 0, 2 * M_PI);
            if (n->radius * map_env->tr_scale < 10) {
                cairo_fill(cr);
            } else {
                cairo_fill_preserve(cr);
                cairo_set_source_rgb(cr, 0.0, 0.0, 0.0);
                cairo_stroke(cr);
            }
        }
    }

    // set transform for drawing text
    cairo_identity_matrix(cr);
    cairo_translate(cr, 0.5 * width, 0.5 * height);

    if (map_env->layout->child_layout == NULL) {
        // paper text
        cairo_set_source_rgb(cr, 0, 0, 0);
        cairo_set_font_size(cr, 10);
        for (int i = 0; i < map_env->num_papers; i++) {
            Common_paper_t *p = map_env->papers[i];
            draw_paper_text(cr, map_env, p);
        }
    }

    // big labels
    cairo_set_source_rgb(cr, 0, 0, 0);
    cairo_set_font_size(cr, 16);
    draw_big_labels(cr, map_env);

    // category labels
    draw_category_labels(cr, map_env);
}

void map_env_draw(map_env_t *map_env, cairo_t *cr, int width, int height, vstr_t* vstr_info) {
    //layout_propagate_positions_to_children(map_env->layout); this is now done each force iteration

    map_env_draw_all(map_env, cr, width, height);

    // create info string to return
    if (vstr_info != NULL) {
        vstr_printf(vstr_info, "have %d layout nodes in graph; %d finer levels, %d coarser levels\n", map_env->layout->num_nodes, map_env_number_of_finer_layouts(map_env), map_env_number_of_coarser_layouts(map_env));
        vstr_printf(vstr_info, "have %d papers connected and included in graph\n", map_env->num_papers);
        if (map_env->num_papers > 0) {
            int id0 = map_env->papers[0]->id;
            int id1 = map_env->papers[map_env->num_papers - 1]->id;
            int y0, m0, d0;
            int y1, m1, d1;
            Common_unique_id_to_date(id0, &y0, &m0, &d0);
            Common_unique_id_to_date(id1, &y1, &m1, &d1);
            vstr_printf(vstr_info, "date range is %d/%d/%d -- %d/%d/%d\n", d0, m0, y0, d1, m1, y1);
        }
        vstr_printf(vstr_info, "\n");
        vstr_printf(vstr_info, "graph size: %u x %u\n", (int)(map_env->quad_tree->max_x - map_env->quad_tree->min_x), (int)(map_env->quad_tree->max_y - map_env->quad_tree->min_y));
        vstr_printf(vstr_info, "zoom factor: %.3g\n", map_env->tr_scale);
        vstr_printf(vstr_info, "energy: %.3g\n", map_env->energy);
        vstr_printf(vstr_info, "step size: %.3g\n", map_env->step_size);
        vstr_printf(vstr_info, "max link force: %.2g\n", map_env->max_link_force_mag);
        vstr_printf(vstr_info, "max total force: %.2g\n", map_env->max_total_force_mag);
        vstr_printf(vstr_info, "\n");
        vstr_printf(vstr_info, "use ref freq: %d\n", map_env->force_params.use_ref_freq);
#ifdef ENABLE_TRED
        vstr_printf(vstr_info, "transitive reduction: %d\n", map_env->do_tred);
#endif
        vstr_printf(vstr_info, "\n");
        vstr_printf(vstr_info, "(r) do close repulsion: %d\n", map_env->force_params.do_close_repulsion);
        vstr_printf(vstr_info, "(1/!) anti-gravity r*^2: %.3g\n", map_env->force_params.anti_gravity_falloff_rsq);
        vstr_printf(vstr_info, "(2/@) link strength: %.3f\n", map_env->force_params.link_strength);
        vstr_printf(vstr_info, "(3/#) close repulsion A: %.3g\n", map_env->force_params.close_repulsion_a);
        vstr_printf(vstr_info, "(4/$) close repulsion B: %.3g\n", map_env->force_params.close_repulsion_b);
        vstr_printf(vstr_info, "(5/%) close repulsion C: %.3g\n", map_env->force_params.close_repulsion_c);
        vstr_printf(vstr_info, "(6/^) close repulsion D: %.3g\n", map_env->force_params.close_repulsion_d);
    }
}
