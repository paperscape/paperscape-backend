package main

import (
    "io"
    "io/ioutil"
    "fmt"
    "net/http"
    "strconv"
    "encoding/json"
    "bytes"
    "strings"
    "math/rand"
    "crypto/sha1"
    "crypto/sha256"
    "sort"
    "net/smtp"
    "log"
    //"bitbucket.org/kardianos/osext"
    "github.com/kardianos/osext"
)


/****************************************************************/

type IdSliceSort []uint
func (id IdSliceSort) Len() int           { return len(id) }
func (id IdSliceSort) Less(i, j int) bool { return id[i] < id[j] }
func (id IdSliceSort) Swap(i, j int)      { id[i], id[j] = id[j], id[i] }

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

func generateRandString(minLen int, maxLen int) string {
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

func hashSha1 (str string) string {
    hash := sha1.New()
    io.WriteString(hash, fmt.Sprintf("%s", string(str)))
    return fmt.Sprintf("%x",hash.Sum(nil))
}

func hashSha256 (str string) string {
    hash := sha256.New()
    io.WriteString(hash, fmt.Sprintf("%s", string(str)))
    return fmt.Sprintf("%x",hash.Sum(nil))
}


func generateUserPassword() (string, int, int64, string) {
    password := generateRandString(8,8)
    salt := rand.Int63()
    // hash+salt password
    pwdversion := 2 // password hashing strength
    var userhash string
    userhash = hashSha256(fmt.Sprintf("%s%d", hashSha256(hashSha1(password)), salt))
    return password, pwdversion, salt, userhash
}


func readAndReplaceFromFile(path string, dict map[string]string) (message string, err error) {
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

}

/****************************************************************/

func (h *MyHTTPHandler) ResponseMyPscp(rw *MyResponseWriter, req *http.Request) (requestFound bool) {
    
    requestFound = true

    if req.Form["gdb"] != nil {
        // get-date-boundaries
        success := h.GetDateBoundaries(rw)
        if !success {
            rw.logDescription = fmt.Sprintf("gdb")
        }
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
            rw.logDescription = fmt.Sprintf("pchal %s",req.Form["pchal"][0])
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
            rw.logDescription = fmt.Sprintf("pload %s",req.Form["pload"][0])
        }
    } else if req.Form["pchpw"] != nil && req.Form["h"] != nil && req.Form["p"] != nil && req.Form["s"] != nil && req.Form["pv"] != nil {
        // profile-change-password: change password request
        // h = passHash, p = payload, s = sprinkle (salt), pv = password version
        h.ProfileChangePassword(req.Form["pchpw"][0], req.Form["h"][0], req.Form["p"][0], req.Form["s"][0], req.Form["pv"][0], rw)
        rw.logDescription = fmt.Sprintf("pchpw %s pv=%s",req.Form["pchpw"][0],req.Form["pv"][0])
    } else if req.Form["prrp"] != nil {
        // profile-request-reset-password: request reset link sent to email
        h.ProfileRequestResetPassword(req.Form["prrp"][0], rw)
        rw.logDescription = fmt.Sprintf("prrp %s",req.Form["prrp"][0])
    } else if req.Form["prpw"] != nil {
        // profile-reset-password: resets password request and sends new one to email
        h.ProfileResetPassword(req.Form["prpw"][0], rw)
        rw.logDescription = fmt.Sprintf("prpw %s",req.Form["prpw"][0])
    } else if req.Form["preg"] != nil {
        // profile-register: register email address as new user 
        h.ProfileRegister(req.Form["preg"][0], rw)
        rw.logDescription = fmt.Sprintf("preg %s",req.Form["preg"][0])
    } else if req.Form["lload"] != nil {
        // link-load: from a page load
        h.LinkLoad(req.Form["lload"][0], rw)
        rw.logDescription = fmt.Sprintf("lload %s",req.Form["lload"][0])
    } else if req.Form["psync"] != nil && req.Form["h"] != nil && req.Form["nh"] != nil && req.Form["gh"] != nil && req.Form["th"] != nil && req.Form["sh"] != nil {
        // profile-sync: sync request
        // h = passHash, n = notesdiff, g = graphsdiff, t = tagsdiff, s = settingsdiff (and the end result hashes)
        var n, g, t, s string
        if req.Form["n"] != nil { n = req.Form["n"][0] }
        if req.Form["g"] != nil { g = req.Form["g"][0] }
        if req.Form["t"] != nil { t = req.Form["t"][0] }
        if req.Form["s"] != nil { s = req.Form["s"][0] }
        h.ProfileSync(req.Form["psync"][0], req.Form["h"][0], n, req.Form["nh"][0], g, req.Form["gh"][0], t, req.Form["th"][0], s, req.Form["sh"][0], rw)
        rw.logDescription = fmt.Sprintf("psync %s",req.Form["psync"][0])
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
        rw.logDescription = fmt.Sprintf("lsave %s",descSaveHash)
    } else if req.Form["snp"] != nil && req.Form["f"] != nil && req.Form["t"] != nil {
        // *OBSOLETE*
        // search-new-papers: search papers between given id range
        // f = from, t = to
        h.SearchNewPapers(req.Form["f"][0], req.Form["t"][0], rw)
        rw.logDescription = fmt.Sprintf("snp (%s,%s)", req.Form["f"][0], req.Form["t"][0])
    } else if req.Form["chids"] != nil && (req.Form["arx[]"] != nil ||  req.Form["doi[]"] != nil || req.Form["jrn[]"] != nil) {
        // convert-human-ids: convert human IDs to internal IDs
        // arx: list of arxiv IDs
        // jrn: list of journal IDs
        h.ConvertHumanToInternalIds(req.Form["arx[]"],req.Form["doi[]"],req.Form["jrn[]"], rw)
        rw.logDescription = fmt.Sprintf("chids (%d,%d,%d)",len(req.Form["arx[]"]),len(req.Form["doi[]"]),len(req.Form["jrn[]"]))
    } else {
        requestFound = false
    }

    return
}

/****************************************************************/

func (h *MyHTTPHandler) GetDateBoundaries(rw http.ResponseWriter) (success bool) {
    success = false

    fmt.Fprintf(rw, "{\"v\":\"%s\"",VERSION_MYPSCP)
    if h.papers.cfg.Sql.Date.Name != "" {
        //query := "SELECT daysAgo,id FROM datebdry WHERE daysAgo <= 5"
        query := "SELECT " + h.papers.cfg.Sql.Date.FieldDays
        query += "," + h.papers.cfg.Sql.Date.FieldId
        query += " FROM "  + h.papers.cfg.Sql.Date.Name
        query += " WHERE " + h.papers.cfg.Sql.Date.FieldDays + " <= 5"

        stmt := h.papers.StatementBegin(query)

        if stmt == nil {
            fmt.Fprintf(rw, "}")
            fmt.Println("MySQL statement error; empty")
            return
        }

        var daysAgo,id uint64
        stmt.BindResult(&daysAgo,&id)
        defer h.papers.StatementEnd(stmt) 
        
        fmt.Fprintf(rw, ",")
        numResults := 0
        for {
            eof, err := stmt.Fetch()
            if err != nil {
                fmt.Println("MySQL statement error;", err)
                break
            } else if eof { break }
            if numResults > 0 {
                fmt.Fprintf(rw, ",")
            }
            fmt.Fprintf(rw, "\"d%d\":%d", daysAgo, id)
            numResults += 1
        }
        err := stmt.FreeResult()
        if err != nil {
            fmt.Println("MySQL statement error;", err)
        }
    }
    fmt.Fprintf(rw, "}")
    success = true
    return
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
        //sql := fmt.Sprintf("SELECT id, arxiv FROM meta_data WHERE arxiv IN %s LIMIT %d",args.String(),len(arxivIds))
        sql := "SELECT " + h.papers.cfg.Sql.Meta.FieldId 
        sql += "," + h.papers.cfg.Sql.Meta.FieldArxiv 
        sql += " FROM " + h.papers.cfg.Sql.Meta.Name
        sql += fmt.Sprintf(" WHERE %s IN %s LIMIT %d", h.papers.cfg.Sql.Meta.FieldArxiv, args.String(),len(arxivIds))
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
            //args.WriteString("publ LIKE ?")
            args.WriteString(h.papers.cfg.Sql.Meta.FieldPubl + " LIKE ?")
        }
        //sql := fmt.Sprintf("SELECT id, publ FROM meta_data WHERE %s LIMIT %d",args.String(),len(doiIds))
        sql := "SELECT " + h.papers.cfg.Sql.Meta.FieldId 
        sql += "," + h.papers.cfg.Sql.Meta.FieldPubl 
        sql += " FROM " + h.papers.cfg.Sql.Meta.Name
        sql += fmt.Sprintf(" WHERE %s LIMIT %d", args.String(),len(doiIds))

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
            //args.WriteString("publ LIKE ?")
            args.WriteString(h.papers.cfg.Sql.Meta.FieldPubl + " LIKE ?")
        }
        //sql := fmt.Sprintf("SELECT id, publ FROM meta_data WHERE %s LIMIT %d",args.String(),len(journalIds))
        sql := "SELECT " + h.papers.cfg.Sql.Meta.FieldId 
        sql += "," + h.papers.cfg.Sql.Meta.FieldPubl 
        sql += " FROM " + h.papers.cfg.Sql.Meta.Name
        sql += fmt.Sprintf(" WHERE %s LIMIT %d", args.String(),len(journalIds))

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
    tryhash := hashSha256(fmt.Sprintf("%s%d", userhash, challenge))

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
        code := generateRandString(64,64)
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
    msgDir, _ := osext.ExecutableFolder()
    message, _ := readAndReplaceFromFile(msgDir + "/mypscp_emails/pwd_reset_request.email",dict)

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
    password, pwdversion, salt, userhash := generateUserPassword()

    // load new password into, and remove resetcode 
    stmt = h.papers.StatementBegin("UPDATE userdata SET userhash = ?, salt = ?, pwdversion = ?, resetcode = NULL WHERE usermail = ?", userhash, salt, pwdversion, usermail)
    if !h.papers.StatementEnd(stmt) {
        return
    }

    dict := make(map[string]string)
    dict["@@USERMAIL@@"] = usermail
    dict["@@PASSWORD@@"] = password
    msgDir, _ := osext.ExecutableFolder()
    message, _ := readAndReplaceFromFile(msgDir + "/mypscp_emails/pwd_reset.email",dict)

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
    password, pwdversion, salt, userhash := generateUserPassword()

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
    msgDir, _ := osext.ExecutableFolder()
    message, _ := readAndReplaceFromFile(msgDir + "/mypscp_emails/user_registration.email",dict)

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
    noteshashDb := hashSha1(string(notes))
    graphshashDb := hashSha1(string(graphs))
    tagshashDb := hashSha1(string(tags))
    settingshashDb := hashSha1(string(settings))

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
    newNoteshash := hashSha1(notes)
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
        newNoteshash = hashSha1(string(newNotesJSON))
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
    newGraphshash := hashSha1(graphs)
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
        newGraphshash = hashSha1(string(newGraphsJSON))
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
    newTagshash := hashSha1(tags)
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
        newTagshash = hashSha1(string(newTagsJSON))
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
    newSettingshash := hashSha1(settings)
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
        newSettingshash = hashSha1(string(newSettingsJSON))
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
    noteshash := hashSha1(string(notes))
    fmt.Fprintf(rw, ",\"note\":%s,\"nh\":\"%s\"",string(notes),noteshash)

    // GRAPHS
    if len(graphs) == 0 { graphs =  []byte("[]") }
    graphshash := hashSha1(string(graphs))
    fmt.Fprintf(rw, ",\"grph\":%s,\"gh\":\"%s\"",string(graphs),graphshash)

    // TAGS
    if len(tags) == 0 { tags = []byte("[]") }
    tagshash := hashSha1(string(tags))
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
    if notesInHash != hashSha1(string(notesOut)) {
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
    if graphsInHash != hashSha1(string(graphsOut)) {
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
    if tagsInHash != hashSha1(string(tagsOut)) {
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
            code = generateRandString(8,8)
            modcode = generateRandString(16,16)
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

    //query := "SELECT meta_data.id,meta_data.allcats,pcite.numCites FROM meta_data,pcite WHERE meta_data.id >= ? AND meta_data.id <= ? AND meta_data.id = pcite.id LIMIT 500"
    query := "SELECT " + h.papers.cfg.Sql.Meta.Name + "." + h.papers.cfg.Sql.Meta.FieldId 
    query += "," + h.papers.cfg.Sql.Meta.Name + "." + h.papers.cfg.Sql.Meta.FieldAllcats 
    query += "," + h.papers.cfg.Sql.Refs.Name + "." + h.papers.cfg.Sql.Refs.FieldNumCites 
    query += " FROM " + h.papers.cfg.Sql.Meta.Name + "," + h.papers.cfg.Sql.Refs.Name
    query += " WHERE " + h.papers.cfg.Sql.Meta.Name + "." + h.papers.cfg.Sql.Meta.FieldId + " >= ?"
    query += " AND " + h.papers.cfg.Sql.Meta.Name + "." + h.papers.cfg.Sql.Meta.FieldId + " <= ?"
    query += " AND " + h.papers.cfg.Sql.Meta.Name + "." + h.papers.cfg.Sql.Meta.FieldId
    query += " = " + h.papers.cfg.Sql.Refs.Name + "." + h.papers.cfg.Sql.Refs.FieldId
    query += " LIMIT 500"
    
    stmt := h.papers.StatementBegin(query,idFrom,idTo)
    if stmt == nil {
        fmt.Println("MySQL statement error; empty")
        fmt.Fprintf(rw, "[]")
        return
    }

    var id, numCites uint64
    var allcats string
    stmt.BindResult(&id,&allcats,&numCites)
    defer h.papers.StatementEnd(stmt) 
    
    fmt.Fprintf(rw, "[")
    numResults := 0
    for {
        eof, err := stmt.Fetch()
        if err != nil {
            fmt.Println("MySQL statement error;", err)
            break
        } else if eof { break }
        if numResults > 0 {
            fmt.Fprintf(rw, ",")
        }
        fmt.Fprintf(rw, "{\"id\":%d,\"cat\":\"%s\",\"nc\":%d}", id, allcats, numCites)
        numResults += 1
    }
    err := stmt.FreeResult()
    if err != nil {
        fmt.Println("MySQL statement error;", err)
    }
    fmt.Fprintf(rw, "]")
}

