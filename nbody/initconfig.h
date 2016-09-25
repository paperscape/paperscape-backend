#ifndef _INCLUDED_INITCONFIG_H
#define _INCLUDED_INITCONFIG_H

// Initial configuration
// Obeys same structure as JSON file
typedef struct _init_config_t {

    bool   ids_time_ordered;
    
    struct _config_nbody_t {
        bool   use_external_cites;
        double mass_cites_exponent;
        bool add_missing_cats;
        
        struct _config_forces_t {
            bool   use_ref_freq;
            bool   initial_close_repulsion;
            double close_repulsion_a;
            double close_repulsion_b;
            double close_repulsion_c;
            double close_repulsion_d;
            double link_strength;
            double anti_gravity_falloff_rsq;
        } forces;

        struct _config_map_orientation_t {
            const char *category;
            double angle;
        } map_orientation;
    } nbody;

    struct _config_tiles_t {
        float background_col[3];
    } tiles;

    struct _config_sql_t {
        
        struct _config_sql_meta_table_t {
            const char *name;
            const char *where_clause;
            const char *extra_clause;
            const char *field_id;
            const char *field_title;
            const char *field_authors;
            const char *field_allcats;
            const char *field_keywords;
        } meta_table;

        struct _config_sql_refs_table_t {
            const char *name;
            const char *field_id;
            const char *field_refs;
            bool rblob_order;
            bool rblob_freq;
            bool rblob_cites;
            bool add_missing_cats;
        } refs_table;

        struct _config_sql_map_table_t {
            const char *name;
            const char *field_id;
            const char *field_x;
            const char *field_y;
            const char *field_r;
        } map_table;

    } sql;

} init_config_t;

bool init_config_new(const char *filename, init_config_t **config);

#endif // _INCLUDED_INITCONFIG_H
