#include <stdlib.h>

#include "xiwilib.h"

unsigned short decode_le16(byte *buf) {
    return buf[0] | (buf[1] << 8);
}

unsigned int decode_le32(byte *buf) {
    return buf[0] | (buf[1] << 8) | (buf[2] << 16) | (buf[3] << 24);
}

void encode_le16(byte *buf, unsigned short i) {
    buf[0] = i & 0xff;
    buf[1] = (i >> 8) & 0xff;
}

void encode_le32(byte *buf, unsigned int i) {
    buf[0] = i & 0xff;
    buf[1] = (i >> 8) & 0xff;
    buf[2] = (i >> 16) & 0xff;
    buf[3] = (i >> 24) & 0xff;
}
