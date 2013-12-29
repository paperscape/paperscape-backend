#ifndef _INCLUDED_FORCE_H
#define _INCLUDED_FORCE_H

#include "Layout.h"
#include "Quadtree.h"

typedef struct _Force_params_t {
    bool do_close_repulsion;
    double close_repulsion_a;
    double close_repulsion_b;
    double close_repulsion_c;
    double close_repulsion_d;
    bool use_ref_freq;
    double anti_gravity_falloff_rsq;
    double anti_gravity_falloff_rsq_inv;
    double link_strength;
} Force_params_t;

struct _Quadtree_t;

void Force_quad_tree_forces(Force_params_t *param, struct _Quadtree_t *qt);
void Force_quad_tree_apply_if(Force_params_t *param, struct _Quadtree_t *qt, bool (*f)(Layout_node_t*));

void Force_compute_attractive_link_force(Force_params_t *param, bool do_tred, Layout_t *layout);

#endif // _INCLUDED_FORCE_H
