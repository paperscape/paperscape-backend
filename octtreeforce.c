#include <stdlib.h>
#include <assert.h>
#include <math.h>

#include "xiwilib.h"
#include "common.h"
#include "octtree.h"
#include "force.h"

// o1 is a leaf against which we check o2
static void oct_tree_node_forces2(force_params_t *param, oct_tree_node_t *o1, oct_tree_node_t *o2) {
    if (o2 == NULL) {
        // o2 is empty node
    } else {
        // o2 is leaf or internal node

        // compute distance from o1 to centroid of o2
        double dx = o1->x - o2->x;
        double dy = o1->y - o2->y;
        double dz = o1->z - o2->z;
        double rsq = dx * dx + dy * dy + dz * dz;
        if (rsq < 1e-6) {
            // minimum distance cut-off
            rsq = 1e-6;
        }

        if (o2->num_papers == 1) {
            // o2 is leaf node
            double fac;
            if (param->do_close_repulsion) {
                double rad_sum_sq = 1.4 * pow(o1->paper->r + o2->paper->r, 2);
                if (rsq < rad_sum_sq) {
                    // papers overlap, use stronger repulsive force
                    fac = fmin(200000, (exp(rad_sum_sq - rsq) - 1)) * 500 * fmax(1, pow(o1->mass * o2->mass, 3.0)) * param->anti_gravity_strength * pow(rsq, -1.5)
                        + o1->mass * o2->mass * param->anti_gravity_strength * pow(rad_sum_sq, -1.5);
                } else {
                    // normal anti-gravity repulsive force
                    fac = o1->mass * o2->mass * param->anti_gravity_strength * pow(rsq, -1.5);
                }
            } else {
                // normal anti-gravity repulsive force
                fac = o1->mass * o2->mass * param->anti_gravity_strength * pow(rsq, -1.5);
            }
            double fx = dx * fac;
            double fy = dy * fac;
            double fz = dz * fac;
            o1->fx += fx;
            o1->fy += fy;
            o1->fz += fz;
            o2->fx -= fx;
            o2->fy -= fy;
            o2->fz -= fz;

        } else {
            // o2 is internal node
            if (o2->side_length_x * o2->side_length_x + o2->side_length_y * o2->side_length_y + o2->side_length_z * o2->side_length_z < 1.35 * rsq) {
                // o1 and the cell o2 are "well separated"
                // approximate force by centroid of o2
                double fac = o1->mass * o2->mass * param->anti_gravity_strength * pow(rsq, -1.5);
                double fx = dx * fac;
                double fy = dy * fac;
                double fz = dz * fac;
                o1->fx += fx;
                o1->fy += fy;
                o1->fz += fz;
                o2->fx -= fx;
                o2->fy -= fy;
                o2->fz -= fz;

            } else {
                // o1 and o2 are not "well separated"
                // descend into children of o2
                for (int i = 0; i < 8; i++) {
                    oct_tree_node_forces2(param, o1, o2->o[i]);
                }
            }
        }
    }
}

static void oct_tree_node_forces1(force_params_t *param, oct_tree_node_t *o) {
    assert(o->num_papers == 1); // must be a leaf node
    for (oct_tree_node_t *o2 = o; o2->parent != NULL; o2 = o2->parent) {
        oct_tree_node_t *parent = o2->parent;
        assert(parent->num_papers > 1); // all parents should be internal nodes
        for (int i = 0; i < 8; i++) {
            if (parent->o[i] != o2) { oct_tree_node_forces2(param, o, parent->o[i]); }
        }
    }
}

static void oct_tree_node_forces0(force_params_t *param, oct_tree_node_t *o) {
    if (o == NULL) {
    } else if (o->num_papers == 1) {
        oct_tree_node_forces1(param, o);
    } else {
        for (int i = 0; i < 8; i++) {
            oct_tree_node_forces0(param, o->o[i]);
        }
    }
}

static void oct_tree_node_forces_propagate(force_params_t *param, oct_tree_node_t *o, double fx, double fy, double fz) {
    if (o == NULL) {
    } else {
        fx *= o->mass;
        fy *= o->mass;
        fz *= o->mass;
        fx += o->fx;
        fy += o->fy;
        fz += o->fz;

        if (o->num_papers == 1) {
            o->paper->fx += fx;
            o->paper->fy += fy;
            o->paper->fz += fz;
        } else {
            fx /= o->mass;
            fy /= o->mass;
            fz /= o->mass;
            for (int i = 0; i < 8; i++) {
                oct_tree_node_forces_propagate(param, o->o[i], fx, fy, fz);
            }
        }
    }
}

void oct_tree_forces(force_params_t *param, oct_tree_t *ot) {
    oct_tree_node_forces0(param, ot->root);
    oct_tree_node_forces_propagate(param, ot->root, 0, 0, 0);
}
