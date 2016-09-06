Paperscape Web Server
=====================

This is the source code of the web server that runs the [Paperscape map](http://paperscape.org) and [My Paperscape](http://my.paperscape.org) projects.
The source code of the [map generation](https://github.com/paperscape/paperscape-mapgen) and [browser-based map client](https://github.com/paperscape/paperscape-mapclient), as well as [Paperscape data](https://github.com/paperscape/paperscape-data), are also available on Github. 
For more details and progress on Paperscape please visit the [development blog](http://blog.paperscape.org).

Installation and Usage
----------------------

The web server is written in [Go](https://golang.org).
It has one external dependency, the [GoMySQL](https://github.com/yanatan16/GoMySQL) package, which must be installed to a location referred to by the environment variable `GOPATH` (see this [introduction to go tool](https://golang.org/doc/code.html)).
Once this dependency has been met, the web server can be built with the command

```shell
go build
```

This should create a binary named after the parent directory.
The web server can be run using the FactCGI or HTTP protocols using the command-line arguments `--fcgi :<port number>` or `--http :<port number>`, respectively.
For example

```shell
./paperscape-webserver --http :8089
```

To see a full list of command-line arguments run

```shell
./paperscape-webserver --help
```

The web server is run on the Paperscape server using the _run-webserver_ script.

Served data
-----------

The web server serves data from a MySQL database containing the following tables:
- *meta_data* - paper meta data
- *pcite* - paper reference and citation information
- *datebdry* - current date boundaries
- *userdata** - user login and saved profile information
- *sharedata** - shared profile link information

The tables with a * are only used by the _My Paperscape_ project ie not the map project.
Detailing their schemas is currently beyond the scope of this documentation.

The web server also serves paper abstracts from a local directory specified by the `--meta` flag.

#### MySQL database access ####

Access to the MySQL database requires the following environment variables to be set:

| Environment variable | Description                                             |
| -------------------- | ------------------------------------------------------- |
| `PSCP_MYSQL_HOST`    | Hostname of the MySQL server e.g. `localhost`           |
| `PSCP_MYSQL_SOCKET`  | Path to MySQL socket e.g. `/var/run/mysqld/mysqld.sock` |
| `PSCP_MYSQL_DB`      | Name of the database to use                             |
| `PSCP_MYSQL_USER`    | Username                                                |
| `PSCP_MYSQL_PWD`     | Password                                                |

If both a socket and hostname are specified, the socket is used.

#### meta_data table ####
_Only relevant fields listed_

| Field      | Type             | Description                                   |
| ---------- |----------------- | --------------------------------------------- |
| id         | int(10) unsigned | Unique paper identifier                       |
| arxiv      | varchar(16)      | Unique arXiv identifier                       |
| maincat    | varchar(8)       | Main arXiv category                           |
| allcats    | varchar(130)     | List of arXiv categories (comma separated)    |
| inspire    | int(8) unsigned  | Inspire record number                         |
| publ       | varchar(200)     | Journal publication information               |
| title      | varchar(500)     | Paper title                                   |
| authors    | text             | Paper authors                                 |

The _id_ field is ordered by publication date (version 1) as follows:
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
| id         | int(10) unsigned | Unique paper identifier                       |
| refs       | blob             | Binary blob encoding references               |
| numRefs    | int(10) unsigned | Number of references                          |
| cites      | blob             | Binary blob encoding citation                 |
| numCites   | int(10) unsigned | Number of citations                           |
| dNumCites1 | tinyint(4)       | Change in number citations past 1 day         |
| dNumCites5 | tinyint(4)       | Change in number citations past 5 days        |

For a given paper (A), a _reference_ is a paper (B) that paper (A) refers to in its text, while a _citation_ is a paper (C) that refers to paper (A) ie a reverse _reference_.

The _refs_ field encodes a list of references in binary, with each reference represented by 10 bytes as follows:

| Reference fields                         | Encoding                                     |
| ---------------------------------------- | -------------------------------------------- |
| id of referenced paper (B)               | unsigned little-endian 32-bit int -> 4 bytes |
| order of (B) in bibliography of (A)      | unsigned little-endian 16-bit int -> 2 bytes |
| frequency - how often (B) appears in (A) | unsigned little-endian 16-bit int -> 2 bytes |
| number of citations referenced paper (B) | unsigned little-endian 16-bit int -> 2 bytes |

Likewise the _cites_ field has a similar encoding:

| Citation fields                          | Encoding                                     |
| ---------------------------------------- | -------------------------------------------- |
| id of citing paper (C)                   | unsigned little-endian 32-bit int -> 4 bytes |
| order of (A) in bibliography of (C)      | unsigned little-endian 16-bit int -> 2 bytes |
| frequency - how often (A) appears in (C) | unsigned little-endian 16-bit int -> 2 bytes |
| number of citations of citing paper (C)  | unsigned little-endian 16-bit int -> 2 bytes |

#### datebdry table ####

| Field   | Type             | Description                       |
| --------| ---------------- | --------------------------------- |
| daysAgo | int(10) unsigned | Number of days ago (0-31)         |
| id      | int(10) unsigned | id corresponding to cut-off       |

The cut-off _id_ does not refer to an actual paper, but is the maximum paper id for that day + 1.
The _id_ for _daysAgo_ = 1 is therefore a lower-bound for all papers of the current (submission) day, and an upper-bound for all papers from the day before.

#### Abstract meta data ####

Paper abstracts are accessed directly from the raw arXiv meta data xml files ([example xml file](http://export.arxiv.org/oai2?verb=GetRecord&identifier=oai:arXiv.org:0804.2273&metadataPrefix=arXivRaw)) available from the [arXiv OAI](http://arxiv.org/help/oa/index).
The root directory of these files can be specified by the `--meta` flag.
If no directory is specified then the server returns "(no abstract)".
The xml files are organized by their arXiv ids into year and month subdirectories ie _YYMM.12345.xml_ is stored as `<--meta dir>/YYxx/YYMM/YYMM.12345.xml`, and _arxiv-cat/YYMM123_ as `<--meta dir>/YYxx/YYMM/arxiv-cat/YYMM123.xml`, etc. 


About the Paperscape map
------------------------

Paperscape is an interactive map that visualises the [arXiv](http://arxiv.org/), an open, online repository for scientific research papers. 
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
---------

The MIT License (MIT)

Copyright (C) 2011-2016 Damien P. George and Robert Knegjens

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

