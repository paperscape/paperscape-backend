"""
purpose of db2json.py: Extract meta data and pcite from DB and write to a JSON for reading by nbody
"""

# PYTHON3
import platform
assert platform.python_version_tuple()[0] == '3'

import argparse
from xiwi.py3klib import mysql
from xiwi.py3klib import pciteblob

def do_work(db_cursor, from_year, to_year, category):
    from_id = (from_year - 1800) * 10000000
    to_id = (to_year - 1800) * 10000000
    hits = db_cursor.execute("SELECT meta_data.id,arxiv,allcats,refs FROM meta_data,pcite WHERE meta_data.id=pcite.id AND arxiv IS NOT NULL AND meta_data.id>=%s AND meta_data.id<=%s AND maincat=%s", (from_id, to_id, category))
    #hits = db_cursor.execute("SELECT meta_data.id,arxiv,allcats,refs FROM meta_data,pcite WHERE meta_data.id=pcite.id AND arxiv IS NOT NULL AND meta_data.id>=%s AND meta_data.id<=%s AND (maincat='hep-ph' OR maincat='hep-th' OR maincat='hep-ex' OR maincat='hep-lat')", (from_id, to_id))

    success_refs = 0
    total_refs = 0

    # start JSON array
    print('[')

    for hit_num in range(hits):
        row = db_cursor.fetchone()
        id, arxiv, allcats, refs_blob = row
        refs = pciteblob.decodeBlob(refs_blob)
        arxiv_refs = ['[{},{}]'.format(id, ref_freq) for id, ref_order, ref_freq, num_refs in refs]
        print('{{"id":{},"arxiv":"{}","allcats":"{}","refs":[{}]}}{}'.format(id, arxiv, allcats, ','.join(arxiv_refs), '' if hit_num + 1 == hits else ','))
        # Include no ref info:
        #print('{{"id":{},"arxiv":"{}","allcats":"{}","refs":[]}}{}'.format(id, arxiv, allcats, '' if hit_num + 1 == hits else ','))

    # end JSON array
    print(']')

if __name__ == "__main__":
    cmd_parser = argparse.ArgumentParser(description='Extract meta data and pcite from DB and write to a JSON for reading by nbody')
    cmd_parser.add_argument('--db', metavar='<MySQL database>', help='server name (or localhost) of MySQL database to connect to')
    cmd_parser.add_argument('fromyear', nargs=1, help='from year')
    cmd_parser.add_argument('toyear', nargs=1, help='to year')
    cmd_parser.add_argument('category', nargs=1, help='category')
    args = cmd_parser.parse_args()

    # connect to our meta-data database
    db_connection = mysql.dbconnect(args.db)
    db_cursor = db_connection.cursor()

    # do the work
    do_work(db_cursor, int(args.fromyear[0]), int(args.toyear[0]), args.category[0])

    # close database connection
    db_connection.close()
