"""
Find centre of mass points for popular keywords (experimental).

"""

import math
import argparse
import json
from xiwi.common import mysql

papers = []
keywords = []

class Keyword(object):
    @classmethod
    def make_keyword(cls,keyword):
        for kw in keywords:
            if kw.word == keyword :
                return kw
        kw = Keyword(keyword)
        keywords.append(kw)
        return kw

    def __init__(self,word):
        self.word = word
        self.papers = []

    def add_paper(self,paper):
        self.papers.append(paper)

    def __repr__(self):
        return repr((self.word))

class Paper(object):
    @classmethod
    def make_paper(cls, id, x, y, r):
        # Check if already exists
        for paper in papers :
            if paper.id == id :
                return paper
        paper = Paper(id, x, y, r)
        papers.append(paper)
        return paper

    def __init__(self, id, x, y, r):
        self.id = id
        self.x = x
        self.y = y
        self.r = r
        #self.keywords = keywords


def do_work(db_cursor):
    hits = db_cursor.execute('SELECT map_data.id,map_data.x,map_data.y,map_data.r,keywords.keywords FROM map_data,keywords WHERE map_data.id = keywords.id')
    papers = []
    rows = db_cursor.fetchall()
    for id,x,y,r,kws in rows :
        paper = Paper.make_paper(id,x,y,r)
        kws = kws.split(',')
        for keyword in kws:
            kwObj = Keyword.make_keyword(keyword)
            kwObj.add_paper(paper)
        #for i in range(len(keywords)):
        #    if keywords[i].startswith('Higgs'):
        #        keywords[i] = 'Higgs'

        #print(keywords[-1])
    keywords.sort(key=lambda kw: len(kw.papers))
    keywords.reverse()
    print keywords[0:20]

def main():
    # command line arguments
    cmd_parser = argparse.ArgumentParser(description='Convert userdata.')
    cmd_parser.add_argument('--db', metavar='<MySQL database>', help='server name (or localhost) of MySQL database to connect to')
    args = cmd_parser.parse_args()

    # connect to the database
    db_connection = mysql.dbconnect(args.db, 0)
    db_cursor = db_connection.cursor()

    # do the work
    do_work(db_cursor)

    # close database connection
    db_connection.close()

if __name__ == '__main__':
    main()
