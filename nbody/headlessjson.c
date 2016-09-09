#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/time.h>

#include "util/xiwilib.h"
#include "common.h"
#include "layout.h"
#include "map.h"
#include "mapauto.h"
#include "json.h"

static int usage(const char *progname) {
    printf("\n");
    printf("usage: %s [options]\n", progname);
    printf("\n");
    printf("options:\n");
    printf("    --pscp-refs <file>          JSON file with paperscape references\n");
    printf("    --other-links <file>        JSON file with links from other source\n");
    printf("    --write-pos <file>          JSON file to write final positions to\n");
    printf("    --write-link <file>         JSON file to write links to\n");
    printf("    --factor-ref-freq <num>     factor to use for reference frequency (default 1)\n");
    printf("    --factor-other-link <num>   factor to use for other link (default 0)\n");
    printf("\n");
    return 1;
}

int main(int argc, char *argv[]) {

    // parse command line arguments
    const char *arg_settings = NULL;
    const char *arg_pscp_refs = NULL;
    const char *arg_other_links = NULL;
    const char *arg_write_pos = NULL;
    const char *arg_write_link = NULL;
    double arg_factor_ref_freq = 1;
    double arg_factor_other_link = 0;
    bool other_links_veto = false;
    bool no_fake_links = false;
    for (int a = 1; a < argc; a++) {
        if (streq(argv[a], "--settings")) {
            a += 1;
            if (a >= argc) {
                return usage(argv[0]);
            }
            arg_settings = argv[a];
        } else if (streq(argv[a], "--pscp-refs") || streq(argv[a], "--pr")) {
            if (++a >= argc) {
                return usage(argv[0]);
            }
            arg_pscp_refs = argv[a];
        } else if (streq(argv[a], "--other-links") || streq(argv[a], "--ol")) {
            if (++a >= argc) {
                return usage(argv[0]);
            }
            arg_other_links = argv[a];
        } else if (streq(argv[a], "--no-fake-links") || streq(argv[a], "--nfl")) {
            no_fake_links = true;
        } else if (streq(argv[a], "--other-links-veto") || streq(argv[a], "--olv")) {
            other_links_veto = true;
        } else if (streq(argv[a], "--write-pos") || streq(argv[a], "--wp")) {
            if (++a >= argc) {
                return usage(argv[0]);
            }
            arg_write_pos = argv[a];
        } else if (streq(argv[a], "--write-link") || streq(argv[a], "--wl")) {
            if (++a >= argc) {
                return usage(argv[0]);
            }
            arg_write_link = argv[a];
        } else if (streq(argv[a], "--factor-ref-freq") || streq(argv[a], "--frf")) {
            if (++a >= argc) {
                return usage(argv[0]);
            }
            arg_factor_ref_freq = strtod(argv[a], NULL);;
        } else if (streq(argv[a], "--factor-other-link") || streq(argv[a], "--fol")) {
            if (++a >= argc) {
                return usage(argv[0]);
            }
            arg_factor_other_link = strtod(argv[a], NULL);;
        } else {
            return usage(argv[0]);
        }
    }

    // load settings from json file
    const char *settings_file = "settings.json";
    if (arg_settings != NULL) {
        settings_file = arg_settings;
    }
    init_config_t *init_config = NULL;
    if (!config_new(settings_file,&init_config)) {
        return 1;
    }

    //if (arg_pscp_refs == NULL || arg_other_links == NULL) {
    if (arg_pscp_refs == NULL) {
        //printf("--pscp-refs and --other-links must be specified\n");
        printf("--pscp-refs\n");
        return 1;
    }

    // load the papers from the DB
    int num_papers;
    paper_t *papers;
    keyword_set_t *keyword_set;
    if (!json_load_papers(arg_pscp_refs, &num_papers, &papers, &keyword_set)) {
        return 1;
    }
    if (arg_other_links != NULL) {
        if (!json_load_other_links(arg_other_links, num_papers, papers)) {
            return 1;
        }
    }

    // create the map object
    map_env_t *map_env = map_env_new();

    // set initial configuration
    if (init_config != NULL) {
        map_env_set_init_config(map_env,init_config);
    }

    // whether to create fake links for disconnected papers
    map_env_set_make_fake_links(map_env,!no_fake_links);
    map_env_set_other_links_veto(map_env,other_links_veto);

    // set the papers
    map_env_set_papers(map_env, num_papers, papers, keyword_set);

    // select the date range
    {
        unsigned int id_min;
        unsigned int id_max;
        map_env_get_max_id_range(map_env, &id_min, &id_max);
        map_env_select_date_range(map_env, id_min, id_max);
    }

    // create a new layout with 10 levels of coarsening
    printf("using weight formula: %lf * ref_freq + %lf * other_link\n", arg_factor_ref_freq, arg_factor_other_link);
    map_env_layout_new(map_env, 10, arg_factor_ref_freq, arg_factor_other_link);

    // do the layout
    map_env_do_complete_layout(map_env, 4000, 10000);

    // align the map in a fixed direction
    if (num_papers > 0) {
        map_env_orient_using_paper(map_env, &papers[0], 0);
    }

    // write position info to JSON
    if (arg_write_pos != NULL) {
        map_env_layout_pos_save_to_json(map_env, arg_write_pos);
    }

    // write link info to JSON
    if (arg_write_link != NULL) {
        map_env_layout_link_save_to_json(map_env, arg_write_link);
    }

    return 0;
}
