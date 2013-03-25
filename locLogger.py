#!/usr/bin/env python
import sys
import urllib
import codecs # for uni-code files
import datetime
import time
import argparse

# for geoplugin (xml)
from xml.dom.minidom import parse, parseString

class IpLocation :

    def __init__(self, ip, date):
        self.__ip = ip
        self.__date = date
        self.__city      = "_"
        self.__region    = "_"
        self.__country   = "_"
        self.__latitude  = "_"
        self.__longitude = "_"

    def __getitem__(self, index):
        if index == "ip" :
            return self.__ip
        elif index == "date" :
            return self.__date
        elif index == "city" :
            return self.__city
        elif index == "region" :
            return self.__region
        elif index == "country" :
            return self.__country
        elif index == "latitude" :
            return self.__latitude
        elif index == "longitude" :
            return self.__longitude

    def __setitem__(self, index, item):
        if index == "ip" :
            self.__ip = item
        elif index == "date" :
            self.__date = item
        elif index == "city" :
            self.__city = item
        elif index == "region" :
            self.__region = item
        elif index == "country" :
            self.__country = item
        elif index == "latitude" :
            self.__latitude = item
        elif index == "longitude" :
            self.__longitude = item
    

storedIps = []

def getIpLocation (ip, date) :
    global storedIps
   
    err = False
    
    for ipLoc in storedIps :
        if ipLoc["ip"] == ip :
            # TODO check date ok, otherwise throw out
            return (ipLoc,err)

    ipLoc = IpLocation(ip,date)

    url = "http://www.geoplugin.net/xml.gp?ip=" + ip    
    
    try :       
        sock = urllib.urlopen(url)
        xmlSource = sock.read()
        sock.close()

        dom = parseString(xmlSource)

        try :
            ipLoc["city"] = getFirstTag("geoplugin_city",dom).decode('utf-8')
        except : None
        try :
            ipLoc["region"] = getFirstTag("geoplugin_region",dom).decode('utf-8')
        except : None
        try :
            ipLoc["country"] = getFirstTag("geoplugin_countryName",dom).decode('utf-8')
        except : None
        try: 
            ipLoc["latitude"] = getFirstTag("geoplugin_latitude",dom).decode('utf-8')
        except : None
        try :
            ipLoc["longitude"] = getFirstTag("geoplugin_longitude",dom).decode('utf-8')
        except : None
    
        dom.unlink()
    except :
        err = True
        # This means we've probably thrashed geoplugin server too much
        # TODO implement a break
        print "Parsing error..."
   
    if not err :
        storedIps.append(ipLoc)

    return (ipLoc, err)

# GEOPLUGIN APPROACH
def getFirstTag(tag, xmldoc) :
    nodes = xmldoc.getElementsByTagName(tag)[0].childNodes
    if len(nodes) > 0 : 
        res = nodes[0].data
        return res
    return "EMPTY_TAG"

# the pyLog file attempts to replicate wombat log file, 
# so it looks at the number of lines it has written
# and starts from there

def doWork(inputLog, outputLog, errorLog) :
    
    # first check what is in the output file
    pyLen = 0
    pyLast = ""

    try :
        pyFile = open(outputLog)
        pyLines = pyFile.readlines()
        pyLen = len(pyLines)
        if pyLen > 0 : 
            pyLast = pyLines[-1]
        pyFile.close()
    except : None

    # Place to store ip addresses and their locations

    # Proceeding
    while True :
        
        # Read log file then close
        logFile = open(inputLog,'r')
        logLines = logFile.readlines()
        logFile.close()
        
        for i in range(len(logLines)):
        
            # only relevant if locLog not empty
            # make sure we do not duplicate entries
            if i <= pyLen-1 :
                continue

            #line = logFile.readline()
            line = logLines[i]
            
            # TODO check consistency
            #elif i == pyLen-1 :
            #   if pyLast.split(' ')[0] != (line.split(' ')[0]).split('-')[0] :
            #       print "logTrack.txt is not consistent with pyLog.txt: aborting..."
            #       errFile = open(errPath,"a")
            #       errFile.write(datetime.datetime.now().ctime() + " " + "logTrack.txt is not consistent with pyLog.txt\n")
            #       errFile.close()
            #       break
            #   else :
            #       continue
            
            items = line.split(' ')
        
            # TIME
            logDate = items[0]
            logTime = items[1]
        
            # only proceed if this entry doesn't already exist in pyLog.txt
        
        
            # IP
            ip = (items[2].split(':'))[0]
       
            remainder = (' ').join(items[3:])
            remainder = remainder.split('--')[1].strip().decode('utf-8')

            ipLoc, err = getIpLocation(ip, logDate)
            
            if err :
                errFile = open(errorLog,"a")
                errFile.write(datetime.datetime.now().ctime() + " " + "Parsing error (probably thrashed geoplugin server too much)\n")
                errFile.close()
                time.sleep(360)
        
            # WRITE TO FILE
            # TODO put quotes around fields that may have spaces in them (or change the delimiter)
            DL = " "
            out = logDate + DL + logTime + DL + ip + DL + "--" + DL + remainder + DL + "--" + ipLoc["city"] + DL + ipLoc["region"] + DL + ipLoc["country"] + DL + ipLoc["latitude"] + DL + ipLoc["longitude"]

            output = codecs.open(outputLog,"a","utf-8")
            output.write(out + "\n")
            output.close()
        
            # WRITE TO STDOUT (use ansi colours)
            pageCol = '\033[36m' # cyan
            #if(page == "gravitywar.php") : pageCol = '\033[34m' # blue
            stdout = "\n\033[0m" + logDate + DL + logTime + \
                    DL + '\033[0m' + ip + DL + pageCol + "--" + DL + remainder + DL + "--" +\
                    DL + '\033[32m' + ipLoc["city"] +'\033[0m'+ \
                    DL + ipLoc["region"] + \
                    DL +'\033[33m'+ ipLoc["country"] + '\033[0m' + \
                    DL + ipLoc["latitude"] + DL + ipLoc["longitude"] + "\n"

            sys.stdout.write((stdout).encode("ascii","replace"))


            pyLen = pyLen + 1

        #logFile.close()

        # sleep for x seconds
        time.sleep(30)   
    
if __name__ == "__main__":

    cmdParser = argparse.ArgumentParser(description="Give geo ip data for wombat log")
    cmdParser.add_argument("--dir", metavar="<dir>", default="./", help="Working directory")
    cmdParser.add_argument("--input", metavar="<file>", default="wombat.log", help="Input log file")
    cmdParser.add_argument("--output", metavar="<file>", default="wombat-loc.log", help="Output log file")
    cmdParser.add_argument("--error", metavar="<file>", default="wombat-loc.err", help="Erorr log file")
    args = cmdParser.parse_args()

    # do the work
    doWork(args.dir + args.input,args.dir + args.output, args.dir + args.error)
