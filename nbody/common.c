#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <assert.h>

#include "util/xiwilib.h"
#include "common.h"
#include "util/jsmnenv.h"

bool init_config_new(const char *filename, init_config_t **config) {

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
    if (!jsmn_env_get_object_member_value(&jsmn_env, jsmn_env.js_tok, "description", JSMN_VALUE_STRING, &descr_val)) {
        return false;
    }
    printf("Reading in settings for: %s\n",descr_val.str);

    // create new config
    (*config) = m_new(init_config_t,1);

    // look for member: ids_time_ordered
    // =================================
    // set defaults
    (*config)->ids_time_ordered = true;
    // attempt to set from JSON file
    jsmn_env_token_value_t ito_val;
    if(jsmn_env_get_object_member_value_boolean(&jsmn_env, jsmn_env.js_tok, "ids_time_ordered", &ito_val)) {
        (*config)->ids_time_ordered = (ito_val.kind == JSMN_VALUE_TRUE);
    }

    // look for member: forces
    // =======================
    // set defaults
    (*config)->force_close_repulsion_a        = 1e9;
    (*config)->force_close_repulsion_b        = 1e14;
    (*config)->force_close_repulsion_c        = 1.1;
    (*config)->force_close_repulsion_d        = 0.6;
    (*config)->force_link_strength            = 1.17;
    (*config)->force_use_ref_freq             = true;
    (*config)->force_anti_gravity_falloff_rsq = 1e6;
    (*config)->force_initial_close_repulsion  = false;
    // attempt to set from JSON file
    jsmntok_t *forces_tok;
    if(jsmn_env_get_object_member_token(&jsmn_env, jsmn_env.js_tok, "forces", JSMN_OBJECT, &forces_tok)) {
        jsmn_env_token_value_t do_cr_val, use_rf_val, cr_a_val, cr_b_val, cr_c_val, cr_d_val, link_val, anti_grav_val;
        if(jsmn_env_get_object_member_value(&jsmn_env, forces_tok, "close_repulsion_a", JSMN_VALUE_REAL, &cr_a_val)) {
            (*config)->force_close_repulsion_a        = cr_a_val.real;
        }
        if(jsmn_env_get_object_member_value(&jsmn_env, forces_tok, "close_repulsion_b", JSMN_VALUE_REAL, &cr_b_val)) {
            (*config)->force_close_repulsion_b        = cr_b_val.real;
        }
        if(jsmn_env_get_object_member_value(&jsmn_env, forces_tok, "close_repulsion_c", JSMN_VALUE_REAL, &cr_c_val)) {
            (*config)->force_close_repulsion_c        = cr_c_val.real;
        }
        if(jsmn_env_get_object_member_value(&jsmn_env, forces_tok, "close_repulsion_d", JSMN_VALUE_REAL, &cr_d_val)) {
            (*config)->force_close_repulsion_d        = cr_d_val.real;
        }
        if(jsmn_env_get_object_member_value(&jsmn_env, forces_tok, "link_strength", JSMN_VALUE_REAL, &link_val)) {
            (*config)->force_link_strength            = link_val.real;
        }
        if(jsmn_env_get_object_member_value_boolean(&jsmn_env, forces_tok, "use_ref_freq", &use_rf_val)) {
            (*config)->force_use_ref_freq             = use_rf_val.real;
        }
        if(jsmn_env_get_object_member_value(&jsmn_env, forces_tok, "anti_gravity_falloff_rsq", JSMN_VALUE_REAL, &anti_grav_val)) {
            (*config)->force_anti_gravity_falloff_rsq = anti_grav_val.real;
        }
        if(jsmn_env_get_object_member_value_boolean(&jsmn_env, forces_tok, "initial_close_repulsion", &do_cr_val)) {
            (*config)->force_initial_close_repulsion  = do_cr_val.real;
        }
    }



    // look for member: sql 
    // ====================
    // set defaults
    // fields defaulted to empty are not used if not specified
    (*config)->sql_meta_name           = "meta_data";
    (*config)->sql_meta_clause         = "WHERE (arxiv IS NOT NULL AND status != 'WDN')";
    (*config)->sql_meta_field_id       = "id";
    (*config)->sql_meta_field_allcats  = "allcats";
    (*config)->sql_meta_field_title    = "";
    (*config)->sql_meta_field_authors  = "";
    (*config)->sql_meta_field_keywords = "";
    (*config)->sql_refs_name           = "pcite";
    (*config)->sql_refs_field_id       = "id";
    (*config)->sql_refs_field_refs     = "refs";
    (*config)->sql_refs_rblob_order    = true;
    (*config)->sql_refs_rblob_freq     = true;
    (*config)->sql_refs_rblob_cites    = true;
    // attempt to set from JSON file
    jsmntok_t *sql_tok;
    if(jsmn_env_get_object_member_token(&jsmn_env, jsmn_env.js_tok, "sql", JSMN_OBJECT, &sql_tok)) {
        // look for member: meta_table
        // ---------------------------
        jsmntok_t *meta_tok;
        if(jsmn_env_get_object_member_token(&jsmn_env, sql_tok, "meta_table", JSMN_OBJECT, &meta_tok)) {
            jsmn_env_token_value_t name_val, clause_val, id_val, title_val,authors_val,allcats_val, keywords_val;
            if(jsmn_env_get_object_member_value(&jsmn_env, meta_tok, "name", JSMN_VALUE_STRING, &name_val)) {
                (*config)->sql_meta_name = name_val.str;
            }
            if(jsmn_env_get_object_member_value(&jsmn_env, meta_tok, "clause", JSMN_VALUE_STRING, &clause_val)) {
                (*config)->sql_meta_clause = clause_val.str;
            }
            if(jsmn_env_get_object_member_value(&jsmn_env, meta_tok, "field_id", JSMN_VALUE_STRING, &id_val)) {
                (*config)->sql_meta_field_id = id_val.str;
            }
            if(jsmn_env_get_object_member_value(&jsmn_env, meta_tok, "field_title", JSMN_VALUE_STRING, &title_val)) {
                (*config)->sql_meta_field_title = title_val.str;
            }
            if(jsmn_env_get_object_member_value(&jsmn_env, meta_tok, "field_authors", JSMN_VALUE_STRING, &authors_val)) {
                (*config)->sql_meta_field_authors = authors_val.str;
            }
            if(jsmn_env_get_object_member_value(&jsmn_env, meta_tok, "field_allcats", JSMN_VALUE_STRING, &allcats_val)) {
                (*config)->sql_meta_field_allcats = allcats_val.str;
            }
            if(jsmn_env_get_object_member_value(&jsmn_env, meta_tok, "field_keywords", JSMN_VALUE_STRING, &keywords_val)) {
                (*config)->sql_meta_field_keywords = keywords_val.str;
            }

        }
        // look for member: refs_table
        // ---------------------------
        jsmntok_t *refs_tok;
        if(jsmn_env_get_object_member_token(&jsmn_env, sql_tok, "refs_table", JSMN_OBJECT, &refs_tok)) {
            jsmn_env_token_value_t name_val, id_val, refs_val, ref_freq_val, ref_order_val, ref_cites_val;
            if(jsmn_env_get_object_member_value(&jsmn_env, refs_tok, "name", JSMN_VALUE_STRING, &name_val)) {
                (*config)->sql_refs_name = name_val.str;
            }
            if(jsmn_env_get_object_member_value(&jsmn_env, refs_tok, "field_id", JSMN_VALUE_STRING, &id_val)) {
                (*config)->sql_refs_field_id = id_val.str;
            }
            if(jsmn_env_get_object_member_value(&jsmn_env, refs_tok, "field_refs", JSMN_VALUE_STRING, &refs_val)) {
                (*config)->sql_refs_field_refs = refs_val.str;
            }
            if(jsmn_env_get_object_member_value_boolean(&jsmn_env, refs_tok, "rblob_order", &ref_order_val)) {
                (*config)->sql_refs_rblob_order = (ref_order_val.kind == JSMN_VALUE_TRUE);
            }
            if(jsmn_env_get_object_member_value_boolean(&jsmn_env, refs_tok, "rblob_freq", &ref_freq_val)) {
                (*config)->sql_refs_rblob_freq  = (ref_freq_val.kind  == JSMN_VALUE_TRUE);
            }
            if(jsmn_env_get_object_member_value_boolean(&jsmn_env, refs_tok, "rblob_cites", &ref_cites_val)) {
                (*config)->sql_refs_rblob_cites = (ref_cites_val.kind == JSMN_VALUE_TRUE);
            }
        }
        // look for member: map_table
        // ---------------------------
        jsmntok_t *map_tok;
        if(jsmn_env_get_object_member_token(&jsmn_env, sql_tok, "map_table", JSMN_OBJECT, &map_tok)) {
            jsmn_env_token_value_t name_val, id_val, x_val, y_val, r_val;
            if(jsmn_env_get_object_member_value(&jsmn_env, map_tok, "name", JSMN_VALUE_STRING, &name_val)) {
                (*config)->sql_map_name = name_val.str;
            }
            if(jsmn_env_get_object_member_value(&jsmn_env, map_tok, "field_id", JSMN_VALUE_STRING, &id_val)) {
                (*config)->sql_map_field_id = id_val.str;
            }
            if(jsmn_env_get_object_member_value(&jsmn_env, map_tok, "field_x", JSMN_VALUE_STRING, &x_val)) {
                (*config)->sql_map_field_x = x_val.str;
            }
            if(jsmn_env_get_object_member_value(&jsmn_env, map_tok, "field_y", JSMN_VALUE_STRING, &y_val)) {
                (*config)->sql_map_field_y = y_val.str;
            }
            if(jsmn_env_get_object_member_value(&jsmn_env, map_tok, "field_r", JSMN_VALUE_STRING, &r_val)) {
                (*config)->sql_map_field_r = r_val.str;
            }
        }
    }

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

unsigned int date_to_unique_id(int y, int m, int d) {
    return ((unsigned int)y - 1800) * 10000000 + (unsigned int)m * 625000 + (unsigned int)d * 15625;
}

void unique_id_to_date(unsigned int id, int *y, int *m, int *d) {
    *y = id / 10000000 + 1800;
    *m = ((id % 10000000) / 625000) + 1;
    *d = ((id % 625000) / 15625) + 1;
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


