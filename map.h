#ifndef _INCLUDED_MAP_H
#define _INCLUDED_MAP_H

typedef struct _map_env_t map_env_t;

map_env_t *map_env_new();
void map_env_world_to_screen(map_env_t *map_env, double *x, double *y);
void map_env_screen_to_world(map_env_t *map_env, double *x, double *y);
void map_env_set_papers(map_env_t *map_env, int num_papers, paper_t *papers);
void map_env_random_papers(map_env_t *map_env, int n);
void map_env_papers_test1(map_env_t *map_env, int n);
void map_env_papers_test2(map_env_t *map_env, int n);
void map_env_draw(map_env_t *map_env, cairo_t *cr, guint width, guint height, bool do_tred);
bool map_env_forces(map_env_t *map_env, bool do_touch, bool do_attr, bool do_tred, paper_t *hold_still);
void map_env_grow(map_env_t *map_env, double amt);
void map_env_inc_num_papers(map_env_t *map_env, int amt);
void map_env_jolt(map_env_t *map_env, double amt);
paper_t *map_env_get_paper_at(map_env_t *map_env, int mx, int my);

#endif // _INCLUDED_MAP_H

