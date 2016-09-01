#include <stdio.h>
#include <stdlib.h>
#include <assert.h>
#include <string.h>
#include <math.h>

#include "xiwilib.h"
#include "jsmn.h"
#include "Common.h"
#include "layout.h"

#define JS_TOK_MAX (4000) // need lots for papers with lots of references

typedef struct _env_t {
    // for JSON parsing
    FILE *fp;
    vstr_t *js_buf;
    jsmntok_t js_tok[JS_TOK_MAX];
    jsmn_parser js_parser;

    // graph data
    int num_papers;
    Common_paper_t *papers;
    Common_keyword_set_t *keyword_set;
} env_t;

static bool have_error(env_t *env, const char *msg) {
    printf("JSON error: %s\n", msg);
    return false;
}

static bool env_set_up(env_t* env, const char *filename) {
    env->fp = NULL;
    env->js_buf = vstr_new();

    env->num_papers = 0;
    env->papers = NULL;
    env->keyword_set = keyword_set_new();

    return true;
}

static void env_finish(env_t* env, bool free_keyword_set) {
    if (env->fp != NULL) {
        fclose(env->fp);
    }

    if (free_keyword_set) {
        Common_keyword_set_free(env->keyword_set);
        env->keyword_set = NULL;
    }
}

static bool env_open_json_file(env_t* env, const char *filename) {
    if (env->fp != NULL) {
        fclose(env->fp);
    }

    // open the JSON file
    env->fp = fopen(filename, "r");
    if (env->fp == NULL) {
        printf("can't open JSON file '%s' for reading\n", filename);
        return false;
    }

    printf("opened JSON file '%s'\n", filename);

    return true;
}

// returns next non-whitespace character, or EOF
static int js_next_char(env_t *env) {
    for (;;) {
        int c = fgetc(env->fp);
        switch (c) {
            case '\t': case '\r': case '\n': case ' ':
                break;
            default:
                return c;
        }
    }
}

// reset JSON stream to start
static bool js_reset(env_t *env, bool *more_objects) {
    // reset file to start
    fseek(env->fp, 0, SEEK_SET);

    // seek into the outer JSON array
    if (js_next_char(env) != '[') {
        return have_error(env, "expecting '[' to start array");
    }

    // check if empty array or not
    int c = js_next_char(env);
    if (c == ']') {
        *more_objects = false;
    } else {
        ungetc(c, env->fp);
        *more_objects = true;
    }

    return true;
}

// parse the next JSON object in the stream
static bool js_next_object(env_t *env, bool *more_objects) {
    // get open brace of object
    int c = js_next_char(env);
    if (c != '{') {
        return have_error(env, "expecting '{' to start object");
    }

    // read in string of object
    vstr_reset(env->js_buf);
    vstr_add_byte(env->js_buf, '{');
    int nest = 1;
    while (nest > 0) {
        c = fgetc(env->fp);
        if (c == EOF) {
            return have_error(env, "JSON file ended prematurely");
        }
        vstr_add_byte(env->js_buf, c);
        // TODO don't look at braces inside strings
        switch (c) {
            case '{': nest++; break;
            case '}': nest--; break;
        }
    }

    // check character following object
    c = js_next_char(env);
    if (c == ']') {
        // end of array
        *more_objects = false;
        if (js_next_char(env) != EOF) {
            return have_error(env, "JSON file had unexpected data following array");
        }
    } else if (c == ',') {
        // more entries in array
        *more_objects = true;
    }

    jsmn_init(&env->js_parser);
    jsmnerr_t ret = jsmn_parse(&env->js_parser, vstr_str(env->js_buf), env->js_tok, JS_TOK_MAX);
    if (ret != JSMN_SUCCESS) {
        return have_error(env, "error parsing JSON");
    }

    return true;
}

static bool env_get_num_entries(env_t *env, int *num_entries) {
    *num_entries = 0;
    bool more_objects;
    if (!js_reset(env, &more_objects)) {
        return false;
    }
    while (more_objects) {
        if (!js_next_object(env, &more_objects)) {
            return false;
        }
        *num_entries += 1;
    }
    return true;
}

static int paper_cmp_id(const void *in1, const void *in2) {
    Common_paper_t *p1 = (Common_paper_t *)in1;
    Common_paper_t *p2 = (Common_paper_t *)in2;
    if (p1->id < p2->id) {
        return -1;
    } else if (p1->id > p2->id) {
        return 1;
    } else {
        return 0;
    }
}

typedef enum {
    JSMN_VALUE_TRUE,
    JSMN_VALUE_FALSE,
    JSMN_VALUE_NULL,
    JSMN_VALUE_UINT,
    JSMN_VALUE_SINT,
    JSMN_VALUE_REAL,
    JSMN_VALUE_STRING,
    JSMN_VALUE_ARRAY,
    JSMN_VALUE_OBJECT,
} jsmn_value_kind_t;

typedef struct _jsmn_token_value_t {
    jsmn_value_kind_t kind;
    const char *str;
    unsigned int uint;
    int sint;
    double real;
} jsmn_token_value_t;

// note that this function can modify the js_buf buffer
// for strings, it replaces the ending quote " by \0 to make a proper ASCIIZ string
static void jsmn_get_token_value(env_t *env, jsmntok_t *tok, jsmn_token_value_t *val) {
    const char *str = vstr_str(env->js_buf) + tok->start;
    char *top = vstr_str(env->js_buf) + tok->end;

    //printf("parsing %.*s\n", top - str, str);
    switch (tok->type) {
        case JSMN_PRIMITIVE:
            if (str[0] == 't') {
                val->kind = JSMN_VALUE_TRUE;
                break;
            } else if (str[0] == 'f') {
                val->kind = JSMN_VALUE_FALSE;
                break;
            } else if (str[0] == 'n') {
                val->kind = JSMN_VALUE_NULL;
                break;
            } else {
                // a number
                val->uint = 0;
                val->sint = 0;
                val->real = 0;
                if (str[0] == '-') {
                    val->kind = JSMN_VALUE_SINT;
                    str++;
                } else {
                    val->kind = JSMN_VALUE_UINT;
                }
                // parse integer part
                for (; str < top && '0' <= *str && *str <= '9'; str++) {
                    val->uint = val->uint * 10 + *str - '0';
                    val->sint = val->sint * 10 + *str - '0';
                    val->real = val->real * 10 + *str - '0';
                }
                // check for decimal
                if (str < top && *str == '.') {
                    str++;
                    val->kind = JSMN_VALUE_REAL;
                    double frac = 0.1;
                    for (; str < top && '0' <= *str && *str <= '9'; str++) {
                        val->real = val->real + frac * (*str - '0');
                        frac *= 0.1;
                    }
                }
                // check for exponent
                if (str < top && (*str == 'E' || *str == 'e')) {
                    str++;
                    val->kind = JSMN_VALUE_REAL;
                    bool neg = false;
                    if (*str == '+') {
                        str++;
                    } else if (*str == '-') {
                        str++;
                        neg = true;
                    }
                    int expo = 0;
                    for (; str < top && '0' <= *str && *str <= '9'; str++) {
                        expo = expo * 10 + *str - '0';
                    }
                    if (neg) {
                        expo = -expo;
                    }
                    val->real *= pow(10, expo);
                }
            }
            break;

        case JSMN_STRING:
            val->kind = JSMN_VALUE_STRING;
            val->str = str;
            *top = 0; // replace the ending " of the string with \0
            break;

        case JSMN_ARRAY:
            val->kind = JSMN_VALUE_ARRAY;
            break;

        case JSMN_OBJECT:
            val->kind = JSMN_VALUE_OBJECT;
            break;
    }
}

// objects points to start of object to skip
// returns pointer to token of object after this object
jsmntok_t *jsmn_skip_object(jsmntok_t *object) {
    switch (object->type) {
        case JSMN_PRIMITIVE:
        case JSMN_STRING:
            return object + 1;
            //return object + 1;
        case JSMN_ARRAY:
        case JSMN_OBJECT:
        {
            int size = object->size;
            object += 1;
            for (; size > 0; size--) {
                object = jsmn_skip_object(object);
            }
            return object;
        }
        default:
            return object;
    }
}

// array points to the starting array token, with following tokens being its members
bool jsmn_get_array_member(env_t *env, jsmntok_t *array, int wanted_member, jsmntok_t **found_token, jsmn_token_value_t *found_value) {
    // check we are given an array
    if (array->type != JSMN_ARRAY) {
        return have_error(env, "expecting an array");
    }

    // go through each element of the array until we find the one we want
    int size = array->size;
    array += 1;
    for (; size > 0; size--) {
        if (wanted_member == 0) {
            if (found_token != NULL) {
                *found_token = array;
            }
            if (found_value != NULL) {
                jsmn_get_token_value(env, array, found_value);
            }
            return true;
        }

        wanted_member--;
        array = jsmn_skip_object(array);
    }

    // not found
    return have_error(env, "array does not have member");
}

// object points to the starting object token, with following tokens being its members
bool jsmn_get_object_member(env_t *env, jsmntok_t *object, const char *wanted_member, jsmntok_t **found_token, jsmn_token_value_t *found_value) {
    // check we are given an object
    if (object->type != JSMN_OBJECT) {
        return have_error(env, "expecting an object");
    }

    // go through each element of the object
    int size = object->size;
    object += 1;
    for (; size >= 2; size -= 2) {
        jsmn_token_value_t vkey;
        jsmn_get_token_value(env, object, &vkey);

        if (vkey.kind != JSMN_VALUE_STRING) {
            return have_error(env, "expecting a string for object member name");
        }

        object = jsmn_skip_object(object);

        if (strcmp(vkey.str, wanted_member) == 0) {
            if (found_token != NULL) {
                *found_token = object;
            }
            if (found_value != NULL) {
                jsmn_get_token_value(env, object, found_value);
            }
            return true;
        }

        object = jsmn_skip_object(object);
    }

    // not found
    return have_error(env, "object does not have member");
}

static bool env_load_ids(env_t *env) {
    printf("reading ids from JSON file\n");

    // get the number of entries, so we can allocate the correct amount of memory
    int num_entries;
    if (!env_get_num_entries(env, &num_entries)) {
        return false;
    }

    // allocate memory for the papers
    env->papers = m_new(Common_paper_t, num_entries);
    if (env->papers == NULL) {
        return false;
    }

    // start the JSON stream
    bool more_objects;
    if (!js_reset(env, &more_objects)) {
        return false;
    }

    // iterate through the JSON stream
    int i = 0;
    while (more_objects) {
        if (!js_next_object(env, &more_objects)) {
            return false;
        }
        if (i >= num_entries) {
            return have_error(env, "got more entries than expected");
        }

        // look for the id member
        jsmn_token_value_t id_val;
        if (!jsmn_get_object_member(env, env->js_tok, "id", NULL, &id_val)) {
            return false;
        }

        // check the id is an integer
        if (id_val.kind != JSMN_VALUE_UINT) {
            return have_error(env, "expecting an unsigned integer for id");
        }

        // create the paper object, with the id
        Common_paper_t *paper = &env->papers[i];
        Common_paper_init(paper, id_val.uint);

        // look for the allcats member
        jsmn_token_value_t allcats_val;
        if (!jsmn_get_object_member(env, env->js_tok, "allcats", NULL, &allcats_val)) {
            return false;
        }

        // check allcats is a string
        if (allcats_val.kind != JSMN_VALUE_STRING) {
            return have_error(env, "expecting a string for allcats");
        }

        // parse categories
        int cat_num = 0;
        for (const char *start = allcats_val.str, *cur = allcats_val.str; cat_num < COMMON_PAPER_MAX_CATS; cur++) {
            if (*cur == ',' || *cur == '\0') {
                Common_category_t cat = category_strn_to_enum(start, cur - start);
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
    env->num_papers = i;

    // sort the papers array by id
    qsort(env->papers, env->num_papers, sizeof(Common_paper_t), paper_cmp_id);

    // assign the index based on their sorted position
    for (int i = 0; i < env->num_papers; i++) {
        env->papers[i].index = i;
    }

    printf("read %d ids\n", env->num_papers);

    return true;
}

static Common_paper_t *env_get_paper_by_id(env_t *env, unsigned int id) {
    int lo = 0;
    int hi = env->num_papers - 1;
    while (lo <= hi) {
        int mid = (lo + hi) / 2;
        if (id == env->papers[mid].id) {
            return &env->papers[mid];
        } else if (id < env->papers[mid].id) {
            hi = mid - 1;
        } else {
            lo = mid + 1;
        }
    }
    return NULL;
}

static bool env_load_refs(env_t *env) {
    printf("reading refs from JSON file\n");

    // start the JSON stream
    bool more_objects;
    if (!js_reset(env, &more_objects)) {
        return false;
    }

    // iterate through the JSON stream
    int total_refs = 0;
    while (more_objects) {
        if (!js_next_object(env, &more_objects)) {
            return false;
        }

        // look for the id member
        jsmn_token_value_t id_val;
        if (!jsmn_get_object_member(env, env->js_tok, "id", NULL, &id_val)) {
            return false;
        }

        // check the id is an integer
        if (id_val.kind != JSMN_VALUE_UINT) {
            return have_error(env, "expecting an unsigned integer");
        }

        // lookup the paper object with this id
        Common_paper_t *paper = env_get_paper_by_id(env, id_val.uint);

        // if paper found, parse its refs
        if (paper != NULL) {
            // look for the refs member
            jsmntok_t *refs_tok;
            if (!jsmn_get_object_member(env, env->js_tok, "refs", &refs_tok, NULL)) {
                return false;
            }

            // check the refs is an array
            if (refs_tok->type != JSMN_ARRAY) {
                return have_error(env, "expecting an array");
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
                paper->refs = m_new(Common_paper_t*, paper->num_refs);
                paper->refs_ref_freq = m_new(byte, paper->num_refs);
                if (paper->refs == NULL || paper->refs_ref_freq == NULL) {
                    return false;
                }

                // parse the refs
                paper->num_refs = 0;
                for (int i = 0; i < refs_tok->size; i++) {
                    // get current element
                    jsmntok_t *elem_tok;
                    if (!jsmn_get_array_member(env, refs_tok, i, &elem_tok, NULL)) {
                        return false;
                    }

                    // check the element is an array of size 2
                    if (elem_tok->type != JSMN_ARRAY || elem_tok->size != 2) {
                        return have_error(env, "expecting an array of size 2");
                    }

                    // get the 2 values
                    jsmn_token_value_t ref_id_val;
                    jsmn_token_value_t ref_freq_val;
                    if (!jsmn_get_array_member(env, elem_tok, 0, NULL, &ref_id_val)) {
                        return false;
                    }
                    if (!jsmn_get_array_member(env, elem_tok, 1, NULL, &ref_freq_val)) {
                        return false;
                    }
                    if (ref_id_val.kind != JSMN_VALUE_UINT) {
                        return have_error(env, "expecting an unsigned integer for ref_id");
                    }
                    if (ref_freq_val.kind != JSMN_VALUE_UINT) {
                        return have_error(env, "expecting an unsigned integer for ref_freq");
                    }
                    if (ref_id_val.uint == paper->id) {
                        // make sure paper doesn't ref itself (yes, they exist, see eg 1202.2631)
                        continue;
                    }
                    paper->refs[paper->num_refs] = env_get_paper_by_id(env, ref_id_val.uint);
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
static bool env_load_keywords(env_t *env) {
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
        Common_paper_t *paper = env_get_paper_by_id(env, atoll(row[0]));
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
                paper->keywords = m_new(Common_keyword_t*, num_keywords);
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
                    Common_keyword_t *unique_keyword = keyword_set_lookup_or_insert(env->keyword_set, kw, kw_end - kw);
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

    printf("read %d unique, %d total keywords\n", Common_keyword_set_get_total(env->keyword_set), total_keywords);

    return true;
}
#endif

bool Json_load_papers(const char *filename, int *num_papers_out, Common_paper_t **papers_out, Common_keyword_set_t **keyword_set_out) {
    // set up environment
    env_t env;
    if (!env_set_up(&env, filename)) {
        env_finish(&env, true);
        return false;
    }

    // load our data
    if (!env_open_json_file(&env, filename)) {
        return false;
    }
    if (!env_load_ids(&env)) {
        return false;
    }
    if (!env_load_refs(&env)) {
        return false;
    }
    if (!Common_build_citation_links(env.num_papers, env.papers)) {
        return false;
    }

    // pull down the environment (doesn't free the papers or keywords)
    env_finish(&env, false);

    // return the papers and keywords
    *num_papers_out = env.num_papers;
    *papers_out = env.papers;
    *keyword_set_out = env.keyword_set;

    return true;
}

static bool env_load_other_links_helper(env_t *env) {
    printf("reading other links from JSON file\n");

    // start the JSON stream
    bool more_objects;
    if (!js_reset(env, &more_objects)) {
        return false;
    }

    // iterate through the JSON stream
    int total_links = 0;
    int total_new_links = 0;
    while (more_objects) {
        if (!js_next_object(env, &more_objects)) {
            return false;
        }

        // look for the id member
        jsmn_token_value_t id_val;
        if (!jsmn_get_object_member(env, env->js_tok, "id", NULL, &id_val)) {
            return false;
        }

        // check the id is an integer
        if (id_val.kind != JSMN_VALUE_UINT) {
            return have_error(env, "expecting an unsigned integer");
        }

        // lookup the paper object with this id
        Common_paper_t *paper = env_get_paper_by_id(env, id_val.uint);

        // if paper found, parse its links
        if (paper != NULL) {
            // look for the links member
            jsmntok_t *links_tok;
            if (!jsmn_get_object_member(env, env->js_tok, "refs", &links_tok, NULL)) {
                return false;
            }

            // check the links is an array
            if (links_tok->type != JSMN_ARRAY) {
                return have_error(env, "expecting an array");
            }

            if (links_tok->size == 0) {
                // no links to parse

            } else {
                // some links to parse

                // reallocate memory to add links to refs
                int n_alloc = paper->num_refs + links_tok->size;
                paper->refs = m_renew(Common_paper_t*, paper->refs, n_alloc);
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
                    if (!jsmn_get_array_member(env, links_tok, i, &elem_tok, NULL)) {
                        return false;
                    }

                    // check the element is an array of size 2
                    if (elem_tok->type != JSMN_ARRAY || elem_tok->size != 2) {
                        return have_error(env, "expecting an array of size 2");
                    }

                    // get the 2 values
                    jsmn_token_value_t link_id_val;
                    jsmn_token_value_t link_weight_val;
                    if (!jsmn_get_array_member(env, elem_tok, 0, NULL, &link_id_val)) {
                        return false;
                    }
                    if (!jsmn_get_array_member(env, elem_tok, 1, NULL, &link_weight_val)) {
                        return false;
                    }
                    if (link_id_val.kind != JSMN_VALUE_UINT) {
                        return have_error(env, "expecting an unsigned integer for link_id");
                    }
                    if (link_weight_val.kind != JSMN_VALUE_UINT && link_weight_val.kind != JSMN_VALUE_SINT && link_weight_val.kind != JSMN_VALUE_REAL) {
                        return have_error(env, "expecting a number link_weight");
                    }

                    // get linked-to paper
                    Common_paper_t *paper2 = env_get_paper_by_id(env, link_id_val.uint);

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

bool Json_load_other_links(const char *filename, int num_papers, Common_paper_t *papers) {
    // set up environment
    env_t env;
    if (!env_set_up(&env, filename)) {
        env_finish(&env, true);
        return false;
    }

    // set papers
    env.num_papers = num_papers;
    env.papers = papers;

    // load other data
    if (!env_open_json_file(&env, filename)) {
        return false;
    }
    if (!env_load_other_links_helper(&env)) {
        return false;
    }

    // TODO this is a hack
    // we need to rebuild cites so that graph colouring etc works
    // but then the number of citations a paper has is wrong, since
    // the count includes these new links
    for (int i = 0; i < env.num_papers; i++) {
        if (env.papers[i].num_cites > 0) {
            m_free(env.papers[i].cites);
        }
    }
    if (!Common_build_citation_links(env.num_papers, env.papers)) {
        return false;
    }

    // pull down the environment (doesn't free the papers)
    env_finish(&env, true);

    return true;
}
