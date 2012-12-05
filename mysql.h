#ifndef _INCLUDED_MYSQL_H
#define _INCLUDED_MYSQL_H

bool load_papers_from_mysql(const char *wanted_maincat, int *num_papers_out, paper_t **papers_out);

#endif // _INCLUDED_MYSQL_H
