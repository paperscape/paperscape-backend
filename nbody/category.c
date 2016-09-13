#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "util/xiwilib.h"
#include "category.h"

category_set_t *category_set_new(void) {
    category_set_t *cats = m_new0(category_set_t, 1);
    cats->num_cats = 1;
    cats->cats[0].cat_id = 0;
    cats->cats[0].cat_name = "unknown";
    cats->hashmap = hashmap_new();
    return cats;
}

bool category_set_add_category(category_set_t *cats, const char *str, size_t n, float rgb[3]) {
    if (cats->num_cats >= CATEGORY_MAX_CATS) {
        printf("error: too many categories, cannot add %.*s\n", (int)n, str);
        return false;
    }

    // set new category
    category_info_t *cat = &cats->cats[cats->num_cats];
    cat->cat_id = cats->num_cats++;
    cat->cat_name = strndup(str, n);
    cat->r = rgb[0];
    cat->g = rgb[1];
    cat->b = rgb[2];
    cat->num = 0;
    cat->x = 0;
    cat->y = 0;

    // add category to hash table for fast lookup by name
    hashmap_entry_t *entry = hashmap_lookup_or_insert(cats->hashmap, str, n, true);
    if (entry->value != 0) {
        printf("error: category %.*s already exists\n", (int)n, str);
        return false;
    }
    entry->value = (uintptr_t)cat;
    return true;
}
