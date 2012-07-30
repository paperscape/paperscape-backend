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
func (h *MyHTTPHandler) PaperListToDBString (username string, paperList []*Paper) string {

	// This SHOULD be identical to JS code in kea i.e. it should be parseable
	// by the paperListFromDBString code below
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
	return w.String()
}


// Returns a list of papers stored in userdata string field
func (h *MyHTTPHandler) PaperListFromDBString (papers []byte) []*Paper {

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

		if req.Form["pchal"] != nil {
			// profile-challenge: authenticate request (send user a new "challenge")
			giveSalt := false
			// give user their salt once, so they can salt passwords in this session
			if req.Form["s"] != nil {
                // client requested salt
				giveSalt = true
			}
			h.ProfileChallenge(req.Form["pchal"][0], giveSalt, rw)
		} else if req.Form["plogin"] != nil && req.Form["h"] != nil {
            // profile-login: login request
            // h = passHash
            h.ProfileLogin(req.Form["plogin"][0], req.Form["h"][0], rw)
		} else if req.Form["pcull"] != nil && req.Form["h"] != nil {
            // profile-cull: clear newpapers db field request
            // h = passHash
			h.ProfileCullNewPapers(req.Form["pcull"][0], req.Form["h"][0], rw)
		} else if req.Form["pchpw"] != nil && req.Form["h"] != nil && req.Form["p"] != nil && req.Form["s"] != nil {
            // profile-change-password: change password request
            // h = passHash, p = payload, s = sprinkle (salt)
            h.ProfileChangePassword(req.Form["pchpw"][0], req.Form["h"][0], req.Form["p"][0], req.Form["s"][0], rw)
        } else if req.Form["gmrc"] != nil {
            // get-meta-refs-cites: get the meta data, refs and cites for the given paper id
            var id uint = 0
            if idNum, er := strconv.ParseUint(req.Form["gmrc"][0], 10, 0); er == nil {
                id = uint(idNum)
            }
            h.GetMetaRefsCites(id, rw)
        } else if req.Form["gm[]"] != nil {
            // get-metas: get the meta data for given list of paper ids
            var ids []uint
            for _, strId := range req.Form["gm[]"] {
                if preId, er := strconv.ParseUint(strId, 10, 0); er == nil {
                    ids = append(ids, uint(preId))
                } else {
                    fmt.Printf("ERROR: can't convert id '%s'; skipping\n", strId)
                }
            }
            h.GetMetas(ids, rw)
        } else if req.Form["grc"] != nil {
            // get-refs-cites: get the references and citations for a given paper id
            var id uint = 0
            if preId, er := strconv.ParseUint(req.Form["grc"][0], 10, 0); er == nil {
                id = uint(preId)
            }
            h.GetRefsCites(id, rw)
        } else if req.Form["ga"] != nil {
            // get-abstract: get the abstract for a paper
            var id uint = 0
            if idNum, er := strconv.ParseUint(req.Form["ga"][0], 10, 0); er == nil {
                id = uint(idNum)
            }
            abs, _ := json.Marshal(h.papers.GetAbstract(id))
            fmt.Fprintf(rw, "%s", abs)
        } else if req.Form["sax"] != nil {
            // search-arxiv: search papers for arxiv number
            h.SearchArxiv(req.Form["sax"][0], rw)
        } else if req.Form["sau"] != nil {
            // search-author: search papers for authors
            h.SearchPaper("authors", req.Form["sau"][0], rw)
        } else if req.Form["skw"] != nil {
            // search-keyword: search papers for keywords (just a title search at the moment)
            h.SearchPaper("title", req.Form["skw"][0], rw)
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

        if req.Form["psync"] != nil && req.Form["h"] != nil && req.Form["p"] != nil && req.Form["t"] != nil {
            // profile-sync: sync request
            // h = passHash, p = papers, t = tags
            h.ProfileSync(req.Form["psync"][0], req.Form["h"][0], req.Form["p"][0], req.Form["t"][0], rw)
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
    fmt.Fprintf(w, "{\"id\":%d,\"arxv\":\"%s\",\"auth\":%s,\"titl\":%s,\"nc\":%d", id, arxiv, authorsJSON, titleJSON, numCites)
    if len(journal) > 0 {
        fmt.Fprintf(w, ",\"jour\":\"%s\"", journal)
    }
    if len(doiJSON) > 0 {
        fmt.Fprintf(w, ",\"doi\":%s", doiJSON)
    }
}

func PrintJSONContextInfo(w io.Writer, paper *Paper) {
	fmt.Fprintf(w, ",\"x\":%d,\"note\":%s,", paper.xPos, paper.notes)
	fmt.Fprintf(w, "\"layr\":[")
	for j, layer := range paper.layers {
		if j > 0 {
			fmt.Fprintf(w, ",")
		}
		fmt.Fprintf(w, "%s", layer)
	}
	fmt.Fprintf(w, "],\"tag\":[")
	for j, tag := range paper.tags {
		if j > 0 {
			fmt.Fprintf(w, ",")
		}
		fmt.Fprintf(w, "%s", tag)
	}
	fmt.Fprintf(w, "],\"ntag\":[")
	for j, newTag := range paper.newTags {
		if j > 0 {
			fmt.Fprintf(w, ",")
		}
		fmt.Fprintf(w, "%s", newTag)
	}
	fmt.Fprintf(w, "]")
}

func PrintJSONRelevantRefs(w io.Writer, paper *Paper, paperList []*Paper) {
	fmt.Fprintf(w, ",\"allrc\":false,\"refs\":[")
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
    fmt.Fprintf(w, "{\"id\":%d,\"rord\":%d,\"rfrq\":%d,\"nc\":%d}", link.pastId, link.refOrder, link.refFreq, link.pastCited)
}

func PrintJSONLinkFutureInfo(w io.Writer, link *Link) {
    fmt.Fprintf(w, "{\"id\":%d,\"rord\":%d,\"rfrq\":%d,\"nc\":%d}", link.futureId, link.refOrder, link.refFreq, link.futureCited)
}

func PrintJSONAllRefsCites(w io.Writer, paper *Paper) {
    fmt.Fprintf(w, "\"allrc\":true,\"ref\":[")

    // output the refs (future -> past)
    for i, link := range paper.refs {
        if i > 0 {
            fmt.Fprintf(w, ",")
        }
        PrintJSONLinkPastInfo(w, link)
    }

    // output the cites (past -> future)
    fmt.Fprintf(w, "],\"cite\":[")
    for i, link := range paper.cites {
        if i > 0 {
            fmt.Fprintf(w, ",")
        }
        PrintJSONLinkFutureInfo(w, link)
    }

    fmt.Fprintf(w, "]")
}

func (h *MyHTTPHandler) SetChallenge(username string) int64 {
	// generate random "challenge" code
	challenge := rand.Int63();

	// store new challenge code in user database entry
	query := fmt.Sprintf("UPDATE userdata SET challenge = '%d' WHERE username = '%s'", challenge, username)
    if !h.papers.QueryFull(query) {
		fmt.Printf("ERROR: failed to set new challenge\n", username)
    }
	return challenge
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
	challenge := h.SetChallenge(username)

	// return challenge code
	if giveSalt {
		fmt.Fprintf(rw, "{\"name\":\"%s\",\"chal\":\"%d\",\"salt\":\"%d\"}", username, challenge, salt)
	} else {
		fmt.Fprintf(rw, "{\"name\":\"%s\",\"chal\":\"%d\"}", username, challenge)
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
	h.papers.QueryEnd()

    var papers,tags,newpapers []byte

    if row == nil {
		return
	} else {
        var ok bool
        papers, ok = row[0].([]byte)
        if !ok { papers = nil }
        tags, ok = row[1].([]byte)
        if !ok { tags = nil }
        newpapers, ok = row[2].([]byte)
        if !ok { newpapers = nil }
    }

	/* PAPERS */
	/**********/

    // build a list of PAPERS and their metadata for this profile 
	paperList := h.PaperListFromDBString(papers)
    fmt.Printf("for user %s, read %d papers\n", username, len(paperList))

	// and check for new papers that we don't already have
	newPaperList := h.PaperListFromDBString(newpapers)
    fmt.Printf("for user %s, read %d new papers\n", username, len(newPaperList))

	// make one super list of unique papers
	// if newPaperList has duplicates (it shouldn't), takes the first
	newPapersAdded := 0
	for _, newPaper := range newPaperList {
		exists := false
		for _, paper := range paperList {
			if newPaper.id == paper.id {
				exists = true
				break
			}
		}
		if !exists {
			paperList = append(paperList,newPaper)
			newPapersAdded += 1
		}
	}

	// if we added new papers, save the new string and clear new papers field in db
	if len(newPaperList) > 0 {
		if newPapersAdded > 0 {
			papersStr := h.PaperListToDBString(username,paperList)
			query := fmt.Sprintf("UPDATE userdata SET papers = '%s' WHERE username = '%s'", papersStr, username)
			if h.papers.QueryFull(query) {
				fmt.Printf("for user %s, migrated %d of %d newpapers to papers in database\n", username, newPapersAdded, len(newPaperList))
			} else {
				fmt.Printf("for user %s, error migrating %d of %d newpapers to papers in database\n", username, newPapersAdded, len(newPaperList))
			}
		} else {
			fmt.Printf("for user %s, migrated none of %d newpapers to papers in database\n", username, len(newPaperList))
		}
		// clear newPapers db field:
		query := fmt.Sprintf("UPDATE userdata SET newpapers = '' WHERE username = '%s'", username)
		h.papers.QueryFull(query)
	}

	// output papers in json format
    fmt.Fprintf(rw, "{\"name\":\"%s\",\"papr\":[", username)

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

	fmt.Fprintf(rw, "],\"tag\":[")

	// output tags in json format
    for i, tag := range tagList {
        if i > 0 {
            fmt.Fprintf(rw, ",")
        }
		fmt.Fprintf(rw, "{\"name\":%s,\"star\":\"%t\",\"blob\":\"%t\"}", tag.name, tag.starred, tag.blobbed)
    }

    fmt.Fprintf(rw, "]}")
}

func (h *MyHTTPHandler) ProfileSync(username string, passhash string, papers string, tags string, rw http.ResponseWriter) {

	if !h.ProfileAuthenticate(username,passhash) {
		return
	}

	papersStr := papers

	// Check if there are new papers we need to sync with
	query := fmt.Sprintf("SELECT newpapers FROM userdata WHERE username = '%s'", username)
	row := h.papers.QuerySingleRow(query)
    h.papers.QueryEnd()

    var newpapers []byte
    if row != nil {
        var ok bool
        newpapers, ok = row[0].([]byte)
        if !ok { newpapers = nil }
    }

	// if new papers, we may need to alter saved string
	var newPapersAdded,paperList []*Paper
	if newpapers != nil {
		paperList = h.PaperListFromDBString([]byte(papers))
		fmt.Printf("for user %s, read %d papers\n", username, len(paperList))

		newPaperList := h.PaperListFromDBString(newpapers)
		fmt.Printf("for user %s, read %d new papers\n", username, len(newPaperList))

		// make one super list of unique papers
		for _, newPaper := range newPaperList {
			exists := false
			for _, paper := range paperList {
				if newPaper.id == paper.id {
					exists = true
					break
				}
			}
			if !exists {
				paperList = append(paperList,newPaper)
				newPapersAdded = append(newPapersAdded,newPaper)
			}
		}

		// if we added new papers, save the new string
		if len(newPapersAdded) > 0 {
			papersStr = h.PaperListToDBString(username,paperList)
		}

	}

	query = fmt.Sprintf("UPDATE userdata SET papers = '%s', tags = '%s' WHERE username = '%s'", papersStr, tags, username)
    if !h.papers.QueryFull(query) {
		fmt.Fprintf(rw, "{\"succ\":\"false\"}")
    } else if len(newPapersAdded) > 0 {
		// generate random "challenge", as we expect user to reply
		// with a cull order of the newpapers field in db
		challenge := h.SetChallenge(username)
		// output new papers in json format
		fmt.Fprintf(rw, "{\"name\":\"%s\",\"succ\":\"true\",\"chal\":\"%d\",\"papr\":[", username,challenge)

		for i, paper := range newPapersAdded {
			if i > 0 {
				fmt.Fprintf(rw, ",")
			}
			PrintJSONMetaInfo(rw, paper)
			PrintJSONContextInfo(rw, paper)
			PrintJSONRelevantRefs(rw, paper, paperList)
			fmt.Fprintf(rw, "}")
		}
		fmt.Fprintf(rw, "]}")
		fmt.Printf("for user %s, sent %d new papers for sync\n", username, len(newPapersAdded))
	} else {
		fmt.Fprintf(rw, "{\"succ\":\"true\"}")
	}
}

func (h *MyHTTPHandler) ProfileChangePassword(username string, passhash string, newhash string, salt string, rw http.ResponseWriter) {
	if !h.ProfileAuthenticate(username,passhash) {
		return
	}

	saltNum, _ := strconv.ParseUint(salt, 10, 64)

	query := fmt.Sprintf("UPDATE userdata SET userhash = '%s', salt = %d WHERE username = '%s'", newhash, uint64(saltNum), username)
	fmt.Fprintf(rw, "{\"succ\":\"%t\",\"salt\":\"%d\"}",h.papers.QueryFull(query),uint64(saltNum))
}


func (h *MyHTTPHandler) ProfileCullNewPapers(username string, passhash string, rw http.ResponseWriter) {
	if !h.ProfileAuthenticate(username,passhash) {
		fmt.Fprintf(rw, "{\"succ\":\"false\"}")
		return
	}

	// clear newPapers db field:
	query := fmt.Sprintf("UPDATE userdata SET newpapers = '' WHERE username = '%s'", username)
	fmt.Fprintf(rw, "{\"succ\":\"%t\"}",h.papers.QueryFull(query))
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

// this version just returns id and numCites for up to 500 results
func (h *MyHTTPHandler) SearchPaper(searchWhat string, searchString string, rw http.ResponseWriter) {
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
        fmt.Fprintf(rw, "{\"id\":%d,\"nc\":%d}", id, numCites)
        numResults += 1
    }
    fmt.Fprintf(rw, "]")
}
