#ifndef _INCLUDED_LAYOUT_H
#define _INCLUDED_LAYOUT_H

typedef struct _layout_link_t {
    float weight;
    struct _layout_t *layout;
} layout_link_t;

typedef struct _layout_t {
    struct _layout_t *parent;
    struct _layout_t *child1;
    struct _layout_t *child2;
    unsigned int num_links;
    layout_link_t *links;
    float mass;
    float x;
    float y;
    float fx;
    float fy;
} layout_t;

void build_layout_from_papers(int num_papers, paper_t **papers, int *num_layouts, layout_t **layouts);
void build_reduced_layout_from_layout(int num_layouts, layout_t *layouts, int *num_layouts2, layout_t **layouts2);

#endif // _INCLUDED_LAYOUT_H
