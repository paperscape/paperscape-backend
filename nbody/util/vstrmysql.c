#include <stdlib.h>
#include <mysql/mysql.h>
#include "xiwilib.h"

void vstr_add_mysql_real_escape_string(MYSQL *mysql, vstr_t *vstr0, vstr_t *vstr1) {
    if (vstr_had_error(vstr0) || vstr_had_error(vstr1)) {
        return;
    }
    int max_len = vstr_len(vstr1) * 2 + 1;
    char *buf = vstr_add_len(vstr0, max_len);
    if (buf == NULL) {
        return;
    }
    int len = mysql_real_escape_string(mysql, buf, vstr_str(vstr1), vstr_len(vstr1));
    vstr_cut_tail(vstr0, max_len - len);
}
