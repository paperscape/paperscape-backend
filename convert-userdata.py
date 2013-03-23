"""
Convert old v:4 papers column to new JSON (tags,notes,graphs) columns.
"""

# we have converted: Thomas' data

import os
import os.path
import argparse
import hashlib
import json
from xiwi.common import mysql

def executeSqlQuery(dbCursor, dryRun, cmd, arg):
    if dryRun:
        print "a dry-run, so not executing the following MySQL command:"
        print cmd, arg
    else:
        try:
            dbCursor.execute(cmd, arg)
        except Exception as e:
            print "WARNING: MySQL: {}".format(str(e))
            return False
    return True

class MiniLexer(object):
    def __init__(self, str):
        self.__str = str
        self.__pos = 0

    def isEnd(self):
        return self.__pos >= len(self.__str)

    def isChar(self, c):
        return self.__pos < len(self.__str) and self.__str[self.__pos] == c

    def getChar(self, c):
        if not self.isChar(c):
            raise Exception("ERROR: expecting character {}".format(c))
        self.__pos += 1

    def getNumber(self):
        neg = False
        if self.isChar('-'):
            neg = True
            self.__pos += 1
        if not self.__str[self.__pos].isdigit():
            raise Exception("ERROR: expecting a digit, found {}".format(self.__str[self.__pos]))
        val = 0
        while self.__pos < len(self.__str) and self.__str[self.__pos].isdigit():
            val = 10 * val + ord(self.__str[self.__pos]) - ord('0')
            self.__pos += 1
        if neg:
            val = -val
        return val

    def getString(self):
        val = []
        self.getChar('"')
        start = self.__pos
        while not self.isChar('"'):
            self.__pos += 1
        end = self.__pos
        self.getChar('"')
        return self.__str[start:end]

def doWork(dbCursor, usermail, dryRun):
    hits = dbCursor.execute("SELECT papers,tags,notes,graphs FROM userdata WHERE usermail=%s", (usermail,))
    if hits != 1:
        print "ERROR: unknown usermail {}".format(usermail)
        return

    # parse papers column

    row = dbCursor.fetchone()
    papers = row[0]
    if not papers.startswith("v:4"):
        print "ERROR: papers column not v:4"
        return

    # these are the colmuns relevant for new profile format
    origTags = row[1]
    origNotes = row[2]
    origGraphs = row[3]
    if len(origGraphs) > 0:
        origGraphsJson = json.loads(origGraphs)
    else:
        origGraphsJson = []

    # parse old papers column
    lexer = MiniLexer(papers[3:])
    papers = []
    while not lexer.isEnd():
        lexer.getChar('(')
        id = lexer.getNumber()
        lexer.getChar(',')
        xpos = lexer.getNumber()
        lexer.getChar(',')
        rmod = lexer.getNumber()
        lexer.getChar(',')
        notes = lexer.getString()
        lexer.getChar(',')

        # labels (old graphs?)
        lexer.getChar('l')
        lexer.getChar('[')
        labels = []
        while not lexer.isChar(']'):
            label = lexer.getString()
            labels.append(label)
            if lexer.isChar(','):
                lexer.getChar(',')
        lexer.getChar(']')
        lexer.getChar(',')

        # tags
        lexer.getChar('t')
        lexer.getChar('[')
        tags = []
        while not lexer.isChar(']'):
            tag = lexer.getString()
            tags.append(tag)
            if lexer.isChar(','):
                lexer.getChar(',')
        lexer.getChar(']')

        lexer.getChar(')')

        papers.append((id, xpos, rmod, notes, tags, labels))
        print id, xpos, rmod, notes, tags, labels

    # make tags column
    tags = {}
    for paper in papers:
        for t in paper[4]:
            if t not in tags:
                tags[t] = [t, 0, [paper[0]]]
            else:
                tags[t][2].append(paper[0])
    tags = tags.values()
    tags.sort(lambda x, y: cmp(x[0], y[0]))
    for i in xrange(len(tags)):
        tags[i][1] = i
        tags[i][2].sort()

    if len(origTags) > 0:
        if len(tags) != 0:
            print "ERROR: cannot combine old and new tags (not implemented)"
            return
        print "using original tags column"
        tagsColumn = origTags
    else:
        tagsColumn = "[{}]".format(','.join("{{\"name\":\"{}\",\"ind\":{},\"blob\":false,\"halo\":false,\"ids\":[{}]}}".format(tagName, tagIndex, ','.join(str(id) for id in tagIds)) for tagName, tagIndex, tagIds in tags))

    # make notes column
    notes = []
    for paper in papers:
        if len(paper[3]) > 0:
            notes.append((paper[0], paper[3]))
    notes.sort(lambda x, y: cmp(x[0], y[0]))

    if len(origNotes) > 0:
        if len(notes) != 0:
            print "ERROR: cannot combine old and new notes (not implemented)"
            return
        print "using original notes column"
        notesColumn = origNotes
    else:
        notesColumn = "[{}]".format(','.join("{{\"id\":{},\"notes\":\"{}\"}}".format(note[0], note[1].replace('/', '\\/')) for note in notes))

    # make graphs column
    graphs = {}
    for paper in papers:
        for l in paper[5]:
            if l not in graphs:
                graphs[l] = ["Old graph {}".format(l), 0, [(paper[0], paper[1], paper[2])]]
            else:
                graphs[l][2].append((paper[0], paper[1], paper[2]))
    graphs = graphs.values()
    graphs.sort(lambda x, y: cmp(x[0], y[0]))
    for i in xrange(len(graphs)):
        graphs[i][1] = len(origGraphsJson) + i
        graphs[i][2].sort(lambda x, y: cmp(x[0], y[0]))

    if len(graphs) == 0:
        print "using original graphs column"
        graphsColumn = origGraphs
    else:
        # graphs of the format: [{"name":"Graph1","ind":<index>,"drawn":[{"id":<id>,"x":<x>,"r":<r>},{..}]},{"name":...},...]
        graphsColumnInner = ','.join("{{\"name\":\"{}\",\"ind\":{},\"drawn\":[{}]}}".format(graphName, graphIndex, ','.join("{{\"id\":{},\"x\":{},\"r\":{}}}".format(id, x, r) for id, x, r in graphPapers)) for graphName, graphIndex, graphPapers in graphs)
        if origGraphs == '[]':
            print "converted {} new graphs".format(len(graphs))
            graphsColumn = '[' + graphsColumnInner + ']'
        elif origGraphs.startswith('[{') and origGraphs.endswith('}]'):
            print "merging {} old graphs with {} new graphs".format(len(origGraphsJson), len(graphs))
            graphsColumn = origGraphs[:-1] + ',' + graphsColumnInner + ']'
        else:
            print "ERROR: origGraphs string is of wrong format"
            return

    # verify our string are valid JSON
    json.loads(tagsColumn)
    json.loads(notesColumn)
    json.loads(graphsColumn)

    # update the userdata table
    executeSqlQuery(dbCursor, dryRun, "UPDATE userdata SET tags=%s, notes=%s, graphs=%s WHERE usermail=%s", (tagsColumn, notesColumn, graphsColumn, usermail))

if __name__ == "__main__":

    cmdParser = argparse.ArgumentParser(description="Convert userdata.")
    cmdParser.add_argument("--db", metavar="<MySQL database>", help="server name (or localhost) of MySQL database to connect to")
    cmdParser.add_argument("--dry-run", action="store_true", help="do not do anything destructive (like modify the database)")
    cmdParser.add_argument("usermail", nargs=1, help="usermail")
    args = cmdParser.parse_args()

    # connect to our meta-data database
    dbConnection = mysql.dbconnect(args.db)
    dbCursor = dbConnection.cursor()

    # do the work
    doWork(dbCursor, args.usermail[0], args.dry_run)

    # close database connection
    dbConnection.close()
