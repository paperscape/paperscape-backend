typedef struct _map_env_t map_env_t;

map_env_t *map_env_new();

void map_env_set_papers(map_env_t *map_env, int num_papers, paper_t *papers, keyword_set_t *keyword_set);
void map_env_random_papers(map_env_t *map_env, int n);
void map_env_papers_test1(map_env_t *map_env, int n);
void map_env_papers_test2(map_env_t *map_env, int n);

void map_env_world_to_screen(map_env_t *map_env, double *x, double *y);
void map_env_screen_to_world(map_env_t *map_env, double *x, double *y);
int map_env_get_num_papers(map_env_t *map_env);
layout_node_t *map_env_get_layout_node_at(map_env_t *map_env, double screen_x, double screen_y);

void map_env_centre_view(map_env_t *map_env);
void map_env_set_zoom_to_fit_n_standard_deviations(map_env_t *map_env, double n, double screen_w, double screen_h);
void map_env_scroll(map_env_t *map_env, double dx, double dy);
void map_env_zoom(map_env_t *map_env, double screen_x, double screen_y, double amt);
void map_env_set_do_close_repulsion(map_env_t *map_env, bool value);
void map_env_set_anti_gravity(map_env_t *map_env, double val);
void map_env_set_link_strength(map_env_t *map_env, double val);
void map_env_toggle_do_tred(map_env_t *map_env);
void map_env_toggle_draw_grid(map_env_t *map_env);
void map_env_toggle_draw_paper_links(map_env_t *map_env);
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
void map_env_orient(map_env_t *map_env, category_t wanted_cat, double wanted_angle);
void map_env_flip_x(map_env_t *map_env);

bool map_env_iterate(map_env_t *map_env, layout_node_t *hold_still, bool boost_step_size);

void map_env_get_max_id_range(map_env_t *map_env, int *id_min, int *id_max);
void map_env_inc_num_papers(map_env_t *map_env, int amt);
void map_env_select_date_range(map_env_t *map_env, int id_start, int id_end);

void map_env_layout_new(map_env_t *map_env, int num_coarsenings);
int map_env_layout_place_new_papers(map_env_t *map_env);
void map_env_layout_finish_placing_new_papers(map_env_t *map_env);
void map_env_layout_load_from_db(map_env_t *map_env);
void map_env_layout_load_from_json(map_env_t *map_env, const char *json_filename);
void map_env_layout_save_to_db(map_env_t *map_env);
void map_env_layout_save_to_json(map_env_t *map_env, const char *file);
