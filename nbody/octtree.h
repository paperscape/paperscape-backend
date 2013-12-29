#ifndef _INCLUDED_OCTTREE_H
#define _INCLUDED_OCTTREE_H

typedef struct _oct_tree_node_t {
    struct _oct_tree_node_t *parent;
    float side_length_x;
    float side_length_y;
    float side_length_z;
    int num_papers; // if 1, this node is a leaf, else an internal node
    float mass;
    float x;
    float y;
    float z;
    float fx;
    float fy;
    float fz;
    union {
        struct {
            int depth;
            Common_paper_t *paper;
        };
        struct {
            struct _oct_tree_node_t *o[8];
        };
    };
} oct_tree_node_t;

typedef struct _oct_tree_t {
    double min_x;
    double min_y;
    double min_z;
    double max_x;
    double max_y;
    double max_z;
    oct_tree_node_t *root;
} oct_tree_t;

void oct_tree_build(int num_papers, Common_paper_t** papers, oct_tree_t *qt);

#endif // _INCLUDED_OCTTREE_H
