#include <stdlib.h>
#include <assert.h>

#include "xiwilib.h"
#include "common.h"
#include "layout.h"

void build_layout_from_papers(int num_papers, paper_t **papers, int *num_layouts, layout_t **layouts) {
    *num_layouts = num_papers;
    *layouts = m_new(layout_t, num_papers);
    // assign each paper to a layout
    for (int i = 0; i < num_papers; i++) {
        papers[i]->layout = &(*layouts)[i];
    }
    // build the layouts
    for (int i = 0; i < num_papers; i++) {
        paper_t *paper = papers[i];
        layout_t *layout = &(*layouts)[i];
        layout->parent = NULL;
        layout->child1 = NULL;
        layout->child2 = NULL;
        layout->num_links = paper->num_refs;
        layout->links = m_new(layout_link_t, paper->num_refs);
        for (int i = 0; i < paper->num_refs; i++) {
            layout->links[i].weight = paper->refs_ref_freq[i];
            layout->links[i].layout = paper->refs[i]->layout;
        }
        layout->mass = paper->mass;
        layout->x = 0;
        layout->y = 0;
        layout->fx = 0;
        layout->fy = 0;
    }
}

// adds links2 to links, combining weights if destination already exists in links
static void add_links(unsigned int *num_links, layout_link_t *links, unsigned int num_links2, layout_link_t *links2) {
    unsigned int n = *num_links;
    for (int i = 0; i < num_links2; i++) {
        layout_link_t *link_to_add = &links2[i];
        bool found = false;
        for (int j = 0; j < n; j++) {
            if (links[j].layout == link_to_add->layout->parent) {
                links[j].weight += link_to_add->weight;
                found = true;
                break;
            }
        }
        if (!found) {
            links[n].weight = link_to_add->weight;
            links[n].layout = link_to_add->layout->parent;
            n += 1;
        }
    }
    *num_links = n;
}

void build_reduced_layout_from_layout(int num_layouts, layout_t *layouts, int *num_layouts2, layout_t **layouts2) {
    *num_layouts2 = 0;
    *layouts2 = m_new(layout_t, num_layouts);

    // clear the parents
    for (int i = 0; i < num_layouts; i++) {
        layouts[i].parent = NULL;
    }

    // combine up to 2 layouts into a new layout
    for (int i = 0; i < num_layouts; i++) {
        layout_t *layout = &layouts[i];
        if (layout->parent != NULL) {
            continue;
        }

        // find the link with the largest weight
        layout_link_t *max_link = NULL;
        for (int i = 0; i < layout->num_links; i++) {
            layout_link_t *link = &layout->links[i];
            if (link->layout->parent == NULL && (max_link == NULL || link->weight > max_link->weight)) {
                max_link = link;
            }
        }

        // combine
        layout_t *layout2 = &(*layouts2)[(*num_layouts2)++];
        layout2->parent = NULL;
        layout2->num_links = 0;
        layout2->links = NULL;
        layout2->x = 0;
        layout2->y = 0;
        layout2->fx = 0;
        layout2->fy = 0;

        if (max_link == NULL) {
            // no available link, so this layout does not get combined with anything
            layout->parent = layout2;
            layout2->child1 = layout;
            layout2->child2 = NULL;
            layout2->mass = layout->mass;
        } else {
            // combine layout with link->layout into layout2
            layout->parent = layout2;
            max_link->layout->parent = layout2;
            layout2->child1 = layout;
            layout2->child2 = max_link->layout;
            layout2->mass = layout->mass + max_link->layout->mass;
        }
    }

    for (int i = 0; i < num_layouts; i++) {
        layout_t *layout = &layouts[i];
        assert(layout->parent != NULL);
        bool found = false;
        for (int j = 0; j < *num_layouts2; j++) {
            if (layout->parent == &(*layouts2)[j]) {
                found = true;
                break;
            }
        }
        assert(found);
    }

    // make links for new, reduced layout
    for (int i = 0; i < *num_layouts2; i++) {
        layout_t *layout2 = &(*layouts2)[i];
        layout2->num_links = layout2->child1->num_links;
        if (layout2->child2 != NULL) {
            layout2->num_links += layout2->child2->num_links;
        }
        layout2->links = m_new(layout_link_t, layout2->num_links);
        layout2->num_links = 0;
        add_links(&layout2->num_links, layout2->links, layout2->child1->num_links, layout2->child1->links);
        if (layout2->child2 != NULL) {
            add_links(&layout2->num_links, layout2->links, layout2->child2->num_links, layout2->child2->links);
        }
        //printf("layout of mass %f has %d links\n", layout2->mass, layout2->num_links);
    }
}
