#include <stdlib.h>
#include <string.h>
#include <cairo.h>

#include "util/xiwilib.h"
#include "cairohelper.h"

void cairo_helper_draw_text_lines(cairo_t *cr, double x, double y, vstr_t *vstr) {
    char *s1 = vstr_str(vstr);
    while (*s1 != '\0') {
        char *s2 = s1;
        while (*s2 != '\0' && *s2 != '\n') {
            s2 += 1;
        }
        int old_c = *s2;
        *s2 = '\0';
        cairo_move_to(cr, x, y);
        cairo_show_text(cr, s1);
        *s2 = old_c;
        y += 12;
        s1 = s2;
        if (*s1 == '\n') {
            s1 += 1;
        }
    }
}

void cairo_helper_draw_horizontal_scale(cairo_t *cr, double x, double y, double length, const char *label, bool right_aligned) {
    double xmin, xmax;
    if (right_aligned) {
        xmax = x;
        xmin = x - length;
    } else {
        xmin = x;
        xmax = x + length;
    }
    cairo_move_to(cr, xmin, y);
    cairo_line_to (cr, xmax, y);
    cairo_move_to(cr, xmin, y+5);
    cairo_line_to (cr, xmin, y-5);
    cairo_move_to(cr, xmax, y+5);
    cairo_line_to (cr, xmax, y-5);
    cairo_stroke (cr);
    cairo_set_font_size(cr, 10);
    if (right_aligned) {
        cairo_move_to(cr, xmax - 5*strnlen(label,100), y-10);
    } else {
        cairo_move_to(cr, xmin, y-10);
    }
    cairo_show_text(cr, label);
}
