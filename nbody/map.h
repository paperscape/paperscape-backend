#ifndef _INCLUDED_MAP_H
#define _INCLUDED_MAP_H

#include "util/hashmap.h"
#include "common.h"
#include "quadtree.h"
#include "layout.h"
#include "force.h"

typedef struct _category_info_t {
    unsigned int num;       // number of papers in this category
    float x, y;     // position of this category
} category_info_t;

typedef struct _map_env_t {
    // loaded
    int max_num_papers;
    paper_t *all_papers;

    // currently in the graph
    int num_papers;
    paper_t **papers;

    quadtree_t *quad_tree;

    bool make_fake_links;
    bool other_links_veto;

    force_params_t force_params;

    bool do_tred;
    bool draw_grid;
    bool draw_paper_links;
    bool draw_categories;
    bool ids_time_ordered;

    // transformation matrix
    double tr_scale;
    double tr_x0;
    double tr_y0;

    double energy;
    int progress;
    double step_size;
    double max_link_force_mag;
    double max_total_force_mag;

    // standard deviation of the positions of the papers
    double x_sd, y_sd;

    layout_t *layout;

    // info for keywords
    hashmap_t *keyword_set;

    // info for each category
    category_info_t category_info[CAT_NUMBER_OF];
} map_env_t;

map_env_t *map_env_new();

void map_env_set_init_config(map_env_t *map_env, init_config_t *init_config);
void map_env_set_papers(map_env_t *map_env, int num_papers, paper_t *papers, hashmap_t *keyword_set);
void map_env_random_papers(map_env_t *map_env, int n);
void map_env_papers_test1(map_env_t *map_env, int n);
void map_env_papers_test2(map_env_t *map_env, int n);

void map_env_world_to_screen(map_env_t *map_env, double *x, double *y);
void map_env_screen_to_world(map_env_t *map_env, double screen_w, double screen_h, double *x, double *y);
int map_env_get_num_papers(map_env_t *map_env);
layout_node_t *map_env_get_layout_node_at(map_env_t *map_env, double screen_w, double screen_h, double screen_x, double screen_y);

void map_env_centre_view(map_env_t *map_env);
void map_env_set_zoom_to_fit_n_standard_deviations(map_env_t *map_env, double n, double screen_w, double screen_h);
void map_env_scroll(map_env_t *map_env, double dx, double dy);
void map_env_zoom(map_env_t *map_env, double screen_x, double screen_y, double amt);
double map_env_get_step_size(map_env_t *map_env);
double map_env_get_link_strength(map_env_t *map_env);
double map_env_get_anti_gravity(map_env_t *map_env);
void map_env_set_step_size(map_env_t *map_env, double value);
void map_env_set_do_close_repulsion(map_env_t *map_env, bool value);
void map_env_set_make_fake_links(map_env_t *map_env, bool value);
void map_env_set_other_links_veto(map_env_t *map_env, bool value);
void map_env_set_anti_gravity(map_env_t *map_env, double val);
void map_env_set_link_strength(map_env_t *map_env, double val);
void map_env_toggle_do_tred(map_env_t *map_env);
void map_env_toggle_draw_grid(map_env_t *map_env);
void map_env_toggle_draw_paper_links(map_env_t *map_env);
void map_env_toggle_draw_categories(map_env_t *map_env);
void map_env_toggle_do_close_repulsion(map_env_t *map_env);
void map_env_toggle_use_ref_freq(map_env_t *map_env);
void map_env_adjust_anti_gravity(map_env_t *map_env, double amt);
void map_env_adjust_link_strength(map_env_t *map_env, double amt);
void map_env_adjust_close_repulsion(map_env_t *map_env, double amt_a, double amt_b);
void map_env_adjust_close_repulsion2(map_env_t *map_env, double amt_a, double amt_b);
int map_env_number_of_coarser_layouts(map_env_t *map_env);
int map_env_number_of_finer_layouts(map_env_t *map_env);
void map_env_coarsen_layout(map_env_t *map_env);
void map_env_refine_layout(map_env_t *map_env);
void map_env_jolt(map_env_t *map_env, double amt);
void map_env_rotate_all(map_env_t *map_env, double angle);
void map_env_orient_using_category(map_env_t *map_env, category_t wanted_cat, double wanted_angle);
void map_env_orient_using_paper(map_env_t *map_env, paper_t *wanted_paper, double wanted_angle);
void map_env_flip_x(map_env_t *map_env);

bool map_env_iterate(map_env_t *map_env, layout_node_t *hold_still, bool boost_step_size, bool very_fine_steps);

void map_env_get_max_id_range(map_env_t *map_env, unsigned int *id_min, unsigned int *id_max);
void map_env_inc_num_papers(map_env_t *map_env, int amt);
void map_env_select_date_range(map_env_t *map_env, unsigned int id_start, unsigned int id_end);

void map_env_layout_new(map_env_t *map_env, int num_coarsenings, double factor_ref_freq, double factor_other_link);
int map_env_layout_place_new_papers(map_env_t *map_env);
void map_env_layout_finish_placing_new_papers(map_env_t *map_env);
void map_env_layout_pos_load_from_json(map_env_t *map_env, const char *json_filename);
void map_env_layout_pos_save_to_json(map_env_t *map_env, const char *file);
void map_env_layout_link_save_to_json(map_env_t *map_env, const char *file);

#endif // _INCLUDED_MAP_H
