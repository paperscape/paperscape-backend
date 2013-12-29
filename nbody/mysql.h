#ifndef _INCLUDED_MYSQL_H
#define _INCLUDED_MYSQL_H

bool mysql_load_papers(const char *where_clause, bool load_authors_and_titles, int *num_papers_out, Common_paper_t **papers_out, Common_keyword_set_t **keyword_set_out);
bool mysql_save_paper_positions(layout_t *layout);
bool mysql_load_paper_positions(layout_t *layout);

#endif // _INCLUDED_MYSQL_H 
