#ifndef _INCLUDED_CATEGORY_H
#define _INCLUDED_CATEGORY_H

#include "util/hashmap.h"

// this special identifier is for unknown categories
#define CATEGORY_UNKNOWN_ID (0)

// hardcoded maximum for efficiency/simplicity
#define CATEGORY_MAX_CATS (256)

typedef struct _category_info_t {
    size_t cat_id;          // unique id of this category
    const char *cat_name;   // name of this category
    float r, g, b;          // colour of this category
    size_t num;             // number of papers in this category
    float x, y;             // position of this category
} category_info_t;

typedef struct _category_set_t {
    size_t num_cats;
    category_info_t cats[CATEGORY_MAX_CATS];
    hashmap_t *hashmap;
} category_set_t;

category_set_t *category_set_new(void);
bool category_set_add_category(category_set_t *cats, const char *str, size_t n, float rgb[3]);

static inline size_t category_set_get_num(category_set_t *cats) {
    return cats->num_cats;
}

static inline category_info_t *category_set_get_by_id(category_set_t *cats, size_t cat_id) {
    return &cats->cats[cat_id];
}

static inline category_info_t *category_set_get_by_name(category_set_t *cats, const char *str, size_t n) {
    hashmap_entry_t *entry = hashmap_lookup_or_insert(cats->hashmap, str, n, false);
    if (entry == NULL) {
        return NULL;
    } else {
        return (category_info_t*)entry->value;
    }
}

#endif // _INCLUDED_CATEGORY_H
