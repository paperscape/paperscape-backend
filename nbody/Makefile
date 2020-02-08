CC = gcc
CFLAGS_COMMON = -Wall -ansi -std=gnu99 -O3
LDFLAGS_COMMON =
CFLAGS_GUI = $(shell pkg-config --cflags gtk+-3.0)
LDFLAGS_GUI = $(shell pkg-config --libs gtk+-3.0)

SRC_COMMON = \
	common.c \
	initconfig.c \
	category.c \
	layout.c \
	quadtree.c \
	force.c \
	json.c \
	map.c \
	mapauto.c \

SRC_MYSQL = \
	mysql.c \
	mapmysql.c \

SRC_HEADLESS = \
	headless.c \

SRC_GUI = \
	cairohelper.c \
	mapcairo.c \
	gui.c \

SRC_ALL = \
	$(SRC_COMMON) \
	$(SRC_MYSQL) \
	$(SRC_HEADLESS) \
	$(SRC_GUI) \

OBJ_COMMON = $(SRC_COMMON:.c=.o)
OBJ_MYSQL = $(SRC_MYSQL:.c=.o)
OBJ_HEADLESS = $(SRC_HEADLESS:.c=.o)
OBJ_GUI = $(SRC_GUI:.c=.o)

LIB_COMMON = -lm -lpthread util/xiwilib.a
LIB_MYSQL  = -lmariadb

PROG_HEADLESS = nbody-headless
PROG_GUI = nbody-gui

all: $(PROG_HEADLESS) $(PROG_GUI)

$(PROG_HEADLESS): CFLAGS = $(CFLAGS_COMMON)
$(PROG_HEADLESS): $(OBJ_COMMON) $(OBJ_MYSQL) $(OBJ_HEADLESS)
	$(CC) -o $@ $(OBJ_COMMON) $(OBJ_MYSQL) $(OBJ_HEADLESS) $(LIB_COMMON) $(LIB_MYSQL) $(LDFLAGS)

$(PROG_GUI): CFLAGS = $(CFLAGS_COMMON) $(CFLAGS_GUI)
$(PROG_GUI): $(OBJ_COMMON) $(OBJ_MYSQL) $(OBJ_GUI)
	$(CC) -o $@ $(OBJ_COMMON) $(OBJ_MYSQL) $(OBJ_GUI) $(LIB_COMMON) $(LIB_MYSQL) $(LDFLAGS) $(LDFLAGS_GUI)

# Auto-dependency tracking
# (see http://make.mad-scientist.net/papers/advanced-auto-dependency-generation/)
DEPDIR = .deps
$(shell mkdir -p $(DEPDIR) >/dev/null)

$(DEPDIR)/%.P : %.c
	$(CC) -MM $(CFLAGS_COMMON) $(CFLAGS_GUI) -o $*.d $<
	@sed 's/\($*\)\.o[ :]*/\1.o $*.P : /g' < $*.d > $@; \
		/bin/rm -f $*.d; [ -s $@ ] || /bin/rm -f $@;

ifneq ($(MAKECMDGOALS),clean)
-include $(SRC_ALL:%.c=$(DEPDIR)/%.P)
endif

clean:
	/bin/rm -f $(SRC_ALL:%.c=%.o) $(SRC_ALL:%.c=%.d) $(SRC_ALL:%.c=$(DEPDIR)/%.P)

#clean:
#	/bin/rm $(OBJ_COMMON) $(OBJ_HEADLESS) $(OBJ_GUI)

.PHONY: all clean
