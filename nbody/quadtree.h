#ifndef _INCLUDED_QUADTREE_H
#define _INCLUDED_QUADTREE_H

typedef struct _quad_tree_node_t {
    struct _quad_tree_node_t *parent;
    float side_length;  // cells are square
    int num_papers;     // if 1, this node is a leaf, else an internal node
    float mass;
    float x;
    float y;
    float fx;
    float fy;
    union {
        struct {
            int depth;
            paper_t *paper;
        };
        struct {
            struct _quad_tree_node_t *q0;
            struct _quad_tree_node_t *q1;
            struct _quad_tree_node_t *q2;
            struct _quad_tree_node_t *q3;
        };
    };
} quad_tree_node_t;

typedef struct _quad_tree_t {
    struct _quad_tree_pool_t *quad_tree_pool;
    double min_x;
    double min_y;
    double max_x;
    double max_y;
    quad_tree_node_t *root;
} quad_tree_t;

quad_tree_t *quad_tree_new();
void quad_tree_build(int num_papers, paper_t** papers, quad_tree_t *qt);

#endif // _INCLUDED_QUADTREE_H
