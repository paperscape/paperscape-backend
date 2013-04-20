"""
Generate tiles.

Input: [{id,x,y,r}]
Output: tiles.jpg
"""

import math
import argparse
import json
import cairo
from xiwi.common import mysql

BACKGROUND_PATTERN = cairo.SolidPattern(4.0/15, 5.0/15, 6.0/15)

class Paper(object):
    @classmethod
    def make_paper(cls, db_cursor, id, x, y, r, age):
        hits = db_cursor.execute('SELECT maincat,allcats FROM meta_data WHERE id=%s', (id,))
        if hits != 1:
            maincat = 'unknown'
        else:
            maincat, allcats = db_cursor.fetchone()
            if maincat == 'astro-ph':
                allcats = allcats.split(',')[0]
                if allcats in ['astro-ph.GA', 'astro-ph.CO']:
                    maincat = allcats
        return Paper(id, maincat, x, y, r, age)

    def __init__(self, id, maincat, x, y, r, age):
        self.id = id
        self.maincat = maincat
        self.x = x
        self.y = y
        self.r = r
        self.age = age

        # basic colour of paper
        if maincat == 'hep-th':
            r, g, b = 0, 0, 1
        elif maincat == 'hep-ph':
            r, g, b = 0, 1, 0
        elif maincat == 'hep-ex':
            r, g, b = 1, 1, 0 # yellow
        elif maincat == 'gr-qc':
            r, g, b = 0, 1, 1 # cyan
        elif maincat == 'astro-ph.GA':
            r, g, b = 1, 0, 1 # purple
        elif maincat == 'hep-lat':
            r, g, b = 0.7, 0.36, 0.2 # tan brown
        elif maincat == 'astro-ph.CO':
            r, g, b = 0.62, 0.86, 0.24 # lime green
        elif maincat == 'astro-ph':
            r, g, b = 0.89, 0.53, 0.6 # skin pink
        else:
            r, g, b = 0.7, 1, 0.3

        # background colour
        self.colour_bg = (0.7 + 0.3 * r, 0.7 + 0.3 * g, 0.7 + 0.3 * b)

        # older papers are more saturated in colour
        saturation = 0.4 * (1 - age)

        # foreground colour; newer papers tend towards red
        age = age * age
        r = saturation + (r * (1 - age) + age) * (1 - saturation)
        g = saturation + (g * (1 - age)      ) * (1 - saturation)
        b = saturation + (b * (1 - age)      ) * (1 - saturation)
        self.colour_fg = (r, g, b)

class Graph(object):
    def __init__(self, papers):
        self.papers = papers
        self.min_x = 0
        self.min_y = 0
        self.max_x = 0
        self.max_y = 0
        for paper in papers:
            self.min_x = min(self.min_x, paper.x)
            self.min_y = min(self.min_y, paper.y)
            self.max_x = max(self.max_x, paper.x)
            self.max_y = max(self.max_y, paper.y)
        print('graph has {} papers; min=({},{}), max=({},{})'.format(len(self.papers), self.min_x, self.min_y, self.max_x, self.max_y))

def do_work(db_cursor, pos_filename, kws_filename):
    with open(pos_filename) as f:
        papers = json.load(f)
    #papers = papers[:2000]
    for i, paper in enumerate(papers):
        age = 1.0 * i / len(papers)
        papers[i] = Paper.make_paper(db_cursor, paper[0], paper[1], paper[2], paper[3], age)
    graph = Graph(papers)

    with open(kws_filename) as f:
        kws = json.load(f)

    surface = cairo.ImageSurface(cairo.FORMAT_ARGB32, 8000, 8000) # try RGB24
    make_tile(graph, kws, surface, 0, 0, 0.15, 99)
    surface.finish()
    return

    surface = cairo.ImageSurface(cairo.FORMAT_ARGB32, 800, 800) # try RGB24
    i = 0
    for y in [-2400, -1600, -800, 0, 800, 1600, 2400]:
        for x in [-2400, -1600, -800, 0, 800, 1600, 2400]:
            make_tile(graph, kws, surface, x, y, 0.1, i)
            i += 1
    surface.finish()

def make_tile(graph, kws, surface, x, y, scale, img_id):
    cr = cairo.Context(surface)
    cr.translate(surface.get_width() / 2 + x, surface.get_height() / 2 + y)
    cr.scale(scale, scale)

    # clear background
    cr.set_source(BACKGROUND_PATTERN)
    cr.paint()

    # draw background of papers
    for paper in graph.papers:
        cr.set_source_rgb(paper.colour_bg[0], paper.colour_bg[1], paper.colour_bg[2])
        cr.arc(paper.x, paper.y, 2 * paper.r, 0, 2 * math.pi)
        cr.fill()

    # draw foreground of papers
    for paper in graph.papers:
        cr.set_source_rgb(paper.colour_fg[0], paper.colour_fg[1], paper.colour_fg[2])
        cr.arc(paper.x, paper.y, paper.r, 0, 2 * math.pi)
        cr.fill()
        cr.set_source_rgb(0, 0, 0)
        cr.arc(paper.x, paper.y, paper.r, 0, 2 * math.pi)
        cr.stroke()

    # draw labels
    cr.identity_matrix()
    cr.translate(surface.get_width() / 2 + x, surface.get_height() / 2 + y)
    cr.set_font_size(16)
    for kw in kws:
        cr.set_source_rgb(0, 0, 0)
        cr.move_to(scale * kw['x'], scale * kw['y'])
        cr.show_text(' '.join(kw['kws'][:2]))
        cr.fill()

    surface.write_to_png('out-{:02}.png'.format(img_id))

def main():
    # command line arguments
    cmd_parser = argparse.ArgumentParser(description='Convert userdata.')
    cmd_parser.add_argument('--db', metavar='<MySQL database>', help='server name (or localhost) of MySQL database to connect to')
    cmd_parser.add_argument('pos', nargs=1, help='input JSON file')
    cmd_parser.add_argument('kws', nargs=1, help='input JSON file')
    args = cmd_parser.parse_args()

    # connect to the database
    db_connection = mysql.dbconnect(args.db, 0)
    db_cursor = db_connection.cursor()

    # do the work
    do_work(db_cursor, args.pos[0], args.kws[0])

    # close database connection
    db_connection.close()

if __name__ == '__main__':
    main()
