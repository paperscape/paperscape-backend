#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <assert.h>
#include <math.h>

#include "xiwilib.h"
#include "common.h"
#include "layout.h"

void layout_combine_duplicate_links(layout_t *layout) {
    // combine duplicate links
    for (int i = 0; i < layout->num_nodes; i++) {
        layout_node_t *node = &layout->nodes[i];
        assert(node != NULL);
        for (int j = 0; j < node->num_links; j++) {
            layout_node_t *node2 = node->links[j].node;
            assert(node2 != NULL);
            assert(node2 != node); // shouldn't be any nodes linking to themselves
            for (int k = 0; k < node2->num_links; k++) {
                if (node2->links[k].node == node) {
                    // a duplicate link (node -> node2, and node2 -> node)
                    node->links[j].weight += node2->links[k].weight;
                    memmove(&node2->links[k], &node2->links[k + 1], (node2->num_links - k - 1) * sizeof(layout_link_t));
                    node2->num_links -= 1;
                    break;
                }
            }
        }
    }

    // count number of links layout
    layout->num_links = 0;
    for (int i = 0; i < layout->num_nodes; i++) {
        layout->num_links += layout->nodes[i].num_links;
    }
}

layout_t *build_layout_from_papers(int num_papers, paper_t **papers, bool age_weaken) {
    // allocate memory for the nodes
    int num_nodes = num_papers;
    layout_node_t *nodes = m_new(layout_node_t, num_nodes);

    // assign each paper to a node
    for (int i = 0; i < num_papers; i++) {
        papers[i]->layout_node = &nodes[i];
    }

    // build the nodes, and count the number of links we need
    int num_links = 0;
    for (int i = 0; i < num_papers; i++) {
        paper_t *paper = papers[i];
        layout_node_t *node = &nodes[i];
        node->parent = NULL;
        node->paper = paper;
        node->mass = paper->mass;
        node->radius = paper->r;
        node->x = 0;
        node->y = 0;
        node->fx = 0;
        node->fy = 0;
        num_links += paper->num_refs + paper->num_fake_links;
    }

    // build the links
    layout_link_t *all_links = m_new(layout_link_t, num_links);
    layout_link_t *links = all_links;
    for (int i = 0; i < num_papers; i++) {
        paper_t *paper = papers[i];
        layout_node_t *node = &nodes[i];
        node->num_links = paper->num_refs + paper->num_fake_links;
        node->links = links;
        for (int j = 0; j < paper->num_refs; j++) {
            node->links[j].weight = paper->refs_ref_freq[j];
            if (age_weaken) {
                //node->links[j].weight *= 1.0 - 0.5 * fabs(paper->age - paper->refs[j]->age);
                node->links[j].weight *= 0.4 + 0.6 * exp(-pow(1e-7 * paper->id - 1e-7 * paper->refs[j]->id, 2));
            }
            node->links[j].node = paper->refs[j]->layout_node;
            assert(node->links[j].node != NULL);
        }
        for (int j = 0; j < paper->num_fake_links; j++) {
            node->links[paper->num_refs + j].weight = 0.25; // what to use for fake link weight??
            node->links[paper->num_refs + j].node = paper->fake_links[j]->layout_node;
            assert(node->links[paper->num_refs + j].node != NULL);
        }
        links += node->num_links;
    }
    assert(all_links + num_links == links);

    // make layout object
    layout_t *layout = m_new(layout_t, 1);
    layout->parent_layout = NULL;
    layout->child_layout = NULL;
    layout->num_nodes = num_nodes;
    layout->nodes = nodes;
    layout->num_links = num_links;
    layout->links = all_links;

    // combine duplicate links
    layout_combine_duplicate_links(layout);

    return layout;
}

// adds links2 to links in node, combining weights if destination already exists in links
static void add_links(layout_node_t *node, unsigned int num_links2, layout_link_t *links2) {
    for (int i = 0; i < num_links2; i++) {
        layout_link_t *link_to_add = &links2[i];
        if (link_to_add->node->parent == node) {
            // a link to itself, don't include
            continue;
        }

        // look to see if link already exists
        bool found = false;
        for (int j = 0; j < node->num_links; j++) {
            if (node->links[j].node == link_to_add->node->parent) {
                // combine weights
                node->links[j].weight += link_to_add->weight;
                found = true;
                break;
            }
        }

        // link does not exist, make a new one
        if (!found) {
            node->links[node->num_links].weight = link_to_add->weight;
            node->links[node->num_links].node = link_to_add->node->parent;
            node->num_links += 1;
        }
    }
}

typedef struct _node_weight_t {
    layout_node_t *node;
    float weight;
} node_weight_t;

int node_weight_cmp(const void *nw1_in, const void *nw2_in) {
    node_weight_t *nw1 = (node_weight_t*)nw1_in;
    node_weight_t *nw2 = (node_weight_t*)nw2_in;
    // largest weight first
    if (nw1->weight < nw2->weight) {
        return 1;
    } else if (nw1->weight > nw2->weight) {
        return -1;
    // if equal weights, smallest mass first
    } else if (nw1->node->mass < nw2->node->mass) {
        return -1;
    } else if (nw1->node->mass > nw2->node->mass) {
        return 1;
    } else {
        return 0;
    }
}

layout_t *build_reduced_layout_from_layout(layout_t *layout) {
    int num_nodes2 = 0;
    layout_node_t *nodes2 = m_new(layout_node_t, layout->num_nodes);

    // clear the parents, and count number of links
    int num_nodes_with_links = 0;
    for (int i = 0; i < layout->num_nodes; i++) {
        layout_node_t *node = &layout->nodes[i];
        node->parent = NULL;
        if (node->num_links > 0) {
            num_nodes_with_links += 1;
        }
    }

    //
    node_weight_t *nodes_with_links = m_new(node_weight_t, num_nodes_with_links);
    for (int i = 0, j = 0; i < layout->num_nodes; i++) {
        layout_node_t *node = &layout->nodes[i];
        if (node->num_links > 0) {
            float max_weight = 0;
            for (int i = 0; i < node->num_links; i++) {
                if (node->links[i].weight > max_weight) {
                    max_weight = node->links[i].weight;
                }
            }
            nodes_with_links[j].node = node;
            nodes_with_links[j].weight = max_weight;
            j += 1;
        }
    }
    qsort(nodes_with_links, num_nodes_with_links, sizeof(node_weight_t), node_weight_cmp);

    // where possible, combine 2 nodes into a new single node
    for (int i = 0; i < num_nodes_with_links; i++) {
        layout_node_t *node = nodes_with_links[i].node;
        if (node->parent != NULL) {
            // node already combined
            continue;
        }

        // find the link with the largest weight
        layout_link_t *max_link = NULL;
        for (int i = 0; i < node->num_links; i++) {
            layout_link_t *link = &node->links[i];
            if (link->node->parent == NULL && (max_link == NULL || link->weight > max_link->weight)) {
                max_link = link;
            }
        }

        if (max_link == NULL) {
            // no available link
            continue;
        }

        // combine node with link->node into node2
        layout_node_t *node2 = &nodes2[num_nodes2++];
        node2->parent = NULL;
        node2->child1 = node;
        node2->child2 = max_link->node;
        node2->num_links = 0;
        node2->links = NULL;
        node2->mass = node->mass + max_link->node->mass;
        node2->radius = sqrt(node->radius*node->radius + max_link->node->radius*max_link->node->radius);
        node2->x = 0;
        node2->y = 0;
        node2->fx = 0;
        node2->fy = 0;
        node->parent = node2;
        max_link->node->parent = node2;
    }
    m_free(nodes_with_links);

    // put left over nodes into single new node
    for (int i = 0; i < layout->num_nodes; i++) {
        layout_node_t *node = &layout->nodes[i];
        if (node->parent != NULL) {
            // node already combined
            continue;
        }

        // put node into node2
        layout_node_t *node2 = &nodes2[num_nodes2++];
        node2->parent = NULL;
        node2->child1 = node;
        node2->child2 = NULL;
        node2->num_links = 0;
        node2->links = NULL;
        node2->mass = node->mass;
        node2->radius = node->radius;
        node2->x = 0;
        node2->y = 0;
        node2->fx = 0;
        node2->fy = 0;
        node->parent = node2;
    }

    // sanity checks; takes ages since it's O(N^2)
    /*
    for (int i = 0; i < layout->num_nodes; i++) {
        layout_node_t *node = &layout->nodes[i];
        assert(node->parent != NULL);
        bool found_parent = false;
        int num_parents = 0;
        for (int j = 0; j < num_nodes2; j++) {
            if (nodes2[j].child1 != NULL) {
                assert(nodes2[j].child1 != nodes2[j].child2);
            }
            if (node->parent == &nodes2[j]) {
                assert(nodes2[j].child1 == node || nodes2[j].child2 == node);
                found_parent = true;
            }
            if (nodes2[j].child1 == node) {
                num_parents += 1;
            }
            if (nodes2[j].child2 == node) {
                num_parents += 1;
            }
        }
        assert(found_parent);
        assert(num_parents == 1);
    }
    */

    // make links for new, reduced layout
    for (int i = 0; i < num_nodes2; i++) {
        layout_node_t *node2 = &nodes2[i];
        node2->num_links = node2->child1->num_links;
        if (node2->child2 != NULL) {
            node2->num_links += node2->child2->num_links;
        }
        node2->links = m_new(layout_link_t, node2->num_links);
        node2->num_links = 0;
        add_links(node2, node2->child1->num_links, node2->child1->links);
        if (node2->child2 != NULL) {
            add_links(node2, node2->child2->num_links, node2->child2->links);
        }
    }

    // make layout object
    layout_t *layout2 = m_new(layout_t, 1);
    layout2->parent_layout = NULL;
    layout2->child_layout = layout;
    layout2->num_nodes = num_nodes2;
    layout2->nodes = nodes2;
    layout2->num_links = 0;
    layout2->links = NULL;
    layout->parent_layout = layout2;

    // combine duplicate links
    layout_combine_duplicate_links(layout2);

    return layout2;
}

void layout_node_propagate_position_to_children(layout_t *layout, layout_node_t *node) {
    if (layout->child_layout != NULL) {
        node->child1->x = node->x;
        node->child1->y = node->y;
        layout_node_propagate_position_to_children(layout->child_layout, node->child1);
        if (node->child2 != NULL) {
            node->child2->x = node->x;
            node->child2->y = node->y;
            layout_node_propagate_position_to_children(layout->child_layout, node->child2);
        }
    }
}

void layout_propagate_positions_to_children(layout_t *layout) {
    for (int i = 0; i < layout->num_nodes; i++) {
        layout_node_propagate_position_to_children(layout, &layout->nodes[i]);
    }
}

void layout_print(layout_t *l) {
    double mass = 0;
    double radius = 0;
    for (int i = 0; i < l->num_nodes; i++) {
        mass += l->nodes[i].mass;
        radius += l->nodes[i].radius*l->nodes[i].radius;
    }
    bool finest = l->child_layout == NULL;
    printf("layout has %d nodes, %d links, %lg total mass, %lg total radius", l->num_nodes, l->num_links, mass, sqrt(radius));
    if (finest) {
        printf("\n");
    } else {
        printf("; ratio to child: %f nodes, %f links\n", 1.0 * l->num_nodes / l->child_layout->num_nodes, 1.0 * l->num_links / l->child_layout->num_links);
    }
    /*
    for (int i = 0; i < l->num_nodes; i++) {
        layout_node_t *n = &l->nodes[i];
        printf("  %d, mass %g, parent (", i, n->mass);
        if (n->parent != NULL) {
            printf("%ld", n->parent - l->parent_layout->nodes);
        }
        printf(") children (");
        if (finest) {
            printf("%u", n->paper->id);
        } else {
            printf("%ld", n->child1 - l->child_layout->nodes);
            if (n->child2 != NULL) {
                printf(",%ld", n->child2 - l->child_layout->nodes);
            }
        }
        printf(") linked to (");
        for (int j = 0; j < n->num_links; j++) {
            printf("%ldw%g,", n->links[j].node - l->nodes, n->links[j].weight);
        }
        printf(")\n");
    }
    */
}
