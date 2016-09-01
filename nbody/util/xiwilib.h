// a mini library of useful types and functions

#ifndef _INCLUDED_MINILIB_H
#define _INCLUDED_MINILIB_H

/** types *******************************************************/

typedef int bool;
enum {
    false = 0,
    true = 1
};

typedef unsigned char byte;

/** memomry allocation ******************************************/

#define m_new(type, num) ((type*)(m_malloc(sizeof(type) * (num))))
#define m_new0(type, num) ((type*)(m_malloc0(sizeof(type) * (num))))
#define m_renew(type, ptr, num) ((type*)(m_realloc((ptr), sizeof(type) * (num))))

void m_free(void *ptr);
void *m_malloc(int num_bytes);
void *m_malloc0(int num_bytes);
void *m_realloc(void *ptr, int num_bytes);

int m_get_total_bytes_allocated();

/** blob ********************************************************/

unsigned short decode_le16(byte *buf);
unsigned int decode_le32(byte *buf);
void encode_le16(byte *buf, unsigned short i);
void encode_le32(byte *buf, unsigned int i);

/** string ******************************************************/

#define streq(s1, s2) (strcmp((s1), (s2)) == 0)

bool strneq(const char *str1, const char *str2, size_t len2);
unsigned int strhash(const char *str);
unsigned int strnhash(const char *str, size_t len);

/** variable string *********************************************/

typedef struct _vstr_t vstr_t;

vstr_t *vstr_new();
void vstr_free(vstr_t *vstr);
void vstr_reset(vstr_t *vstr);
bool vstr_had_error(vstr_t *vstr);
char *vstr_str(vstr_t *vstr);
int vstr_len(vstr_t *vstr);
void vstr_hint_size(vstr_t *vstr, int size);
char *vstr_add_len(vstr_t *vstr, int len);
void vstr_add_str(vstr_t *vstr, const char *str);
void vstr_add_strn(vstr_t *vstr, const char *str, int len);
void vstr_add_byte(vstr_t *vstr, byte v);
void vstr_add_le16(vstr_t *vstr, unsigned short v);
void vstr_add_le32(vstr_t *vstr, unsigned int v);
void vstr_cut_tail(vstr_t *vstr, int len);
void vstr_printf(vstr_t *vstr, const char *fmt, ...);

typedef struct st_mysql MYSQL;
void vstr_add_mysql_real_escape_string(MYSQL *mysql, vstr_t *vstr0, vstr_t *vstr1);

#endif // _INCLUDED_MINILIB_H
