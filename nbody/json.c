#include <stdio.h>
#include <stdlib.h>
#include <assert.h>
#include <string.h>
#include <math.h>

#include "util/xiwilib.h"
#include "util/jsmnenv.h"
#include "common.h"
#include "layout.h"

typedef struct _json_data_t {
    int num_papers;
    paper_t *papers;
    keyword_set_t *keyword_set;
} json_data_t;

static void json_data_setup(json_data_t* data) {
    data->num_papers = 0;
    data->papers = NULL;
    data->keyword_set = keyword_set_new();
}

static int paper_cmp_id(const void *in1, const void *in2) {
    paper_t *p1 = (paper_t *)in1;
    paper_t *p2 = (paper_t *)in2;
    if (p1->id < p2->id) {
        return -1;
    } else if (p1->id > p2->id) {
        return 1;
    } else {
        return 0;
    }
}

static bool load_idc(jsmn_env_t *env, json_data_t *data) {
    printf("reading ids from JSON file\n");

    // get the number of entries, so we can allocate the correct amount of memory
    int num_entries;
    if (!jsmn_env_get_num_entries(env, &num_entries)) {
        return false;
    }

    // allocate memory for the papers
    data->papers = m_new(paper_t, num_entries);
    if (data->papers == NULL) {
        return false;
    }

    // start the JSON stream
    bool more_objects;
    if (!jsmn_env_reset(env, &more_objects)) {
        return false;
    }

    // iterate through the JSON stream
    int i = 0;
    while (more_objects) {
        if (!jsmn_env_next_object(env, &more_objects)) {
            return false;
        }
        if (i >= num_entries) {
            return jsmn_env_error(env, "got more entries than expected");
        }

        // look for the id member
        jsmn_env_token_value_t id_val;
        if (!jsmn_env_get_object_member(env, env->js_tok, "id", NULL, &id_val)) {
            return false;
        }

        // check the id is an integer
        if (id_val.kind != JSMN_VALUE_UINT) {
            return jsmn_env_error(env, "expecting an unsigned integer for id");
        }

        // create the paper object, with the id
        paper_t *paper = &data->papers[i];
        paper_init(paper, id_val.uint);

        // look for the allcats member
        jsmn_env_token_value_t allcats_val;
        if (!jsmn_env_get_object_member(env, env->js_tok, "allcats", NULL, &allcats_val)) {
            return false;
        }

        // check allcats is a string
        if (allcats_val.kind != JSMN_VALUE_STRING) {
            return jsmn_env_error(env, "expecting a string for allcats");
        }

        // parse categories
        int cat_num = 0;
        for (const char *start = allcats_val.str, *cur = allcats_val.str; cat_num < COMMON_PAPER_MAX_CATS; cur++) {
            if (*cur == ',' || *cur == '\0') {
                category_t cat = category_strn_to_enum(start, cur - start);
                if (cat == CAT_UNKNOWN) {
                    // print unknown categories; for adding to cats.h
                    printf("%.*s\n", (int)(cur - start), start);
                } else {
                    paper->allcats[cat_num++] = cat;
                }
                if (*cur == '\0') {
                    break;
                }
                start = cur + 1;
            }
        }
        // fill in unused entries in allcats with UNKNOWN category
        for (; cat_num < COMMON_PAPER_MAX_CATS; cat_num++) {
            paper->allcats[cat_num] = CAT_UNKNOWN;
        }

        /*
        // load authors and title if wanted
        if (load_authors_and_titles) {
            paper->authors = strdup(row[2]);
            paper->title = strdup(row[3]);
        }
        */

        i += 1;
    }
    data->num_papers = i;

    // sort the papers array by id
    qsort(data->papers, data->num_papers, sizeof(paper_t), paper_cmp_id);

    // assign the index based on their sorted position
    for (int i = 0; i < data->num_papers; i++) {
        data->papers[i].index = i;
    }

    printf("read %d ids\n", data->num_papers);

    return true;
}

static paper_t *get_paper_by_id(jsmn_env_t *env, json_data_t *data, unsigned int id) {
    int lo = 0;
    int hi = data->num_papers - 1;
    while (lo <= hi) {
        int mid = (lo + hi) / 2;
        if (id == data->papers[mid].id) {
            return &data->papers[mid];
        } else if (id < data->papers[mid].id) {
            hi = mid - 1;
        } else {
            lo = mid + 1;
        }
    }
    return NULL;
}

static bool load_refs(jsmn_env_t *env, json_data_t *data) {
    printf("reading refs from JSON file\n");

    // start the JSON stream
    bool more_objects;
    if (!jsmn_env_reset(env, &more_objects)) {
        return false;
    }

    // iterate through the JSON stream
    int total_refs = 0;
    while (more_objects) {
        if (!jsmn_env_next_object(env, &more_objects)) {
            return false;
        }

        // look for the id member
        jsmn_env_token_value_t id_val;
        if (!jsmn_env_get_object_member(env, env->js_tok, "id", NULL, &id_val)) {
            return false;
        }

        // check the id is an integer
        if (id_val.kind != JSMN_VALUE_UINT) {
            return jsmn_env_error(env, "expecting an unsigned integer");
        }

        // lookup the paper object with this id
        paper_t *paper = get_paper_by_id(env, data, id_val.uint);

        // if paper found, parse its refs
        if (paper != NULL) {
            // look for the refs member
            jsmntok_t *refs_tok;
            if (!jsmn_env_get_object_member(env, env->js_tok, "refs", &refs_tok, NULL)) {
                return false;
            }

            // check the refs is an array
            if (refs_tok->type != JSMN_ARRAY) {
                return jsmn_env_error(env, "expecting an array");
            }

            // set the number of refs
            paper->num_refs = refs_tok->size;

            if (paper->num_refs == 0) {
                // no refs to parse
                paper->refs = NULL;
                paper->refs_ref_freq = NULL;

            } else {
                // some refs to parse

                // allocate memory
                paper->refs = m_new(paper_t*, paper->num_refs);
                paper->refs_ref_freq = m_new(byte, paper->num_refs);
                if (paper->refs == NULL || paper->refs_ref_freq == NULL) {
                    return false;
                }

                // parse the refs
                paper->num_refs = 0;
                for (int i = 0; i < refs_tok->size; i++) {
                    // get current element
                    jsmntok_t *elem_tok;
                    if (!jsmn_env_get_array_member(env, refs_tok, i, &elem_tok, NULL)) {
                        return false;
                    }

                    // check the element is an array of size 2
                    if (elem_tok->type != JSMN_ARRAY || elem_tok->size != 2) {
                        return jsmn_env_error(env, "expecting an array of size 2");
                    }

                    // get the 2 values
                    jsmn_env_token_value_t ref_id_val;
                    jsmn_env_token_value_t ref_freq_val;
                    if (!jsmn_env_get_array_member(env, elem_tok, 0, NULL, &ref_id_val)) {
                        return false;
                    }
                    if (!jsmn_env_get_array_member(env, elem_tok, 1, NULL, &ref_freq_val)) {
                        return false;
                    }
                    if (ref_id_val.kind != JSMN_VALUE_UINT) {
                        return jsmn_env_error(env, "expecting an unsigned integer for ref_id");
                    }
                    if (ref_freq_val.kind != JSMN_VALUE_UINT) {
                        return jsmn_env_error(env, "expecting an unsigned integer for ref_freq");
                    }
                    if (ref_id_val.uint == paper->id) {
                        // make sure paper doesn't ref itself (yes, they exist, see eg 1202.2631)
                        continue;
                    }
                    paper->refs[paper->num_refs] = get_paper_by_id(env, data, ref_id_val.uint);
                    if (paper->refs[paper->num_refs] != NULL) {
                        paper->refs[paper->num_refs]->num_cites += 1;
                        unsigned short ref_freq = ref_freq_val.uint;
                        if (ref_freq > 255) {
                            ref_freq = 255;
                        }
                        paper->refs_ref_freq[paper->num_refs] = ref_freq;
                        paper->num_refs++;
                    }
                }
                total_refs += paper->num_refs;
            }
        }
    }

    printf("read %d total refs\n", total_refs);

    return true;
}

#if 0
static bool env_load_keywords(jsmn_env_t *env) {
    MYSQL_RES *result;
    MYSQL_ROW row;
    unsigned long *lens;

    printf("reading keywords\n");

    // get the keywords from the db
    vstr_t *vstr = env->vstr[VSTR_0];
    vstr_reset(vstr);
    vstr_printf(vstr, "SELECT id,keywords FROM meta_data");
    if (vstr_had_error(vstr)) {
        return false;
    }
    if (!env_query_many_rows(env, vstr_str(vstr), 2, &result)) {
        return false;
    }

    int total_keywords = 0;
    while ((row = mysql_fetch_row(result))) {
        lens = mysql_fetch_lengths(result);
        paper_t *paper = get_paper_by_id(env, data, atoll(row[0]));
        if (paper != NULL) {
            unsigned long len = lens[1];
            if (len == 0) {
                paper->num_keywords = 0;
                paper->keywords = NULL;
            } else {
                const char *kws_start = row[1];
                const char *kws_end = row[1] + len;

                // count number of keywords
                int num_keywords = 1;
                for (const char *kw = kws_start; kw < kws_end; kw++) {
                    if (*kw == ',') {
                        num_keywords += 1;
                    }
                }

                // limit number of keywords per paper
                if (num_keywords > 5) {
                    num_keywords = 5;
                }

                // allocate memory
                paper->keywords = m_new(keyword_t*, num_keywords);
                if (paper->keywords == NULL) {
                    mysql_free_result(result);
                    return false;
                }

                // populate keyword list for this paper
                paper->num_keywords = 0;
                for (const char *kw = kws_start; kw < kws_end && num_keywords > 0; num_keywords--) {
                    const char *kw_end = kw;
                    while (kw_end < kws_end && *kw_end != ',') {
                        kw_end++;
                    }
                    keyword_t *unique_keyword = keyword_set_lookup_or_insert(env->keyword_set, kw, kw_end - kw);
                    if (unique_keyword != NULL) {
                        paper->keywords[paper->num_keywords++] = unique_keyword;
                    }
                    kw = kw_end;
                    if (kw < kws_end) {
                        kw += 1; // skip comma
                    }
                }
                total_keywords += paper->num_keywords;
            }
        }
    }
    mysql_free_result(result);

    printf("read %d unique, %d total keywords\n", keyword_set_get_total(env->keyword_set), total_keywords);

    return true;
}
#endif

bool json_load_papers(const char *filename, int *num_papers_out, paper_t **papers_out, keyword_set_t **keyword_set_out) {
    // set up environment
    jsmn_env_t env;
    if (!jsmn_env_set_up(&env, filename)) {
        jsmn_env_finish(&env);
        return false;
    }
    // set up data
    json_data_t data;
    json_data_setup(&data);

    // load our data
    if (!jsmn_env_open_json_file(&env, filename)) {
        return false;
    }
    if (!load_idc(&env,&data)) {
        return false;
    }
    if (!load_refs(&env,&data)) {
        return false;
    }
    if (!build_citation_links(data.num_papers, data.papers)) {
        return false;
    }

    // pull down the environment 
    jsmn_env_finish(&env);

    // return the papers and keywords
    *num_papers_out = data.num_papers;
    *papers_out = data.papers;
    *keyword_set_out = data.keyword_set;

    return true;
}

static bool load_other_links_helper(jsmn_env_t *env, json_data_t *data) {
    printf("reading other links from JSON file\n");

    // start the JSON stream
    bool more_objects;
    if (!jsmn_env_reset(env, &more_objects)) {
        return false;
    }

    // iterate through the JSON stream
    int total_links = 0;
    int total_new_links = 0;
    while (more_objects) {
        if (!jsmn_env_next_object(env, &more_objects)) {
            return false;
        }

        // look for the id member
        jsmn_env_token_value_t id_val;
        if (!jsmn_env_get_object_member(env, env->js_tok, "id", NULL, &id_val)) {
            return false;
        }

        // check the id is an integer
        if (id_val.kind != JSMN_VALUE_UINT) {
            return jsmn_env_error(env, "expecting an unsigned integer");
        }

        // lookup the paper object with this id
        paper_t *paper = get_paper_by_id(env, data, id_val.uint);

        // if paper found, parse its links
        if (paper != NULL) {
            // look for the links member
            jsmntok_t *links_tok;
            if (!jsmn_env_get_object_member(env, env->js_tok, "refs", &links_tok, NULL)) {
                return false;
            }

            // check the links is an array
            if (links_tok->type != JSMN_ARRAY) {
                return jsmn_env_error(env, "expecting an array");
            }

            if (links_tok->size == 0) {
                // no links to parse

            } else {
                // some links to parse

                // reallocate memory to add links to refs
                int n_alloc = paper->num_refs + links_tok->size;
                paper->refs = m_renew(paper_t*, paper->refs, n_alloc);
                paper->refs_ref_freq = m_renew(byte, paper->refs_ref_freq, n_alloc);
                paper->refs_other_weight = m_new(float, n_alloc);
                if (paper->refs == NULL || paper->refs_ref_freq == NULL || paper->refs_other_weight == NULL) {
                    return false;
                }

                // zero the new weights to begin with
                for (int i = 0; i < paper->num_refs; i++) {
                    paper->refs_other_weight[i] = 0;
                }

                // parse the links
                for (int i = 0; i < links_tok->size; i++) {
                    // get current element
                    jsmntok_t *elem_tok;
                    if (!jsmn_env_get_array_member(env, links_tok, i, &elem_tok, NULL)) {
                        return false;
                    }

                    // check the element is an array of size 2
                    if (elem_tok->type != JSMN_ARRAY || elem_tok->size != 2) {
                        return jsmn_env_error(env, "expecting an array of size 2");
                    }

                    // get the 2 values
                    jsmn_env_token_value_t link_id_val;
                    jsmn_env_token_value_t link_weight_val;
                    if (!jsmn_env_get_array_member(env, elem_tok, 0, NULL, &link_id_val)) {
                        return false;
                    }
                    if (!jsmn_env_get_array_member(env, elem_tok, 1, NULL, &link_weight_val)) {
                        return false;
                    }
                    if (link_id_val.kind != JSMN_VALUE_UINT) {
                        return jsmn_env_error(env, "expecting an unsigned integer for link_id");
                    }
                    if (link_weight_val.kind != JSMN_VALUE_UINT && link_weight_val.kind != JSMN_VALUE_SINT && link_weight_val.kind != JSMN_VALUE_REAL) {
                        return jsmn_env_error(env, "expecting a number link_weight");
                    }

                    // get linked-to paper
                    paper_t *paper2 = get_paper_by_id(env, data, link_id_val.uint);

                    if (paper2 != NULL && paper2 != paper) {
                        // search for existing link
                        bool found = false;
                        for (int i = 0; i < paper->num_refs; i++) {
                            if (paper->refs[i] == paper2) {
                                // found existing link; set its weight
                                paper->refs_other_weight[i] = link_weight_val.real;
                                found = true;
                                break;
                            }
                        }
                        if (!found) {
                            // a new link; add it with ref_freq 0 (since it's not a real reference)
                            paper->refs[paper->num_refs] = paper2;
                            paper->refs_ref_freq[paper->num_refs] = 0;
                            paper->refs_other_weight[paper->num_refs] = link_weight_val.real;
                            paper->num_refs++;
                            paper2->num_cites += 1; // TODO a bit of a hack at the moment
                            total_new_links += 1;
                        }
                        total_links += 1;
                    }
                }
            }
        }
    }

    printf("read %d total links, %d of those were additional ones\n", total_links, total_new_links);

    return true;
}

bool json_load_other_links(const char *filename, int num_papers, paper_t *papers) {
    // set up environment
    jsmn_env_t env;
    if (!jsmn_env_set_up(&env, filename)) {
        jsmn_env_finish(&env);
        return false;
    }
    // set up data
    json_data_t data;
    json_data_setup(&data);

    // set papers
    data.num_papers = num_papers;
    data.papers = papers;

    // load other data
    if (!jsmn_env_open_json_file(&env, filename)) {
        return false;
    }
    if (!load_other_links_helper(&env,&data)) {
        return false;
    }

    // TODO this is a hack
    // we need to rebuild cites so that graph colouring etc works
    // but then the number of citations a paper has is wrong, since
    // the count includes these new links
    for (int i = 0; i < data.num_papers; i++) {
        if (data.papers[i].num_cites > 0) {
            m_free(data.papers[i].cites);
        }
    }
    if (!build_citation_links(data.num_papers, data.papers)) {
        return false;
    }

    // pull down the environment 
    jsmn_env_finish(&env);
    // free keyword set
    keyword_set_free(data.keyword_set);
    data.keyword_set = NULL;

    return true;
}
