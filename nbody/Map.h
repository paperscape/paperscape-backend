#ifndef _INCLUDED_MAP_H
#define _INCLUDED_MAP_H

#include "common.h"
#include "quadtree.h"
#include "layout.h"
#include "force.h"

typedef struct _category_info_t {
    unsigned int num;       // number of papers in this category
    float x, y;     // position of this category
} category_info_t;

typedef struct _Map_env_t {
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
    keyword_set_t *keyword_set;

    // info for each category
    category_info_t category_info[CAT_NUMBER_OF];
} Map_env_t;

Map_env_t *Map_env_new();

void Map_env_set_papers(Map_env_t *map_env, int num_papers, paper_t *papers, keyword_set_t *keyword_set);
void Map_env_random_papers(Map_env_t *map_env, int n);
void Map_env_papers_test1(Map_env_t *map_env, int n);
void Map_env_papers_test2(Map_env_t *map_env, int n);

void Map_env_world_to_screen(Map_env_t *map_env, double *x, double *y);
void Map_env_screen_to_world(Map_env_t *map_env, double screen_w, double screen_h, double *x, double *y);
int Map_env_get_num_papers(Map_env_t *map_env);
layout_node_t *Map_env_get_layout_node_at(Map_env_t *map_env, double screen_w, double screen_h, double screen_x, double screen_y);

void Map_env_centre_view(Map_env_t *map_env);
void Map_env_set_zoom_to_fit_n_standard_deviations(Map_env_t *map_env, double n, double screen_w, double screen_h);
void Map_env_scroll(Map_env_t *map_env, double dx, double dy);
void Map_env_zoom(Map_env_t *map_env, double screen_x, double screen_y, double amt);
double Map_env_get_step_size(Map_env_t *map_env);
double Map_env_get_link_strength(Map_env_t *map_env);
void Map_env_set_step_size(Map_env_t *map_env, double value);
void Map_env_set_do_close_repulsion(Map_env_t *map_env, bool value);
void Map_env_set_make_fake_links(Map_env_t *map_env, bool value);
void Map_env_set_other_links_veto(Map_env_t *map_env, bool value);
void Map_env_set_anti_gravity(Map_env_t *map_env, double val);
void Map_env_set_link_strength(Map_env_t *map_env, double val);
void Map_env_toggle_do_tred(Map_env_t *map_env);
void Map_env_toggle_draw_grid(Map_env_t *map_env);
void Map_env_toggle_draw_paper_links(Map_env_t *map_env);
void Map_env_toggle_do_close_repulsion(Map_env_t *map_env);
void Map_env_toggle_use_ref_freq(Map_env_t *map_env);
void Map_env_adjust_anti_gravity(Map_env_t *map_env, double amt);
void Map_env_adjust_link_strength(Map_env_t *map_env, double amt);
void Map_env_adjust_close_repulsion(Map_env_t *map_env, double amt_a, double amt_b);
void Map_env_adjust_close_repulsion2(Map_env_t *map_env, double amt_a, double amt_b);
int Map_env_number_of_coarser_layouts(Map_env_t *map_env);
int Map_env_number_of_finer_layouts(Map_env_t *map_env);
void Map_env_coarsen_layout(Map_env_t *map_env);
void Map_env_refine_layout(Map_env_t *map_env);
void Map_env_jolt(Map_env_t *map_env, double amt);
void Map_env_rotate_all(Map_env_t *map_env, double angle);
void Map_env_orient_using_category(Map_env_t *map_env, category_t wanted_cat, double wanted_angle);
void Map_env_orient_using_paper(Map_env_t *map_env, paper_t *wanted_paper, double wanted_angle);
void Map_env_flip_x(Map_env_t *map_env);

bool Map_env_iterate(Map_env_t *map_env, layout_node_t *hold_still, bool boost_step_size, bool very_fine_steps);

void Map_env_get_max_id_range(Map_env_t *map_env, unsigned int *id_min, unsigned int *id_max);
void Map_env_inc_num_papers(Map_env_t *map_env, int amt);
void Map_env_select_date_range(Map_env_t *map_env, unsigned int id_start, unsigned int id_end);

void Map_env_layout_new(Map_env_t *map_env, int num_coarsenings, double factor_ref_freq, double factor_other_link);
int Map_env_layout_place_new_papers(Map_env_t *map_env);
void Map_env_layout_finish_placing_new_papers(Map_env_t *map_env);
void Map_env_layout_pos_load_from_json(Map_env_t *map_env, const char *json_filename);
void Map_env_layout_pos_save_to_json(Map_env_t *map_env, const char *file);
void Map_env_layout_link_save_to_json(Map_env_t *map_env, const char *file);

#endif // _INCLUDED_MAP_H
