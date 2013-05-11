#include <stdlib.h>
#include <assert.h>
#include <math.h>

#include "xiwilib.h"
#include "common.h"
#include "quadtree.h"
#include "force.h"

// q1 is a leaf against which we check q2
static void quad_tree_forces_leaf_vs_node(force_params_t *param, quad_tree_node_t *q1, quad_tree_node_t *q2) {
    if (q2 == NULL) {
        // q2 is empty node
    } else {
        // q2 is leaf or internal node

        // compute distance from q1 to centroid of q2
        double dx = q1->x - q2->x;
        double dy = q1->y - q2->y;
        double rsq = dx * dx + dy * dy;
        if (rsq < 1e-6) {
            // minimum distance cut-off
            rsq = 1e-6;
        }

        if (q2->num_items == 1) {
            // q2 is leaf node
            double fac;
            if (param->do_close_repulsion) {
                double rad_sum_sq = 1.2 * pow(q1->r + q2->r, 2);
                if (rsq < rad_sum_sq) {
                    // papers overlap, use stronger repulsive force
                    fac = fmin(200000, (exp(rad_sum_sq - rsq) - 1)) * 500 * fmax(1, pow(q1->mass * q2->mass, 3.0)) * param->anti_gravity_strength / rsq
                        + q1->mass * q2->mass * param->anti_gravity_strength / rad_sum_sq;
                } else {
                    // normal anti-gravity repulsive force
                    fac = q1->mass * q2->mass * param->anti_gravity_strength / rsq;
                }
            } else {
                // normal anti-gravity repulsive force
                fac = q1->mass * q2->mass * param->anti_gravity_strength / rsq;
            }
            double fx = dx * fac;
            double fy = dy * fac;
            q1->fx += fx;
            q1->fy += fy;
            q2->fx -= fx;
            q2->fy -= fy;

        } else {
            // q2 is internal node
            if (q2->side_length * q2->side_length < 0.45 * rsq) {
                // q1 and the cell q2 are "well separated"
                // approximate force by centroid of q2
                double fac = q1->mass * q2->mass * param->anti_gravity_strength / rsq;
                double fx = dx * fac;
                double fy = dy * fac;
                q1->fx += fx;
                q1->fy += fy;
                q2->fx -= fx;
                q2->fy -= fy;

            } else {
                // q1 and q2 are not "well separated"
                // descend into children of q2
                quad_tree_forces_leaf_vs_node(param, q1, q2->q0);
                quad_tree_forces_leaf_vs_node(param, q1, q2->q1);
                quad_tree_forces_leaf_vs_node(param, q1, q2->q2);
                quad_tree_forces_leaf_vs_node(param, q1, q2->q3);
            }
        }
    }
}

static void quad_tree_forces_ascend(force_params_t *param, quad_tree_node_t *q) {
    assert(q->num_items == 1); // must be a leaf node
    for (quad_tree_node_t *q2 = q; q2->parent != NULL; q2 = q2->parent) {
        quad_tree_node_t *parent = q2->parent;
        assert(parent->num_items > 1); // all parents should be internal nodes
        if (parent->q0 != q2) { quad_tree_forces_leaf_vs_node(param, q, parent->q0); }
        if (parent->q1 != q2) { quad_tree_forces_leaf_vs_node(param, q, parent->q1); }
        if (parent->q2 != q2) { quad_tree_forces_leaf_vs_node(param, q, parent->q2); }
        if (parent->q3 != q2) { quad_tree_forces_leaf_vs_node(param, q, parent->q3); }
    }
}

static void quad_tree_forces_descend(force_params_t *param, quad_tree_node_t *q) {
    if (q->num_items == 1) {
        quad_tree_forces_ascend(param, q);
    } else {
        if (q->q0 != NULL) { quad_tree_forces_descend(param, q->q0); }
        if (q->q1 != NULL) { quad_tree_forces_descend(param, q->q1); }
        if (q->q2 != NULL) { quad_tree_forces_descend(param, q->q2); }
        if (q->q3 != NULL) { quad_tree_forces_descend(param, q->q3); }
    }
}

static void quad_tree_node_forces_propagate(quad_tree_node_t *q, double fx, double fy) {
    if (q == NULL) {
    } else {
        fx *= q->mass;
        fy *= q->mass;
        fx += q->fx;
        fy += q->fy;

        if (q->num_items == 1) {
            ((paper_t*)q->item)->fx += fx;
            ((paper_t*)q->item)->fy += fy;
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

// descending then ascending is almost twice as fast (for large graphs) as
// just naively iterating through all the leaves, possibly due to cache effects
void quad_tree_forces(force_params_t *param, quad_tree_t *qt) {
    if (qt->root != NULL) {
        quad_tree_forces_descend(param, qt->root);
        quad_tree_node_forces_propagate(qt->root, 0, 0);
    }
}
