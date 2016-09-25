package main

import (
    "io"
    "flag"
    "os"
    "fmt"
    "net"
    "net/http"
    "net/http/fcgi"
    "runtime"
    "time"
    "strings"
    "math/rand"
    "compress/gzip"
    "log"
    //"GoMySQL"
    "github.com/yanatan16/GoMySQL"
)

// Current version of my.paperscape.
// Any client my.paperscape instances that don't match this 
// will be prompted to do a hard reload of the latest version
// Client equivalent is set in profile.js
var VERSION_MYPSCP = "0.14"

// Current version of paperscape map.
// Any client papercape-map instances that don't match this will
// be prompted to do a hard reload of the latest version
// Client equivalent is set in main.coffee
var VERSION_PSCPMAP = "0.3"

// Max number of ids we will convert from human to internal form at a time
// We need some sane limit to stop mysql being spammed! If profile bigger than this,
// it will need to make several calls
var ID_CONVERSION_LIMIT = 50

var flagSettingsFile   = flag.String("settings", "../config/arxiv-settings.json", "Read settings from JSON file")
var flagLogFile = flag.String("log-file", "", "file to output log information to")
var flagMetaBaseDir = flag.String("meta", "", "Base directory for meta file data (abstracts etc.)")

var flagFastCGIAddr = flag.String("fcgi", "", "listening on given address using FastCGI protocol (eg -fcgi :9100)")
var flagHTTPAddr = flag.String("http", "", "listening on given address using HTTP protocol (eg -http :8089)")

var flagTestQueryId = flag.Uint("test-id", 0, "run a test query with id")
var flagTestQueryArxiv = flag.String("test-arxiv", "", "run a test query with arxiv")

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

    // read in settings
    config := ReadConfigFromJSON(*flagSettingsFile)
    if config == nil {
        log.Fatal("Could not read in config settings")
        return
    }

    // connect to db
    db := ConnectToDB()
    if db == nil {
        return
    }
    defer db.Close()

    // create papers database, and assign config to it
    papers := NewPapersEnv(db, config)

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

func NewPapersEnv(db *mysql.Client, config *Config) *PapersEnv {
    papers := new(PapersEnv)
    papers.db = db
    papers.cfg = config
    db.Reconnect = true
    return papers
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

// a simple wrapper for http.ResponseWriter that counts number of bytes and keeps a log description
type MyResponseWriter struct {
    rw http.ResponseWriter
    bytesWritten int
    logDescription string
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
        rw = &MyResponseWriter{rwIn, 0,""}
    } else {
        rwIn.Header().Set("Content-Encoding", "gzip")
        gz := gzip.NewWriter(rwIn)
        defer gz.Close()
        rw = &MyResponseWriter{gzipResponseWriter{Writer: gz, ResponseWriter: rwIn},0,""}
    }

    if req.Form["callback"] != nil {
        // construct a JSON object to return
        rw.Header().Set("Content-Type", "application/json")
        callback := req.Form["callback"][0]
        fmt.Fprintf(rw, "%s({\"r\":", callback)
        resultBytesStart := rw.bytesWritten

        if req.Form["test"] != nil {
            fmt.Fprintf(rw, "{\"test\":\"success\", \"POST\":false}")
        } else if h.ResponsePscpGeneral(rw,req) {
        } else if h.ResponsePscpMap(rw,req) {
        } else if h.papers.cfg.Settings.ServeMyPscp && h.ResponseMyPscp(rw,req) {
        } else {
            // unknown ajax request
            rw.logDescription = fmt.Sprintf("unknown")
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
        rw.logDescription = fmt.Sprintf("echo \"%s\"",req.Form["fn"][0])
    } else if req.Method == "POST" {
        // POST verb

        // construct a JSON object to return
        rw.Header().Set("Access-Control-Allow-Origin", "*") // for cross domain POSTing; see https://developer.mozilla.org/en/http_access_control
        rw.Header().Set("Content-Type", "application/json")
        fmt.Fprintf(rw, "{\"r\":")
        resultBytesStart := rw.bytesWritten

        if req.Form["test"] != nil {
            fmt.Fprintf(rw, "{\"test\":\"success\", \"POST\":true}")
        } else if h.ResponsePscpGeneral(rw,req) {
        } else if h.ResponsePscpMap(rw,req) {
        } else if h.papers.cfg.Settings.ServeMyPscp && h.ResponseMyPscp(rw,req) {
        } else {
            // unknown ajax request
            rw.logDescription = fmt.Sprintf("unknown")
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

    if rw.logDescription != "" {
        log.Printf("%s -- %s %s -- bytes: %d URL, %d content, %d replied\n", req.RemoteAddr, req.Method, rw.logDescription, len(req.URL.String()), req.ContentLength, rw.bytesWritten)
    }

    runtime.GC()
}

func ConnectToDB() *mysql.Client {
    // connect to MySQL database; using a socket is preferred since it's faster
    var db *mysql.Client
    var err error

    mysql_host := os.Getenv("PSCP_MYSQL_HOST")
    mysql_user := os.Getenv("PSCP_MYSQL_USER")
    mysql_pwd  := os.Getenv("PSCP_MYSQL_PWD")
    mysql_db   := os.Getenv("PSCP_MYSQL_DB")
    mysql_sock := os.Getenv("PSCP_MYSQL_SOCKET")

    // if nothing requested, default to something sensible
    var dbConnection string
    if fileExists(mysql_sock) {
        dbConnection = mysql_sock
    } else {
        dbConnection = mysql_host
    }

    // make the connection
    if strings.HasSuffix(dbConnection, ".sock") {
        db, err = mysql.DialUnix(dbConnection, mysql_user, mysql_pwd, mysql_db)
    } else {
        db, err = mysql.DialTCP(dbConnection, mysql_user, mysql_pwd, mysql_db)
    }


    if err != nil {
        fmt.Println("cannot connect to database;", err)
        return nil
    } else {
        fmt.Println("connected to database:", dbConnection)
        return db
    }
    return db
}

// returns whether the given file or directory exists or not
func fileExists(path string) bool {
    _, err := os.Stat(path)
    return err == nil
}
