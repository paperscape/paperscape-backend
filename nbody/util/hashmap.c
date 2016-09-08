#include <string.h>

#include "xiwilib.h"
#include "hashmap.h"

// the hashmap pool is a linked list of hash tables, each one bigger than the previous
typedef struct _hashmap_pool_t {
    size_t size;
    hashmap_entry_t *table;
    size_t used;
    bool full;
    struct _hashmap_pool_t *next;
} hashmap_pool_t;

struct _hashmap_t {
    hashmap_pool_t *pool;
};

// approximatelly doubling primes; made with Mathematica command: Table[Prime[Floor[(1.7)^n]], {n, 9, 24}]
static size_t doubling_primes[] = {647, 1229, 2297, 4243, 7829, 14347, 26017, 47149, 84947, 152443, 273253, 488399, 869927, 1547173, 2745121, 4861607};

hashmap_t *hashmap_new() {
    hashmap_t *hm = m_new(hashmap_t, 1);
    hm->pool = NULL;
    return hm;
}

void hashmap_free(hashmap_t * hm) {
    if (hm == NULL) {
        return;
    }

    for (hashmap_pool_t *hmp = hm->pool; hmp != NULL;) {
        hashmap_pool_t *next = hmp->next;
        m_free(hmp->table);
        m_free(hmp);
        hmp = next;
    }

    m_free(hm);
}

size_t hashmap_get_total(hashmap_t *hm) {
    size_t n = 0;
    for (hashmap_pool_t *hmp = hm->pool; hmp != NULL; hmp = hmp->next) {
        n += hmp->used;
    }
    return n;
}

void hashmap_clear_all_values(hashmap_t *hm, uintptr_t reset_value) {
    for (hashmap_pool_t *hmp = hm->pool; hmp != NULL; hmp = hmp->next) {
        for (size_t i = 0; i < hmp->size; ++i) {
            hmp->table[i].value = reset_value;
        }
    }
}

hashmap_entry_t *hashmap_lookup_or_insert(hashmap_t *hm, const char *key, size_t key_len) {
    if (key_len == 0) {
        return NULL;
    }

    unsigned int hash = strnhash(key, key_len);
    hashmap_pool_t *hmp;
    hashmap_pool_t *avail_hmp = NULL;
    size_t avail_pos = 0;

    // first search for hashmap to see if we already have it
    for (hmp = hm->pool; hmp != NULL; hmp = hmp->next) {
        size_t pos = hash % hmp->size;
        for (;;) {
            hashmap_entry_t *found = &hmp->table[pos];
            if (found->key == NULL) {
                // key not in table
                if (!hmp->full) {
                    // table not full, so we can use this entry if we don't find the key
                    avail_hmp = hmp;
                    avail_pos = pos;
                }
                break;
            } else if (strneq(found->key, key, key_len)) {
                // found it
                return found;
            } else {
                // not yet found, keep searching in this table
                pos = (pos + 1) % hmp->size;
            }
        }
    }

    // key not found in any table

    // found an available slot, so use it
    if (avail_hmp != NULL) {
        avail_hmp->table[avail_pos].key = strndup(key, key_len);
        avail_hmp->used += 1;
        if (10 * avail_hmp->used > 8 * avail_hmp->size) {
            // set the full flag if we reached a certain fraction of used entries
            avail_hmp->full = true;
        }
        return &avail_hmp->table[avail_pos];
    }

    // no available slots, so make a new table
    hmp = m_new(hashmap_pool_t, 1);
    if (hmp == NULL) {
        return NULL;
    }
    if (hm->pool == NULL) {
        // first table
        hmp->size = doubling_primes[0];
    } else {
        // successive tables
        for (int i = 0; i < sizeof(doubling_primes) / sizeof(int); i++) {
            hmp->size = doubling_primes[i];
            if (doubling_primes[i] > hm->pool->size) {
                break;
            }
        }
    }
    hmp->table = m_new0(hashmap_entry_t, hmp->size);
    if (hmp->table == NULL) {
        m_free(hmp);
        return NULL;
    }
    hmp->used = 0;
    hmp->full = false;
    hmp->next = hm->pool;
    hm->pool = hmp;

    // make and insert new entry
    hmp->table[hash % hmp->size].key = strndup(key, key_len);
    hmp->used += 1;

    // return new hashmap
    return &hmp->table[hash % hmp->size];
}
