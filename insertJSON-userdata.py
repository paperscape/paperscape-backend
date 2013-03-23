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

def doWork(dbCursor, dryRun):
    hits = dbCursor.execute("SELECT tags, usermail FROM userdata")
    if hits == 0:
        print "ERROR:"
        return

    tagsPerUser = [] 

    row = dbCursor.fetchone()
    while row is not None :
        tagsPerUser.append([row[0],row[1]])
        row = dbCursor.fetchone()

    for i in xrange(len(tagsPerUser)) : 
        tagsStr = tagsPerUser[i][0]
        usermail = tagsPerUser[i][1]
        
        

        try :
            # check valid json:
            json.loads(tagsStr)

            executeSqlQuery(dbCursor, dryRun, "UPDATE userdata SET tags=%s WHERE usermail=%s", (tagsStr, usermail))
        except Exception as e:
            print "ERROR", e, tagsStr


if __name__ == "__main__":

    cmdParser = argparse.ArgumentParser(description="Convert userdata.")
    cmdParser.add_argument("--db", metavar="<MySQL database>", help="server name (or localhost) of MySQL database to connect to")
    cmdParser.add_argument("--dry-run", action="store_true", help="do not do anything destructive (like modify the database)")
    #cmdParser.add_argument("")
    #cmdParser.add_argument("usermail", nargs=1, help="usermail")
    args = cmdParser.parse_args()

    # connect to our meta-data database
    dbConnection = mysql.dbconnect(args.db)
    dbCursor = dbConnection.cursor()

    # do the work
    doWork(dbCursor, args.dry_run)

    # close database connection
    dbConnection.close()

