#include <stdlib.h>
#include <string.h>

#include "xiwilib.h"

bool strneq(const char *str1, const char *str2, size_t len2) {
    return strncmp(str1, str2, len2) == 0 && str1[len2] == '\0';
}

// uses FNV-1a 32-bit version, http://isthe.com/chongo/tech/comp/fnv/#FNV-1a
unsigned int strhash(const char *str) {
    if (str == NULL) {
        return 0;
    }
    unsigned int hash = 2166136261;
    for (const char *s = str; *s != '\0'; s++) {
        hash = hash ^ ((*s) & 0xff);
        hash = hash * 16777619;
    }
    return hash;
}

// uses FNV-1a 32-bit version, http://isthe.com/chongo/tech/comp/fnv/#FNV-1a
unsigned int strnhash(const char *str, size_t len) {
    if (str == NULL) {
        return 0;
    }
    unsigned int hash = 2166136261;
    for (const char *s = str, *end = str + len; s < end; s++) {
        hash = hash ^ ((*s) & 0xff);
        hash = hash * 16777619;
    }
    return hash;
}
