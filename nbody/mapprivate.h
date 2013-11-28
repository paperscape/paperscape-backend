typedef struct _category_info_t {
    uint num;       // number of papers in this category
    float x, y;     // position of this category
} category_info_t;

struct _map_env_t {
    // loaded
    int max_num_papers;
    paper_t *all_papers;

    // currently in the graph
    int num_papers;
    paper_t **papers;

    quad_tree_t *quad_tree;

    bool make_fake_links;

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
};
