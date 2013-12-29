#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>
#include <sys/stat.h>
#include <fcntl.h>
#include <string.h>
#include <math.h>

#include "xiwilib.h"
#include "Common.h"
#include "Layout.h"
#include "Force.h"
#include "Quadtree.h"
#include "map.h"
#include "mapprivate.h"

map_env_t *map_env_new() {
    map_env_t *map_env = m_new(map_env_t, 1);
    map_env->max_num_papers = 0;
    map_env->all_papers = NULL;
    map_env->num_papers = 0;
    map_env->papers = NULL;
    map_env->quad_tree = Quadtree_new();

    map_env->make_fake_links = true;
    map_env->other_links_veto = false;

    map_env->force_params.do_close_repulsion = false;
    map_env->force_params.close_repulsion_a = 1e9;
    map_env->force_params.close_repulsion_b = 1e14;
    map_env->force_params.close_repulsion_c = 1.1;
    map_env->force_params.close_repulsion_d = 0.6;
    map_env->force_params.use_ref_freq = true;
    map_env->force_params.anti_gravity_falloff_rsq = 1e6;
    map_env->force_params.anti_gravity_falloff_rsq_inv = 1.0 / map_env->force_params.anti_gravity_falloff_rsq;
    map_env->force_params.link_strength = 0.77;

    map_env->do_tred = false;
    map_env->draw_grid = false;
    map_env->draw_paper_links = false;

    map_env->tr_scale = 4;
    map_env->tr_x0 = 280;
    map_env->tr_y0 = 280;

    map_env->energy = 0;
    map_env->progress = 0;
    map_env->step_size = 0.1;
    map_env->max_link_force_mag = 0;
    map_env->max_total_force_mag = 0;

    map_env->x_sd = 1;
    map_env->y_sd = 1;

    map_env->keyword_set = NULL;

    return map_env;
}

void map_env_world_to_screen(map_env_t *map_env, double *x, double *y) {
    *x = map_env->tr_scale * (*x) + map_env->tr_x0;
    *y = map_env->tr_scale * (*y) + map_env->tr_y0;
}

void map_env_screen_to_world(map_env_t *map_env, double screen_w, double screen_h, double *x, double *y) {
    *x = ((*x) - 0.5 * screen_w - map_env->tr_x0) / map_env->tr_scale;
    *y = ((*y) - 0.5 * screen_h - map_env->tr_y0) / map_env->tr_scale;
}

int map_env_get_num_papers(map_env_t *map_env) {
    return map_env->num_papers;
}

Layout_node_t *map_env_get_layout_node_at(map_env_t *map_env, double screen_w, double screen_h, double x, double y) {
    map_env_screen_to_world(map_env, screen_w, screen_h, &x, &y);
    return layout_get_node_at(map_env->layout, x, y);
}

void map_env_set_papers(map_env_t *map_env, int num_papers, Common_paper_t *papers, Common_keyword_set_t *kws) {
    map_env->max_num_papers = num_papers;
    map_env->all_papers = papers;
    map_env->papers = m_renew(Common_paper_t*, map_env->papers, map_env->max_num_papers);
    map_env->keyword_set = kws;
    for (int i = 0; i < map_env->max_num_papers; i++) {
        Common_paper_t *p = &map_env->all_papers[i];
#ifdef ENABLE_TRED
        p->refs_tred_computed = m_new(int, p->num_refs);
#endif
        p->num_included_cites = p->num_cites;
        p->mass = 0.2 + 0.2 * p->num_included_cites;
        p->radius = sqrt(p->mass / M_PI);
    }
}

void map_env_random_papers(map_env_t *map_env, int n) {
    map_env->max_num_papers = n;
    map_env->all_papers = m_renew(Common_paper_t, map_env->all_papers, n);
    map_env->papers = m_renew(Common_paper_t*, map_env->papers, map_env->max_num_papers);
    for (int i = 0; i < n; i++) {
        Common_paper_t *p = &map_env->all_papers[i];
        Common_paper_init(p, i + 1);
        p->allcats[0] = random() % CAT_NUMBER_OF;
        p->index = i;
        p->radius = 0.1 + 0.05 / (0.01 + 1.0 * random() / RAND_MAX);
        if (p->radius > 4) { p->radius = 4; }
        p->mass = M_PI * p->radius * p->radius;
    }
}

void map_env_papers_test1(map_env_t *map_env, int n) {
    // the first paper is cited by the rest
    map_env->max_num_papers = n;
    map_env->all_papers = m_renew(Common_paper_t, map_env->all_papers, n);
    map_env->papers = m_renew(Common_paper_t*, map_env->papers, map_env->max_num_papers);
    for (int i = 0; i < n; i++) {
        Common_paper_t *p = &map_env->all_papers[i];
        Common_paper_init(p, i + 1);
        p->allcats[0] = 1;
        p->index = i;
        if (i == 0) {
            p->mass = 0.2 + 0.1 * (n - 1);
        } else {
            p->mass = 0.2;
        }
        p->radius = sqrt(p->mass / M_PI);
        if (i > 0) {
            p->num_refs = 1;
            p->refs = m_new(Common_paper_t*, 1);
            p->refs[0] = &map_env->all_papers[0];
        }
    }
}

void map_env_papers_test2(map_env_t *map_env, int n) {
    // the first 2 papers are cited both by the rest
    map_env->max_num_papers = n;
    map_env->all_papers = m_renew(Common_paper_t, map_env->all_papers, n);
    map_env->papers = m_renew(Common_paper_t*, map_env->papers, map_env->max_num_papers);
    for (int i = 0; i < n; i++) {
        Common_paper_t *p = &map_env->all_papers[i];
        p->allcats[0] = 1;
        p->index = i;
        if (i < 2) {
            p->mass = 0.2 + 0.1 * (n - 2);
        } else {
            p->mass = 0.2;
        }
        p->radius = sqrt(p->mass / M_PI);
        if (i >= 2) {
            p->num_refs = 2;
            p->refs = m_new(Common_paper_t*, 2);
            p->refs[0] = &map_env->all_papers[0];
            p->refs[1] = &map_env->all_papers[1];
        }
    }
}

void map_env_centre_view(map_env_t *map_env) {
    map_env->tr_x0 = 0.0;
    map_env->tr_y0 = 0.0;
}

void map_env_set_zoom_to_fit_n_standard_deviations(map_env_t *map_env, double n, double screen_w, double screen_h) {
    if (map_env->x_sd < 1e-3 || map_env->y_sd < 1e-3) {
        return;
    }
    double tr_xx = screen_w / (2 * n * map_env->x_sd);
    double tr_yy = screen_h / (2 * n * map_env->y_sd);
    if (tr_xx < tr_yy) {
        map_env->tr_scale = tr_xx;
    } else {
        map_env->tr_scale = tr_yy;
    }
}

void map_env_scroll(map_env_t *map_env, double dx, double dy) {
    map_env->tr_x0 += dx;
    map_env->tr_y0 += dy;
}

void map_env_zoom(map_env_t *map_env, double screen_x, double screen_y, double amt) {
    map_env->tr_scale *= amt;
    map_env->tr_x0 = map_env->tr_x0 * amt + screen_x * (1.0 - amt);
    map_env->tr_y0 = map_env->tr_y0 * amt + screen_y * (1.0 - amt);
}

double map_env_get_step_size(map_env_t *map_env) {
    return map_env->step_size;
}

void map_env_set_step_size(map_env_t *map_env, double value) {
    map_env->step_size = value;
}

void map_env_set_do_close_repulsion(map_env_t *map_env, bool value) {
    map_env->force_params.do_close_repulsion = value;
}

void map_env_set_make_fake_links(map_env_t *map_env, bool value) {
    map_env->make_fake_links = value;
}

void map_env_set_other_links_veto(map_env_t *map_env, bool value) {
    map_env->other_links_veto = value;
}

void map_env_set_anti_gravity(map_env_t *map_env, double val) {
    map_env->force_params.anti_gravity_falloff_rsq = val;
    map_env->force_params.anti_gravity_falloff_rsq_inv = 1.0 / map_env->force_params.anti_gravity_falloff_rsq;
}

void map_env_set_link_strength(map_env_t *map_env, double val) {
    map_env->force_params.link_strength = val;
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

void map_env_toggle_do_close_repulsion(map_env_t *map_env) {
    map_env->force_params.do_close_repulsion = !map_env->force_params.do_close_repulsion;
}

void map_env_toggle_use_ref_freq(map_env_t *map_env) {
    map_env->force_params.use_ref_freq = !map_env->force_params.use_ref_freq;
}

void map_env_adjust_anti_gravity(map_env_t *map_env, double amt) {
    map_env->force_params.anti_gravity_falloff_rsq *= amt;
    map_env->force_params.anti_gravity_falloff_rsq_inv = 1.0 / map_env->force_params.anti_gravity_falloff_rsq;
}

void map_env_adjust_link_strength(map_env_t *map_env, double amt) {
    map_env->force_params.link_strength *= amt;
}

void map_env_adjust_close_repulsion(map_env_t *map_env, double amt_a, double amt_b) {
    map_env->force_params.close_repulsion_a *= amt_a;
    map_env->force_params.close_repulsion_b *= amt_b;
}

void map_env_adjust_close_repulsion2(map_env_t *map_env, double amt_a, double amt_b) {
    map_env->force_params.close_repulsion_c *= amt_a;
    map_env->force_params.close_repulsion_d += amt_b;
}

int map_env_number_of_coarser_layouts(map_env_t *map_env) {
    int num_coarser = 0;
    for (Layout_t *l = map_env->layout->parent_layout; l != NULL; l = l->parent_layout) {
        num_coarser += 1;
    }
    return num_coarser;
}

int map_env_number_of_finer_layouts(map_env_t *map_env) {
    int num_finer = 0;
    for (Layout_t *l = map_env->layout->child_layout; l != NULL; l = l->child_layout) {
        num_finer += 1;
    }
    return num_finer;
}

void map_env_coarsen_layout(map_env_t *map_env) {
    if (map_env->layout->parent_layout != NULL) {
        map_env->layout = map_env->layout->parent_layout;
        Layout_t *l = map_env->layout;
        for (int i = 0; i < l->num_nodes; i++) {
            l->nodes[i].x = l->nodes[i].child1->x;
            l->nodes[i].y = l->nodes[i].child1->y;
        }
    }
}

void map_env_refine_layout(map_env_t *map_env) {
    if (map_env->layout->child_layout != NULL) {
        map_env->layout = map_env->layout->child_layout;
        Layout_t *l = map_env->layout;
        for (int i = 0; i < l->num_nodes; i++) {
            Layout_node_t *node = &l->nodes[i];
            Layout_node_t *parent = node->parent;
            if (parent->child2 == NULL) {
                // only 1 child, being this node
                node->x = parent->x;
                node->y = parent->y;
            } else if (parent->child1 == node) {
                // 2 children; put this node on the left
                node->x = parent->x - (1.0 - node->mass / parent->mass) * parent->radius;
                node->y = parent->y;
            } else {
                // 2 children; put this node on the right
                node->x = parent->x + (1.0 - node->mass / parent->mass) * parent->radius;
                node->y = parent->y;
            }
        }
    }
}

void map_env_jolt(map_env_t *map_env, double amt) {
    for (int i = 0; i < map_env->layout->num_nodes; i++) {
        Layout_node_t *n = &map_env->layout->nodes[i];
        n->x += amt * (-0.5 + 1.0 * random() / RAND_MAX);
        n->y += amt * (-0.5 + 1.0 * random() / RAND_MAX);
    }
}

void map_env_rotate_all(map_env_t *map_env, double angle) {
    Layout_rotate_all(map_env->layout, angle);
}

void map_env_orient_using_category(map_env_t *map_env, Common_category_t wanted_cat, double wanted_angle) {
    // must be finest layout and must have at least 1 node
    if (map_env->layout->child_layout != NULL || map_env->layout->num_nodes == 0) {
        return;
    }

    // compute average of papers with wanted category
    double avg_cat_x = 0.0;
    double avg_cat_y = 0.0;
    double mass_cat = 0.0;
    for (int i = 0; i < map_env->layout->num_nodes; i++) {
        Layout_node_t *n = &map_env->layout->nodes[i];
        if (n->paper->allcats[0] == wanted_cat) {
            avg_cat_x += n->mass * n->x;
            avg_cat_y += n->mass * n->y;
            mass_cat += n->mass;
        }
    }

    // orient papers
    if (mass_cat == 0.0) {
        printf("could not orient graph using category %s\n", Common_category_enum_to_str(wanted_cat));
    } else {
        avg_cat_x /= mass_cat;
        avg_cat_y /= mass_cat;
        double angle = wanted_angle - atan2(avg_cat_y, avg_cat_x);
        Layout_rotate_all(map_env->layout, angle);
        printf("rotated graph by %.2f rad to orient %s at %.2f rad\n", angle, Common_category_enum_to_str(wanted_cat), wanted_angle);
    }
}

void map_env_orient_using_paper(map_env_t *map_env, Common_paper_t *wanted_paper, double wanted_angle) {
    // must be finest layout and must have at least 1 node
    if (map_env->layout->child_layout != NULL || map_env->layout->num_nodes == 0 || !wanted_paper->included || !wanted_paper->connected) {
        return;
    }

    // orient papers
    double angle = wanted_angle - atan2(wanted_paper->layout_node->y, wanted_paper->layout_node->x);
    Layout_rotate_all(map_env->layout, angle);
    printf("rotated graph by %.2f rad to orient paper %u at %.2f rad\n", angle, wanted_paper->id, wanted_angle);
}

void map_env_flip_x(map_env_t *map_env) {
    for (int i = 0; i < map_env->layout->num_nodes; i++) {
        Layout_node_t *n = &map_env->layout->nodes[i];
        n->x = -n->x;
    }
}

/* compute node-node forces using naive gravity/anti-gravity method
 * this method is of order N^2, and hence very slow (but accurate)
 */
void compute_naive_node_node_force(Force_params_t *force_params, Layout_t *layout) {
    for (int i = 0; i < layout->num_nodes; i++) {
        Layout_node_t *n1 = &layout->nodes[i];
        for (int j = i + 1; j < layout->num_nodes; j++) {
            Layout_node_t *n2 = &layout->nodes[j];
            double dx = n1->x - n2->x;
            double dy = n1->y - n2->y;
            double rsq = dx*dx + dy*dy;
            if (rsq > 1e-4) {
                if (rsq > force_params->anti_gravity_falloff_rsq) { rsq *= rsq * force_params->anti_gravity_falloff_rsq_inv; }
                double fac = n1->mass * n2->mass / rsq;
                double fx = dx * fac;
                double fy = dy * fac;
                n1->fx += fx;
                n1->fy += fy;
                n2->fx -= fx;
                n2->fy -= fy;
            }
        }
    }
}

/* attraction of disconnected papers to centre of papers with the same category
 */
void attract_disconnected_to_centre_of_category(map_env_t *map_env) {
    for (int i = 0; i < map_env->num_papers; i++) {
        Common_paper_t *p = map_env->papers[i];
        if (!p->connected) {
            for (int j = 0; j < COMMON_PAPER_MAX_CATS && p->allcats[j] != CAT_UNKNOWN; j++) {
                category_info_t *cat = &map_env->category_info[p->allcats[j]];

                double dx = p->layout_node->x - cat->x;
                double dy = p->layout_node->y - cat->y;
                double r = sqrt(dx*dx + dy*dy);
                double rest_len = 0.1 * sqrt(cat->num);

                double fac = 0.1 * map_env->force_params.link_strength;

                if (r > rest_len) {
                    fac *= (r - rest_len) / r;
                    double fx = dx * fac;
                    double fy = dy * fac;
                    p->layout_node->fx -= fx;
                    p->layout_node->fy -= fy;
                }
            }
        }
    }
}

/* obsolete
void compute_keyword_force(Force_params_t *param, int num_papers, Common_paper_t **papers) {
    // reset keyword locations
    for (int i = 0; i < num_papers; i++) {
        Common_paper_t *p = papers[i];
        for (int j = 0; j < p->num_keywords; j++) {
            Common_keyword_t *kw = p->keywords[j];
            kw->num_papers = 0;
            kw->x = 0;
            kw->y = 0;
        }
    }

    // compute keyword locations, by averaging connected papers that have that keyword
    for (int i = 0; i < num_papers; i++) {
        Common_paper_t *p = papers[i];
        if (p->connected) {
            for (int j = 0; j < p->num_keywords; j++) {
                Common_keyword_t *kw = p->keywords[j];
                kw->num_papers += 1;
                kw->x += p->layout_node->x;
                kw->y += p->layout_node->y;
            }
        }
    }
    for (int i = 0; i < num_papers; i++) {
        Common_paper_t *p = papers[i];
        for (int j = 0; j < p->num_keywords; j++) {
            Common_keyword_t *kw = p->keywords[j];
            if (kw->num_papers > 0) {
                kw->x /= kw->num_papers;
                kw->y /= kw->num_papers;
                kw->num_papers = 0;
            }
        }
    }

    // compute forces on disconnected papers due to keywords
    for (int i = 0; i < num_papers; i++) {
        Common_paper_t *p = papers[i];
        if (p->connected) {
            continue;
        }
        for (int j = 0; j < p->num_keywords; j++) {
            Common_keyword_t *kw = p->keywords[j];

            double dx = p->x - kw->x;
            double dy = p->y - kw->y;
            double r = sqrt(dx*dx + dy*dy);
            double rest_len = 0.1;

            double fac = 1.1 * param->link_strength;

            if (r > rest_len) {
                fac *= (r - rest_len) / r;
                double fx = dx * fac;
                double fy = dy * fac;

                p->layout_node->fx -= fx;
                p->layout_node->fy -= fy;
            }
        }
    }
}
*/

static void compute_category_locations(map_env_t *map_env) {
    for (int i = 0; i < CAT_NUMBER_OF; i++) {
        category_info_t *cat = &map_env->category_info[i];
        cat->num = 0;
        cat->y = 0.0;
        cat->x = 0.0;
    }
    for (int i = 0; i < map_env->num_papers; i++) {
        Common_paper_t *p = map_env->papers[i];
        category_info_t *cat = &map_env->category_info[p->allcats[0]];
        cat->num += 1;
        cat->x += p->layout_node->x;
        cat->y += p->layout_node->y;
    }
    for (int i = 0; i < CAT_NUMBER_OF; i++) {
        category_info_t *cat = &map_env->category_info[i];
        if (cat->num > 0) {
            cat->x /= cat->num;
            cat->y /= cat->num;
        }
    }
}

static bool layout_node_is_not_held(Layout_node_t *n) {
    return (n->flags & LAYOUT_NODE_HOLD_STILL) == 0;
}

static void map_env_compute_forces(map_env_t *map_env) {
    // reset the forces, and work out if any nodes are held
    int any_nodes_held = 0;
    for (int i = 0; i < map_env->layout->num_nodes; i++) {
        Layout_node_t *n = &map_env->layout->nodes[i];
        any_nodes_held |= (n->flags & LAYOUT_NODE_HOLD_STILL);
        n->fx = 0;
        n->fy = 0;
    }

    // rotate everything by a little each iteration to eliminate artifacts from quad tree force algo
    map_env_rotate_all(map_env, 0.002);

    // compute node-link-node spring forces
    Force_compute_attractive_link_force(&map_env->force_params, map_env->do_tred, map_env->layout);

    // compute maximum force (purely for user display, to make sure it's not too huge)
    double max_fmag = 0;
    for (int i = 0; i < map_env->layout->num_nodes; i++) {
        Layout_node_t *n = &map_env->layout->nodes[i];
        max_fmag = fmax(max_fmag, (double)n->fx * (double)n->fx + (double)n->fy * (double)n->fy);
    }
    map_env->max_link_force_mag = sqrt(max_fmag);

    // compute node-node anti-gravity forces using quad tree
    Quadtree_build(map_env->layout, map_env->quad_tree);
    if (any_nodes_held) {
        Force_quad_tree_apply_if(&map_env->force_params, map_env->quad_tree, layout_node_is_not_held);
    } else {
        Force_quad_tree_forces(&map_env->force_params, map_env->quad_tree);
    }

    //compute_keyword_force(&map_env->force_params, map_env->num_papers, map_env->papers);

    //attract_disconnected_to_centre_of_category(map_env);
}

bool map_env_iterate(map_env_t *map_env, Layout_node_t *hold_still, bool boost_step_size, bool very_fine_steps) {
    map_env_compute_forces(map_env);

    // boost the step size if asked
    if (boost_step_size) {
        if (map_env->step_size < 1) {
            map_env->step_size = 2;
        } else {
            map_env->step_size *= 2;
        }
    }

    // when doing close repulsion, make sure step size is not too big
    if (map_env->force_params.do_close_repulsion) {
        map_env->step_size = fmin(1.0, map_env->step_size);
    }

    // when doing very fine steps, make sure the step size is small
    if (very_fine_steps) {
        map_env->step_size = fmin(0.05, map_env->step_size);
    }

    // use the computed forces to update the (x,y) positions of the papers
    double energy = 0;
    double x_sum = 0;
    double y_sum = 0;
    double xsq_sum = 0;
    double ysq_sum = 0;
    double total_mass = 0;
    double max_fmag = 0;
    for (int i = 0; i < map_env->layout->num_nodes; i++) {
        Layout_node_t *n = &map_env->layout->nodes[i];

        n->fx /= n->mass;
        n->fy /= n->mass;

        double fmag = (double)n->fx * (double)n->fx + (double)n->fy * (double)n->fy;
        if (!isfinite(fmag)) {
            fmag = 1e100;
        }
        fmag = sqrt(fmag);
        max_fmag = fmax(max_fmag, fmag);

        energy += fmag;

        double dt = map_env->step_size / fmag;

        if (!(n == hold_still || (n->flags & LAYOUT_NODE_HOLD_STILL))) {
            n->x += dt * n->fx;
            n->y += dt * n->fy;
        }

        x_sum += n->x * n->mass;
        y_sum += n->y * n->mass;
        xsq_sum += n->x * n->x * n->mass;
        ysq_sum += n->y * n->y * n->mass;
        total_mass += n->mass;
    }

    map_env->max_total_force_mag = max_fmag;

    // centre papers on the centre of mass
    x_sum /= total_mass;
    y_sum /= total_mass;
    for (int i = 0; i < map_env->layout->num_nodes; i++) {
        Layout_node_t *n = &map_env->layout->nodes[i];
        if (n == hold_still) {
            continue;
        }
        n->x -= x_sum;
        n->y -= y_sum;
    }

    // compute standard deviation in x, y
    xsq_sum /= total_mass;
    ysq_sum /= total_mass;
    map_env->x_sd = sqrt(xsq_sum - x_sum * x_sum);
    map_env->y_sd = sqrt(ysq_sum - y_sum * y_sum);

    // propagate node positions to children (to calculate locations of categories)
    Layout_propagate_positions_to_children(map_env->layout);

    // update the locations of the categories
    compute_category_locations(map_env);

    // adjust the step size
    if (!isfinite(energy)) {
        map_env->step_size = 2;
    } else if (energy < map_env->energy) {
        // energy went down
        if (map_env->progress < 3) {
            map_env->progress += 1;
        } else {
            if (map_env->step_size < 5) {
                map_env->step_size *= 1.3;
            }
        }
    } else {
        // energy went up
        map_env->progress = 0;
        if (map_env->step_size > 0.025) {
            map_env->step_size *= 0.95;
        }
    }
    map_env->energy = energy;

    if (!very_fine_steps && map_env->force_params.do_close_repulsion && map_env->max_total_force_mag > pow(map_env->max_link_force_mag, 2)) {
        if (map_env->step_size < 0.15) {
            map_env->step_size = 0.15;
        }
        return false;
    }

    return map_env->step_size <= 1e-1;

    #if 0
    // work out maximum force
    double fmax = 0;
    for (int i = 0; i < map_env->num_papers; i++) {
        Common_paper_t *p = map_env->papers[i];
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
        Common_paper_t *p = map_env->papers[i];
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

void map_env_get_max_id_range(map_env_t *map_env, int *id_min, int *id_max) {
    if (map_env->max_num_papers > 0) {
        *id_min = map_env->all_papers[0].id;
        *id_max = map_env->all_papers[map_env->max_num_papers - 1].id;
    } else {
        *id_min = 0;
        *id_max = 0;
    }
}

/*
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
    Common_recompute_num_included_cites(map_env->cur_num_papers, map_env->all_papers);
    Common_recompute_colours(map_env->cur_num_papers, map_env->all_papers, false);
    //Common_compute_tred(map_env->cur_num_papers, map_env->all_papers);
    for (int i = 0; i < map_env->cur_num_papers; i++) {
        Common_paper_t *p = &map_env->all_papers[i];
        p->mass = 0.05 + 0.2 * p->num_included_cites;
        p->r = sqrt(p->mass / M_PI);
        p->index2 = i;
    }
    // compute initial position for newly added papers (average of all its references)
    for (int i = old_num_papers; i < map_env->cur_num_papers; i++) {
        Common_paper_t *p = &map_env->all_papers[i];
        if (!p->pos_valid) {
            map_env_compute_best_start_position_for_paper(map_env, p);
            p->pos_valid = true;
        }
    }

    // make array of papers that we want to include (only include biggest connected graph)
    int biggest_col = 0;
    int num_with_biggest_col = 10;
    for (int i = 0; i < map_env->cur_num_papers; i++) {
        Common_paper_t *p = &map_env->all_papers[i];
        if (p->num_with_my_colour > num_with_biggest_col) {
            biggest_col = p->colour;
            num_with_biggest_col = p->num_with_my_colour;
        }
    }
    map_env->num_papers = 0;
    for (int i = 0; i < map_env->cur_num_papers; i++) {
        Common_paper_t *p = &map_env->all_papers[i];
        if (p->colour == biggest_col) {
            map_env->papers[map_env->num_papers++] = p;
        }
    }

    if (amt > 10) {
        map_env->step_size = 1;
    }

    //printf("now have %d papers, %d connected and included in graph, maximum id is %d\n", map_env->cur_num_papers, map_env->num_papers, map_env->all_papers[map_env->cur_num_papers - 1].id);
}
*/

// makes fake links for a paper to the connected part of the graph
static void make_fake_links_for_paper(map_env_t *map_env, Common_paper_t *paper) {
    // allocate memory for the fake links
    paper->num_fake_links = 0;
    paper->fake_links = m_new(Common_paper_t*, paper->num_keywords == 0 ? 1 : paper->num_keywords);

    // want to make links only to papers in the same main category
    Common_category_t want_cat = paper->allcats[0];

    // go through all the keywords for this paper
    for (int i = 0; i < paper->num_keywords; i++) {
        Common_keyword_t *want_kw = paper->keywords[i];

        // found an appropriate paper, so make a fake link
        if (want_kw->paper != NULL) {
            paper->fake_links[paper->num_fake_links++] = want_kw->paper;
            /*
            if (paper->title == NULL || p_found->title == NULL) {
                printf("connected %d to %d\n", paper->id, p_found->id);
            } else {
                printf("connected %s to %s\n", paper->title, p_found->title);
            }
            */
        }
    }

    // if we couldn't find anything, try just looking for something in the same category
    if (paper->num_fake_links == 0) {
        //printf("for paper %d, resorting to category link (it has %d keywords)\n", paper->id, paper->num_keywords);
        Common_paper_t *p_found = NULL;
        for (int i = 0; i < map_env->num_papers; i++) {
            Common_paper_t *p2 = map_env->papers[i];
            if (p2->included && p2->connected && p2->allcats[0] == want_cat) {
                if (p_found == NULL || p2->mass > p_found->mass) {
                    p_found = p2;
                }
            }
        }
        if (p_found != NULL) {
            paper->fake_links[paper->num_fake_links++] = p_found;
        }
    }
}

void paper_propagate_connectivity(Common_paper_t *paper) {
    if (paper->included && !paper->connected) {
        paper->connected = true;
        for (int i = 0; i < paper->num_refs; i++) {
            paper_propagate_connectivity(paper->refs[i]);
        }
        for (int i = 0; i < paper->num_cites; i++) {
            paper_propagate_connectivity(paper->cites[i]);
        }
    }
}

void map_env_select_date_range(map_env_t *map_env, int id_start, int id_end) {
    int i_start = map_env->max_num_papers - 1;
    int i_end = 0;
    for (int i = 0; i < map_env->max_num_papers; i++) {
        Common_paper_t *p = &map_env->all_papers[i];
        p->included = false;
        if (p->id >= id_start && p->id <= id_end) {
            if (i < i_start) {
                i_start = i;
            }
            if (i > i_end) {
                i_end = i;
            }
        }
    }

    if (i_start > i_end) {
        // no papers in id range
        map_env->num_papers = 0;
        return;
    }

    printf("date range: %d - %d; index %d - %d\n", id_start, id_end, i_start, i_end);

    for (int i = i_start; i <= i_end; i++) {
        Common_paper_t *p = &map_env->all_papers[i];
        p->included = true;
        p->age = 1.0 * (p->id - id_start) / (id_end - id_start);
    }

    // Disclude papers with only references
    if (map_env->other_links_veto) {
        for (int i = i_start; i <= i_end; i++) {
            Common_paper_t *p = &map_env->all_papers[i];
            if (p->included) {
                // keep only papers that have "other" links, as we intend to ignore normal links
                bool valid_other_paper = false;
                for (int j = 0; j < p->num_refs; j++) {
                    if (p->refs_ref_freq[j] == 0 && p->refs_other_weight[j] > 0) {
                        valid_other_paper = true;
                        break;
                    }
                }
                if (!valid_other_paper) {
                    p->included = false;
                }
            }
        }
    }

    // CALCULATE CONNECTED

    // think this can be placed later, as may not include disconnected papers
    //Common_recompute_num_included_cites(map_env->max_num_papers, map_env->all_papers);
    
    Common_recompute_colours(map_env->max_num_papers, map_env->all_papers, false);

#ifdef ENABLE_TRED
    Common_compute_tred(map_env->max_num_papers, map_env->all_papers);
#endif

    // think this can be placed later, as may not include disconnected papers
    
    // recompute mass and radius based on num_included_cites
    //for (int i = 0; i < map_env->max_num_papers; i++) {
    //    Common_paper_t *p = &map_env->all_papers[i];
    //    p->mass = 0.2 + 0.2 * p->num_included_cites;
    //    p->radius = sqrt(p->mass / M_PI);
    //}

    // work out the colour of the graph with the most number of connected papers
    int biggest_col = 0;
    int num_with_biggest_col = 2;
    for (int i = 0; i < map_env->max_num_papers; i++) {
        Common_paper_t *p = &map_env->all_papers[i];
        if (p->included && p->num_with_my_colour > num_with_biggest_col) {
            biggest_col = p->colour;
            num_with_biggest_col = p->num_with_my_colour;
        }
    }

    // make array of papers that we want to include, first the big connected graph
    map_env->num_papers = 0;
    for (int i = 0; i < map_env->max_num_papers; i++) {
        Common_paper_t *p = &map_env->all_papers[i];
        if (p->included) {
            p->connected = (p->colour == biggest_col);
            map_env->papers[map_env->num_papers++] = p;
        }
    }

    // print some info
    printf("have %d papers in total\n", map_env->num_papers);
    printf("have %d papers in big connected graph\n", num_with_biggest_col);

    // now link all the disconnected pieces to the big graph, where possible
    // for efficiency, do it on a per-category basis
    int total_fake_papers = 0;
    int total_fake_links = 0;
    if (map_env->make_fake_links) {
        for (int cat = 0; cat < CAT_NUMBER_OF; cat++) {
            // for each keyword, find the paper in this category that has the largest mass
            Common_keyword_set_clear_data(map_env->keyword_set);
            for (int i = 0; i < map_env->num_papers; i++) {
                Common_paper_t *p = map_env->papers[i];
                if (p->included && p->connected && p->allcats[0] == cat) {
                    for (int j = 0; j < p->num_keywords; j++) {
                        if (p->keywords[j]->paper == NULL || p->mass > p->keywords[j]->paper->mass) {
                            p->keywords[j]->paper = p;
                        }
                    }
                }
            }

            // for each disconnected paper, try to connect it
            for (int i = 0; i < map_env->num_papers; i++) {
                Common_paper_t *p = map_env->papers[i];
                if (!p->connected && p->allcats[0] == cat) {
                    // try to connect this paper to the big graph
                    make_fake_links_for_paper(map_env, p);
                    if (p->num_fake_links > 0) {
                        total_fake_papers += 1;
                        total_fake_links += p->num_fake_links;
                        paper_propagate_connectivity(p);
                    }
                }
            }
        }
    }

    // print some info
    printf("connected %d papers with %d fake links\n", total_fake_papers, total_fake_links);

    // check what couldn't be connected
    int total_not_connected = 0;
    //for (int i = 0; i < map_env->num_papers; i++) {
    for (int i = map_env->num_papers-1; i >= 0; i--) {
        Common_paper_t *p = map_env->papers[i];
        if (p->included && !p->connected) {
            if (map_env->make_fake_links) {
                printf("WARNING: could not connect paper %d with fake links; allcats[0]=%s, keywords=", p->id, Common_category_enum_to_str(p->allcats[0]));
                for (int j = 0; j < p->num_keywords; j++) {
                    printf("%s,", p->keywords[j]->keyword);
                }
                printf("\n");
            }
            // also disclude from graph
            p->included = false;
            total_not_connected += 1;

            // remove this paper from the list
            //memmove(&map_env->papers[i], &map_env->papers[i + 1], (map_env->num_papers - i - 1) * sizeof(Common_paper_t*));
            //map_env->num_papers -= 1;
            if (i+1 < map_env->num_papers) {
                memmove(&map_env->papers[i], &map_env->papers[i + 1], (map_env->num_papers - i - 1) * sizeof(Common_paper_t*));
            }
            map_env->num_papers -= 1;
        }
    }

    // print some info
    printf("after making fake links, have %d papers not connected\n", total_not_connected);

    // RECOMPUTE DIMENSIONS

    Common_recompute_num_included_cites(map_env->max_num_papers, map_env->all_papers);

    for (int i = 0; i < map_env->max_num_papers; i++) {
        Common_paper_t *p = &map_env->all_papers[i];
        p->mass = 0.2 + 0.2 * p->num_included_cites;
        p->radius = sqrt(p->mass / M_PI);
    }

}

void map_env_layout_new(map_env_t *map_env, int num_coarsenings, double factor_ref_freq, double factor_other_link) {
    // make the layouts, each one coarser than the previous
    Layout_t *l = build_layout_from_papers(map_env->num_papers, map_env->papers, false, factor_ref_freq, factor_other_link);
    for (int i = 0; i < num_coarsenings && l->num_links > 1; i++) {
        l = build_reduced_layout_from_layout(l);
    }
    map_env->layout = l;

    // initialise the coarsest layout with random positions for the nodes
    for (int i = 0; i < l->num_nodes; i++) {
        l->nodes[i].x = 100.0 * (-0.5 + 1.0 * random() / RAND_MAX);
        l->nodes[i].y = 100.0 * (-0.5 + 1.0 * random() / RAND_MAX);
    }

    // print info about the layouts
    for (; l != NULL; l = l->child_layout) {
        Layout_print(l);
    }

    // increase the step size for the next force iteration
    map_env->step_size = 1;
}

int map_env_layout_place_new_papers(map_env_t *map_env) {
    Layout_t *l = map_env->layout;
    int num = 0;
    int id_low = 0;
    for (int i = 0; i < l->num_nodes; i++) {
        Layout_node_t *n = &l->nodes[i];
        if (n->flags & LAYOUT_NODE_POS_VALID) {
            n->flags |= LAYOUT_NODE_HOLD_STILL;
        } else {
            num += 1;
            if (id_low == 0 || n->paper->id < id_low) {
                id_low = n->paper->id;
            }
            Layout_node_compute_best_start_position(n);
        }
    }
    printf("have %d papers that need new positions, min id %d\n", num, id_low);
    return num;
}

void map_env_layout_finish_placing_new_papers(map_env_t *map_env) {
    Layout_t *l = map_env->layout;
    for (int i = 0; i < l->num_nodes; i++) {
        Layout_node_t *n = &l->nodes[i];
        n->flags = (n->flags & ~LAYOUT_NODE_HOLD_STILL) | LAYOUT_NODE_POS_VALID;
    }
}

void map_env_layout_pos_load_from_json(map_env_t *map_env, const char *json_filename) {
    // make a single layout
    Layout_t *l = build_layout_from_papers(map_env->num_papers, map_env->papers, false, 1, 0);
    map_env->layout = l;

    // initialise random positions, in case we can't/don't load a position for a given paper
    for (int i = 0; i < l->num_nodes; i++) {
        l->nodes[i].x = 100.0 * random() / RAND_MAX;
        l->nodes[i].y = 100.0 * random() / RAND_MAX;
    }

    // print info about the layout
    Layout_print(l);

    // read in the json file to set the node positions
    FILE *fp = fopen(json_filename, "rb");
    if (fp == NULL) {
        printf("WARNING: could not open %s for reading; using random initial positions\n", json_filename);
        return;
    }

    int c = fgetc(fp);
    if (c != '[') {
        printf("WARNING: malformed JSON file %s; reading first character, got %c\n", json_filename, c);
        return;
    }
    if ((c = fgetc(fp)) != '\n') {
        ungetc(c, fp);
    }
    int entry_num = 0;
    for (;;) {
        if ((c = fgetc(fp)) != '[') {
            ungetc(c, fp);
            break;
        }
        int id, x, y, r;
        if (fscanf(fp, "%d,%d,%d,%d]", &id, &x, &y, &r) != 4) {
            printf("WARNING: malformed JSON file %s; reading entry %d\n", json_filename, entry_num);
            return;
        }
        Layout_node_t *n = layout_get_node_by_id(l, id);
        if (n != NULL) {
            Layout_node_import_quantities(n, x, y);
            n->flags |= LAYOUT_NODE_POS_VALID;
        }
        entry_num += 1;
        if ((c = fgetc(fp)) != ',') {
            ungetc(c, fp);
        }
        if ((c = fgetc(fp)) != '\n') {
            ungetc(c, fp);
        }
    }
    if ((c = fgetc(fp)) != ']') {
        printf("WARNING: malformed JSON file %s; reading last character, got %c\n", json_filename, c);
        return;
    }
    printf("read %d entries from JSON file %s\n", entry_num, json_filename);
    fclose(fp);

    // set do_close_repulsion, since we are loading a layout that was saved this way
    //map_env->force_params.do_close_repulsion = true;
    map_env_set_do_close_repulsion(map_env, true);

    // small step size for the next force iteration
    map_env->step_size = 0.1;
}

/* obsolete
void vstr_add_json_str(vstr_t *vstr, const char *s) {
    vstr_add_byte(vstr, '"');
    for (; *s != '\0'; s++) {
        if (*s == '"') {
            vstr_add_byte(vstr, '\\');
            vstr_add_byte(vstr, '"');
        } else {
            vstr_add_byte(vstr, *s);
        }
    }
    vstr_add_byte(vstr, '"');
}
*/

void map_env_layout_pos_save_to_json(map_env_t *map_env, const char *file) {
    // write the positions as JSON to a vstr
    vstr_t *vstr = vstr_new();
    vstr_printf(vstr, "[\n");
    for (int i = 0; i < map_env->num_papers; i++) {
        Common_paper_t *p = map_env->papers[i];
        int x, y, r;
        Layout_node_export_quantities(p->layout_node, &x, &y, &r);
        vstr_printf(vstr, "[%d,%d,%d,%d]", p->id, x, y, r);
        if (i + 1 < map_env->num_papers) {
            vstr_printf(vstr, ",\n");
        } else {
            vstr_printf(vstr, "\n");
        }
    }
    vstr_printf(vstr, "]\n");

    // write the vstr to a file
    int fd = open(file, O_WRONLY | O_CREAT | O_TRUNC, S_IRUSR | S_IWUSR);
    write(fd, vstr_str(vstr), vstr_len(vstr));
    close(fd);
    vstr_free(vstr);

    printf("wrote positions for %d papers to JSON file %s\n", map_env->num_papers, file);
}

void map_env_layout_link_save_to_json(map_env_t *map_env, const char *file) {
    // write the links as JSON to a vstr
    vstr_t *vstr = vstr_new();
    vstr_printf(vstr, "[\n");
    for (int i = 0; i < map_env->num_papers; i++) {
        Common_paper_t *p = map_env->papers[i];
        vstr_printf(vstr, "[%d,[", p->id);
        Layout_node_t *ln = p->layout_node;
        for (int j = 0; j < ln->num_links; j++) {
            if (j > 0) {
                vstr_printf(vstr, ",");
            }
            vstr_printf(vstr, "[%d,%.6g]", ln->links[j].node->paper->id, ln->links[j].weight);
        }
        if (i + 1 < map_env->num_papers) {
            vstr_printf(vstr, "]],\n");
        } else {
            vstr_printf(vstr, "]]\n");
        }
    }
    vstr_printf(vstr, "]\n");

    // write the vstr to a file
    int fd = open(file, O_WRONLY | O_CREAT | O_TRUNC, S_IRUSR | S_IWUSR);
    write(fd, vstr_str(vstr), vstr_len(vstr));
    close(fd);
    vstr_free(vstr);

    printf("wrote links for %d papers to JSON file %s\n", map_env->num_papers, file);
}
