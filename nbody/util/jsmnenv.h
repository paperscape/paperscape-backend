#ifndef __JSMNENV_H_
#define __JSMNENV_H_

#include <stdlib.h>
#include <stdio.h>

#include "jsmn.h"
#include "xiwilib.h"

#define JSMN_TOK_MAX (4000) // // need lots for papers with lots of references

typedef struct _jsmn_env_t {
    FILE *fp;
    vstr_t *js_buf;
    jsmntok_t js_tok[JSMN_TOK_MAX];
    jsmn_parser js_parser;
} jsmn_env_t;

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


bool jsmn_env_set_up(jsmn_env_t* jsmn_env, const char *filename);
bool jsmn_env_reset(jsmn_env_t *env, bool *more_objects);
void jsmn_env_finish(jsmn_env_t* jsmn_env);
bool jsmn_env_open_json_file(jsmn_env_t* jsmn_env, const char *filename);

bool jsmn_env_next_object(jsmn_env_t *jsmn_env, bool *more_objects);
bool jsmn_env_get_array_member(jsmn_env_t *jsmn_env, jsmntok_t *array, int wanted_member, jsmntok_t **found_token, jsmn_env_token_value_t *found_value);
bool jsmn_env_get_num_entries(jsmn_env_t *env, int *num_entries);
bool jsmn_env_error(jsmn_env_t *jsmn_env, const char *msg);

bool jsmn_env_get_object_member(jsmn_env_t *jsmn_env, jsmntok_t *object, const char *wanted_member, jsmntok_t **found_token, jsmn_env_token_value_t *found_value);

bool jsmn_env_get_object_member_value(jsmn_env_t *jsmn_env, jsmntok_t *object, const char *wanted_member, jsmn_env_value_kind_t wanted_kind, jsmn_env_token_value_t *found_value);
bool jsmn_env_get_object_member_value_boolean(jsmn_env_t *jsmn_env, jsmntok_t *object, const char *wanted_member, jsmn_env_token_value_t *found_value);
bool jsmn_env_get_object_member_token(jsmn_env_t *jsmn_env, jsmntok_t *object, const char *wanted_member, jsmntype_t wanted_type,  jsmntok_t **found_token);

#endif /* __JSMNENV_H_ */
