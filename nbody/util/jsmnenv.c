#include <stdlib.h>
#include <string.h>
#include <math.h>

#include "jsmn.h"
#include "jsmnenv.h"

bool jsmn_env_error(jsmn_env_t *jsmn_env, const char *msg) {
    printf("JSON: %s\n", msg);
    return false;
}


bool jsmn_env_set_up(jsmn_env_t* jsmn_env, const char *filename) {
    jsmn_env->fp = NULL;
    jsmn_env->js_buf = vstr_new();

    return true;
}

void jsmn_env_finish(jsmn_env_t* jsmn_env) {
    if (jsmn_env->fp != NULL) {
        fclose(jsmn_env->fp);
    }
}

bool jsmn_env_open_json_file(jsmn_env_t* jsmn_env, const char *filename) {
    if (jsmn_env->fp != NULL) {
        fclose(jsmn_env->fp);
    }

    // open the JSON file
    jsmn_env->fp = fopen(filename, "r");
    if (jsmn_env->fp == NULL) {
        printf("JSON error: can't open file '%s' for reading\n", filename);
        return false;
    }

    printf("JSON: opened file '%s'\n", filename);

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
bool jsmn_env_reset(jsmn_env_t *env, bool *more_objects) {
    // reset file to start
    fseek(env->fp, 0, SEEK_SET);

    // seek into the outer JSON array
    if (jsmn_env_next_char(env) != '[') {
        return jsmn_env_error(env, "expecting '[' to start array");
    }

    // check if empty array or not
    int c = jsmn_env_next_char(env);
    if (c == ']') {
        *more_objects = false;
    } else {
        ungetc(c, env->fp);
        *more_objects = true;
    }

    return true;
}

bool jsmn_env_get_num_entries(jsmn_env_t *env, int *num_entries) {
    *num_entries = 0;
    bool more_objects;
    if (!jsmn_env_reset(env, &more_objects)) {
        return false;
    }
    while (more_objects) {
        if (!jsmn_env_next_object(env, &more_objects)) {
            return false;
        }
        *num_entries += 1;
    }
    return true;
}

// parse the next JSON object in the stream
bool jsmn_env_next_object(jsmn_env_t *jsmn_env, bool *more_objects) {
    // get open brace of object
    int c = jsmn_env_next_char(jsmn_env);
    if (c != '{') {
        return jsmn_env_error(jsmn_env, "expecting '{' to start object");
    }

    // read in string of object
    vstr_reset(jsmn_env->js_buf);
    vstr_add_byte(jsmn_env->js_buf, '{');
    int nest = 1;
    while (nest > 0) {
        c = fgetc(jsmn_env->fp);
        if (c == EOF) {
            return jsmn_env_error(jsmn_env, "JSON file ended prematurely");
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
            return jsmn_env_error(jsmn_env, "JSON file had unexpected data following array");
        }
    } else if (c == ',') {
        // more entries in array
        *more_objects = true;
    }

    jsmn_init(&jsmn_env->js_parser);
    jsmnerr_t ret = jsmn_parse(&jsmn_env->js_parser, vstr_str(jsmn_env->js_buf), jsmn_env->js_tok, JSMN_TOK_MAX);
    if (ret != JSMN_SUCCESS) {
        return jsmn_env_error(jsmn_env, "error parsing JSON");
    }

    return true;
}

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
        return jsmn_env_error(jsmn_env, "expecting an array");
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
    return jsmn_env_error(jsmn_env, "array does not have member");
}

// object points to the starting object token, with following tokens being its members
bool jsmn_env_get_object_member(jsmn_env_t *jsmn_env, jsmntok_t *object, const char *wanted_member, jsmntok_t **found_token, jsmn_env_token_value_t *found_value) {
    // check we are given an object
    if (object->type != JSMN_OBJECT) {
        return jsmn_env_error(jsmn_env, "expecting an object");
    }

    // go through each element of the object
    int size = object->size;
    object += 1;
    for (; size >= 2; size -= 2) {
        jsmn_env_token_value_t vkey;
        jsmn_env_get_token_value(jsmn_env, object, &vkey);

        if (vkey.kind != JSMN_VALUE_STRING) {
            return jsmn_env_error(jsmn_env, "expecting a string for object member name");
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
    vstr_t *msg = vstr_new();
    vstr_add_str(msg,"object does not have member: ");
    vstr_add_str(msg,wanted_member);
    jsmn_env_error(jsmn_env, vstr_str(msg));
    vstr_free(msg);
    return false;
}

// similar to jsmn_env_get_object_member, but only gives value, which must match given kind
bool jsmn_env_get_object_member_value(jsmn_env_t *jsmn_env, jsmntok_t *object, const char *wanted_member, jsmn_env_value_kind_t wanted_kind, jsmn_env_token_value_t *found_value) {
    if (found_value == NULL) {
        return jsmn_env_error(jsmn_env, "object member value can't assign to null");
    }
    if(!jsmn_env_get_object_member(jsmn_env,object,wanted_member,NULL,found_value)) {
        return false;
    }
    if (found_value->kind != wanted_kind) {
        return jsmn_env_error(jsmn_env, "object member value is of wrong kind");
    }
    return true;
}

// similar to jsmn_env_get_object_member, but only gives value, which must be boolean
bool jsmn_env_get_object_member_value_boolean(jsmn_env_t *jsmn_env, jsmntok_t *object, const char *wanted_member, jsmn_env_token_value_t *found_value) {
    if (found_value == NULL) {
        return jsmn_env_error(jsmn_env, "object member value can't assign to null");
    }
    if(!jsmn_env_get_object_member(jsmn_env,object,wanted_member,NULL,found_value)) {
        return false;
    }
    if (found_value->kind != JSMN_VALUE_TRUE && found_value->kind != JSMN_VALUE_FALSE) {
        return jsmn_env_error(jsmn_env, "object member value is of non-boolean kind");
    }
    return true;
}

// similar to jsmn_env_get_object_member, but only gives token, which must match given type
bool jsmn_env_get_object_member_token(jsmn_env_t *jsmn_env, jsmntok_t *object, const char *wanted_member, jsmntype_t wanted_type, jsmntok_t **found_token) {
    if (found_token == NULL) {
        return jsmn_env_error(jsmn_env, "object member token can't assign to null");
    }
    if(!jsmn_env_get_object_member(jsmn_env,object,wanted_member,found_token,NULL)) {
        return false;
    }
    if ((*found_token)->type != wanted_type) {
        return jsmn_env_error(jsmn_env, "object member token is of wrong type");
    }
    return true;
}
