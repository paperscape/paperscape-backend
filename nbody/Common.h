#ifndef _INCLUDED_COMMON_H
#define _INCLUDED_COMMON_H

// uncomment this to enable tredding option
//#define ENABLE_TRED (1)

typedef enum {
    CAT_UNKNOWN = 0,
    CAT_INSPIRE = 1,
#   define CAT(id, str) CAT_##id,
#   include "cats.h"
#   undef CAT
    CAT_NUMBER_OF,
} Common_category_t;

#define COMMON_PAPER_MAX_CATS (4)

typedef struct _Common_paper_t {
    // stuff loaded from the DB
    unsigned int id;
    byte allcats[COMMON_PAPER_MAX_CATS]; // store fixed number of categories; more efficient than having a tiny, dynamic array; unused entries are CAT_UNKNOWN
    short num_refs;
    short num_cites;
    struct _Common_paper_t **refs;     // array of referenced/linked papers
    byte *refs_ref_freq;        // ref_freq weight of corresponding ref
    float *refs_other_weight;   // other weight (eg ScienceWise data) of corresponding ref
    struct _Common_paper_t **cites;
    int index;
    const char *authors;
    const char *title;

    int num_keywords;
    struct _Common_keyword_t **keywords;

    // stuff for colouring
    int colour;
    int num_with_my_colour;

    // stuff for connecting disconnected papers
    int num_fake_links;
    struct _Common_paper_t **fake_links;

    // stuff for tred
    int tred_visit_index;
    int *refs_tred_computed;
    struct _Common_paper_t *tred_follow_back_paper;
    int tred_follow_back_ref;

    // stuff for the placement of papers
    bool included;
    int num_included_cites;
    bool connected;
    float age; // between 0.0 and 1.0
    float radius;
    float mass;

    struct _Layout_node_t *layout_node;
} Common_paper_t;

typedef struct _Common_keyword_t {
    char *keyword;      // the keyword
    Common_paper_t *paper;     // for general use
} Common_keyword_t;

typedef struct _keyword_set_t Common_keyword_set_t;

void Common_paper_init(Common_paper_t *p, unsigned int id);

const char *Common_category_enum_to_str(Common_category_t cat);
Common_category_t category_str_to_enum(const char *str);
Common_category_t category_strn_to_enum(const char *str, size_t n);

Common_keyword_set_t *keyword_set_new();
void Common_keyword_set_free(Common_keyword_set_t *kws);
int Common_keyword_set_get_total(Common_keyword_set_t *kws);
void Common_keyword_set_clear_data(Common_keyword_set_t *kws);
Common_keyword_t *keyword_set_lookup_or_insert(Common_keyword_set_t *kws, const char *kw, size_t kw_len);

int Common_date_to_unique_id(int y, int m, int d);
void Common_unique_id_to_date(int id, int *y, int *m, int *d);

bool Common_build_citation_links(int num_papers, Common_paper_t *papers);
void Common_recompute_num_included_cites(int num_papers, Common_paper_t *papers);
void Common_recompute_colours(int num_papers, Common_paper_t *papers, int verbose);
void Common_compute_tred(int num_papers, Common_paper_t *papers);

#endif // _INCLUDED_COMMON_H
