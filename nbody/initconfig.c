#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdbool.h>

#include "initconfig.h"
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

    // look for member: use_external_cites
    // =================================
    // set defaults
    (*config)->use_external_cites = false;
    // attempt to set from JSON file
    jsmn_env_token_value_t ues_val;
    if(jsmn_env_get_object_member_value_boolean(&jsmn_env, jsmn_env.js_tok, "use_external_cites", &ues_val)) {
        (*config)->use_external_cites = (ues_val.kind == JSMN_VALUE_TRUE);
    }

    // look for member: forces
    // =======================
    // set defaults
    (*config)->force_close_repulsion_a        = 1e9;
    (*config)->force_close_repulsion_b        = 1e14;
    (*config)->force_close_repulsion_c        = 1.1;
    (*config)->force_close_repulsion_d        = 0.6;
    (*config)->force_link_strength            = 1.17;
    (*config)->force_anti_gravity_falloff_rsq = 1e6;
    (*config)->force_use_ref_freq             = true;
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
            (*config)->force_use_ref_freq             = (use_rf_val.kind == JSMN_VALUE_TRUE);
        }
        if(jsmn_env_get_object_member_value(&jsmn_env, forces_tok, "anti_gravity_falloff_rsq", JSMN_VALUE_REAL, &anti_grav_val)) {
            (*config)->force_anti_gravity_falloff_rsq = anti_grav_val.real;
        }
        if(jsmn_env_get_object_member_value_boolean(&jsmn_env, forces_tok, "initial_close_repulsion", &do_cr_val)) {
            (*config)->force_initial_close_repulsion  = (do_cr_val.kind == JSMN_VALUE_TRUE);
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
            jsmn_env_token_value_t name_val, clause_val, id_val, title_val,authors_val,allcats_val, keywords_val, missing_cats_val;
            if(jsmn_env_get_object_member_value(&jsmn_env, meta_tok, "name", JSMN_VALUE_STRING, &name_val)) {
                (*config)->sql_meta_name = strdup(name_val.str);
            }
            if(jsmn_env_get_object_member_value(&jsmn_env, meta_tok, "clause", JSMN_VALUE_STRING, &clause_val)) {
                (*config)->sql_meta_clause = strdup(clause_val.str);
            }
            if(jsmn_env_get_object_member_value(&jsmn_env, meta_tok, "field_id", JSMN_VALUE_STRING, &id_val)) {
                (*config)->sql_meta_field_id = strdup(id_val.str);
            }
            if(jsmn_env_get_object_member_value(&jsmn_env, meta_tok, "field_title", JSMN_VALUE_STRING, &title_val)) {
                (*config)->sql_meta_field_title = strdup(title_val.str);
            }
            if(jsmn_env_get_object_member_value(&jsmn_env, meta_tok, "field_authors", JSMN_VALUE_STRING, &authors_val)) {
                (*config)->sql_meta_field_authors = strdup(authors_val.str);
            }
            if(jsmn_env_get_object_member_value(&jsmn_env, meta_tok, "field_allcats", JSMN_VALUE_STRING, &allcats_val)) {
                (*config)->sql_meta_field_allcats = strdup(allcats_val.str);
            }
            if(jsmn_env_get_object_member_value(&jsmn_env, meta_tok, "field_keywords", JSMN_VALUE_STRING, &keywords_val)) {
                (*config)->sql_meta_field_keywords = strdup(keywords_val.str);
            }
            if(jsmn_env_get_object_member_value_boolean(&jsmn_env, meta_tok, "add_missing_cats", &missing_cats_val)) {
                (*config)->sql_meta_add_missing_cats = (missing_cats_val.kind == JSMN_VALUE_TRUE);
            }
        }
        // look for member: refs_table
        // ---------------------------
        jsmntok_t *refs_tok;
        if(jsmn_env_get_object_member_token(&jsmn_env, sql_tok, "refs_table", JSMN_OBJECT, &refs_tok)) {
            jsmn_env_token_value_t name_val, id_val, refs_val, ref_freq_val, ref_order_val, ref_cites_val;
            if(jsmn_env_get_object_member_value(&jsmn_env, refs_tok, "name", JSMN_VALUE_STRING, &name_val)) {
                (*config)->sql_refs_name = strdup(name_val.str);
            }
            if(jsmn_env_get_object_member_value(&jsmn_env, refs_tok, "field_id", JSMN_VALUE_STRING, &id_val)) {
                (*config)->sql_refs_field_id = strdup(id_val.str);
            }
            if(jsmn_env_get_object_member_value(&jsmn_env, refs_tok, "field_refs", JSMN_VALUE_STRING, &refs_val)) {
                (*config)->sql_refs_field_refs = strdup(refs_val.str);
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
                (*config)->sql_map_name = strdup(name_val.str);
            }
            if(jsmn_env_get_object_member_value(&jsmn_env, map_tok, "field_id", JSMN_VALUE_STRING, &id_val)) {
                (*config)->sql_map_field_id = strdup(id_val.str);
            }
            if(jsmn_env_get_object_member_value(&jsmn_env, map_tok, "field_x", JSMN_VALUE_STRING, &x_val)) {
                (*config)->sql_map_field_x = strdup(x_val.str);
            }
            if(jsmn_env_get_object_member_value(&jsmn_env, map_tok, "field_y", JSMN_VALUE_STRING, &y_val)) {
                (*config)->sql_map_field_y = strdup(y_val.str);
            }
            if(jsmn_env_get_object_member_value(&jsmn_env, map_tok, "field_r", JSMN_VALUE_STRING, &r_val)) {
                (*config)->sql_map_field_r = strdup(r_val.str);
            }
        }
    }

    // finish up
    jsmn_env_finish(&jsmn_env);

    return true;
}

