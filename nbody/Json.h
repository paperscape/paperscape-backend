#ifndef _INCLUDED_JSON_H
#define _INCLUDED_JSON_H

#include "Common.h"

bool Json_load_papers(const char *filename, int *num_papers_out, Common_paper_t **papers_out, Common_keyword_set_t **keyword_set_out);
bool Json_load_other_links(const char *filename, int num_papers, Common_paper_t *papers);

#endif // _INCLUDED_JSON_H
