#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <assert.h>

#include "xiwilib.h"
#include "common.h"

int date_to_unique_id(int y, int m, int d) {
    return (y - 1800) * 10000000 + m * 625000 + d * 15625;
}

void unique_id_to_date(int id, int *y, int *m, int *d) {
    *y = id / 10000000 + 1800;
    *m = ((id % 10000000) / 625000) + 1;
    *d = ((id % 625000) / 15625) + 1;
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
