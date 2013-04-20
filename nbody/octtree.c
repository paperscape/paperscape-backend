#include <stdio.h>
#include <stdlib.h>
#include <assert.h>
#include <math.h>

#include "xiwilib.h"
#include "common.h"
#include "octtree.h"

typedef struct _oct_tree_pool_t {
    int num_nodes_alloc;
    int num_nodes_used;
    oct_tree_node_t *nodes;
    struct _oct_tree_pool_t *next;
} oct_tree_pool_t;

oct_tree_pool_t *oct_tree_pool_new(int alloc, oct_tree_pool_t *next) {
    oct_tree_pool_t *otp = m_new(oct_tree_pool_t, 1);
    otp->num_nodes_alloc = alloc;
    otp->num_nodes_used = 0;
    otp->nodes = m_new(oct_tree_node_t, otp->num_nodes_alloc);
    otp->next = next;
    return otp;
}

static oct_tree_pool_t *oct_tree_pool = NULL;

void oct_tree_pool_free_all() {
    for (oct_tree_pool_t *otp = oct_tree_pool; otp != NULL; otp = otp->next) {
        otp->num_nodes_used = 0;
    }
}

void oct_tree_pool_init() {
    if (oct_tree_pool == NULL) {
        oct_tree_pool = oct_tree_pool_new(1024, NULL);
    } else {
        oct_tree_pool_free_all();
    }
}

oct_tree_node_t *oct_tree_pool_alloc() {
    // look for a free node
    for (oct_tree_pool_t *otp = oct_tree_pool; otp != NULL; otp = otp->next) {
        if (otp->num_nodes_used < otp->num_nodes_alloc) {
            return &otp->nodes[otp->num_nodes_used++];
        }
    }
    // ran out of pre-allocated nodes, so allocate some more
    oct_tree_pool = oct_tree_pool_new(oct_tree_pool->num_nodes_alloc * 2, oct_tree_pool);
    return &oct_tree_pool->nodes[oct_tree_pool->num_nodes_used++];
}

void oct_tree_insert_paper(oct_tree_node_t *parent, oct_tree_node_t **o, paper_t *p, int depth, double min_x, double min_y, double min_z, double max_x, double max_y, double max_z) {
    if (*o == NULL) {
        // hit an empty node; create a new leaf cell and put this paper in it
        *o = oct_tree_pool_alloc();
        (*o)->parent = parent;
        (*o)->side_length_x = max_x - min_x;
        (*o)->side_length_y = max_y - min_y;
        (*o)->side_length_z = max_z - min_z;
        (*o)->num_papers = 1;
        (*o)->mass = p->mass;
        (*o)->x = p->x;
        (*o)->y = p->y;
        (*o)->z = p->z;
        (*o)->fx = 0;
        (*o)->fy = 0;
        (*o)->fz = 0;
        (*o)->depth = depth;
        (*o)->paper = p;

    } else if ((*o)->num_papers == 1) {
        // hit a leaf; turn it into an internal node and re-insert the papers
        paper_t *p0 = (*o)->paper;
        (*o)->mass = 0;
        (*o)->x = 0;
        (*o)->y = 0;
        (*o)->z = 0;
        (*o)->fx = 0;
        (*o)->fy = 0;
        (*o)->fz = 0;
        for (int i = 0; i < 8; i++) {
            (*o)->o[i] = NULL;
        }
        (*o)->num_papers = 0; // so it treats this node as an internal node
        oct_tree_insert_paper(parent, o, p0, depth, min_x, min_y, min_z, max_x, max_y, max_z);
        (*o)->num_papers = 0; // so it treats this node as an internal node
        oct_tree_insert_paper(parent, o, p, depth, min_x, min_y, min_z, max_x, max_y, max_z);
        (*o)->num_papers = 2; // we now have 2 papers in this node

    } else {
        // hit an internal node

        // update centre of mass and mass of cell
        (*o)->num_papers += 1;
        double new_mass = (*o)->mass + p->mass;
        (*o)->x = ((*o)->mass * (*o)->x + p->mass * p->x) / new_mass;
        (*o)->y = ((*o)->mass * (*o)->y + p->mass * p->y) / new_mass;
        (*o)->z = ((*o)->mass * (*o)->z + p->mass * p->z) / new_mass;
        (*o)->mass = new_mass;

        // check cell size didn't get too small
        if (fabs(min_x - max_x) < 1e-10 || fabs(min_y - max_y) < 1e-10 || fabs(min_z - max_z) < 1e-10) {
            printf("ERROR: oct_tree_insert hit minimum cell size\n");
            return;
        }

        // compute the dividing x, y and y positions
        double mid_x = 0.5 * (min_x + max_x);
        double mid_y = 0.5 * (min_y + max_y);
        double mid_z = 0.5 * (min_z + max_z);

        // work out the cell and the bounding box
        int cell_index = 0;
        if (p->x < mid_x) {
            max_x = mid_x;
        } else {
            cell_index |= 1;
            min_x = mid_x;
        }
        if (p->y < mid_y) {
            max_y = mid_y;
        } else {
            cell_index |= 2;
            min_y = mid_y;
        }
        if (p->z < mid_z) {
            max_z = mid_z;
        } else {
            cell_index |= 4;
            min_z = mid_z;
        }

        // insert the new paper in the correct cell
        oct_tree_insert_paper(*o, &(*o)->o[cell_index], p, depth + 1, min_x, min_y, min_z, max_x, max_y, max_z);
    }
}

void oct_tree_build(int num_papers, paper_t** papers, oct_tree_t *ot) {
    ot->root = NULL;

    // if no papers, return
    if (num_papers == 0) {
        ot->min_x = 0;
        ot->min_y = 0;
        ot->min_z = 0;
        ot->max_x = 0;
        ot->max_y = 0;
        ot->max_z = 0;
        return;
    }

    // first work out the bounding box of all papers
    paper_t *p0 = papers[0];
    ot->min_x = p0->x;
    ot->min_y = p0->y;
    ot->min_z = p0->z;
    ot->max_x = p0->x;
    ot->max_y = p0->y;
    ot->max_z = p0->z;
    for (int i = 1; i < num_papers; i++) {
        paper_t *p = papers[i];
        if (p->x < ot->min_x) { ot->min_x = p->x; }
        if (p->y < ot->min_y) { ot->min_y = p->y; }
        if (p->z < ot->min_z) { ot->min_z = p->z; }
        if (p->x > ot->max_x) { ot->max_x = p->x; }
        if (p->y > ot->max_y) { ot->max_y = p->y; }
        if (p->z > ot->max_z) { ot->max_z = p->z; }
    }

    // build the oct tree
    oct_tree_pool_init();
    for (int i = 0; i < num_papers; i++) {
        paper_t *p = papers[i];
        oct_tree_insert_paper(NULL, &ot->root, p, 0, ot->min_x, ot->min_y, ot->min_z, ot->max_x, ot->max_y, ot->max_z);
    }
}
