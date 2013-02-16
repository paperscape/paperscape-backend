"""
Purpose of analyse.py: analyse log from wombat and produce a pretty graph.
"""

import re
import argparse
import datetime

knownIpAddr = {
    "62.131.60.99":"Rob home?",
    "82.8.60.38":"Damien home",
    "86.85.14.8":"Rob home?",
    "86.209.15.122":"Annecy",
    "128.141.230.188":"CERN",
    "128.232.140.19":"Cambridge Uni",
    "128.232.140.54":"Cambridge Uni",
    "128.232.141.70":"Cambridge Uni",
    "128.232.141.229":"Cambridge Uni",
    "128.232.143.13":"Cambridge Uni",
    "131.111.16.20":"Cambridge DAMTP",
    "131.111.16.115":"Cambridge DAMTP",
    "131.111.16.196":"Cambridge DAMTP",
    "131.225.23.168":"Fermilab",
    "149.169.90.54":"Arizona State Uni",
    "149.169.222.42":"Arizona State Uni",
    "192.16.197.194":"CWI",
    "192.16.199.150":"Nikhef",
}

def ipLookup(ipAddr):
    if ipAddr not in knownIpAddr:
        print "UNKNOWN:", ipAddr
        knownIpAddr[ipAddr] = "unknown" + str(len(knownIpAddr))
    return knownIpAddr[ipAddr]

def regexMatchLongest(st, regexList):
    """Given a list of (kind, regex), returns (kind, match) corresponding to the largest match, or (None, None)."""
    foundKind = None
    foundMatch = None
    for kind, regex in regexList:
        match = regex.match(st)
        if match and (not foundMatch or match.end() > foundMatch.end()):
            foundKind = kind
            foundMatch = match
    return foundKind, foundMatch

class LogEntry(object):
    wombatLogRegex = re.compile(r"\[(?P<datetime>\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z)\] (?P<ip>[0-9.]+):0 -- (?P<verb>[A-Z]+) (?P<url>[^ ]+) \(bytes: \d+ URL, \d+ content, \d+ replied\)\n$")
    wombatURLRegexList = [
        ("gdata[]", re.compile(r".*wombat.*&gdata%5B%5D=")),
        ("gdb", re.compile(r".*wombat.*&gdb&")),
        ("lload", re.compile(r".*wombat.*&lload=(?P<link>[A-Za-z0-9]+)&")),
        ("pchal", re.compile(r".*wombat.*&pchal=(?P<usermail>[A-Za-z0-9.%\-]+)&")),
        ("pload", re.compile(r".*wombat.*&pload=(?P<usermail>[A-Za-z0-9.%\-]+)&")),
        ("sau", re.compile(r".*wombat.*&sau=(?P<query>[A-Za-z0-9.%\-]+)&")),
        ("sax", re.compile(r".*wombat.*&sax=(?P<query>[A-Za-z0-9.%\-]+)&")),
        ("str[]", re.compile(r".*wombat.*&str%5B%5D=(?P<query>[A-Za-z0-9.%\-]+)&")),
    ]

    def __init__(self, line):
        match = LogEntry.wombatLogRegex.match(line)
        if not match:
            print "ERROR: unknown wombat log line format"
            print line
            raise Exception("unknown wombat log line format")
        dt, self.ipAddr, self.verb, self.url = match.groups()
        self.datetime = datetime.datetime.strptime(dt, "%Y-%m-%dT%H:%M:%SZ")
        self.ipStr = ipLookup(self.ipAddr)
        urlKind, urlMatch = regexMatchLongest(self.url, LogEntry.wombatURLRegexList)
        if urlKind is not None:
            if len(urlMatch.groups()) == 0:
                self.request = urlKind
            else:
                self.request = "{}: {}".format(urlKind, ','.join(arg for arg in urlMatch.groups()))
        else:
            self.request = "unknown wombat request"

def doWork(fileList):
    # parse the files
    logEntries = []
    for fileName in fileList:
        with open(fileName) as f:
            for line in f.readlines():
                if len(line) > 2 and line.startswith('[') and line[1].isdigit():
                    entry = LogEntry(line)
                    if entry is not None:
                        logEntries.append(entry)

    # sort entries on date
    logEntries.sort(lambda x, y: cmp(x.datetime, y.datetime))

    # print out access over time
    for entry in logEntries:
        print entry.datetime, entry.ipStr, entry.request

if __name__ == "__main__":
    cmdParser = argparse.ArgumentParser(description="Analyse output log of wombat.")
    cmdParser.add_argument("files", nargs="+", help="input log files")
    args = cmdParser.parse_args()

    # do the work
    doWork(args.files)
