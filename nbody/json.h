#ifndef _INCLUDED_JSON_H
#define _INCLUDED_JSON_H

#include "common.h"

bool json_load_papers(const char *filename, int *num_papers_out, paper_t **papers_out, keyword_set_t **keyword_set_out);
bool json_load_other_links(const char *filename, int num_papers, paper_t *papers);

#endif // _INCLUDED_JSON_H
