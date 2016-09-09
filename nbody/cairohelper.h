#ifndef _INCLUDED_CAIROHELPER_H
#define _INCLUDED_CAIROHELPER_H

void cairo_helper_draw_text_lines(cairo_t *cr, double x, double y, vstr_t *vstr);

void cairo_helper_draw_horizontal_scale(cairo_t *cr, double x, double y, double length, const char *label, bool right_aligned);

#endif // _INCLUDED_CAIROHELPER_H
