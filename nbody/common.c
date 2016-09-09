#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <assert.h>

#include "util/xiwilib.h"
#include "common.h"
#include "util/jsmnenv.h"

bool config_new(const char *filename, init_config_t **config) {
    // create new config
    *config = m_new(init_config_t,1);

    // set up jsmn_environment
    jsmn_env_t jsmn_env;
    if (!jsmn_env_set_up(&jsmn_env, filename)) {
        jsmn_env_finish(&jsmn_env);
        return false;
    }

    // load our data
    if (!jsmn_env_open_json_file(&jsmn_env, filename)) {
        return false;
    }

    bool more_objects;
    if (!jsmn_env_next_object(&jsmn_env, &more_objects)) {
        return false;
    }
    if (more_objects) {
        return false;
    }

    // look for member: description
    jsmn_env_token_value_t descr_val;
    if (!jsmn_env_get_object_member(&jsmn_env, jsmn_env.js_tok, "description", NULL, &descr_val)
        || descr_val.kind != JSMN_VALUE_STRING) {
        return false;
    }
    printf("Reading in settings for: %s\n",descr_val.str);

    // look for member: ids_time_ordered
    jsmn_env_token_value_t ito_val;
    jsmn_env_get_object_member(&jsmn_env, jsmn_env.js_tok, "ids_time_ordered", NULL, &ito_val);
    if (ito_val.kind != JSMN_VALUE_NULL && ito_val.kind == JSMN_VALUE_TRUE) {
        (*config)->ids_time_ordered = true;
    }

    // ### look for member: forces
    jsmn_env_token_value_t forces_val;
    jsmntok_t *forces_tok;
    if(!jsmn_env_get_object_member(&jsmn_env, jsmn_env.js_tok, "forces", &forces_tok, &forces_val) 
        || forces_val.kind != JSMN_VALUE_OBJECT) {
        return false;
    }
    jsmn_env_token_value_t cr_a_val, cr_b_val, cr_c_val, cr_d_val, link_val, anti_grav_val;
    if(!jsmn_env_get_object_member(&jsmn_env, forces_tok, "close_repulsion_a", NULL, &cr_a_val)
        || cr_a_val.kind != JSMN_VALUE_REAL) {
        return false;
    }
    if(!jsmn_env_get_object_member(&jsmn_env, forces_tok, "close_repulsion_b", NULL, &cr_b_val)
        || cr_b_val.kind != JSMN_VALUE_REAL) {
        return false;
    }
    if(!jsmn_env_get_object_member(&jsmn_env, forces_tok, "close_repulsion_c", NULL, &cr_c_val)
        || cr_c_val.kind != JSMN_VALUE_REAL) {
        return false;
    }
    if(!jsmn_env_get_object_member(&jsmn_env, forces_tok, "close_repulsion_d", NULL, &cr_d_val)
        || cr_d_val.kind != JSMN_VALUE_REAL) {
        return false;
    }
    if(!jsmn_env_get_object_member(&jsmn_env, forces_tok, "link_strength", NULL, &link_val)
        || link_val.kind != JSMN_VALUE_REAL) {
        return false;
    }
    if(!jsmn_env_get_object_member(&jsmn_env, forces_tok, "anti_gravity_falloff_rsq", NULL, &anti_grav_val)
        || anti_grav_val.kind != JSMN_VALUE_REAL) {
        return false;
    }
    (*config)->force_close_repulsion_a = cr_a_val.real;
    (*config)->force_close_repulsion_b = cr_b_val.real;
    (*config)->force_close_repulsion_c = cr_c_val.real;
    (*config)->force_close_repulsion_d = cr_d_val.real;
    (*config)->force_link_strength = link_val.real;
    (*config)->force_anti_gravity_falloff_rsq = anti_grav_val.real;

    // ### look for member: refsblob
    jsmn_env_token_value_t refsblob_val;
    jsmntok_t *refsblob_tok;
    if(!jsmn_env_get_object_member(&jsmn_env, jsmn_env.js_tok, "refsblob", &refsblob_tok, &refsblob_val) 
        || refsblob_val.kind != JSMN_VALUE_OBJECT) {
        return false;
    }
    jsmn_env_token_value_t ref_freq_val, ref_order_val, ref_cites_val;
    if(!jsmn_env_get_object_member(&jsmn_env, refsblob_tok, "ref_order", NULL, &ref_order_val)
        || (ref_order_val.kind != JSMN_VALUE_TRUE && ref_order_val.kind != JSMN_VALUE_FALSE)) {
        return false;
    }
    if(!jsmn_env_get_object_member(&jsmn_env, refsblob_tok, "ref_freq", NULL, &ref_freq_val)
        || (ref_freq_val.kind != JSMN_VALUE_TRUE && ref_freq_val.kind != JSMN_VALUE_FALSE)) {
        return false;
    }
    if(!jsmn_env_get_object_member(&jsmn_env, refsblob_tok, "ref_cites", NULL, &ref_cites_val)
        || (ref_cites_val.kind != JSMN_VALUE_TRUE && ref_cites_val.kind != JSMN_VALUE_FALSE)) {
        return false;
    }
    (*config)->refsblob_ref_order = (ref_order_val.kind == JSMN_VALUE_TRUE);
    (*config)->refsblob_ref_freq  = (ref_freq_val.kind  == JSMN_VALUE_TRUE);
    (*config)->refsblob_ref_cites = (ref_cites_val.kind == JSMN_VALUE_TRUE);

    // ### look for member: query_extra_clause
    jsmn_env_token_value_t query_val;
    if(!jsmn_env_get_object_member(&jsmn_env, jsmn_env.js_tok, "query_extra_clause", NULL, &query_val) 
        || query_val.kind != JSMN_VALUE_STRING) {
        return false;
    }
    (*config)->query_extra_clause = query_val.str;

    // finish up
    jsmn_env_finish(&jsmn_env);

    return true;
}

void paper_init(paper_t *p, unsigned int id) {
    // all entries have initial state which is 0x00
    memset(p, 0, sizeof(paper_t));
    // set the paper id
    p->id = id;
}

static const char *category_string[] = {
    "unknown",
    "inspire",
#define CAT(id, str) str,
#include "cats.h"
#undef CAT
};

unsigned int date_to_unique_id(int y, int m, int d) {
    return ((unsigned int)y - 1800) * 10000000 + (unsigned int)m * 625000 + (unsigned int)d * 15625;
}

void unique_id_to_date(unsigned int id, int *y, int *m, int *d) {
    *y = id / 10000000 + 1800;
    *m = ((id % 10000000) / 625000) + 1;
    *d = ((id % 625000) / 15625) + 1;
}

const char *category_enum_to_str(category_t cat) {
    return category_string[cat];
}

category_t category_str_to_enum(const char *str) {
    for (int i = 0; i < CAT_NUMBER_OF; i++) {
        if (streq(category_string[i], str)) {
            return i;
        }
    }
    return CAT_UNKNOWN;
}

category_t category_strn_to_enum(const char *str, size_t n) {
    for (int i = 0; i < CAT_NUMBER_OF; i++) {
        if (strncmp(category_string[i], str, n) == 0 && category_string[i][n] == '\0') {
            return i;
        }
    }
    return CAT_UNKNOWN;
}

// compute the citations from the references
// allocates memory for paper->cites and fills it with pointers to citing papers
bool build_citation_links(int num_papers, paper_t *papers) {
    printf("building citation links\n");

    // allocate memory for cites for each paper
    for (int i = 0; i < num_papers; i++) {
        paper_t *paper = &papers[i];
        if (paper->num_cites > 0) {
            paper->cites = m_new(paper_t*, paper->num_cites);
            if (paper->cites == NULL) {
                return false;
            }
        }
        // use num cites to count which entry in the array we are up to when inserting cite links
        paper->num_cites = 0;
    }

    // link the cites
    for (int i = 0; i < num_papers; i++) {
        paper_t *paper = &papers[i];
        for (int j = 0; j < paper->num_refs; j++) {
            paper_t *ref_paper = paper->refs[j];
            ref_paper->cites[ref_paper->num_cites++] = paper;
        }
    }

    return true;
}

// compute the num_included_cites field in the paper_t objects
// only includes papers that have their "included" flag set
// only counts references that have non-zero ref_freq
void recompute_num_included_cites(int num_papers, paper_t *papers) {
    // reset citation count
    for (int i = 0; i < num_papers; i++) {
        paper_t *p = &papers[i];
        p->num_included_cites = 0;
    }

    // compute citation count by following references
    for (int i = 0; i < num_papers; i++) {
        paper_t *p = &papers[i];
        if (p->included) {
            for (int j = 0; j < p->num_refs; j++) {
                if (p->refs_ref_freq[j] > 0) {
                    paper_t *p2 = p->refs[j];
                    if (p2->included) {
                        p2->num_included_cites += 1;
                    }
                }
            }
        }
    }
}

typedef struct _paper_stack_t {
    int alloc;
    int used;
    paper_t **stack;
} paper_stack_t;

static paper_stack_t *paper_stack_new() {
    paper_stack_t *s = m_new(paper_stack_t, 1);
    s->alloc = 1024;
    s->used = 0;
    s->stack = m_new(paper_t*, s->alloc);
    return s;
}

static void paper_stack_free(paper_stack_t *s) {
    m_free(s->stack);
    m_free(s);
}

static void paper_stack_push(paper_stack_t *s, paper_t *p) {
    if (s->used >= s->alloc) {
        s->alloc *= 2;
        s->stack = m_renew(paper_t*, s->stack, s->alloc);
    }
    s->stack[s->used++] = p;
}

static paper_t *paper_stack_pop(paper_stack_t *s) {
    assert(s->used > 0);
    return s->stack[--s->used];
}

static void paper_paint(paper_t *p, int colour, paper_stack_t *stack) {
    assert(p->colour == 0);
    p->colour = colour;
    paper_stack_push(stack, p);
    while (stack->used > 0) {
        p = paper_stack_pop(stack);
        assert(p->colour == colour);
        for (int i = 0; i < p->num_refs; i++) {
            paper_t *p2 = p->refs[i];
            if (p2->included && p2->colour != colour) {
                assert(p2->colour == 0);
                p2->colour = colour;
                paper_stack_push(stack, p2);
            }
        }
        for (int i = 0; i < p->num_cites; i++) {
            paper_t *p2 = p->cites[i];
            if (p2->included && p2->colour != colour) {
                assert(p2->colour == 0);
                p2->colour = colour;
                paper_stack_push(stack, p2);
            }
        }
    }
}

// works out connected class for each paper (the colour after a flood fill painting algorigth)
// only includes papers that have their "included" flag set
void recompute_colours(int num_papers, paper_t *papers, int verbose) {
    // clear colour
    for (int i = 0; i < num_papers; i++) {
        papers[i].colour = 0;
    }

    // assign colour
    int cur_colour = 1;
    paper_stack_t *paper_stack = paper_stack_new();
    for (int i = 0; i < num_papers; i++) {
        paper_t *paper = &papers[i];
        if (paper->included && paper->colour == 0) {
            paper_paint(paper, cur_colour++, paper_stack);
        }
    }
    paper_stack_free(paper_stack);

    // compute and assign num_with_my_colour for each paper
    int *num_with_col = m_new0(int, cur_colour);
    for (int i = 0; i < num_papers; i++) {
        num_with_col[papers[i].colour] += 1;
    }
    for (int i = 0; i < num_papers; i++) {
        papers[i].num_with_my_colour = num_with_col[papers[i].colour];
    }

    if (verbose) {
        // compute histogram
        int hist_max = 100;
        int hist_num = 0;
        int *hist_s = m_new(int, hist_max);
        int *hist_n = m_new(int, hist_max);
        for (int colour = 1; colour < cur_colour; colour++) {
            int n = num_with_col[colour];

            int i;
            for (i = 0; i < hist_num; i++) {
                if (hist_s[i] == n) {
                    break;
                }
            }
            if (i == hist_num && hist_num < hist_max) {
                hist_num += 1;
                hist_s[i] = n;
                hist_n[i] = 0;
            }
            hist_n[i] += 1;
        }

        printf("%d colours, %d unique sizes\n", cur_colour - 1, hist_num);
        for (int i = 0; i < hist_num; i++) {
            printf("size %d occured %d times\n", hist_s[i], hist_n[i]);
        }
    }

    m_free(num_with_col);
}

// for tred

static void compute_tred_refs_mark(int p_top_index, paper_t *p_cur, paper_t *follow_back_paper, int follow_back_ref) {
    if (p_cur->tred_visit_index != p_top_index) {
        // haven't visited this paper yet
        p_cur->tred_visit_index = p_top_index;
        p_cur->tred_follow_back_paper = follow_back_paper;
        p_cur->tred_follow_back_ref = follow_back_ref;
        // visit all refs
        for (int i = 0; i < p_cur->num_refs; i++) {
            if (p_cur->refs_tred_computed[i] != 0) { // only follow refs that are in the transitively reduced graph
                paper_t *p_ref = p_cur->refs[i];
                if (p_ref->index < p_cur->index) { // allow only past references
                    compute_tred_refs_mark(p_top_index, p_ref, p_cur, i);
                }
            }
        }
    }
}

/* Transitively reduce the graph */
void compute_tred(int num_papers, paper_t *papers) {
    // reset the visit id and tred computed number
    for (int i = 0; i < num_papers; i++) {
        paper_t *p = &papers[i];
        p->tred_visit_index = 0;
        // reset the tred_computed value
        for (int j = 0; j < p->num_refs; j++) {
            p->refs_tred_computed[j] = 0;
        }
        p->tred_follow_back_paper = NULL;
        p->tred_follow_back_ref = 0;
    }

    // transitively reduce
    for (int i = 0; i < num_papers; i++) {
        paper_t *p = &papers[i];

        // clear the follow back pointer for this paper
        p->tred_follow_back_paper = NULL;
        p->tred_follow_back_ref = 0;

        // iterate all refs, from largest index to smallest index (youngest to oldest)
        for (int j = p->num_refs - 1; j >= 0; j--) {
            paper_t *p_past = p->refs[j];

            // only allow references to the past
            if (p_past->index >= p->index) {
                p->refs_tred_computed[j] = 1;
                continue;
            }

            if (p_past->tred_visit_index == p->index) {
                // have already visited this paper
                // follow this path; increase weight of ref path
                paper_t *p2 = p_past->tred_follow_back_paper;
                int ref = p_past->tred_follow_back_ref;
                while (p2 != NULL) {
                    p2->refs_tred_computed[ref] += 1;
                    ref = p2->tred_follow_back_ref;
                    p2 = p2->tred_follow_back_paper;
                }
                continue;
            }

            // haven't visited this paper yet
            // mark link as belonging to tred graph and mark past
            p->refs_tred_computed[j] = 1;
            compute_tred_refs_mark(p->index, p_past, p, j);
        }
    }
}


