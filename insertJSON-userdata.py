"""
Insert a new JSON field into userdata column for all users (if not existing)
"""

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
        self.__inString = False
        self.__depth = []

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

    def nextControlChar(self):
        while not self.isEnd() :
            # JSON shouldn't have two consecutive strings
            if self.isChar('"') :
                self.getString()
            if self.isEnd() : break 
            c = self.__str[self.__pos]
            self.__pos += 1
            # Keep account of our depth
            if c == '{' :
                self.__depth.append(0)
            elif c == '}' :
                self.__depth.pop()
            elif c == '[' :
                self.__depth.append(1)
            elif c == ']' :
                self.__depth.pop()
            if c == ',' or c == '{' or c == '}' or c == '[' or c == ']' or c == ':' :
                return c
        return None

    def getControlChar(self,c) :
        while not self.isEnd() :
            if self.nextControlChar() == c :
                return
        raise Exception("ERROR: expecting character {}".format(c))
            

    def insideObject(self) :
        return (len(self.__depth) > 0 and self.__depth[-1] == 0)

    def insideList(self) :
        return (len(self.__depth) > 0 and self.__depth[-1] == 1)

    def __getitem__(self, index):
        if index == "pos" :
            return self.__pos

    def __setitem__(self, index, item):
        if index == "pos" :
            self.__pos = item

def doWork(dbCursor, dryRun, column, afterLabel, insertStr):
    print column, afterLabel, insertStr

    query = "SELECT %s, usermail FROM userdata" % column
    hits = dbCursor.execute(query)
    if hits == 0:
        print "ERROR:"
        return

    columnPerUser = [] 

    row = dbCursor.fetchone()
    while row is not None :
        columnPerUser.append([row[0],row[1]])
        row = dbCursor.fetchone()

    for columnStr, usermail in columnPerUser : 
        insertPositions = []

        # Find positions to insert
        lexer = MiniLexer(columnStr)
        depth = []
        while not lexer.isEnd():
            c = lexer.nextControlChar()
            if lexer.insideObject() and (c == ',' or c == '{') :
                # get label
                label = lexer.getString()
                lexer.getChar(':')
                if afterLabel == "" or label == afterLabel :
                    # Find next insertion point at this depth
                    # (ignore deeper depths ?)
                    while not lexer.isEnd():
                        nc = lexer.nextControlChar()
                        if nc == ',' or nc == '}' :
                            insertPositions.append(lexer["pos"]-1)
                            if nc != '}' :
                                lexer.getControlChar('}')
                            break

        # Do insertion
        for ind in reversed(insertPositions) :
            columnStr = columnStr[:ind] + "," + insertStr + columnStr[ind:]

        try :
            # check valid json:
            json.loads(columnStr)

            executeSqlQuery(dbCursor, dryRun, "UPDATE userdata SET tags=%s WHERE usermail=%s", (columnStr, usermail))
        except Exception as e:
            print "ERROR", e, columnStr


if __name__ == "__main__":

    cmdParser = argparse.ArgumentParser(description="Convert userdata.")
    cmdParser.add_argument("--db", metavar="<MySQL database>", help="server name (or localhost) of MySQL database to connect to")
    cmdParser.add_argument("--dry-run", action="store_true", help="do not do anything destructive (like modify the database)")
    #cmdParser.add_argument("")
    cmdParser.add_argument("column", nargs=1, help="JSON column in userdata to operate on")
    cmdParser.add_argument("--after-label", metavar="<string>", default="", help="insert after this label (useful when order of JSON is strict)")
    cmdParser.add_argument("insert", nargs=1, help="label string to insert into objects in format '\"key\":value'")
    args = cmdParser.parse_args()

    # connect to our meta-data database
    dbConnection = mysql.dbconnect(args.db)
    dbCursor = dbConnection.cursor()

    # do the work
    doWork(dbCursor, args.dry_run, args.column[0], args.after_label, args.insert[0])

    # close database connection
    dbConnection.close()

