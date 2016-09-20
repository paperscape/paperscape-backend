#ifndef _INCLUDED_LAYOUT_H
#define _INCLUDED_LAYOUT_H

#include <stdint.h>
#include "common.h"

// for layout_node_t.flags (default state is unset)
#define LAYOUT_NODE_IS_FINEST   (0x0001)
#define LAYOUT_NODE_POS_VALID   (0x0002)
#define LAYOUT_NODE_HOLD_STILL  (0x0004)

struct _paper_t;

typedef struct _layout_node_t {
    unsigned int flags;
    unsigned int num_links;
    struct _layout_node_t *parent;
    union {
        struct {    // for when this layout is the finest layout
            struct _paper_t *paper;
        };
        struct {    // for when this layout is coarse
            struct _layout_node_t *child1;
            struct _layout_node_t *child2;
        };
    };
    struct _layout_link_t *links;
    float mass;
    float radius;
    float x;
    float y;
    float fx;
    float fy;
} layout_node_t;

// with this option enabled on a 64-bit machine the layout links use half the RAM
#define LAYOUT_USE_COMPRESSED_LINKS 0

#if LAYOUT_USE_COMPRESSED_LINKS
// assumptions for this option to work:
//  - 64-bit machine
//  - layout_node_t pointer is always 8-byte aligned
//  - addresses fit in 47 bits
//  - weights are integers below 1024
#define LAYOUT_LINK_GET_WEIGHT(l) ((float)((l)->data >> 44))
#define LAYOUT_LINK_GET_NODE(l) ((layout_node_t*)(((l)->data & 0x00000fffffffffff) << 3))
#define LAYOUT_LINK_SET_WEIGHT(l, w) do { (l)->data = ((l)->data & 0x00000fffffffffff) | (((uint64_t)truncf(w)) << 44); } while (0)
#define LAYOUT_LINK_SET_NODE(l, n) do { assert(((uint64_t)n & 0xffff800000000007) == 0); (l)->data = ((l)->data & 0xfffff00000000000) | ((uint64_t)n >> 3 & 0xfffffffffff); } while (0)
#else
#define LAYOUT_LINK_GET_WEIGHT(l) ((l)->weight)
#define LAYOUT_LINK_GET_NODE(l) ((l)->node)
#define LAYOUT_LINK_SET_WEIGHT(l, w) do { (l)->weight = w; } while (0)
#define LAYOUT_LINK_SET_NODE(l, n) do { (l)->node = n; } while (0)
#endif

typedef struct _layout_link_t {
    #if LAYOUT_USE_COMPRESSED_LINKS
    uint64_t data;
    #else
    float weight;
    layout_node_t *node;
    #endif
} layout_link_t;

typedef struct _layout_t {
    struct _layout_t *parent_layout;
    struct _layout_t *child_layout;
    int num_nodes;
    layout_node_t *nodes;
    int num_links;
    layout_link_t *links;
} layout_t;

layout_t *layout_build_from_papers(int num_papers, struct _paper_t **papers, bool age_weaken, double factor_ref_freq, double factor_other_link);
layout_t *layout_build_reduced_from_layout(layout_t *layout);

void layout_propagate_positions_to_children(layout_t *layout);
void layout_print(layout_t *layout);
layout_node_t *layout_get_node_by_id(layout_t *layout, unsigned int id);
layout_node_t *layout_get_node_at(layout_t *layout, double x, double y);
void layout_node_compute_best_start_position(layout_node_t *n);
void layout_rotate_all(layout_t *layout, double angle);
void layout_node_export_quantities(layout_node_t *l, int *x_out, int *y_out, int *r_out);
void layout_node_import_quantities(layout_node_t *l, int x_in, int y_in);
void layout_recompute_mass_radius(layout_t *layout);

#endif // _INCLUDED_LAYOUT_H
