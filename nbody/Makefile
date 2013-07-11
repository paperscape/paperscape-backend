CC = gcc
CFLAGS = -Wall -ansi -std=gnu99 -O3 -I../../clib
LDFLAGS =
CFLAGS_GUI = $(shell pkg-config --cflags gtk+-3.0)
LDFLAGS_GUI = $(shell pkg-config --libs gtk+-3.0)

SRC_COMMON = \
	common.c \
	layout.c \
	quadtree.c \
	quadtreeforce.c \
	force.c \
	map.c \
	mysql.c \
	init.c \

SRC_HEADLESS = \
	headless.c \

SRC_GUI = \
	mapdraw.c \
	cairohelper.c \
	tile.c \
	gui.c \

OBJ_COMMON = $(SRC_COMMON:.c=.o)
OBJ_HEADLESS = $(SRC_HEADLESS:.c=.o)
OBJ_GUI = $(SRC_GUI:.c=.o)
LIB = -lm -lmysqlclient ../../clib/xiwilib.a
PROG_HEADLESS = nbody-headless
PROG_GUI = nbody-gui

$(PROG_HEADLESS): $(OBJ_COMMON) $(OBJ_HEADLESS)
	$(CC) -o $@ $(OBJ_COMMON) $(OBJ_HEADLESS) $(LIB) $(LDFLAGS)

$(PROG_GUI): $(OBJ_COMMON) $(OBJ_GUI)
	$(CC) -o $@ $(OBJ_COMMON) $(OBJ_GUI) $(LIB) $(LDFLAGS) $(LDFLAGS_GUI)

clean:
	/bin/rm $(OBJ_COMMON) $(OBJ_HEADLESS) $(OBJ_GUI)

depend:
	makedepend -Y $(SRC) 2>/dev/null

