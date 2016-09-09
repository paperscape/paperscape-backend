#include <stdio.h>
#include <stdlib.h>
#include <assert.h>
#include <string.h>
#include <math.h>

#include "util/xiwilib.h"
#include "config.h"
#include "util/jsmnenv.h"

bool config_new(const char *filename, config_t **config) {
    // create new config
    *config = m_new(config_t,1);

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
