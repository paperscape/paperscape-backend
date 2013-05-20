#include <stdlib.h>
#include <math.h>

#include "xiwilib.h"
#include "common.h"
#include "layout.h"
#include "force.h"

void compute_attractive_link_force(force_params_t *param, bool do_tred, layout_t *layout) {
    for (int i = 0; i < layout->num_nodes; i++) {
        layout_node_t *n1 = &layout->nodes[i];
        for (int j = 0; j < n1->num_links; j++) {
            layout_node_t *n2 = n1->links[j].node;
            double weight = n1->links[j].weight;

            double dx = n1->x - n2->x;
            double dy = n1->y - n2->y;
            double r = sqrt(dx*dx + dy*dy);
            double rest_len = 1.5 * (n1->radius + n2->radius);

            double fac = param->link_strength;

            if (param->use_ref_freq) {
                fac *= 0.65 * weight;
            }

            /*
            // these things we can only do if the nodes are papers
            if (layout->child_layout == NULL) {
                if (do_tred) {
                    //fac *= n1->paper->refs_tred_computed[j];
                }

                // loosen the force between papers in different categories
                if (n1->paper->kind != n2->paper->kind) {
                    fac *= 0.5;
                }

                // loosen the force between papers of different age
                fac *= 1.01 - 0.5 * fabs(n1->paper->age - n2->paper->age); // trying out the 0.5* factor; not tested yet
            }
            */

            // normalise refs so each paper has 1 unit for all references (doesn't really produce a good graph)
            //fac /= n1->num_links;

            if (r > 1e-2) {
                fac *= (r - rest_len) / r;
                double fx = dx * fac;
                double fy = dy * fac;

                n1->fx -= fx;
                n1->fy -= fy;
                n2->fx += fx;
                n2->fy += fy;
            }
        }
    }
}
