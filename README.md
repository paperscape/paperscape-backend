Paperscape Web Server
=========================

This is the source code for the backend map generation of the <a href="http://paperscape.org">Paperscape</a> map project.

For more details and progress on Paperscape refer to the <a href="http://blog.paperscape.org">development blog</a>.

Mysql data tables
-----------------

If loading from MySQL, the webserver needs the following environment variables set:

| Environment variable | Description                                         |
| -------------------- | --------------------------------------------------- |
| `PSCP_MYSQL_HOST`    | Hostname of the MySQL server e.g. `localhost`       |     
| `PSCP_MYSQL_SOCKET`  | Path to MySQL socket e.g. `/run/mysqld/mysqld.sock` |
| `PSCP_MYSQL_DB`      | Name of the database to use                         |
| `PSCP_MYSQL_USER`    | Username                                            |
| `PSCP_MYSQL_PWD`     | Password                                            |

If both a socket and hostname are specified, the socket is used.

#### meta_data table ####
_Only relevant fields listed_

| Field      | Type             | Description                                   |
| ---------- |----------------- | --------------------------------------------- |
| id         | int(10) unsigned | Unique identifier (see below)                 |
| arxiv      | varchar(16)      | Unique arXiv identifier                       |
| maincat    | varchar(8)       | Main arXiv category                           |
| allcats    | varchar(130)     | List of arXiv categories (comma separated)    |
| inspire    | int(8) unsigned  | Inspire record number                         |
| publ       | varchar(200)     | Journal publication information               |
| title      | varchar(500)     | Paper title                                   |
| authors    | text             | Paper authors                                 |

The `id` field is ordered by publication date as follows:
```
ymdh = (year - 1800) * 10000000
       + (month - 1) * 625000
       + (day - 1)   * 15625
unique_id = ymdh + 4*num
```

#### pcite table ####
_Only relevant fields listed_

| Field      | Type             | Description                                   |
| ---------- | ---------------- | --------------------------------------------- |
| id         | int(10) unsigned | Unique identifier (see above)                 |
| refs       | blob             | Binary blob encoding references (see below)   |
| numRefs    | int(10) unsigned | Number of references                          |
| cites      | blob             | Binary blob encoding citation (see below)     |
| numCites   | int(10) unsigned | Number of citations                           |
| dNumCites1 | tinyint(4)       | Change in number citations past 1 day         |
| dNumCites5 | tinyint(4)       | Change in number citations past 5 days        |

The _refs_ field encodes a list of references in binary, with each reference represented by 10 bytes as follows:

| Reference fields                        | Encoding                                     |
| --------------------------------------- | -------------------------------------------- |
| id                                      | unsigned little-endian 32-bit int -> 4 bytes |
| order in bibliography                   | unsigned little-endian 16-bit int -> 2 bytes |
| frequency  (how often it appears)       | unsigned little-endian 16-bit int -> 2 bytes |
| number of citations of referenced paper | unsigned little-endian 16-bit int -> 2 bytes |

The _cites_ field stores citations the same format.

Installation
------------

Need the following extenral Go package [GoMySQL](https://github.com/yanatan16/GoMySQL) installed and added to `GOPATH`.

```shell
go build
./run-webserver
```

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

