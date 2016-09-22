package main

import (
    "os"
    "bufio"
    "fmt"
    "unicode"
    "encoding/json"
    "bytes"
    "log"
    "strconv"
    //"GoMySQL"
    "github.com/yanatan16/GoMySQL"
)

type PapersEnv struct {
    db *mysql.Client
    cfg *Config
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
    var query string

    if id == 0 {
        if len(arxiv) == 0 || papers.cfg.Sql.Meta.FieldArxiv == "" {
            return nil
        } else {
            // given an arxiv string, so properly sanitise 
            //query = fmt.Sprintf("SELECT id FROM meta_data WHERE arxiv = ?")
            query  = "SELECT " + papers.cfg.Sql.Meta.FieldId
            query += " FROM " +  papers.cfg.Sql.Meta.Name
            query += " WHERE " + papers.cfg.Sql.Meta.FieldArxiv + " = ?"
            stmt := papers.StatementBegin(query,arxiv)
            if !papers.StatementBindSingleRow(stmt,&id) {
                return nil
            }
        }
    }

    // Construct the query
    //query = fmt.Sprintf("SELECT id,arxiv,maincat,allcats,inspire,authors,title,publ FROM meta_data WHERE id = %d", id)
    query  = "SELECT " + papers.cfg.Sql.Meta.FieldId
    if papers.cfg.Sql.Meta.FieldArxiv != "" {
        query += "," + papers.cfg.Sql.Meta.FieldArxiv
    }
    if papers.cfg.Sql.Meta.FieldMaincat != "" {
        query += "," + papers.cfg.Sql.Meta.FieldMaincat
    }
    if papers.cfg.Sql.Meta.FieldAllcats != "" {
        query += "," + papers.cfg.Sql.Meta.FieldAllcats
    }
    if papers.cfg.Sql.Meta.FieldInspire != "" {
        query += "," + papers.cfg.Sql.Meta.FieldInspire
    }
    query += "," + papers.cfg.Sql.Meta.FieldAuthors
    query += "," + papers.cfg.Sql.Meta.FieldTitle
    query += "," + papers.cfg.Sql.Meta.FieldPubl
    if papers.cfg.Sql.Meta.FieldAuxInt1 != "" {
        query += "," + papers.cfg.Sql.Meta.FieldAuxInt1
    }
    if papers.cfg.Sql.Meta.FieldAuxInt2 != "" {
        query += "," + papers.cfg.Sql.Meta.FieldAuxInt2
    }
    if papers.cfg.Sql.Meta.FieldAuxStr1 != "" {
        query += "," + papers.cfg.Sql.Meta.FieldAuxStr1
    }
    if papers.cfg.Sql.Meta.FieldAuxStr2 != "" {
        query += "," + papers.cfg.Sql.Meta.FieldAuxStr2
    }
    query += " FROM " + papers.cfg.Sql.Meta.Name
    query += fmt.Sprintf(" WHERE %s = %d",papers.cfg.Sql.Meta.FieldId,id)
    
    // Run the query
    row := papers.QuerySingleRow(query)
    papers.QueryEnd()

    if row == nil { return nil }

    // get the fields
    paper := new(Paper)
    i := 0
    if idNum, ok := row[i].(uint64); !ok {
        return nil
    } else {
        paper.id = uint(idNum)
    }
    var ok bool
    i++
    if papers.cfg.Sql.Meta.FieldArxiv != "" {
        if row[i] != nil {
            if paper.arxiv, ok = row[i].(string); !ok { return nil }
        }
        i++
    }
    if papers.cfg.Sql.Meta.FieldMaincat != "" {
        if row[i] != nil {
            if paper.maincat, ok = row[i].(string); !ok { return nil }
        }
        i++
    }
    if papers.cfg.Sql.Meta.FieldAllcats != "" {
        if paper.allcats, ok = row[i].(string); !ok { paper.allcats = "" }
        i++
    }
    if papers.cfg.Sql.Meta.FieldInspire != "" {
        if row[i] != nil {
            if inspire, ok := row[i].(uint64); ok { paper.inspire = uint(inspire); }
        }
        i++
    }
    if row[i] == nil {
        paper.authors = "(unknown authors)"
    } else if au, ok := row[i].([]byte); !ok {
        log.Printf("ERROR: cannot get authors for id=%d; %v\n", paper.id, row[i])
        return nil
    } else {
        paper.authors = string(au)
    }
    i++
    if row[i] == nil {
        paper.authors = "(unknown title)"
    } else if title, ok := row[i].(string); !ok {
        log.Printf("ERROR: cannot get title for id=%d; %v\n", paper.id, row[i])
        return nil
    } else {
        paper.title = title
    }
    i++
    if row[i] == nil {
        paper.publJSON = "";
    } else if publ, ok := row[i].(string); !ok {
        log.Printf("ERROR: cannot get publ for id=%d; %v\n", paper.id, row[i])
        paper.publJSON = "";
    } else {
        publ2 := string(publ) // convert to string so marshalling does the correct thing
        publ3, _ := json.Marshal(publ2)
        paper.publJSON = string(publ3)
    }
    i++
    // handle loading of auxillary fields
    auxObjectJSON := "{"
    first_entry := true
    if papers.cfg.Sql.Meta.FieldAuxInt1 != "" {
        if row[i] != nil {
            if auxint, ok := row[i].(uint64); ok {
                first_entry = false
                auxObjectJSON += "\"int1\":" + strconv.Itoa(int(auxint))
            }
        }
        i++
    }
    if papers.cfg.Sql.Meta.FieldAuxInt2 != "" {
        if row[i] != nil {
            if auxint, ok := row[i].(uint64); ok {
                if !first_entry { auxObjectJSON += "," }
                first_entry = false
                auxObjectJSON += "\"int2\":" + strconv.Itoa(int(auxint))
            }
        }
        i++
    }
    if papers.cfg.Sql.Meta.FieldAuxStr1 != "" {
        if row[i] != nil {
            if auxstr, ok := row[i].([]byte); ok {
                // NOTE no check for good JSON 
                if !first_entry { auxObjectJSON += "," }
                first_entry = false
                auxObjectJSON += "\"str1\":\"" + string(auxstr) + "\""
            }
        }
        i++
    }
    if papers.cfg.Sql.Meta.FieldAuxStr2 != "" {
        if row[i] != nil {
            if auxstr, ok := row[i].([]byte); ok {
                // NOTE no check for good JSON 
                if !first_entry { auxObjectJSON += "," }
                first_entry = false
                auxObjectJSON += "\"str2\":\"" + string(auxstr) + "\""
            }
        }
        i++
    }
    auxObjectJSON += "}"
    if len(auxObjectJSON) > 2 {
        paper.auxJSON = auxObjectJSON
    }

    //// Get number of times cited, and change in number of cites
    //query = fmt.Sprintf("SELECT numRefs,numCites,dNumCites1,dNumCites5 FROM pcite WHERE id = %d", paper.id)
    query  = "SELECT " + papers.cfg.Sql.Refs.FieldNumRefs
    query += "," + papers.cfg.Sql.Refs.FieldNumCites
    if papers.cfg.Sql.Refs.FieldDNumCites1 != "" {
        query += "," + papers.cfg.Sql.Refs.FieldDNumCites1
    }
    if papers.cfg.Sql.Refs.FieldDNumCites5 != "" {
        query += "," + papers.cfg.Sql.Refs.FieldDNumCites5
    }
    query += " FROM " + papers.cfg.Sql.Refs.Name
    query += fmt.Sprintf(" WHERE %s = %d", papers.cfg.Sql.Refs.FieldId, paper.id)
    row2 := papers.QuerySingleRow(query)

    if row2 != nil {
        i := 0
        if numRefs, ok := row2[i].(uint64); ok {
            paper.numRefs = uint(numRefs)
        }
        i++
        if numCites, ok := row2[i].(uint64); ok {
            paper.numCites = uint(numCites)
        }
        i++
        if papers.cfg.Sql.Refs.FieldDNumCites1 != "" {
            if dNumCites1, ok := row2[i].(int64); ok {
                paper.dNumCites1 = uint(dNumCites1)
            }
            i++
        }
        if papers.cfg.Sql.Refs.FieldDNumCites5 != "" {
            if dNumCites5, ok := row2[i].(int64); ok {
                paper.dNumCites5 = uint(dNumCites5)
            }
            i++
        }
    } else {
        log.Printf("ERROR: cannot get pcite data for id=%d\n", paper.id)
    }

    papers.QueryEnd()

    return paper
}

func (papers *PapersEnv) QueryRefs(paper *Paper, queryRefsMeta bool) {
    if paper == nil { return }

    // check if refs already exist
    if len(paper.refs) != 0 { return }

    // perform query
    //query := fmt.Sprintf("SELECT refs FROM pcite WHERE id = %d", paper.id)
    query := "SELECT " + papers.cfg.Sql.Refs.FieldRefs
    query += " FROM " + papers.cfg.Sql.Refs.Name
    query += fmt.Sprintf(" WHERE %s = %d", papers.cfg.Sql.Refs.FieldId, paper.id)
    row := papers.QuerySingleRow(query)
    if row == nil { papers.QueryEnd(); return }

    var ok bool
    var refStr []byte
    if refStr, ok = row[0].([]byte); !ok { papers.QueryEnd(); return }

    // parse the ref string, creating links
    papers.ParseRefsCitesString(paper, refStr, true)
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
    //query := fmt.Sprintf("SELECT cites FROM pcite WHERE id = %d", paper.id)
    query := "SELECT " + papers.cfg.Sql.Refs.FieldCites
    query += " FROM " + papers.cfg.Sql.Refs.Name
    query += fmt.Sprintf(" WHERE %s = %d", papers.cfg.Sql.Refs.FieldId, paper.id)
    row := papers.QuerySingleRow(query)
    if row == nil { papers.QueryEnd(); return }

    var ok bool
    var citeStr []byte
    if citeStr, ok = row[0].([]byte); !ok { papers.QueryEnd(); return }

    // parse the cite string, creating links
    papers.ParseRefsCitesString(paper, citeStr, false)
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

// Query locations for everything we already have available
// e.g also refs/cites if we have them
func (papers *PapersEnv) QueryLocations(paper *Paper, tableSuffix string) {
    if paper == nil { return }
   
    // build list of ids
    var ids []uint64

    ids = append(ids, uint64(paper.id))
    if paper.refs != nil {
        for _, ref := range paper.refs {
            ids = append(ids, uint64(ref.pastId))
        }
    }
    if paper.cites != nil {
        for _, cite := range paper.cites {
            ids = append(ids, uint64(cite.futureId))
        }
    }

    var x,y int 
    var resId uint64
    var r uint

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

    loc_table := papers.cfg.Sql.Map.Name
    if isValidTableSuffix(tableSuffix) {
        loc_table += "_" + tableSuffix
    }

    //sql := fmt.Sprintf("SELECT id,x,y,r FROM " + loc_table + " WHERE id IN %s LIMIT %d",args.String(),len(ids))
    sql := "SELECT " + papers.cfg.Sql.Map.FieldId
    sql += "," + papers.cfg.Sql.Map.FieldX
    sql += "," + papers.cfg.Sql.Map.FieldY
    sql += "," + papers.cfg.Sql.Map.FieldR
    sql += " FROM " + loc_table
    sql += fmt.Sprintf(" WHERE %s IN %s LIMIT %d", papers.cfg.Sql.Map.FieldId, args.String(), len(ids))
    // create interface of arguments for statement
    hIdsInt := make([]interface{},len(ids))
    for i, id := range ids {
        hIdsInt[i] = interface{}(id)
    }
    
    // Execute statement
    stmt := papers.StatementBegin(sql,hIdsInt...)
    if stmt != nil {
        stmt.BindResult(&resId,&x,&y,&r)
        for {
            eof, err := stmt.Fetch()
            if err != nil {
                fmt.Println("MySQL statement error;", err)
                break
            } else if eof { break }
            // attach location to correct reference
            foundIt := false
            if paper.id == uint(resId) {
                paper.location = &Location{x,y,r}
                foundIt = true
            }
            if !foundIt && paper.refs != nil {
                for _, ref := range paper.refs {
                    if ref.pastId == uint(resId) {
                        ref.location = &Location{x,y,r} 
                        foundIt = true
                        break
                    }
                }
            }
            if !foundIt && paper.cites != nil {
                for _, cite := range paper.cites {
                    if cite.futureId == uint(resId) {
                        cite.location = &Location{x,y,r} 
                        foundIt = true
                        break
                    }
                }
            }
            //fmt.Fprintf(rw, "{\"id\":%d,\"x\":%d,\"y\":%d,\"r\":%d}",resId, x, y,r)
        }
        err := stmt.FreeResult()
        if err != nil {
            fmt.Println("MySQL statement error;", err)
        }
    }
    papers.StatementEnd(stmt) 
}

func (papers *PapersEnv) GetAbstract(paperId uint) string {
    
    if papers.cfg.Sql.Abst.Name == "" && papers.cfg.Sql.Meta.FieldArxiv != "" {
        // get the arxiv name for this id
        //query := fmt.Sprintf("SELECT arxiv FROM meta_data WHERE id = %d", paperId)
        query := "SELECT " + papers.cfg.Sql.Meta.FieldArxiv
        query += " FROM " + papers.cfg.Sql.Meta.Name
        query += fmt.Sprintf(" WHERE %s = %d", papers.cfg.Sql.Meta.FieldId, paperId)
        row := papers.QuerySingleRow(query)
        if row == nil { papers.QueryEnd(); return "(no abstract)" }
        arxiv, ok := row[0].(string)
        papers.QueryEnd()
        if !ok { return "(no abstract)" }

        // work out the meta filename for this arxiv
        var filename string
        if (len(arxiv) == 9 || len(arxiv) == 10) && arxiv[4] == '.' {
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
    } else if papers.cfg.Sql.Abst.Name != "" {
        query := "SELECT " + papers.cfg.Sql.Abst.FieldAbstract
        query += " FROM " + papers.cfg.Sql.Abst.Name
        query += fmt.Sprintf(" WHERE %s = %d", papers.cfg.Sql.Abst.FieldId, paperId)
        row := papers.QuerySingleRow(query)
        if row == nil { 
            fmt.Printf("here...\n")
            papers.QueryEnd(); 
            return "(no abstract)" 
        }
        abstract, ok := row[0].([]byte)
        papers.QueryEnd()
        if !ok { 
            fmt.Printf("or here...\n")
            return "(no abstract)" 
        } else {
            return string(abstract)
        }
    }

    return "(no abstract)"
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
func (papers *PapersEnv) ParseRefsCitesString(paper *Paper, blob []byte, isRefStr bool) bool {
    if len(blob) == 0 {
        // nothing to do, that's okay
        return true
    }

    /* uncomment this to disable 999 information
    if ((paper.id % 15625) % 4) == 2 {
        if isRefStr {
            return true
        }
    }
    */
    blobLen := 4
    if papers.cfg.Sql.Refs.RblobOrder { blobLen += 2 }
    if papers.cfg.Sql.Refs.RblobFreq  { blobLen += 2 }
    if papers.cfg.Sql.Refs.RblobCites { blobLen += 2 }

    if len(blob) % blobLen != 0 {
        log.Printf("ERROR: blob length %d is not a multiple of %d as expected, for id=%d\n", len(blob),blobLen,paper.id)
        return false
    }

    for i := 0; i < len(blob); i += blobLen {
        refId := getLE32(blob, i)
        buf_ind := 4
        var refOrder uint = 0
        if papers.cfg.Sql.Refs.RblobOrder {
            refOrder = getLE16(blob, i + buf_ind)
            buf_ind += 2
        }
        var refFreq uint = 1
        if papers.cfg.Sql.Refs.RblobFreq {
            refFreq = getLE16(blob, i + buf_ind)
            buf_ind += 2
        }
        var numCites uint = 0
        if papers.cfg.Sql.Refs.RblobCites {
            numCites = getLE16(blob, i + buf_ind)
            //buf_ind += 2
        }
        // make link and add to list in paper
        if isRefStr {
            link := &Link{uint(refId), paper.id, nil, paper, uint(refOrder), uint(refFreq), uint(numCites), paper.numCites, nil}
            paper.refs = append(paper.refs, link)
        } else {
            link := &Link{paper.id, uint(refId), paper, nil, uint(refOrder), uint(refFreq), paper.numCites, uint(numCites), nil}
            paper.cites = append(paper.cites, link)
        }
    }

    return true
}

func isValidTableSuffix(tableSuffix string) bool {
    if len(tableSuffix) == 0 || len(tableSuffix) > 32 {
        return false
    }
    // TODO simply use regex?
    validCharacters := []byte{'a','b','c','d','e','f','g','h','i','j','k','l','m','n','o','p','q','r','s','t','u','v','w','x','y','z','A','B','C','D','E','F','G','H','I','J','K','L','M','N','O','P','Q','R','S','T','U','V','W','X','Y','Z','1','2','3','4','5','6','7','8','9','0','_'}
    
    for _, c := range tableSuffix {
        valid := false
        for _, vc := range validCharacters {
            if byte(c) == vc {
                valid = true
                break
            }
        }
        if !valid {
            return false
        }
    }
    return true
}

