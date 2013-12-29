#ifndef _INCLUDED_MYSQL_H
#define _INCLUDED_MYSQL_H

#include "Layout.h"

bool Mysql_load_papers(const char *where_clause, bool load_authors_and_titles, int *num_papers_out, Common_paper_t **papers_out, Common_keyword_set_t **keyword_set_out);
bool Mysql_save_paper_positions(Layout_t *layout);
bool Mysql_load_paper_positions(Layout_t *layout);

#endif // _INCLUDED_MYSQL_H 
