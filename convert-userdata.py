"""
Convert old v:4 papers column to new JSON (tags,notes,graphs) columns.
"""

import os
import os.path
import argparse
import hashlib
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
    hits = dbCursor.execute("SELECT papers FROM userdata WHERE usermail=%s", (usermail,))
    if hits != 1:
        print "ERROR: unknown usermail {}".format(usermail)
        return

    # parse papers column

    papers = dbCursor.fetchone()[0]
    if not papers.startswith("v:4"):
        print "ERROR: papers column not v:4"
        return

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
        lexer.getChar('l')
        lexer.getChar('[')
        lexer.getChar(']')
        lexer.getChar(',')
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

        papers.append((id, xpos, rmod, notes, tags))
        print id, xpos, rmod, notes, tags

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
    
    tagsColumn = "[{}]".format(','.join("{{\"name\":\"{}\",\"ind\":{},\"blob\":false,\"ids\":[{}]}}".format(tagName, tagIndex, ','.join(str(id) for id in tagIds)) for tagName, tagIndex, tagIds in tags))

    # make notes column
    notes = []
    for paper in papers:
        if len(paper[3]) > 0:
            notes.append((paper[0], paper[3]))
    notes.sort(lambda x, y: cmp(x[0], y[0]))

    notesColumn = "[{}]".format(','.join("{{\"id\":{},\"notes\":\"{}\"}}".format(note[0], note[1].replace('/', '\\/')) for note in notes))

    # update the userdata table
    executeSqlQuery(dbCursor, dryRun, "UPDATE userdata SET tags=%s, notes=%s WHERE usermail=%s", (tagsColumn, notesColumn, usermail))

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
