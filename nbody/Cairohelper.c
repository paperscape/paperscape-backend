#include <stdlib.h>
#include <cairo.h>

#include "xiwilib.h"
#include "Cairohelper.h"

void Cairo_helper_draw_text_lines(cairo_t *cr, double x, double y, vstr_t *vstr) {
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
