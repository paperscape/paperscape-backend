#include <stdio.h>
#include <stdlib.h>
#include <assert.h>
#include <string.h>
#include <mysql/mysql.h>

#include "xiwilib.h"
#include "common.h"
#include "layout.h"
#include "mysql.h"

#define VSTR_0 (0)
#define VSTR_1 (1)
#define VSTR_2 (2)
#define VSTR_MAX (3)

typedef struct _env_t {
    vstr_t *vstr[VSTR_MAX];
    bool close_mysql;
    MYSQL mysql;
    int num_papers;
    paper_t *papers;
    keyword_set_t *keyword_set;
} env_t;

static bool have_error(env_t *env) {
    printf("MySQL error %d: %s\n", mysql_errno(&env->mysql), mysql_error(&env->mysql));
    return false;
}

static bool env_set_up(env_t* env) {
    for (int i = 0; i < VSTR_MAX; i++) {
        env->vstr[i] = vstr_new();
    }
    env->close_mysql = false;
    env->num_papers = 0;
    env->papers = NULL;
    env->keyword_set = keyword_set_new();

    // initialise the connection object
    if (mysql_init(&env->mysql) == NULL) {
        have_error(env);
        return false;
    }
    env->close_mysql = true;

    // connect to the MySQL server
    if (mysql_real_connect(&env->mysql, "localhost", "hidden", "hidden", "xiwi", 0, NULL, 0) == NULL) {
        if (mysql_real_connect(&env->mysql, "susi", "hidden", "hidden", "xiwi", 0, NULL, 0) == NULL) {
            if (mysql_real_connect(&env->mysql, "localhost", "hidden", "hidden", "xiwi", 0, "/home/damien/mysql/mysql.sock", 0) == NULL) {
                have_error(env);
                return false;
            }
        }
    }

    return true;
}

static void env_finish(env_t* env, bool free_keyword_set) {
    for (int i = 0; i < VSTR_MAX; i++) {
        vstr_free(env->vstr[i]);
    }

    if (env->close_mysql) {
        mysql_close(&env->mysql);
    }

    if (free_keyword_set) {
        keyword_set_free(env->keyword_set);
        env->keyword_set = NULL;
    }
}

static bool env_query_one_row(env_t *env, const char *q, int expected_num_fields, MYSQL_RES **result) {
    if (mysql_query(&env->mysql, q) != 0) {
        return have_error(env);
    }
    if ((*result = mysql_store_result(&env->mysql)) == NULL) {
        return have_error(env);
    }
    if (mysql_num_rows(*result) != 1) {
        printf("env_query_one_row: expecting only 1 result, got %llu\n", mysql_num_rows(*result));
        mysql_free_result(*result);
        return false;
    }
    if (mysql_num_fields(*result) != expected_num_fields) {
        printf("env_query_one_row: expecting %d fields, got %u\n", expected_num_fields, mysql_num_fields(*result));
        mysql_free_result(*result);
        return false;
    }
    return true;
}

static bool env_query_many_rows(env_t *env, const char *q, int expected_num_fields, MYSQL_RES **result) {
    if (mysql_query(&env->mysql, q) != 0) {
        return have_error(env);
    }
    if ((*result = mysql_use_result(&env->mysql)) == NULL) {
        return have_error(env);
    }
    if (mysql_num_fields(*result) != expected_num_fields) {
        printf("env_query_many_rows: expecting %d fields, got %u\n", expected_num_fields, mysql_num_fields(*result));
        mysql_free_result(*result);
        return false;
    }
    return true;
}

static bool env_query_no_result(env_t *env, const char *q, unsigned long len) {
    if (mysql_real_query(&env->mysql, q, len) != 0) {
        return have_error(env);
    }
    return true;
}

static bool env_get_num_ids(env_t *env, int *num_ids) {
    MYSQL_RES *result;
    if (!env_query_one_row(env, "SELECT count(id) FROM meta_data", 1, &result)) {
        return false;
    }
    MYSQL_ROW row = mysql_fetch_row(result);
    *num_ids = atoi(row[0]);
    mysql_free_result(result);
    return true;
}

static int paper_cmp_id(const void *in1, const void *in2) {
    paper_t *p1 = (paper_t *)in1;
    paper_t *p2 = (paper_t *)in2;
    return p1->id - p2->id;
}

static bool env_load_ids(env_t *env, const char *where_clause, bool load_authors_and_titles) {
    MYSQL_RES *result;
    MYSQL_ROW row;

    printf("reading ids from meta_data\n");

    // get the number of ids, so we can allocate the correct amount of memory
    int num_ids;
    if (!env_get_num_ids(env, &num_ids)) {
        return false;
    }

    // allocate memory for the papers
    env->papers = m_new(paper_t, num_ids);
    if (env->papers == NULL) {
        return false;
    }

    // get the ids
    vstr_t *vstr = env->vstr[VSTR_0];
    vstr_reset(vstr);
    int num_fields;
    if (load_authors_and_titles) {
        vstr_printf(vstr, "SELECT id,allcats,authors,title FROM meta_data");
        num_fields = 4;
    } else {
        vstr_printf(vstr, "SELECT id,allcats FROM meta_data");
        num_fields = 2;
    }
    if (where_clause != NULL && where_clause[0] != 0) {
        vstr_printf(vstr, " WHERE (%s)", where_clause);
    }
    if (vstr_had_error(vstr)) {
        return false;
    }

    if (!env_query_many_rows(env, vstr_str(vstr), num_fields, &result)) {
        return false;
    }
    int i = 0;
    while ((row = mysql_fetch_row(result))) {
        if (i >= num_ids) {
            printf("got more ids than expected\n");
            mysql_free_result(result);
            return false;
        }
        int id = atoi(row[0]);
        paper_t *paper = &env->papers[i];
        paper_init(paper, id);

        // parse categories
        int cat_num = 0;
        for (char *start = row[1], *cur = row[1]; cat_num < PAPER_MAX_CATS; cur++) {
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
        for (; cat_num < PAPER_MAX_CATS; cat_num++) {
            paper->allcats[cat_num] = CAT_UNKNOWN;
        }

        // load authors and title if wanted
        if (load_authors_and_titles) {
            paper->authors = strdup(row[2]);
            paper->title = strdup(row[3]);
        }

        i += 1;
    }
    env->num_papers = i;
    mysql_free_result(result);

    // sort the papers array by id
    qsort(env->papers, env->num_papers, sizeof(paper_t), paper_cmp_id);

    // assign the index based on their sorted position
    for (int i = 0; i < env->num_papers; i++) {
        env->papers[i].index = i;
    }

    printf("read %d ids\n", env->num_papers);

    return true;
}

static paper_t *env_get_paper_by_id(env_t *env, int id) {
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
    MYSQL_RES *result;
    MYSQL_ROW row;
    unsigned long *lens;

    printf("reading pcite\n");

    // get the refs blobs from the pcite table
    vstr_t *vstr = env->vstr[VSTR_0];
    vstr_reset(vstr);
    vstr_printf(vstr, "SELECT id,refs FROM pcite");
    if (vstr_had_error(vstr)) {
        return false;
    }
    if (!env_query_many_rows(env, vstr_str(vstr), 2, &result)) {
        return false;
    }

    int total_refs = 0;
    while ((row = mysql_fetch_row(result))) {
        lens = mysql_fetch_lengths(result);
        paper_t *paper = env_get_paper_by_id(env, atoi(row[0]));
        if (paper != NULL) {
            unsigned long len = lens[1];
            if (len == 0) {
                paper->num_refs = 0;
                paper->refs = NULL;
                paper->refs_ref_freq = NULL;
            } else {
                if (len % 10 != 0) {
                    printf("length of refs blob should be a multiple of 10; got %lu\n", len);
                    mysql_free_result(result);
                    return false;
                }
                paper->refs = m_new(paper_t*, len / 10);
                paper->refs_ref_freq = m_new(byte, len / 10);
                if (paper->refs == NULL || paper->refs_ref_freq == NULL) {
                    mysql_free_result(result);
                    return false;
                }
                paper->num_refs = 0;
                for (int i = 0; i < len; i += 10) {
                    byte *buf = (byte*)row[1] + i;
                    unsigned int id = decode_le32(buf + 0);
                    if (id == paper->id) {
                        // make sure paper doesn't ref itself (yes, they exist, see eg 1202.2631)
                        continue;
                    }
                    paper->refs[paper->num_refs] = env_get_paper_by_id(env, id);
                    if (paper->refs[paper->num_refs] != NULL) {
                        paper->refs[paper->num_refs]->num_cites += 1;
                        unsigned short ref_freq = decode_le16(buf + 6);
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
    mysql_free_result(result);

    printf("read %d total refs\n", total_refs);

    return true;
}

static bool env_build_cites(env_t *env) {
    printf("building citation links\n");

    // allocate memory for cites for each paper
    for (int i = 0; i < env->num_papers; i++) {
        paper_t *paper = &env->papers[i];
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
    for (int i = 0; i < env->num_papers; i++) {
        paper_t *paper = &env->papers[i];
        for (int j = 0; j < paper->num_refs; j++) {
            paper_t *ref_paper = paper->refs[j];
            ref_paper->cites[ref_paper->num_cites++] = paper;
        }
    }

    return true;
}

static bool env_load_keywords(env_t *env) {
    MYSQL_RES *result;
    MYSQL_ROW row;
    unsigned long *lens;

    printf("reading keywords\n");

    // get the keywords from the keywords table
    vstr_t *vstr = env->vstr[VSTR_0];
    vstr_reset(vstr);
    vstr_printf(vstr, "SELECT id,keywords FROM keywords");
    if (vstr_had_error(vstr)) {
        return false;
    }
    if (!env_query_many_rows(env, vstr_str(vstr), 2, &result)) {
        return false;
    }

    int total_keywords = 0;
    while ((row = mysql_fetch_row(result))) {
        lens = mysql_fetch_lengths(result);
        paper_t *paper = env_get_paper_by_id(env, atoi(row[0]));
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

bool mysql_load_papers(const char *where_clause, bool load_authors_and_titles, int *num_papers_out, paper_t **papers_out, keyword_set_t **keyword_set_out) {
    // set up environment
    env_t env;
    if (!env_set_up(&env)) {
        env_finish(&env, true);
        return false;
    }

    // load the DB
    if (!env_load_ids(&env, where_clause, load_authors_and_titles)) {
        return false;
    }
    if (!env_load_refs(&env)) {
        return false;
    }
    if (!env_load_keywords(&env)) {
        return false;
    }
    if (!env_build_cites(&env)) {
        return false;
    }

    // pull down the MySQL environment (doesn't free the papers or keywords)
    env_finish(&env, false);

    // return the papers and keywords
    *num_papers_out = env.num_papers;
    *papers_out = env.papers;
    *keyword_set_out = env.keyword_set;

    return true;
}

/****************************************************************/
/* stuff to save papers positions to DB                         */
/****************************************************************/

bool mysql_save_paper_positions(layout_t *layout) {
    // set up environment
    env_t env;
    if (!env_set_up(&env)) {
        env_finish(&env, true);
        return false;
    }

    // save positions
    vstr_t *vstr = env.vstr[VSTR_0];
    assert(layout->child_layout == NULL);
    int total_pos = 0;
    for (int i = 0; i < layout->num_nodes; i++) {
        layout_node_t *n = &layout->nodes[i];

        if (n->flags & LAYOUT_NODE_POS_VALID) {
            vstr_reset(vstr);
            int x, y, r;
            layout_node_export_quantities(n, &x, &y, &r);
            vstr_printf(vstr, "REPLACE INTO map_data (id,x,y,r) VALUES (%d,%d,%d,%d)", n->paper->id, x, y, r);
            if (vstr_had_error(vstr)) {
                env_finish(&env, true);
                return false;
            }
            if (!env_query_no_result(&env, vstr_str(vstr), vstr_len(vstr))) {
                env_finish(&env, true);
                return false;
            }
            total_pos += 1;
        }
    }

    printf("saved %d positions to map_data\n", total_pos);

    // pull down the MySQL environment
    env_finish(&env, true);

    return true;
}

bool mysql_load_paper_positions(layout_t *layout) {
    // set up environment
    env_t env;
    if (!env_set_up(&env)) {
        env_finish(&env, true);
        return false;
    }

    printf("reading map_data\n");

    // query the positions from the map_data table
    vstr_t *vstr = env.vstr[VSTR_0];
    vstr_reset(vstr);
    vstr_printf(vstr, "SELECT id,x,y FROM map_data");
    if (vstr_had_error(vstr)) {
        env_finish(&env, true);
        return false;
    }
    MYSQL_RES *result;
    if (!env_query_many_rows(&env, vstr_str(vstr), 3, &result)) {
        env_finish(&env, true);
        return false;
    }

    // load in all positions
    int total_pos = 0;
    MYSQL_ROW row;
    while ((row = mysql_fetch_row(result))) {
        layout_node_t *n = layout_get_node_by_id(layout, atoi(row[0]));
        if (n != NULL) {
            layout_node_import_quantities(n, atoi(row[1]), atoi(row[2]));
            n->flags |= LAYOUT_NODE_POS_VALID;
            total_pos += 1;
        }
    }
    mysql_free_result(result);

    printf("read %d total positions\n", total_pos);

    // pull down the MySQL environment
    env_finish(&env, true);

    return true;
}
