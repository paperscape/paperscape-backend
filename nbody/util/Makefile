CC = gcc
AR = ar
CFLAGS = -Wall -ansi -std=gnu99 -O3

SRC = \
	malloc.c \
	string.c \
	blob.c \
	vstr.c \
	vstrmysql.c \
	jsmn.c \
	hashmap.c \

OBJ = $(SRC:.c=.o)
LIB = xiwilib.a

$(LIB): $(OBJ)
	$(AR) rc $@ $(OBJ)

clean:
	/bin/rm $(OBJ)
