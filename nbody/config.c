#include <stdio.h>
#include <stdlib.h>
#include <assert.h>
#include <string.h>
#include <math.h>

#include "util/xiwilib.h"
#include "config.h"
#include "util/jsmn.h"

#define JSMN__TOK_MAX (100) // some sane default

typedef struct _jsmn_env_t {
    FILE *fp;
    vstr_t *js_buf;
    jsmntok_t js_tok[JSMN__TOK_MAX];
    jsmn_parser js_parser;
} jsmn_env_t;

static bool have_error(jsmn_env_t *jsmn_env, const char *msg) {
    printf("JSON error: %s\n", msg);
    return false;
}

static bool jsmn_env_set_up(jsmn_env_t* jsmn_env, const char *filename) {
    jsmn_env->fp = NULL;
    jsmn_env->js_buf = vstr_new();

    return true;
}

static void jsmn_env_finish(jsmn_env_t* jsmn_env) {
    if (jsmn_env->fp != NULL) {
        fclose(jsmn_env->fp);
    }
}

static bool jsmn_env_open_json_file(jsmn_env_t* jsmn_env, const char *filename) {
    if (jsmn_env->fp != NULL) {
        fclose(jsmn_env->fp);
    }

    // open the JSON file
    jsmn_env->fp = fopen(filename, "r");
    if (jsmn_env->fp == NULL) {
        printf("can't open JSON file '%s' for reading\n", filename);
        return false;
    }

    printf("opened JSON file '%s'\n", filename);

    return true;
}

// returns next non-whitespace character, or EOF
static int jsmn_env_next_char(jsmn_env_t *jsmn_env) {
    for (;;) {
        int c = fgetc(jsmn_env->fp);
        switch (c) {
            case '\t': case '\r': case '\n': case ' ':
                break;
            default:
                return c;
        }
    }
}

// reset JSON stream to start

// parse the next JSON object in the stream
static bool jsmn_env_next_object(jsmn_env_t *jsmn_env, bool *more_objects) {
    // get open brace of object
    int c = jsmn_env_next_char(jsmn_env);
    if (c != '{') {
        return have_error(jsmn_env, "expecting '{' to start object");
    }

    // read in string of object
    vstr_reset(jsmn_env->js_buf);
    vstr_add_byte(jsmn_env->js_buf, '{');
    int nest = 1;
    while (nest > 0) {
        c = fgetc(jsmn_env->fp);
        if (c == EOF) {
            return have_error(jsmn_env, "JSON file ended prematurely");
        }
        vstr_add_byte(jsmn_env->js_buf, c);
        // TODO don't look at braces inside strings
        switch (c) {
            case '{': nest++; break;
            case '}': nest--; break;
        }
    }

    // check character following object
    c = jsmn_env_next_char(jsmn_env);
    if (c == ']') {
        // end of array
        *more_objects = false;
        if (jsmn_env_next_char(jsmn_env) != EOF) {
            return have_error(jsmn_env, "JSON file had unexpected data following array");
        }
    } else if (c == ',') {
        // more entries in array
        *more_objects = true;
    } else {
        // end of file
        *more_objects = false;
    }

    jsmn_init(&jsmn_env->js_parser);
    jsmnerr_t ret = jsmn_parse(&jsmn_env->js_parser, vstr_str(jsmn_env->js_buf), jsmn_env->js_tok, JSMN__TOK_MAX);
    if (ret != JSMN_SUCCESS) {
        return have_error(jsmn_env, "error parsing JSON");
    }

    return true;
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
} jsmn_env_value_kind_t;

typedef struct _jsmn_env_token_value_t {
    jsmn_env_value_kind_t kind;
    const char *str;
    unsigned int uint;
    int sint;
    double real;
} jsmn_env_token_value_t;

// note that this function can modify the js_buf buffer
// for strings, it replaces the ending quote " by \0 to make a proper ASCIIZ string
static void jsmn_env_get_token_value(jsmn_env_t *jsmn_env, jsmntok_t *tok, jsmn_env_token_value_t *val) {
    const char *str = vstr_str(jsmn_env->js_buf) + tok->start;
    char *top = vstr_str(jsmn_env->js_buf) + tok->end;

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
jsmntok_t *jsmn_env_skip_object(jsmntok_t *object) {
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
                object = jsmn_env_skip_object(object);
            }
            return object;
        }
        default:
            return object;
    }
}

// array points to the starting array token, with following tokens being its members
bool jsmn_env_get_array_member(jsmn_env_t *jsmn_env, jsmntok_t *array, int wanted_member, jsmntok_t **found_token, jsmn_env_token_value_t *found_value) {
    // check we are given an array
    if (array->type != JSMN_ARRAY) {
        return have_error(jsmn_env, "expecting an array");
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
                jsmn_env_get_token_value(jsmn_env, array, found_value);
            }
            return true;
        }

        wanted_member--;
        array = jsmn_env_skip_object(array);
    }

    // not found
    return have_error(jsmn_env, "array does not have member");
}

// object points to the starting object token, with following tokens being its members
bool jsmn_env_get_object_member(jsmn_env_t *jsmn_env, jsmntok_t *object, const char *wanted_member, jsmntok_t **found_token, jsmn_env_token_value_t *found_value) {
    // check we are given an object
    if (object->type != JSMN_OBJECT) {
        return have_error(jsmn_env, "expecting an object");
    }

    // go through each element of the object
    int size = object->size;
    object += 1;
    for (; size >= 2; size -= 2) {
        jsmn_env_token_value_t vkey;
        jsmn_env_get_token_value(jsmn_env, object, &vkey);

        if (vkey.kind != JSMN_VALUE_STRING) {
            return have_error(jsmn_env, "expecting a string for object member name");
        }

        object = jsmn_env_skip_object(object);

        if (strcmp(vkey.str, wanted_member) == 0) {
            if (found_token != NULL) {
                *found_token = object;
            }
            if (found_value != NULL) {
                jsmn_env_get_token_value(jsmn_env, object, found_value);
            }
            return true;
        }

        object = jsmn_env_skip_object(object);
    }

    // not found
    return have_error(jsmn_env, "object does not have member");
}

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
