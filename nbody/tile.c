#include <unistd.h>
#include <fcntl.h>
#include <stdio.h>
#include <cairo.h>

#include "xiwilib.h"
#include "common.h"
#include "map.h"
#include "cairohelper.h"
#include "tile.h"

void write_tiles(map_env_t *map_env, int width, int height, const char *file, vstr_t *vstr_info) {
    cairo_surface_t *surface = cairo_image_surface_create(CAIRO_FORMAT_RGB24, width, height);
    cairo_t *cr = cairo_create(surface);
    map_env_draw(map_env, cr, width, height, NULL);

    if (vstr_info != NULL) {
        cairo_identity_matrix(cr);
        cairo_set_source_rgb(cr, 0, 0, 0);
        cairo_set_font_size(cr, 10);
        cairo_helper_draw_text_lines(cr, 10, 20, vstr_info);
    }

    cairo_status_t status = cairo_surface_write_to_png(surface, file);
    cairo_destroy(cr);
    cairo_surface_destroy(surface);
    if (status != CAIRO_STATUS_SUCCESS) {
        printf("ERROR: cannot write PNG to file %s\n", file);
    } else {
        printf("wrote PNG to file %s\n", file);
    }
}

void write_tiles_to_json(map_env_t *map_env, const char *file) {
    vstr_t *vstr = vstr_new();
    map_env_draw_to_json(map_env, vstr);
    int fd = open(file, O_WRONLY | O_CREAT | O_TRUNC, S_IRUSR | S_IWUSR);
    write(fd, vstr_str(vstr), vstr_len(vstr));
    close(fd);
    vstr_free(vstr);
    printf("wrote to JSON file %s\n", file);
}
