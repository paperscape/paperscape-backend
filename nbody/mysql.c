#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <mysql/mysql.h>

#include "xiwilib.h"
#include "common.h"
#include "mysql.h"

#define VSTR_0 (0)
#define VSTR_1 (1)
#define VSTR_2 (2)
#define VSTR_MAX (3)

typedef struct _keyword_pool_t {
    int alloc;
    int used;
    keyword_t *keywords;
    struct _keyword_pool_t *next;
} keyword_pool_t;

typedef struct _env_t {
    vstr_t *vstr[VSTR_MAX];
    bool close_mysql;
    MYSQL mysql;
    int num_papers;
    paper_t *papers;
    keyword_pool_t *keyword_pool;
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
    env->keyword_pool = NULL;

    // initialise the connection object
    if (mysql_init(&env->mysql) == NULL) {
        have_error(env);
        return false;
    }
    env->close_mysql = true;

    // connect to the MySQL server
    if (mysql_real_connect(&env->mysql, "localhost", "hidden", "hidden", "xiwi", 0, NULL, 0) == NULL) {
        if (mysql_real_connect(&env->mysql, "localhost", "hidden", "hidden", "xiwi", 0, "/home/damien/mysql/mysql.sock", 0) == NULL) {
            have_error(env);
            return false;
        }
    }

    return true;
}

static void env_finish(env_t* env) {
    for (int i = 0; i < VSTR_MAX; i++) {
        vstr_free(env->vstr[i]);
    }

    // free the keyword pool container, but not the actual keywords
    for (keyword_pool_t *kwp = env->keyword_pool; kwp != NULL;) {
        keyword_pool_t *next = kwp->next;
        m_free(kwp);
        kwp = next;
    }

    if (env->close_mysql) {
        mysql_close(&env->mysql);
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

static bool env_load_ids(env_t *env, const char *where_clause) {
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
    vstr_printf(vstr, "SELECT id,maincat,allcats,authors,title FROM meta_data");
    if (where_clause != NULL && where_clause[0] != 0) {
        vstr_printf(vstr, " WHERE (%s)", where_clause);
    }
    if (vstr_had_error(vstr)) {
        return false;
    }

    if (!env_query_many_rows(env, vstr_str(vstr), 5, &result)) {
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
        paper->id = id;
        paper->num_refs = 0;
        paper->num_cites = 0;
        paper->refs = NULL;
        if (row[1] == NULL) {
            paper->maincat = 4;
        } else if (strcmp(row[1], "hep-th") == 0) {
            paper->maincat = 1;
        } else if (strcmp(row[1], "hep-ph") == 0) {
            paper->maincat = 2;
        } else if (strcmp(row[1], "hep-ex") == 0) {
            paper->maincat = 3;
        } else if (strcmp(row[1], "hep-lat") == 0) {
            paper->maincat = 6;
        } else if (strcmp(row[1], "gr-qc") == 0) {
            paper->maincat = 4;
        } else if (strcmp(row[1], "astro-ph") == 0) {
            if (strncmp(row[2], "astro-ph.GA", 11) == 0) {
                paper->maincat = 5;
            } else if (strncmp(row[2], "astro-ph.HE", 11) == 0) {
                paper->maincat = 7;
            } else {
                paper->maincat = 8;
            }
        } else {
            paper->maincat = 9;
        }
        paper->authors = strdup(row[3]);
        paper->title = strdup(row[4]);
        paper->pos_valid = false;
        paper->num_keywords = 0;
        paper->keywords = NULL;
        paper->x = 0;
        paper->y = 0;
        paper->z = 0;
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

static bool env_load_pos(env_t *env) {
    MYSQL_RES *result;
    MYSQL_ROW row;

    printf("reading mappos\n");

    // get the positions from the mappos table
    vstr_t *vstr = env->vstr[VSTR_0];
    vstr_reset(vstr);
    vstr_printf(vstr, "SELECT id,x,y FROM mappos");
    if (vstr_had_error(vstr)) {
        return false;
    }
    if (!env_query_many_rows(env, vstr_str(vstr), 3, &result)) {
        return false;
    }

    int total_pos = 0;
    while ((row = mysql_fetch_row(result))) {
        paper_t *paper = env_get_paper_by_id(env, atoi(row[0]));
        if (paper != NULL) {
            paper->pos_valid = true;
            paper->x = atof(row[1]);
            paper->y = atof(row[2]);
            paper->z = 0;
            total_pos += 1;
        }
    }
    mysql_free_result(result);

    printf("read %d total positions\n", total_pos);

    return true;
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
            } else {
                if (len % 10 != 0) {
                    printf("length of refs blob should be a multiple of 10; got %lu\n", len);
                    mysql_free_result(result);
                    return false;
                }
                paper->refs = m_new(paper_t*, len / 10);
                if (paper->refs == NULL) {
                    mysql_free_result(result);
                    return false;
                }
                paper->num_refs = 0;
                for (int i = 0; i < len; i += 10) {
                    byte *buf = (byte*)row[1] + i;
                    unsigned int id = decode_le32(buf + 0);
                    paper->refs[paper->num_refs] = env_get_paper_by_id(env, id);
                    if (paper->refs[paper->num_refs] != NULL) {
                        paper->refs[paper->num_refs]->num_cites += 1;
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

static keyword_t *env_get_or_create_keyword(env_t *env, const char *kw, const char *kw_end) {
    if (kw_end <= kw) {
        return NULL;
    }

    keyword_pool_t *kwp;

    // first search for keyword to see if we already have it
    for (kwp = env->keyword_pool; kwp != NULL; kwp = kwp->next) {
        for (int i = 0; i < kwp->used; i++) {
            if (strncmp(kwp->keywords[i].keyword, kw, kw_end - kw) == 0 && kwp->keywords[i].keyword[kw_end - kw] == '\0') {
                //printf("found keyword %s\n", kwp->keywords[i].keyword);
                return &kwp->keywords[i];
            }
        }
    }

    // not found, so make a new keyword object
    kwp = env->keyword_pool;
    if (kwp == NULL || kwp->used >= kwp->alloc) {
        // need to allocate memory for the keyword object
        kwp = m_new(keyword_pool_t, 1);
        if (kwp == NULL) {
            return NULL;
        }
        if (env->keyword_pool == NULL) {
            kwp->alloc = 1024;
        } else {
            kwp->alloc = env->keyword_pool->alloc * 2;
        }
        kwp->used = 0;
        kwp->keywords = m_new(keyword_t, kwp->alloc);
        if (kwp->keywords == NULL) {
            m_free(kwp);
            return NULL;
        }
        kwp->next = env->keyword_pool;
        env->keyword_pool = kwp;
    }

    kwp->keywords[kwp->used].keyword = strndup(kw, kw_end - kw);
    //printf("created keyword %s\n", kwp->keywords[kwp->used].keyword);
    return &kwp->keywords[kwp->used++];
}

static bool env_load_keywords(env_t *env) {
    MYSQL_RES *result;
    MYSQL_ROW row;
    unsigned long *lens;

    printf("reading keywords\n");

    // get the keywords from the mapskw table
    vstr_t *vstr = env->vstr[VSTR_0];
    vstr_reset(vstr);
    vstr_printf(vstr, "SELECT id,keywords FROM mapskw");
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
                paper->num_keywords = 1;
                for (const char *kw = kws_start; kw < kws_end; kw++) {
                    if (*kw == ',') {
                        paper->num_keywords += 1;
                    }
                }

                // allocate memory
                paper->keywords = m_new(keyword_t*, paper->num_keywords);
                if (paper->keywords == NULL) {
                    mysql_free_result(result);
                    return false;
                }

                // populate keyword list for this paper
                paper->num_keywords = 0;
                for (const char *kw = kws_start; kw < kws_end;) {
                    const char *kw_end = kw;
                    while (kw_end < kws_end && *kw_end != ',') {
                        kw_end++;
                    }
                    keyword_t *keyword = env_get_or_create_keyword(env, kw, kw_end);
                    if (keyword != NULL) {
                        paper->keywords[paper->num_keywords++] = keyword;
                    }
                    kw = kw_end;
                    if (kw < kws_end) {
                        kw += 1; // skip comma
                    }
                    if (paper->num_keywords > 2) {
                        break;
                    }
                }
                total_keywords += paper->num_keywords;
            }
        }
    }
    mysql_free_result(result);

    int unique_keywords = 0;
    for (keyword_pool_t *kwp = env->keyword_pool; kwp != NULL; kwp = kwp->next) {
        unique_keywords += kwp->used;
    }
    printf("read %d unique, %d total keywords\n", unique_keywords, total_keywords);

    return true;
}

bool mysql_load_papers(const char *where_clause, int *num_papers_out, paper_t **papers_out) {
    // set up environment
    env_t env;
    if (!env_set_up(&env)) {
        env_finish(&env);
        return false;
    }

    // load the DB
    env_load_ids(&env, where_clause);
    //env_load_pos(&env);
    env_load_refs(&env);
    //env_load_keywords(&env);
    env_build_cites(&env);

    // pull down the MySQL environment (doesn't free the papers or keywords)
    env_finish(&env);

    // return the papers
    *num_papers_out = env.num_papers;
    *papers_out = env.papers;

    return true;
}

/****************************************************************/
/* stuff to save papers positions to DB                         */
/****************************************************************/

// save paper positions to mappos table
static bool env_save_pos(env_t *env) {
    vstr_t *vstr = env->vstr[VSTR_0];
    for (int i = 0; i < env->num_papers; i++) {
        paper_t *paper = &env->papers[i];

        if (paper->pos_valid) {
            vstr_reset(vstr);
            vstr_printf(vstr, "REPLACE INTO mappos (id,x,y) VALUES (%d,%.3f,%.3f)", paper->id, paper->x, paper->y);
            if (vstr_had_error(vstr)) {
                return false;
            }

            if (!env_query_no_result(env, vstr_str(vstr), vstr_len(vstr))) {
                return false;
            }
        }
    }

    printf("saved %d positions to mappos\n", env->num_papers);

    return true;
}

bool mysql_save_paper_positions(int num_papers, paper_t *papers) {
    // set up environment
    env_t env;
    if (!env_set_up(&env)) {
        env_finish(&env);
        return false;
    }

    // set papers
    env.num_papers = num_papers;
    env.papers = papers;

    // save positions
    env_save_pos(&env);

    // pull down the MySQL environment
    env_finish(&env);

    return true;
}
