#include <stdio.h>
#include <stdlib.h>
#include <assert.h>
#include <math.h>

#include "util/xiwilib.h"
#include "layout.h"
#include "quadtree.h"

static quadtree_pool_t *quad_tree_pool_new(int alloc, quadtree_pool_t *next) {
    quadtree_pool_t *qtp = m_new(quadtree_pool_t, 1);
    qtp->num_nodes_alloc = alloc;
    qtp->num_nodes_used = 0;
    qtp->nodes = m_new(quadtree_node_t, qtp->num_nodes_alloc);
    qtp->next = next;
    return qtp;
}

static void quad_tree_pool_free_all(quadtree_pool_t *qtp) {
    for (; qtp != NULL; qtp = qtp->next) {
        qtp->num_nodes_used = 0;
    }
}

static quadtree_node_t *quad_tree_pool_alloc(quadtree_t *qt) {
    // look for a free node
    for (quadtree_pool_t *qtp = qt->quad_tree_pool; qtp != NULL; qtp = qtp->next) {
        if (qtp->num_nodes_used < qtp->num_nodes_alloc) {
            return &qtp->nodes[qtp->num_nodes_used++];
        }
    }
    // ran out of pre-allocated nodes, so allocate some more
    qt->quad_tree_pool = quad_tree_pool_new(qt->quad_tree_pool->num_nodes_alloc * 2, qt->quad_tree_pool);
    return &qt->quad_tree_pool->nodes[qt->quad_tree_pool->num_nodes_used++];
}

static void quad_tree_insert_layout_node(quadtree_t *qt, quadtree_node_t *parent, quadtree_node_t **q, layout_node_t *ln, double min_x, double min_y, double max_x, double max_y) {
    if (*q == NULL) {
        // hit an empty node; create a new leaf cell and put this layout-node in it
        *q = quad_tree_pool_alloc(qt);
        (*q)->parent = parent;
        (*q)->side_length = max_x - min_x;
        (*q)->num_items = 1;
        (*q)->mass = ln->mass;
        (*q)->x = ln->x;
        (*q)->y = ln->y;
        /*
        (*q)->fx = 0;
        (*q)->fy = 0;
        */
        (*q)->radius = ln->radius;
        (*q)->item = ln;

    } else if ((*q)->num_items == 1) {
        // hit a leaf; turn it into an internal node and re-insert the layout-nodes
        layout_node_t *ln0 = (*q)->item;
        (*q)->mass = 0;
        (*q)->x = 0;
        (*q)->y = 0;
        /*
        (*q)->fx = 0;
        (*q)->fy = 0;
        */
        (*q)->q0 = NULL;
        (*q)->q1 = NULL;
        (*q)->q2 = NULL;
        (*q)->q3 = NULL;
        (*q)->num_items = 0; // so it treats this node as an internal node
        quad_tree_insert_layout_node(qt, parent, q, ln0, min_x, min_y, max_x, max_y);
        (*q)->num_items = 0; // so it treats this node as an internal node
        quad_tree_insert_layout_node(qt, parent, q, ln, min_x, min_y, max_x, max_y);
        (*q)->num_items = 2; // we now have 2 layout-nodes in this node

    } else {
        // hit an internal node

        // check cell size didn't get too small
        if (fabs(min_x - max_x) < 1e-10 || fabs(min_y - max_y) < 1e-10) {
            printf("ERROR: quad_tree_insert hit minimum cell size; moving node by random amount\n");
            ln->x += 0.1 * ((double)random() / (double)RAND_MAX - 0.5);
            ln->y += 0.1 * ((double)random() / (double)RAND_MAX - 0.5);
            return;
        }

        // update centre of mass and mass of cell
        (*q)->num_items += 1;
        double new_mass = (*q)->mass + ln->mass;
        (*q)->x = ((*q)->mass * (*q)->x + ln->mass * ln->x) / new_mass;
        (*q)->y = ((*q)->mass * (*q)->y + ln->mass * ln->y) / new_mass;
        (*q)->mass = new_mass;

        // compute the dividing x and y positions
        double mid_x = 0.5 * (min_x + max_x);
        double mid_y = 0.5 * (min_y + max_y);

        // insert the new layout-node in the correct cell
        if (ln->y < mid_y) {
            if (ln->x < mid_x) {
                quad_tree_insert_layout_node(qt, *q, &(*q)->q0, ln, min_x, min_y, mid_x, mid_y);
            } else {
                quad_tree_insert_layout_node(qt, *q, &(*q)->q1, ln, mid_x, min_y, max_x, mid_y);
            }
        } else {
            if (ln->x < mid_x) {
                quad_tree_insert_layout_node(qt, *q, &(*q)->q2, ln, min_x, mid_y, mid_x, max_y);
            } else {
                quad_tree_insert_layout_node(qt, *q, &(*q)->q3, ln, mid_x, mid_y, max_x, max_y);
            }
        }
    }
}

quadtree_t *quadtree_new() {
    quadtree_t *qt = m_new(quadtree_t, 1);
    qt->quad_tree_pool = quad_tree_pool_new(1024, NULL);
    qt->root = NULL;
    return qt;
}

void quadtree_build(layout_t *layout, quadtree_t *qt) {
    qt->root = NULL;

    // if no nodes, return
    if (layout->num_nodes == 0) {
        qt->min_x = 0;
        qt->min_y = 0;
        qt->max_x = 0;
        qt->max_y = 0;
        return;
    }

    // first work out the bounding box of all nodes
    layout_node_t *n0 = &layout->nodes[0];
    qt->min_x = n0->x;
    qt->min_y = n0->y;
    qt->max_x = n0->x;
    qt->max_y = n0->y;
    for (int i = 1; i < layout->num_nodes; i++) {
        layout_node_t *n = &layout->nodes[i];
        if (n->x < qt->min_x) { qt->min_x = n->x; }
        if (n->y < qt->min_y) { qt->min_y = n->y; }
        if (n->x > qt->max_x) { qt->max_x = n->x; }
        if (n->y > qt->max_y) { qt->max_y = n->y; }
    }

    // increase the bounding box so it's square
    {
        double dx = qt->max_x - qt->min_x;
        double dy = qt->max_y - qt->min_y;
        if (dx > dy) {
            double cen_y = 0.5 * (qt->min_y + qt->max_y);
            qt->min_y = cen_y - 0.5 * dx;
            qt->max_y = cen_y + 0.5 * dx;
        } else {
            double cen_x = 0.5 * (qt->min_x + qt->max_x);
            qt->min_x = cen_x - 0.5 * dy;
            qt->max_x = cen_x + 0.5 * dy;
        }
        //printf("quad tree bounding box: (%f,%f) -- (%f,%f)\n", qt->min_x, qt->min_y, qt->max_x, qt->max_y);
    }

    if (!isfinite(qt->min_x) || !isfinite(qt->min_y) || !isfinite(qt->max_x) || !isfinite(qt->max_y)) {
        printf("ERROR: quad tree bounds are not finite; exiting\n");
        exit(1);
    }

    // build the quad tree
    quad_tree_pool_free_all(qt->quad_tree_pool);
    for (int i = 0; i < layout->num_nodes; i++) {
        quad_tree_insert_layout_node(qt, NULL, &qt->root, &layout->nodes[i], qt->min_x, qt->min_y, qt->max_x, qt->max_y);
    }
}
