package main

import (
    //"io"
    //"io/ioutil"
    "flag"
    "os"
    //"bufio"
    "fmt"
    //"net"
    //"net/http"
    //"net/http/fcgi"
    //"strconv"
    //"unicode"
    "encoding/json"
    //"text/scanner"
    "GoMySQL"
    //"runtime"
    //"bytes"
    //"time"
    //"strings"
    //"math/rand"
    //"crypto/sha1"
    //"crypto/sha256"
    //"compress/gzip"
    //"crypto/aes"
    //"sort"
    //"net/smtp"
    "log"
    //"xiwi"
    "image"
    "image/color"
    "image/draw"
    "image/png"
    "image/jpeg"
)

var flagDB      = flag.String("db", "localhost", "MySQL database to connect to")
var flagLogFile = flag.String("log-file", "", "file to output log information to")
var flagPciteTable = flag.String("table", "pcite", "MySQL database table to get pcite data from")
var flagFastCGIAddr = flag.String("fcgi", "", "listening on given address using FastCGI protocol (eg -fcgi :9100)")
var flagHTTPAddr = flag.String("http", "", "listening on given address using HTTP protocol (eg -http :8089)")
var flagTestQueryId = flag.Uint("test-id", 0, "run a test query with id")
var flagTestQueryArxiv = flag.String("test-arxiv", "", "run a test query with arxiv")
var flagMetaBaseDir = flag.String("meta", "", "Base directory for meta file data (abstracts etc.)")

func main() {
    // parse command line options
    flag.Parse()

    if flag.NArg() != 1 {
        log.Fatal("need to specify map.json file")
    }

    if len(*flagMetaBaseDir) == 0 {
        *flagMetaBaseDir = "/home/xiwi/data/meta"
    }

    // connect to MySQL database
    db, err := mysql.DialTCP(*flagDB, "hidden", "hidden", "xiwi")
    if err != nil {
        fmt.Println("cannot connect to database;", err)
        return
    }
    defer db.Close()

    DoWork(db, flag.Arg(0))
}

type Paper struct {
    id      uint
    maincat string
    x       int
    y       int
    radius  int
    age     float32
    colBG   color.Color
    colFG   color.Color
}

type Graph struct {
    papers  []*Paper
    bounds  image.Rectangle
}

func QueryCategories(db *mysql.Client, id uint) string {
    // execute the query
    query := fmt.Sprintf("SELECT maincat,allcats FROM meta_data WHERE id=%d", id)
    err := db.Query(query)
    if err != nil {
        fmt.Println("MySQL query error;", err)
        return ""
    }

    // get result set  
    result, err := db.StoreResult()
    if err != nil {
        fmt.Println("MySQL store result error;", err)
        return ""
    }

    // check if there are any results
    if result.RowCount() == 0 {
        return ""
    }

    // should be only 1 result
    if result.RowCount() != 1 {
        fmt.Println("MySQL multiple results; result count =", result.RowCount())
        return ""
    }

    // get the row
    row := result.FetchRow()
    if row == nil {
        return ""
    }

    // get the categories
    var ok bool
    var maincat string
    if row[0] != nil {
        if maincat, ok = row[0].(string); !ok { return "" }
    }
    /*
    var allcats string
    if row[1] != nil {
        if allcats, ok := row[1].(string); !ok { return "" }
    }
    */

    db.FreeResult()

    return maincat
}

func getPaperById(papers []*Paper, id uint) *Paper {
    lo := 0;
    hi := len(papers)
    for lo <= hi {
        mid := (lo + hi) / 2
        if id == papers[mid].id {
            return papers[mid]
        } else if id < papers[mid].id {
            hi = mid - 1
        } else {
            lo = mid + 1
        }
    }
    return nil
}

func QueryCategories2(db *mysql.Client, papers []*Paper) {
    // execute the query
    err := db.Query("SELECT id,maincat,allcats FROM meta_data")
    if err != nil {
        fmt.Println("MySQL query error;", err)
        return
    }

    // get result set  
    result, err := db.UseResult()
    if err != nil {
        fmt.Println("MySQL use result error;", err)
        return
    }

    // get each row from the result
    for {
        row := result.FetchRow()
        if row == nil {
            break
        }

        var ok bool
        var id uint64
        var maincat string
        //var allcats string
        if id, ok = row[0].(uint64); !ok { continue }
        if maincat, ok = row[1].(string); !ok { continue }
        //if allcats, ok = row[2].(string); !ok { continue }

        paper := getPaperById(papers, uint(id))
        if paper != nil {
            paper.maincat = maincat
        }
    }

    db.FreeResult()
}

func MakePaper(db *mysql.Client, id uint, x int, y int, radius int, age float64) *Paper {
    paper := new(Paper)
    paper.id = id
    paper.x = x
    paper.y = y
    paper.radius = radius
    paper.age = float32(age)
    //paper.maincat = QueryCategories(db, id)

    return paper
}

func (paper *Paper) setColour() {
    // basic colour of paper
    var r, g, b float64
    if paper.maincat == "hep-th" {
        r, g, b = 0, 0, 1
    } else if paper.maincat == "hep-ph" {
        r, g, b = 0, 1, 0
    } else if paper.maincat == "hep-ex" {
        r, g, b = 1, 1, 0 // yellow
    } else if paper.maincat == "gr-qc" {
        r, g, b = 0, 1, 1 // cyan
    } else if paper.maincat == "astro-ph.GA" {
        r, g, b = 1, 0, 1 // purple
    } else if paper.maincat == "hep-lat" {
        r, g, b = 0.7, 0.36, 0.2 // tan brown
    } else if paper.maincat == "astro-ph.CO" {
        r, g, b = 0.62, 0.86, 0.24 // lime green
    } else if paper.maincat == "astro-ph" {
        r, g, b = 0.89, 0.53, 0.6 // skin pink
    } else if paper.maincat == "cont-mat" {
        r, g, b = 0.6, 0.4, 0.4
    } else if paper.maincat == "quant-ph" {
        r, g, b = 0.4, 0.7, 0.7
    } else if paper.maincat == "physics" {
        r, g, b = 0, 0.5, 0 // dark green
    } else {
        r, g, b = 0.7, 1, 0.3
    }

    // background colour
    paper.colBG = color.RGBA{uint8(255 * (0.7 + 0.3 * r)), uint8(255 * (0.7 + 0.3 * g)), uint8(255 * (0.7 + 0.3 * b)), 255}

    // older papers are more saturated in colour
    age := float64(paper.age)
    saturation := 0.4 * (1 - age)

    // foreground colour; newer papers tend towards red
    age = age * age
    r = saturation + (r * (1 - age) + age) * (1 - saturation)
    g = saturation + (g * (1 - age)      ) * (1 - saturation)
    b = saturation + (b * (1 - age)      ) * (1 - saturation)
    paper.colFG = color.RGBA{uint8(255 * r), uint8(255 * g), uint8(255 * b), 255}
}

func ReadGraph(db *mysql.Client, posFilename string) *Graph {
    file, err := os.Open(flag.Arg(0))
    if err != nil {
        log.Fatal(err)
    }
    defer file.Close()

    dec := json.NewDecoder(file)
    var papers [][]int
    if err := dec.Decode(&papers); err != nil {
        log.Fatal(err)
    }

    fmt.Printf("parsed %v papers\n", len(papers))

    graph := new(Graph)
    graph.papers = make([]*Paper, len(papers))
    for index, paper := range papers {
        var age float64 = float64(index) / float64(len(papers))
        paperObj := MakePaper(db, uint(paper[0]), paper[1], paper[2], paper[3], age)
        graph.papers[index] = paperObj
        if paperObj.x < graph.bounds.Min.X { graph.bounds.Min.X = paperObj.x }
        if paperObj.y < graph.bounds.Min.Y { graph.bounds.Min.Y = paperObj.y }
        if paperObj.x > graph.bounds.Max.X { graph.bounds.Max.X = paperObj.x }
        if paperObj.y > graph.bounds.Max.Y { graph.bounds.Max.Y = paperObj.y }
    }

    QueryCategories2(db, graph.papers)

    for _, paper := range graph.papers {
        paper.setColour()
    }

    fmt.Printf("graph has %v papers; min=(%v,%v), max=(%v,%v)\n", len(papers), graph.bounds.Min.X, graph.bounds.Min.Y, graph.bounds.Max.X, graph.bounds.Max.Y)
    return graph
}

type circle struct {
    p image.Point
    r int
}

func (c *circle) ColorModel() color.Model {
    return color.AlphaModel
}

func (c *circle) Bounds() image.Rectangle {
    return image.Rect(c.p.X-c.r, c.p.Y-c.r, c.p.X+c.r, c.p.Y+c.r)
}

func (c *circle) At(x, y int) color.Color {
    xx, yy, rr := float64(x-c.p.X)+0.5, float64(y-c.p.Y)+0.5, float64(c.r)
    if xx*xx+yy*yy < rr*rr {
        return color.Alpha{255}
    }
    return color.Alpha{0}
}

type circle_stroke struct {
    p image.Point
    r int
}

func (c *circle_stroke) ColorModel() color.Model {
    return color.AlphaModel
}

func (c *circle_stroke) Bounds() image.Rectangle {
    return image.Rect(c.p.X-c.r, c.p.Y-c.r, c.p.X+c.r, c.p.Y+c.r)
}

func (c *circle_stroke) At(x, y int) color.Color {
    xx, yy, rr := float64(x-c.p.X)+0.5, float64(y-c.p.Y)+0.5, float64(c.r)
    dist := xx*xx+yy*yy - rr*rr
    if dist < 0 && dist > -1 {
        return color.Alpha{255}
    }
    return color.Alpha{0}
}

type Canvas struct {
    img     draw.Image
    x0      int
    y0      int
    scale   int
}

func NewCanvas(w int, h int, scale int) *Canvas {
    fmt.Printf("canvas size: %dx%d\n", w, h)
    canv := new(Canvas)
    canv.img = image.NewRGBA(image.Rect(0, 0, w, h))
    canv.x0 = -w / 2
    canv.y0 = -h / 2
    canv.scale = scale
    return canv
}

func (canv *Canvas) EncodePNG(filename string) {
    file, err := os.Create(filename)
    if err != nil {
        log.Fatal(err)
    }
    defer file.Close()
    err = png.Encode(file, canv.img)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("saved %vx%v png to %v\n", canv.img.Bounds().Dx(), canv.img.Bounds().Dy(), filename)
}

func (canv *Canvas) EncodeJPEG(filename string) {
    file, err := os.Create(filename)
    if err != nil {
        log.Fatal(err)
    }
    defer file.Close()
    err = jpeg.Encode(file, canv.img, nil)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("saved %vx%v jpeg to %v\n", canv.img.Bounds().Dx(), canv.img.Bounds().Dy(), filename)
}

func HTMLColour(col uint) color.RGBA {
    return color.RGBA{uint8((col >> 16) & 0xff), uint8((col >> 8) & 0xff), uint8(col & 0xff), 255}
}

func (canv *Canvas) Clear(col color.Color) {
    draw.Draw(canv.img, canv.img.Bounds(), &image.Uniform{col}, image.ZP, draw.Src)
}

func (canv *Canvas) CircleFill(pt image.Point, radius int, col color.Color) {
    pt.X = pt.X / canv.scale - canv.x0
    pt.Y = pt.Y / canv.scale - canv.y0
    radius /= canv.scale
    draw.DrawMask(canv.img, canv.img.Bounds(), &image.Uniform{col}, image.ZP, &circle{pt, radius}, image.ZP, draw.Over)
}

func (canv *Canvas) CircleStroke(pt image.Point, radius int, col color.Color) {
    pt.X = pt.X / canv.scale - canv.x0
    pt.Y = pt.Y / canv.scale - canv.y0
    radius /= canv.scale
    draw.DrawMask(canv.img, canv.img.Bounds(), &image.Uniform{col}, image.ZP, &circle_stroke{pt, radius}, image.ZP, draw.Over)
}

func DoWork(db *mysql.Client, posFilename string) {
    graph := ReadGraph(db, posFilename)
    canv := NewCanvas(graph.bounds.Dx() / 15, graph.bounds.Dy() / 15, 12)
    canv.Clear(HTMLColour(0x445566))
    for _, paper := range graph.papers {
        canv.CircleFill(image.Pt(paper.x, paper.y), 2*paper.radius, paper.colBG)
    }
    for _, paper := range graph.papers {
        canv.CircleFill(image.Pt(paper.x, paper.y), paper.radius, paper.colFG)
        //canv.CircleStroke(image.Pt(paper.x, paper.y), paper.radius, color.Black)
    }
    canv.EncodePNG("out-2.png")
    //canv.EncodeJPEG("out-.jpg")
}
