#ifndef _INCLUDED_COMMON_H
#define _INCLUDED_COMMON_H

typedef struct _paper_t {
    // stuff loaded from the DB
    unsigned int id;
    unsigned int maincat;
    short num_refs;
    short num_cites;
    struct _paper_t **refs;
    int index;

    // stuff for the placement of papers
    int kind;
    float r;
    float mass;
    float x;
    float y;
    float fx;
    float fy;
} paper_t;

#endif // _INCLUDED_COMMON_H
