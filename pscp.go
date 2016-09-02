package main

import (
    "io"
    "fmt"
    "net/http"
    "strconv"
    "unicode"
    "encoding/json"
    "bytes"
    "strings"
    "math"
    "sort"
    "log"
)

type Location struct {
    x,y            int
    r              uint
}

type Link struct {
    pastId         uint     // id of the earlier paper
    futureId       uint     // id of the later paper
    pastPaper      *Paper   // pointer to the earlier paper, can be nil
    futurePaper    *Paper   // pointer to the later paper, can be nil
    refOrder       uint     // ordering of this reference made by future to past
    refFreq        uint     // number of in-text references made by future to past
    pastCited      uint      // number of times past paper cited
    futureCited    uint      // number of times future paper cited
    location       *Location // pointer to location, can be nil
}

type Paper struct {
    id         uint     // unique id
    arxiv      string   // arxiv id, simplified
    maincat    string   // main arxiv category
    allcats    string   // all arxiv categories (as a comma-separated string)
    inspire    uint     // inspire record number
    authors    string   // authors
    title      string   // title
    publJSON   string   // publication string in JSON format
    refs       []*Link  // makes references to
    cites      []*Link  // cited by 
    numRefs    uint     // number of times refered to
    numCites   uint     // number of times cited
    dNumCites1 uint     // change in numCites in past day
    dNumCites5 uint     // change in numCites in past 5 days
    location   *Location // pointer to location, can be nil
}

// first is one with smallest id
type PaperSliceSortId []*Paper
func (ps PaperSliceSortId) Len() int           { return len(ps) }
func (ps PaperSliceSortId) Less(i, j int) bool { return ps[i].id < ps[j].id }
func (ps PaperSliceSortId) Swap(i, j int)      { ps[i], ps[j] = ps[j], ps[i] }

type TrendingPaper struct {
    id         uint
    numCites   uint
    score      uint     // trending score
    maincat    string   // main arxiv category
}

type TrendingPaperSortScore []*TrendingPaper
func (tp TrendingPaperSortScore) Len() int           { return len(tp) }
func (tp TrendingPaperSortScore) Less(i, j int) bool { return tp[i].score > tp[j].score }
func (tp TrendingPaperSortScore) Swap(i, j int)      { tp[i], tp[j] = tp[j], tp[i] }

func (h *MyHTTPHandler) ResponsePscpGeneral(rw *MyResponseWriter, req *http.Request) (requestFound bool) {

    requestFound = true

    if req.Form["gdb"] != nil {
        // get-date-boundaries
        success := h.GetDateBoundaries(rw)
        if !success {
            rw.logDescription = fmt.Sprintf("gdb")
        }
    } else if req.Form["gdata[]"] != nil && req.Form["flags[]"] != nil {
        var ids, flags []uint
        for _, strId := range req.Form["gdata[]"] {
            if preId, er := strconv.ParseUint(strId, 10, 0); er == nil {
                ids = append(ids, uint(preId))
            } else {
                log.Printf("ERROR: can't convert id '%s'; skipping\n", strId)
            }
        }
        for _, strId := range req.Form["flags[]"] {
            // Read flags as hex
            if preId, er := strconv.ParseUint(strId, 16, 0); er == nil {
                flags = append(flags, uint(preId))
            } else {
                log.Printf("ERROR: can't convert flag '%s'; skipping\n", strId)
            }
        }
        h.GetDataForIDs(ids,flags,rw)
        rw.logDescription = fmt.Sprintf("gdata (%d)",len(req.Form["gdata[]"]))
    } else if req.Form["chids"] != nil && (req.Form["arx[]"] != nil ||  req.Form["doi[]"] != nil || req.Form["jrn[]"] != nil) {
        // convert-human-ids: convert human IDs to internal IDs
        // arx: list of arxiv IDs
        // jrn: list of journal IDs
        h.ConvertHumanToInternalIds(req.Form["arx[]"],req.Form["doi[]"],req.Form["jrn[]"], rw)
        rw.logDescription = fmt.Sprintf("chids (%d,%d,%d)",len(req.Form["arx[]"]),len(req.Form["doi[]"]),len(req.Form["jrn[]"]))
    } else if req.Form["sge"] != nil {
        // search-general: do fulltext search of authors and titles
        h.SearchGeneral(req.Form["sge"][0], rw)
        rw.logDescription = fmt.Sprintf("sge \"%s\"",req.Form["sge"][0])
    } else if req.Form["skw"] != nil {
        // search-keyword: do fulltext search of keywords
        h.SearchKeyword(req.Form["skw"][0], rw)
        rw.logDescription = fmt.Sprintf("skw \"%s\"",req.Form["skw"][0])
    } else if req.Form["sax"] != nil {
        // search-arxiv: search papers for arxiv number
        h.SearchArxiv(req.Form["sax"][0], rw)
        rw.logDescription = fmt.Sprintf("sax \"%s\"",req.Form["sax"][0])
    } else if req.Form["saxm"] != nil {
        // search-arxiv-minimal: search papers for arxiv number
        // returning minimal information
        h.SearchArxivMinimal(req.Form["saxm"][0], rw)
        rw.logDescription = fmt.Sprintf("saxm \"%s\"",req.Form["saxm"][0])
    } else if req.Form["sau"] != nil {
        // search-author: search papers for authors
        h.SearchAuthor(req.Form["sau"][0], rw)
        rw.logDescription = fmt.Sprintf("sau \"%s\"",req.Form["sau"][0])
    } else if req.Form["sti"] != nil {
        // search-title: search papers for words in title
        h.SearchTitle(req.Form["sti"][0], rw)
        rw.logDescription = fmt.Sprintf("sti \"%s\"",req.Form["sti"][0])
    } else if req.Form["sca"] != nil && (req.Form["f"] != nil || req.Form["fd"] != nil) && (req.Form["t"] != nil || req.Form["td"] != nil) {
        // search-category: search papers between given id range, in given category
        // x = include cross lists, f = from - ID, t = to - ID, 
        // fd = from - numer of days ago, td = to - number of days ago
        var fId, tId string;
        var fd, td uint64;
        if req.Form["f"] != nil {
            fId = req.Form["f"][0]
        } else {
            fId = "0"
        }
        if req.Form["t"] != nil  {
            tId = req.Form["t"][0]
        } else {
            tId = "0"
        }
        if req.Form["fd"] != nil {
            fd, _ = strconv.ParseUint(req.Form["fd"][0], 10, 0)
        }
        if req.Form["td"] != nil {
            td, _ = strconv.ParseUint(req.Form["td"][0], 10, 0)
        }
        h.SearchCategory(req.Form["sca"][0], req.Form["x"] != nil && req.Form["x"][0] == "true", fId, tId, uint(fd), uint(td), rw)
        rw.logDescription = fmt.Sprintf("sca \"%s\" (%s,%s,%d,%d)", req.Form["sca"][0], fId,tId,fd,td)
    } else if req.Form["snp"] != nil && req.Form["f"] != nil && req.Form["t"] != nil {
        // search-new-papers: search papers between given id range
        // f = from, t = to
        h.SearchNewPapers(req.Form["f"][0], req.Form["t"][0], rw)
        rw.logDescription = fmt.Sprintf("snp (%s,%s)", req.Form["f"][0], req.Form["t"][0])
    } else if req.Form["str[]"] != nil {
        // search-trending: search papers that are "trending"
        h.SearchTrending(req.Form["str[]"], rw)
        // this is actually interesting info:
        var buf bytes.Buffer
        for i, str := range req.Form["str[]"] {
            if i > 0 {
                buf.WriteString(",")
            }
            buf.WriteString(str)
        }
        rw.logDescription = fmt.Sprintf("str \"%s\"",buf.String())
    } else {
        requestFound = false
    }

    return
}

func (h *MyHTTPHandler) ResponsePscpMap(rw *MyResponseWriter, req *http.Request) (requestFound bool) {

    requestFound = true
    
    if req.Form["mp2l[]"] != nil && req.Form["tbl"] != nil {
        // map: paper ids to locations
        var ids []uint64
        for _, strId := range req.Form["mp2l[]"] {
            if preId, er := strconv.ParseUint(strId, 10, 0); er == nil {
                ids = append(ids, uint64(preId))
            } else {
                log.Printf("ERROR: can't convert id '%s'; skipping\n", strId)
            }
        }
        rw.logDescription = fmt.Sprintf("Paper ids to map locations for")
        h.MapLocationFromPaperIds(ids,req.Form["tbl"][0],rw)
    } else if req.Form["mr2l"] != nil && req.Form["tbl"] != nil {
        // mr2l: arxiv id to references (incl locations) 
        rw.logDescription = fmt.Sprintf("mr2l \"%s\"",req.Form["mr2l"][0])
        h.MapReferencesFromArxivId(req.Form["mr2l"][0],req.Form["tbl"][0],rw)
    } else if req.Form["mc2l"] != nil && req.Form["tbl"] != nil {
        // mc2l: arxiv id to citations (incl locations) 
        rw.logDescription = fmt.Sprintf("m2cl \"%s\"",req.Form["mc2l"][0])
        h.MapCitationsFromArxivId(req.Form["mc2l"][0],req.Form["tbl"][0],rw)
    } else if req.Form["ml2p[]"] != nil && req.Form["tbl"] != nil {
        // map: location to paper id
        x, erx := strconv.ParseFloat(req.Form["ml2p[]"][0], 0)
        y, ery := strconv.ParseFloat(req.Form["ml2p[]"][1], 0)
        if erx != nil || ery != nil || math.IsNaN(x) || math.IsNaN(y) || math.IsInf(x, 0) || math.IsInf(y, 0) {
            // error parsing x and/or y
            rw.logDescription = fmt.Sprintf("Paper id from map location, invalid location (%s, %s)", req.Form["ml2p[]"][0], req.Form["ml2p[]"][1])
        } else {
            // parsed coordinates ok, do request
            rw.logDescription = fmt.Sprintf("Paper id from map location (%.2f, %.2f)", x, y)
            h.MapPaperIdAtLocation(x, y, req.Form["tbl"][0], rw)
        }
    } else if req.Form["gdmv"] != nil {
        // get-date-maps-version
        success := h.GetDateMapsVersion(rw)
        if !success {
            rw.logDescription = fmt.Sprintf("gdmv")
        }
    } else {
        requestFound = false
    }

    return
}


func (h *MyHTTPHandler) GetDataForIDs(ids []uint, flags []uint, rw http.ResponseWriter) {
    if len(ids) != len(flags) {
        log.Printf("ERROR: GetDataForIDs has length mismatch between ids and flags\n")
        fmt.Fprintf(rw, "null")
        return
    }
    // Get date boundary
    row := h.papers.QuerySingleRow("SELECT id FROM datebdry WHERE daysAgo = 5")
    h.papers.QueryEnd()
    if row == nil {
        log.Printf("ERROR: GetNewCitesAndUpdateMetas could not get 5 day boundary from MySQL\n")
        fmt.Fprintf(rw, "[]")
        return
    }
    var ok bool
    var db uint64
    if db, ok = row[0].(uint64); !ok {
        log.Printf("ERROR: GetNewCitesAndUpdateMetas could not get 5 day boundary from Row\n")
        fmt.Fprintf(rw, "[]")
        return
    }

    fmt.Fprintf(rw, "{\"papr\":[")
    first := true
    for i, _ := range ids {
        id := ids[i]
        flag := flags[i]
        paper := h.papers.QueryPaper(id, "")
        // check the paper exists
        if paper == nil {
            log.Printf("ERROR: GetDataForIDs could not find paper for id %d; skipping\n", id)
            continue
        }
        if !first {
            fmt.Fprintf(rw, ",")
        } else {
            first = false
        }
        fmt.Fprintf(rw, "{\"id\":%d", paper.id)
        if flag & 0x01 != 0 {
            // Meta
            fmt.Fprintf(rw, ",")
            printJSONMetaInfo(rw, paper)
        } else if flag & 0x02 != 0 {
            // Update meta
            fmt.Fprintf(rw, ",")
            printJSONUpdateMetaInfo(rw, paper)
        }
        if flag & 0x04 != 0 {
            // All refs 
            h.papers.QueryRefs(paper, false)
            fmt.Fprintf(rw, ",")
            printJSONAllRefs(rw, paper,false)
        }
        if flag & 0x08 != 0 {
            // All cites
            h.papers.QueryCites(paper, false)
            fmt.Fprintf(rw, ",")
            printJSONAllCites(rw, paper, 0,false)
        } else if flag & 0x10 != 0 {
            // New cites
            h.papers.QueryCites(paper, false)
            if len(paper.cites) < 26 {
                fmt.Fprintf(rw, ",")
                printJSONAllCites(rw, paper, 0,false)
            } else {
                fmt.Fprintf(rw, ",")
                printJSONNewCites(rw, paper, uint(db),false)
            }
        }
        if flag & 0x20 != 0 {
            // Abstract
            abs, _ := json.Marshal(h.papers.GetAbstract(paper.id))
            fmt.Fprintf(rw, ",")
            fmt.Fprintf(rw,"\"abst\":%s",abs)
        }

        fmt.Fprintf(rw, "}")
    }
    fmt.Fprintf(rw, "]}")
}

/****************************************************************/

func (h *MyHTTPHandler) SearchArxiv(arxivString string, rw http.ResponseWriter) {
    // check for valid characters in arxiv string
    for _, r := range arxivString {
        if !(unicode.IsLetter(r) || unicode.IsNumber(r) || r == '-' || r == '/' || r == '.') {
            // invalid character
            return
        }
    }

    // query the paper and its refs and cites
    paper := h.papers.QueryPaper(0, arxivString)
    h.papers.QueryRefs(paper, false)
    h.papers.QueryCites(paper, false)

    // check the paper exists
    if paper == nil {
        return
    }

    // print the json output
    fmt.Fprintf(rw, "{\"papr\":[{\"id\":%d,", paper.id)
    printJSONMetaInfo(rw, paper)
    fmt.Fprintf(rw, ",")
    printJSONAllRefs(rw, paper,false)
    fmt.Fprintf(rw, ",")
    printJSONAllCites(rw, paper, 0,false)
    fmt.Fprintf(rw, "}]}")
}

// Same as above, but returns less details
func (h *MyHTTPHandler) SearchArxivMinimal(arxivString string, rw http.ResponseWriter) {
    // check for valid characters in arxiv string
    for _, r := range arxivString {
        if !(unicode.IsLetter(r) || unicode.IsNumber(r) || r == '-' || r == '/' || r == '.') {
            // invalid character
            return
        }
    }

    // query the paper and its refs and cites
    paper := h.papers.QueryPaper(0, arxivString)

    // check the paper exists
    if paper == nil {
        return
    }

    // print the json output
    fmt.Fprintf(rw, "[{\"id\":%d,\"nc\":%d}]", paper.id, paper.numCites)
}


func (h *MyHTTPHandler) SearchKeyword(searchString string, rw http.ResponseWriter) {

    stmt := h.papers.StatementBegin("SELECT meta_data.id,pcite.numCites FROM meta_data,pcite WHERE meta_data.id = pcite.id AND MATCH(meta_data.keywords) AGAINST (?) LIMIT 150",h.papers.db.Escape(searchString))

    var id uint64
    var numCites uint

    numResults := 0
    fmt.Fprintf(rw, "[")
    if stmt != nil {
        //stmt.BindResult(&id,&numCites,&refStr)
        stmt.BindResult(&id,&numCites)
        for {
            eof, err := stmt.Fetch()
            if err != nil {
                fmt.Println("MySQL statement error;", err)
                break
            } else if eof { break }
            if numResults > 0 {
                fmt.Fprintf(rw, ",")
            }
            //fmt.Fprintf(rw, "{\"id\":%d,\"nc\":%d,\"o\":%d,\"ref\":", id, numCites,numResults)
            //ParseRefsCitesStringToJSONListOfIds(refStr, rw)
            //fmt.Fprintf(rw, "}")
            fmt.Fprintf(rw, "{\"id\":%d,\"nc\":%d,\"o\":%d}", id, numCites,numResults)
            numResults += 1
        }
        err := stmt.FreeResult()
        if err != nil {
            fmt.Println("MySQL statement error;", err)
        }
    }
    h.papers.StatementEnd(stmt) 
    fmt.Fprintf(rw, "]")
}

func (h *MyHTTPHandler) SearchGeneral(searchString string, rw http.ResponseWriter) {


    newWord := true
    var booleanSearchString bytes.Buffer
    for _, r := range h.papers.db.Escape(searchString) {
        if unicode.IsSpace(r) || r == '\'' || r == '+' || r == '\\' {
            // this characted is a word separator
            // "illegal" characters are considered word separators
            if !newWord {
                booleanSearchString.WriteRune('"')
            }
            newWord = true;
        } else {
            if newWord {
                booleanSearchString.WriteString(" +\"")
                newWord = false
            }
            booleanSearchString.WriteRune(r)
        }
    }
    if !newWord {
        booleanSearchString.WriteRune('"')
    }

    //stmt := h.papers.StatementBegin("SELECT meta_data.id,pcite.numCites FROM meta_data,pcite WHERE meta_data.id = pcite.id AND MATCH(meta_data.authors,meta_data.title) AGAINST (?) LIMIT 150",h.papers.db.Escape(searchString))
    stmt := h.papers.StatementBegin("SELECT meta_data.id,pcite.numCites FROM meta_data,pcite WHERE (meta_data.id = pcite.id) AND (MATCH(meta_data.authors,meta_data.keywords) AGAINST (? IN BOOLEAN MODE)) LIMIT 150",booleanSearchString.String())

    var id uint64
    var numCites uint
    //var refStr []byte
    
    numResults := 0
    fmt.Fprintf(rw, "[")
    if stmt != nil {
        //stmt.BindResult(&id,&numCites,&refStr)
        stmt.BindResult(&id,&numCites)
        for {
            eof, err := stmt.Fetch()
            if err != nil {
                fmt.Println("MySQL statement error;", err)
                break
            } else if eof { break }
            if numResults > 0 {
                fmt.Fprintf(rw, ",")
            }
            //fmt.Fprintf(rw, "{\"id\":%d,\"nc\":%d,\"o\":%d,\"ref\":", id, numCites,numResults)
            //ParseRefsCitesStringToJSONListOfIds(refStr, rw)
            //fmt.Fprintf(rw, "}")
            fmt.Fprintf(rw, "{\"id\":%d,\"nc\":%d,\"o\":%d}", id, numCites,numResults)
            numResults += 1
        }
        err := stmt.FreeResult()
        if err != nil {
            fmt.Println("MySQL statement error;", err)
        }
    }
    h.papers.StatementEnd(stmt) 
    fmt.Fprintf(rw, "]")
}

// TODO use prepared statements to gaurd against sql injection
func (h *MyHTTPHandler) SearchAuthor(authors string, rw http.ResponseWriter) {
    // turn authors into boolean search terms
    // add surrounding double quotes for each author in case they have initials with them
    newWord := true
    var searchString bytes.Buffer
    for _, r := range authors {
        if unicode.IsSpace(r) || r == '\'' || r == '+' || r == '\\' {
            // this characted is a word separator
            // "illegal" characters are considered word separators
            if !newWord {
                searchString.WriteRune('"')
            }
            newWord = true;
        } else {
            if newWord {
                searchString.WriteString(" +\"")
                newWord = false
            }
            searchString.WriteRune(r)
        }
    }
    if !newWord {
        searchString.WriteRune('"')
    }

    // do the search
    h.SearchGeneric("MATCH (authors) AGAINST ('" + searchString.String() + "' IN BOOLEAN MODE)", rw)
}

// TODO use prepared statements to gaurd against sql injection
func (h *MyHTTPHandler) SearchTitle(titleWords string, rw http.ResponseWriter) {
    // turn title words into boolean search terms
    newWord := true
    var searchString bytes.Buffer
    for _, r := range titleWords {
        if unicode.IsSpace(r) || r == '\'' || r == '+' || r == '\\' {
            // this characted is a word separator
            // "illegal" characters are considered word separators
            newWord = true;
        } else {
            if newWord {
                searchString.WriteString(" +")
                newWord = false
            }
            searchString.WriteRune(r)
        }
    }

    // do the search
    h.SearchGeneric("MATCH (title) AGAINST ('" + searchString.String() + "' IN BOOLEAN MODE)", rw)
}

func sanityCheckId(id string) bool {
    if len(id) == 1 && id[0] == '0' {
        // just '0' is okay
        return true
    }
    if len(id) != 10 {
        // not correct length
        return false
    }
    for _, r := range id {
        if !unicode.IsDigit(r) {
            // illegal character
            return false
        }
    }
    return true
}

// TODO use prepared statements to gaurd against sql injection
// searches for all papers within the id range, with main category as given
// returns id, numCites, refs for up to 500 results
func (h *MyHTTPHandler) SearchCategory(category string, includeCrossLists bool, idFrom string, idTo string, daysagoFrom uint, daysagoTo uint, rw http.ResponseWriter) {
    // sanity check of category, and build query
    // comma is used to separate multiple categories, which means "or"

    // choose the type of MySQL query based on whether we want cross-lists or not
    var catQueryStart string
    var catQueryEnd string
    if includeCrossLists {
        // include cross lists; check "allcats" column for any occurrence of the wanted category string
        catQueryStart = "meta_data.allcats LIKE '%%"
        catQueryEnd = "%%'"
    } else {
        // no cross lists; "maincat" column must match the wanted category exactly
        catQueryStart = "meta_data.maincat='"
        catQueryEnd = "'"
    }

    var catQuery bytes.Buffer
    catQuery.WriteString("(")
    catQuery.WriteString(catQueryStart)
    catChars := 0
    for _, r := range category {
        if r == ',' {
            if catChars < 2 {
                // bad category
                return
            }
            catQuery.WriteString(catQueryEnd)
            catQuery.WriteString(" OR ")
            catQuery.WriteString(catQueryStart)
            catChars = 0
        } else if unicode.IsLower(r) || r == '-' {
            catQuery.WriteRune(r)
            catChars += 1
        } else {
            // illegal character
            return
        }
    }
    if catChars < 2 {
        // bad category
        return
    }
    catQuery.WriteString(catQueryEnd)
    catQuery.WriteString(")")

    if (daysagoFrom <= 0 && daysagoFrom > 31) { daysagoFrom = 0 }
    if (daysagoTo <= 0 && daysagoTo > 31) { daysagoTo = 0 }
    
    // if given non-trivial "daysago" number to lookup
    if daysagoFrom > daysagoTo {
        stmt := h.papers.StatementBegin("SELECT daysAgo,id FROM datebdry WHERE daysAgo = ? OR daysAgo = ?",daysagoFrom,daysagoTo)

        var id uint64
        var results [2]uint64
        var daysAgo uint
        
        if stmt != nil {
            stmt.BindResult(&daysAgo,&id)
            for i, _ := range results {
                eof, err := stmt.Fetch()
                if err != nil {
                    fmt.Println("MySQL statement error;", err)
                    break
                } else if eof { break }
                results[i] = id 
            }
            err := stmt.FreeResult()
            if err != nil {
                fmt.Println("MySQL statement error;", err)
            }
        }
        h.papers.StatementEnd(stmt) 
        
        if results[0] > results[1] {
            idFrom = strconv.FormatUint(results[1],10)
            idTo = strconv.FormatUint(results[0],10)
        } else {
            idFrom = strconv.FormatUint(results[0],10)
            idTo = strconv.FormatUint(results[1],10)
        }
    }

    // sanity check of id numbers
    if !sanityCheckId(idFrom) || idFrom == "0" {
        return
    }
    if !sanityCheckId(idTo) {
        return
    }

    // a top of 0 means infinitely far into the future
    if idTo == "0" {
        idTo = "4000000000"
    }

    // do the search
    h.SearchGeneric("meta_data.id >= " + idFrom + " AND meta_data.id <= " + idTo + " AND " + catQuery.String(), rw)
}

// searches for papers using the given where-clause
// builds a JSON list with id, numCites, refs for up to 500 results
func (h *MyHTTPHandler) SearchGeneric(whereClause string, rw http.ResponseWriter) {
    // build basic query
    query := "SELECT meta_data.id,pcite.numCites FROM meta_data,pcite WHERE meta_data.id=pcite.id AND (" + whereClause + ")"

    // don't include results that we have no way of uniquely identifying (ie must have arxiv or publ info)
    query += " AND (meta_data.arxiv IS NOT NULL OR meta_data.publ IS NOT NULL)"

    // limit 500 results
    query += " LIMIT 500"

    // do the query
    if !h.papers.QueryBegin(query) {
        fmt.Fprintf(rw, "[]")
        return
    }

    defer h.papers.QueryEnd()

    // get result set  
    result, err := h.papers.db.UseResult()
    if err != nil {
        fmt.Println("MySQL use result error;", err)
        fmt.Fprintf(rw, "[]")
        return
    }

    // get each row from the result and create the JSON object
    fmt.Fprintf(rw, "[")
    numResults := 0
    for {
        row := result.FetchRow()
        if row == nil {
            break
        }

        var ok bool
        var id uint64
        var numCites uint64
        //var refStr []byte
        if id, ok = row[0].(uint64); !ok { continue }
        if numCites, ok = row[1].(uint64); !ok { numCites = 0 }
        //if refStr, ok = row[2].([]byte); !ok { /* refStr is empty, that's okay */ }

        if numResults > 0 {
            fmt.Fprintf(rw, ",")
        }
        //fmt.Fprintf(rw, "{\"id\":%d,\"nc\":%d,\"ref\":", id, numCites)
        //ParseRefsCitesStringToJSONListOfIds(refStr, rw)
        //fmt.Fprintf(rw, "}")
        fmt.Fprintf(rw, "{\"id\":%d,\"nc\":%d}", id, numCites)
        numResults += 1
    }
    fmt.Fprintf(rw, "]")
}

// searches for all new papers within the id range
// returns id,allcats,numCites,refs for up to 500 results
func (h *MyHTTPHandler) SearchNewPapers(idFrom string, idTo string, rw http.ResponseWriter) {
    // sanity check of id numbers
    if !sanityCheckId(idFrom) {
        return
    }
    if !sanityCheckId(idTo) {
        return
    }

    // a top of 0 means infinitely far into the future
    if idTo == "0" {
        idTo = "4000000000";
    }

    if !h.papers.QueryBegin("SELECT meta_data.id,meta_data.allcats,pcite.numCites FROM meta_data,pcite WHERE meta_data.id >= " + idFrom + " AND meta_data.id <= " + idTo + " AND meta_data.id = pcite.id LIMIT 500") {
        fmt.Fprintf(rw, "[]")
        return
    }

    defer h.papers.QueryEnd()

    // get result set  
    result, err := h.papers.db.UseResult()
    if err != nil {
        fmt.Println("MySQL use result error;", err)
        fmt.Fprintf(rw, "[]")
        return
    }

    // get each row from the result and create the JSON object
    fmt.Fprintf(rw, "[")
    numResults := 0
    for {
        row := result.FetchRow()
        if row == nil {
            break
        }

        var ok bool
        var id uint64
        var allcats string
        var numCites uint64
        //var refStr []byte
        if id, ok = row[0].(uint64); !ok { continue }
        if allcats, ok = row[1].(string); !ok { continue }
        if numCites, ok = row[2].(uint64); !ok { numCites = 0 }
        //if refStr, ok = row[3].([]byte); !ok { }

        if numResults > 0 {
            fmt.Fprintf(rw, ",")
        }
        //fmt.Fprintf(rw, "{\"id\":%d,\"cat\":\"%s\",\"nc\":%d,\"ref\":", id, allcats, numCites)
        //ParseRefsCitesStringToJSONListOfIds(refStr, rw)
        //fmt.Fprintf(rw, "}")
        fmt.Fprintf(rw, "{\"id\":%d,\"cat\":\"%s\",\"nc\":%d}", id, allcats, numCites)
        numResults += 1
    }
    fmt.Fprintf(rw, "]")
}

// searches for trending papers
// returns list of id and numCites
func (h *MyHTTPHandler) SearchTrending(categories []string, rw http.ResponseWriter) {

    // create sql statement dynamically based on number of categories
    var args bytes.Buffer
    args.WriteString("(")
    for i, _ := range categories {
        if i > 0 { 
            args.WriteString(",")
        }
        args.WriteString("?")
    }
    args.WriteString(")")
    sql := fmt.Sprintf("SELECT field,value FROM misc WHERE field IN %s LIMIT %d",args.String(),len(categories))

    // create interface of arguments for statement
    catsInterface := make([]interface{},len(categories))
    for i, category := range categories {
        catsInterface[i] = interface{}(h.papers.db.Escape(category))
    }

    // collect in object list
    var trendingPapers []*TrendingPaper

    // Execute statement
    stmt := h.papers.StatementBegin(sql,catsInterface...)
    var value,field string
    if stmt != nil {
        stmt.BindResult(&field,&value)
        for {
            eof, err := stmt.Fetch()
            if err != nil {
                fmt.Println("MySQL statement error;", err)
                break
            } else if eof { break }
            items := strings.Split(value, ",")
            for i := 0; i + 2 < len(items); i += 3 {
                var id,score,nc uint64
                var err error
                id, err  = strconv.ParseUint(items[i], 10, 0)
                if err != nil { continue }
                score, err = strconv.ParseUint(items[i+1], 10, 0)
                if err != nil { continue }
                nc, err  = strconv.ParseUint(items[i+2], 10, 0)
                if err != nil { continue }
                tp := &TrendingPaper{uint(id),uint(nc),uint(score),field}
                trendingPapers = append(trendingPapers,tp)
            }
        }
        err := stmt.FreeResult()
        if err != nil {
            fmt.Println("MySQL statement error;", err)
        }
    }
    h.papers.StatementEnd(stmt) 

    sort.Sort(TrendingPaperSortScore(trendingPapers))    

    // create the JSON object
    fmt.Fprintf(rw, "[")
    for i, trendingPaper := range(trendingPapers) {
        // cap it at 10
        if i >= 10 { break }
        if i > 0 {
            fmt.Fprintf(rw, ",")
        }
        if trendingPaper.maincat == "top10" || trendingPaper.maincat == "none" {
            fmt.Fprintf(rw, "{\"id\":%d,\"nc\":%d}", trendingPaper.id, trendingPaper.numCites)
        } else {
            fmt.Fprintf(rw, "{\"id\":%d,\"nc\":%d,\"mc\":\"%s\"}", trendingPaper.id, trendingPaper.numCites, trendingPaper.maincat)
        }
    }
    fmt.Fprintf(rw, "]")
}

/****************************************************************/

func (h *MyHTTPHandler) MapLocationFromPaperIds(ids []uint64, tableSuffix string, rw http.ResponseWriter) {
    
    var x,y int
    var r uint 
    var resId uint64

    fmt.Fprintf(rw, "[")
    
    if len(ids) == 0 { 
        fmt.Fprintf(rw, "]")
        return 
    }
  
    first := true
    // create sql statement dynamically based on number of IDs
    var args bytes.Buffer
    args.WriteString("(")
    for i, _ := range ids {
        if i > 0 { 
            args.WriteString(",")
        }
        args.WriteString("?")
    }
    args.WriteString(")")

    loc_table := "map_data"
    if isValidTableSuffix(tableSuffix) {
        loc_table += "_" + tableSuffix
    }

    sql := fmt.Sprintf("SELECT id,x,y,r FROM " + loc_table + " WHERE id IN %s LIMIT %d",args.String(),len(ids))

    // create interface of arguments for statement
    hIdsInt := make([]interface{},len(ids))
    for i, id := range ids {
        hIdsInt[i] = interface{}(id)
    }
    
    // Execute statement
    stmt := h.papers.StatementBegin(sql,hIdsInt...)
    if stmt != nil {
        stmt.BindResult(&resId,&x,&y,&r)
        for {
            eof, err := stmt.Fetch()
            if err != nil {
                fmt.Println("MySQL statement error;", err)
                break
            } else if eof { break }
            if first { first = false } else { fmt.Fprintf(rw, ",") }
            // write directly to output!
            fmt.Fprintf(rw, "{\"id\":%d,\"x\":%d,\"y\":%d,\"r\":%d}",resId, x, y,r)
        }
        err := stmt.FreeResult()
        if err != nil {
            fmt.Println("MySQL statement error;", err)
        }
    }
    h.papers.StatementEnd(stmt) 

    fmt.Fprintf(rw, "]")
}

func (h *MyHTTPHandler) MapPaperIdAtLocation(x, y float64, tableSuffix string, rw http.ResponseWriter) {
    
    var id uint64
    var resr uint
    var resx, resy int 

    loc_table := "map_data"
    if isValidTableSuffix(tableSuffix) {
        loc_table += "_" + tableSuffix
    }

    // TODO
    // Current implentation is slow (order n)
    // use quad tree: order log n
    // OR try using MySQL spatial extensions

    sql := "SELECT id,x,y,r FROM " + loc_table + " WHERE sqrt(pow(x - ?,2) + pow(y - ?,2)) - r <= 0 LIMIT 1"

    stmt := h.papers.StatementBegin(sql,x,y)
    if !h.papers.StatementBindSingleRow(stmt,&id,&resx,&resy,&resr) {
        return
    }
    
    fmt.Fprintf(rw, "{\"id\":%d,\"x\":%d,\"y\":%d,\"r\":%d}",id,resx,resy,resr)
}

func (h *MyHTTPHandler) MapReferencesFromArxivId(arxivString string, tableSuffix string, rw http.ResponseWriter) {
    // check for valid characters in arxiv string
    for _, r := range arxivString {
        if !(unicode.IsLetter(r) || unicode.IsNumber(r) || r == '-' || r == '/' || r == '.') {
            // invalid character
            return
        }
    }

    // query the paper and its refs and cites
    paper := h.papers.QueryPaper(0, arxivString)
    h.papers.QueryRefs(paper, false)
    h.papers.QueryLocations(paper,tableSuffix)

    // check the paper exists
    if paper == nil {
        return
    }

    // print the json output
    fmt.Fprintf(rw, "{\"papr\":[{\"id\":%d,", paper.id)
    if paper.location != nil {
        fmt.Fprintf(rw, "\"x\":%d,\"y\":%d,\"r\":%d,", paper.location.x,paper.location.y,paper.location.r)
    }
    // print all references that have a location
    printJSONAllRefs(rw, paper,true)
    fmt.Fprintf(rw, "}]}")
}

func (h *MyHTTPHandler) MapCitationsFromArxivId(arxivString string, tableSuffix string, rw http.ResponseWriter) {
    // check for valid characters in arxiv string
    for _, r := range arxivString {
        if !(unicode.IsLetter(r) || unicode.IsNumber(r) || r == '-' || r == '/' || r == '.') {
            // invalid character
            return
        }
    }

    // query the paper and its refs and cites
    paper := h.papers.QueryPaper(0, arxivString)
    h.papers.QueryCites(paper, false)
    h.papers.QueryLocations(paper,tableSuffix)

    // check the paper exists
    if paper == nil {
        return
    }

    // print the json output
    fmt.Fprintf(rw, "{\"papr\":[{\"id\":%d,", paper.id)
    if paper.location != nil {
        fmt.Fprintf(rw, "\"x\":%d,\"y\":%d,\"r\":%d,", paper.location.x,paper.location.y,paper.location.r)
    }
    // print all cites that have a location
    printJSONAllCites(rw, paper, 0,true)
    fmt.Fprintf(rw, "}]}")

}

func (h *MyHTTPHandler) GetDateMapsVersion(rw http.ResponseWriter) (success bool) {
    
    success = false

    // perform query
    if !h.papers.QueryBegin("SELECT daysAgo,id FROM datebdry WHERE daysAgo = 0") {
        return
    }

    defer h.papers.QueryEnd()

    // get result set  
    result, err := h.papers.db.UseResult()
    if err != nil {
        fmt.Println("MySQL use result error;", err)
        return
    }

    fmt.Fprintf(rw, "{\"v\":\"%s\",",VERSION_PSCPMAP)
    // get each row from the result and create the JSON object
    numResults := 0
    for {
        // get the row
        row := result.FetchRow()
        if row == nil {
            break
        }

        // parse the row
        var ok bool
        var daysAgo uint64
        var id uint64
        if daysAgo, ok = row[0].(uint64); !ok { continue }
        if id, ok = row[1].(uint64); !ok {
            id = 0
        }

        // print the result
        if numResults > 0 {
            fmt.Fprintf(rw, ",")
        }
        fmt.Fprintf(rw, "\"d%d\":%d", daysAgo, id)
        numResults += 1
    }
    fmt.Fprintf(rw, "}")
    success = true
    return
}

/****************************************************************/

func printJSONMetaInfo(w io.Writer, paper *Paper) {
    var err error
    var authorsJSON, titleJSON []byte

    authorsJSON, err = json.Marshal(paper.authors)
    if err != nil {
        log.Printf("ERROR: Author string failed for %d, error: %s\n",paper.id,err)
        authorsJSON = []byte("\"\"")
    }
    titleJSON, err = json.Marshal(paper.title)
    if err != nil {
        log.Printf("ERROR: Title string failed for %d, error: %s\n",paper.id,err)
        titleJSON = []byte("\"\"")
    }

    //fmt.Fprintf(w, "{\"id\":%d,\"auth\":%s,\"titl\":%s,\"nc\":%d,\"dnc1\":%d,\"dnc5\":%d", paper.id, authorsJSON, titleJSON, paper.numCites, paper.dNumCites1, paper.dNumCites5)
    fmt.Fprintf(w, "\"auth\":%s,\"titl\":%s,\"nr\":%d,\"nc\":%d,\"dnc1\":%d,\"dnc5\":%d", authorsJSON, titleJSON, paper.numRefs, paper.numCites, paper.dNumCites1, paper.dNumCites5)
    if len(paper.arxiv) > 0 {
        fmt.Fprintf(w, ",\"arxv\":\"%s\"", paper.arxiv)
        if len(paper.allcats) > 0 {
            fmt.Fprintf(w, ",\"cats\":\"%s\"", paper.allcats)
        }
    }
    if paper.inspire > 0 {
        fmt.Fprintf(w, ",\"insp\":%d", paper.inspire)
    }
    if len(paper.publJSON) > 0 {
        fmt.Fprintf(w, ",\"publ\":%s", paper.publJSON)
    }
}

// Returns possibly updated meta info only (this is called on a date change)
func printJSONUpdateMetaInfo(w io.Writer, paper *Paper) {
    //fmt.Fprintf(w, "{\"id\":%d,\"nc\":%d,\"dnc1\":%d,\"dnc5\":%d", paper.id, paper.numCites, paper.dNumCites1, paper.dNumCites5)
    fmt.Fprintf(w, "\"nc\":%d,\"dnc1\":%d,\"dnc5\":%d", paper.numCites, paper.dNumCites1, paper.dNumCites5)
    if len(paper.publJSON) > 0 {
        fmt.Fprintf(w, ",\"publ\":%s", paper.publJSON)
    }
}

func printJSONRelevantRefs(w io.Writer, paper *Paper, paperList []*Paper) {
    fmt.Fprintf(w, "\"allr\":false,\"ref\":[")
    first := true
    for _, link := range paper.refs {
        // only return links that point to other papers in this profile
        for _, paper2 := range paperList {
            if link.pastId == paper2.id {
                if first {
                    first = false
                } else {
                    fmt.Fprintf(w, ",")
                }
                printJSONLinkPastInfo(w, link)
                break
            }
        }
    }
    fmt.Fprintf(w, "]")
}

func printJSONLinkPastInfo(w io.Writer, link *Link) {
    fmt.Fprintf(w, "{\"id\":%d,\"o\":%d,\"f\":%d,\"nc\":%d", link.pastId, link.refOrder, link.refFreq, link.pastCited)
    if link.location != nil {
        fmt.Fprintf(w, ",\"x\":%d,\"y\":%d,\"r\":%d", link.location.x, link.location.y, link.location.r)
    }
    fmt.Fprintf(w, "}")
}

func printJSONLinkFutureInfo(w io.Writer, link *Link) {
    fmt.Fprintf(w, "{\"id\":%d,\"o\":%d,\"f\":%d,\"nc\":%d", link.futureId, link.refOrder, link.refFreq, link.futureCited)
    if link.location != nil {
        fmt.Fprintf(w, ",\"x\":%d,\"y\":%d,\"r\":%d", link.location.x, link.location.y, link.location.r)
    }
    fmt.Fprintf(w, "}")
}

func printJSONAllRefs(w io.Writer, paper *Paper, ignoreUnmappedIds bool) {
    fmt.Fprintf(w, "\"allr\":true,\"ref\":[")
    // output the refs (future -> past)
    first := true
    for _, link := range paper.refs {
        if !ignoreUnmappedIds || link.location != nil {
            if !first {
                fmt.Fprintf(w, ",")
            }
            printJSONLinkPastInfo(w, link)
            first = false
        }
    }
    fmt.Fprintf(w, "]")
}

func printJSONAllCites(w io.Writer, paper *Paper, dateBoundary uint, ignoreUnmappedIds bool) {
    fmt.Fprintf(w, "\"allc\":true,\"cite\":[")
    first := true
    for _, link := range paper.cites {
        if link.futureId < dateBoundary  {
            continue
        }
        if !ignoreUnmappedIds || link.location != nil{
            if !first {
                fmt.Fprintf(w, ",")
            }
            printJSONLinkFutureInfo(w, link)
            first = false
        }
    }

    fmt.Fprintf(w, "]")
}

func printJSONNewCites(w io.Writer, paper *Paper, dateBoundary uint, ignoreUnmappedIds bool) {
    fmt.Fprintf(w, "\"allnc\":true,\"cite\":[")

    // output the cites (past -> future)
    first := true
    for _, link := range paper.cites {
        if link.futureId < dateBoundary  {
            continue
        }
        if !ignoreUnmappedIds || link.location != nil  {
            if !first {
                fmt.Fprintf(w, ",")
            }
            printJSONLinkFutureInfo(w, link)
            first = false
        }
    }

    fmt.Fprintf(w, "]")
}
