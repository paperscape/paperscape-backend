package main

import (
    "flag"
    "os"
    "bufio"
    "fmt"
    "path/filepath"
    "strings"
    "strconv"
    "GoMySQL"
    "runtime"
    "math"
    "sort"
    "log"
    "xiwi"
    "github.com/ungerik/go-cairo"
    "time"
)

var GRAPH_PADDING = 100 // what to pad graph by on each side
var TILE_PIXEL_LEN = 256

var flagDB         = flag.String("db", "", "MySQL database to connect to")
var flagGrayScale  = flag.Bool("gs", false, "Make grayscale tiles")
var flagHeatMap    = flag.Bool("hm", false, "Make heatmap tiles")
var flagDoSingle   = flag.Bool("single-tile", false, "Only generate a large single tile, no labels or world index information")

var flagSkipTiles  = flag.Bool("skip-tiles", false, "Do not generate tiles (still generates index information)")
var flagSkipLabels = flag.Bool("skip-labels", false, "Do not generate labels (still generates index information)")

var flagMaxCores = flag.Int("cores",-1,"Max number of system cores to use, default is all of them")

func main() {
    // parse command line options
    flag.Parse()

    if flag.NArg() != 1 {
        log.Fatal("need to specify output prefix")
    }
    outPrefix := flag.Arg(0)

    // connect to the db
    db := xiwi.ConnectToDB(*flagDB)
    if db == nil {
        return
    }
    defer db.Close()

    // read in the graph
    graph := ReadGraph(db)

    // build the quad tree
    graph.BuildQuadTree()
    
    runtime.GC()

    if *flagDoSingle {
        DrawTile(graph, graph.BoundsX, graph.BoundsY, 1, 1, 18000, 18000, outPrefix)
    } else {
        // Create index file
        indexFile := outPrefix + "/world_index.json"

        if err := os.MkdirAll(filepath.Dir(indexFile),0755); err != nil {
            fmt.Println(err)
            return
        }
        fo, _ := os.Create(indexFile)
        defer fo.Close()
        w := bufio.NewWriter(fo)

        num_papers := len(graph.papers)

        fmt.Fprintf(w,"world_index({\"latestid\":%d,\"numpapers\":%d,\"xmin\":%d,\"ymin\":%d,\"xmax\":%d,\"ymax\":%d,\"pixelw\":%d,\"pixelh\":%d",graph.LatestId,num_papers,graph.MinX,graph.MinY,graph.MaxX,graph.MaxY,TILE_PIXEL_LEN,TILE_PIXEL_LEN)

        graph.QueryNewPapersId(db)
        if graph.NewPapersId != 0 {
            fmt.Fprintf(w,",\"newid\":%d",graph.NewPapersId)
        }

        graph.QueryLastMetaDownload(db)
        if graph.LastMetaDownload != "" {
            fmt.Fprintf(w,",\"lastdl\":\"%s\"",graph.LastMetaDownload)
        }

        GenerateAllTiles(graph, w, outPrefix)
        runtime.GC()

        GenerateAllLabelZones(graph, w, outPrefix)

        fmt.Fprintf(w,"})")
        w.Flush()
    }
}

type CairoColor struct {
    r, g, b float32
}

type CategoryLabel struct {
    x,y,radius int
    label string
}

type Paper struct {
    id          uint
    x           int
    y           int
    radius      int
    age         float32
    heat        float32
    col         CairoColor
    maincat     string
    label       string
}

type PaperSortId []*Paper
func (p PaperSortId) Len() int           { return len(p) }
func (p PaperSortId) Less(i, j int) bool { return p[i].id < p[j].id }
func (p PaperSortId) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }


type Graph struct {
    papers  []*Paper
    qt      *QuadTree
    catLabels []*CategoryLabel
    MinX, MinY, MaxX, MaxY int
    BoundsX, BoundsY int
    LatestId, NewPapersId uint
    LastMetaDownload string
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

func cleanJsonString(input string) string {
    // TODO work out exactly which chars are causing
    // parsing error and blacklist or escape them
    // inplace of this whitelist
    validChars := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789 -/.,<>"

    output := make([]rune, 0)

    for _, r := range input {
        if strings.ContainsRune(validChars, r) {
            output = append(output, r)
        }
    }

    return string(output)
}

func idToDaysAgo(id uint) uint {
    tId := time.Date(((int(id) / 10000000) + 1800),time.Month((((int(id) % 10000000) / 625000) + 1)),(((int(id) % 625000) / 15625) + 1),0,0,0,0,time.UTC)
    days := uint(time.Now().Sub(tId).Hours()/24)
    return days
}

func (graph *Graph) CalculateCategoryLabels() {
    categories := []struct{
        maincat, label string
    }{
        {"hep-th","high energy theory,(hep-th),,"},
        {"hep-ph","high energy phenomenology,(hep-ph),,"},
        {"hep-ex","high energy experiment,(hep-ex),,"},
        {"gr-qc","general relativity/quantum cosmology,(gr-gc),,"},
        //{"astro-ph.GA","astronomy,astro-ph.GA,,"},
        {"hep-lat","high energy lattice,(hep-lat),,"},
        //{"astro-ph.CO","cosmology,astro-ph.CO,,"},
        {"astro-ph","astrophysics,(astro-ph),,"},
        {"cond-mat","condensed matter,(cond-mat),,"},
        {"quant-ph","quantum physics,(quant-ph),,"},
        {"physics","general physics,(physics),,"},
    }

    for _, category := range(categories) {

        var sumMass, sumMassX, sumMassY int64
        var minX,minY,maxX,maxY int
        for _, paper := range(graph.papers) {
            if paper.maincat == category.maincat {
                mass := int64(paper.radius*paper.radius)
                sumMass += mass
                sumMassX += mass*int64(paper.x)
                sumMassY += mass*int64(paper.y)
                if paper.x < minX { minX = paper.x }
                if paper.x > maxX { maxX = paper.x }
                if paper.y < minY { minY = paper.y }
                if paper.y > maxY { maxY = paper.y }
            }
        }
        if sumMass > 0 {
            label := new(CategoryLabel)
            label.label = category.label

            label.x = int(sumMassX/sumMass)
            label.y = int(sumMassY/sumMass)
            label.radius = int(math.Sqrt(math.Pow(float64(maxX-minX),2) + math.Pow(float64(maxY-minY),2))/2)

            graph.catLabels = append(graph.catLabels,label)
        }
    }
}


func (graph *Graph) QueryLastMetaDownload(db *mysql.Client) {

    // execute the query
    query := fmt.Sprintf("SELECT value FROM misc WHERE field = \"lastmetadownload\"")
    err := db.Query(query)
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

    defer db.FreeResult()

    var ok bool
    var value string

    row := result.FetchRow()
    if row == nil {
        fmt.Println("MySQL row error;", err)
        return
    }

    if value, ok = row[0].(string); !ok {
        fmt.Println("MySQL id cast error;", err)
        return
    }
    
    pieces := strings.Split(value, "-")
    if len(pieces) == 3 {
        year,_ := strconv.ParseInt(pieces[0],10,0)
        month,_ := strconv.ParseInt(pieces[1],10,0)
        day,_ := strconv.ParseInt(pieces[2],10,0)
        //t := time.Date(int(year),time.Month(int(month)),int(day),0,0,0,0,time.UTC)
        value = fmt.Sprintf("%d %s %d",day,time.Month(int(month)),year)
    }

    graph.LastMetaDownload = value

}

func (graph *Graph) QueryNewPapersId(db *mysql.Client) {

    // execute the query
    query := fmt.Sprintf("SELECT max(datebdry.id) FROM datebdry WHERE datebdry.id < %d",graph.LatestId)
    err := db.Query(query)
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

    defer db.FreeResult()

    var ok bool
    var id uint64

    row := result.FetchRow()
    if row == nil {
        fmt.Println("MySQL row error;", err)
        return
    }

    if id, ok = row[0].(uint64); !ok {
        fmt.Println("MySQL id cast error;", err)
        return
    }
    
    graph.NewPapersId = uint(id)
}

func getLE16(blob []byte, i int) uint {
    return uint(blob[i]) | (uint(blob[i + 1]) << 8)
}
func getLE32(blob []byte, i int) uint {
    return uint(blob[i]) | (uint(blob[i + 1]) << 8) | (uint(blob[i + 2]) << 16) | (uint(blob[i + 3]) << 24)
}

func (graph *Graph) QueryHeat(db *mysql.Client) {

    // execute the query
    err := db.Query("SELECT id,numCites,cites FROM pcite")
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

    // for normalizing heat
    var maxHeat float32
    //var totalHeat float32

    // get each row from the result
    for {
        row := result.FetchRow()
        if row == nil {
            break
        }

        var ok bool
        var id uint64
        var numCites uint64
        var citesBlob []byte
        if id, ok = row[0].(uint64); !ok { continue }
        if numCites, ok = row[1].(uint64); !ok { continue }
        if citesBlob, ok = row[2].([]byte); !ok { continue }

        citeIds := make([]uint,0)

        for i := 0; i < len(citesBlob); i += 10 {
            citeIds = append(citeIds,getLE32(citesBlob, i))
            getLE16(citesBlob, i + 4) //citeOrder
            getLE16(citesBlob, i + 6) //citeFreq
            getLE16(citesBlob, i + 8) //numcites
        }

        var heat float32

        lifetime := float64(365.)

        for _, citeId := range(citeIds) {
            citeHeat := float32(math.Exp(-float64(idToDaysAgo(citeId))/lifetime))
            heat += citeHeat
            //fmt.Printf("%d,%f,%f,%f\n",citeId,float64(idToDaysAgo(citeId)),citeHeat,heat)
        }

        if heat > maxHeat { maxHeat = heat }

        //heat = math.Pow(float64(numRecentCites)/float64(numCites),1/4)
        //heat = float64(numRecentCites)
        //totalHeat += heat
        //if heat > 0 { total = heat }

        paper := graph.GetPaperById(uint(id))
        if paper != nil && numCites > 0 {
            paper.heat = heat/float32(numCites)
        }
    }

    // normalize heat
    for _, paper := range (graph.papers) {
        paper.heat /= maxHeat
    }
    //fmt.Printf("max %f\n",maxHeat)

    db.FreeResult()

    fmt.Println("read heat from cites")
}

func (graph *Graph) QueryCategories(db *mysql.Client) {

    // execute the query
    err := db.Query("SELECT id,maincat FROM meta_data")
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
        if id, ok = row[0].(uint64); !ok { continue }
        if maincat, ok = row[1].(string); !ok { continue }

        paper := graph.GetPaperById(uint(id))
        if paper != nil {
            paper.maincat = maincat
        }
    }

    db.FreeResult()

}

func (graph *Graph) QueryLabels(db *mysql.Client) {
    // execute the query
    err := db.Query("SELECT meta_data.id,keywords.keywords,meta_data.authors FROM meta_data,keywords WHERE meta_data.id = keywords.id")
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
        var keywords []byte
        //var allcats string
        var authors string
        if id, ok = row[0].(uint64); !ok { continue }
        if keywords, ok = row[1].([]byte); !ok { continue }
        //if allcats, ok = row[2].(string); !ok { continue }
        if row[2] == nil {
            continue
        } else if au, ok := row[2].([]byte); !ok {
            continue
        } else {
            authors = string(au)
        }

        paper := graph.GetPaperById(uint(id))
        if paper != nil {
            paper.DetermineLabel(authors,string(keywords))
        }
    }

    db.FreeResult()
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
    //papers := make([]*Paper, numPapers)
    papers := make([]*Paper, 0)

    // execute the query
    //err = db.Query("SELECT map_data.id,map_data.x,map_data.y,map_data.r,keywords.keywords FROM map_data,keywords WHERE map_data.id = keywords.id")
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

        papers = append(papers,MakePaper(uint(id), int(x), int(y), int(r)))
    }

    db.FreeResult()

    // make sure papers are sorted!
    sort.Sort(PaperSortId(papers))

    // calculate age
    for index, paper := range(papers) {
        paper.age = float32(index) / float32(numPapers)
    }

    if len(papers) != int(numPapers) {
        fmt.Println("could not read all papers from map_data; wanted", numPapers, "got", len(papers))
        return nil
    }

    return papers
}

func MakePaper(id uint, x int, y int, radius int) *Paper {
    paper := new(Paper)
    paper.id = id
    paper.x = x
    paper.y = y
    paper.radius = radius

    return paper
}

func (paper *Paper) DetermineLabel(authors, keywords string) {
    // work out author string; maximum 2 authors
    aus := strings.SplitN(authors, ",", 3)
    for i, au := range aus {
        // get the last name
        parts := strings.Split(au, ".")
        aus[i] = parts[len(parts) - 1]
    }
    var auStr string
    if len(aus) <= 1 {
        // 0 or 1 author
        auStr = aus[0] + ","
    } else if len(aus) == 2 {
        // 2 authors
        auStr = aus[0] + "," + aus[1]
    } else {
        // 3 or more authors
        auStr = aus[0] + ",et al."
    }

    // work out keyword string; maximum 2 keywords
    kws := strings.SplitN(keywords, ",", 3)
    var kwStr string
    if len(kws) <= 1 {
        // 0 or 1 keywords
        kwStr = keywords + ","
    } else if len(kws) == 2 {
        // 2 keywords
        kwStr = keywords
    } else {
        // 3 or more keywords
        kwStr = kws[0] + "," + kws[1]
    }

    paper.label = cleanJsonString(kwStr + "," + auStr)
}

func (paper *Paper) SetColour() {
    // basic colour of paper
    var r, g, b float32


    if *flagHeatMap {
        
        // Try pure heatmap instead
        var coldR, coldG, coldB, hotR, hotG, hotB float32

        coldR, coldG, coldB = 0, 0, 1
        hotR, hotG, hotB = 1, 0, 0
        r = (hotR - coldR)*paper.heat + coldR
        g = (hotG - coldG)*paper.heat + coldG
        b = (hotB - coldB)*paper.heat + coldB
    
    } else {

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
        } else if paper.maincat == "cond-mat" {
            r, g, b = 0.6, 0.4, 0.4
        } else if paper.maincat == "quant-ph" {
            r, g, b = 0.4, 0.7, 0.7
        } else if paper.maincat == "physics" {
            r, g, b = 0, 0.5, 0 // dark green
        } else {
            r, g, b = 0.7, 1, 0.3
        }

        // older papers are more saturated in colour
        var age float32 = paper.age

        // foreground colour; select one by making it's if condition true
        if (false) {
            // older papers are saturated, newer papers are coloured
            var saturation float32 = 0.4 * (1 - age)
            r = saturation + (r) * (1 - saturation)
            g = saturation + (g) * (1 - saturation)
            b = saturation + (b) * (1 - saturation)
        } else if (false) {
            // older papers are saturated, newer papers are coloured and tend towards a full red component
            var saturation float32 = 0.4 * (1 - age)
            age = age * age
            r = saturation + (r * (1 - age) + age) * (1 - saturation)
            g = saturation + (g * (1 - age)      ) * (1 - saturation)
            b = saturation + (b * (1 - age)      ) * (1 - saturation)
        } else if (true) {
            // older papers are saturated and dark, newer papers are coloured and bright
            //saturation := 0.4 * (1 - age)
            var saturation float32 = 0.0
            var dim_factor float32 = 0.4 + 0.6 * float32(math.Exp(float64(-10*age*age)))
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
    }

    paper.col = CairoColor{r, g, b}
}

func ReadGraph(db *mysql.Client) *Graph {
    graph := new(Graph)

    // load positions from the data base
    graph.papers = QueryPapers(db)
    if graph.papers == nil {
        log.Fatal("could not read papers from db")
    }
    fmt.Printf("read %v papers from db\n", len(graph.papers))

    graph.QueryCategories(db)

    if !*flagSkipLabels {
        graph.QueryLabels(db)
        graph.CalculateCategoryLabels()
    }
   
    // determine labels to use for each paper
    //for _, paper := range graph.papers {
    //    paper.DetermineLabel()
    //}

    if *flagHeatMap {
        graph.QueryHeat(db)
    }

    for _, paper := range graph.papers {
        if paper.x - paper.radius < graph.MinX { graph.MinX = paper.x - paper.radius }
        if paper.y - paper.radius < graph.MinY { graph.MinY = paper.y - paper.radius }
        if paper.x + paper.radius > graph.MaxX { graph.MaxX = paper.x + paper.radius }
        if paper.y + paper.radius > graph.MaxY { graph.MaxY = paper.y + paper.radius }
    }

    graph.MinX -= GRAPH_PADDING
    graph.MaxX += GRAPH_PADDING
    graph.MinY -= GRAPH_PADDING
    graph.MaxY += GRAPH_PADDING

    graph.BoundsX = graph.MaxX - graph.MinX
    graph.BoundsY = graph.MaxY - graph.MinY

    graph.LatestId = graph.papers[len(graph.papers)-1].id

    for _, paper := range graph.papers {
        paper.SetColour()
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

    surf := cairo.NewSurface(cairo.FORMAT_ARGB32, surfWidth, surfHeight)
    surf.SetSourceRGBA(0, 0, 0, 0)
    surf.Paint()

    matrix := new(cairo.Matrix)
    matrix.Xx = float64(surf.GetWidth()) / float64(worldWidth)
    matrix.Yy = float64(surf.GetHeight()) / float64(worldHeight)

    matrix.X0 = -float64(graph.MinX)*matrix.Xx + float64((1-xi)*surf.GetWidth())
    matrix.Y0 = -float64(graph.MinY)*matrix.Yy + float64((1-yi)*surf.GetHeight())

    // Use quadtree to only draw papers within given tile region
    surf.IdentityMatrix()
    matrixInv := *matrix
    matrixInv.Invert()
    x, y := matrixInv.TransformPoint(float64(surfWidth)/2., float64(surfHeight)/2.)
    rx, ry := matrixInv.TransformDistance(float64(surfWidth)/2., float64(surfHeight)/2.)

    // set font
    //surf.SelectFontFace("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_NORMAL)
    //surf.SetFontSize(35)

    surf.SetMatrix(*matrix)
    surf.SetLineWidth(3)
    // Need to add largest radius to dimensions to ensure we don't miss any papers

    // foreground
    graph.qt.ApplyIfWithin(int(x), int(y), int(rx)+graph.qt.MaxR, int(ry)+graph.qt.MaxR, func(paper *Paper) {
        pixelRadius, _ := matrix.TransformDistance(float64(paper.radius), float64(paper.radius))
        if pixelRadius < 0.09 {
            newRadius, _ := matrixInv.TransformDistance(0.09, 0.09)
            surf.Arc(float64(paper.x), float64(paper.y), newRadius, 0, 2 * math.Pi)
        } else {
            surf.Arc(float64(paper.x), float64(paper.y), float64(paper.radius), 0, 2 * math.Pi)
        }
        surf.SetSourceRGB(float64(paper.col.r), float64(paper.col.g), float64(paper.col.b))
        surf.Fill()
    })

    if err := os.MkdirAll(filepath.Dir(filename),0755); err != nil {
        fmt.Println(err)
        return
    }

    // save with full colours
    surf.WriteToPNG(filename+".png")

    surf.Finish()

}

func GenerateLabelZone(graph *Graph, scale, width, height, depth, xi, yi int, showCategories bool, filename string) {

    if err := os.MkdirAll(filepath.Dir(filename),0755); err != nil {
        log.Fatal(err)
    }

    fo, _ := os.Create(filename+".json")
    defer fo.Close()
    w := bufio.NewWriter(fo)

    // Get midpoint of zone
    rx := width/2
    ry := height/2
    x  := graph.MinX + (xi-1)*int(width) + rx
    y  := graph.MinY + (yi-1)*int(height) + ry

    // TODO consider adding depth, x, y, width, height etc.
    // Tho in practice should already have this info before d/l label zone
    fmt.Fprintf(w,"lz_%d_%d_%d({\"scale\":%d,\"lbls\":[",depth,xi,yi,scale)

    min_rad := int(float32(scale)*0.01)

    first := true
    graph.qt.ApplyIfWithin(x, y, rx, ry, func(paper *Paper) {
        if paper.label != "" && paper.radius > min_rad {
            if first {
                first = false
            } else {
                fmt.Fprintf(w,",")
            }
            fmt.Fprintf(w,"{\"x\":%d,\"y\":%d,\"r\":%d,\"lbl\":\"%s\"}",paper.x,paper.y,paper.radius,paper.label)
        }
    })

    if showCategories {
        for _, catLabel := range(graph.catLabels) {
            if catLabel.x > x-rx && catLabel.x < x+rx && catLabel.y > y-ry && catLabel.y < y+ry {
                if first {
                    first = false
                } else {
                    fmt.Fprintf(w,",")
                }
                fmt.Fprintf(w,"{\"x\":%d,\"y\":%d,\"r\":%d,\"lbl\":\"%s\"}",catLabel.x,catLabel.y,catLabel.radius,catLabel.label)
            }
        }
    }

    fmt.Fprintf(w,"]})")
    w.Flush()
}

func ParallelDrawTile(graph *Graph, outPrefix string, depth, worldDim, xiFirst, xiLast, yiFirst, yiLast int, channel chan int) {
    suffix := ""
    if *flagGrayScale {
        suffix = "-bw"
    }
    if *flagHeatMap {
        // TODO put back and implement on boa
        //suffix = "-hm"
        suffix = ""
    }
    for xi := xiFirst; xi <= xiLast; xi++ {
        for yi := yiFirst; yi <= yiLast; yi++ {
            filename := fmt.Sprintf("%s/tiles%s/%d/%d/%d", outPrefix, suffix, depth, xi, yi)
            DrawTile(graph, worldDim, worldDim, xi, yi, TILE_PIXEL_LEN, TILE_PIXEL_LEN, filename)
        }
    }
    channel <- 1 // signal that this set of tiles is done
}

func ParallelGenerateLabelZone(graph *Graph, outPrefix string, depth, scale, width, height, xiFirst, xiLast, yiFirst, yiLast int, showCategories bool, channel chan int) {
    for xi := xiFirst; xi <= xiLast; xi++ {
        for yi := yiFirst; yi <= yiLast; yi++ {
            filename := fmt.Sprintf("%s/zones/%d/%d/%d", outPrefix, depth, xi, yi)
            GenerateLabelZone(graph, scale, width, height, depth, xi, yi, showCategories, filename)
        }
    }
    channel <- 1 // signal that this set of tiles is done
}

func GenerateAllTiles(graph *Graph, w *bufio.Writer, outPrefix string) {

    fmt.Fprintf(w,",\"tilings\":[")

    divisionSet := [...]int{4,8,24,72,216}
    //divisionSet := [...]int{4,8,24,72}
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
    fmt.Fprintf(w,"]")
}

func GenerateAllLabelZones(graph *Graph, w *bufio.Writer, outPrefix string) {

    fmt.Fprintf(w,",\"zones\":[")

    // tile divisions, scale divisions
    depthSet := []struct {
        tdivs, sdivs uint  
        showCats bool
    }{
        {1,1,true},
        {1,2,true},
        {1,4,true},
        {1,8,true},
        {2,16,false},
        {4,32,false},
        {8,64,false},
        {16,128,false},
        {32,256,false},
    }

    first := true

    for depth, labelDepth := range depthSet {
        tile_width := int(math.Max(float64(graph.BoundsX)/float64(labelDepth.tdivs), float64(graph.BoundsY)/float64(labelDepth.tdivs)))
        tile_height := tile_width

        // typical scale of tile
        scale := int(math.Max(float64(graph.BoundsX)/float64(labelDepth.sdivs), float64(graph.BoundsY)/float64(labelDepth.sdivs)))

        if !first {
             fmt.Fprintf(w,",")
        }
        first = false
        fmt.Fprintf(w,"{\"z\":%d,\"s\":%d,\"w\":%d,\"h\":%d,\"nx\":%d,\"ny\":%d}",depth, scale, tile_width, tile_height,labelDepth.tdivs,labelDepth.tdivs)

        if !*flagSkipLabels {
            fmt.Printf("Generating label zones at depth %d\n",depth)
            // TODO if graph far from from square, shorten tile directions accordingly

            // parallelise the drawing of zones, using as many cpus as we have available to us
            maxCpu := runtime.NumCPU()
            if *flagMaxCores > 0 && *flagMaxCores < maxCpu {
                maxCpu = *flagMaxCores
            }
            runtime.GOMAXPROCS(maxCpu)
            channel := make(chan int, maxCpu)
            numRoutines := 0
            xiPerCpu := (int(labelDepth.tdivs) + maxCpu - 1) / maxCpu
            for xi := 1; xi <= int(labelDepth.tdivs); {
                xiLast := xi + xiPerCpu - 1
                if xiLast > int(labelDepth.tdivs) {
                    xiLast = int(labelDepth.tdivs)
                }
                go ParallelGenerateLabelZone(graph, outPrefix, depth, scale, tile_width, tile_height, xi, xiLast, 1, int(labelDepth.tdivs),labelDepth.showCats, channel)
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
    fmt.Fprintf(w,"]")
}
