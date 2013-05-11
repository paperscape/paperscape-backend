#ifndef _INCLUDED_COMMON_H
#define _INCLUDED_COMMON_H

typedef struct _paper_t {
    // stuff loaded from the DB
    unsigned int id;
    unsigned int maincat;
    short num_refs;
    short num_cites;
    struct _paper_t **refs;
    byte *refs_ref_freq;
    struct _paper_t **cites;
    int index;
    const char *authors;
    const char *title;

    int num_keywords;
    struct _keyword_t **keywords;

    bool pos_valid;
    float x;
    float y;
    float z;

    // stuff for colouring
    int colour;
    int num_with_my_colour;

    // stuff for tred
    int tred_visit_index;
    int *refs_tred_computed;
    struct _paper_t *tred_follow_back_paper;
    int tred_follow_back_ref;

    // stuff for the placement of papers
    bool included;
    int num_included_cites;
    int kind;
    float age; // between 0.0 and 1.0
    float r;
    float mass;
    float fx;
    float fy;
    float fz;

    struct _layout_t *layout;
} paper_t;

typedef struct _keyword_t {
    char *keyword;
    int num_papers; // number of papers with this keyword
    float x;
    float y;
} keyword_t;

int date_to_unique_id(int y, int m, int d);
void unique_id_to_date(int id, int *y, int *m, int *d);
void recompute_num_included_cites(int num_papers, paper_t *papers);
void recompute_colours(int num_papers, paper_t *papers, int verbose);
void compute_tred(int num_papers, paper_t *papers);

#endif // _INCLUDED_COMMON_H
