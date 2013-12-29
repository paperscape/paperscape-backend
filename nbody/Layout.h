#ifndef _INCLUDED_LAYOUT_H
#define _INCLUDED_LAYOUT_H

#include "Common.h"

// for Layout_node_t.flags (default state is unset)
#define LAYOUT_NODE_IS_FINEST   (0x0001)
#define LAYOUT_NODE_POS_VALID   (0x0002)
#define LAYOUT_NODE_HOLD_STILL  (0x0004)

struct _Common_paper_t;

typedef struct _Layout_node_t {
    unsigned int flags;
    struct _Layout_node_t *parent;
    union {
        struct {    // for when this layout is the finest layout
            struct _Common_paper_t *paper;
        };
        struct {    // for when this layout is coarse
            struct _Layout_node_t *child1;
            struct _Layout_node_t *child2;
        };
    };
    unsigned int num_links;
    struct _Layout_link_t *links;
    float mass;
    float radius;
    float x;
    float y;
    float fx;
    float fy;
} Layout_node_t;

typedef struct _Layout_link_t {
    float weight;
    Layout_node_t *node;
} Layout_link_t;

typedef struct _Layout_t {
    struct _Layout_t *parent_layout;
    struct _Layout_t *child_layout;
    int num_nodes;
    Layout_node_t *nodes;
    int num_links;
    Layout_link_t *links;
} Layout_t;

Layout_t *Layout_build_from_papers(int num_papers, struct _Common_paper_t **papers, bool age_weaken, double factor_ref_freq, double factor_other_link);
Layout_t *Layout_build_reduced_from_layout(Layout_t *layout);

void Layout_propagate_positions_to_children(Layout_t *layout);
void Layout_print(Layout_t *layout);
Layout_node_t *Layout_get_node_by_id(Layout_t *layout, int id);
Layout_node_t *Layout_get_node_at(Layout_t *layout, double x, double y);
void Layout_node_compute_best_start_position(Layout_node_t *n);
void Layout_rotate_all(Layout_t *layout, double angle);
void Layout_node_export_quantities(Layout_node_t *l, int *x_out, int *y_out, int *r_out);
void Layout_node_import_quantities(Layout_node_t *l, int x_in, int y_in);

#endif // _INCLUDED_LAYOUT_H
