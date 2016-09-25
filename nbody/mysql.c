#include <stdio.h>
#include <stdlib.h>
#include <assert.h>
#include <string.h>
#include <mysql/mysql.h>

#include "util/xiwilib.h"
#include "common.h"
#include "initconfig.h"
#include "category.h"
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
    init_config_t *config;
    int num_papers;
    paper_t *papers;
    hashmap_t *keyword_set;
    category_set_t *category_set;
} env_t;

static bool have_error(env_t *env) {
    printf("MySQL error %d: %s\n", mysql_errno(&env->mysql), mysql_error(&env->mysql));
    return false;
}

static bool env_set_up(env_t* env, init_config_t *init_config) {
    char *mysql_host, *mysql_user, *mysql_pwd, *mysql_db, *mysql_sock;
    
    mysql_host = getenv("PSCP_MYSQL_HOST");
    mysql_user = getenv("PSCP_MYSQL_USER");
    mysql_pwd  = getenv("PSCP_MYSQL_PWD");
    mysql_db   = getenv("PSCP_MYSQL_DB");
    mysql_sock = getenv("PSCP_MYSQL_SOCKET");
    if (mysql_host == NULL) mysql_host = "localhost";

    for (int i = 0; i < VSTR_MAX; i++) {
        env->vstr[i] = vstr_new();
    }
    env->config = init_config;
    env->close_mysql = false;
    env->num_papers = 0;
    env->papers = NULL;
    env->keyword_set = hashmap_new();
    env->category_set = NULL;

    // initialise the connection object
    if (mysql_init(&env->mysql) == NULL) {
        have_error(env);
        return false;
    }
    env->close_mysql = true;

    // connect to the MySQL server
    if (mysql_real_connect(&env->mysql, mysql_host, mysql_user, mysql_pwd, mysql_db, 0, mysql_sock, 0) == NULL) {
        have_error(env);
        return false;
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
        hashmap_free(env->keyword_set);
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
    const char *meta_table = env->config->sql.meta_table.name;
    const char *id         = env->config->sql.meta_table.field_id;
    vstr_t *vstr = env->vstr[VSTR_0];
    vstr_reset(vstr);
    vstr_printf(vstr, "SELECT count(%s) FROM %s",id,meta_table);
    const char *where_clause = env->config->sql.meta_table.where_clause;
    if (strcmp(where_clause,"") != 0) {
        vstr_printf(vstr, " WHERE (%s)", where_clause);
    }
    const char *extra_clause = env->config->sql.meta_table.extra_clause;
    if (strcmp(extra_clause,"") != 0) {
        vstr_printf(vstr, " %s", extra_clause);
    }
    if (vstr_had_error(vstr)) {
        return false;
    }
    
    MYSQL_RES *result;
    if (!env_query_one_row(env, vstr_str(vstr), 1, &result)) {
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
    if (p1->id < p2->id) {
        return -1;
    } else if (p1->id > p2->id) {
        return 1;
    } else {
        return 0;
    }
}

static bool env_load_ids(env_t *env, bool load_display_fields) {
    // TODO sanity checks when allcats or authors or title is null (need to create such entries in DB to test)

    MYSQL_RES *result;
    MYSQL_ROW row;

    printf("reading ids from %s\n",env->config->sql.meta_table.name);

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
    const char *meta_table = env->config->sql.meta_table.name;
    const char *id         = env->config->sql.meta_table.field_id;
    //const char *agesort    = env->config->sql.meta_table.field_agesort;
    const char *allcats    = env->config->sql.meta_table.field_allcats;
    const char *title      = env->config->sql.meta_table.field_title;
    const char *authors    = env->config->sql.meta_table.field_authors;
    vstr_t *vstr = env->vstr[VSTR_0];
    vstr_reset(vstr);
    int num_fields;
    if (load_display_fields && !(strcmp(env->config->sql.meta_table.field_authors,"") == 0 || strcmp(env->config->sql.meta_table.field_title,"") == 0)) {
        vstr_printf(vstr, "SELECT %s,%s,%s,%s FROM %s",id,allcats,authors,title,meta_table);
        num_fields = 4;
    } else {
        vstr_printf(vstr, "SELECT %s,%s FROM %s",id,allcats,meta_table);
        num_fields = 2;
    }
    const char *where_clause = env->config->sql.meta_table.where_clause;
    if (strcmp(where_clause,"") != 0) {
        vstr_printf(vstr, " WHERE (%s)", where_clause);
    }
    const char *extra_clause = env->config->sql.meta_table.extra_clause;
    if (strcmp(extra_clause,"") != 0) {
        vstr_printf(vstr, " %s", extra_clause);
    }
    //vstr_printf(vstr, ") ORDER BY %s", agesort);
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
        unsigned long long id = atoll(row[0]);
        paper_t *paper = &env->papers[i];
        paper_init(paper, id);

        // parse categories
        int cat_num = 0;
        if (row[1] != NULL) {
            float def_col[3] = {1,1,1};
            for (char *start = row[1], *cur = row[1]; cat_num < COMMON_PAPER_MAX_CATS; cur++) {
                if (*cur == ',' || *cur == '\0') {
                    category_info_t *cat = category_set_get_by_name(env->category_set, start, cur - start);
                    if (cat == NULL) {
                        if (load_display_fields) {
                            // 'load_display_fields' indicates we're in gui mode
                            // print unknown categories; for adding to input JSON file
                            //printf("warning: no colour for category %.*s\n", (int)(cur - start), start);
                        }
                        if (env->config->nbody.add_missing_cats) {
                            // include it in category set anyway, as it may still be needed to make fake links
                            if(category_set_add_category(env->category_set, start, cur - start, def_col)) {
                                // NOTE: WoS exceeds 256 categories hard limit, so check if limit exceeded
                                cat = category_set_get_by_name(env->category_set, start, cur - start);
                            }
                        }
                    } 
                    if (cat != NULL) {
                        if (cat->cat_id > 255) {
                            // we use a byte to store the cat id, so it must be small enough
                            printf("error: too many categories to store as a byte\n");
                            exit(1);
                        }
                        paper->allcats[cat_num++] = cat->cat_id;
                    }
                    if (*cur == '\0') {
                        break;
                    }
                    start = cur + 1;
                }
            }
        }
        // fill in unused entries in allcats with UNKNOWN category
        for (; cat_num < COMMON_PAPER_MAX_CATS; cat_num++) {
            paper->allcats[cat_num] = CATEGORY_UNKNOWN_ID;
        }

        // load authors and title if wanted
        if (num_fields >= 4) {
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

    printf("read %d ids %u -- %u\n", env->num_papers, env->papers[0].id, env->papers[env->num_papers - 1].id);

    return true;
}

static paper_t *env_get_paper_by_id(env_t *env, unsigned int id) {
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
    const char *refs_table = env->config->sql.refs_table.name;
    const char *id         = env->config->sql.refs_table.field_id;
    const char *refs       = env->config->sql.refs_table.field_refs;
    vstr_t *vstr = env->vstr[VSTR_0];
    vstr_reset(vstr);
    vstr_printf(vstr, "SELECT %s,%s FROM %s",id,refs,refs_table);
    const char *where_clause = env->config->sql.meta_table.where_clause;
    if (strcmp(where_clause,"") != 0) {
        const char *meta_table = env->config->sql.meta_table.name;
        const char *meta_id = env->config->sql.meta_table.field_id;
        // NOTE: MySQL doesn't seem to be support LIMIT statement inside subquery i.e. can't include extra_clause
        vstr_printf(vstr, " WHERE %s IN (SELECT %s FROM %s WHERE (%s))",id,meta_id,meta_table,where_clause);
    }
    if (vstr_had_error(vstr)) {
        return false;
    }
    if (!env_query_many_rows(env, vstr_str(vstr), 2, &result)) {
        return false;
    }

    // find length of a single ref blob
    // note that order of these rblob properties important
    unsigned int len_blob = 4;
    if (env->config->sql.refs_table.rblob_order) len_blob += 2;
    if (env->config->sql.refs_table.rblob_freq)  len_blob += 2;
    if (env->config->sql.refs_table.rblob_cites) len_blob += 2;

    int total_refs = 0;
    while ((row = mysql_fetch_row(result))) {
        lens = mysql_fetch_lengths(result);
        paper_t *paper = env_get_paper_by_id(env, atoll(row[0]));
        if (paper != NULL) {
            unsigned long len = lens[1];
            if (len == 0) {
                paper->num_refs = 0;
                paper->refs = NULL;
                paper->refs_ref_freq = NULL;
                paper->refs_other_weight = NULL;
            } else {
                if (len % len_blob != 0) {
                    printf("length of refs blob should be a multiple of %u; got %lu\n", len_blob,len);
                    mysql_free_result(result);
                    return false;
                }
                paper->refs = m_new(paper_t*, len / len_blob);
                paper->refs_ref_freq = m_new(byte, len / len_blob);
                paper->refs_other_weight = NULL;
                if (paper->refs == NULL || paper->refs_ref_freq == NULL) {
                    mysql_free_result(result);
                    return false;
                }
                paper->num_refs = 0;
                for (int i = 0; i < len; i += len_blob) {
                    byte *buf = (byte*)row[1] + i;
                    unsigned int id = decode_le32(buf + 0);
                    if (id == paper->id) {
                        // make sure paper doesn't ref itself (yes, they exist, see eg 1202.2631)
                        continue;
                    }
                    paper->refs[paper->num_refs] = env_get_paper_by_id(env, id);
                    if (paper->refs[paper->num_refs] != NULL) {
                        paper->refs[paper->num_refs]->num_cites += 1;
                        unsigned short buf_index = 4, ref_freq = 1;
                        if (env->config->sql.refs_table.rblob_order) {
                            // refs blob contains reference order info
                            buf_index += 2;
                        }
                        if (env->config->sql.refs_table.rblob_freq) {
                            // refs blob contains reference frequency info
                            ref_freq = decode_le16(buf + buf_index);
                            if (ref_freq > 255) {
                                ref_freq = 255;
                            }
                            buf_index += 2;
                        }
                        if (env->config->sql.refs_table.rblob_cites) {
                            // refs blob contain reference cites info
                            if (env->config->nbody.use_external_cites) {
                                paper->refs[paper->num_refs]->num_graph_cites = decode_le16(buf + buf_index);
                            }
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

static bool env_load_keywords(env_t *env) {
    MYSQL_RES *result;
    MYSQL_ROW row;
    unsigned long *lens;

    printf("reading keywords\n");

    // get the keywords from the db
    const char *meta_table = env->config->sql.meta_table.name;
    const char *id         = env->config->sql.meta_table.field_id;
    const char *keywords   = env->config->sql.meta_table.field_keywords;
    if (strcmp(keywords,"") == 0) {
        printf("no keywords table specified, skipping...\n");
        return true;
    }
    vstr_t *vstr = env->vstr[VSTR_0];
    vstr_reset(vstr);
    vstr_printf(vstr, "SELECT %s,%s FROM %s",id,keywords,meta_table);
    const char *where_clause = env->config->sql.meta_table.where_clause;
    if (strcmp(where_clause,"") != 0) {
        vstr_printf(vstr, " WHERE (%s)", where_clause);
    }
    const char *extra_clause = env->config->sql.meta_table.extra_clause;
    if (strcmp(extra_clause,"") != 0) {
        vstr_printf(vstr, " %s", extra_clause);
    }
    if (vstr_had_error(vstr)) {
        return false;
    }
    if (!env_query_many_rows(env, vstr_str(vstr), 2, &result)) {
        return false;
    }

    int total_keywords = 0;
    while ((row = mysql_fetch_row(result))) {
        lens = mysql_fetch_lengths(result);
        paper_t *paper = env_get_paper_by_id(env, atoll(row[0]));
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
                paper->keywords = m_new(keyword_entry_t*, num_keywords);
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
                    keyword_entry_t *unique_keyword = (keyword_entry_t*)hashmap_lookup_or_insert(env->keyword_set, kw, kw_end - kw, true);
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

    printf("read %d unique, %d total keywords\n", (int)hashmap_get_total(env->keyword_set), total_keywords);

    return true;
}

bool mysql_load_papers(init_config_t *init_config, bool load_display_fields, category_set_t *category_set, int *num_papers_out, paper_t **papers_out, hashmap_t **keyword_set_out) {
    // set up environment
    env_t env;
    if (!env_set_up(&env, init_config)) {
        env_finish(&env, true);
        return false;
    }

    // set the category set for reference later
    env.category_set = category_set;

    // load the DB
    if (!env_load_ids(&env, load_display_fields)) {
        return false;
    }
    if (!env_load_refs(&env)) {
        return false;
    }
    if (!env_load_keywords(&env)) {
        // we have keywords but failed to load them
        return false;
    }
    if (!build_citation_links(env.num_papers, env.papers)) {
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

bool mysql_save_paper_positions(init_config_t *init_config, layout_t *layout) {
    // set up environment
    env_t env;
    if (!env_set_up(&env, init_config)) {
        env_finish(&env, true);
        return false;
    }

    // save positions
    const char *map_table = env.config->sql.map_table.name;
    const char *id_f      = env.config->sql.map_table.field_id;
    const char *x_f       = env.config->sql.map_table.field_x;
    const char *y_f       = env.config->sql.map_table.field_y;
    const char *r_f       = env.config->sql.map_table.field_r;
    vstr_t *vstr = env.vstr[VSTR_0];
    assert(layout->child_layout == NULL);
    int total_pos = 0;
    for (int i = 0; i < layout->num_nodes; i++) {
        layout_node_t *n = &layout->nodes[i];

        if (n->flags & LAYOUT_NODE_POS_VALID) {
            vstr_reset(vstr);
            int x, y, r;
            layout_node_export_quantities(n, &x, &y, &r);
            vstr_printf(vstr, "REPLACE INTO %s (%s,%s,%s,%s) VALUES (%u,%d,%d,%d)", map_table,id_f,x_f,y_f,r_f,n->paper->id, x, y, r);
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

bool mysql_load_paper_positions(init_config_t *init_config, layout_t *layout) {
    // set up environment
    env_t env;
    if (!env_set_up(&env,init_config)) {
        env_finish(&env, true);
        return false;
    }

    printf("reading map_data\n");

    // query the positions from the map_data table
    const char *map_table = env.config->sql.map_table.name;
    const char *id_f      = env.config->sql.map_table.field_id;
    const char *x_f       = env.config->sql.map_table.field_x;
    const char *y_f       = env.config->sql.map_table.field_y;
    vstr_t *vstr = env.vstr[VSTR_0];
    vstr_reset(vstr);
    vstr_printf(vstr, "SELECT %s,%s,%s FROM %s",id_f,x_f,y_f,map_table);
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
        layout_node_t *n = layout_get_node_by_id(layout, atoll(row[0]));
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
