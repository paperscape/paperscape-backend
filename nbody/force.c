#include <stdlib.h>
#include <math.h>

#include "xiwilib.h"
#include "common.h"
#include "layout.h"
#include "force.h"

void compute_attractive_link_force_2d(force_params_t *param, bool do_tred, layout_t *layout) {
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

void compute_attractive_link_force_3d(force_params_t *param, bool do_tred, int num_papers, paper_t **papers) {
    for (int i = 0; i < num_papers; i++) {
        paper_t *p1 = papers[i];
        for (int j = 0; j < p1->num_refs; j++) {
            paper_t *p2 = p1->refs[j];
            if ((!do_tred || p1->refs_tred_computed[j]) && p2->included) {
                double dx = p1->x - p2->x;
                double dy = p1->y - p2->y;
                double dz = p1->z - p2->z;
                double r = sqrt(dx*dx + dy*dy + dz*dz);
                double rest_len = 1.1 * (p1->r + p2->r);

                double fac = 2.4 * param->link_strength;

                if (do_tred) {
                    fac = param->link_strength * p1->refs_tred_computed[j];
                    //fac *= p1->refs_tred_computed[j];
                }

                if (r > 1e-2) {
                    fac *= (r - rest_len) * fabs(r - rest_len) / r;
                    double fx = dx * fac;
                    double fy = dy * fac;
                    double fz = dz * fac;

                    /* doesn't work with new layout code
                    p1->fx -= fx;
                    p1->fy -= fy;
                    p1->fz -= fz;
                    p2->fx += fx;
                    p2->fy += fy;
                    p2->fz += fz;
                    */
                }
            }
        }
    }
}
