CC = gcc
CFLAGS_COMMON = -Wall -ansi -std=gnu99 -O3 -I../../clib
LDFLAGS_COMMON =
CFLAGS_GUI = $(shell pkg-config --cflags gtk+-3.0)
LDFLAGS_GUI = $(shell pkg-config --libs gtk+-3.0)

SRC_COMMON = \
	Common.c \
	Layout.c \
	Quadtree.c \
	Force.c \
	map.c \
	map2.c \

SRC_HEADLESS = \
	Mysql.c \
	mapmysql.c \
	headless.c \

SRC_HEADLESSJSON = \
	Json.c \
	headlessjson.c \

SRC_GUI = \
	Mysql.c \
	Cairohelper.c \
	mapmysql.c \
	mapdraw.c \
	gui.c \

OBJ_COMMON = $(SRC_COMMON:.c=.o)
OBJ_HEADLESS = $(SRC_HEADLESS:.c=.o)
OBJ_HEADLESSJSON = $(SRC_HEADLESSJSON:.c=.o)
OBJ_GUI = $(SRC_GUI:.c=.o)

LIB_COMMON = -lm -lpthread ../../clib/xiwilib.a
LIB_HEADLESS = -lmysqlclient
LIB_HEADLESSJSON =
LIB_GUI = -lmysqlclient

PROG_HEADLESS = nbody-headless
PROG_HEADLESSJSON = nbody-headlessjson
PROG_GUI = nbody-gui

$(PROG_HEADLESS): CFLAGS = $(CFLAGS_COMMON)
$(PROG_HEADLESS): $(OBJ_COMMON) $(OBJ_HEADLESS)
	$(CC) -o $@ $(OBJ_COMMON) $(OBJ_HEADLESS) $(LIB_COMMON) $(LIB_HEADLESS) $(LDFLAGS)

all: $(PROG_HEADLESS) $(PROG_HEADLESSJSON) $(PROG_GUI)

$(PROG_HEADLESSJSON): CFLAGS = $(CFLAGS_COMMON)
$(PROG_HEADLESSJSON): $(OBJ_COMMON) $(OBJ_HEADLESSJSON)
	$(CC) -o $@ $(OBJ_COMMON) $(OBJ_HEADLESSJSON) $(LIB_COMMON) $(LIB_HEADLESSJSON) $(LDFLAGS)

$(PROG_GUI): CFLAGS = $(CFLAGS_COMMON) $(CFLAGS_GUI)
$(PROG_GUI): $(OBJ_COMMON) $(OBJ_GUI)
	$(CC) -o $@ $(OBJ_COMMON) $(OBJ_GUI) $(LIB_COMMON) $(LIB_GUI) $(LDFLAGS) $(LDFLAGS_GUI)

clean:
	/bin/rm $(OBJ_COMMON) $(OBJ_HEADLESS) $(OBJ_HEADLESSJSON) $(OBJ_GUI)

.PHONY: all clean depend
