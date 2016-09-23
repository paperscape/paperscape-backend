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

    // look for member: nbody
    // =================================
    // set defaults
    (*config)->nbody.use_external_cites = false;
    (*config)->nbody.mass_cites_exponent = 1.;
    (*config)->nbody.forces.close_repulsion_a        = 1e9;
    (*config)->nbody.forces.close_repulsion_b        = 1e14;
    (*config)->nbody.forces.close_repulsion_c        = 1.1;
    (*config)->nbody.forces.close_repulsion_d        = 0.6;
    (*config)->nbody.forces.link_strength            = 1.17;
    (*config)->nbody.forces.anti_gravity_falloff_rsq = 1e6;
    (*config)->nbody.forces.use_ref_freq             = true;
    (*config)->nbody.forces.initial_close_repulsion  = false;
    // attempt to set from JSON file
    jsmntok_t *nbody_tok;
    if(jsmn_env_get_object_member_token(&jsmn_env, jsmn_env.js_tok, "nbody", JSMN_OBJECT, &nbody_tok)) {
        // look for member: use_external_cites
        // =================================
        jsmn_env_token_value_t ues_val;
        if(jsmn_env_get_object_member_value_boolean(&jsmn_env, nbody_tok, "use_external_cites", &ues_val)) {
            (*config)->nbody.use_external_cites = (ues_val.kind == JSMN_VALUE_TRUE);
        }
        // look for member: mass_cites_exponent
        // =================================
        jsmn_env_token_value_t mce_val;
        if(jsmn_env_get_object_member_value(&jsmn_env, nbody_tok, "mass_cites_exponent", JSMN_VALUE_REAL, &mce_val)) {
            (*config)->nbody.mass_cites_exponent = mce_val.real;
        }
        // look for member: forces
        // =======================
        jsmntok_t *forces_tok;
        if(jsmn_env_get_object_member_token(&jsmn_env, nbody_tok, "forces", JSMN_OBJECT, &forces_tok)) {
            jsmn_env_token_value_t do_cr_val, use_rf_val, cr_a_val, cr_b_val, cr_c_val, cr_d_val, link_val, anti_grav_val;
            if(jsmn_env_get_object_member_value(&jsmn_env, forces_tok, "close_repulsion_a", JSMN_VALUE_REAL, &cr_a_val)) {
                (*config)->nbody.forces.close_repulsion_a        = cr_a_val.real;
            }
            if(jsmn_env_get_object_member_value(&jsmn_env, forces_tok, "close_repulsion_b", JSMN_VALUE_REAL, &cr_b_val)) {
                (*config)->nbody.forces.close_repulsion_b        = cr_b_val.real;
            }
            if(jsmn_env_get_object_member_value(&jsmn_env, forces_tok, "close_repulsion_c", JSMN_VALUE_REAL, &cr_c_val)) {
                (*config)->nbody.forces.close_repulsion_c        = cr_c_val.real;
            }
            if(jsmn_env_get_object_member_value(&jsmn_env, forces_tok, "close_repulsion_d", JSMN_VALUE_REAL, &cr_d_val)) {
                (*config)->nbody.forces.close_repulsion_d        = cr_d_val.real;
            }
            if(jsmn_env_get_object_member_value(&jsmn_env, forces_tok, "link_strength", JSMN_VALUE_REAL, &link_val)) {
                (*config)->nbody.forces.link_strength            = link_val.real;
            }
            if(jsmn_env_get_object_member_value_boolean(&jsmn_env, forces_tok, "use_ref_freq", &use_rf_val)) {
                (*config)->nbody.forces.use_ref_freq             = (use_rf_val.kind == JSMN_VALUE_TRUE);
            }
            if(jsmn_env_get_object_member_value(&jsmn_env, forces_tok, "anti_gravity_falloff_rsq", JSMN_VALUE_REAL, &anti_grav_val)) {
                (*config)->nbody.forces.anti_gravity_falloff_rsq = anti_grav_val.real;
            }
            if(jsmn_env_get_object_member_value_boolean(&jsmn_env, forces_tok, "initial_close_repulsion", &do_cr_val)) {
                (*config)->nbody.forces.initial_close_repulsion  = (do_cr_val.kind == JSMN_VALUE_TRUE);
            }
        }
        // look for member: map_orientation
        // ================================
        jsmntok_t *map_orient_tok;
        if(jsmn_env_get_object_member_token(&jsmn_env, nbody_tok, "map_orientation", JSMN_OBJECT, &map_orient_tok)) {
            jsmn_env_token_value_t cat_val, angle_val;
            if(jsmn_env_get_object_member_value(&jsmn_env, map_orient_tok, "category", JSMN_VALUE_STRING, &cat_val)) {
                (*config)->nbody.map_orientation.category = strdup(cat_val.str);
            }
            if(jsmn_env_get_object_member_value(&jsmn_env, map_orient_tok, "angle", JSMN_VALUE_REAL, &angle_val)) {
                (*config)->nbody.map_orientation.angle    = angle_val.real;
            }
        }
    }

    // look for member: tiles
    // =================================
    // set defaults
    (*config)->tiles.background_col[0] = 0;
    (*config)->tiles.background_col[1] = 0;
    (*config)->tiles.background_col[2] = 0;
    // attempt to set from JSON file
    jsmntok_t *tiles_tok;
    if(jsmn_env_get_object_member_token(&jsmn_env, jsmn_env.js_tok, "tiles", JSMN_OBJECT, &tiles_tok)) {
        // look for member: background_col
        // =================================
        jsmntok_t *bkgd_tok;
        if (jsmn_env_get_object_member_token(&jsmn_env, tiles_tok, "background_col", JSMN_ARRAY, &bkgd_tok)) {
            if (bkgd_tok->size != 3) {
                return jsmn_env_error(&jsmn_env,"expecting an array of size 3");
            }
            // parse the r,g,b values
            for (int i = 0; i < 3; i++) {
                // get current element
                jsmn_env_token_value_t elem_val;
                if (!jsmn_env_get_array_member(&jsmn_env, bkgd_tok, i, NULL, &elem_val)) {
                    return false;
                }

                // check the element is a number
                if (elem_val.kind == JSMN_VALUE_UINT) {
                    (*config)->tiles.background_col[i] = elem_val.uint;
                } else if (elem_val.kind == JSMN_VALUE_REAL) {
                    (*config)->tiles.background_col[i] = elem_val.real;
                } else {
                    return jsmn_env_error(&jsmn_env,"expecting a number");
                }
            }
        }
    }

    // look for member: sql 
    // ====================
    // set defaults
    // fields defaulted to empty are not used if not specified
    (*config)->sql.meta_table.name           = "meta_data";
    (*config)->sql.meta_table.where_clause   = "arxiv IS NOT NULL AND status != 'WDN'";
    (*config)->sql.meta_table.extra_clause   = "";
    (*config)->sql.meta_table.field_id       = "id";
    (*config)->sql.meta_table.field_allcats  = "allcats";
    (*config)->sql.meta_table.field_title    = "";
    (*config)->sql.meta_table.field_authors  = "";
    (*config)->sql.meta_table.field_keywords = "";
    (*config)->sql.refs_table.name           = "pcite";
    (*config)->sql.refs_table.field_id       = "id";
    (*config)->sql.refs_table.field_refs     = "refs";
    (*config)->sql.refs_table.rblob_order    = true;
    (*config)->sql.refs_table.rblob_freq     = true;
    (*config)->sql.refs_table.rblob_cites    = true;
    // attempt to set from JSON file
    jsmntok_t *sql_tok;
    if(jsmn_env_get_object_member_token(&jsmn_env, jsmn_env.js_tok, "sql", JSMN_OBJECT, &sql_tok)) {
        // look for member: meta_table
        // ---------------------------
        jsmntok_t *meta_tok;
        if(jsmn_env_get_object_member_token(&jsmn_env, sql_tok, "meta_table", JSMN_OBJECT, &meta_tok)) {
            jsmn_env_token_value_t name_val, where_clause_val, extra_clause_val, id_val, title_val,authors_val,allcats_val, keywords_val, missing_cats_val;
            if(jsmn_env_get_object_member_value(&jsmn_env, meta_tok, "name", JSMN_VALUE_STRING, &name_val)) {
                (*config)->sql.meta_table.name = strdup(name_val.str);
            }
            if(jsmn_env_get_object_member_value(&jsmn_env, meta_tok, "where_clause", JSMN_VALUE_STRING, &where_clause_val)) {
                (*config)->sql.meta_table.where_clause = strdup(where_clause_val.str);
            }
            if(jsmn_env_get_object_member_value(&jsmn_env, meta_tok, "extra_clause", JSMN_VALUE_STRING, &extra_clause_val)) {
                (*config)->sql.meta_table.extra_clause = strdup(extra_clause_val.str);
            }
            if(jsmn_env_get_object_member_value(&jsmn_env, meta_tok, "field_id", JSMN_VALUE_STRING, &id_val)) {
                (*config)->sql.meta_table.field_id = strdup(id_val.str);
            }
            if(jsmn_env_get_object_member_value(&jsmn_env, meta_tok, "field_title", JSMN_VALUE_STRING, &title_val)) {
                (*config)->sql.meta_table.field_title = strdup(title_val.str);
            }
            if(jsmn_env_get_object_member_value(&jsmn_env, meta_tok, "field_authors", JSMN_VALUE_STRING, &authors_val)) {
                (*config)->sql.meta_table.field_authors = strdup(authors_val.str);
            }
            if(jsmn_env_get_object_member_value(&jsmn_env, meta_tok, "field_allcats", JSMN_VALUE_STRING, &allcats_val)) {
                (*config)->sql.meta_table.field_allcats = strdup(allcats_val.str);
            }
            if(jsmn_env_get_object_member_value(&jsmn_env, meta_tok, "field_keywords", JSMN_VALUE_STRING, &keywords_val)) {
                (*config)->sql.meta_table.field_keywords = strdup(keywords_val.str);
            }
            if(jsmn_env_get_object_member_value_boolean(&jsmn_env, meta_tok, "add_missing_cats", &missing_cats_val)) {
                (*config)->sql.meta_table.add_missing_cats = (missing_cats_val.kind == JSMN_VALUE_TRUE);
            }
        }
        // look for member: refs_table
        // ---------------------------
        jsmntok_t *refs_tok;
        if(jsmn_env_get_object_member_token(&jsmn_env, sql_tok, "refs_table", JSMN_OBJECT, &refs_tok)) {
            jsmn_env_token_value_t name_val, id_val, refs_val, ref_freq_val, ref_order_val, ref_cites_val;
            if(jsmn_env_get_object_member_value(&jsmn_env, refs_tok, "name", JSMN_VALUE_STRING, &name_val)) {
                (*config)->sql.refs_table.name = strdup(name_val.str);
            }
            if(jsmn_env_get_object_member_value(&jsmn_env, refs_tok, "field_id", JSMN_VALUE_STRING, &id_val)) {
                (*config)->sql.refs_table.field_id = strdup(id_val.str);
            }
            if(jsmn_env_get_object_member_value(&jsmn_env, refs_tok, "field_refs", JSMN_VALUE_STRING, &refs_val)) {
                (*config)->sql.refs_table.field_refs = strdup(refs_val.str);
            }
            if(jsmn_env_get_object_member_value_boolean(&jsmn_env, refs_tok, "rblob_order", &ref_order_val)) {
                (*config)->sql.refs_table.rblob_order = (ref_order_val.kind == JSMN_VALUE_TRUE);
            }
            if(jsmn_env_get_object_member_value_boolean(&jsmn_env, refs_tok, "rblob_freq", &ref_freq_val)) {
                (*config)->sql.refs_table.rblob_freq  = (ref_freq_val.kind  == JSMN_VALUE_TRUE);
            }
            if(jsmn_env_get_object_member_value_boolean(&jsmn_env, refs_tok, "rblob_cites", &ref_cites_val)) {
                (*config)->sql.refs_table.rblob_cites = (ref_cites_val.kind == JSMN_VALUE_TRUE);
            }
        }
        // look for member: map_table
        // ---------------------------
        jsmntok_t *map_tok;
        if(jsmn_env_get_object_member_token(&jsmn_env, sql_tok, "map_table", JSMN_OBJECT, &map_tok)) {
            jsmn_env_token_value_t name_val, id_val, x_val, y_val, r_val;
            if(jsmn_env_get_object_member_value(&jsmn_env, map_tok, "name", JSMN_VALUE_STRING, &name_val)) {
                (*config)->sql.map_table.name = strdup(name_val.str);
            }
            if(jsmn_env_get_object_member_value(&jsmn_env, map_tok, "field_id", JSMN_VALUE_STRING, &id_val)) {
                (*config)->sql.map_table.field_id = strdup(id_val.str);
            }
            if(jsmn_env_get_object_member_value(&jsmn_env, map_tok, "field_x", JSMN_VALUE_STRING, &x_val)) {
                (*config)->sql.map_table.field_x = strdup(x_val.str);
            }
            if(jsmn_env_get_object_member_value(&jsmn_env, map_tok, "field_y", JSMN_VALUE_STRING, &y_val)) {
                (*config)->sql.map_table.field_y = strdup(y_val.str);
            }
            if(jsmn_env_get_object_member_value(&jsmn_env, map_tok, "field_r", JSMN_VALUE_STRING, &r_val)) {
                (*config)->sql.map_table.field_r = strdup(r_val.str);
            }
        }
    }

    // finish up
    jsmn_env_finish(&jsmn_env);

    return true;
}

