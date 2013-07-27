package main

import (
    "flag"
    "os"
    "bufio"
    "fmt"
    "path/filepath"
    "strings"
    "encoding/json"
    "GoMySQL"
    "runtime"
    "math"
    "sort"
    "log"
    "xiwi"
    "github.com/ungerik/go-cairo"
)

var GRAPH_PADDING = 100 // what to pad graph by on each side
var TILE_PIXEL_LEN = 256


var flagDB = flag.String("db", "", "MySQL database to connect to")
var flagGrayScale = flag.Bool("gs", false, "Make grayscale tiles")
var flagDoSingle = flag.Bool("single", false, "Do a large single tile") // now the default
var flagSkipTiles = flag.Bool("skip-tiles", false, "Only generate index file not tiles")
var flagMaxCores = flag.Int("cores",-1,"Max number of system cores to use, default is all of them")

func main() {
    // parse command line options
    flag.Parse()

    if flag.NArg() != 2 {
        log.Fatal("need to specify map.json file (db to load from DB), and output prefix (without extension)")
    }

    // connect to the db
    db := xiwi.ConnectToDB(*flagDB)
    if db == nil {
        return
    }
    defer db.Close()

    // read in the graph
    graph := ReadGraph(db, flag.Arg(0))

    // build the quad tree
    graph.BuildQuadTree()

    if *flagDoSingle {
        DrawTile(graph, graph.BoundsX, graph.BoundsY, 1, 1, 18000, 18000, flag.Arg(1))
    } else {
        GenerateAllTiles(graph, flag.Arg(1))
    }
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
    keyword string
    colBG   CairoColor
    colFG   CairoColor
}

type PaperSortId []*Paper
func (p PaperSortId) Len() int           { return len(p) }
func (p PaperSortId) Less(i, j int) bool { return p[i].id > p[j].id }
func (p PaperSortId) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }


type Graph struct {
    papers  []*Paper
    qt      *QuadTree
    MinX, MinY, MaxX, MaxY int
    BoundsX, BoundsY int
}

func (graph *Graph) GetPaperById(id uint) *Paper {
    lo := 0
    hi := len(graph.papers) - 1
    for lo <= hi {
        mid := (lo + hi) / 2
        if id == graph.papers[mid].id {
            return graph.papers[mid]
        } else if id < graph.papers[mid].id {
            hi = mid - 1
        } else {
            lo = mid + 1
        }
    }
    return nil
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

func QueryCategories2(db *mysql.Client, graph *Graph) {
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

        paper := graph.GetPaperById(uint(id))
        if paper != nil {
            paper.maincat = maincat
        }
    }

    db.FreeResult()

    fmt.Println("read categories")
}

func QueryKeywords(db *mysql.Client, graph *Graph) {
    // execute the query
    err := db.Query("SELECT id,keywords FROM keywords")
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
        var keywords1 []byte
        if id, ok = row[0].(uint64); !ok { continue }
        if keywords1, ok = row[1].([]byte); !ok { continue }

        paper := graph.GetPaperById(uint(id))
        if paper != nil {
            keywords := string(keywords1)
            idx := strings.Index(keywords, ",")
            if idx < 0 {
                idx = len(keywords)
            }
            paper.keyword = keywords[0:idx]
        }
    }

    db.FreeResult()

    fmt.Println("read keywords")
}

func QueryPapers(db *mysql.Client) []*Paper {
    // count number of papers
    err := db.Query("SELECT count(id) FROM map_data")
    if err != nil {
        fmt.Println("MySQL query error;", err)
        return nil
    }

    // get result set
    result, err := db.UseResult()
    if err != nil {
        fmt.Println("MySQL use result error;", err)
        return nil
    }
    row := result.FetchRow()
    if row == nil {
        fmt.Println("MySQL didn't return a row")
        return nil
    }

    // get number of papers
    var numPapers int64
    var ok bool
    if numPapers, ok = row[0].(int64); !ok {
        fmt.Println("MySQL didn't return a number")
        return nil
    }
    db.FreeResult()

    // allocate paper array
    papers := make([]*Paper, numPapers)

    // execute the query
    err = db.Query("SELECT id,x,y,r FROM map_data")
    if err != nil {
        fmt.Println("MySQL query error;", err)
        return nil
    }

    // get result set
    result, err = db.UseResult()
    if err != nil {
        fmt.Println("MySQL use result error;", err)
        return nil
    }

    // get each row from the result
    index := 0
    for {
        row := result.FetchRow()
        if row == nil {
            break
        }

        var ok bool
        var id uint64
        var x, y, r int64
        if id, ok = row[0].(uint64); !ok { continue }
        if x, ok = row[1].(int64); !ok { continue }
        if y, ok = row[2].(int64); !ok { continue }
        if r, ok = row[3].(int64); !ok { continue }

        var age float64 = float64(index) / float64(numPapers)
        papers[index] = MakePaper(uint(id), int(x), int(y), int(r), age)
        index += 1
    }

    db.FreeResult()

    if int64(index) != numPapers {
        fmt.Println("could not read all papers from map_data; wanted", numPapers, "got", index)
        return nil
    }

    return papers
}

func MakePaper(id uint, x int, y int, radius int, age float64) *Paper {
    paper := new(Paper)
    paper.id = id
    paper.x = x
    paper.y = y
    paper.radius = radius
    paper.age = float32(age)

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

    // foreground colour; select one by making it's if condition true
    if (false) {
        // older papers are saturated, newer papers are coloured
        saturation := 0.4 * (1 - age)
        r = saturation + (r) * (1 - saturation)
        g = saturation + (g) * (1 - saturation)
        b = saturation + (b) * (1 - saturation)
    } else if (false) {
        // older papers are saturated, newer papers are coloured and tend towards a full red component
        saturation := 0.4 * (1 - age)
        age = age * age
        r = saturation + (r * (1 - age) + age) * (1 - saturation)
        g = saturation + (g * (1 - age)      ) * (1 - saturation)
        b = saturation + (b * (1 - age)      ) * (1 - saturation)
    } else if (true) {
        // older papers are saturated and dark, newer papers are coloured and bright
        //saturation := 0.4 * (1 - age)
        saturation := 0.0
        dim_factor := 0.2 + 0.8 * age
        r = dim_factor * (saturation + r * (1 - saturation))
        g = dim_factor * (saturation + g * (1 - saturation))
        b = dim_factor * (saturation + b * (1 - saturation))
    }

    if *flagGrayScale {
        //lum := 0.21 * r + 0.72 * g + 0.07 * b 
        lum := 0.289 * r + 0.587 * g + 0.114 * b
        r = lum
        g = lum
        b = lum
    }

    // Try pure heatmap instead
    //var coldR, coldG, coldB, hotR, hotG, hotB float64
    //scale := float64(paper.age)

    //coldR, coldG, coldB = 0, 0, 1
    //hotR, hotG, hotB = 1, 0, 0
    //r = (hotR - coldR)*scale + coldR
    //g = (hotG - coldG)*scale + coldG
    //b = (hotB - coldB)*scale + coldB

    paper.colFG = CairoColor{r, g, b}
}

func ReadGraph(db *mysql.Client, posFilename string) *Graph {
    graph := new(Graph)

    if (posFilename == "db") {
        // load positions from the data base
        graph.papers = QueryPapers(db)
        if graph.papers == nil {
            log.Fatal("could not read papers from db")
        }
        fmt.Printf("read %v papers from db\n", len(graph.papers))

    } else {
        // load positions from a json file

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

        graph.papers = make([]*Paper, len(papers))
        for index, paper := range papers {
            var age float64 = float64(index) / float64(len(papers))
            paperObj := MakePaper(uint(paper[0]), paper[1], paper[2], paper[3], age)
            graph.papers[index] = paperObj
        }
    }

    QueryCategories2(db, graph)
    QueryKeywords(db, graph)

    for _, paper := range graph.papers {
        if paper.x - paper.radius < graph.MinX { graph.MinX = paper.x - paper.radius }
        if paper.y - paper.radius < graph.MinY { graph.MinY = paper.y - paper.radius }
        if paper.x + paper.radius > graph.MaxX { graph.MaxX = paper.x + paper.radius }
        if paper.y + paper.radius > graph.MaxY { graph.MaxY = paper.y + paper.radius }
    }

    // TRY Add safety buffers, if we use these must
    // account for them later in client code!
    graph.MinX -= GRAPH_PADDING
    graph.MaxX += GRAPH_PADDING
    graph.MinY -= GRAPH_PADDING
    graph.MaxY += GRAPH_PADDING

    graph.BoundsX = graph.MaxX - graph.MinX
    graph.BoundsY = graph.MaxY - graph.MinY

    for _, paper := range graph.papers {
        paper.setColour()
    }

    fmt.Printf("graph has %v papers; min=(%v,%v), max=(%v,%v)\n", len(graph.papers), graph.MinX, graph.MinY, graph.MaxX, graph.MaxY)

    return graph
}

type QuadTreeNode struct {
    //Parent          *QuadTreeNode
    //SideLength      int
    Leaf            *Paper
    Q0, Q1, Q2, Q3  *QuadTreeNode
}

type QuadTree struct {
    MinX, MinY, MaxX, MaxY, MaxR  int
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

func (graph *Graph) BuildQuadTree() {
    qt := new(QuadTree)

    // if no papers, return
    if len(graph.papers) == 0 {
        return
    }

    // first work out the bounding box of all papers
    qt.MinX = graph.papers[0].x
    qt.MinY = graph.papers[0].y
    qt.MaxX = graph.papers[0].x
    qt.MaxY = graph.papers[0].y
    qt.MaxR = graph.papers[0].radius
    for _, paper := range graph.papers {
        if (paper.x < qt.MinX) { qt.MinX = paper.x; }
        if (paper.y < qt.MinY) { qt.MinY = paper.y; }
        if (paper.x > qt.MaxX) { qt.MaxX = paper.x; }
        if (paper.y > qt.MaxY) { qt.MaxY = paper.y; }
        if (paper.radius > qt.MaxR) { qt.MaxR = paper.radius; }
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
    for _, paper := range graph.papers {
        QuadTreeInsertPaper(nil, &qt.Root, paper, qt.MinX, qt.MinY, qt.MaxX, qt.MaxY)
    }

    fmt.Printf("quad tree bounding box: (%v,%v) -- (%v,%v)\n", qt.MinX, qt.MinY, qt.MaxX, qt.MaxY)

    // store the quad tree in the graph object
    graph.qt = qt
}

func (q *QuadTreeNode) ApplyIfWithin(MinX, MinY, MaxX, MaxY int, x, y, rx, ry int, f func(paper *Paper)) {
    if q == nil {
    } else if q.Leaf != nil {
        rx += q.Leaf.radius
        ry += q.Leaf.radius
        if x - rx <= q.Leaf.x && q.Leaf.x <= x + rx && y - ry <= q.Leaf.y && q.Leaf.y <= y + ry {
            f(q.Leaf)
        }
    } else if ((MinX <= x - rx && x - rx <= MaxX) || (MinX <= x + rx && x + rx <= MaxX) || (x - rx <= MinX && x + rx >= MaxX)) &&
              ((MinY <= y - ry && y - ry <= MaxY) || (MinY <= y + ry && y + ry <= MaxY) || (y - ry <= MinY && y + ry >= MaxY)) {
        MidX := (MinX + MaxX) / 2
        MidY := (MinY + MaxY) / 2
        q.Q0.ApplyIfWithin(MinX, MinY, MidX, MidY, x, y, rx, ry, f)
        q.Q1.ApplyIfWithin(MidX, MinY, MaxX, MidY, x, y, rx, ry, f)
        q.Q2.ApplyIfWithin(MinX, MidY, MidX, MaxY, x, y, rx, ry, f)
        q.Q3.ApplyIfWithin(MidX, MidY, MaxX, MaxY, x, y, rx, ry, f)
    }
}

func (qt *QuadTree) ApplyIfWithin(x, y, rx, ry int, f func(paper *Paper)) {
    qt.Root.ApplyIfWithin(qt.MinX, qt.MinY, qt.MaxX, qt.MaxY, x, y, rx, ry, f)
}

func DrawTile(graph *Graph, worldWidth, worldHeight, xi, yi, surfWidth, surfHeight int, filename string) {

    //surf := cairo.NewSurface(cairo.FORMAT_RGB24, surfWidth, surfHeight)
    surf := cairo.NewSurface(cairo.FORMAT_ARGB32, surfWidth, surfHeight)
    //surf.SetSourceRGB(4.0/15, 5.0/15, 6.0/15)
    //surf.SetSourceRGB(0, 0, 0)
    surf.SetSourceRGBA(0, 0, 0, 0)
    surf.Paint()

    matrix := new(cairo.Matrix)
    matrix.Xx = float64(surf.GetWidth()) / float64(worldWidth)
    matrix.Yy = float64(surf.GetHeight()) / float64(worldHeight)

    matrix.X0 = -float64(graph.MinX)*matrix.Xx + float64((1-xi)*surf.GetWidth())
    matrix.Y0 = -float64(graph.MinY)*matrix.Yy + float64((1-yi)*surf.GetHeight())

    //fmt.Println("rendering background")
    // simple halo background circle for each paper
    //surf.SetMatrix(*matrix)
    //for _, paper := range graph.papers {
    //    surf.SetSourceRGB(paper.colBG.r, paper.colBG.g, paper.colBG.b)
    //    surf.Arc(float64(paper.x), float64(paper.y), 2 * float64(paper.radius), 0, 2 * math.Pi)
    //    surf.Fill()
    //}

    // area-based background
    //qt := graph.qt
    //surf.IdentityMatrix()
    //matrixInv := *matrix
    //matrixInv.Invert()
    //for v := 0; v + 1 < surf.GetHeight(); v += 2 {
    //    for u := 0; u + 1 < surf.GetWidth(); u += 2 {
    //        x, y := matrixInv.TransformPoint(float64(u), float64(v))
    //        ptR := 0.0
    //        ptG := 0.0
    //        ptB := 0.0
    //        n := 0
    //        qt.ApplyIfWithin(int(x), int(y), 200, 200, func(paper *Paper) {
    //            ptR += paper.colBG.r
    //            ptG += paper.colBG.g
    //            ptB += paper.colBG.b
    //            n += 1
    //        })
    //        if n > 10 {
    //            if n < 20 {
    //                ptR += float64(20 - n) * 4.0/15
    //                ptG += float64(20 - n) * 5.0/15
    //                ptB += float64(20 - n) * 6.0/15
    //                n = 20
    //            }
    //            ptR /= float64(n)
    //            ptG /= float64(n)
    //            ptB /= float64(n)
    //            surf.SetSourceRGB(ptR, ptG, ptB)
    //            surf.Rectangle(float64(u), float64(v), 2, 2)
    //            surf.Fill()
    //        }
    //    }
    //}

    // apply smoothing
    //{
    //    data := surf.GetData()
    //    w := surf.GetStride()
    //    fmt.Println(surf.GetFormat())
    //    data2 := make([]byte, len(data))
    //    for v := 1; v + 1 < surf.GetHeight(); v += 1 {
    //        for u := 1; u + 1 < surf.GetWidth(); u += 1 {
    //            var r, g, b uint
    //            /*
    //            if data[v * w + u * 4 + 0] == 0 && data[v * w + u * 4 + 1] == 0 && data[v * w + u * 4 + 2] == 0 {
    //                r = 5*0x44
    //                g = 5*0x55
    //                b = 5*0x66
    //            } else {
    //                */
    //                b = uint(data[(v - 1) * w + (u + 0) * 4 + 0]) +
    //                    uint(data[(v + 0) * w + (u - 1) * 4 + 0]) +
    //                    uint(data[(v + 0) * w + (u + 0) * 4 + 0]) +
    //                    uint(data[(v + 0) * w + (u + 1) * 4 + 0]) +
    //                    uint(data[(v + 1) * w + (u + 0) * 4 + 0])
    //                g = uint(data[(v - 1) * w + (u + 0) * 4 + 1]) +
    //                    uint(data[(v + 0) * w + (u - 1) * 4 + 1]) +
    //                    uint(data[(v + 0) * w + (u + 0) * 4 + 1]) +
    //                    uint(data[(v + 0) * w + (u + 1) * 4 + 1]) +
    //                    uint(data[(v + 1) * w + (u + 0) * 4 + 1])
    //                r = uint(data[(v - 1) * w + (u + 0) * 4 + 2]) +
    //                    uint(data[(v + 0) * w + (u - 1) * 4 + 2]) +
    //                    uint(data[(v + 0) * w + (u + 0) * 4 + 2]) +
    //                    uint(data[(v + 0) * w + (u + 1) * 4 + 2]) +
    //                    uint(data[(v + 1) * w + (u + 0) * 4 + 2])
    //            //}

    //            data2[v * w + u * 4 + 0] = byte(b / 5)
    //            data2[v * w + u * 4 + 1] = byte(g / 5)
    //            data2[v * w + u * 4 + 2] = byte(r / 5)
    //        }
    //    }
    //    surf.SetData(data2)
    //}

    // Use quadtree to only draw papers within given tile region
    surf.IdentityMatrix()
    matrixInv := *matrix
    matrixInv.Invert()
    x, y := matrixInv.TransformPoint(float64(surfWidth)/2., float64(surfHeight)/2.)
    rx, ry := matrixInv.TransformDistance(float64(surfWidth)/2., float64(surfHeight)/2.)

    // set font
    surf.SelectFontFace("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_NORMAL)
    surf.SetFontSize(35)

    surf.SetMatrix(*matrix)
    surf.SetLineWidth(3)
    // Need to add largest radius to dimensions to ensure we don't miss any papers

    // foreground
    graph.qt.ApplyIfWithin(int(x), int(y), int(rx)+graph.qt.MaxR, int(ry)+graph.qt.MaxR, func(paper *Paper) {
        pixelRadius, _ := matrix.TransformDistance(float64(paper.radius), float64(paper.radius))
        if pixelRadius < 0.2 {
            newRadius, _ := matrixInv.TransformDistance(0.2, 0.2)
            surf.Arc(float64(paper.x), float64(paper.y), newRadius, 0, 2 * math.Pi)
        } else {
            surf.Arc(float64(paper.x), float64(paper.y), float64(paper.radius), 0, 2 * math.Pi)
        }
        surf.SetSourceRGB(paper.colFG.r, paper.colFG.g, paper.colFG.b)
        surf.Fill()
        /* this bit draws a border around each paper; not needed when we have a black background
        surf.FillPreserve()
        surf.SetSourceRGB(0, 0, 0)
        surf.Stroke()
        */
        // this bit draws the keyword of the paper if it'll fit
        if (pixelRadius > 21) {
            surf.SetSourceRGB(1, 1, 1)
            surf.MoveTo(float64(paper.x) - 10 * float64(len(paper.keyword)), float64(paper.y) + 5)
            surf.ShowText(paper.keyword)
        }
    })

    //fmt.Println("writing file")
    if err := os.MkdirAll(filepath.Dir(filename),0755); err != nil {
        fmt.Println(err)
        return
    }

    // save with full colours
    surf.WriteToPNG(filename+".png")

    //fo, _ := os.Create(filename+"_v2.png")
    //defer fo.Close()
    //w := bufio.NewWriter(fo)
    //err:= png.Encode(w, surf.GetImage())
    //if err != nil {
    //    fmt.Println(err)
    //}
    //w.Flush()

    // TODO save grayscale (or dimmer version)
    //surf.WriteToPNG(filename+"g.png")
    surf.Finish()

}

func ParallelDrawTile(graph *Graph, outPrefix string, depth, worldDim, xiFirst, xiLast, yiFirst, yiLast int, channel chan int) {
    suffix := ""
    if *flagGrayScale {
        suffix = "-bw"
    }
    for xi := xiFirst; xi <= xiLast; xi++ {
        for yi := yiFirst; yi <= yiLast; yi++ {
            //filename := fmt.Sprintf("%stiles/%d-%d/tile_%d-%d_%d-%d.png",outPrefix,divs,divs,divs,divs,xi,yi)
            filename := fmt.Sprintf("%s/tiles%s/%d/%d/%d", outPrefix, suffix, depth, xi, yi)
            DrawTile(graph, worldDim, worldDim, xi, yi, TILE_PIXEL_LEN, TILE_PIXEL_LEN, filename)
        }
    }
    channel <- 1 // signal that this set of tiles is done
}

func GenerateAllTiles(graph *Graph, outPrefix string) {
    indexFile := outPrefix + "/tiles/tile_index.json"
    if err := os.MkdirAll(filepath.Dir(indexFile),0755); err != nil {
        fmt.Println(err)
        return
    }
    fo, _ := os.Create(indexFile)
    defer fo.Close()
    w := bufio.NewWriter(fo)

    sort.Sort(PaperSortId(graph.papers))
    latestId := graph.papers[0].id

    fmt.Fprintf(w,"tile_index(\"latestid\":%d,\"xmin\":%d,\"ymin\":%d,\"xmax\":%d,\"ymax\":%d,\"pixelw\":%d,\"pixelh\":%d,\"tilings\":[",flag.Arg(0),latestId,graph.MinX,graph.MinY,graph.MaxX,graph.MaxY,TILE_PIXEL_LEN,TILE_PIXEL_LEN)

    divisionSet := [...]int{4,8,24,72,216}
    //divisionSet := [...]int{4,8,24}

    //depths := *flagTileDepth
    first := true
    //var depth uint
    //for depth = 0; depth <= depths; depth++ {
    for depth, divs := range divisionSet {
        //divs := int(math.Pow(2.,float64(depth)))
        worldDim := int(math.Max(float64(graph.BoundsX)/float64(divs), float64(graph.BoundsY)/float64(divs)))

        if !first {
             fmt.Fprintf(w,",")
        }
        first = false
        fmt.Fprintf(w,"{\"z\":%d,\"tw\":%d,\"th\":%d,\"nx\":%d,\"ny\":%d}",depth, worldDim, worldDim, divs,divs)

        if !*flagSkipTiles {
            fmt.Printf("Generating tiles at depth %d\n",divs)
            // TODO if graph far from from square, shorten tile directions accordingly

            // parallelise the drawing of tiles, using as many cpus as we have available to us
            maxCpu := runtime.NumCPU()
            if *flagMaxCores > 0 && *flagMaxCores < maxCpu {
                maxCpu = *flagMaxCores
            }
            runtime.GOMAXPROCS(maxCpu)
            channel := make(chan int, maxCpu)
            numRoutines := 0
            xiPerCpu := (divs + maxCpu - 1) / maxCpu
            for xi := 1; xi <= divs; {
                xiLast := xi + xiPerCpu - 1
                if xiLast > divs {
                    xiLast = divs
                }
                go ParallelDrawTile(graph, outPrefix, depth, worldDim, xi, xiLast, 1, divs, channel)
                numRoutines += 1
                xi = xiLast + 1
            }
            // drain the channel
            for i := 0; i < numRoutines; i++ {
                <-channel // wait for one task to complete
            }
            // all tasks are finished
        }
    }
    fmt.Fprintf(w,"]})")
    w.Flush()
}
