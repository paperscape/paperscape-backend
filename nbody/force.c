#include <math.h>

#include "xiwilib.h"
#include "common.h"
#include "force.h"

void compute_attractive_link_force_2d(force_params_t *param, bool do_tred, int num_papers, paper_t **papers) {
    for (int i = 0; i < num_papers; i++) {
        paper_t *p1 = papers[i];
        for (int j = 0; j < p1->num_refs; j++) {
            paper_t *p2 = p1->refs[j];
            if ((!do_tred || p1->refs_tred_computed[j]) && p2->included) {
                double dx = p1->x - p2->x;
                double dy = p1->y - p2->y;
                double r = sqrt(dx*dx + dy*dy);
                double rest_len = 1.5 * (p1->r + p2->r);

                double fac = param->link_strength;

                if (param->use_ref_freq) {
                    fac *= 0.65 * p1->refs_ref_freq[j];
                }

                if (do_tred) {
                    fac *= p1->refs_tred_computed[j];
                }

                // loosen the force between papers in different categories
                if (p1->kind != p2->kind) {
                    fac *= 0.5;
                }

                // loosen the force between papers of different age
                fac *= 1.01 - 0.5 * fabs(p1->age - p2->age); // trying out the 0.5* factor; not tested yet

                // normalise refs so each paper has 1 unit for all references (doesn't really produce a good graph)
                //fac /= p1->num_refs;

                if (r > 1e-2) {
                    //fac *= (r - rest_len) * fabs(r - rest_len) / r;
                    fac *= (r - rest_len) / r;
                    double fx = dx * fac;
                    double fy = dy * fac;

                    p1->fx -= fx;
                    p1->fy -= fy;
                    p2->fx += fx;
                    p2->fy += fy;
                }
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

                    p1->fx -= fx;
                    p1->fy -= fy;
                    p1->fz -= fz;
                    p2->fx += fx;
                    p2->fy += fy;
                    p2->fz += fz;
                }
            }
        }
    }
}
