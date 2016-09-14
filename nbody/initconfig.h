#ifndef _INCLUDED_INITCONFIG_H
#define _INCLUDED_INITCONFIG_H

// Initial configuration
typedef struct _init_config_t {
    // ### MySQL config
    // meta table
    const char *sql_meta_name;
    const char *sql_meta_clause;
    const char *sql_meta_field_id;
    const char *sql_meta_field_title;
    const char *sql_meta_field_authors;
    const char *sql_meta_field_allcats;
    const char *sql_meta_field_keywords;
    bool sql_meta_add_missing_cats;
    // refs table
    const char *sql_refs_name;
    const char *sql_refs_field_id;
    const char *sql_refs_field_refs;
    bool        sql_refs_rblob_freq;
    bool        sql_refs_rblob_order;
    bool        sql_refs_rblob_cites;
    // map table
    const char *sql_map_name;
    const char *sql_map_field_id;
    const char *sql_map_field_x;
    const char *sql_map_field_y;
    const char *sql_map_field_r;
    // ### Map Environment initial configuration
    bool   ids_time_ordered;
    bool   use_external_cites;
    double mass_cites_exponent;
    bool   force_use_ref_freq;
    bool   force_initial_close_repulsion;
    double force_close_repulsion_a;
    double force_close_repulsion_b;
    double force_close_repulsion_c;
    double force_close_repulsion_d;
    double force_link_strength;
    double force_anti_gravity_falloff_rsq;
} init_config_t;

bool init_config_new(const char *filename, init_config_t **config);

#endif // _INCLUDED_INITCONFIG_H
