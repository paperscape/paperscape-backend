#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <assert.h>

#include "xiwilib.h"
#include "common.h"

void paper_init(paper_t *p, unsigned int id) {
    // all entries have initial state which is 0x00
    memset(p, 0, sizeof(paper_t));
    // set the paper id
    p->id = id;
}

// the keyword pool is a linked list of hash tables, each one bigger than the previous
typedef struct _keyword_pool_t {
    int size;
    keyword_t *table;
    int used;
    bool full;
    struct _keyword_pool_t *next;
} keyword_pool_t;

struct _keyword_set_t {
    keyword_pool_t *pool;
};

// approximatelly doubling primes; made with Mathematica command: Table[Prime[Floor[(1.7)^n]], {n, 9, 24}]
static int doubling_primes[] = {647, 1229, 2297, 4243, 7829, 14347, 26017, 47149, 84947, 152443, 273253, 488399, 869927, 1547173, 2745121, 4861607};

keyword_set_t *keyword_set_new() {
    keyword_set_t *kws = m_new(keyword_set_t, 1);
    kws->pool = NULL;
    return kws;
}

void keyword_set_free(keyword_set_t * kws) {
    if (kws == NULL) {
        return;
    }

    for (keyword_pool_t *kwp = kws->pool; kwp != NULL;) {
        keyword_pool_t *next = kwp->next;
        m_free(kwp->table);
        m_free(kwp);
        kwp = next;
    }

    m_free(kws);
}

int keyword_set_get_total(keyword_set_t *kws) {
    int n = 0;
    for (keyword_pool_t *kwp = kws->pool; kwp != NULL; kwp = kwp->next) {
        n += kwp->used;
    }
    return n;
}

void keyword_set_clear_data(keyword_set_t *kws) {
    for (keyword_pool_t *kwp = kws->pool; kwp != NULL; kwp = kwp->next) {
        for (int i = 0; i < kwp->size; i++) {
            kwp->table[i].paper = NULL;
        }
    }
}

keyword_t *keyword_set_lookup_or_insert(keyword_set_t *kws, const char *kw, size_t kw_len) {
    if (kw_len <= 0) {
        return NULL;
    }

    unsigned int hash = strnhash(kw, kw_len);
    keyword_pool_t *kwp;
    keyword_pool_t *avail_kwp = NULL;
    int avail_pos = 0;

    // first search for keyword to see if we already have it
    for (kwp = kws->pool; kwp != NULL; kwp = kwp->next) {
        int pos = hash % kwp->size;
        for (;;) {
            keyword_t *found_kw = &kwp->table[pos];
            if (found_kw->keyword == NULL) {
                // kw not in table
                if (!kwp->full) {
                    // table not full, so we can use this entry if we don't find the kw
                    avail_kwp = kwp;
                    avail_pos = pos;
                }
                break;
            } else if (strneq(found_kw->keyword, kw, kw_len)) {
                // found it
                return found_kw;
            } else {
                // not yet found, keep searching in this table
                pos = (pos + 1) % kwp->size;
            }
        }
    }

    // keyword not found in any table

    // found an available slot, so use it
    if (avail_kwp != NULL) {
        avail_kwp->table[avail_pos].keyword = strndup(kw, kw_len);
        avail_kwp->used += 1;
        if (10 * avail_kwp->used > 8 * avail_kwp->size) {
            // set the full flag if we reached a certain fraction of used entries
            avail_kwp->full = true;
        }
        return &avail_kwp->table[avail_pos];
    }

    // no available slots, so make a new table
    kwp = m_new(keyword_pool_t, 1);
    if (kwp == NULL) {
        return NULL;
    }
    if (kws->pool == NULL) {
        // first table
        kwp->size = doubling_primes[0];
    } else {
        // successive tables
        for (int i = 0; i < sizeof(doubling_primes) / sizeof(int); i++) {
            kwp->size = doubling_primes[i];
            if (doubling_primes[i] > kws->pool->size) {
                break;
            }
        }
    }
    kwp->table = m_new0(keyword_t, kwp->size);
    if (kwp->table == NULL) {
        m_free(kwp);
        return NULL;
    }
    kwp->used = 0;
    kwp->full = false;
    kwp->next = kws->pool;
    kws->pool = kwp;

    // make and insert new keyword
    kwp->table[hash % kwp->size].keyword = strndup(kw, kw_len);
    kwp->used += 1;

    // return new keyword
    return &kwp->table[hash % kwp->size];
}

static const char *category_string[] = {
    "unknown",
    "inspire",
#define CAT(id, str) str,
#include "cats.h"
#undef CAT
};

int date_to_unique_id(int y, int m, int d) {
    return (y - 1800) * 10000000 + m * 625000 + d * 15625;
}

void unique_id_to_date(int id, int *y, int *m, int *d) {
    *y = id / 10000000 + 1800;
    *m = ((id % 10000000) / 625000) + 1;
    *d = ((id % 625000) / 15625) + 1;
}

const char *category_enum_to_str(category_t cat) {
    return category_string[cat];
}

category_t category_str_to_enum(const char *str) {
    for (int i = 0; i < CAT_NUMBER_OF; i++) {
        if (streq(category_string[i], str)) {
            return i;
        }
    }
    return CAT_UNKNOWN;
}

category_t category_strn_to_enum(const char *str, size_t n) {
    for (int i = 0; i < CAT_NUMBER_OF; i++) {
        if (strncmp(category_string[i], str, n) == 0 && category_string[i][n] == '\0') {
            return i;
        }
    }
    return CAT_UNKNOWN;
}

// compute the citations from the references
// allocates memory for paper->cites and fills it with pointers to citing papers
bool build_citation_links(int num_papers, paper_t *papers) {
    printf("building citation links\n");

    // allocate memory for cites for each paper
    for (int i = 0; i < num_papers; i++) {
        paper_t *paper = &papers[i];
        if (paper->num_cites > 0) {
            paper->cites = m_new(paper_t*, paper->num_cites);
            if (paper->cites == NULL) {
                return false;
            }
        }
        // use num cites to count which entry in the array we are up to when inserting cite links
        paper->num_cites = 0;
    }

    // link the cites
    for (int i = 0; i < num_papers; i++) {
        paper_t *paper = &papers[i];
        for (int j = 0; j < paper->num_refs; j++) {
            paper_t *ref_paper = paper->refs[j];
            ref_paper->cites[ref_paper->num_cites++] = paper;
        }
    }

    return true;
}

// compute the num_included_cites field in the paper_t objects
// only includes papers that have their "included" flag set
void recompute_num_included_cites(int num_papers, paper_t *papers) {
    // reset citation count
    for (int i = 0; i < num_papers; i++) {
        paper_t *p = &papers[i];
        p->num_included_cites = 0;
    }

    // compute citation count by following references
    for (int i = 0; i < num_papers; i++) {
        paper_t *p = &papers[i];
        if (p->included) {
            for (int j = 0; j < p->num_refs; j++) {
                paper_t *p2 = p->refs[j];
                if (p2->included) {
                    p2->num_included_cites += 1;
                }
            }
        }
    }
}

typedef struct _paper_stack_t {
    int alloc;
    int used;
    paper_t **stack;
} paper_stack_t;

static paper_stack_t *paper_stack_new() {
    paper_stack_t *s = m_new(paper_stack_t, 1);
    s->alloc = 1024;
    s->used = 0;
    s->stack = m_new(paper_t*, s->alloc);
    return s;
}

static void paper_stack_free(paper_stack_t *s) {
    m_free(s->stack);
    m_free(s);
}

static void paper_stack_push(paper_stack_t *s, paper_t *p) {
    if (s->used >= s->alloc) {
        s->alloc *= 2;
        s->stack = m_renew(paper_t*, s->stack, s->alloc);
    }
    s->stack[s->used++] = p;
}

static paper_t *paper_stack_pop(paper_stack_t *s) {
    assert(s->used > 0);
    return s->stack[--s->used];
}

static void paper_paint(paper_t *p, int colour, paper_stack_t *stack) {
    assert(p->colour == 0);
    p->colour = colour;
    paper_stack_push(stack, p);
    while (stack->used > 0) {
        p = paper_stack_pop(stack);
        assert(p->colour == colour);
        for (int i = 0; i < p->num_refs; i++) {
            paper_t *p2 = p->refs[i];
            if (p2->included && p2->colour != colour) {
                assert(p2->colour == 0);
                p2->colour = colour;
                paper_stack_push(stack, p2);
            }
        }
        for (int i = 0; i < p->num_cites; i++) {
            paper_t *p2 = p->cites[i];
            if (p2->included && p2->colour != colour) {
                assert(p2->colour == 0);
                p2->colour = colour;
                paper_stack_push(stack, p2);
            }
        }
    }
}

// works out connected class for each paper (the colour after a flood fill painting algorigth)
// only includes papers that have their "included" flag set
void recompute_colours(int num_papers, paper_t *papers, int verbose) {
    // clear colour
    for (int i = 0; i < num_papers; i++) {
        papers[i].colour = 0;
    }

    // assign colour
    int cur_colour = 1;
    paper_stack_t *paper_stack = paper_stack_new();
    for (int i = 0; i < num_papers; i++) {
        paper_t *paper = &papers[i];
        if (paper->included && paper->colour == 0) {
            paper_paint(paper, cur_colour++, paper_stack);
        }
    }
    paper_stack_free(paper_stack);

    // compute and assign num_with_my_colour for each paper
    int *num_with_col = m_new0(int, cur_colour);
    for (int i = 0; i < num_papers; i++) {
        num_with_col[papers[i].colour] += 1;
    }
    for (int i = 0; i < num_papers; i++) {
        papers[i].num_with_my_colour = num_with_col[papers[i].colour];
    }

    if (verbose) {
        // compute histogram
        int hist_max = 100;
        int hist_num = 0;
        int *hist_s = m_new(int, hist_max);
        int *hist_n = m_new(int, hist_max);
        for (int colour = 1; colour < cur_colour; colour++) {
            int n = num_with_col[colour];

            int i;
            for (i = 0; i < hist_num; i++) {
                if (hist_s[i] == n) {
                    break;
                }
            }
            if (i == hist_num && hist_num < hist_max) {
                hist_num += 1;
                hist_s[i] = n;
                hist_n[i] = 0;
            }
            hist_n[i] += 1;
        }

        printf("%d colours, %d unique sizes\n", cur_colour - 1, hist_num);
        for (int i = 0; i < hist_num; i++) {
            printf("size %d occured %d times\n", hist_s[i], hist_n[i]);
        }
    }

    m_free(num_with_col);
}
