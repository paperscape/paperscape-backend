package main

import (
    "io"
    "flag"
    "os"
    "bufio"
    "fmt"
    "net"
    "net/http"
    "net/http/fcgi"
    "strconv"
    "unicode"
    "encoding/json"
    "text/scanner"
    "GoMySQL"
    "runtime"
    "bytes"
    "time"
	"math/rand"
	"crypto/sha1"
    //"xiwi"
)


var flagDB = flag.String("db", "localhost", "MySQL database to connect to")
var flagPciteTable = flag.String("table", "pcite", "MySQL database table to get pcite data from")
var flagFastCGIAddr = flag.String("fcgi", "", "listening on given address using FastCGI protocol (eg -fcgi :9100)")
var flagHTTPAddr = flag.String("http", "", "listening on given address using HTTP protocol (eg -http :8089)")
var flagTestQueryId = flag.Uint("test-id", 0, "run a test query with id")
var flagTestQueryArxiv = flag.String("test-arxiv", "", "run a test query with arxiv")
var flagMetaBaseDir = flag.String("meta", "", "Base directory for meta file data (abstracts etc.)")

func main() {
    // parse command line options
    flag.Parse()

    // auto-detect some command line arguments that are not given
    ondpg := false
    if  _, err := os.Stat("/opt/pscp/arXiv/meta"); err != nil {
        ondpg = true
    }

    if len(*flagMetaBaseDir) == 0 {
        if ondpg {
            *flagMetaBaseDir = "/opt/pscp/meta"
        } else {
            *flagMetaBaseDir = "/opt/pscp/arXiv/meta"
        }
    }

    // connect to MySQL database
    //db, err := mysql.DialUnix("/tmp/mysql.sock", "hidden", "hidden", "xiwi")
    db, err := mysql.DialTCP(*flagDB, "hidden", "hidden", "xiwi")
    if err != nil {
        fmt.Println("cannot connect to database;", err)
        return
    }
    fmt.Println("connect to database", *flagDB)
    defer db.Close()

    // create papers database
    papers := NewPapersEnv(db)

    // check if we want to run a test query
    if *flagTestQueryId != 0 || *flagTestQueryArxiv != "" {
        p := papers.QueryPaper(*flagTestQueryId, *flagTestQueryArxiv)
        if p == nil {
            fmt.Printf("could not find paper %d %s\n", *flagTestQueryId, *flagTestQueryArxiv)
            return
        }
        papers.QueryRefs(p, true)
        papers.QueryCites(p, true)
        fmt.Printf("%d\n%s\n%s\n%s\n", p.id, p.arxiv, p.title, p.authors)
        fmt.Printf("refs:\n")
        for _, link := range p.refs {
            fmt.Printf("  - %d ([%d] x%d)\n    %s\n    %s\n    %s\n", link.pastPaper.id, link.refOrder, link.refFreq, link.pastPaper.arxiv, link.pastPaper.title, link.pastPaper.authors)
        }
        fmt.Printf("cites:\n")
        for _, link := range p.cites {
            fmt.Printf("  - %d ([%d] x%d)\n    %s\n    %s\n    %s\n", link.futurePaper.id, link.refOrder, link.refFreq, link.futurePaper.arxiv, link.futurePaper.title, link.futurePaper.authors)
        }
        fmt.Printf("abstract:\n%s\n", papers.GetAbstract(p.id))
    }

    // serve requests using FastCGI if wanted
    if len(*flagFastCGIAddr) > 0 {
        serveFastCGI(*flagFastCGIAddr, papers)
    }

    // serve requests using HTTP if wanted
    if len(*flagHTTPAddr) > 0 {
        serveHTTP(*flagHTTPAddr, papers)
    }
}

/****************************************************************/

type Link struct {
    pastId         uint     // id of the earlier paper
    futureId       uint     // id of the later paper
    pastPaper      *Paper   // pointer to the earlier paper, can be nil
    futurePaper    *Paper   // pointer to the later paper, can be nil
    refOrder       uint     // ordering of this reference made by future to past
    refFreq        uint     // number of in-text references made by future to past
    pastCited      uint      // number of times past paper cited
    futureCited    uint      // number of times future paper cited
    //tredWeightFull float64  // transitively reduced weight, full
    //tredWeightNorm float64  // transitively reduced weight, normalised
}

type Paper struct {
    id         uint     // unique id
    arxiv      string   // arxiv id, simplified
    maincat    string   // main arxiv category
    authors    string   // authors
    title      string   // title
    journal    string   // journal string
    doiJSON    string   // DOI in JSON format
    refs       []*Link  // makes references to
    cites      []*Link  // cited by 
    numCites   uint     // number of times cited
    xPos       int      // for loaded profile
    notes      string   // for loaded profile
    layers     []string // for loaded profile
    tags       []string // for loaded profile
    newTags    []string // for loaded profile
}

type Tag struct {
    name       string   // unique name
	blobbed    bool		// whether tag is blobbed
	blobCol	   int      // index of blob colour array
	starred    bool		// whether tag is starred
}

type PapersEnv struct {
    db *mysql.Client
}

func NewPapersEnv(db *mysql.Client) *PapersEnv {
    papers := new(PapersEnv)
    papers.db = db
    db.Reconnect = true
    return papers
}

func (papers *PapersEnv) QueryBegin(query string) bool {
    // perform query
    //fmt.Println("waiting for lock")
    papers.db.Lock()
    //fmt.Println("query:", query)
    err := papers.db.Query(query)

    // error
    if err != nil {
        fmt.Println("MySQL query error;", err)
        return false
    }

    return true
}

func (papers *PapersEnv) QueryEnd() {
    papers.db.FreeResult()
    //fmt.Println("query done, unlocking")
    papers.db.Unlock()
}

func (papers *PapersEnv) QuerySingleRow(query string) mysql.Row {
    // perform query
    if !papers.QueryBegin(query) {
        return nil
    }

    // get result set  
    result, err := papers.db.StoreResult()
    if err != nil {
        fmt.Println("MySQL store result error;", err)
        return nil
    }

    // check if there are any results
    if result.RowCount() == 0 {
        return nil
    }

    // should be only 1 result
    if result.RowCount() != 1 {
        fmt.Println("MySQL multiple results; result count =", result.RowCount())
        return nil
    }

    // get the row
    row := result.FetchRow()
    if row == nil {
        return nil
    }

    return row
}

func (papers *PapersEnv) QueryFull(query string) bool {
    if !papers.QueryBegin(query) {
        // need to end the query because it unlocks the DB for another thread to use
        papers.QueryEnd()
        return false
    }
    papers.QueryEnd()
    return true
}

func (papers *PapersEnv) QueryPaper(id uint, arxiv string) *Paper {
    // perform query
    var query string
    if id != 0 {
        query = fmt.Sprintf("SELECT id,arxiv,maincat,authors,title,jname,jyear,jvol,jpage,doi FROM meta_data WHERE id = %d", id)
    } else if len(arxiv) > 0 {
        // security issue: should make sure arxiv string is sanitised
        query = fmt.Sprintf("SELECT id,arxiv,maincat,authors,title,jname,jyear,jvol,jpage,doi FROM meta_data WHERE arxiv = '%s'", arxiv)
    } else {
        return nil
    }
    row := papers.QuerySingleRow(query)

    papers.QueryEnd()

    if row == nil { return nil }

    // get the fields
    paper := new(Paper)
    if idNum, ok := row[0].(uint64); !ok {
        return nil
    } else {
        paper.id = uint(idNum)
    }
    var ok bool
    if paper.arxiv, ok = row[1].(string); !ok { return nil }
    if paper.maincat, ok = row[2].(string); !ok { return nil }
    if row[3] == nil {
        paper.authors = "(unknown authors)"
    } else if au, ok := row[3].([]byte); !ok {
        fmt.Printf("ERROR: cannot get authors for id=%d; %v\n", paper.id, row[3])
        return nil
    } else {
        paper.authors = string(au)
    }
    if row[4] == nil {
        paper.authors = "(unknown title)"
    } else if title, ok := row[4].(string); !ok {
        fmt.Printf("ERROR: cannot get title for id=%d; %v\n", paper.id, row[4])
        return nil
    } else {
        paper.title = title
    }
    if row[5] == nil {
    } else {
        if year, ok := row[6].(int64); ok && year != 0 {
            if row[7] == nil {
                paper.journal = fmt.Sprintf("%v/%d//", row[5], year)
            } else if row[8] == nil {
                paper.journal = fmt.Sprintf("%v/%d/%v/", row[5], year, row[7])
            } else {
                paper.journal = fmt.Sprintf("%v/%d/%v/%v", row[5], year, row[7], row[8])
            }
        }
    }
    if row[9] == nil {
    } else if doi, ok := row[9].(string); !ok {
        fmt.Printf("ERROR: cannot get doi for id=%d; %v\n", paper.id, row[9])
    } else {
        doi, _ := json.Marshal(doi)
        paper.doiJSON = string(doi)
    }

    //// Get number of times cited
    query = fmt.Sprintf("SELECT numCites FROM %s WHERE id = %d", *flagPciteTable, paper.id)
    row2 := papers.QuerySingleRow(query)

    if row2 != nil {
        if numCites, ok := row2[0].(uint64); !ok {
            paper.numCites = 0
        } else {
            paper.numCites = uint(numCites)
        }
    }

    papers.QueryEnd()

    return paper
}

func FindNextComma(str string, idx int) (int, int) {
    for ; ; idx++ {
        if idx == len(str) || str[idx] == ';' {
            return idx, idx
        } else if str[idx] == ',' {
            return idx + 1, idx
        }
    }
    return idx, idx
}

// parse a reference/citation string for the given paper
// adds links to the paper
// doesn't lookup the new papers for meta data
// returns true if okay, false if error
func getLE16(blob []byte, i int) uint {
    return uint(blob[i]) | (uint(blob[i + 1]) << 8)
}
func getLE32(blob []byte, i int) uint {
    return uint(blob[i]) | (uint(blob[i + 1]) << 8) | (uint(blob[i + 2]) << 16) | (uint(blob[i + 3]) << 24)
}
func ParseRefsCitesString(paper *Paper, blob []byte, isRefStr bool) bool {
    if len(blob) == 0 {
        // nothing to do, that's okay
        return true
    }

    for i := 0; i < len(blob); i += 10 {
        refId := getLE32(blob, i)
        refOrder := getLE16(blob, i + 4)
        refFreq := getLE16(blob, i + 6)
        numCites := getLE16(blob, i + 8)
        // make link and add to list in paper
        if isRefStr {
            link := &Link{uint(refId), paper.id, nil, paper, uint(refOrder), uint(refFreq), uint(numCites), paper.numCites}
            paper.refs = append(paper.refs, link)
        } else {
            link := &Link{paper.id, uint(refId), paper, nil, uint(refOrder), uint(refFreq), paper.numCites, uint(numCites)}
            paper.cites = append(paper.cites, link)
        }
    }

    return true
}
/*
func ParseRefsCitesString(paper *Paper, str string, isRefStr bool) bool {
    if len(str) == 0 {
        // nothing to do, that's okay
        return true
    }

    for i := 0; i <= len(str); i++ {
        var idx_comma1, idx_comma2, idx_comma3 int
        idx_id := i
        i, idx_comma1 = FindNextComma(str, i)
        i, idx_comma2 = FindNextComma(str, i)
        i, idx_comma3 = FindNextComma(str, i)
        // scan to end of field
        for ; i < len(str) && str[i] != ',' && str[i] != ';'; i++ {
        }
        if (i == len(str) || str[i] == ';') && idx_id < idx_comma1 && idx_comma1+1 < idx_comma2 && idx_comma2+1 < idx_comma3 && idx_comma3+1 < i {
            refId, _ := strconv.ParseUint(str[idx_id : idx_comma1], 10, 0)
            refOrder, _ := strconv.ParseUint(str[idx_comma1+1 : idx_comma2], 10, 0)
            refFreq, _ := strconv.ParseUint(str[idx_comma2+1 : idx_comma3], 10, 0)
            numCites, _ := strconv.ParseUint(str[idx_comma3+1 : i], 10, 0)
            // make link and add to list in paper
            if isRefStr {
                link := &Link{uint(refId), paper.id, nil, paper, uint(refOrder), uint(refFreq), uint(numCites), paper.numCites}
                paper.refs = append(paper.refs, link)
            } else {
                link := &Link{paper.id, uint(refId), paper, nil, uint(refOrder), uint(refFreq), paper.numCites, uint(numCites)}
                paper.cites = append(paper.cites, link)
            }
        } else {
            fmt.Printf("malformed reference string at i=%d:%s\n", i, str)
            return false
        }
    }

    return true
}
*/

func (papers *PapersEnv) QueryRefs(paper *Paper, queryRefsMeta bool) {
    if paper == nil { return }

    // check if refs already exist
    if len(paper.refs) != 0 { return }

    // perform query
    query := fmt.Sprintf("SELECT refs FROM %s WHERE id = %d", *flagPciteTable, paper.id)
    row := papers.QuerySingleRow(query)
    if row == nil { papers.QueryEnd(); return }

    var ok bool
    var refStr []byte
    if refStr, ok = row[0].([]byte); !ok { papers.QueryEnd(); return }

    // parse the ref string, creating links
    ParseRefsCitesString(paper, refStr, true)

    papers.QueryEnd()

    // if requested, also query the meta data of the ref links
    if queryRefsMeta {
        for _, link := range paper.refs {
            if link.pastPaper == nil {
                link.pastPaper = papers.QueryPaper(link.pastId, "")
            }
        }
    }
}


func (papers *PapersEnv) QueryCites(paper *Paper, queryCitesMeta bool) {
    if paper == nil { return }

    // check if refs already exist
    if len(paper.cites) != 0 { return }

    // perform query
    query := fmt.Sprintf("SELECT cites FROM %s WHERE id = %d", *flagPciteTable, paper.id)
    row := papers.QuerySingleRow(query)
    if row == nil { papers.QueryEnd(); return }

    var ok bool
    var citeStr []byte
    if citeStr, ok = row[0].([]byte); !ok { papers.QueryEnd(); return }

    // parse the cite string, creating links
    ParseRefsCitesString(paper, citeStr, false)

    papers.QueryEnd()

    // if requested, also query the meta data of the ref links
    if queryCitesMeta {
        for _, link := range paper.cites {
            if link.futurePaper == nil {
                link.futurePaper = papers.QueryPaper(link.futureId, "")
            }
        }
    }
}

/****************************************************************/


func (papers *PapersEnv) GetAbstract(paperId uint) string {
    // get the arxiv name for this id
    query := fmt.Sprintf("SELECT arxiv FROM meta_data WHERE id = %d", paperId)
    row := papers.QuerySingleRow(query)
    if row == nil { papers.QueryEnd(); return "(no abstract)" }
    arxiv, ok := row[0].(string)
    papers.QueryEnd()
    if !ok { return "(no abstract)" }

    // work out the meta filename for this arxiv
    var filename string
    if len(arxiv) == 9 && arxiv[4] == '.' {
        filename = fmt.Sprintf("%s/%sxx/%s/%s.xml", *flagMetaBaseDir, arxiv[:2], arxiv[:4], arxiv)
    } else if len(arxiv) >= 10 {
        l := len(arxiv)
        filename = fmt.Sprintf("%s/%sxx/%s/%s%s.xml", *flagMetaBaseDir, arxiv[l - 7 : l - 5], arxiv[l - 7 : l - 3], arxiv[:l - 8], arxiv[l - 7:])
    } else {
        return "(no abstract)"
    }

    // open the meta file
    file, _ := os.Open(filename)
    if file == nil {
        return "(no abstract)"
    }
    defer file.Close()
    reader := bufio.NewReader(file)

    // parse the meta file, looking for the <abstract> tag
    for {
        r, _, err := reader.ReadRune()
        if err != nil {
            break
        }
        if r == '<' {
            tag, err := reader.ReadString('>')
            if err == nil {
                if tag == "abstract>" {
                    // found the tag, now get the abstract contents
                    var abs bytes.Buffer
                    firstNonSpace := false
                    needSpace := false
                    for {
                        r, _, err := reader.ReadRune()
                        if err != nil || r == '<' {
                            break
                        }
                        if unicode.IsSpace(r) {
                            needSpace = firstNonSpace
                        } else {
                            if needSpace {
                                abs.WriteRune(' ')
                                needSpace = false
                            }
                            firstNonSpace = true
                            abs.WriteRune(r)
                        }
                    }
                    // return the abstract
                    return abs.String()
                }
            }
        }
    }

    return "(no abstract)"
}

/****************************************************************/

// Converts papers list into string and stores this in userdata table's 'papers' field
func (h *MyHTTPHandler) paperListToDatabase (username string, paperList []*Paper) bool {

	// This SHOULD be identical to JS code in kea i.e. it should be parseable
	// by the paperListFromDatabase code below
	w := new(bytes.Buffer)
	fmt.Fprintf(w,"v:2"); // PAPERS VERSION 2
	for _, paper := range paperList {
		fmt.Fprintf(w,"(%d,%d,%s,l[",paper.id,paper.xPos,paper.notes);
		for i, layer := range paper.layers {
			if i > 0 { fmt.Fprintf(w,","); }
			fmt.Fprintf(w,"%s",layer);
		}
		fmt.Fprintf(w,"],t[");
		for i, tag := range paper.tags {
			if i > 0 { fmt.Fprintf(w,","); }
			fmt.Fprintf(w,"%s",tag);
		}
		fmt.Fprintf(w,"],n[");
		for i, newTag := range paper.newTags {
			if i > 0 { fmt.Fprintf(w,","); }
			fmt.Fprintf(w,"%s",newTag);
		}
		fmt.Fprintf(w,"])");
	}

	query := fmt.Sprintf("UPDATE userdata SET papers = '%s' WHERE username = '%s'", w.String(), username)
    if h.papers.QueryFull(query) {
		return true;
	}
	return false
}


// Returns a list of papers stored in userdata string field
func (h *MyHTTPHandler) paperListFromDatabase (papers []byte) []*Paper {

    var paperList []*Paper
    var s scanner.Scanner
    s.Init(bytes.NewReader(papers))
    s.Mode = scanner.ScanInts | scanner.ScanStrings | scanner.ScanIdents
    tok := s.Scan()
	papersVersion := 0 // there is no zero version

	// Firstly discover format of saved data
	if tok == '(' {
		papersVersion = 1;
	} else if tok == scanner.Ident && s.TokenText() == "v" {
		if tok = s.Scan(); tok == ':' {
			if tok = s.Scan(); tok == scanner.Int {
				version, _ := strconv.ParseUint(s.TokenText(), 10, 0)
				papersVersion = int(version)
				tok = s.Scan()
			}
		}
	}

	if papersVersion == 1 {
		// VERSION 1 (deprecated)
		for tok != scanner.EOF {
			if tok != '(' { break }
			if tok = s.Scan(); tok != scanner.Int { break }
			paperId, _ := strconv.ParseUint(s.TokenText(), 10, 0)
			if tok = s.Scan(); tok != ',' { break }
			tok = s.Scan()
			negate := false;
			if tok == '-' { negate = true; tok = s.Scan() }
			if tok != scanner.Int { break }
			xPos, _ := strconv.ParseInt(s.TokenText(), 10, 0)
			if negate { xPos = -xPos }
			if tok = s.Scan(); tok != ',' { break }
			// pinned is obsolete, but we need to parse it for backwards compat
			if tok = s.Scan(); tok == scanner.Ident && (s.TokenText() == "pinned" || s.TokenText() == "unpinned") {
				if tok = s.Scan(); tok != ',' { break }
				tok = s.Scan()
			}
			if tok != scanner.String { break }
			notes := s.TokenText()
			var tags []string
			for tok = s.Scan(); tok == ','; tok = s.Scan() {
				if tok = s.Scan(); tok != scanner.String { break }
				tags = append(tags, s.TokenText())
			}
			if tok != ')' { break }
			paper := h.papers.QueryPaper(uint(paperId), "")
			h.papers.QueryRefs(paper, false)
			paper.xPos = int(xPos)
			paper.notes = notes
			paper.tags = tags
			tok = s.Scan()
			paperList = append(paperList, paper)
		}
	} else if papersVersion == 2 {
		// PAPERS VERSION 2
		for tok != scanner.EOF {
			if tok != '(' { break }
			if tok = s.Scan(); tok != scanner.Int { break }
			paperId, _ := strconv.ParseUint(s.TokenText(), 10, 0)
			if tok = s.Scan(); tok != ',' { break }
			tok = s.Scan()
			negate := false;
			if tok == '-' { negate = true; tok = s.Scan() }
			if tok != scanner.Int { break }
			xPos, _ := strconv.ParseInt(s.TokenText(), 10, 0)
			if negate { xPos = -xPos }
			if tok = s.Scan(); tok != ',' { break }
			if tok = s.Scan(); tok != scanner.String { break }
			notes := s.TokenText()
			if tok = s.Scan(); tok != ',' { break }
			if tok = s.Scan(); tok == scanner.Ident && s.TokenText() != "l" { break }
			var layers []string
			for tok = s.Scan(); tok == '[' || tok == ','; tok = s.Scan() {
				if tok = s.Scan(); tok != scanner.String { break }
				layers = append(layers, s.TokenText())
			}
			if tok != ']' { break }
			if tok = s.Scan(); tok != ',' { break }
			if tok = s.Scan(); tok == scanner.Ident && s.TokenText() != "t" { break }
			var tags []string
			for tok = s.Scan(); tok == '[' || tok == ','; tok = s.Scan() {
				if tok = s.Scan(); tok != scanner.String { break }
				tags = append(tags, s.TokenText())
			}
			if tok != ']' { break }
			if tok = s.Scan(); tok != ',' { break }
			if tok = s.Scan(); tok == scanner.Ident && s.TokenText() != "n" { break }
			var newTags []string
			for tok = s.Scan(); tok == '[' || tok == ','; tok = s.Scan() {
				if tok = s.Scan(); tok != scanner.String { break }
				newTags = append(newTags, s.TokenText())
			}
			if tok != ']' { break }
			if tok = s.Scan(); tok != ')' { break }
			paper := h.papers.QueryPaper(uint(paperId), "")
			h.papers.QueryRefs(paper, false)
			paper.xPos = int(xPos)
			paper.notes = notes
			paper.tags = tags
			paper.layers = layers
			paper.newTags = newTags
			tok = s.Scan()
			paperList = append(paperList, paper)
		}


	}

	return paperList
}

// Returns a list of tags stored in userdata string field
func (h *MyHTTPHandler) tagListFromDatabase (tags []byte) []*Tag {

    var tagList []*Tag
    var s scanner.Scanner
    s.Init(bytes.NewReader(tags)) // user scanner from above
    s.Mode = scanner.ScanInts | scanner.ScanStrings | scanner.ScanIdents
	tok := s.Scan()
	tagsVersion := 0 // there is no zero version

	// Firstly discover format of saved data
	if tok == scanner.Ident && s.TokenText() == "v" {
		if tok = s.Scan(); tok == ':' {
			if tok = s.Scan(); tok == scanner.Int {
				version, _ := strconv.ParseUint(s.TokenText(), 10, 0)
				tagsVersion = int(version)
				tok = s.Scan()
			}
		}
	}

	if tagsVersion == 1 {
		// TAGS VERSION 1
		for tok != scanner.EOF {
			if tok != '(' { break }
			tag := new(Tag)
			// tag name
			if tok = s.Scan(); tok != scanner.String { break }
			tag.name = s.TokenText()
			if tok = s.Scan(); tok != ',' { break }
			// tag starred?
			tag.starred = true
			if tok = s.Scan(); tok == scanner.Ident && s.TokenText() != "s" { break }
			if tok = s.Scan(); tok == '!' {
				tag.starred = false
				tok = s.Scan()
			}
			if tok != ',' { break }
			// tag blobbed?
			tag.blobbed = true
			if tok = s.Scan(); tok == scanner.Ident && s.TokenText() != "b" { break }
			if tok = s.Scan(); tok == '!' {
				tag.blobbed = false
				tok = s.Scan()
			}
			//tag.blobCol = int(blobCol)
			if tok != ')' { break }
			tok = s.Scan()
			tagList = append(tagList, tag)
		}
	}

	return tagList
}



/****************************************************************/

func serveFastCGI(listenAddr string, papers *PapersEnv) {
    laddr, er := net.ResolveTCPAddr("tcp", listenAddr)
    if er != nil {
        fmt.Println("ResolveTCPAddr error:", er)
        return
    }
    l, er := net.ListenTCP("tcp", laddr)
    if er != nil {
        fmt.Println("ListenTCP error:", er)
        return
    }
    h := &MyHTTPHandler{papers}

    fmt.Println("listening with FastCGI protocol on", laddr)

    er = fcgi.Serve(l, h)
    if er != nil {
        fmt.Println("FastCGI serve error:", er)
        return
    }
}

func serveHTTP(listenAddr string, papers *PapersEnv) {
    h := &MyHTTPHandler{papers}
    //http.Handle("/pull", h)
    http.Handle("/wombat", h)

    fmt.Println("listening with HTTP protocol on", listenAddr)

    err := http.ListenAndServe(listenAddr, nil)
    if err != nil {
        fmt.Println("ListenAndServe error: ", err)
        return
    }
}

type MyHTTPHandler struct {
    papers* PapersEnv
}

// a simple wrapper for http.ResponseWriter that counts number of bytes
type MyResponseWriter struct {
    rw http.ResponseWriter
    bytesWritten int
}

func (myrw *MyResponseWriter) Header() http.Header {
    return myrw.rw.Header()
}

func (myrw *MyResponseWriter) Write(data []byte) (int, error) {
    myrw.bytesWritten += len(data)
    return myrw.rw.Write(data)
}

func (myrw *MyResponseWriter) WriteHeader(val int) {
    myrw.rw.WriteHeader(val)
}

func (h *MyHTTPHandler) ServeHTTP(rwIn http.ResponseWriter, req *http.Request) {
    req.ParseForm()

    rw := &MyResponseWriter{rwIn, 0}

    if req.Form["callback"] != nil {
        // construct a JSON object to return
        rw.Header().Set("Content-Type", "application/json")
        callback := req.Form["callback"][0]
        fmt.Fprintf(rw, "%s({\"r\":", callback)
        resultBytesStart := rw.bytesWritten

		if req.Form["profileChallenge"] != nil {
			// authenticate request (send user a new "challenge")
			giveSalt := false
			// give user their salt once, so they can salt passwords in this session
			if req.Form["giveSalt"] != nil {
				giveSalt = true
			}
			h.ProfileChallenge(req.Form["profileChallenge"][0], giveSalt, rw)
		} else if req.Form["profileLogin"] != nil && req.Form["passHash"] != nil {
            // login request
            h.ProfileLogin(req.Form["profileLogin"][0], req.Form["passHash"][0], rw)
        } else if req.Form["profileSync"] != nil && req.Form["passHash"] != nil {
            // sync request
            if req.Form["papers"] != nil && req.Form["tags"] != nil {
                h.ProfileSync(req.Form["profileSync"][0], req.Form["passHash"][0], req.Form["papers"][0], req.Form["tags"][0], rw)
            }
        } else if req.Form["profileChangePassword"] != nil && req.Form["passHash"] != nil {
            // change password request
			// deliberate use of cryptic JSON names to fool robotic packet sniffers...
            if req.Form["payload"] != nil && req.Form["sprinkle"] != nil {
                h.ProfileChangePassword(req.Form["profileChangePassword"][0], req.Form["passHash"][0], req.Form["payload"][0], req.Form["sprinkle"][0], rw)
            }
            /*
        } else if req.Form["lookupId"] != nil || req.Form["lookupArxiv"] != nil {
            // lookup details of specific paper

            // parse the request parameters
            var id uint = 0
            if req.Form["lookupId"] != nil {
                if idNum, er := strconv.ParseUint(req.Form["lookupId"][0], 10, 0); er == nil {
                    id = uint(idNum)
                }
            }
            var arxiv string = ""
            if req.Form["lookupArxiv"] != nil {
                arxiv = req.Form["lookupArxiv"][0]
            }
            refsOnly := req.Form["refsOnly"] != nil

            // do the lookup
            h.LookupPaper(id, arxiv, refsOnly, rw)

            */
        } else if req.Form["getMetaRefsCites"] != nil {
            // get the meta data, refs and citse for the given paper id
            var id uint = 0
            if idNum, er := strconv.ParseUint(req.Form["getMetaRefsCites"][0], 10, 0); er == nil {
                id = uint(idNum)
            }
            h.GetMetaRefsCites(id, rw)
        } else if req.Form["getMetas[]"] != nil {
            // get the meta data for given list of paper ids
            var ids []uint
            for _, strId := range req.Form["getMetas[]"] {
                if preId, er := strconv.ParseUint(strId, 10, 0); er == nil {
                    ids = append(ids, uint(preId))
                } else {
                    fmt.Printf("ERROR: can't convert id '%s'; skipping\n", strId)
                }
            }
            h.GetMetas(ids, rw)
        } else if req.Form["getRefsCites"] != nil {
            // get the references and citations for a given paper id
            var id uint = 0
            if preId, er := strconv.ParseUint(req.Form["getRefsCites"][0], 10, 0); er == nil {
                id = uint(preId)
            }
            h.GetRefsCites(id, rw)
        } else if req.Form["searchArxiv"] != nil {
            // search papers for arxiv number
            h.SearchArxiv(req.Form["searchArxiv"][0], rw)
        } else if req.Form["getAbstract"] != nil {
            // get the abstract for a paper
            var id uint = 0
            if idNum, er := strconv.ParseUint(req.Form["getAbstract"][0], 10, 0); er == nil {
                id = uint(idNum)
            }
            abs, _ := json.Marshal(h.papers.GetAbstract(id))
            fmt.Fprintf(rw, "%s", abs)
        } else if req.Form["searchAuthor"] != nil {
            // search papers for authors
            h.SearchPaperV2("authors", req.Form["searchAuthor"][0], rw)
        } else if req.Form["searchKeyword"] != nil {
            // search papers for keywords
            h.SearchPaperV2("title", req.Form["searchKeyword"][0], rw)
        } else {
            // unknown ajax request
        }

        if rw.bytesWritten == resultBytesStart {
            // if no result written, write the null object
            fmt.Fprintf(rw, "null")
        }

        // tail of JSON object
        fmt.Fprintf(rw, ",\"bC2S\":%d,\"bS2C\":%d})\n", int64(len(req.URL.String())) + req.ContentLength, rw.bytesWritten)
    } else if req.Method == "POST" {
        // POST verb

        // construct a JSON object to return
        rw.Header().Set("Access-Control-Allow-Origin", "*") // for cross domain POSTing; see https://developer.mozilla.org/en/http_access_control
        rw.Header().Set("Content-Type", "application/json")
        fmt.Fprintf(rw, "{\"r\":")
        resultBytesStart := rw.bytesWritten

        if req.Form["profileSync"] != nil {
            // sync request
            if req.Form["papers"] != nil && req.Form["tags"] != nil {
                h.ProfileSync(req.Form["profileSync"][0], req.Form["passHash"][0], req.Form["papers"][0], req.Form["tags"][0], rw)
            }
        } else {
            // unknown ajax request
        }

        if rw.bytesWritten == resultBytesStart {
            // if no result written, write the null object
            fmt.Fprintf(rw, "null")
        }

        // tail of JSON object
        fmt.Fprintf(rw, ",\"bC2S\":%d,\"bS2C\":%d}\n", int64(len(req.URL.String())) + req.ContentLength, rw.bytesWritten)
    } else {
        // unknown request, return html
        fmt.Fprintf(rw, "<html><head></head><body><p>Unknown request</p></body>\n")
    }

    fmt.Printf("[%s] %s -- %s %s (bytes: %d URL, %d content, %d replied)\n", time.Now().Format(time.RFC3339), req.RemoteAddr, req.Method, req.URL, len(req.URL.String()), req.ContentLength, rw.bytesWritten)

    runtime.GC()
}

func PrintJSONMetaInfo(w io.Writer, paper *Paper) {
    PrintJSONMetaInfoUsing(w, paper.id, paper.arxiv, paper.authors, paper.title, paper.numCites, paper.journal, paper.doiJSON)
}

func PrintJSONMetaInfoUsing(w io.Writer, id uint, arxiv string, authors string, title string, numCites uint, journal string, doiJSON string) {
    authorsJSON, _ := json.Marshal(authors)
    titleJSON, _ := json.Marshal(title)
    fmt.Fprintf(w, "{\"id\":%d,\"arxiv\":\"%s\",\"authors\":%s,\"title\":%s,\"numCites\":%d", id, arxiv, authorsJSON, titleJSON, numCites)
    if len(journal) > 0 {
        fmt.Fprintf(w, ",\"journal\":\"%s\"", journal)
    }
    if len(doiJSON) > 0 {
        fmt.Fprintf(w, ",\"doi\":%s", doiJSON)
    }
}

func PrintJSONContextInfo(w io.Writer, paper *Paper) {
	fmt.Fprintf(w, ",\"xPos\":%d,\"notes\":%s,", paper.xPos, paper.notes)
	fmt.Fprintf(w, "\"layers\":[")
	for j, layer := range paper.layers {
		if j > 0 {
			fmt.Fprintf(w, ",")
		}
		fmt.Fprintf(w, "%s", layer)
	}
	fmt.Fprintf(w, "],\"tags\":[")
	for j, tag := range paper.tags {
		if j > 0 {
			fmt.Fprintf(w, ",")
		}
		fmt.Fprintf(w, "%s", tag)
	}
	fmt.Fprintf(w, "],\"newTags\":[")
	for j, newTag := range paper.newTags {
		if j > 0 {
			fmt.Fprintf(w, ",")
		}
		fmt.Fprintf(w, "%s", newTag)
	}
	fmt.Fprintf(w, "]")
}

func PrintJSONRelevantRefs(w io.Writer, paper *Paper, paperList []*Paper) {
	fmt.Fprintf(w, ",\"allRefsCites\":false,\"refs\":[")
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
				PrintJSONLinkPastInfo(w, link)
				break
			}
		}
	}
	fmt.Fprintf(w, "]")
}

func PrintJSONLinkPastInfo(w io.Writer, link *Link) {
    fmt.Fprintf(w, "{\"id\":%d,\"refOrder\":%d,\"refFreq\":%d,\"numCites\":%d}", link.pastId, link.refOrder, link.refFreq, link.pastCited)
}

func PrintJSONLinkFutureInfo(w io.Writer, link *Link) {
    fmt.Fprintf(w, "{\"id\":%d,\"refOrder\":%d,\"refFreq\":%d,\"numCites\":%d}", link.futureId, link.refOrder, link.refFreq, link.futureCited)
}

func PrintJSONAllRefsCites(w io.Writer, paper *Paper) {
    fmt.Fprintf(w, "\"allRefsCites\":true,\"refs\":[")

    // output the refs (future -> past)
    for i, link := range paper.refs {
        if i > 0 {
            fmt.Fprintf(w, ",")
        }
        PrintJSONLinkPastInfo(w, link)
    }

    // output the cites (past -> future)
    fmt.Fprintf(w, "],\"cites\":[")
    for i, link := range paper.cites {
        if i > 0 {
            fmt.Fprintf(w, ",")
        }
        PrintJSONLinkFutureInfo(w, link)
    }

    fmt.Fprintf(w, "]")
}

func (h *MyHTTPHandler) ProfileChallenge(username string, giveSalt bool, rw http.ResponseWriter) {

	// check username exists and get the 'salt'
	var salt uint64
    query := fmt.Sprintf("SELECT salt FROM userdata WHERE username = '%s'", username)
    row := h.papers.QuerySingleRow(query)
	h.papers.QueryEnd()
    if row == nil {
        // unknown username
		fmt.Printf("ERROR: challenging '%s' - no such user\n", username)
		fmt.Fprintf(rw, "false")
		return
	} else if giveSalt {
        var ok bool
		if salt, ok = row[0].(uint64); !ok {
			fmt.Printf("ERROR: challenging '%s' - salt\n", username)
			fmt.Fprintf(rw, "false")
			return
		}
	}

	// generate random "challenge" code
	challenge := rand.Int63();

	// store new challenge code in user database entry
    query = fmt.Sprintf("UPDATE userdata SET challenge = '%d' WHERE username = '%s'", challenge, username)
    if !h.papers.QueryFull(query) {
		fmt.Printf("ERROR: challenging '%s' - couldn't get salt\n", username)
		fmt.Fprintf(rw, "false")
		return
    }

	// return challenge code
	if giveSalt {
		fmt.Fprintf(rw, "{\"username\":\"%s\",\"challenge\":\"%d\",\"salt\":\"%d\"}", username, challenge, salt)
	} else {
		fmt.Fprintf(rw, "{\"username\":\"%s\",\"challenge\":\"%d\"}", username, challenge)
	}
}

func (h *MyHTTPHandler) ProfileAuthenticate(username string, passhash string) (success bool) {
	success = false

	// Check for valid username and get the user challenge and hash
	var challenge uint64
    var userhash string = ""
	query := fmt.Sprintf("SELECT challenge,userhash FROM userdata WHERE username = '%s'", username)
    row := h.papers.QuerySingleRow(query)
    if row == nil {
        h.papers.QueryEnd()
		fmt.Printf("ERROR: authenticating '%s' - no such user\n", username)
		return
	} else {
        var ok bool
		proceed := true
		if challenge, ok = row[0].(uint64); !ok { proceed = false }
		if userhash, ok = row[1].(string); !ok { proceed = false }
		h.papers.QueryEnd()
		if !proceed || userhash == ""  {
			fmt.Printf("ERROR: '%s', '%d'\n", userhash,challenge)
			fmt.Printf("ERROR: authenticating '%s' - challenge,hash error\n", username)
			return
		}
	}

	// Check the passhash!
	hash := sha1.New()
	io.WriteString(hash, fmt.Sprintf("%s%d", userhash, challenge))
	tryhash := fmt.Sprintf("%x",hash.Sum(nil))
	if passhash != tryhash {
		fmt.Printf("ERROR: authenticating '%s' - invalid password:  %s vs %s\n", username, passhash, tryhash)
		return
	}

	// we're THROUGH!!
	fmt.Printf("Succesfully authenticated user '%s'\n",username)
	success = true
	return
}


func (h *MyHTTPHandler) ProfileLogin(username string, passhash string, rw http.ResponseWriter) {

	if !h.ProfileAuthenticate(username,passhash) {
		return
	}

    // TODO security issue, make sure username is sanitised
	query := fmt.Sprintf("SELECT papers,tags,newpapers FROM userdata WHERE username = '%s'", username)
	row := h.papers.QuerySingleRow(query)

    var papers,tags,newpapers []byte

    if row == nil {
        h.papers.QueryEnd()
		return
    } else {
        var ok bool
        papers, ok = row[0].([]byte)
        if !ok { papers = nil }
        tags, ok = row[1].([]byte)
        if !ok { tags = nil }
        newpapers, ok = row[2].([]byte)
        if !ok { newpapers = nil }
        h.papers.QueryEnd()
    }

	/* PAPERS */
	/**********/

    // build a list of PAPERS and their metadata for this profile 
	paperList := h.paperListFromDatabase(papers)
    fmt.Printf("for user %s, read %d papers\n", username, len(paperList))

	// and check for new papers that we don't already have
	newPaperList := h.paperListFromDatabase(newpapers)
    fmt.Printf("for user %s, read %d new papers\n", username, len(newPaperList))

	// make one super list of unique papers
	// if newPaperList has duplicates (it shouldn't), takes the first
	listModified := false
	for _, newPaper := range newPaperList {
		exists := false
		for _, paper := range paperList {
			if newPaper.id == paper.id {
				exists = true
				break
			}
		}
		if !exists {
			listModified = true
			paperList = append(paperList,newPaper)
		}
	}

	h.paperListToDatabase(username,paperList)
	// if we added new papers, save the new string and clear new papers field in db
	if listModified {
		if h.paperListToDatabase(username,paperList) {
			// clear newPapers db field
			query := fmt.Sprintf("UPDATE userdata SET newpapers = '' WHERE username = '%s'", username)
			if h.papers.QueryFull(query) {
				fmt.Printf("for user %s, migrated newpapers to papers in database\n", username)
			}
		}
	}

	// output papers in json format
    fmt.Fprintf(rw, "{\"username\":\"%s\",\"papers\":[", username)

    for i, paper := range paperList {
        if i > 0 {
            fmt.Fprintf(rw, ",")
        }
        PrintJSONMetaInfo(rw, paper)
		PrintJSONContextInfo(rw, paper)
		PrintJSONRelevantRefs(rw, paper, paperList)
        fmt.Fprintf(rw, "}")
    }

	/* TAGS */
	/********/
    // build a list of TAGS  this profile
	tagList := h.tagListFromDatabase(tags)

    fmt.Printf("for user %s, read %d tags\n", username, len(tagList))

	fmt.Fprintf(rw, "],\"tags\":[")

	// output tags in json format
    for i, tag := range tagList {
        if i > 0 {
            fmt.Fprintf(rw, ",")
        }
		fmt.Fprintf(rw, "{\"name\":%s,\"starred\":\"%t\",\"blobbed\":\"%t\"}", tag.name, tag.starred, tag.blobbed)
    }

    fmt.Fprintf(rw, "]}")
}

func (h *MyHTTPHandler) ProfileSync(username string, passhash string, papers string, tags string, rw http.ResponseWriter) {

	if !h.ProfileAuthenticate(username,passhash) {
		return
	}


	// TODO parse given string and merge with newPapers
	// then save this back again
	// give user a JSON string of the new papers added during the sync

	//query = fmt.Sprintf("REPLACE INTO userdata (username,data) VALUES ('%s','%s')", username, data)
	query := fmt.Sprintf("UPDATE userdata SET papers = '%s', tags = '%s' WHERE username = '%s'", papers, tags, username)
    if !h.papers.QueryFull(query) {
        fmt.Fprintf(rw, "false")
    } else {
        fmt.Fprintf(rw, "true")
    }
}

func (h *MyHTTPHandler) ProfileChangePassword(username string, passhash string, newhash string, salt string, rw http.ResponseWriter) {
	if !h.ProfileAuthenticate(username,passhash) {
		return
	}

	saltNum, _ := strconv.ParseUint(salt, 10, 64)

	query := fmt.Sprintf("UPDATE userdata SET userhash = '%s', salt = %d WHERE username = '%s'", newhash, uint64(saltNum), username)
	fmt.Fprintf(rw, "{\"success\":\"%t\",\"salt\":\"%d\"}",h.papers.QueryFull(query),uint64(saltNum))
}

func (h *MyHTTPHandler) GetMetaRefsCites(id uint, rw http.ResponseWriter) {
    // query the paper and its refs and cites
    paper := h.papers.QueryPaper(id, "")
    h.papers.QueryRefs(paper, false)
    h.papers.QueryCites(paper, false)

    // check the paper exists
    if paper == nil {
        fmt.Fprintf(rw, "null")
        return
    }

    // print the json output
    PrintJSONMetaInfo(rw, paper)
    fmt.Fprintf(rw, ",")
    PrintJSONAllRefsCites(rw, paper)
    fmt.Fprintf(rw, "}")
}

func (h *MyHTTPHandler) GetMetas(ids []uint, rw http.ResponseWriter) {
    fmt.Fprintf(rw, "[")
    first := true
    for _, id := range ids {
        paper := h.papers.QueryPaper(id, "")
        if paper == nil {
            fmt.Printf("ERROR: GetMetas could not get meta for id %d; skipping\n", id)
            continue
        }

        if first {
            first = false
        } else {
            fmt.Fprintf(rw, ",")
        }

        PrintJSONMetaInfo(rw, paper)
        fmt.Fprintf(rw, "}")
    }
    fmt.Fprintf(rw, "]")
}

func (h *MyHTTPHandler) GetRefsCites(id uint, rw http.ResponseWriter) {
    // query the paper and its refs and cites
    paper := h.papers.QueryPaper(id, "")
    h.papers.QueryRefs(paper, false)
    h.papers.QueryCites(paper, false)

    // check the paper exists
    if paper == nil {
        fmt.Fprintf(rw, "null")
        return
    }

    // print the json output
    fmt.Fprintf(rw, "{\"id\":%d,", paper.id)
    PrintJSONAllRefsCites(rw, paper)
    fmt.Fprintf(rw, "}")
}

/* obsolete
func (h *MyHTTPHandler) LookupPaper(id uint, arxiv string, refsOnly bool, rw http.ResponseWriter) {
    paper := h.papers.QueryPaper(id, arxiv)
    h.papers.QueryRefs(paper, true)
    h.papers.QueryCites(paper, true)

    if paper == nil {
        fmt.Fprintf(rw, "{}")
        return
    }

    if !refsOnly {
        authors, _ := json.Marshal(paper.authors)
        title, _ := json.Marshal(paper.title)
        fmt.Fprintf(rw, "{\"id\":%d,\"arxiv\":\"%s\",\"authors\":%s,\"title\":%s,\"numRefs\":%d,\"numCites\":%d,\"allRefsCites\":true,\"refs\":[", paper.id, paper.arxiv, authors, title, len(paper.refs), paper.numCites)
    } else {
        fmt.Fprintf(rw, "{\"id\":%d,\"allRefsCites\":true,\"refs\":[", paper.id)
    }
    // Refs (future-> past)
    first := true
    for _, link := range paper.refs {
        if link.pastPaper != nil {
            if first {
                first = false
            } else {
                fmt.Fprintf(rw, ", ")
            }
            authors, _ := json.Marshal(link.pastPaper.authors)
            title, _ := json.Marshal(link.pastPaper.title)
            fmt.Fprintf(rw, "{\"id\":%d,\"arxiv\":\"%s\",\"authors\":%s,\"title\":%s,\"numCites\":%d,\"refFreq\":%d,\"tred\":%.4g}", link.pastPaper.id, link.pastPaper.arxiv, authors, title, link.pastPaper.numCites, link.refFreq, link.tredWeightNorm)
        }
    }
    // Cites (past -> future)
    fmt.Fprintf(rw, "], \"cites\":[")
    first = true
    for _, link := range paper.cites {
        if link.futurePaper != nil {
            if first {
                first = false
            } else {
                fmt.Fprintf(rw, ", ")
            }
            authors, _ := json.Marshal(link.futurePaper.authors)
            title, _ := json.Marshal(link.futurePaper.title)
            fmt.Fprintf(rw, "{\"id\":%d,\"arxiv\":\"%s\",\"authors\":%s,\"title\":%s,\"numCites\":%d,\"refFreq\":%d,\"tred\":%.4g}", link.futurePaper.id, link.futurePaper.arxiv, authors, title, link.futurePaper.numCites, link.refFreq, link.tredWeightNorm)
        }
    }
    fmt.Fprintf(rw, "]}")
}
*/

func (h *MyHTTPHandler) SearchArxiv(arxivString string, rw http.ResponseWriter) {
    // query the paper and its refs and cites
    paper := h.papers.QueryPaper(0, arxivString)
    h.papers.QueryRefs(paper, false)
    h.papers.QueryCites(paper, false)

    // check the paper exists
    if paper == nil {
        fmt.Fprintf(rw, "null")
        return
    }

    // print the json output
    PrintJSONMetaInfo(rw, paper)
    fmt.Fprintf(rw, ",")
    PrintJSONAllRefsCites(rw, paper)
    fmt.Fprintf(rw, "}")
}

func (h *MyHTTPHandler) SearchPaper(searchWhat string, searchString string, rw http.ResponseWriter) {
    if !h.papers.QueryBegin("SELECT meta_data.id,meta_data.arxiv,meta_data.authors,meta_data.title," + *flagPciteTable + ".numCites FROM meta_data," + *flagPciteTable + " WHERE MATCH (" + searchWhat + ") AGAINST ('" + searchString + "' IN BOOLEAN MODE) AND meta_data.id = " + *flagPciteTable + ".id") {
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
    for numResults < 20 {
        row := result.FetchRow()
        if row == nil {
            break
        }

        var ok bool
        var id uint64
        var arxiv string
        var authors string
        var title string
        var numCites uint64
        if id, ok = row[0].(uint64); !ok { continue }
        if arxiv, ok = row[1].(string); !ok { continue }
        if au, ok := row[2].([]byte); !ok {
            continue
        } else {
            authors = string(au)
        }
        if title, ok = row[3].(string); !ok { continue }
        if numCites, ok = row[4].(uint64); !ok {
            numCites = 0
        }

        if numResults > 0 {
            fmt.Fprintf(rw, ",")
        }
        PrintJSONMetaInfoUsing(rw, uint(id), arxiv, authors, title, uint(numCites), "", "")
        fmt.Fprintf(rw, ",\"allRefsCites\":false}")
        numResults += 1
    }
    fmt.Fprintf(rw, "]")
}

// this version just returns id and numCites for up to 500 results
func (h *MyHTTPHandler) SearchPaperV2(searchWhat string, searchString string, rw http.ResponseWriter) {
    if !h.papers.QueryBegin("SELECT meta_data.id," + *flagPciteTable + ".numCites FROM meta_data," + *flagPciteTable + " WHERE MATCH (" + searchWhat + ") AGAINST ('" + searchString + "' IN BOOLEAN MODE) AND meta_data.id = " + *flagPciteTable + ".id LIMIT 500") {
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
        if id, ok = row[0].(uint64); !ok { continue }
        if numCites, ok = row[1].(uint64); !ok {
            numCites = 0
        }

        if numResults > 0 {
            fmt.Fprintf(rw, ",")
        }
        fmt.Fprintf(rw, "{\"id\":%d,\"numCites\":%d}", id, numCites)
        numResults += 1
    }
    fmt.Fprintf(rw, "]")
}
