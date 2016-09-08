#ifndef _INCLUDED_HASHMAP_H
#define _INCLUDED_HASHMAP_H

#include <stdint.h>
#include <unistd.h>

// a generic entry in the hashmap, to be cast to a specific struct (of same layout)
typedef struct _hashmap_entry_t {
    char *key;
    uintptr_t value;
} hashmap_entry_t;

typedef struct _hashmap_t hashmap_t;

hashmap_t *hashmap_new();
void hashmap_free(hashmap_t *hm);
size_t hashmap_get_total(hashmap_t *hm);
void hashmap_clear_all_values(hashmap_t *hm, uintptr_t reset_value);
hashmap_entry_t *hashmap_lookup_or_insert(hashmap_t *hm, const char *key, size_t key_len);

#endif // _INCLUDED_HASHMAP_H
