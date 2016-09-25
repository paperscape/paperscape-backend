Paperscape Map Generation
=========================

This is the source code of the backend map generation for the [Paperscape map](http://paperscape.org).
The source code of the [browser-based map client](https://github.com/paperscape/paperscape-mapclient) and [web server](https://github.com/paperscape/paperscape-webserver), as well as [Paperscape data](https://github.com/paperscape/paperscape-data), are also available on Github.

For more details and progress on Paperscape please visit the [development blog](http://blog.paperscape.org).

Map generation using N-body simulation
--------------------------------------

#### Compilation ####

The n-body map generation source code is located in the `nbody/` directory. 
It is written in C.
The map generator can be run with a gui, which is useful for tuning the map, or without one (headless), which is useful for incremental updates on a server.
The corresponding programs that can be built are _nbody-gui_ and _nbody-headless_, respectively.

**Dependencies:** the MySql C library is required by both _nbody-gui_ and _nbody-headless_, while _nbody-gui_ also depends on [Cairo 2D graphics](https://cairographics.org/) and Gtk+ 3.

Before building the nbody programs the utility library _xiwilib_ must first be built by running `make` in the `nbody/util/` directory.

To build the nbody program of your choice, run `make <nbody-program>` in the `nbody/` directory.
That is, choose from

```shell
make nbody-gui
make nbody-headless
```

or simply run `make` to build them all.

#### Basic usage ####

Run any of the nbody programs with the `--help` command-line flag to see a list of command-line options eg

```shell
./nbody-gui --help
```

Both _nbody-gui_ and _nbody-headless_ can read in Json files to set initial configuration settings, category colours and an existing map layout.
Default configuration files are located in the `config/` directory, and also contain comments to explain some of the available features.
Running the _nbody_ programs with no command-line options loads the default configuration settings and category colours for the arXiv map
i.e. the following two commands are equivalent:

```shell
./nbody-gui
./nbody-gui --settings ../config/arxiv-settings.json --categories ../config/arxiv-categories.json
```

This will load all available arXiv papers from the database and begin building a new map.
The default behaviour of _nbody-headless_, on the other hand, is to load an existing layout from the *map_data* MySQL table, check for new papers,
and run a fixed number of iterations.
To load an existing layout from the *map_data* MySQL table in _nbody-gui_ add the flag `--layout-db`.
To instead load an existing map layout from a Json file use `--layout <filename>` in both _nbody_ programs.

Keyboard shortcuts for controlling the map in _nbody-gui_ are printed to the terminal.
Here are some useful keyboard shortcuts:
- By default the view is locked and will adjust its zoom as the graph rotates - the graph rotates to eliminate quadtree artifacts in the force calculation.
To enable manual panning and zooming, toggle the view lock with __V__.
- Pressing ___space___ pauses or resumes graph updates.
- By default a maximum of 100k papers are shown to speed up draw times. To force a full draw of all papers press ___f___. 
- To write the current map layout positions to a Json file press ___J___.
- To draw the current map layout positions to a png image file press ___w___.


Tile and label generation for map
---------------------------------

#### Compilation and basic usage ####

The tile generator source code is located in the `tiles/` directory and is written in [Go](https://golang.org).
It has two external dependencies, the Go packages [go-cairo](github.com/ungerik/go-cairo) and [GoMySQL](https://github.com/yanatan16/GoMySQL), which must be installed to a location referred to by the environment variable `GOPATH` (see this [introduction to go tool](https://golang.org/doc/code.html)).
Once these dependencies have been met, the web server can be built by running the following command in the `tiles/` directory

```shell
go build
```

This should create the program _tiles_, named after its parent directory.

To see a full list of command-line arguments run

```shell
./tiles --help
```

The _tiles_ program requires an output directory to be specified, and by default it will load a paper graph from the *map_data* MySQL table and generate standard tiles and labels for it:

```shell
./tiles <output_dir>
```

The _tiles_ program can read in Json files to set initial configuration settings, category colours and the map layout.
Default configuration files are located in the `config/` directory, and also contain comments to explain some of the available features.
Running the _tiles_ program with no command-line options loads the default configuration settings and category colours for the arXiv map
i.e. the above command is equivalent to:

```shell
./tiles --settings ../config/arxiv-settings.json --categories ../config/arxiv-categories.json <output_dir>
```

In addition to normal tiles, which are coloured according to their categories, it is also possible to generate heatmap and grayscale tiles with the flags `--hm` and `--gs`, respectively.
By default the heatmap tiles are coloured according to their age spectrum. 
An alternative heat parameter can be specified in the configuration file. 



The generated tiles are saved in PNG format and can be optimized slightly to reduce disk space with the _optitiles_ script.
This script can be run on the chosen output directory:

```shell
./optitiles <output_dir>
```

The generated labels are saved as Json files.
A Json file called _world_index.json_ is also generated at the base of the chosen output directory.
It describes the dimensions and location paths of the tiles and labels created for the browser-based map client, which reads this file statically.
To reduce the size of both _world_index.json_ and the generated labels the _gzipjson_ script can be run on the output directory:

```shell
./gzipjson <output_dir>
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
Reading in _keywords_, _title_ and _author_ are currently not supported.

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
