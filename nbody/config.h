#ifndef _INCLUDED_CONFIG_H
#define _INCLUDED_CONFIG_H

#include "util/xiwilib.h"

// Initial configuration
typedef struct _config_t {
    // MySQL config
    const char *query_extra_clause;
    bool refsblob_ref_freq;
    bool refsblob_ref_order;
    bool refsblob_ref_cites;
    // Map Environment initial configuration
    bool   ids_time_ordered;
    double force_close_repulsion_a;
    double force_close_repulsion_b;
    double force_close_repulsion_c;
    double force_close_repulsion_d;
    double force_link_strength;
    double force_anti_gravity_falloff_rsq;
} config_t;

bool config_new(const char *filename, config_t **config);

#endif // _INCLUDED_CONFIG_H

