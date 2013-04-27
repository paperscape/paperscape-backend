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
    "math"
    //"math/rand"
    //"crypto/sha1"
    //"crypto/sha256"
    //"compress/gzip"
    //"crypto/aes"
    //"sort"
    //"net/smtp"
    "log"
    //"xiwi"
    //"image"
    //"image/color"
    //"image/draw"
    //"image/png"
    //"image/jpeg"
    "github.com/ungerik/go-cairo"
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

    if flag.NArg() != 2 {
        log.Fatal("need to specify map.json file, and output file (without extension)")
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

    DoWork(db, flag.Arg(0), flag.Arg(1))
}

type CairoColor struct {
    r, g, b float64
}

type Paper struct {
    id      uint
    maincat string
    x       int
    y       int
    radius  int
    age     float32
    colBG   CairoColor
    colFG   CairoColor
}

type Graph struct {
    papers  []*Paper
    MinX, MinY, MaxX, MaxY int
    BoundsX, BoundsY int
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
    lo := 0
    hi := len(papers) - 1
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
    paper.colBG = CairoColor{0.7 + 0.3 * r, 0.7 + 0.3 * g, 0.7 + 0.3 * b}

    // older papers are more saturated in colour
    age := float64(paper.age)
    saturation := 0.4 * (1 - age)

    // foreground colour; newer papers tend towards red
    age = age * age
    r = saturation + (r * (1 - age) + age) * (1 - saturation)
    g = saturation + (g * (1 - age)      ) * (1 - saturation)
    b = saturation + (b * (1 - age)      ) * (1 - saturation)
    paper.colFG = CairoColor{r, g, b}
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

    //papers = papers[0:10000]
    fmt.Printf("parsed %v papers\n", len(papers))

    graph := new(Graph)
    graph.papers = make([]*Paper, len(papers))
    for index, paper := range papers {
        var age float64 = float64(index) / float64(len(papers))
        paperObj := MakePaper(db, uint(paper[0]), paper[1], paper[2], paper[3], age)
        graph.papers[index] = paperObj
        if paperObj.x < graph.MinX { graph.MinX = paperObj.x }
        if paperObj.y < graph.MinY { graph.MinY = paperObj.y }
        if paperObj.x > graph.MaxX { graph.MaxX = paperObj.x }
        if paperObj.y > graph.MaxY { graph.MaxY = paperObj.y }
    }

    graph.BoundsX = graph.MaxX - graph.MinX
    graph.BoundsY = graph.MaxY - graph.MinY

    QueryCategories2(db, graph.papers)

    for _, paper := range graph.papers {
        paper.setColour()
    }

    fmt.Printf("graph has %v papers; min=(%v,%v), max=(%v,%v)\n", len(papers), graph.MinX, graph.MinY, graph.MaxX, graph.MaxY)
    return graph
}

type QuadTreeNode struct {
    //Parent          *QuadTreeNode
    //SideLength      int
    Leaf            *Paper
    Q0, Q1, Q2, Q3  *QuadTreeNode
}

type QuadTree struct {
    MinX, MinY, MaxX, MaxY  int
    Root                    *QuadTreeNode
}

func QuadTreeInsertPaper(parent *QuadTreeNode, q **QuadTreeNode, paper *Paper, MinX, MinY, MaxX, MaxY int) {
    if *q == nil {
        // hit an empty node; create a new leaf cell and put this paper in it
        *q = new(QuadTreeNode)
        //(*q).Parent = parent
        //(*q).SideLength = MaxX - MinX
        (*q).Leaf = paper

    } else if (*q).Leaf != nil {
        // hit a leaf; turn it into an internal node and re-insert the papers
        oldPaper := (*q).Leaf
        (*q).Leaf = nil
        (*q).Q0 = nil
        (*q).Q1 = nil
        (*q).Q2 = nil
        (*q).Q3 = nil
        QuadTreeInsertPaper(parent, q, oldPaper, MinX, MinY, MaxX, MaxY)
        QuadTreeInsertPaper(parent, q, paper, MinX, MinY, MaxX, MaxY)

    } else {
        // hit an internal node

        // check cell size didn't get too small
        if (MaxX <= MinX + 1 || MaxY <= MinY + 1) {
            log.Println("ERROR: QuadTreeInsertPaper hit minimum cell size")
            return
        }

        // compute the dividing x and y positions
        MidX := (MinX + MaxX) / 2
        MidY := (MinY + MaxY) / 2

        // insert the new paper in the correct cell
        if ((paper.y) < MidY) {
            if ((paper.x) < MidX) {
                QuadTreeInsertPaper(*q, &(*q).Q0, paper, MinX, MinY, MidX, MidY)
            } else {
                QuadTreeInsertPaper(*q, &(*q).Q1, paper, MidX, MinY, MaxX, MidY)
            }
        } else {
            if ((paper.x) < MidX) {
                QuadTreeInsertPaper(*q, &(*q).Q2, paper, MinX, MidY, MidX, MaxY)
            } else {
                QuadTreeInsertPaper(*q, &(*q).Q3, paper, MidX, MidY, MaxX, MaxY)
            }
        }
    }
}

func BuildQuadTree(papers []*Paper) *QuadTree {
    qt := new(QuadTree)

    // if no papers, return
    if len(papers) == 0 {
        return qt
    }

    // first work out the bounding box of all papers
    qt.MinX = papers[0].x
    qt.MinY = papers[0].y
    qt.MaxX = papers[0].x
    qt.MaxY = papers[0].y
    for _, paper := range papers {
        if (paper.x < qt.MinX) { qt.MinX = paper.x; }
        if (paper.y < qt.MinY) { qt.MinY = paper.y; }
        if (paper.x > qt.MaxX) { qt.MaxX = paper.x; }
        if (paper.y > qt.MaxY) { qt.MaxY = paper.y; }
    }

    // increase the bounding box so it's square
    {
        dx := qt.MaxX - qt.MinX
        dy := qt.MaxY - qt.MinY
        if dx > dy {
            cen_y := (qt.MinY + qt.MaxY) / 2
            qt.MinY = cen_y - dx / 2
            qt.MaxY = cen_y + dx / 2
        } else {
            cen_x := (qt.MinX + qt.MaxX) / 2
            qt.MinX = cen_x - dy / 2
            qt.MaxX = cen_x + dy / 2
        }
    }

    // build the quad tree
    for _, paper := range papers {
        QuadTreeInsertPaper(nil, &qt.Root, paper, qt.MinX, qt.MinY, qt.MaxX, qt.MaxY)
    }

    fmt.Printf("quad tree bounding box: (%v,%v) -- (%v,%v)\n", qt.MinX, qt.MinY, qt.MaxX, qt.MaxY)
    return qt
}

func (q *QuadTreeNode) ApplyIfWithin(MinX, MinY, MaxX, MaxY int, x, y, r int, f func(paper *Paper)) {
    if q == nil {
    } else if q.Leaf != nil {
        r += q.Leaf.radius
        if x - r <= q.Leaf.x && q.Leaf.x <= x + r && y - r <= q.Leaf.y && q.Leaf.y <= y + r {
            f(q.Leaf)
        }
    } else if ((MinX <= x - r && x - r < MaxX) || (MinX <= x + r && x + r < MaxX) || (x - r < MinX && x + r >= MaxX)) &&
              ((MinY <= y - r && y - r < MaxY) || (MinY <= y + r && y + r < MaxY) || (y - r < MinY && y + r >= MaxY)) {
        MidX := (MinX + MaxX) / 2
        MidY := (MinY + MaxY) / 2
        q.Q0.ApplyIfWithin(MinX, MinY, MidX, MidY, x, y, r, f)
        q.Q1.ApplyIfWithin(MidX, MinY, MaxX, MidY, x, y, r, f)
        q.Q2.ApplyIfWithin(MinX, MidY, MidX, MaxY, x, y, r, f)
        q.Q3.ApplyIfWithin(MidX, MidY, MaxX, MaxY, x, y, r, f)
    }
}

func (qt *QuadTree) ApplyIfWithin(x, y, r int, f func(paper *Paper)) {
    qt.Root.ApplyIfWithin(qt.MinX, qt.MinY, qt.MaxX, qt.MaxY, x, y, r, f)
}

func sq(x float64) float64 {
    return x * x
}

func DoWork(db *mysql.Client, posFilename string, outFilename string) {
    graph := ReadGraph(db, posFilename)
    qt := BuildQuadTree(graph.papers)

    surf := cairo.NewSurface(cairo.FORMAT_RGB24, graph.BoundsX / 12, graph.BoundsY / 12)
    surf.SetSourceRGB(4.0/15, 5.0/15, 6.0/15)
    surf.Paint()

    matrix := new(cairo.Matrix)
    matrix.Xx = float64(surf.GetWidth()) / float64(graph.BoundsX)
    matrix.Yy = float64(surf.GetHeight()) / float64(graph.BoundsY)
    if matrix.Xx < matrix.Yy {
        matrix.Yy = matrix.Xx
    } else {
        matrix.Xx = matrix.Yy
    }
    matrix.X0 = 0.5 * float64(surf.GetWidth())
    matrix.Y0 = 0.5 * float64(surf.GetHeight())

    surf.SetMatrix(*matrix)

    /*
    // simple halo background circle for each paper
    for _, paper := range graph.papers {
        surf.SetSourceRGB(paper.colBG.r, paper.colBG.g, paper.colBG.b)
        surf.Arc(float64(paper.x), float64(paper.y), 2 * float64(paper.radius), 0, 2 * math.Pi)
        surf.Fill()
    }
    */

    // area-based background
    fmt.Println("rendering background")
    surf.IdentityMatrix()
    matrixInv := *matrix
    matrixInv.Invert()
    for v := 0; v + 1 < surf.GetHeight(); v += 2 {
        for u := 0; u + 1 < surf.GetWidth(); u += 2 {
            x, y := matrixInv.TransformPoint(float64(u), float64(v))
            ptR := 0.0
            ptG := 0.0
            ptB := 0.0
            n := 0
            qt.ApplyIfWithin(int(x), int(y), 200, func(paper *Paper) {
                ptR += paper.colBG.r
                ptG += paper.colBG.g
                ptB += paper.colBG.b
                n += 1
            })
            if n > 10 {
                if n < 20 {
                    ptR += float64(20 - n) * 4.0/15
                    ptG += float64(20 - n) * 5.0/15
                    ptB += float64(20 - n) * 6.0/15
                    n = 20
                }
                ptR /= float64(n)
                ptG /= float64(n)
                ptB /= float64(n)
                surf.SetSourceRGB(ptR, ptG, ptB)
                surf.Rectangle(float64(u), float64(v), 2, 2)
                surf.Fill()
            }
        }
    }

    // foreground
    fmt.Println("rendering foreground")
    surf.SetMatrix(*matrix)
    surf.SetLineWidth(3)
    for _, paper := range graph.papers {
        surf.Arc(float64(paper.x), float64(paper.y), float64(paper.radius), 0, 2 * math.Pi)
        surf.SetSourceRGB(paper.colFG.r, paper.colFG.g, paper.colFG.b)
        surf.FillPreserve()
        surf.SetSourceRGB(0, 0, 0)
        surf.Stroke()
    }

    fmt.Println("writing file")
    surf.WriteToPNG(outFilename + ".png")
    //canv.EncodeJPEG("out-.jpg")
    surf.Finish()
}
