"""
Determine the regions of the map and their names.

Input: [{id,x,y,r}], DB.mapskw (id->keywords)
Output: [{x,y,name}]

"""

import math
import argparse
import json
from xiwi.common import mysql

class Paper(object):
    @classmethod
    def make_paper(cls, db_cursor, id, x, y):
        hits = db_cursor.execute('SELECT keywords FROM mapskw WHERE id=%s', (id,))
        if hits != 1:
            keywords = []
        else:
            keywords = db_cursor.fetchone()[0].split(',')
            for i in range(len(keywords)):
                if keywords[i].startswith('Higgs'):
                    keywords[i] = 'Higgs'
        return Paper(id, x, y, keywords)

    def __init__(self, id, x, y, keywords):
        self.id = id
        self.x = x
        self.y = y
        self.keywords = keywords

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

def keywords_for_area(graph, x, y, r, max_kw):
    """Given a position and radius (in world coordinates), returns the keywords for that area in the map."""
    r = r * r
    hist = {}
    for paper in graph.papers:
        if (paper.x - x)**2 + (paper.y - y)**2 < r:
            for kw in paper.keywords:
                try:
                    hist[kw] += 1
                except KeyError:
                    hist[kw] = 1
    keys = []
    for key in hist:
        if hist[key] > 2:
            keys.append((key, hist[key]))
    keys.sort(lambda a, b: cmp(b[1], a[1]))
    area = []
    for i, k in enumerate(keys):
        if i < max_kw:
            area.append(k[0])

    return area

class GridPoint(object):
    def __init__(self, x, y, valid, kws):
        self.x = x
        self.y = y
        self.valid = valid
        self.kws = kws
        self.region = None

class Region(object):
    def __init__(self, id, x, y, kws):
        self.id = id
        self.x = x
        self.y = y
        self.kws = kws

def determine_regions(graph, hex_rad):
    print('creating hex grid')
    hex_w = hex_rad
    hex_h = int(math.sqrt(3) / 2 * hex_rad)
    grid_w = int((graph.max_x - graph.min_x) / hex_w)
    grid_h = int((graph.max_y - graph.min_y) / hex_h)
    grid = []
    for j in range(grid_h):
        if j % 2 == 0:
            odd = 0
        else:
            odd = 1
        for i in range(odd, grid_w):
            y = graph.min_y + j * hex_h
            x = graph.min_x + (i - 0.5 * odd) * hex_w
            kws = keywords_for_area(graph, x, y, hex_rad, 10)

            valid = 0x00
            # points above
            if j >= 1:
                if odd:
                    valid |= 0x03
                else:
                    if i >= 1:
                        valid |= 0x01
                    if i < grid_w - 1:
                        valid |= 0x02
            # points left/right
            if odd:
                if i >= 2:
                    valid |= 0x04
                if i < grid_w - 1:
                    valid |= 0x08
            else:
                if i >= 1:
                    valid |= 0x04
                if i < grid_w - 1:
                    valid |= 0x08
            # points below
            if j < grid_h - 1:
                if odd:
                    valid |= 0x30
                else:
                    if i >= 1:
                        valid |= 0x10
                    if i < grid_w - 1:
                        valid |= 0x20

            # add grid point
            grid.append(GridPoint(x, y, valid, kws))

    # find seed areas:
    #   a hexagon of 6 areas must have at least 2 kw common in their top 10; that defines the name of the area
    print('finding seed areas')
    regions = []
    for offset, g in enumerate(grid):
        if g.valid == 0x3f:
            kws = get_common_keywords7(grid, grid_w, offset)
            if len(kws) != 0:
                region = Region(len(regions), g.x, g.y, kws)
                regions.append(region)
                grid[offset - grid_w].region = region
                grid[offset - grid_w + 1].region = region
                grid[offset - 1].region = region
                grid[offset].region = region
                grid[offset + 1].region = region
                grid[offset + grid_w - 1].region = region
                grid[offset + grid_w].region = region

    # grow areas
    print('growing areas')
    keep_growing = True
    while keep_growing:
        keep_growing = False
        for offset, g in enumerate(grid):
            if g.region is not None:
                if (g.valid & 0x01):
                    if grow_to(g, grid[offset - grid_w]):
                        keep_growing = True
                if (g.valid & 0x02):
                    if grow_to(g, grid[offset - grid_w + 1]):
                        keep_growing = True
                if (g.valid & 0x04):
                    if grow_to(g, grid[offset - 1]):
                        keep_growing = True
                if (g.valid & 0x08):
                    if grow_to(g, grid[offset + 1]):
                        keep_growing = True
                if (g.valid & 0x10):
                    if grow_to(g, grid[offset + grid_w - 1]):
                        keep_growing = True
                if (g.valid & 0x20):
                    if grow_to(g, grid[offset + grid_w]):
                        keep_growing = True

    return regions

def grow_to(g1, g2):
    if g2.region is not None:
        return False
    for kw in g1.region.kws:
        if kw not in g1.kws:
            return False
    g2.region = g1.region
    return True

def get_common_keywords7(grid, grid_w, offset):
    # get the 7 grid points that make a hexagon
    gs = [grid[offset - grid_w], grid[offset - grid_w + 1], grid[offset - 1], grid[offset], grid[offset + 1], grid[offset + grid_w - 1], grid[offset + grid_w]]
    # check all 7 points are unassigned, and build list of keyword sets
    list_of_kws = []
    for g in gs:
        if g.region is not None:
            return []
        list_of_kws.append(g.kws)
    return get_common_keywords(list_of_kws)

def get_common_keywords(list_of_kws):
    # work out common keywords
    kws = [[kw, True] for kw in list_of_kws[0]]
    for i, g in enumerate(list_of_kws):
        if i > 0:
            for kw in kws:
                if kw[0] not in g:
                    kw[1] = False
    kw2 = []
    for kw in kws:
        if kw[1]:
            kw2.append(kw[0])
    return kw2

def do_work(db_cursor, graph_filename):
    with open(graph_filename) as f:
        papers = json.load(f)
    print('read JSON graph data')
    for i, paper in enumerate(papers):
        if i % 10000 == 0:
            print('made {} papers'.format(i))
        papers[i] = Paper.make_paper(db_cursor, paper[0], paper[1], paper[2])
    graph = Graph(papers)
    print('have graph with {} papers'.format(len(graph.papers)))

    regions = determine_regions(graph, 700)

    json_out = []
    for region in regions:
        json_out.append({'x':region.x, 'y':region.y, 'kws':region.kws})
    print json.dumps(json_out, separators=(',', ':'))

def main():
    # command line arguments
    cmd_parser = argparse.ArgumentParser(description='Convert userdata.')
    cmd_parser.add_argument('--db', metavar='<MySQL database>', help='server name (or localhost) of MySQL database to connect to')
    cmd_parser.add_argument('input', nargs=1, help='input JSON file')
    args = cmd_parser.parse_args()

    # connect to the database
    db_connection = mysql.dbconnect(args.db, 0)
    db_cursor = db_connection.cursor()

    # do the work
    do_work(db_cursor, args.input[0])

    # close database connection
    db_connection.close()

if __name__ == '__main__':
    main()
