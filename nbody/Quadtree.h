#ifndef _INCLUDED_QUADTREE_H
#define _INCLUDED_QUADTREE_H

typedef struct _Quadtree_node_t {
    struct _Quadtree_node_t *parent;
    float side_length;      // cells are square
    int num_items;          // if 1, this node is a leaf, else an internal node
    float mass;             // total mass of this cell (sum of all children)
    float x;                // centre of mass
    float y;                // centre of mass
    /* OBSOLETE
    float fx;               // net force on this cell
    float fy;               // net force on this cell
    */
    union {
        struct {            // for a leaf
            float radius;   // radius of item
            void *item;     // pointer to actual item
        };
        struct {            // for an internal node
            struct _Quadtree_node_t *q0;
            struct _Quadtree_node_t *q1;
            struct _Quadtree_node_t *q2;
            struct _Quadtree_node_t *q3;
        };
    };
} Quadtree_node_t;

typedef struct _Quadtree_pool_t {
    int num_nodes_alloc;
    int num_nodes_used;
    Quadtree_node_t *nodes;
    struct _Quadtree_pool_t *next;
} Quadtree_pool_t;

typedef struct _Quadtree_t {
    struct _Quadtree_pool_t *quad_tree_pool;
    double min_x;
    double min_y;
    double max_x;
    double max_y;
    Quadtree_node_t *root;
} Quadtree_t;

Quadtree_t *Quadtree_new();
void Quadtree_build(Layout_t *layout, Quadtree_t *qt);

#endif // _INCLUDED_QUADTREE_H
