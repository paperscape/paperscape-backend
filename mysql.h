#ifndef _INCLUDED_MYSQL_H
#define _INCLUDED_MYSQL_H

bool mysql_load_papers(const char *where_clause, int *num_papers_out, paper_t **papers_out);
bool mysql_save_paper_positions(int num_papers, paper_t *papers);

#endif // _INCLUDED_MYSQL_H
