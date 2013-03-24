#!/usr/bin/env python
import sys
import urllib
import codecs # for uni-code files
import datetime
import time

# for geoplugin (xml)
from xml.dom.minidom import parse, parseString

# Paths
webDir = "/opt/pscp/logs/"
logsPath = webDir + "wombat.log"
outPath  = webDir + "wombat-loc.log"
errPath  = webDir + "wombat-loc.err"

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


# first check what is in the output file
pyLen = 0
pyLast = ""

try :
    pyFile = open(outPath)
    pyLines = pyFile.readlines()
    pyLen = len(pyLines)
    if pyLen > 0 : 
        pyLast = pyLines[-1]
    pyFile.close()
except : None


# Proceeding
while True :
    

    logFile = open(logsPath)
    logLines = logFile.readlines()
    
    for i in range(len(logLines)):
    
    
        # only relevant if locLog not empty
        # make sure we do not duplicate entries
        if i <= pyLen-1 :
            continue

        #line = logFile.readline()
        line = logLines[i]

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
        remainder = remainder.split('--')[1].strip()

        # LOCATION
        url = "http://www.geoplugin.net/xml.gp?ip=" + ip    
        
        try :       
            sock = urllib.urlopen(url)
            xmlSource = sock.read()
            sock.close()
    
            dom = parseString(xmlSource)
    
            try :
                city = getFirstTag("geoplugin_city",dom)
            except :
                city = "error"
            try :
                region = getFirstTag("geoplugin_region",dom)
            except :
                region = "error"
            try :
                country = getFirstTag("geoplugin_countryName",dom)
            except :
                country = "error"
            try: 
                latitude = getFirstTag("geoplugin_latitude",dom)
            except :
                latitude = "error"
            try :
                longitude = getFirstTag("geoplugin_longitude",dom)
            except :
                longitude = "error"
        
            dom.unlink()
        except :
            # This means we've probably thrashed geoplugin server too much
            # TODO implement a break
            print "Parsing error, aborting..."
            
            city      = "error"
            region    = "error"
            country   = "error"
            latitude  = "error"
            longitude = "error"
            
            errFile = open(errPath,"a")
            errFile.write(datetime.datetime.now().ctime() + " " + "Parsing error (probably thrashed geoplugin server too much)\n")
            errFile.close()
            time.sleep(360)
    
        # WRITE TO FILE
        # TODO put quotes around fields that may have spaces in them (or change the delimiter)
        DL = " "
        out = logDate + DL + logTime + DL + ip + DL + "--" + DL + remainder + DL + "--" + city + DL + region + DL + country + DL + latitude + DL + longitude

        output = codecs.open(outPath,"a","utf-8")
        output.write(out + "\n")
        output.close()
    
        # WRITE TO STDOUT (use ansi colours)
        pageCol = '\033[36m' # cyan
        #if(page == "gravitywar.php") : pageCol = '\033[34m' # blue
        stdout = "\n\033[0m" + logDate + DL + logTime + \
                DL + '\033[0m' + ip + DL + pageCol + "--" + DL + remainder + DL + "--" +\
                DL + '\033[32m' + city +'\033[0m'+ \
                DL + region + \
                DL +'\033[33m'+ country + '\033[0m' + \
                DL + latitude + DL + longitude + "\n"

        sys.stdout.write((stdout).encode("ascii","replace"))


        pyLen = pyLen + 1

    logFile.close()

    # sleep for x seconds
    time.sleep(10)   
    
    
