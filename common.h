#ifndef _INCLUDED_COMMON_H
#define _INCLUDED_COMMON_H

typedef struct _paper_t {
    // stuff loaded from the DB
    unsigned int id;
    unsigned int maincat;
    short num_refs;
    short num_cites;
    struct _paper_t **refs;
    struct _paper_t **cites;
    int index;
    const char *authors;
    const char *title;

    // stuff for colouring
    int colour;
    int num_with_my_colour;

    // stuff for tred
    int tred_visit_index;
    int *refs_tred_computed;

    // stuff for the placement of papers
    int kind;
    float r;
    float mass;
    float x;
    float y;
    float fx;
    float fy;
} paper_t;

void recompute_num_cites(int num_papers, paper_t *papers);
void recompute_colours(int num_papers, paper_t *papers, int verbose);
void compute_tred(int num_papers, paper_t *papers);

#endif // _INCLUDED_COMMON_H
