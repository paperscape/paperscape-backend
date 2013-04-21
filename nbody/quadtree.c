#include <stdio.h>
#include <stdlib.h>
#include <assert.h>
#include <math.h>

#include "xiwilib.h"
#include "common.h"
#include "quadtree.h"

typedef struct _quad_tree_pool_t {
    int num_nodes_alloc;
    int num_nodes_used;
    quad_tree_node_t *nodes;
    struct _quad_tree_pool_t *next;
} quad_tree_pool_t;

quad_tree_pool_t *quad_tree_pool_new(int alloc, quad_tree_pool_t *next) {
    quad_tree_pool_t *qtp = m_new(quad_tree_pool_t, 1);
    qtp->num_nodes_alloc = alloc;
    qtp->num_nodes_used = 0;
    qtp->nodes = m_new(quad_tree_node_t, qtp->num_nodes_alloc);
    qtp->next = next;
    return qtp;
}

static quad_tree_pool_t *quad_tree_pool = NULL;

void quad_tree_pool_free_all() {
    for (quad_tree_pool_t *qtp = quad_tree_pool; qtp != NULL; qtp = qtp->next) {
        qtp->num_nodes_used = 0;
    }
}

void quad_tree_pool_init() {
    if (quad_tree_pool == NULL) {
        quad_tree_pool = quad_tree_pool_new(1024, NULL);
    } else {
        quad_tree_pool_free_all();
    }
}

quad_tree_node_t *quad_tree_pool_alloc() {
    // look for a free node
    for (quad_tree_pool_t *qtp = quad_tree_pool; qtp != NULL; qtp = qtp->next) {
        if (qtp->num_nodes_used < qtp->num_nodes_alloc) {
            return &qtp->nodes[qtp->num_nodes_used++];
        }
    }
    // ran out of pre-allocated nodes, so allocate some more
    quad_tree_pool = quad_tree_pool_new(quad_tree_pool->num_nodes_alloc * 2, quad_tree_pool);
    return &quad_tree_pool->nodes[quad_tree_pool->num_nodes_used++];
}

void quad_tree_insert_paper(quad_tree_node_t *parent, quad_tree_node_t **q, paper_t *p, int depth, double min_x, double min_y, double max_x, double max_y) {
    if (*q == NULL) {
        // hit an empty node; create a new leaf cell and put this paper in it
        *q = quad_tree_pool_alloc();
        (*q)->parent = parent;
        (*q)->side_length_x = max_x - min_x;
        (*q)->side_length_y = max_y - min_y;
        (*q)->num_papers = 1;
        (*q)->mass = p->mass;
        (*q)->x = p->x;
        (*q)->y = p->y;
        (*q)->fx = 0;
        (*q)->fy = 0;
        (*q)->depth = depth;
        (*q)->paper = p;

    } else if ((*q)->num_papers == 1) {
        // hit a leaf; turn it into an internal node and re-insert the papers
        paper_t *p0 = (*q)->paper;
        (*q)->mass = 0;
        (*q)->x = 0;
        (*q)->y = 0;
        (*q)->fx = 0;
        (*q)->fy = 0;
        (*q)->q0 = NULL;
        (*q)->q1 = NULL;
        (*q)->q2 = NULL;
        (*q)->q3 = NULL;
        (*q)->num_papers = 0; // so it treats this node as an internal node
        quad_tree_insert_paper(parent, q, p0, depth, min_x, min_y, max_x, max_y);
        (*q)->num_papers = 0; // so it treats this node as an internal node
        quad_tree_insert_paper(parent, q, p, depth, min_x, min_y, max_x, max_y);
        (*q)->num_papers = 2; // we now have 2 papers in this node

    } else {
        // hit an internal node

        // update centre of mass and mass of cell
        (*q)->num_papers += 1;
        double new_mass = (*q)->mass + p->mass;
        (*q)->x = ((*q)->mass * (*q)->x + p->mass * p->x) / new_mass;
        (*q)->y = ((*q)->mass * (*q)->y + p->mass * p->y) / new_mass;
        (*q)->mass = new_mass;

        // check cell size didn't get too small
        if (fabs(min_x - max_x) < 1e-10 || fabs(min_y - max_y) < 1e-10) {
            printf("ERROR: quad_tree_insert hit minimum cell size\n");
            return;
        }

        // compute the dividing x and y positions
        double mid_x = 0.5 * (min_x + max_x);
        double mid_y = 0.5 * (min_y + max_y);

        // insert the new paper in the correct cell
        if (p->y < mid_y) {
            if (p->x < mid_x) {
                quad_tree_insert_paper(*q, &(*q)->q0, p, depth + 1, min_x, min_y, mid_x, mid_y);
            } else {
                quad_tree_insert_paper(*q, &(*q)->q1, p, depth + 1, mid_x, min_y, max_x, mid_y);
            }
        } else {
            if (p->x < mid_x) {
                quad_tree_insert_paper(*q, &(*q)->q2, p, depth + 1, min_x, mid_y, mid_x, max_y);
            } else {
                quad_tree_insert_paper(*q, &(*q)->q3, p, depth + 1, mid_x, mid_y, max_x, max_y);
            }
        }
    }
}

void quad_tree_build(int num_papers, paper_t** papers, quad_tree_t *qt) {
    qt->root = NULL;

    // if no papers, return
    if (num_papers == 0) {
        qt->min_x = 0;
        qt->min_y = 0;
        qt->max_x = 0;
        qt->max_y = 0;
        return;
    }

    // first work out the bounding box of all papers
    paper_t *p0 = papers[0];
    qt->min_x = p0->x;
    qt->min_y = p0->y;
    qt->max_x = p0->x;
    qt->max_y = p0->y;
    for (int i = 1; i < num_papers; i++) {
        paper_t *p = papers[i];
        if (p->x < qt->min_x) { qt->min_x = p->x; }
        if (p->y < qt->min_y) { qt->min_y = p->y; }
        if (p->x > qt->max_x) { qt->max_x = p->x; }
        if (p->y > qt->max_y) { qt->max_y = p->y; }
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

    // build the quad tree
    quad_tree_pool_init();
    for (int i = 0; i < num_papers; i++) {
        paper_t *p = papers[i];
        quad_tree_insert_paper(NULL, &qt->root, p, 0, qt->min_x, qt->min_y, qt->max_x, qt->max_y);
    }
}
