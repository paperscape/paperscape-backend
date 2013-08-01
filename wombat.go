package main

import (
    "io"
    "io/ioutil"
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
    //"text/scanner"
    "GoMySQL"
    "runtime"
    "bytes"
    "time"
    "strings"
    "math/rand"
    "crypto/sha1"
    "crypto/sha256"
    "compress/gzip"
    //"crypto/aes"
    "sort"
    "net/smtp"
    "log"
    "xiwi"
)

// Current version of paperscape.
// Any client "kea" instances that don't match this will
// be prompted to do a hard reload of the latest version
// Kea equivalent is set in profile.js
var VERSION = "0.14"

// Max number of ids we will convert from human to internal form at a time
// We need some sane limit to stop mysql being spammed! If profile bigger than this,
// it will need to make several calls
var ID_CONVERSION_LIMIT = 50

var flagDB      = flag.String("db", "", "MySQL database to connect to")
var flagLogFile = flag.String("log-file", "", "file to output log information to")
var flagFastCGIAddr = flag.String("fcgi", "", "listening on given address using FastCGI protocol (eg -fcgi :9100)")
var flagHTTPAddr = flag.String("http", "", "listening on given address using HTTP protocol (eg -http :8089)")
var flagTestQueryId = flag.Uint("test-id", 0, "run a test query with id")
var flagTestQueryArxiv = flag.String("test-arxiv", "", "run a test query with arxiv")
var flagMetaBaseDir = flag.String("meta", "", "Base directory for meta file data (abstracts etc.)")

func main() {
    // pick random seed (default is Seed(1)...)
    rand.Seed(time.Now().UTC().UnixNano())

    // parse command line options
    flag.Parse()

    // set log file to use file instead of stdout
    if len(*flagLogFile) != 0 {
        file, err := os.OpenFile(*flagLogFile, os.O_WRONLY|os.O_APPEND|os.O_CREATE,0640)
        if err == nil {
            log.SetOutput(file)
        } else {
            fmt.Println(err)
        }
    }

    if len(*flagMetaBaseDir) == 0 {
        *flagMetaBaseDir = "/opt/pscp/data/meta"
    }

    // connect to db
    db := xiwi.ConnectToDB(*flagDB)
    if db == nil {
        return
    }
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

// returns whether the given file or directory exists or not
func fileExists(path string) bool {
    _, err := os.Stat(path)
    return err == nil
}

/****************************************************************/

// As these SavedX objects will be unmarshaled, make relevant elements pointers
// so that if an element is missing it will be nil (instead of 0, "", false etc)

type SavedDrawnForm struct {
    Id      uint     `json:"id"`
    X       *int     `json:"x"`  // x position
    R       *int     `json:"r"`  // radialModifier
    Rm      bool     `json:"rm,omitempty"` // remove
}

type SavedDrawnFormSortId []*SavedDrawnForm
func (df SavedDrawnFormSortId) Len() int           { return len(df) }
func (df SavedDrawnFormSortId) Less(i, j int) bool { return df[i].Id < df[j].Id }
func (df SavedDrawnFormSortId) Swap(i, j int)      { df[i], df[j] = df[j], df[i] }

type SavedMultiGraph struct {
    Name    string            `json:"name"`
    Ind     *uint             `json:"ind"` // index of graph in array
    Drawn   []*SavedDrawnForm `json:"drawn"`
    Rm      bool              `json:"rm,omitempty"` // remove
}

type SavedMultiGraphSliceSortInd []SavedMultiGraph
func (mg SavedMultiGraphSliceSortInd) Len() int           { return len(mg) }
func (mg SavedMultiGraphSliceSortInd) Less(i, j int) bool { return *mg[i].Ind < *mg[j].Ind }
func (mg SavedMultiGraphSliceSortInd) Swap(i, j int)      { mg[i], mg[j] = mg[j], mg[i] }

type SavedTag struct {
    Name    string    `json:"name"`
    Ind     *uint     `json:"ind"` // index of graph in array
    Blob    *bool     `json:"blob"`
    Halo    *bool     `json:"halo"`
    Ids     []uint    `json:"ids"`
    Rm      bool      `json:"rm,omitempty"` // remove
}

type SavedTagSliceSortIndex []SavedTag
func (ts SavedTagSliceSortIndex) Len() int           { return len(ts) }
func (ts SavedTagSliceSortIndex) Less(i, j int) bool { return *ts[i].Ind < *ts[j].Ind }
func (ts SavedTagSliceSortIndex) Swap(i, j int)      { ts[i], ts[j] = ts[j], ts[i] }


type SavedNote struct {
    Id      uint    `json:"id"`
    Notes   *string `json:"notes"`
    Rm      bool    `json:"rm,omitempty"` // remove
}

type SavedNoteSliceSortId []SavedNote
func (sn SavedNoteSliceSortId) Len() int           { return len(sn) }
func (sn SavedNoteSliceSortId) Less(i, j int) bool { return sn[i].Id < sn[j].Id }
func (sn SavedNoteSliceSortId) Swap(i, j int)      { sn[i], sn[j] = sn[j], sn[i] }


// NOTE: If you change this, also change the default entry for new user row in ProfileRegister
type SavedUserSettings struct {
    // Paper Vertical Ordering:
    Pvo     *uint    `json:"pvo"`
    // New papers if this many Days Ago
    Nda     *uint    `json:"nda"`
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
    numCites   uint     // number of times cited
    dNumCites1 uint     // change in numCites in past day
    dNumCites5 uint     // change in numCites in past 5 days
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

type IdSliceSort []uint
func (id IdSliceSort) Len() int           { return len(id) }
func (id IdSliceSort) Less(i, j int) bool { return id[i] < id[j] }
func (id IdSliceSort) Swap(i, j int)      { id[i], id[j] = id[j], id[i] }


/****************************************************************/

func MergeSavedNotes (diffSavedNotes []SavedNote, oldSavedNotes []SavedNote) []SavedNote {
    // Merge oldNotes with diff
    for _, diffNote := range diffSavedNotes {
        // try to find oldNote match
        var oldNotePtr *SavedNote
        for i, _ := range oldSavedNotes {
            if oldSavedNotes[i].Id == diffNote.Id {
                oldNotePtr = &oldSavedNotes[i]
                if diffNote.Rm {
                    // splice
                    if i+1 < len(oldSavedNotes) {
                        oldSavedNotes = append(oldSavedNotes[:i],oldSavedNotes[i+1:]...)
                    } else {
                        oldSavedNotes = oldSavedNotes[:i]
                    }
                }
                break
            }
        }
        if diffNote.Rm { continue }
        // Check if diff specified
        if oldNotePtr == nil {
            oldSavedNotes = append(oldSavedNotes,diffNote)
            continue
        }
        // Check if marked for removal
        // Else ### MERGE ###
        if diffNote.Notes != nil {
            oldNotePtr.Notes = diffNote.Notes
        }
    }
    sort.Sort(SavedNoteSliceSortId(oldSavedNotes))
    return oldSavedNotes
}

func MergeSavedMultiGraphs (diffSavedMultiGraphs []SavedMultiGraph, oldSavedMultiGraphs []SavedMultiGraph) []SavedMultiGraph {
    // Merge oldMultiGraphs with diff
    for _, diffMultiGraph := range diffSavedMultiGraphs {
        // try to find oldMultiGraph match
        var oldMultiGraphPtr *SavedMultiGraph
        for i, _ := range oldSavedMultiGraphs {
            if oldSavedMultiGraphs[i].Name == diffMultiGraph.Name {
                oldMultiGraphPtr = &oldSavedMultiGraphs[i]
                // Check if marked for removal
                if diffMultiGraph.Rm {
                    // splice
                    if i+1 < len(oldSavedMultiGraphs) {
                        oldSavedMultiGraphs = append(oldSavedMultiGraphs[:i],oldSavedMultiGraphs[i+1:]...)
                    } else {
                        oldSavedMultiGraphs = oldSavedMultiGraphs[:i]
                    }
                }
                break
            }
        }
        if diffMultiGraph.Rm { continue }
        // if diff doesn't exist yet, add it to the list
        if oldMultiGraphPtr == nil {
            oldSavedMultiGraphs = append(oldSavedMultiGraphs,diffMultiGraph)
            continue
        }
        // ### MERGE ###
        if diffMultiGraph.Ind != nil {
            oldMultiGraphPtr.Ind = diffMultiGraph.Ind
        }
        if len(diffMultiGraph.Drawn) > 0 {
            for _, diffDrawnForm := range diffMultiGraph.Drawn {
                // try to find oldDrawnForm match
                var oldDrawnFormPtr *SavedDrawnForm
                for i, odf := range oldMultiGraphPtr.Drawn {
                    if odf.Id == diffDrawnForm.Id {
                        oldDrawnFormPtr = odf
                        // Check if marked for removal
                        if diffDrawnForm.Rm {
                            if i+1 < len(oldMultiGraphPtr.Drawn) {
                                oldMultiGraphPtr.Drawn = append(oldMultiGraphPtr.Drawn[:i],oldMultiGraphPtr.Drawn[i+1:]...)
                            } else {
                                oldMultiGraphPtr.Drawn = oldMultiGraphPtr.Drawn[:i]
                            }
                        }
                        break
                    }
                }
                if diffDrawnForm.Rm { continue }
                // if diff doesn't exist yet, add it to the list
                if oldDrawnFormPtr == nil {
                    oldMultiGraphPtr.Drawn = append(oldMultiGraphPtr.Drawn,diffDrawnForm)
                    continue
                }
                // merge DrawnForm
                if diffDrawnForm.X != nil {
                    oldDrawnFormPtr.X = diffDrawnForm.X
                }
                if diffDrawnForm.R != nil {
                    oldDrawnFormPtr.R = diffDrawnForm.R
                }
            }
            // TODO sort
            sort.Sort(SavedDrawnFormSortId(oldMultiGraphPtr.Drawn))
        }
    }
    sort.Sort(SavedMultiGraphSliceSortInd(oldSavedMultiGraphs))
    return oldSavedMultiGraphs
}

func MergeSavedTags (diffSavedTags []SavedTag, oldSavedTags []SavedTag) []SavedTag {
    // Merge oldTags with diff
    for _, diffTag := range diffSavedTags {
        // try to find diffTag match
        var oldTagPtr *SavedTag
        for i, _ := range oldSavedTags {
            if oldSavedTags[i].Name == diffTag.Name {
                oldTagPtr = &oldSavedTags[i]
                if diffTag.Rm {
                    if i+1 < len(oldSavedTags) {
                        oldSavedTags = append(oldSavedTags[:i],oldSavedTags[i+1:]...)
                    } else {
                        oldSavedTags = oldSavedTags[:i]
                    }
                }
                break
            }
        }
        if diffTag.Rm { continue }
        // if diff doesn't exist yet, add it to the list
        if oldTagPtr == nil {
            oldSavedTags = append(oldSavedTags,diffTag)
            continue
        }
        // Check if marked for removal
        // ### MERGE ###
        if diffTag.Blob != nil {
            oldTagPtr.Blob = diffTag.Blob
        }
        if diffTag.Halo != nil {
            oldTagPtr.Halo = diffTag.Halo
        }
        if diffTag.Ind != nil {
            oldTagPtr.Ind = diffTag.Ind
        }
        if len(diffTag.Ids) > 0 {
            // compare IDs
            // Sending an ID with a diff object 'toggles' it on the server
            // e.g if sent ID already exists, it is removed, and vice versa
            // This is safe because 'old' and 'new' hashes must also match
            oldTagPtr.Ids = append(oldTagPtr.Ids,diffTag.Ids...)
            sort.Sort(IdSliceSort(oldTagPtr.Ids))
            // now remove any id appearing more than once
            prevSet := true
            for i:= len(oldTagPtr.Ids)-2; i>= 0; i = i-1 {
                if prevSet && oldTagPtr.Ids[i] == oldTagPtr.Ids[i+1] {
                    if (i+2 < len(oldTagPtr.Ids)) {
                        oldTagPtr.Ids = append(oldTagPtr.Ids[:i], oldTagPtr.Ids[i+2:]...)
                    } else {
                        oldTagPtr.Ids = oldTagPtr.Ids[:i]
                    }
                    prevSet = false
                } else {
                    prevSet = true
                }
            }
        }
    }
    sort.Sort(SavedTagSliceSortIndex(oldSavedTags))
    return oldSavedTags
}

func MergeSavedSettings(diffSettings SavedUserSettings, oldSettings SavedUserSettings) SavedUserSettings {
    // ### MERGE ###
    if diffSettings.Pvo != nil {
        oldSettings.Pvo = diffSettings.Pvo
    }
    if diffSettings.Nda != nil {
        oldSettings.Nda = diffSettings.Nda
    }
    return oldSettings
}

/****************************************************************/

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
            log.Printf("MySQL statement error; %s\n", err)
            success = false
        } else if eof {
            //fmt.Println("MySQL statement error; eof")
            // Row just didn't exist, return false but don't print error
            success = false
        }
        err = stmt.FreeResult()
        if err != nil {
            log.Printf("MySQL statement error; %s\n", err)
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
        query = fmt.Sprintf("SELECT id,arxiv,maincat,allcats,inspire,authors,title,publ FROM meta_data WHERE id = %d", id)
    } else if len(arxiv) > 0 {
        // security issue: should make sure arxiv string is sanitised
        query = fmt.Sprintf("SELECT id,arxiv,maincat,allcats,inspire,authors,title,publ FROM meta_data WHERE arxiv = '%s'", arxiv)
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
    if row[1] != nil {
        if paper.arxiv, ok = row[1].(string); !ok { return nil }
    }
    if row[2] != nil {
        if paper.maincat, ok = row[2].(string); !ok { return nil }
    }
    if paper.allcats, ok = row[3].(string); !ok { paper.allcats = "" }
    if row[4] != nil {
        if inspire, ok := row[4].(uint64); ok { paper.inspire = uint(inspire); }
    }
    if row[5] == nil {
        paper.authors = "(unknown authors)"
    } else if au, ok := row[5].([]byte); !ok {
        log.Printf("ERROR: cannot get authors for id=%d; %v\n", paper.id, row[5])
        return nil
    } else {
        paper.authors = string(au)
    }
    if row[6] == nil {
        paper.authors = "(unknown title)"
    } else if title, ok := row[6].(string); !ok {
        log.Printf("ERROR: cannot get title for id=%d; %v\n", paper.id, row[6])
        return nil
    } else {
        paper.title = title
    }
    if row[7] == nil {
        paper.publJSON = "";
    } else if publ, ok := row[7].(string); !ok {
        log.Printf("ERROR: cannot get publ for id=%d; %v\n", paper.id, row[7])
        paper.publJSON = "";
    } else {
        publ2 := string(publ) // convert to string so marshalling does the correct thing
        publ3, _ := json.Marshal(publ2)
        paper.publJSON = string(publ3)
    }

    //// Get number of times cited, and change in number of cites
    query = fmt.Sprintf("SELECT numCites,dNumCites1,dNumCites5 FROM pcite WHERE id = %d", paper.id)
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
    } else {
        log.Printf("ERROR: cannot get pcite data for id=%d\n", paper.id)
    }

    papers.QueryEnd()

    return paper
}

func Sha1 (str string) string {
    hash := sha1.New()
    io.WriteString(hash, fmt.Sprintf("%s", string(str)))
    return fmt.Sprintf("%x",hash.Sum(nil))
}

func Sha256 (str string) string {
    hash := sha256.New()
    io.WriteString(hash, fmt.Sprintf("%s", string(str)))
    return fmt.Sprintf("%x",hash.Sum(nil))
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

func GenerateUserPassword() (string, int, int64, string) {
    password := GenerateRandString(8,8)
    salt := rand.Int63()
    // hash+salt password
    pwdversion := 2 // password hashing strength
    var userhash string
    userhash = Sha256(fmt.Sprintf("%s%d", Sha256(Sha1(password)), salt))
    return password, pwdversion, salt, userhash
}

func ReadAndReplaceFromFile(path string, dict map[string]string) (message string, err error) {
    var data []byte
    data, err = ioutil.ReadFile(path)

    message = string(data)
    for key, val := range dict {
        message = strings.Replace(message,key,val,-1)
    }

    return
}

func SendPscpMail(message string, usermail string) {
    
    var (
        c *smtp.Client
        err error
    )

    //fmt.Println(string(message))

    // Connect to the local SMTP server.
    c, err = smtp.Dial("127.0.0.1:25")
    if err != nil {
        log.Printf("Error: %s",err)
        return
    }
    // Set the sender and recipient.
    c.Mail("noreply@paperscape.org")
    c.Rcpt(usermail)
    // Send the email body.
    wc, err := c.Data()
    if err != nil {
        log.Printf("Error: %s",err)
        return
    }
    defer wc.Close()
    buf := bytes.NewBufferString(message)
    if _, err = buf.WriteTo(wc); err != nil {
        log.Printf("Error: %s",err)
        return
    }

    //auth := smtp.PlainAuth("", "email", "password","smtp.foo.com")
    //err := smtp.SendMail("smtp.foo.com:25", auth,"noreply@paperscape.org", []string{usermail}, w.Bytes())
    //if err != nil {
    //  fmt.Println("ERROR: ProfileRequestResetPassword sendmail:", err)
    //}
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

    /* uncomment this to disable inspire 999 information
    if ((paper.id % 15625) % 4) == 2 {
        // this is an inspire paper
        if isRefStr {
            // don't process inspire references
            // this acts as though we don't have the 999 information
            return true
        }
    }
    */

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

func (papers *PapersEnv) QueryRefs(paper *Paper, queryRefsMeta bool) {
    if paper == nil { return }

    // check if refs already exist
    if len(paper.refs) != 0 { return }

    // perform query
    query := fmt.Sprintf("SELECT refs FROM pcite WHERE id = %d", paper.id)
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
    query := fmt.Sprintf("SELECT cites FROM pcite WHERE id = %d", paper.id)
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

type gzipResponseWriter struct {
    io.Writer
    http.ResponseWriter
    sniffDone bool
}

func (w gzipResponseWriter) Write(b []byte) (int, error) {
    if !w.sniffDone {
        if w.Header().Get("Content-Type") == "" {
            w.Header().Set("Content-Type", http.DetectContentType(b))
        }
        w.sniffDone = true
    }
    return w.Writer.Write(b)
}

func (h *MyHTTPHandler) ServeHTTP(rwIn http.ResponseWriter, req *http.Request) {
    req.ParseForm()

    var rw *MyResponseWriter
    if !strings.Contains(req.Header.Get("Accept-Encoding"), "gzip") {
        rw = &MyResponseWriter{rwIn, 0}
    } else {
        rwIn.Header().Set("Content-Encoding", "gzip")
        gz := gzip.NewWriter(rwIn)
        defer gz.Close()
        rw = &MyResponseWriter{gzipResponseWriter{Writer: gz, ResponseWriter: rwIn},0}
    }

    var logDescription string

    if req.Form["callback"] != nil {
        // construct a JSON object to return
        rw.Header().Set("Content-Type", "application/json")
        callback := req.Form["callback"][0]
        fmt.Fprintf(rw, "%s({\"r\":", callback)
        resultBytesStart := rw.bytesWritten

        if req.Form["test"] != nil {
            fmt.Fprintf(rw, "{\"test\":\"success\", \"POST\":false}")
        //} else if req.Form["mload"] != nil {
        //    // map: load world dims and latest id
        //    logDescription = fmt.Sprintf("Load world map")
        //    h.MapLoadWorld(rw) 
        } else if req.Form["mp2l[]"] != nil {
            // map: paper ids to locations
            var ids []uint
            for _, strId := range req.Form["mp2l[]"] {
                if preId, er := strconv.ParseUint(strId, 10, 0); er == nil {
                    ids = append(ids, uint(preId))
                } else {
                    log.Printf("ERROR: can't convert id '%s'; skipping\n", strId)
                }
            }
            logDescription = fmt.Sprintf("Paper ids to map locations for")
            h.MapLocationFromPaperId(ids,rw)
        } else if req.Form["ml2p[]"] != nil {
            // map: location to paper id
            logDescription = fmt.Sprintf("Paper id from map location")
            x, _ := strconv.ParseFloat(req.Form["ml2p[]"][0], 0)
            y, _ := strconv.ParseFloat(req.Form["ml2p[]"][1], 0)
            h.MapPaperIdAtLocation(x,y,rw)
        //} else if req.Form["mkws[]"] != nil {
        //    // map keywords
        //    logDescription = fmt.Sprintf("Keywords for map window")
        //    x, _ := strconv.ParseInt(req.Form["mkws[]"][0],10, 0)
        //    y, _ := strconv.ParseInt(req.Form["mkws[]"][1],10, 0)
        //    width, _ := strconv.ParseInt(req.Form["mkws[]"][2],10, 0)
        //    height, _ := strconv.ParseInt(req.Form["mkws[]"][3],10, 0)
        //    h.MapKeywordsInWindow(x,y,width,height,rw)
        } else if req.Form["pchal"] != nil {
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
            success := h.ProfileChallenge(req.Form["pchal"][0], giveSalt, giveVersion, rw)
            if !success {
                logDescription = fmt.Sprintf("pchal %s",req.Form["pchal"][0])
            }
        } else if req.Form["pload"] != nil && req.Form["h"] != nil {
            // profile-load: either login request or load request from an autosave
            // h = passHash, nh = notessHash, gh = graphsHash, th = tagsHash, sh = settingshash
            var nh, gh, th, sh string
            if req.Form["nh"] != nil { nh = req.Form["nh"][0] }
            if req.Form["gh"] != nil { gh = req.Form["gh"][0] }
            if req.Form["th"] != nil { th = req.Form["th"][0] }
            if req.Form["sh"] != nil { sh = req.Form["sh"][0] }
            loaded, success := h.ProfileLoad(req.Form["pload"][0], req.Form["h"][0], nh, gh, th, sh, rw)
            if loaded || !success {
                logDescription = fmt.Sprintf("pload %s",req.Form["pload"][0])
            }
        } else if req.Form["pchpw"] != nil && req.Form["h"] != nil && req.Form["p"] != nil && req.Form["s"] != nil && req.Form["pv"] != nil {
            // profile-change-password: change password request
            // h = passHash, p = payload, s = sprinkle (salt), pv = password version
            h.ProfileChangePassword(req.Form["pchpw"][0], req.Form["h"][0], req.Form["p"][0], req.Form["s"][0], req.Form["pv"][0], rw)
            logDescription = fmt.Sprintf("pchpw %s pv=%s",req.Form["pchpw"][0],req.Form["pv"][0])
        } else if req.Form["prrp"] != nil {
            // profile-request-reset-password: request reset link sent to email
            h.ProfileRequestResetPassword(req.Form["prrp"][0], rw)
            logDescription = fmt.Sprintf("prrp %s",req.Form["prrp"][0])
        } else if req.Form["prpw"] != nil {
            // profile-reset-password: resets password request and sends new one to email
            h.ProfileResetPassword(req.Form["prpw"][0], rw)
            logDescription = fmt.Sprintf("prpw %s",req.Form["prpw"][0])
        } else if req.Form["preg"] != nil {
            // profile-register: register email address as new user 
            h.ProfileRegister(req.Form["preg"][0], rw)
            logDescription = fmt.Sprintf("preg %s",req.Form["preg"][0])
        } else if req.Form["lload"] != nil {
            // link-load: from a page load
            h.LinkLoad(req.Form["lload"][0], rw)
            logDescription = fmt.Sprintf("lload %s",req.Form["lload"][0])
        } else if req.Form["gdb"] != nil {
            // get-date-boundaries
            success := h.GetDateBoundaries(rw)
            if !success {
                logDescription = fmt.Sprintf("gdb")
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
            logDescription = fmt.Sprintf("gdata (%d)",len(req.Form["gdata[]"]))
        } else if req.Form["chids"] != nil && (req.Form["arx[]"] != nil ||  req.Form["doi[]"] != nil || req.Form["jrn[]"] != nil) {
            // convert-human-ids: convert human IDs to internal IDs
            // arx: list of arxiv IDs
            // jrn: list of journal IDs
            h.ConvertHumanToInternalIds(req.Form["arx[]"],req.Form["doi[]"],req.Form["jrn[]"], rw)
            logDescription = fmt.Sprintf("chids (%d,%d,%d)",len(req.Form["arx[]"]),len(req.Form["doi[]"]),len(req.Form["jrn[]"]))
        } else if req.Form["sge"] != nil {
            // search-general: do fulltext search of authors and titles
            h.SearchGeneral(req.Form["sge"][0], rw)
            logDescription = fmt.Sprintf("sge \"%s\"",req.Form["sge"][0])
        } else if req.Form["skw"] != nil {
            // search-keyword: do fulltext search of keywords
            h.SearchKeyword(req.Form["skw"][0], rw)
            logDescription = fmt.Sprintf("skw \"%s\"",req.Form["skw"][0])
        } else if req.Form["sax"] != nil {
            // search-arxiv: search papers for arxiv number
            h.SearchArxiv(req.Form["sax"][0], rw)
            logDescription = fmt.Sprintf("sax \"%s\"",req.Form["sax"][0])
        } else if req.Form["saxm"] != nil {
            // search-arxiv-minimal: search papers for arxiv number
            // returning minimal information
            h.SearchArxivMinimal(req.Form["saxm"][0], rw)
            logDescription = fmt.Sprintf("saxm \"%s\"",req.Form["saxm"][0])
        } else if req.Form["sau"] != nil {
            // search-author: search papers for authors
            h.SearchAuthor(req.Form["sau"][0], rw)
            logDescription = fmt.Sprintf("sau \"%s\"",req.Form["sau"][0])
        } else if req.Form["sti"] != nil {
            // search-title: search papers for words in title
            h.SearchTitle(req.Form["sti"][0], rw)
            logDescription = fmt.Sprintf("sti \"%s\"",req.Form["sti"][0])
        } else if req.Form["sca"] != nil && req.Form["f"] != nil && req.Form["t"] != nil {
            // search-category: search papers between given id range, in given category
            // x = include cross lists, f = from, t = to
            h.SearchCategory(req.Form["sca"][0], req.Form["x"] != nil && req.Form["x"][0] == "true", req.Form["f"][0], req.Form["t"][0], rw)
            logDescription = fmt.Sprintf("sca \"%s\" (%d,%d)",req.Form["sca"][0],req.Form["f"][0],req.Form["t"][0])
        } else if req.Form["snp"] != nil && req.Form["f"] != nil && req.Form["t"] != nil {
            // search-new-papers: search papers between given id range
            // f = from, t = to
            h.SearchNewPapers(req.Form["f"][0], req.Form["t"][0], rw)
            logDescription = fmt.Sprintf("snp (%d,%d)",req.Form["f"][0],req.Form["t"][0])
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
            logDescription = fmt.Sprintf("str \"%s\"",buf.String())
        } else {
            // unknown ajax request
            logDescription = fmt.Sprintf("unknown")
        }

        if rw.bytesWritten == resultBytesStart {
            // if no result written, write the null object
            fmt.Fprintf(rw, "null")
        }

        // tail of JSON object
        fmt.Fprintf(rw, "})\n")
    } else if req.Method == "POST" && req.Form["echo"] != nil && req.Form["fn"] != nil {
        // POST - echo file so that it can be saved
        rw.Header().Set("Access-Control-Allow-Origin", "*") // for cross domain POSTing; see https://developer.mozilla.org/en/http_access_control
        rw.Header().Set("Cache-Control", "public")
        rw.Header().Set("Content-Description", "File Transfer")
        rw.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s",req.Form["fn"][0]))
        rw.Header().Set("Content-Type", "application/octet-stream")
        fmt.Fprintf(rw, req.Form["echo"][0])
        logDescription = fmt.Sprintf("echo \"%s\"",req.Form["fn"][0])
    } else if req.Method == "POST" {
        // POST verb

        // construct a JSON object to return
        rw.Header().Set("Access-Control-Allow-Origin", "*") // for cross domain POSTing; see https://developer.mozilla.org/en/http_access_control
        rw.Header().Set("Content-Type", "application/json")
        fmt.Fprintf(rw, "{\"r\":")
        resultBytesStart := rw.bytesWritten

        if req.Form["test"] != nil {
            fmt.Fprintf(rw, "{\"test\":\"success\", \"POST\":true}")
        } else if req.Form["psync"] != nil && req.Form["h"] != nil && req.Form["nh"] != nil && req.Form["gh"] != nil && req.Form["th"] != nil && req.Form["sh"] != nil {
            // profile-sync: sync request
            // h = passHash, n = notesdiff, g = graphsdiff, t = tagsdiff, s = settingsdiff (and the end result hashes)
            var n, g, t, s string
            if req.Form["n"] != nil { n = req.Form["n"][0] }
            if req.Form["g"] != nil { g = req.Form["g"][0] }
            if req.Form["t"] != nil { t = req.Form["t"][0] }
            if req.Form["s"] != nil { s = req.Form["s"][0] }
            h.ProfileSync(req.Form["psync"][0], req.Form["h"][0], n, req.Form["nh"][0], g, req.Form["gh"][0], t, req.Form["th"][0], s, req.Form["sh"][0], rw)
            logDescription = fmt.Sprintf("psync %s",req.Form["psync"][0])
        } else if req.Form["lsave"] != nil {
            // link-save: existing code (or empty string if none)
            // n = notes, nh = notes hash, g = graphs, gh = graphs hash, t = tags, th = tags hash
            h.LinkSave(req.Form["lsave"][0], req.Form["n"][0], req.Form["nh"][0], req.Form["g"][0], req.Form["gh"][0], req.Form["t"][0], req.Form["th"][0], rw)
            var descSaveHash string
            if req.Form["lsave"][0] != "" {
                descSaveHash = req.Form["lsave"][0]
            } else {
                descSaveHash = "<new>"
            }
            logDescription = fmt.Sprintf("lsave %s",descSaveHash)
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
            logDescription = fmt.Sprintf("gdata (%d)",len(req.Form["gdata[]"]))
        } else if req.Form["mp2l[]"] != nil {
            // map: paper ids to locations
            var ids []uint
            for _, strId := range req.Form["mp2l[]"] {
                if preId, er := strconv.ParseUint(strId, 10, 0); er == nil {
                    ids = append(ids, uint(preId))
                } else {
                    log.Printf("ERROR: can't convert id '%s'; skipping\n", strId)
                }
            }
            logDescription = fmt.Sprintf("Paper ids to map locations for")
            h.MapLocationFromPaperId(ids,rw)
        } else {
            // unknown ajax request
            logDescription = fmt.Sprintf("unknown")
        }

        if rw.bytesWritten == resultBytesStart {
            // if no result written, write the null object
            fmt.Fprintf(rw, "null")
        }

        // tail of JSON object
        fmt.Fprintf(rw, "}\n")
    } else {
        // unknown request, return html
        fmt.Fprintf(rw, "<html><head></head><body><p>Unknown request</p></body>\n")
    }

    //fmt.Printf("[%s] %s -- %s %s (bytes: %d URL, %d content, %d replied)\n", time.Now().Format(time.RFC3339), req.RemoteAddr, req.Method, req.URL, len(req.URL.String()), req.ContentLength, rw.bytesWritten)
    if logDescription != "" {
        log.Printf("%s -- %s %s -- bytes: %d URL, %d content, %d replied\n", req.RemoteAddr, req.Method, logDescription, len(req.URL.String()), req.ContentLength, rw.bytesWritten)
    }

    runtime.GC()
}

/****************************************************************/

func PrintJSONMetaInfo(w io.Writer, paper *Paper) {
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
    fmt.Fprintf(w, "\"auth\":%s,\"titl\":%s,\"nc\":%d,\"dnc1\":%d,\"dnc5\":%d", authorsJSON, titleJSON, paper.numCites, paper.dNumCites1, paper.dNumCites5)
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
func PrintJSONUpdateMetaInfo(w io.Writer, paper *Paper) {
    //fmt.Fprintf(w, "{\"id\":%d,\"nc\":%d,\"dnc1\":%d,\"dnc5\":%d", paper.id, paper.numCites, paper.dNumCites1, paper.dNumCites5)
    fmt.Fprintf(w, "\"nc\":%d,\"dnc1\":%d,\"dnc5\":%d", paper.numCites, paper.dNumCites1, paper.dNumCites5)
    if len(paper.publJSON) > 0 {
        fmt.Fprintf(w, ",\"publ\":%s", paper.publJSON)
    }
}

func PrintJSONRelevantRefs(w io.Writer, paper *Paper, paperList []*Paper) {
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
                PrintJSONLinkPastInfo(w, link)
                break
            }
        }
    }
    fmt.Fprintf(w, "]")
}

func PrintJSONLinkPastInfo(w io.Writer, link *Link) {
    fmt.Fprintf(w, "{\"id\":%d,\"o\":%d,\"f\":%d,\"nc\":%d}", link.pastId, link.refOrder, link.refFreq, link.pastCited)
}

func PrintJSONLinkFutureInfo(w io.Writer, link *Link) {
    fmt.Fprintf(w, "{\"id\":%d,\"o\":%d,\"f\":%d,\"nc\":%d}", link.futureId, link.refOrder, link.refFreq, link.futureCited)
}

func PrintJSONAllRefs(w io.Writer, paper *Paper) {
    fmt.Fprintf(w, "\"allr\":true,\"ref\":[")
    // output the refs (future -> past)
    for i, link := range paper.refs {
        if i > 0 {
            fmt.Fprintf(w, ",")
        }
        PrintJSONLinkPastInfo(w, link)
    }
    fmt.Fprintf(w, "]")
}

func PrintJSONAllCites(w io.Writer, paper *Paper, dateBoundary uint) {
    fmt.Fprintf(w, "\"allc\":true,\"cite\":[")
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

/****************************************************************/

/* SetChallenge */
func (h *MyHTTPHandler) SetChallenge(usermail string) (challenge int64, success bool) {
    // generate random "challenge" code
    success = false
    challenge = rand.Int63()

    stmt := h.papers.StatementBegin("UPDATE userdata SET challenge = ? WHERE usermail = ?",challenge,h.papers.db.Escape(usermail))
    if !h.papers.StatementEnd(stmt) {
        return
    }
    success = true
    return
}

/* ProfileChallenge */
/* check usermail exists and get the 'salt' and/or 'version' */
func (h *MyHTTPHandler) ProfileChallenge(usermail string, giveSalt bool, giveVersion bool, rw http.ResponseWriter) (success bool) {
    var salt uint64
    var pwdversion uint64

    success = false

    stmt := h.papers.StatementBegin("SELECT salt,pwdversion FROM userdata WHERE usermail = ?",h.papers.db.Escape(usermail))
    if !h.papers.StatementBindSingleRow(stmt,&salt,&pwdversion) {
        fmt.Fprintf(rw, "false")
        return
    }

    // generate random "challenge" code
    challenge, ok := h.SetChallenge(usermail)
    if ok != true {
        fmt.Fprintf(rw, "false")
        return
    }

    // return challenge code
    fmt.Fprintf(rw, "{\"name\":\"%s\",\"chal\":\"%d\"",usermail, challenge);
    if giveSalt {
        fmt.Fprintf(rw, ",\"salt\":\"%d\"", salt)
    }
    if giveVersion {
        fmt.Fprintf(rw, ",\"pwdv\":\"%d\"", uint(pwdversion))
    }
    fmt.Fprintf(rw, "}")
    
    success = true
    return
}

/* ProfileAuthenticate */
func (h *MyHTTPHandler) ProfileAuthenticate(usermail string, passhash string) (success bool) {
    success = false

    // Check for valid usermail and get the user challenge and hash
    var challenge uint64
    var userhash string

    stmt := h.papers.StatementBegin("SELECT challenge,userhash FROM userdata WHERE usermail = ?",h.papers.db.Escape(usermail))
    if !h.papers.StatementBindSingleRow(stmt,&challenge,&userhash) {
        return
    }

    // Check the passhash!
    tryhash := Sha256(fmt.Sprintf("%s%d", userhash, challenge))

    if passhash != tryhash {
        log.Printf("ERROR: ProfileAuthenticate for '%s' - invalid password:  %s vs %s\n", usermail, passhash, tryhash)
        return
    }

    // we're THROUGH!!
    //log.Printf("Succesfully authenticated user '%s'\n",usermail)
    success = true
    return
}

/* ProfileChangePassword */
func (h *MyHTTPHandler) ProfileChangePassword(usermail string, passhash string, newhash string, salt string, pwdversion string, rw http.ResponseWriter) {
    if !h.ProfileAuthenticate(usermail,passhash) {
        return
    }

    pwdvNum, _ := strconv.ParseUint(pwdversion, 10, 64)
    saltNum, _ := strconv.ParseUint(salt, 10, 64)

    // decrypt newhash
    //var userhash []byte
    //stmt := h.papers.StatementBegin("SELECT userhash FROM userdata WHERE usermail = ?",h.papers.db.Escape(usermail))
    //if !h.papers.StatementBindSingleRow(stmt,&userhash) {
    //  return
    //}
    // convert userhash to 32 byte key
    //fmt.Printf("length of userhash %d\n", len(userhash))
    //cipher, err := aes.NewCipher(userhash[:16])
    //if err != nil {
    //  fmt.Printf("ERROR: for user %s, could not create aes cipher to decrypt new password\n", usermail)
    //}
    //output := make([]byte);
    //cipher.Decrypt([]byte(newhash),output)


    success := true
    stmt := h.papers.StatementBegin("UPDATE userdata SET userhash = ?, salt = ?, pwdversion = ? WHERE usermail = ?", h.papers.db.Escape(newhash), uint64(saltNum), uint64(pwdvNum), h.papers.db.Escape(usermail))
    if !h.papers.StatementEnd(stmt) {
        success = false
    }

    fmt.Fprintf(rw, "{\"succ\":\"%t\",\"salt\":\"%d\",\"pwdv\":\"%d\"}",success,uint64(saltNum),uint64(pwdvNum))
}

/* ProfileRequestResetPassword */
func (h *MyHTTPHandler) ProfileRequestResetPassword(usermail string, rw http.ResponseWriter) {

    // check if email address exists
    var foo string
    stmt := h.papers.StatementBegin("SELECT usermail FROM userdata WHERE usermail = ?", h.papers.db.Escape(usermail))
    if !h.papers.StatementBindSingleRow(stmt,&foo) {
        // it doesn't ...
        fmt.Fprintf(rw, "{\"succ\":\"false\"}")
        return
    }

    // generate resetcode for user
    // the code is 64 characters long, we also record the time it was made
    // so that it can expire after one hour
    // just so this can't crash server, only try it N times
    resetcode := ""
    N := 50
    for i := 0; i < N; i++ {
        code := GenerateRandString(64,64)
        // ensure resetcode is unique (no other user has this resetcode currently set)
        stmt := h.papers.StatementBegin("SELECT usermail FROM userdata WHERE resetcode = ?", code)
        if !h.papers.StatementBindSingleRow(stmt,&foo) {
            resetcode = code
            break
        }
    }
    if resetcode == "" {
        log.Printf("ERROR: ProfileRequestResetPassword couldn't generate a resetcode in %d tries!\n",N)
        return
    }
    stmt = h.papers.StatementBegin("UPDATE userdata SET resetcode = ?, resettime = NOW() WHERE usermail = ?", resetcode, h.papers.db.Escape(usermail))
    if !h.papers.StatementEnd(stmt) {
        return
    }

    dict := make(map[string]string)
    dict["@@USERMAIL@@"] = usermail
    dict["@@RESETCODE@@"] = resetcode
    message, _ := ReadAndReplaceFromFile("pwd_reset_request.email",dict)

    SendPscpMail(message,usermail)

    fmt.Fprintf(rw, "{\"succ\":\"true\"}")
}

/* ProfileResetPassword */
func (h *MyHTTPHandler) ProfileResetPassword(resetcode string, rw http.ResponseWriter) {

    // check if the resetcode exists and hasn't expired (must be less than an hour old!)
    var usermail string
    stmt := h.papers.StatementBegin("SELECT usermail FROM userdata WHERE resetcode = ? AND resettime > DATE_SUB(NOW(), INTERVAL 1 HOUR)", h.papers.db.Escape(resetcode))
    if !h.papers.StatementBindSingleRow(stmt,&usermail) {
        // it doesn't ...
        fmt.Fprintf(rw, "{\"succ\":\"false\"}")
        return
    }

    // Reset password for usermail
    password, pwdversion, salt, userhash := GenerateUserPassword()

    // load new password into, and remove resetcode 
    stmt = h.papers.StatementBegin("UPDATE userdata SET userhash = ?, salt = ?, pwdversion = ?, resetcode = NULL WHERE usermail = ?", userhash, salt, pwdversion, usermail)
    if !h.papers.StatementEnd(stmt) {
        return
    }

    dict := make(map[string]string)
    dict["@@USERMAIL@@"] = usermail
    dict["@@PASSWORD@@"] = password
    message, _ := ReadAndReplaceFromFile("pwd_reset.email",dict)

    SendPscpMail(message,usermail)

    fmt.Fprintf(rw, "{\"succ\":\"true\"}")
}

/* ProfileRegister */
func (h *MyHTTPHandler) ProfileRegister(usermail string, rw http.ResponseWriter) {

    // check if email address exists
    var foo string
    stmt := h.papers.StatementBegin("SELECT usermail FROM userdata WHERE usermail = ?", h.papers.db.Escape(usermail))
    if h.papers.StatementBindSingleRow(stmt,&foo) {
        // it does ...
        fmt.Fprintf(rw, "{\"succ\":\"false\"}")
        return
    }

    // generate a random password for the user
    password, pwdversion, salt, userhash := GenerateUserPassword()

    // generate empty papers and tags strings etc.
    emptyJSON := "[]"
    settingsJSON := "{\"pvo\":0,\"nda\":1}"

    // create database entry
    stmt = h.papers.StatementBegin("INSERT INTO userdata (usermail,userhash,salt,pwdversion,notes,graphs,tags,settings,lastlogin) VALUES (?,?,?,?,?,?,?,?,NOW())",h.papers.db.Escape(usermail),userhash,salt,pwdversion,emptyJSON,emptyJSON,emptyJSON,settingsJSON)
    if !h.papers.StatementEnd(stmt) {
        fmt.Fprintf(rw, "{\"succ\":\"false\"}")
        return
    }

    dict := make(map[string]string)
    dict["@@USERMAIL@@"] = usermail
    dict["@@PASSWORD@@"] = password
    message, _ := ReadAndReplaceFromFile("user_registration.email",dict)

    SendPscpMail(message,usermail)

    fmt.Fprintf(rw, "{\"succ\":\"true\"}")
}

/* If given papers/tags hashes don't match with db, send user all their papers and tags.
   Login also uses this function by providing empty hashes. */
func (h *MyHTTPHandler) ProfileLoad(usermail string, passhash string, noteshash string, graphshash string, tagshash string, settingshash string, rw http.ResponseWriter) (loaded bool, success bool) {
    
    // used for logging whether user loaded
    loaded = false
    success = false

    if !h.ProfileAuthenticate(usermail,passhash) {
        return
    }

    // generate random "challenge", as we expect user to reply
    // with a sync request if this is an autosave
    challenge, ok := h.SetChallenge(usermail)
    if ok != true {
        return
    }

    var notes,graphs,tags,settings []byte
    stmt := h.papers.StatementBegin("SELECT notes,graphs,tags,settings FROM userdata WHERE usermail = ?",h.papers.db.Escape(usermail))
    if !h.papers.StatementBindSingleRow(stmt,&notes,&graphs,&tags,&settings) {
        return
    }
    noteshashDb := Sha1(string(notes))
    graphshashDb := Sha1(string(graphs))
    tagshashDb := Sha1(string(tags))
    settingshashDb := Sha1(string(settings))

    fmt.Fprintf(rw, "{\"name\":\"%s\",\"chal\":\"%d\",\"nh\":\"%s\",\"gh\":\"%s\",\"th\":\"%s\",\"sh\":\"%s\"",usermail,challenge,noteshashDb,graphshashDb,tagshashDb,settingshashDb)

    // If nonzero hashes given, check if they match those stored in db
    // If so, client can proceed with sync without needing load data,
    // just return the hashes
    if noteshash != "" && graphshash != "" && tagshash != "" && settingshash != "" && noteshashDb == noteshash && graphshashDb == graphshash && tagshashDb == tagshash && settingshashDb == settingshash {
        fmt.Fprintf(rw, "}")
        success = true
        return
    }

    // Either this is a regular login, or hashes didn't match during a sync
    // Either way, proceed as if this were a login
    stmt = h.papers.StatementBegin("UPDATE userdata SET numlogin = numlogin + 1, lastlogin = NOW() WHERE usermail = ?",h.papers.db.Escape(usermail))
    if !h.papers.StatementEnd(stmt) {
        fmt.Fprintf(rw, "}")
        return
    }

    // NOTES
    fmt.Fprintf(rw, ",\"note\":%s",string(notes))

    // GRAPHS
    fmt.Fprintf(rw, ",\"grph\":%s",string(graphs))

    // TAGS
    fmt.Fprintf(rw, ",\"tag\":%s",string(tags))

    // SETTINGS
    fmt.Fprintf(rw, ",\"set\":%s",string(settings))

    // end
    fmt.Fprintf(rw, "}")
    
    success = true
    loaded = true
    return
}

/* Profile Sync */
func (h *MyHTTPHandler) ProfileSync(usermail string, passhash string, diffnotes string, noteshash string, diffgraphs string, graphshash string, difftags string, tagshash string, diffsettings string, settingshash string, rw http.ResponseWriter) {
    if !h.ProfileAuthenticate(usermail,passhash) {
        return
    }

    var notes,graphs,tags,settings string
    var err error

    stmt := h.papers.StatementBegin("SELECT notes,graphs,tags,settings FROM userdata WHERE usermail = ?",h.papers.db.Escape(usermail))
    if !h.papers.StatementBindSingleRow(stmt,&notes,&graphs,&tags,&settings) {
        return
    }

    // NOTES
    // default:
    newNotesJSON := []byte(notes)
    newNoteshash := Sha1(notes)
    // see if diff given:
    if len(diffnotes) > 0 {
        var oldSavedNotes []SavedNote
        err = json.Unmarshal([]byte(notes),&oldSavedNotes)
        if err != nil { log.Printf("Unmarshal error: %s\n",err) }

        var diffSavedNotes []SavedNote
        err = json.Unmarshal([]byte(diffnotes),&diffSavedNotes)
        if err != nil { log.Printf("Unmarshal error: %s\n",err) }

        //fmt.Printf("for user %s, read %d notes from db\n", usermail, len(oldSavedNotes))
        //fmt.Printf("for user %s, read %d diff notes from internets\n", usermail, len(diffSavedNotes))

        // Merge
        newSavedNotes := MergeSavedNotes(diffSavedNotes, oldSavedNotes)
        newNotesJSON, err = json.Marshal(newSavedNotes)
        newNoteshash = Sha1(string(newNotesJSON))
    }
    // compare with hashes we were sent (should match!!)
    if newNoteshash != noteshash {
        log.Printf("Error: for user %s, new sync notes hashes don't match those sent from client: %s vs %s\n", usermail,newNoteshash,noteshash)
        fmt.Fprintf(rw, "{\"succ\":\"false\"}")
        return
    }

    // GRAPHS

    // default:
    newGraphsJSON := []byte(graphs)
    newGraphshash := Sha1(graphs)
    // see if diff given:
    if len(diffgraphs) > 0 {
        var oldSavedGraphs []SavedMultiGraph
        err = json.Unmarshal([]byte(graphs),&oldSavedGraphs)
        if err != nil { log.Printf("Unmarshal error: %s\n",err) }

        var diffSavedGraphs []SavedMultiGraph
        err = json.Unmarshal([]byte(diffgraphs),&diffSavedGraphs)
        if err != nil { log.Printf("Unmarshal error: %s\n",err) }

        //fmt.Printf("for user %s, read %d graphs from db\n", usermail, len(oldSavedGraphs))
        //fmt.Printf("for user %s, read %d diff graphs from internets\n", usermail, len(diffSavedGraphs))

        // Merge
        newSavedGraphs := MergeSavedMultiGraphs(diffSavedGraphs, oldSavedGraphs)
        newGraphsJSON, err = json.Marshal(newSavedGraphs)
        //fmt.Printf("%s\n",newGraphsJSON)
        newGraphshash = Sha1(string(newGraphsJSON))
    }
    // compare with hashes we were sent (should match!!)
    if newGraphshash != graphshash {
        log.Printf("Error: for user %s, new sync graph hashes don't match those sent from client: %s vs %s\n", usermail,newGraphshash,graphshash)
        fmt.Fprintf(rw, "{\"succ\":\"false\"}")
        return
    }

    // TAGS

    // default:
    newTagsJSON := []byte(tags)
    newTagshash := Sha1(tags)
    // see if diff given:
    if len(difftags) > 0 {
        var oldSavedTags []SavedTag
        err = json.Unmarshal([]byte(tags),&oldSavedTags)
        if err != nil { log.Printf("Unmarshal error: %s\n",err) }

        var diffSavedTags []SavedTag
        err = json.Unmarshal([]byte(difftags),&diffSavedTags)
        if err != nil { log.Printf("Unmarshal error: %s\n",err) }

        //fmt.Printf("for user %s, read %d tags from db\n", usermail, len(oldSavedTags))
        //fmt.Printf("for user %s, read %d diff tags from internets\n", usermail, len(diffSavedTags))

        // Merge
        newSavedTags := MergeSavedTags(diffSavedTags, oldSavedTags)
        newTagsJSON, err = json.Marshal(newSavedTags)
        newTagshash = Sha1(string(newTagsJSON))
    }
    // compare with hashes we were sent (should match!!)
    if newTagshash != tagshash {
        log.Printf("ERROR: for user %s, new sync tag hashes don't match those sent from client: %s vs %s\n", usermail,newTagshash,tagshash)
        fmt.Fprintf(rw, "{\"succ\":\"false\"}")
        return
    }

    // SETTINGS

    // default:
    newSettingsJSON := []byte(settings)
    newSettingshash := Sha1(settings)
    // see if diff given:
    if len(diffsettings) > 0 {
        var oldSavedSettings SavedUserSettings
        err = json.Unmarshal([]byte(settings),&oldSavedSettings)
        if err != nil { log.Printf("Unmarshal error: %s\n",err) }

        var diffSavedSettings SavedUserSettings
        err = json.Unmarshal([]byte(diffsettings),&diffSavedSettings)
        if err != nil { log.Printf("Unmarshal error: %s\n",err) }

        // Merge
        newSavedSettings := MergeSavedSettings(diffSavedSettings, oldSavedSettings)
        newSettingsJSON, err = json.Marshal(newSavedSettings)
        newSettingshash = Sha1(string(newSettingsJSON))
    }
    // compare with hashes we were sent (should match!!)
    if newSettingshash != settingshash {
        log.Printf("ERROR: for user %s, new sync settings hashes don't match those sent from client: %s vs %s\n", usermail,newSettingshash,settingshash)
        fmt.Fprintf(rw, "{\"succ\":\"false\"}")
        return
    }

    /* MYSQL */
    /*********/

    stmt = h.papers.StatementBegin("UPDATE userdata SET notes = ?, graphs = ?, tags = ?, settings = ?, numsync = numsync + 1, lastsync = NOW() WHERE usermail = ?", newNotesJSON, newGraphsJSON, newTagsJSON, newSettingsJSON, h.papers.db.Escape(usermail))
    if !h.papers.StatementEnd(stmt) {
        fmt.Fprintf(rw, "{\"succ\":\"false\"}")
        return
    }

    // We succeeded
    fmt.Fprintf(rw, "{\"succ\":\"true\",\"nh\":\"%s\",\"gh\":\"%s\",\"th\":\"%s\",\"sh\":\"%s\"}",newNoteshash,newGraphshash,newTagshash,newSettingshash)
}


/* Serves stored graph on user page load */
func (h *MyHTTPHandler) LinkLoad(code string, rw http.ResponseWriter) {

    var notes, graphs, tags []byte
    modcode := ""

    // discover if we've loading code or modcode
    // codes and modcodes are unique
    // first check if its a code
    stmt := h.papers.StatementBegin("SELECT notes,graphs,tags FROM sharedata WHERE code = ?",h.papers.db.Escape(code))
    if !h.papers.StatementBindSingleRow(stmt,&notes,&graphs,&tags) {
        // It wasn't, so check if its a modcode
        var modcodeDb, codeDb string
        stmt := h.papers.StatementBegin("SELECT notes,graphs,tags,code,modkey FROM sharedata WHERE modkey = ?",h.papers.db.Escape(code))
        if !h.papers.StatementBindSingleRow(stmt,&notes,&graphs,&tags,&codeDb,&modcodeDb) {
            return
        }
        code = codeDb
        modcode = modcodeDb
    }

    stmt = h.papers.StatementBegin("UPDATE sharedata SET numloaded = numloaded + 1, lastloaded = NOW() WHERE code = ?",h.papers.db.Escape(code))
    if !h.papers.StatementEnd(stmt) {
        return
    }

    fmt.Fprintf(rw, "{\"code\":\"%s\",\"mkey\":\"%s\"", code, modcode)

    // NOTES
    if len(notes) == 0 { notes = []byte("[]") }
    noteshash := Sha1(string(notes))
    fmt.Fprintf(rw, ",\"note\":%s,\"nh\":\"%s\"",string(notes),noteshash)

    // GRAPHS
    if len(graphs) == 0 { graphs =  []byte("[]") }
    graphshash := Sha1(string(graphs))
    fmt.Fprintf(rw, ",\"grph\":%s,\"gh\":\"%s\"",string(graphs),graphshash)

    // TAGS
    if len(tags) == 0 { tags = []byte("[]") }
    tagshash := Sha1(string(tags))
    fmt.Fprintf(rw, ",\"tag\":%s,\"th\":\"%s\"",string(tags),tagshash)

    // end
    fmt.Fprintf(rw, "}")
}

/* Link Save */
func (h *MyHTTPHandler) LinkSave(modcode string, notesIn string, notesInHash string, graphsIn string, graphsInHash string, tagsIn string, tagsInHash string, rw http.ResponseWriter) {

    // Unmarshal, re-Marshal and hash data strings to ensure consistency
    // Also prevents malicious injection of data in our db
    var err error

    // notes
    var notesOut []byte
    var savedNotes []SavedNote
    err = json.Unmarshal([]byte(notesIn),&savedNotes)
    if err != nil {
        log.Printf("Unmarshal error: %s\n",err)
    }
    notesOut, err = json.Marshal(savedNotes)
    //fmt.Printf("Marshaled notes: %s\n", notesOut)
    if notesInHash != Sha1(string(notesOut)) {
        log.Printf("ERROR: LinkSave notesIn doesn't match notesOut\n")
        return
    }

    // graphs
    var graphsOut []byte
    var savedGraphs []SavedMultiGraph
    err = json.Unmarshal([]byte(graphsIn),&savedGraphs)
    if err != nil {
        log.Printf("Unmarshal error: %s\n",err)
    }
    graphsOut, err = json.Marshal(savedGraphs)
    //fmt.Printf("Marshaled graphs: %s\n", graphsOut)
    if graphsInHash != Sha1(string(graphsOut)) {
        log.Printf("ERROR: LinkSave graphsIn doesn't match graphsOut\n")
        return
    }

    // tags
    var tagsOut []byte
    var savedTags []SavedTag
    err = json.Unmarshal([]byte(tagsIn),&savedTags)
    if err != nil {
        log.Printf("Unmarshal error: %s\n",err)
    }
    tagsOut, err = json.Marshal(savedTags)
    //fmt.Printf("Marshaled tags: %s\n", tagsOut)
    if tagsInHash != Sha1(string(tagsOut)) {
        log.Printf("ERROR: LinkSave tagsIn doesn't match tagsOut\n")
        return
    }

    // Check modcode
    if len(modcode) > 16 {
        log.Printf("ERROR: LinkSave given code are too long\n")
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
        for i := 0; i < N; i++ {
            code = GenerateRandString(8,8)
            modcode = GenerateRandString(16,16)
            stmt := h.papers.StatementBegin("SELECT code FROM sharedata WHERE code = ? OR modkey = ? OR code = ? OR modkey = ?",h.papers.db.Escape(code),h.papers.db.Escape(code),h.papers.db.Escape(modcode),h.papers.db.Escape(modcode))
            var fubar string
            if !h.papers.StatementBindSingleRow(stmt,&fubar) {
                break
            }
        }
        if code == "" || modcode == "" {
            log.Printf("ERROR: LinkSave couldn't generate a code and modcode in %d tries!\n",N)
            return
        } else {
            stmt := h.papers.StatementBegin("INSERT INTO sharedata (code,modkey,lastloaded) VALUES (?,?,NOW())",h.papers.db.Escape(code),h.papers.db.Escape(modcode))
            if !h.papers.StatementEnd(stmt) {return}
        }
    }

    // save
    stmt := h.papers.StatementBegin("UPDATE sharedata SET notes = ?, graphs = ?, tags = ? where code = ? AND modkey = ?", string(notesOut), string(graphsOut), string(tagsOut), h.papers.db.Escape(code), h.papers.db.Escape(modcode))
    if !h.papers.StatementEnd(stmt) {
        return
    }

    // We succeeded
    fmt.Fprintf(rw, "{\"code\":\"%s\",\"mkey\":\"%s\"}",code,modcode)
}

/*
func (h *MyHTTPHandler) MapLoadWorld(rw http.ResponseWriter) {

    var txmin,tymin,txmax,tymax int
    var lxmin,lymin,lxmax,lymax int
    var tpixw,tpixh,idmax,idnew uint
    var tilings,labelings string

    //stmt := h.papers.StatementBegin("SELECT max(id) FROM map_data")
    //if !h.papers.StatementBindSingleRow(stmt,&idmax) {
    //    return
    //}

    stmt := h.papers.StatementBegin("SELECT tile_data.latest_id,tile_data.xmin,tile_data.ymin,tile_data.xmax,tile_data.ymax,tile_data.tile_pixel_w,tile_data.tile_pixel_h,tile_data.tilings,label_data.xmin,label_data.ymin,label_data.xmax,label_data.ymax,label_data.zones FROM tile_data,label_data WHERE tile_data.latest_id = label_data.latest_id")
    if !h.papers.StatementBindSingleRow(stmt,&idmax,&txmin,&tymin,&txmax,&tymax,&tpixw,&tpixh,&tilings,&lxmin,&lymin,&lxmax,&lymax,&labelings) {
        return
    }

    stmt = h.papers.StatementBegin("SELECT max(datebdry.id) FROM datebdry WHERE datebdry.id < ?",idmax)
    if !h.papers.StatementBindSingleRow(stmt,&idnew) {
        return
    }

    fmt.Fprintf(rw, "{\"txmin\":%d,\"tymin\":%d,\"txmax\":%d,\"tymax\":%d,\"idmax\":%d,\"idnew\":%d,\"tpxw\":%d,\"tpxh\":%d,\"tile\":%s,\"lxmin\":%d,\"lymin\":%d,\"lxmax\":%d,\"lymax\":%d,\"label\":%s}",txmin, tymin,txmax,tymax,idmax,idnew,tpixw,tpixh,tilings,lxmin,lymin,lxmax,lymax,labelings)
}*/

func (h *MyHTTPHandler) MapLocationFromPaperId(ids []uint, rw http.ResponseWriter) {
    
    var x,y int 
    var resId, r uint

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

    sql := fmt.Sprintf("SELECT id,x,y,r FROM map_data WHERE id IN %s LIMIT %d",args.String(),len(ids))

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

func (h *MyHTTPHandler) MapPaperIdAtLocation(x, y float64, rw http.ResponseWriter) {
    
    var id, resr uint
    var resx, resy int 

    // TODO
    // Current implentation is slow (order n)
    // use quad tree: order log n
    // OR try using MySQL spatial extensions

    fmt.Printf("%f %f\n",x,y)

    sql := "SELECT id,x,y,r FROM map_data WHERE sqrt(pow(x - ?,2) + pow(y - ?,2)) - r <= 0 LIMIT 1"

    stmt := h.papers.StatementBegin(sql,x,y)
    if !h.papers.StatementBindSingleRow(stmt,&id,&resx,&resy,&resr) {
        return
    }
    
    fmt.Fprintf(rw, "{\"id\":%d,\"x\":%d,\"y\":%d,\"r\":%d}",id,resx,resy,resr)
}

/*
func (h *MyHTTPHandler) MapKeywordsInWindow(x, y, width, height int64, rw http.ResponseWriter) {

    var xmin,ymin,xmax,ymax int
    var idmax uint

    // this is temp until we access labels table directly
    stmt := h.papers.StatementBegin("SELECT max(id) FROM map_data")
    if !h.papers.StatementBindSingleRow(stmt,&idmax) {
        return
    }

    stmt = h.papers.StatementBegin("SELECT tile_data.xmin,tile_data.ymin,tile_data.xmax,tile_data.ymax FROM tile_data WHERE tile_data.latest_id = ?",idmax)
    if !h.papers.StatementBindSingleRow(stmt,&xmin,&ymin,&xmax,&ymax) {
        return
    }

    // TODO!

    // Returns a keyword "region", which is fixed world rectangle
    // with 

    fmt.Fprintf(rw, "{\"id\":\"foo\",\"x\":%d,\"y\":%d,\"w\":%d,\"h\":%d,\"ls\":[{\"kw\":\"test keyword\",\"x\":0,\"y\":0}]}",xmin,ymin,xmax-xmin,ymax-ymin)


}*/


func (h *MyHTTPHandler) GetDateBoundaries(rw http.ResponseWriter) (success bool) {
    
    success = false

    // perform query
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

    fmt.Fprintf(rw, "{\"v\":\"%s\",",VERSION)
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
            PrintJSONMetaInfo(rw, paper)
        } else if flag & 0x02 != 0 {
            // Update meta
            fmt.Fprintf(rw, ",")
            PrintJSONUpdateMetaInfo(rw, paper)
        }
        if flag & 0x04 != 0 {
            // All refs 
            h.papers.QueryRefs(paper, false)
            fmt.Fprintf(rw, ",")
            PrintJSONAllRefs(rw, paper)
        }
        if flag & 0x08 != 0 {
            // All cites
            h.papers.QueryCites(paper, false)
            fmt.Fprintf(rw, ",")
            PrintJSONAllCites(rw, paper, 0)
        } else if flag & 0x10 != 0 {
            // New cites
            h.papers.QueryCites(paper, false)
            if len(paper.cites) < 26 {
                fmt.Fprintf(rw, ",")
                PrintJSONAllCites(rw, paper, 0)
            } else {
                fmt.Fprintf(rw, ",")
                PrintJSONNewCites(rw, paper, uint(db))
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

func (h *MyHTTPHandler) ConvertHumanToInternalIds(arxivIds []string, doiIds []string, journalIds []string, rw http.ResponseWriter) {
    
    // Set sane limit to stop people spamming us by importing file with too many ids!
    idLength := 0
    if arxivIds   != nil { idLength += len(arxivIds) }
    if doiIds     != nil { idLength += len(doiIds) }
    if journalIds != nil { idLength += len(journalIds) }

    if idLength > ID_CONVERSION_LIMIT {
        return
    }

    // send back a JSON dictionary
    // for each ID, try to convert to internal ID
    fmt.Fprintf(rw, "{")
    first := true
   
    // ARXIV IDS
    if arxivIds != nil && len(arxivIds) > 0 {
        // create sql statement dynamically based on number of IDs
        var args bytes.Buffer
        args.WriteString("(")
        for i, _ := range arxivIds {
            if i > 0 { 
                args.WriteString(",")
            }
            args.WriteString("?")
        }
        args.WriteString(")")
        sql := fmt.Sprintf("SELECT id, arxiv FROM meta_data WHERE arxiv IN %s LIMIT %d",args.String(),len(arxivIds))
        //fmt.Println(sql)

        // create interface of arguments for statement
        hIdsInt := make([]interface{},len(arxivIds))
        for i, arxivId := range arxivIds {
            hIdsInt[i] = interface{}(h.papers.db.Escape(arxivId))
        }
        
        // Execute statement
        stmt := h.papers.StatementBegin(sql,hIdsInt...)
        var internalId uint64
        var arxiv string
        if stmt != nil {
            stmt.BindResult(&internalId,&arxiv)
            for {
                eof, err := stmt.Fetch()
                if err != nil {
                    fmt.Println("MySQL statement error;", err)
                    break
                } else if eof { break }
                if first { first = false } else { fmt.Fprintf(rw, ",") }
                // write directly to output!
                fmt.Fprintf(rw, "\"%s\":%d",arxiv,internalId)
            }
            err := stmt.FreeResult()
            if err != nil {
                fmt.Println("MySQL statement error;", err)
            }
        }
        h.papers.StatementEnd(stmt) 
    } // end arxivIds

    // DOI IDs
    if doiIds != nil && len(doiIds) > 0 {
        // create sql statement dynamically based on number of IDs
        var args bytes.Buffer
        for i, _ := range doiIds {
            if i > 0 { 
                args.WriteString(" OR ")
            }
            args.WriteString("publ LIKE ?")
        }
        sql := fmt.Sprintf("SELECT id, publ FROM meta_data WHERE %s LIMIT %d",args.String(),len(doiIds))

        // create interface of arguments for statement
        hIdsInt := make([]interface{},len(doiIds))
        for i, doiId := range doiIds {
            // NOTE dangerous operation!! e.g what if journalId empty?!
            // Therefore we put LIMIT on number of results
            dbEntry := "%#" + h.papers.db.Escape(doiId) + "%"
            hIdsInt[i] = interface{}(dbEntry)
        }
        
        dict := make(map[uint64]string)
        // Execute statement
        stmt := h.papers.StatementBegin(sql,hIdsInt...)
        var internalId uint64
        var publ string
        if stmt != nil {
            stmt.BindResult(&internalId,&publ)
            for {
                eof, err := stmt.Fetch()
                if err != nil {
                    fmt.Println("MySQL statement error;", err)
                    break
                } else if eof { break }
                dict[internalId] = publ
            }
            err := stmt.FreeResult()
            if err != nil {
                fmt.Println("MySQL statement error;", err)
            }
        }
        h.papers.StatementEnd(stmt) 

        // write to output
        for internalId := range dict {
            for _, doiId := range doiIds {
                publ := dict[internalId]
                if (strings.Contains(publ,string(doiId))) {
                    if first { first = false } else { fmt.Fprintf(rw, ",") }
                    fmt.Fprintf(rw, "\"doi:%s\":%d",doiId,internalId)
                }
            }
        }
    } // end doiIds


    // JOURNAL IDs
    // User not likely to generate an exact match by hand, so may as well
    // use same format in saved file as in db. Later, when we have proper
    // journal searching we could reuse that
    if journalIds != nil && len(journalIds) > 0 {
        // create sql statement dynamically based on number of IDs
        var args bytes.Buffer
        for i, _ := range journalIds {
            if i > 0 { 
                args.WriteString(" OR ")
            }
            args.WriteString("publ LIKE ?")
        }
        sql := fmt.Sprintf("SELECT id, publ FROM meta_data WHERE %s LIMIT %d",args.String(),len(journalIds))

        // create interface of arguments for statement
        hIdsInt := make([]interface{},len(journalIds))
        for i, journalId := range journalIds {
            // NOTE dangerous operation!! e.g what if journalId empty?!
            // Therefore we put LIMIT on number of results
            dbEntry := h.papers.db.Escape(journalId) + "#%"
            hIdsInt[i] = interface{}(dbEntry)
        }
        
        dict := make(map[uint64]string)
        // Execute statement
        stmt := h.papers.StatementBegin(sql,hIdsInt...)
        var internalId uint64
        var publ string
        if stmt != nil {
            stmt.BindResult(&internalId,&publ)
            for {
                eof, err := stmt.Fetch()
                if err != nil {
                    fmt.Println("MySQL statement error;", err)
                    break
                } else if eof { break }
                dict[internalId] = publ
            }
            err := stmt.FreeResult()
            if err != nil {
                fmt.Println("MySQL statement error;", err)
            }
        }
        h.papers.StatementEnd(stmt) 

        // write to output
        for internalId := range dict {
            for _, journalId := range journalIds {
                publ := dict[internalId]
                if (strings.Contains(publ,string(journalId))) {
                    if first { first = false } else { fmt.Fprintf(rw, ",") }
                    fmt.Fprintf(rw, "\"%s\":%d",journalId,internalId)
                }
            }
        }
    } // end journalIds

    fmt.Fprintf(rw, "}")
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
    fmt.Fprintf(rw, "{\"papr\":[{\"id\":%d,", paper.id)
    PrintJSONMetaInfo(rw, paper)
    fmt.Fprintf(rw, ",")
    PrintJSONAllRefs(rw, paper)
    fmt.Fprintf(rw, ",")
    PrintJSONAllCites(rw, paper, 0)
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

    stmt := h.papers.StatementBegin("SELECT keywords.id,pcite.numCites FROM keywords,pcite WHERE keywords.id = pcite.id AND MATCH(keywords.keywords) AGAINST (?) LIMIT 100",h.papers.db.Escape(searchString))

    var id,numCites uint64

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

    stmt := h.papers.StatementBegin("SELECT meta_data.id,pcite.numCites FROM meta_data,pcite WHERE meta_data.id = pcite.id AND MATCH(meta_data.authors,meta_data.title) AGAINST (?) LIMIT 100",h.papers.db.Escape(searchString))

    var id,numCites uint64
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
