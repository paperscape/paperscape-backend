Paperscape Map Generation
=========================

This is the source code for the backend map generation of the <a href="http://paperscape.org">Paperscape</a> map project.

For details and progress on Paperscape refer to the <a href="http://blog.paperscape.org">development blog</a>.

Input data formats
==================

Mysql tables
------------

If loading from MySQL, both nbody and tiles need the following environment variables set:

| Environment variable | Description                                         |
| -------------------- | --------------------------------------------------- |
| `PSCP_MYSQL_HOST`    | Hostname of the MySQL server e.g. `localhost`       |     
| `PSCP_MYSQL_SOCKET`  | Path to MySQL socket e.g. `/run/mysqld/mysqld.sock` |
| `PSCP_MYSQL_DB`      | Name of the database to use                         |
| `PSCP_MYSQL_USER`    | Username                                            |
| `PSCP_MYSQL_PWD`     | Password                                            |

If both a socket and hostname are specified, the socket is used.

### meta_data ###
__Only relevant fields listed__

| Field      | Type             | Description                                   |
| ---------- |----------------- | --------------------------------------------- |
| id         | int(10) unsigned | Unique identifier (see below)                 |
| allcats    | varchar(130)     | List of categories (comma separated)          |
| keywords   | text             | List of keywords (comma separated)            |
| title      | varchar(500)     | Paper title (for gui display only)            |
| authors    | text             | Paper authors (for gui display only)          |

The `id` field is ordered by publication date as follows:
```
ymdh = (year - 1800) * 10000000
       + (month - 1) * 625000
       + (day - 1)   * 15625
unique_id = ymdh + 4*num
```

Categories and keywords are used for creating fake links between disconnected graphs.
Categories are also used for colouring papers in the gui display.

### pcite ###
__Only relevant fields listed__

| Field      | Type             | Description                                   |
| ---------- | ---------------- | --------------------------------------------- |
| id         | int(10) unsigned | Unique identifier (see above)                 |
| refs       | blob             | Binary blob encoding references (see below)   |

The `refs` field encodes a list of references in binary, with each reference represented by 10 bytes as follows:
```
4 bytes <- id
2 bytes <- reference order in bibliography
2 bytes <- reference frequency  (how often it appears)
2 bytes <- number of citations of referenced paper 
```

JSON file
---------

TODO: json input format used

Map generation using a N-body simulation
========================================

Installation
------------

Usage
-----


Tile and label generation for map
=================================

Installation
------------

The following external Go packages dependencies need to be installed to a `GOPATH` location:
```
github.com/yanatan16/GoMySQL
github.com/ungerik/go-cairo
```


Usage
-----


About Paperscape
================

Paperscape is an interactive map that visualises the arXiv, an open, online repository for scientific research papers. 
The map, which can be explored by panning and zooming, currently includes all of the papers from the arXiv and is updated daily.

Each scientific paper is represented in the map by a circle whose size is determined by the number of times that paper has been cited by others.
A paper's position in the map is determined by both its citation links (papers that cite it) and its reference links (papers it refers to).
These links pull related papers together, whereas papers with no or few links in common push each other away.

In the default colour scheme, where papers are coloured according to their scientific category, coloured "continents" emerge, such as theoretical high energy physics (blue) or astrophysics (pink).
At their interface one finds cross-disciplinary fields, such as dark matter and cosmological inflation.
Zooming in on a continent reveals substructures representing more specific fields of research.
The automatically extracted keywords that appear on top of papers help to identify interesting papers and fields.

Clicking on a paper reveals its meta data, such as title, authors, journal and abstract, as well as a link to the full text.
It is also possible to view the references or citations for a paper as a star-like background on the map.

Copyright
=========

The MIT License (MIT)
Copyright (C) 2011-2016 Damien P. George and Robert Knegjens

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
