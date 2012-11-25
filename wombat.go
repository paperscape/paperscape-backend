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
    "strings"
	"math/rand"
	"crypto/sha1"
	"crypto/sha256"
	//"crypto/aes"
    "sort"
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
            *flagMetaBaseDir = "/opt/pscp/data/meta"
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
    allcats    string   // all arxiv categories (as a comma-separated string)
    authors    string   // authors
    title      string   // title
    publJSON   string   // publication string in JSON format
    refs       []*Link  // makes references to
    cites      []*Link  // cited by 
    numCites   uint     // number of times cited
    dNumCites1 uint     // change in numCites in past day
    dNumCites5 uint     // change in numCites in past 5 days
    xPos       int      // for loaded profile
    rMod       int      // for loaded profile
    notes      string   // for loaded profile
    layers     []string // for loaded profile
    tags       []string // for loaded profile
    newTags    []string // for loaded profile *obsolete*
	remove     bool     // for loaded profile, mark to remove from db
}

type Tag struct {
    name       string   // unique name
	active     bool		// whether tag is active *obsolete*
	blobbed    bool		// whether tag is blobbed
	blobCol	   int      // index of blob colour array
	starred    bool		// whether tag is starred
	remove     bool     // for loaded profile, mark to remove from db
	index      uint     // its position in tag array (necessary if we send differences rather than whole list)
}

// first is one with smallest id
type PaperSliceSortId []*Paper

func (ps PaperSliceSortId) Len() int           { return len(ps) }
func (ps PaperSliceSortId) Less(i, j int) bool { return ps[i].id < ps[j].id }
func (ps PaperSliceSortId) Swap(i, j int)      { ps[i], ps[j] = ps[j], ps[i] }

// sort alphabetically 
type TagSliceSortName []*Tag

func (ts TagSliceSortName) Len() int           { return len(ts) }
func (ts TagSliceSortName) Less(i, j int) bool {
	// tag names are wrapped with "", so remove these first before sorting
	return ts[i].name[1:len(ts[i].name)-1] < ts[j].name[1:len(ts[j].name)-1]
}
func (ts TagSliceSortName) Swap(i, j int)      { ts[i], ts[j] = ts[j], ts[i] }

// sort by index 
type TagSliceSortIndex []*Tag

func (ts TagSliceSortIndex) Len() int           { return len(ts) }
func (ts TagSliceSortIndex) Less(i, j int) bool { return ts[i].index < ts[j].index }
func (ts TagSliceSortIndex) Swap(i, j int)      { ts[i], ts[j] = ts[j], ts[i] }

type PapersEnv struct {
    db *mysql.Client
}

func NewPapersEnv(db *mysql.Client) *PapersEnv {
    papers := new(PapersEnv)
    papers.db = db
    db.Reconnect = true
    return papers
}

func (papers *PapersEnv) StatementBegin(sql string, params ...interface{}) *mysql.Statement {
    papers.db.Lock()
	stmt, err := papers.db.Prepare(sql)
	if err != nil {
        fmt.Println("MySQL statement error;", err)
		return nil
	}
	err = stmt.BindParams(params...)
	if err != nil {
        fmt.Println("MySQL statement error;", err)
		return nil
	}
	err = stmt.Execute()
	if err != nil {
        fmt.Println("MySQL statement error;", err)
		return nil
	}
	return stmt
}

func (papers *PapersEnv) StatementBindSingleRow(stmt *mysql.Statement, params ...interface{}) (success bool) {
	success = true
	if stmt != nil {
		stmt.BindResult(params...)
		var eof bool
		eof, err := stmt.Fetch()
		// expect only one row:
		if err != nil {
			fmt.Println("MySQL statement error;", err)
			success = false
		} else if eof {
			//fmt.Println("MySQL statement error; eof")
			// Row just didn't exist, return false but don't print error
			success = false
		}
		err = stmt.FreeResult()
		if err != nil {
			fmt.Println("MySQL statement error;", err)
			success = false
		}
	} else {
		success = false
	}
	// as only querying single row, close statement
	if !papers.StatementEnd(stmt) { success = false }
	return
}

func (papers *PapersEnv) StatementEnd(stmt *mysql.Statement) (success bool) {
	success = true
	if stmt != nil {
		err := stmt.Close()
		if err != nil {
			fmt.Println("MySQL statement error;", err)
			success = false
		}
	} else {
		success = false
	}
    papers.db.Unlock()
	return
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
        query = fmt.Sprintf("SELECT id,arxiv,maincat,allcats,authors,title,publ FROM meta_data WHERE id = %d", id)
    } else if len(arxiv) > 0 {
        // security issue: should make sure arxiv string is sanitised
        query = fmt.Sprintf("SELECT id,arxiv,maincat,allcats,authors,title,publ FROM meta_data WHERE arxiv = '%s'", arxiv)
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
    if paper.allcats, ok = row[3].(string); !ok { paper.allcats = "" }
    if row[4] == nil {
        paper.authors = "(unknown authors)"
    } else if au, ok := row[4].([]byte); !ok {
        fmt.Printf("ERROR: cannot get authors for id=%d; %v\n", paper.id, row[4])
        return nil
    } else {
        paper.authors = string(au)
    }
    if row[5] == nil {
        paper.authors = "(unknown title)"
    } else if title, ok := row[5].(string); !ok {
        fmt.Printf("ERROR: cannot get title for id=%d; %v\n", paper.id, row[5])
        return nil
    } else {
        paper.title = title
    }
    if row[6] == nil {
        paper.publJSON = "";
    } else if publ, ok := row[6].([]byte); !ok {
        fmt.Printf("ERROR: cannot get publ for id=%d; %v\n", paper.id, row[6])
        paper.publJSON = "";
    } else {
        publ2 := string(publ) // convert to string so marshalling does the correct thing
        publ3, _ := json.Marshal(publ2)
        paper.publJSON = string(publ3)
    }

    //// Get number of times cited, and change in number of cites
    query = fmt.Sprintf("SELECT numCites,dNumCites1,dNumCites5 FROM %s WHERE id = %d", *flagPciteTable, paper.id)
    row2 := papers.QuerySingleRow(query)

    if row2 != nil {
        if numCites, ok := row2[0].(uint64); ok {
            paper.numCites = uint(numCites)
        }
        if dNumCites1, ok := row2[1].(int64); ok {
            paper.dNumCites1 = uint(dNumCites1)
        }
        if dNumCites5, ok := row2[2].(int64); ok {
            paper.dNumCites5 = uint(dNumCites5)
        }
    }

    papers.QueryEnd()

    return paper
}

func GenerateRandString(minLen int, maxLen int) string {
	characters := []byte{'a','b','c','d','e','f','g','h','i','j','k','l','m','n','o','p','q','r','s','t','u','v','w','x','y','z','A','B','C','D','E','F','G','H','I','J','K','L','M','N','O','P','Q','R','S','T','U','V','W','X','Y','Z','1','2','3','4','5','6','7','8','9','0'}
	if maxLen < minLen { return "" }

	var length = minLen
	if maxLen > minLen {
		length += rand.Intn(maxLen-minLen)
	}

	bytes := make([]byte,0)
	for i:=0; i < length; i++ {
		ind := rand.Intn(62)
		bytes = append(bytes,characters[ind])
	}
	return string(bytes)
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
func (h *MyHTTPHandler) PaperListToDBString (paperList []*Paper) string {

	// This SHOULD be identical to JS code in kea i.e. it should be parseable
	// by the PaperListFromDBString code below
	w := new(bytes.Buffer)
	fmt.Fprintf(w,"v:4"); // PAPERS VERSION 4
	for _, paper := range paperList {
		fmt.Fprintf(w,"(%d,%d,%d,%s,l[",paper.id,paper.xPos,paper.rMod,paper.notes);
		for i, layer := range paper.layers {
			if i > 0 { fmt.Fprintf(w,","); }
			fmt.Fprintf(w,"%s",layer);
		}
		fmt.Fprintf(w,"],t[");
		for i, tag := range paper.tags {
			if i > 0 { fmt.Fprintf(w,","); }
			fmt.Fprintf(w,"%s",tag);
		}
		//fmt.Fprintf(w,"],n[");
		//for i, newTag := range paper.newTags {
		//	if i > 0 { fmt.Fprintf(w,","); }
		//	fmt.Fprintf(w,"%s",newTag);
		//}
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
			paper.rMod = 0
			paper.notes = notes
			paper.tags = tags
			tok = s.Scan()
			paperList = append(paperList, paper)
		}
	} else if papersVersion == 2 {
		// PAPERS VERSION 2 (deprecated)
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
			paper.rMod = 0
			paper.notes = notes
			paper.tags = tags
			paper.layers = layers
			paper.newTags = newTags
			tok = s.Scan()
			paperList = append(paperList, paper)
		}
	} else if papersVersion == 3 {
		// PAPERS VERSION 3 (deprecated)
		for tok != scanner.EOF {
			if tok != '(' { break }
			if tok = s.Scan(); tok != scanner.Int { break }
			paperId, _ := strconv.ParseUint(s.TokenText(), 10, 0)
			tok = s.Scan()
			if tok == ')' {
				// this paper was marked for deletion
				// so fill it with empty data 
				// and mark it as so
				paper := h.papers.QueryPaper(uint(paperId), "")
				paper.remove = true
				paperList = append(paperList, paper)
				tok = s.Scan()
				continue
			} else if tok != ',' { break }
			tok = s.Scan()
			negate := false
			if tok == '-' { negate = true; tok = s.Scan() }
			if tok != scanner.Int { break }
			xPos, _ := strconv.ParseInt(s.TokenText(), 10, 0)
			if negate { xPos = -xPos }
			if tok = s.Scan(); tok != ',' { break }
			tok = s.Scan()
			negate = false
			if tok == '-' { negate = true; tok = s.Scan() }
			if tok != scanner.Int { break }
			rMod, _ := strconv.ParseInt(s.TokenText(), 10, 0)
			if negate { rMod = -rMod }
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
			paper.rMod = int(rMod)
			paper.notes = notes
			paper.tags = tags
			paper.layers = layers
			paper.newTags = newTags
			paper.remove = false
			tok = s.Scan()
			paperList = append(paperList, paper)
		}
	} else if papersVersion == 4 {
		// PAPERS VERSION 4
		// this version removes "new" tags from version 3
		for tok != scanner.EOF {
			if tok != '(' { break }
			if tok = s.Scan(); tok != scanner.Int { break }
			paperId, _ := strconv.ParseUint(s.TokenText(), 10, 0)
			tok = s.Scan()
			if tok == ')' {
				// this paper was marked for deletion
				// so fill it with empty data 
				// and mark it as so
				paper := h.papers.QueryPaper(uint(paperId), "")
				paper.remove = true
				paperList = append(paperList, paper)
				tok = s.Scan()
				continue
			} else if tok != ',' { break }
			tok = s.Scan()
			negate := false
			if tok == '-' { negate = true; tok = s.Scan() }
			if tok != scanner.Int { break }
			xPos, _ := strconv.ParseInt(s.TokenText(), 10, 0)
			if negate { xPos = -xPos }
			if tok = s.Scan(); tok != ',' { break }
			tok = s.Scan()
			negate = false
			if tok == '-' { negate = true; tok = s.Scan() }
			if tok != scanner.Int { break }
			rMod, _ := strconv.ParseInt(s.TokenText(), 10, 0)
			if negate { rMod = -rMod }
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
			if tok = s.Scan(); tok != ')' { break }
			paper := h.papers.QueryPaper(uint(paperId), "")
			h.papers.QueryRefs(paper, false)
			paper.xPos = int(xPos)
			paper.rMod = int(rMod)
			paper.notes = notes
			paper.tags = tags
			paper.layers = layers
			paper.remove = false
			tok = s.Scan()
			paperList = append(paperList, paper)
		}
	}

    if tok != scanner.EOF {
        fmt.Printf("PaperListFromDBString scan error, unexpected token '%v'\n", tok)
    }

	return paperList
}

// Converts tag list into database string
func (h *MyHTTPHandler) TagListToDBString (tagList []*Tag) string {

	// This SHOULD be identical to JS code in kea i.e. it should be parseable
	// by the TagListFromDBString code below
	w := new(bytes.Buffer)
	fmt.Fprintf(w,"v:2"); // TAGS VERSION 2
	for _, tag := range tagList {
		fmt.Fprintf(w,"(%s,%d",tag.name,tag.index);
		fmt.Fprintf(w,",a!");
		// tag.active is obsolete
		//if !tag.active {
		//	fmt.Fprintf(w,"!");
		//}
		fmt.Fprintf(w,",s");
		if !tag.starred {
			fmt.Fprintf(w,"!");
		}
		fmt.Fprintf(w,",b");
		if !tag.blobbed {
			fmt.Fprintf(w,"!");
		}
		fmt.Fprintf(w,")");
	}
	return w.String()
}

// Returns a list of tags stored in userdata string field
func (h *MyHTTPHandler) TagListFromDBString (tags []byte) []*Tag {

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
			tag.index = 0; // for compatability with V2
			tag.active  = false
			tag.starred = true
			tag.blobbed = true
			tag.remove = false
			// tag name
			if tok = s.Scan(); tok != scanner.String { break }
			tag.name = s.TokenText()
			tok = s.Scan()
			if tok == ')' {
				// this tag was marked for deletion
				// so fill it with empty data 
				// and mark it as so
				tag.remove = true
				tagList = append(tagList, tag)
				tok = s.Scan()
				continue
			} else if tok != ',' { break }
			// tag starred?
			if tok = s.Scan(); tok == scanner.Ident && s.TokenText() != "s" { break }
			if tok = s.Scan(); tok == '!' {
				tag.starred = false
				tok = s.Scan()
			}
			if tok != ',' { break }
			// tag blobbed?
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
	} else if tagsVersion == 2 {
		// TAGS VERSION 2
		// this version adds a tag index for ranking and saves which tags are active
		for tok != scanner.EOF {
			if tok != '(' { break }
			tag := new(Tag)
			tag.active  = false // as obsolete now
			tag.starred = true
			tag.blobbed = true
			tag.remove = false
			// tag name
			if tok = s.Scan(); tok != scanner.String { break }
			tag.name = s.TokenText()
			tok = s.Scan()
			if tok == ')' {
				// this tag was marked for deletion
				// so fill it with empty data 
				// and mark it as so
				tag.remove = true
				tagList = append(tagList, tag)
				tok = s.Scan()
				continue
			} else if tok != ',' { break }
			// tag index (rank)
			if tok = s.Scan(); tok != scanner.Int { break }
			rank, _ := strconv.ParseUint(s.TokenText(), 10, 0)
			tag.index = uint(rank)
			if tok = s.Scan(); tok != ',' { break }
			// tag active?
			if tok = s.Scan(); tok == scanner.Ident && s.TokenText() != "a" { break }
			if tok = s.Scan(); tok == '!' {
				tag.active = false
				tok = s.Scan()
			}
			if tok != ',' { break }
			// tag starred?
			if tok = s.Scan(); tok == scanner.Ident && s.TokenText() != "s" { break }
			if tok = s.Scan(); tok == '!' {
				tag.starred = false
				tok = s.Scan()
			}
			if tok != ',' { break }
			// tag blobbed?
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
			giveVersion := false
			// give user their salt once, so they can salt passwords in this session
			if req.Form["s"] != nil {
                // client requested salt
				giveSalt = true
			}
			if req.Form["pv"] != nil {
                // client requested password version (useful if we want to
				// strengthen in future)
				giveVersion = true
			}
			h.ProfileChallenge(req.Form["pchal"][0], giveSalt, giveVersion, rw)
		} else if req.Form["pload"] != nil && req.Form["h"] != nil && req.Form["ph"] != nil && req.Form["th"] != nil {
            // profile-load: either login request or load request from an autosave
            // h = passHash, ph = papersHash, th = tagsHash
            h.ProfileLoad(req.Form["pload"][0], req.Form["h"][0], req.Form["ph"][0], req.Form["th"][0], rw)
		} else if req.Form["pchpw"] != nil && req.Form["h"] != nil && req.Form["p"] != nil && req.Form["s"] != nil && req.Form["pv"] != nil {
            // profile-change-password: change password request
            // h = passHash, p = payload, s = sprinkle (salt), pv = password version
            h.ProfileChangePassword(req.Form["pchpw"][0], req.Form["h"][0], req.Form["p"][0], req.Form["s"][0], req.Form["pv"][0], rw)
		} else if req.Form["gload"] != nil {
            // graph-load: from a page load
            h.GraphLoad(req.Form["gload"][0], rw)
        } else if req.Form["gdb"] != nil {
            // get-date-boundaries
            h.GetDateBoundaries(rw)
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
        } else if req.Form["grc[]"] != nil {
            // get-refs-cites: get the references and citations for given paper ids 
			// and date-boundaries
            var ids []uint
            for _, strId := range req.Form["grc[]"] {
                if preId, er := strconv.ParseUint(strId, 10, 0); er == nil {
                    ids = append(ids, uint(preId))
                } else {
                    fmt.Printf("ERROR: can't convert id '%s'; skipping\n", strId)
                }
            }
            var dbs []uint
            for _, strDb := range req.Form["dbs[]"] {
                if preDb, er := strconv.ParseUint(strDb, 10, 0); er == nil {
                    dbs = append(dbs, uint(preDb))
                } else {
                    fmt.Printf("ERROR: can't convert id '%s'; skipping\n", strDb)
                }
            }
            h.GetRefsCites(ids, dbs, rw)
        } else if req.Form["gnc[]"] != nil {
            // get-new-cites: get the recent citations for given paper ids 
            var ids []uint
            for _, strId := range req.Form["gnc[]"] {
                if preId, er := strconv.ParseUint(strId, 10, 0); er == nil {
                    ids = append(ids, uint(preId))
                } else {
                    fmt.Printf("ERROR: can't convert id '%s'; skipping\n", strId)
                }
            }
            h.GetNewCites(ids, rw)
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
            h.SearchAuthor(req.Form["sau"][0], rw)
        } else if req.Form["sti"] != nil {
            // search-title: search papers for words in title
            h.SearchTitle(req.Form["sti"][0], rw)
        } else if req.Form["sca"] != nil && req.Form["f"] != nil && req.Form["t"] != nil {
            // search-category: search papers between given id range, in given category
            // x = include cross lists, f = from, t = to
            h.SearchCategory(req.Form["sca"][0], req.Form["x"] != nil && req.Form["x"][0] == "true", req.Form["f"][0], req.Form["t"][0], rw)
        } else if req.Form["snp"] != nil && req.Form["f"] != nil && req.Form["t"] != nil {
            // search-new-papers: search papers between given id range
            // f = from, t = to
            h.SearchNewPapers(req.Form["f"][0], req.Form["t"][0], rw)
        } else if req.Form["str"] != nil {
            // search-trending: search papers that are "trending"
            h.SearchTrending(rw)
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

        if req.Form["psync"] != nil && req.Form["h"] != nil && req.Form["p"] != nil && req.Form["t"] != nil && req.Form["ph"] != nil && req.Form["th"] != nil {
            // profile-sync: sync request
            // h = passHash, p = papersdiff, t = tagsdiff
            h.ProfileSync(req.Form["psync"][0], req.Form["h"][0], req.Form["p"][0], req.Form["t"][0], req.Form["ph"][0], req.Form["th"][0], rw)
        } else if req.Form["gsave"] != nil {
            // graph-save: existing code (or empty string if none)
            // p = papers, ph = papers hash
            h.GraphSave(req.Form["gsave"][0], req.Form["p"][0], req.Form["ph"][0], rw)
        } else if req.Form["gm[]"] != nil {
            // get-metas: get the meta data for given list of paper ids
			// In case user wants many many metas, a POST is sent
            var ids []uint
            for _, strId := range req.Form["gm[]"] {
                if preId, er := strconv.ParseUint(strId, 10, 0); er == nil {
                    ids = append(ids, uint(preId))
                } else {
                    fmt.Printf("ERROR: can't convert id '%s'; skipping\n", strId)
                }
            }
            h.GetMetas(ids, rw)
        } else if req.Form["grc[]"] != nil {
            // get-refs-cites: get the references and citations for a given paper ids 
			// and date-boundaries
			// If user wants many many ids, a POST is sent
            var ids []uint
            for _, strId := range req.Form["grc[]"] {
                if preId, er := strconv.ParseUint(strId, 10, 0); er == nil {
                    ids = append(ids, uint(preId))
                } else {
                    fmt.Printf("ERROR: can't convert id '%s'; skipping\n", strId)
                }
            }
            var dbs []uint
            for _, strDb := range req.Form["dbs[]"] {
                if preDb, er := strconv.ParseUint(strDb, 10, 0); er == nil {
                    dbs = append(dbs, uint(preDb))
                } else {
                    fmt.Printf("ERROR: can't convert id '%s'; skipping\n", strDb)
                }
            }
            h.GetRefsCites(ids, dbs, rw)
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
    PrintJSONMetaInfoUsing(w, paper.id, paper.arxiv, paper.allcats, paper.authors, paper.title, paper.numCites, paper.dNumCites1, paper.dNumCites5, paper.publJSON)
}

func PrintJSONMetaInfoUsing(w io.Writer, id uint, arxiv string, allcats string, authors string, title string, numCites uint, dNumCites1 uint, dNumCites5 uint, publJSON string) {
    authorsJSON, _ := json.Marshal(authors)
    titleJSON, _ := json.Marshal(title)
    fmt.Fprintf(w, "{\"id\":%d,\"arxv\":\"%s\",\"cats\":\"%s\",\"auth\":%s,\"titl\":%s,\"nc\":%d,\"dnc1\":%d,\"dnc5\":%d", id, arxiv, allcats, authorsJSON, titleJSON, numCites, dNumCites1, dNumCites5)
    if len(publJSON) > 0 {
        fmt.Fprintf(w, ",\"publ\":%s", publJSON)
    }
}

func PrintJSONContextInfo(w io.Writer, paper *Paper) {
	fmt.Fprintf(w, ",\"x\":%d,\"rad\":%d,\"note\":%s,", paper.xPos, paper.rMod, paper.notes)
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
	// new tags are *obsolete*
	//fmt.Fprintf(w, "],\"ntag\":[")
	//for j, newTag := range paper.newTags {
	//	if j > 0 {
	//		fmt.Fprintf(w, ",")
	//	}
	//	fmt.Fprintf(w, "%s", newTag)
	//}
	fmt.Fprintf(w, "]")
}

func PrintJSONRelevantRefs(w io.Writer, paper *Paper, paperList []*Paper) {
	fmt.Fprintf(w, ",\"allrc\":false,\"ref\":[")
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

func PrintJSONAllRefsCites(w io.Writer, paper *Paper, dateBoundary uint) {
    fmt.Fprintf(w, "\"allrc\":true,\"ref\":[")

    // output the refs (future -> past)
	// If non-zero date boundary given, we already have the 
	// refs
	if dateBoundary == 0 {
		for i, link := range paper.refs {
			if i > 0 {
				fmt.Fprintf(w, ",")
			}
			PrintJSONLinkPastInfo(w, link)
		}
	}

    // output the cites (past -> future)
    fmt.Fprintf(w, "],\"cite\":[")
	first := true
    for _, link := range paper.cites {
		if link.futureId < dateBoundary  {
			continue
		}
        if !first {

            fmt.Fprintf(w, ",")
        }
        PrintJSONLinkFutureInfo(w, link)
		first = false
    }

    fmt.Fprintf(w, "]")
}


func PrintJSONNewCites(w io.Writer, paper *Paper, dateBoundary uint) {
    fmt.Fprintf(w, "\"allnc\":true,\"cite\":[")

    // output the cites (past -> future)
	first := true
    for _, link := range paper.cites {
		if link.futureId < dateBoundary  {
			continue
		}
        if !first {
            fmt.Fprintf(w, ",")
        }
        PrintJSONLinkFutureInfo(w, link)
		first = false
    }

    fmt.Fprintf(w, "]")
}

func (h *MyHTTPHandler) SetChallenge(username string) (challenge int64, success bool) {
	// generate random "challenge" code
	success = false
	challenge = rand.Int63()

	stmt := h.papers.StatementBegin("UPDATE userdata SET challenge = ? WHERE username = ?",challenge,h.papers.db.Escape(username))
	if !h.papers.StatementEnd(stmt) {
		return
	}
	success = true
	return
}

/* check username exists and get the 'salt' and/or 'version' */
func (h *MyHTTPHandler) ProfileChallenge(username string, giveSalt bool, giveVersion bool, rw http.ResponseWriter) {
	var salt uint64
	var pwdversion uint64

	stmt := h.papers.StatementBegin("SELECT salt,pwdversion FROM userdata WHERE username = ?",h.papers.db.Escape(username))
	if !h.papers.StatementBindSingleRow(stmt,&salt,&pwdversion) {
		fmt.Fprintf(rw, "false")
		return
	}

	// generate random "challenge" code
	challenge, success := h.SetChallenge(username)
	if success != true {
		fmt.Fprintf(rw, "false")
		return
	}

	// return challenge code
	fmt.Fprintf(rw, "{\"name\":\"%s\",\"chal\":\"%d\"",username, challenge);
	if giveSalt {
		fmt.Fprintf(rw, ",\"salt\":\"%d\"", salt)
	}
	if giveVersion {
		fmt.Fprintf(rw, ",\"pwdv\":\"%d\"", uint(pwdversion))
	}
	fmt.Fprintf(rw, "}")
}

func (h *MyHTTPHandler) ProfileAuthenticate(username string, passhash string) (success bool) {
	success = false

	// Check for valid username and get the user challenge and hash
	var challenge uint64
    var userhash string

	stmt := h.papers.StatementBegin("SELECT challenge,userhash FROM userdata WHERE username = ?",h.papers.db.Escape(username))
	if !h.papers.StatementBindSingleRow(stmt,&challenge,&userhash) {
		return
	}

	// Check the passhash!
	hash := sha256.New() // use more secure hash for passwords
	io.WriteString(hash, fmt.Sprintf("%s%d", userhash, challenge))
	tryhash := fmt.Sprintf("%x",hash.Sum(nil))

	if passhash != tryhash {
		fmt.Printf("ERROR: ProfileAuthenticate for '%s' - invalid password:  %s vs %s\n", username, passhash, tryhash)
		return
	}

	// we're THROUGH!!
	fmt.Printf("Succesfully authenticated user '%s'\n",username)
	success = true
	return
}

/* If given papers/tags hashes don't match with db, send user all their papers and tags.
   Login also uses this function by providing empty hashes. */
func (h *MyHTTPHandler) ProfileLoad(username string, passhash string, papershash string, tagshash string, rw http.ResponseWriter) {
	if !h.ProfileAuthenticate(username,passhash) {
		return
	}

	//var query string
	//var row mysql.Row

	// generate random "challenge", as we expect user to reply
	// with a sync request if this is an autosave
	challenge, success := h.SetChallenge(username)
	if success != true {
		return
	}

    var papers,tags []byte
	var papershashOld,tagshashOld string

	stmt := h.papers.StatementBegin("SELECT papers,tags,papershash,tagshash FROM userdata WHERE username = ?",h.papers.db.Escape(username))
	if !h.papers.StatementBindSingleRow(stmt,&papers,&tags,&papershashOld,&tagshashOld) {
		return
	}

	/* Check if papers/tags hashes up to date if given (else assume this is a login) */
	if papershash != "" && tagshash != "" {
		if (papershashOld == papershash && tagshashOld == tagshash) {
			// hashes match, 
			fmt.Fprintf(rw, "{\"name\":\"%s\",\"chal\":\"%d\",\"papr\":[],\"tag\":[],\"ph\":\"%s\",\"th\":\"%s\"}",username,challenge,papershashOld,tagshashOld)
			return
		}
	} else {
		// as no useful papers/tags hashes given, record this load as a LOGIN
		stmt := h.papers.StatementBegin("UPDATE userdata SET numlogin = numlogin + 1, lastlogin = NOW() WHERE username = ?",h.papers.db.Escape(username))
		if !h.papers.StatementEnd(stmt) {
			return
		}
	}

	/* PAPERS */
	/**********/

    // build a list of PAPERS and their metadata for this profile 
	papersList := h.PaperListFromDBString(papers)
    fmt.Printf("for user %s, read %d papers\n", username, len(papersList))
    sort.Sort(PaperSliceSortId(papersList))
	papersStr := h.PaperListToDBString(papersList)

	// create papershash, and also store this in db
	hash := sha1.New()
	io.WriteString(hash, fmt.Sprintf("%s", string(papersStr)))
	papershashDb := fmt.Sprintf("%x",hash.Sum(nil))

	// compare hash with what was in db, if different update
	// this is important for users without profile!
	if papershashDb != papershashOld {
		stmt := h.papers.StatementBegin("UPDATE userdata SET papershash = ?, papers = ? WHERE username = ?",papershashDb,papersStr,h.papers.db.Escape(username))
		if !h.papers.StatementEnd(stmt) {
			fmt.Printf("ERROR: failed to set new papers field and hash for user %s\n", username)
		} else {
			fmt.Printf("for user %s, paper string updated\n", username)
		}
	}

	// Get 5 days ago date boundary so we can pass along new cites
	row := h.papers.QuerySingleRow("SELECT id FROM datebdry WHERE daysAgo = 5")
	h.papers.QueryEnd()
	var db uint64
	if row == nil {
		fmt.Printf("ERROR: ProfileLoad could not get 5 day boundary from MySQL\n")
		db = 0
	} else {
		var ok bool
		if db, ok = row[0].(uint64); !ok {
			fmt.Printf("ERROR: ProfileLoad could not get 5 day boundary from Row\n")
			db = 0
		}
	}

	// output papers in json format
	fmt.Fprintf(rw, "{\"name\":\"%s\",\"chal\":\"%d\",\"papr\":[", username,challenge)
    for i, paper := range papersList {
        if i > 0 {
            fmt.Fprintf(rw, ",")
        }
        PrintJSONMetaInfo(rw, paper)
		PrintJSONContextInfo(rw, paper)
		PrintJSONRelevantRefs(rw, paper, papersList)
		if db > 0 {
			h.papers.QueryCites(paper, false)
            fmt.Fprintf(rw, ",")
			PrintJSONNewCites(rw, paper, uint(db))
		}
		fmt.Fprintf(rw, "}")
    }
	fmt.Fprintf(rw, "],\"ph\":\"%s\"",papershashDb)

	/* TAGS */
	/********/

    // build a list of TAGS for this profile
	tagsList := h.TagListFromDBString(tags)
    fmt.Printf("for user %s, read %d tags\n", username, len(tagsList))
    // Keep in original order!
	//sort.Sort(TagSliceSortName(tagsList))
	sort.Sort(TagSliceSortIndex(tagsList))
	tagsStr := h.TagListToDBString(tagsList)

	// create tagshash
	hash = sha1.New()
	io.WriteString(hash, fmt.Sprintf("%s", tagsStr))
	tagshashDb := fmt.Sprintf("%x",hash.Sum(nil))

	// compare hash with what was in db, if different update
	// this is important for users without profile!
	if tagshashDb != tagshashOld {
		stmt := h.papers.StatementBegin("UPDATE userdata SET tagshash = ?, tags = ? WHERE username = ?",tagshashDb,tagsStr,h.papers.db.Escape(username))
		if !h.papers.StatementEnd(stmt) {
			fmt.Printf("ERROR: failed to set new tags field and hash for user %s\n", username)
		} else {
			fmt.Printf("for user %s, tag string updated\n", username)
		}
	}

	// output tags in json format
	fmt.Fprintf(rw, ",\"tag\":[")
    for i, tag := range tagsList {
        if i > 0 {
            fmt.Fprintf(rw, ",")
        }
		fmt.Fprintf(rw, "{\"name\":%s,\"index\":%d,\"star\":\"%t\",\"blob\":\"%t\"}", tag.name, tag.index, tag.starred, tag.blobbed)
    }
	fmt.Fprintf(rw, "],\"th\":\"%s\"}",tagshashDb)
}

/* Profile Sync */
func (h *MyHTTPHandler) ProfileSync(username string, passhash string, diffpapers string, difftags string, papershash string, tagshash string, rw http.ResponseWriter) {
	if !h.ProfileAuthenticate(username,passhash) {
		return
	}

	var papers,tags []byte

	stmt := h.papers.StatementBegin("SELECT papers,tags FROM userdata WHERE username = ?",h.papers.db.Escape(username))
	if !h.papers.StatementBindSingleRow(stmt,&papers,&tags) {
		return
	}

	/* PAPERS */
	/**********/

	oldpapersList := h.PaperListFromDBString(papers)
	fmt.Printf("for user %s, read %d papers from db\n", username, len(oldpapersList))

	// papers without details e.g. (id) are flagged with a "remove" 
	newpapersList := h.PaperListFromDBString([]byte(diffpapers))
	fmt.Printf("for user %s, read %d diff papers from internets\n", username, len(newpapersList))

	// make one super list of unique papers (diffpapers override oldpapers)
	for _, oldpaper := range oldpapersList {
		exists := false
		for _, diffpaper := range newpapersList {
			if diffpaper.id == oldpaper.id {
				exists = true
				break
			}
		}
		if !exists {
			newpapersList = append(newpapersList,oldpaper)
		}
	}

	var papersList []*Paper
	// remove papers marked with "remove" or those with empty layers and tags!
	for _, paper := range newpapersList {
		if !paper.remove && (len(paper.layers) > 0 || len(paper.tags) > 0) {
			papersList = append(papersList,paper)
		}
	}

	// sort this list
    sort.Sort(PaperSliceSortId(papersList))
	papersStr := h.PaperListToDBString(papersList)

	// create new hashes 
	hash := sha1.New()
	io.WriteString(hash, fmt.Sprintf("%s", string(papersStr)))
	papershashDb := fmt.Sprintf("%x",hash.Sum(nil))

	// compare with hashes we were sent (should match!!)
	if papershash != papershashDb {
		fmt.Printf("Error: for user %s, new sync paper hashes don't match those sent from client: %s vs %s\n", username,papershash,papershashDb)
		fmt.Fprintf(rw, "{\"succ\":\"false\"}")
		return
	}

	/* TAGS */
	/********/

	oldtagsList := h.TagListFromDBString(tags);
	fmt.Printf("for user %s, read %d tags from db\n", username, len(oldtagsList))

	// tags without details e.g. (name) are flagged with a "remove" 
	newtagsList := h.TagListFromDBString([]byte(difftags))
	fmt.Printf("for user %s, read %d diff tags from internets\n", username, len(newtagsList))

	// make one super list of unique tags (difftags override oldtags)
	for _, oldtag := range oldtagsList {
		exists := false
		for _, difftag := range newtagsList {
			if difftag.name == oldtag.name {
				exists = true
				break
			}
		}
		if !exists {
			newtagsList = append(newtagsList,oldtag)
		}
	}

	var tagsList []*Tag
	// remove tags marked with "remove" 
	for _, tag := range newtagsList {
		if !tag.remove {
			tagsList = append(tagsList,tag)
		}
	}

	// sort this list
	// Keep in original order!
    //sort.Sort(TagSliceSortName(tagsList))
    sort.Sort(TagSliceSortIndex(tagsList))
	tagsStr := h.TagListToDBString(tagsList)

	hash = sha1.New()
	io.WriteString(hash, fmt.Sprintf("%s", tagsStr))
	tagshashDb := fmt.Sprintf("%x",hash.Sum(nil))

	// compare with hashes we were sent (should match!!)
	if tagshash != tagshashDb {
		fmt.Printf("ERROR: for user %s, new sync tag hashes don't match those sent from client: %s vs %s\n", username,tagshash,tagshashDb)
		fmt.Fprintf(rw, "{\"succ\":\"false\"}")
		return
	}

	/* MYSQL */
	/*********/

	stmt = h.papers.StatementBegin("UPDATE userdata SET papers = ?, tags = ?, papershash = ?, tagshash = ?, numsync = numsync + 1, lastsync = NOW() WHERE username = ?", papersStr, tagsStr, papershashDb, tagshashDb, h.papers.db.Escape(username))
	if !h.papers.StatementEnd(stmt) {
		fmt.Fprintf(rw, "{\"succ\":\"false\"}")
		return
	}

	// We succeeded
	fmt.Fprintf(rw, "{\"succ\":\"true\",\"ph\":\"%s\",\"th\":\"%s\"}",papershashDb,tagshashDb)

}

/* ProfileChangePassword */
func (h *MyHTTPHandler) ProfileChangePassword(username string, passhash string, newhash string, salt string, pwdversion string, rw http.ResponseWriter) {
	if !h.ProfileAuthenticate(username,passhash) {
		return
	}

	pwdvNum, _ := strconv.ParseUint(pwdversion, 10, 64)
	saltNum, _ := strconv.ParseUint(salt, 10, 64)

	// decrypt newhash
	//var userhash []byte
	//stmt := h.papers.StatementBegin("SELECT userhash FROM userdata WHERE username = ?",h.papers.db.Escape(username))
	//if !h.papers.StatementBindSingleRow(stmt,&userhash) {
	//	return
	//}
	// convert userhash to 32 byte key
	//fmt.Printf("length of userhash %d\n", len(userhash))
	//cipher, err := aes.NewCipher(userhash[:16])
	//if err != nil {
	//	fmt.Printf("ERROR: for user %s, could not create aes cipher to decrypt new password\n", username)
	//}
	//output := make([]byte);
	//cipher.Decrypt([]byte(newhash),output)


	success := true
        stmt := h.papers.StatementBegin("UPDATE userdata SET userhash = ?, salt = ?, pwdversion = ? WHERE username = ?", h.papers.db.Escape(newhash), uint64(saltNum), uint64(pwdvNum), h.papers.db.Escape(username))
	if !h.papers.StatementEnd(stmt) {
		success = false
	}

	fmt.Fprintf(rw, "{\"succ\":\"%t\",\"salt\":\"%d\",\"pwdv\":\"%d\"}",success,uint64(saltNum),uint64(pwdvNum))
}

/* Serves stored graph on user page load */
func (h *MyHTTPHandler) GraphLoad(code string, rw http.ResponseWriter) {

    var papers,tags []byte
	modcode := ""

	// discover if we've loading code or modcode
	// codes and modcodes are unique
	// first check if its a code
	stmt := h.papers.StatementBegin("SELECT papers,tags FROM sharedata WHERE code = ?",h.papers.db.Escape(code))
	if !h.papers.StatementBindSingleRow(stmt,&papers,&tags) {
		// It wasn't, so check if its a modcode
		var modcodeDb, codeDb string
		stmt := h.papers.StatementBegin("SELECT papers,tags,code,modkey FROM sharedata WHERE modkey = ?",h.papers.db.Escape(code))
		if !h.papers.StatementBindSingleRow(stmt,&papers,&tags,&codeDb,&modcodeDb) {
			return
		}
		code = codeDb
		modcode = modcodeDb
	}

	stmt = h.papers.StatementBegin("UPDATE sharedata SET numloaded = numloaded + 1, lastloaded = NOW() WHERE code = ?",h.papers.db.Escape(code))
	if !h.papers.StatementEnd(stmt) {
		return
	}

	/* PAPERS */

    // build a list of PAPERS and their metadata for this profile 
	papersList := h.PaperListFromDBString(papers)
    fmt.Printf("for graph code %s, read %d papers\n", code, len(papersList))
    sort.Sort(PaperSliceSortId(papersList))
	papersStr := h.PaperListToDBString(papersList)

	// create papershash, and also store this in db
	hash := sha1.New()
	io.WriteString(hash, fmt.Sprintf("%s", string(papersStr)))
	papershashDb := fmt.Sprintf("%x",hash.Sum(nil))

	// Get 5 days ago date boundary so we can pass along new cites
	row := h.papers.QuerySingleRow("SELECT id FROM datebdry WHERE daysAgo = 5")
	h.papers.QueryEnd()
	var db uint64
	if row == nil {
		fmt.Printf("ERROR: GraphLoad could not get 5 day boundary from MySQL\n")
		db = 0
	} else {
		var ok bool
		if db, ok = row[0].(uint64); !ok {
			fmt.Printf("ERROR: GraphLoad could not get 5 day boundary from Row\n")
			db = 0
		}
	}

	// output papers in json format
	fmt.Fprintf(rw, "{\"code\":\"%s\",\"mkey\":\"%s\",\"papr\":[", code, modcode)
    for i, paper := range papersList {
        if i > 0 {
            fmt.Fprintf(rw, ",")
        }
        PrintJSONMetaInfo(rw, paper)
		PrintJSONContextInfo(rw, paper)
		PrintJSONRelevantRefs(rw, paper, papersList)
		if db > 0 {
			h.papers.QueryCites(paper, false)
            fmt.Fprintf(rw, ",")
			PrintJSONNewCites(rw, paper, uint(db))
		}
		fmt.Fprintf(rw, "}")
    }
	fmt.Fprintf(rw, "],\"ph\":\"%s\"",papershashDb)

	/* TAGS */

    // build a list of TAGS for this profile
	tagsList := h.TagListFromDBString(tags)
    //fmt.Printf("for graph code %s, read %d tags\n", code, len(tagsList))
    // Keep in original order!
	sort.Sort(TagSliceSortIndex(tagsList))
	tagsStr := h.TagListToDBString(tagsList)

	// create tagshash
	hash = sha1.New()
	io.WriteString(hash, fmt.Sprintf("%s", tagsStr))
	tagshashDb := fmt.Sprintf("%x",hash.Sum(nil))

	// output tags in json format
	fmt.Fprintf(rw, ",\"tag\":[")
    for i, tag := range tagsList {
        if i > 0 {
            fmt.Fprintf(rw, ",")
        }
		fmt.Fprintf(rw, "{\"name\":%s,\"index\":%d,\"star\":\"%t\",\"blob\":\"%t\"}", tag.name, tag.index, tag.starred, tag.blobbed)
    }
	fmt.Fprintf(rw, "],\"th\":\"%s\"}",tagshashDb)
}

/* Graph Save */
func (h *MyHTTPHandler) GraphSave(modcode string, papers string, papershash string, rw http.ResponseWriter) {

	papersList := h.PaperListFromDBString([]byte(papers))
	if len(papersList) == 0 {
		return
	}
	fmt.Printf("for graph code %s, read %d papers from db\n", modcode, len(papersList))
    sort.Sort(PaperSliceSortId(papersList))
	papersStr := h.PaperListToDBString(papersList)

	var tagsList []*Tag // leave empty
	tagsStr := h.TagListToDBString(tagsList)


	if len(modcode) > 16 {
		fmt.Printf("ERROR: GraphSave given code are too long\n")
		return
	}
	var code string
	// if user gave modcode, load appropriate sharecode (if valid)
	if len(modcode) > 0 {
		stmt := h.papers.StatementBegin("SELECT code FROM sharedata WHERE modkey = ?",h.papers.db.Escape(modcode))
		if !h.papers.StatementBindSingleRow(stmt,&code) {
			return
		}
	} else {
		// User wants a new code and modcode, so generate them
		// codes and modcodes must be unique wrt each other
		// For now just generate something random by trial and error (i.e. repeat if it exists)
		// If this ends up being too slow (too many codes already taken), then we can precompute
		code = ""
		modcode = ""
		// just so this can't crash server, only try it N times
		N := 50
		for i := 0; i < 50; i++ {
			code = GenerateRandString(8,8)
			modcode = GenerateRandString(8,8)
			stmt := h.papers.StatementBegin("SELECT code FROM sharedata WHERE code = ? OR modkey = ? OR code = ? OR modkey = ?",h.papers.db.Escape(code),h.papers.db.Escape(code),h.papers.db.Escape(modcode),h.papers.db.Escape(modcode))
			var fubar string
			if !h.papers.StatementBindSingleRow(stmt,&fubar) {
				break
			}
		}
		if code == "" || modcode == "" {
			fmt.Printf("ERROR: GraphSave couldn't generate a code and modcode in %d tries!\n",N)
			return
		} else {
			stmt := h.papers.StatementBegin("INSERT INTO sharedata (code,modkey,lastloaded) VALUES (?,?,NOW())",h.papers.db.Escape(code),h.papers.db.Escape(modcode))
			if !h.papers.StatementEnd(stmt) {return}
		}
	}

	// save
	stmt := h.papers.StatementBegin("UPDATE sharedata SET papers = ?, tags = ? where code = ? AND modkey = ?", papersStr, tagsStr, h.papers.db.Escape(code), h.papers.db.Escape(modcode))
	if !h.papers.StatementEnd(stmt) {
		return
	}

	// We succeeded
	fmt.Fprintf(rw, "{\"code\":\"%s\",\"mkey\":\"%s\"}",code,modcode)
}

func (h *MyHTTPHandler) GetDateBoundaries(rw http.ResponseWriter) {
    // perform query
	// TODO convert to Prepared Statements, or keep as normal query, which is faster
    if !h.papers.QueryBegin("SELECT daysAgo,id FROM datebdry WHERE daysAgo <= 5") {
        return
    }

    defer h.papers.QueryEnd()

    // get result set  
    result, err := h.papers.db.UseResult()
    if err != nil {
        fmt.Println("MySQL use result error;", err)
        return
    }

    // get each row from the result and create the JSON object
    fmt.Fprintf(rw, "{")
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
    PrintJSONAllRefsCites(rw, paper, 0)
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

func (h *MyHTTPHandler) GetRefsCites(ids []uint, dbs []uint, rw http.ResponseWriter) {
    fmt.Fprintf(rw, "[")
    first := true
	if len(ids) != len(dbs) {
		fmt.Printf("ERROR: GetRefsCites had incompatible length for ids and their dates\n")
		return
	}
    for i := 0; i < len(ids); i++ {
		id := ids[i]
		db := dbs[i] // date boundary for this id (we have everything before it)
		// query the paper and its refs and cites
		paper := h.papers.QueryPaper(id, "")
		h.papers.QueryRefs(paper, false)
		h.papers.QueryCites(paper, false)

		// check the paper exists
		if paper == nil {
            fmt.Printf("ERROR: GetRefsCites could not find paper for id %d; skipping\n", id)
            continue
		}

		if first {
            first = false
        } else {
            fmt.Fprintf(rw, ",")
        }

		// print the json output
		fmt.Fprintf(rw, "{\"id\":%d,", paper.id)
		PrintJSONAllRefsCites(rw, paper, db)
		fmt.Fprintf(rw, "}")
    }
    fmt.Fprintf(rw, "]")
}

func (h *MyHTTPHandler) GetNewCites(ids []uint, rw http.ResponseWriter) {
	row := h.papers.QuerySingleRow("SELECT id FROM datebdry WHERE daysAgo = 5")
	h.papers.QueryEnd()
	if row == nil {
		fmt.Printf("ERROR: GetNewCites could not get 5 day boundary from MySQL\n")
        fmt.Fprintf(rw, "[]")
		return
	}
	var ok bool
	var db uint64
	if db, ok = row[0].(uint64); !ok {
		fmt.Printf("ERROR: GetNewCites could not get 5 day boundary from Row\n")
        fmt.Fprintf(rw, "[]")
		return
	}
    fmt.Fprintf(rw, "[")
    first := true
    for i := 0; i < len(ids); i++ {
		id := ids[i]
		// query the paper and its refs and cites
		paper := h.papers.QueryPaper(id, "")
		h.papers.QueryCites(paper, false)

		// check the paper exists
		if paper == nil {
            fmt.Printf("ERROR: GetNewCites could not find paper for id %d; skipping\n", id)
            continue
		}

		if first {
            first = false
        } else {
            fmt.Fprintf(rw, ",")
        }

		// print the json output
		fmt.Fprintf(rw, "{\"id\":%d,", paper.id)
		PrintJSONNewCites(rw, paper, uint(db))
		fmt.Fprintf(rw, "}")
    }
    fmt.Fprintf(rw, "]")
}

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
    PrintJSONMetaInfo(rw, paper)
    fmt.Fprintf(rw, ",")
    PrintJSONAllRefsCites(rw, paper, 0)
    fmt.Fprintf(rw, "}")
}

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

// searches for all papers within the id range, with main category as given
// returns id, numCites, refs for up to 500 results
func (h *MyHTTPHandler) SearchCategory(category string, includeCrossLists bool, idFrom string, idTo string, rw http.ResponseWriter) {
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

    // sanity check of id numbers
    if !sanityCheckId(idFrom) {
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
    if !h.papers.QueryBegin("SELECT meta_data.id," + *flagPciteTable + ".numCites," + *flagPciteTable + ".refs FROM meta_data," + *flagPciteTable + " WHERE meta_data.id=" + *flagPciteTable + ".id AND (" + whereClause + ") LIMIT 500") {
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
        var refStr []byte
        if id, ok = row[0].(uint64); !ok { continue }
        if numCites, ok = row[1].(uint64); !ok { numCites = 0 }
        if refStr, ok = row[2].([]byte); !ok { /* refStr is empty, that's okay */ }

        if numResults > 0 {
            fmt.Fprintf(rw, ",")
        }
        fmt.Fprintf(rw, "{\"id\":%d,\"nc\":%d,\"ref\":", id, numCites)
        ParseRefsCitesStringToJSONListOfIds(refStr, rw)
        fmt.Fprintf(rw, "}")
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

    if !h.papers.QueryBegin("SELECT meta_data.id,meta_data.allcats," + *flagPciteTable + ".numCites," + *flagPciteTable + ".refs FROM meta_data," + *flagPciteTable + " WHERE meta_data.id >= " + idFrom + " AND meta_data.id <= " + idTo + " AND meta_data.id = " + *flagPciteTable + ".id LIMIT 500") {
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
        var refStr []byte
        if id, ok = row[0].(uint64); !ok { continue }
        if allcats, ok = row[1].(string); !ok { continue }
        if numCites, ok = row[2].(uint64); !ok { numCites = 0 }
        if refStr, ok = row[3].([]byte); !ok { }

        if numResults > 0 {
            fmt.Fprintf(rw, ",")
        }
        fmt.Fprintf(rw, "{\"id\":%d,\"cat\":\"%s\",\"nc\":%d,\"ref\":", id, allcats, numCites)
        ParseRefsCitesStringToJSONListOfIds(refStr, rw)
        fmt.Fprintf(rw, "}")
        numResults += 1
    }
    fmt.Fprintf(rw, "]")
}

func ParseRefsCitesStringToJSONListOfIds(blob []byte, rw http.ResponseWriter) {
    fmt.Fprintf(rw, "[")
    for i := 0; i + 10 <= len(blob); i += 10 {
        refId := getLE32(blob, i)
        if i > 0 {
            fmt.Fprintf(rw, ",")
        }
        fmt.Fprintf(rw, "%d", refId)
    }
    fmt.Fprintf(rw, "]")
}

// searches for trending papers
// returns list of id and numCites
func (h *MyHTTPHandler) SearchTrending(rw http.ResponseWriter) {
    row := h.papers.QuerySingleRow("SELECT value FROM misc WHERE field='trending'")
    if row == nil {
        h.papers.QueryEnd()
        fmt.Fprintf(rw, "[]")
        return
    }

    if value, ok := row[0].(string); !ok {
        h.papers.QueryEnd()
        fmt.Fprintf(rw, "[]")
        return
    } else {
        // create the JSON object
        h.papers.QueryEnd()
        ids := strings.Split(value, ",")
        fmt.Fprintf(rw, "[")
        for i := 0; i + 1 < len(ids); i += 2 {
            if i > 0 {
                fmt.Fprintf(rw, ",")
            }
            fmt.Fprintf(rw, "{\"id\":%s,\"nc\":%s}", ids[i], ids[i + 1])
        }
        fmt.Fprintf(rw, "]")
    }
}
