Paperscape Map Generation
=========================

This is the source code of the backend map generation for the [Paperscape map](http://paperscape.org).
The source code of the [browser-based map client](https://github.com/paperscape/paperscape-mapclient) and [web server](https://github.com/paperscape/paperscape-webserver), as well as [Paperscape data](https://github.com/paperscape/paperscape-data), are also available on Github. 
For more details and progress on Paperscape please visit the [development blog](http://blog.paperscape.org).

**NOTE:** while the code is fully functional, this README is still a work in progress.

Map generation using N-body simulation
--------------------------------------

#### Compilation ####

The n-body map generation source code is located in the `nbody/` directory. 
It is written in C.
The map generator can be run with a gui, which is useful for tuning the map, or without one (headless), which is useful for incremental updates on a server.
The corresponding programs that can be built are _nbody-gui_, _nbody-headless_ and _nbody-headlessjson_.
The first two programs read their input data from a MySQL database, while the latter reads in data from a Json file.

**Dependencies:** the MySql C library is required by _nbody-gui_ and _nbody-headless_, while _nbody-gui_ also depends on [Cairo 2D graphics](https://cairographics.org/) and Gtk+ 3.

Before building the nbody programs the utility library _xiwilib_ must first be built by running `make` in the `nbody/util/` directory.

To build the nbody program of your choice, run `make <nbody-program>` in the `nbody/` directory.
That is, choose from

```shell
make nbody-gui
make nbody-headless
make nbody-headlessjson
```

or simply run `make` to build them all.

#### Basic usage ####

Run any of the nbody programs with the `--help` command-line flag to see a list of command-line options eg

```shell
./nbody-gui --help
```

Running _nbody-gui_ with no command-line options,

```shell
./nbody-gui
```

defaults to loading all available arXiv papers from the database and starts building a new map.
Keyboard shortcuts for controlling the map in the gui are printed to the terminal.

Tile and label generation for map
---------------------------------

#### Compilation and basic usage ####

The tile generator source code is located in the `tiles/` directory and is written in [Go](https://golang.org).
It has two external dependencies, the Go packages [go-cairo](github.com/ungerik/go-cairo) and [GoMySQL](https://github.com/yanatan16/GoMySQL), which must be installed to a location referred to by the environment variable `GOPATH` (see this [introduction to go tool](https://golang.org/doc/code.html)).
Once these dependencies have been met, the web server can be built by running the following command in the `tiles/` directory

```shell
go build
```

This should create the binary _tiles_, named after its parent directory.

By default _tiles_ will load a paper graph from the *map_data* MySQL table and generate tiles for it.
The only command-line argument that needs to be specified is an output directory, for example

```shell
./tiles <output_dir>
```

To see a full list of command-line arguments run

```shell
./tiles --help
```

Data formats
------------

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

This table can be used as input by both the n-body map generator and the tile generator.

_Only relevant fields listed_

| Field      | Type             | Description                                   |
| ---------- |----------------- | --------------------------------------------- |
| id         | int(10) unsigned | Unique paper identifier                       |
| allcats    | varchar(130)     | List of categories (comma separated)          |
| keywords   | text             | List of keywords (comma separated)            |
| title      | varchar(500)     | Paper title (for gui display only)            |
| authors    | text             | Paper authors (for gui display only)          |

The `id` field is ordered by publication date (version 1) as follows:
```
ymdh = (year - 1800) * 10000000
       + (month - 1) * 625000
       + (day - 1)   * 15625
unique_id = ymdh + 4*num
```

Categories and keywords are used for creating fake links between disconnected graphs.
Categories are also used for colouring papers in the gui display.

#### pcite table ####

This table can be used as input by both the n-body map generator and the tile generator.

_Only relevant fields listed_

| Field      | Type             | Description                                   |
| ---------- | ---------------- | --------------------------------------------- |
| id         | int(10) unsigned | Unique paper identifier                       |
| refs       | blob             | Binary blob encoding references               |

For a given paper (A), a _reference_ is a paper (B) that paper (A) refers to in its text, while a _citation_ is a paper (C) that refers to paper (A) ie a reverse _reference_.

The _refs_ field encodes a list of references in binary, with each reference represented by 10 bytes as follows:

| Reference fields                         | Encoding                                     |
| ---------------------------------------- | -------------------------------------------- |
| id of referenced paper (B)               | unsigned little-endian 32-bit int -> 4 bytes |
| order of (B) in bibliography of (A)      | unsigned little-endian 16-bit int -> 2 bytes |
| frequency - how often (B) appears in (A) | unsigned little-endian 16-bit int -> 2 bytes |
| number of citations referenced paper (B) | unsigned little-endian 16-bit int -> 2 bytes |

#### map_data table ####

This table can be created as output by the n-body map generator, and used as input to the tile generator.

| Field      | Type             | Description                                   |
| ---------- | ---------------- | --------------------------------------------- |
| id         | int(10) unsigned | Unique paper identifier                       |
| x          | int(11)          | X coordinate in map                           |
| y          | int(11)          | X coordinate in map                           |
| r          | int(11)          | Circle radius in map                          |

#### Json reference data ####

This file format can be used as input by the n-body map generator.
The following Json format is used:

```
[
{"id":input-id,"allcats":"input-category,...","refs":[[input-ref-id,input-ref-freq],...]},
...
]
```

where _input-id_, _input-ref-id_ and _input-ref-freq_ are integers, and _input-category_ is a string.


#### Json map data ####

This file format can be created as output by the n-body map generator, and used as input to the tile generator.
The following Json format is used:

```
[
[input-id,input-x,input-y,input-r],
...
]
```

where _input-id_, _input-x_, _input-y_, and _input-r_ are integers.


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
