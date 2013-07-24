// for layout_node_t.flags (default state is unset)
#define LAYOUT_NODE_IS_FINEST   (0x0001)
#define LAYOUT_NODE_POS_VALID   (0x0002)
#define LAYOUT_NODE_HOLD_STILL  (0x0004)

typedef struct _layout_node_t {
    unsigned int flags;
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
    unsigned int num_links;
    struct _layout_link_t *links;
    float mass;
    float radius;
    float x;
    float y;
    float fx;
    float fy;
} layout_node_t;

typedef struct _layout_link_t {
    float weight;
    layout_node_t *node;
} layout_link_t;

typedef struct _layout_t {
    struct _layout_t *parent_layout;
    struct _layout_t *child_layout;
    int num_nodes;
    layout_node_t *nodes;
    int num_links;
    layout_link_t *links;
} layout_t;

struct _paper_t;

layout_t *build_layout_from_papers(int num_papers, struct _paper_t **papers, bool age_weaken);
layout_t *build_reduced_layout_from_layout(layout_t *layout);
void layout_propagate_positions_to_children(layout_t *layout);
void layout_print(layout_t *layout);
layout_node_t *layout_get_node_by_id(layout_t *layout, int id);
layout_node_t *layout_get_node_at(layout_t *layout, double x, double y);
void layout_node_compute_best_start_position(layout_node_t *n);
void layout_rotate_all(layout_t *layout, double angle);
void layout_node_export_quantities(layout_node_t *l, int *x_out, int *y_out, int *r_out);
void layout_node_import_quantities(layout_node_t *l, int x_in, int y_in);
