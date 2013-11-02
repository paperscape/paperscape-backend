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
    "encoding/json"
)

var GRAPH_PADDING = 100 // what to pad graph by on each side
var TILE_PIXEL_LEN = 512

var COLOUR_NORMAL    = 0
var COLOUR_HEATMAP   = 1
var COLOUR_GRAYSCALE = 2

var flagDB           = flag.String("db", "", "MySQL database to connect to")
var flagDBLocSuffix  = flag.String("db-suffix", "", "Suffix of location table in MySQL database: map_data_{suffix}")
var flagJSONLocFile  = flag.String("json-layout", "", "Read paper locations from JSON file instead of DB")

var flagGrayScale  = flag.Bool("gs", false, "Also make grayscale tiles")
var flagHeatMap    = flag.Bool("hm", false, "Also make heatmap tiles")

var flagDoSingle   = flag.String("single-image", "", "Generate a large single image with <WxHxZoom> parameters, eg 100x100x2.5")
var flagDoPoster   = flag.Bool("poster", false, "Generate an image suitable for printing as a poster")

var flagRegionFile = flag.String("region-file", "regions.json", "JSON file with region labels")

var flagSkipNormalTiles  = flag.Bool("skip-tiles", false, "Do not generate normal tiles (still generates index information)")
var flagSkipLabels = flag.Bool("skip-labels", false, "Do not generate labels (still generates index information)")

var flagMaxCores = flag.Int("cores", -1, "Max number of system cores to use, default is all of them")

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
    graph := ReadGraph(db, *flagJSONLocFile)

    // build the quad tree
    graph.BuildQuadTree()

    runtime.GC()

    if len(*flagDoSingle) != 0 {
        geom := strings.Split(*flagDoSingle, "x")
        if len(geom) != 3 {
            log.Fatal("parameters for single-image must be of form WxHxZoom, eg 100x100x2.5")
        }
        resx, _ := strconv.ParseUint(geom[0], 10, 32)
        resy, _ := strconv.ParseUint(geom[1], 10, 32)
        zoomFactor, _ := strconv.ParseFloat(geom[2], 64)
        DrawSingleImage(graph, int(resx), int(resy), zoomFactor, outPrefix, COLOUR_NORMAL)
    } else if *flagDoPoster {
        // A0 at 300 dpi: 9933 x 14043
        // A0 at 72 dpi: 2348 x 3370 
        //resy := 9933; resx := 14043
        resy := 2348; resx := 3370
        DrawPoster(graph, resx, resy, outPrefix, COLOUR_NORMAL)
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

        fmt.Fprintf(w,"world_index({\"dbsuffix\":\"%s\",\"latestid\":%d,\"numpapers\":%d,\"xmin\":%d,\"ymin\":%d,\"xmax\":%d,\"ymax\":%d,\"pixelw\":%d,\"pixelh\":%d",*flagDBLocSuffix,graph.LatestId,num_papers,graph.MinX,graph.MinY,graph.MaxX,graph.MaxY,TILE_PIXEL_LEN,TILE_PIXEL_LEN)

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
    X,Y,Radius int
    label string
}

// need to have members start with upper case so json parser loads them
type RegionLabel struct {
    X, Y, Radius int
    Label string
}

type Paper struct {
    id          uint
    x           int
    y           int
    radius      int
    age         float32
    heat        float32
    //col         CairoColor
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
    regLabels []RegionLabel
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
    validChars := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789 -/.,<>()="

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
        {"hep-lat","high energy lattice,(hep-lat),,"},
        {"astro-ph","astrophysics,(astro-ph),,"},
        {"cond-mat","condensed matter,(cond-mat),,"},
        {"math-ph","mathematical physics,(math-ph),,"},
        {"math","mathematics,(math),,"},
        {"cs","computer science,(cs),,"},
        {"nucl-ex","nuclear experiment,(nucl-ex),,"},
        //{"nucl-th","nuclear theory,(nucl-th),,"},
        {"quant-ph","quantum physics,(quant-ph),,"},
        //{"physics","general physics,(physics),,"}, this cat is so scattered that its centre of mass is no good
        {"q-bio","quantitative biology,(q-bio),,"},
        {"q-fin","quantitative finance,(q-fin),,"},
        {"stat","statistics,(stat),,"},
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

            label.X = int(sumMassX/sumMass)
            label.Y = int(sumMassY/sumMass)
            label.Radius = int(math.Sqrt(math.Pow(float64(maxX-minX),2) + math.Pow(float64(maxY-minY),2))/2)

            graph.catLabels = append(graph.catLabels,label)
        }
    }
}

// read layout in form [[id,x,y,r],...] from JSON file
func ReadPaperLocationFromJSON(filename string) []*Paper {
    fmt.Printf("reading paper layout from JSON file %v\n", filename)

    // open JSON file
    file, err := os.Open(filename)
    if err != nil {
        log.Println(err)
        return nil
    }

    // decode JSON
    dec := json.NewDecoder(file)
    var layout [][]int
    if err := dec.Decode(&layout); err != nil {
        log.Println(err)
        return nil
    }

    // close file
    file.Close()

    // build paper array
    papers := make([]*Paper, 0)
    for _, item := range layout {
        papers = append(papers, MakePaper(uint(item[0]), item[1], item[2], item[3]))
    }

    // make sure papers are sorted!
    sort.Sort(PaperSortId(papers))

    // calculate age
    for index, paper := range(papers) {
        paper.age = float32(index) / float32(len(papers))
    }

    // print info
    fmt.Printf("read %v paper positions\n", len(layout))

    return papers
}

func (graph *Graph) ReadRegionLabels(filename string) {
    // open JSON file
    file, err := os.Open(filename)
    if err != nil {
        log.Println(err)
        return
    }

    // decode JSON
    dec := json.NewDecoder(file)
    if err := dec.Decode(&graph.regLabels); err != nil {
        log.Println(err)
        return
    }

    // close file
    file.Close()

    // print info
    fmt.Printf("read %v region labels\n", len(graph.regLabels))
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

        //lifetime := float64(365.)
        sigma := float64(365.)
        //sqrtTwoPi := float32(math.Sqrt(6.283185))

        for _, citeId := range(citeIds) {
            // exponential decay
            //citeHeat := float32(math.Exp(-float64(idToDaysAgo(citeId))/lifetime))
            // gaussian
            citeHeat := float32(math.Exp(-math.Pow(float64(idToDaysAgo(citeId))/sigma,2)/2.))
            heat += citeHeat
            //fmt.Printf("%d,%f,%f,%f\n",citeId,float64(idToDaysAgo(citeId)),citeHeat,heat)
        }


        //heat = math.Pow(float64(numRecentCites)/float64(numCites),1/4)
        //heat = float64(numRecentCites)
        //totalHeat += heat
        //if heat > 0 { total = heat }

        paper := graph.GetPaperById(uint(id))
        if paper != nil && numCites > 0 {
            paper.heat = heat/float32(numCites)
            if paper.heat > maxHeat { maxHeat = paper.heat }
        }
    
    }

    // normalize heat
    for _, paper := range (graph.papers) {
        paper.heat /= maxHeat
    }
    fmt.Printf("max %f\n",maxHeat)

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
        //var allcats string
        if id, ok = row[0].(uint64); !ok { continue }
        if maincat, ok = row[1].(string); !ok { continue }
        //allcats, ok = row[2].(string)

        paper := graph.GetPaperById(uint(id))
        if paper != nil {
            paper.maincat = maincat
            /* code for if we want to distinguish sub-cats
            if strings.HasPrefix(allcats, "astro-ph.CO") {
                paper.maincat = "astro-ph.CO"
            } else if strings.HasPrefix(allcats, "astro-ph.EP") {
                paper.maincat = "astro-ph.EP"
            } else if strings.HasPrefix(allcats, "astro-ph.GA") {
                paper.maincat = "astro-ph.GA"
            } else if strings.HasPrefix(allcats, "astro-ph.HE") {
                paper.maincat = "astro-ph.HE"
            } else if strings.HasPrefix(allcats, "astro-ph.IM") {
                paper.maincat = "astro-ph.IM"
            } else if strings.HasPrefix(allcats, "astro-ph.SR") {
                paper.maincat = "astro-ph.SR"
            }
            */
        }
    }

    db.FreeResult()

}

func (graph *Graph) QueryLabels(db *mysql.Client) {
    // execute the query
    err := db.Query("SELECT id,keywords,authors FROM meta_data")
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
    
    loc_table := "map_data"
    if *flagDBLocSuffix != "" {
        loc_table += "_" + *flagDBLocSuffix
    }

    // count number of papers
    err := db.Query("SELECT count(id) FROM " + loc_table)
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
    err = db.Query("SELECT id,x,y,r FROM " + loc_table)
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
        fmt.Println("could not read all papers from",loc_table,"; wanted", numPapers, "got", len(papers))
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

func (paper *Paper) GetColour(colourScheme int) *CairoColor {
    // basic colour of paper
    col := new(CairoColor)

    if colourScheme == COLOUR_HEATMAP {
        
        // Try pure heatmap instead
        var coldR, coldG, coldB, hotR, hotG, hotB, dim float32
        
        dim = 0.10
        coldR, coldG, coldB = dim, dim, dim 
        hotR, hotG, hotB = 1, dim, dim
        //col.r = (hotR - coldR)*paper.heat + coldR
        //col.g = (hotG - coldG)*paper.heat + coldG
        //col.b = (hotB - coldB)*paper.heat + coldB

        //heat := float32(math.Exp(-math.Pow(float64(idToDaysAgo(paper.id))/730.,2)/2.))
        // voigt distro
        gamma := float64(0.0001)
        sigma := float64(365*4)
        t := float64(idToDaysAgo(paper.id))
        heat := float32(math.Exp(-math.Pow(t/sigma,2)/2. - gamma*t))

        col.r = (hotR - coldR)*heat + coldR
        col.g = (hotG - coldG)*heat + coldG
        col.b = (hotB - coldB)*heat + coldB
    
    } else {

        if paper.maincat == "hep-th" {
            col.r, col.g, col.b = 0, 0, 1 // blue
        } else if paper.maincat == "hep-ph" {
            col.r, col.g, col.b = 0, 1, 0 // green
        } else if paper.maincat == "hep-ex" {
            col.r, col.g, col.b = 1, 1, 0 // yellow
        } else if paper.maincat == "gr-qc" {
            col.r, col.g, col.b = 0, 1, 1 // cyan
        } else if paper.maincat == "hep-lat" {
            col.r, col.g, col.b = 0.7, 0.36, 0.2 // tan brown
        } else if paper.maincat == "astro-ph" {
            col.r, col.g, col.b = 0.89, 0.53, 0.6 // skin pink
        } else if paper.maincat == "cond-mat" {
            col.r, col.g, col.b = 0.7, 0.5, 0.4
        } else if paper.maincat == "quant-ph" {
            col.r, col.g, col.b = 0.4, 0.7, 0.7
        } else if paper.maincat == "physics" {
            col.r, col.g, col.b = 1, 0, 0 // red
        } else if paper.maincat == "math" {
            col.r, col.g, col.b = 0.62, 0.86, 0.24 // lime green
        } else if paper.maincat == "cs" {
            col.r, col.g, col.b = 0.7, 0.3, 0.6 // purple
        } else {
            col.r, col.g, col.b = 0.7, 1, 0.3
        }

        //if paper.maincat == "astro-ph.CO" {
        //    col.r, col.g, col.b = 0.3, 0.3, 1 // blue
        //} else if paper.maincat == "astro-ph.EP" {
        //    col.r, col.g, col.b = 0.3, 1, 0.3 // green
        //} else if paper.maincat == "astro-ph.GA" {
        //    col.r, col.g, col.b = 1, 1, 0.3 // yellow
        //} else if paper.maincat == "astro-ph.HE" {
        //    col.r, col.g, col.b = 0.3, 1, 1 // cyan
        //} else if paper.maincat == "astro-ph.IM" {
        //    col.r, col.g, col.b = 0.7, 0.36, 0.2 // tan brown
        //} else if paper.maincat == "astro-ph.SR" {
        //    col.r, col.g, col.b = 1, 0.3, 0.3 // red
        //} else {
        //    col.r, col.g, col.b = 1, 1, 1 // white
        //}

        // brighten papers in categories that are mostly tiny dots
        brighten := paper.maincat == "math" || paper.maincat == "cs"


        // foreground colour; select one by making its if condition true
        if (false) {
            // older papers are saturated, newer papers are coloured
            var saturation float32 = 0.3 + 0.4 * (1 - paper.age)
            col.r = saturation + (col.r) * (1 - saturation)
            col.g = saturation + (col.g) * (1 - saturation)
            col.b = saturation + (col.b) * (1 - saturation)
        } else if (false) {
            // older papers are saturated, newer papers are coloured and tend towards a full red component
            var saturation float32 = 0.4 * (1 - paper.age)
            age2 := paper.age * paper.age
            col.r = saturation + (col.r * (1 - age2) + age2) * (1 - saturation)
            col.g = saturation + (col.g * (1 - age2)      ) * (1 - saturation)
            col.b = saturation + (col.b * (1 - age2)      ) * (1 - saturation)
        } else if (true) {
            // older papers are saturated and dark, newer papers are coloured and bright
            var saturation float32 = 0.1 + 0.3 * (1 - paper.age)
            //var saturation float32 = 0.0
            //var dim_factor float32 = 0.4 + 0.6 * float32(math.Exp(float64(-10*age*age)))
            var dim_factor float32
            if brighten {
                dim_factor = 0.8 + 0.20 * float32(math.Exp(float64(-4*(1-paper.age)*(1-paper.age))))
            } else {
                dim_factor = 0.55 + 0.48 * float32(math.Exp(float64(-4*(1-paper.age)*(1-paper.age))))
            }
            if dim_factor > 1 {
                dim_factor = 1
            }
            col.r = dim_factor * (saturation + col.r * (1 - saturation))
            col.g = dim_factor * (saturation + col.g * (1 - saturation))
            col.b = dim_factor * (saturation + col.b * (1 - saturation))
        }

        if colourScheme == COLOUR_GRAYSCALE {
            //lum := 0.21 * r + 0.72 * g + 0.07 * b 
            lum := 0.289 * col.r + 0.587 * col.g + 0.114 * col.b
            col.r = lum
            col.g = lum
            col.b = lum
        }
    }

    //paper.col = CairoColor{r, g, b}
    return col
}

func ReadGraph(db *mysql.Client, jsonLocationFile string) *Graph {
    graph := new(Graph)

    if len(jsonLocationFile) == 0 {
        // load positions from the data base
        graph.papers = QueryPapers(db)
        if graph.papers == nil {
            log.Fatal("could not read papers from db")
        }
        fmt.Printf("read %v papers from db\n", len(graph.papers))
    } else {
        // load positions from JSON file
        graph.papers = ReadPaperLocationFromJSON(jsonLocationFile)
        if graph.papers == nil {
            log.Fatal("could not read papers from JSON file")
        }
    }

    graph.QueryCategories(db)

    if !*flagSkipLabels {
        graph.QueryLabels(db)
        graph.CalculateCategoryLabels()
        if *flagDBLocSuffix == "" {
            // Region labels are specific to default map only
            graph.ReadRegionLabels(*flagRegionFile)
        }
    }

    // determine labels to use for each paper
    //for _, paper := range graph.papers {
    //    paper.DetermineLabel()
    //}

    // Only if using heat calc
    //if *flagHeatMap {
    //    graph.QueryHeat(db)
    //}

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

    //for _, paper := range graph.papers {
    //    paper.SetColour()
    //}

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

func DrawTile(graph *Graph, worldWidth, worldHeight, xi, yi, surfWidth, surfHeight int, filename string, colourScheme int) {

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
        col := paper.GetColour(colourScheme)
        //surf.SetSourceRGB(float64(paper.col.r), float64(paper.col.g), float64(paper.col.b))
        surf.SetSourceRGB(float64(col.r), float64(col.g), float64(col.b))
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

func GenerateLabelZone(graph *Graph, scale, width, height, depth, xi, yi int, showCategories, showRegions bool, filename string) {

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
            if catLabel.X > x-rx && catLabel.X < x+rx && catLabel.Y > y-ry && catLabel.Y < y+ry {
                if first {
                    first = false
                } else {
                    fmt.Fprintf(w,",")
                }
                fmt.Fprintf(w,"{\"x\":%d,\"y\":%d,\"r\":%d,\"lbl\":\"%s\"}",catLabel.X,catLabel.Y,catLabel.Radius,catLabel.label)
            }
        }
    }

    if showRegions {
        for _, regLabel := range(graph.regLabels) {
            if regLabel.X > x-rx && regLabel.X < x+rx && regLabel.Y > y-ry && regLabel.Y < y+ry {
                if first {
                    first = false
                } else {
                    fmt.Fprintf(w,",")
                }
                fmt.Fprintf(w,"{\"x\":%d,\"y\":%d,\"r\":%d,\"lbl\":\"%s\"}",regLabel.X,regLabel.Y,regLabel.Radius,regLabel.Label)
            }
        }
    }

    fmt.Fprintf(w,"]})")
    w.Flush()
}

func ParallelDrawTile(graph *Graph, outPrefix string, depth, worldDim, xiFirst, xiLast, yiFirst, yiLast int, channel chan int) {
    var filename string
    for xi := xiFirst; xi <= xiLast; xi++ {
        for yi := yiFirst; yi <= yiLast; yi++ {
            // Draw normal tile
            if !*flagSkipNormalTiles {
                filename = fmt.Sprintf("%s/tiles/%d/%d/%d", outPrefix, depth, xi, yi)
                DrawTile(graph, worldDim, worldDim, xi, yi, TILE_PIXEL_LEN, TILE_PIXEL_LEN, filename,COLOUR_NORMAL)
            }
            // Draw heatmap tile
            if *flagHeatMap {
                filename = fmt.Sprintf("%s/tiles-hm/%d/%d/%d", outPrefix, depth, xi, yi)
                DrawTile(graph, worldDim, worldDim, xi, yi, TILE_PIXEL_LEN, TILE_PIXEL_LEN, filename,COLOUR_HEATMAP)
            }
            // Draw grayscale tile
            if *flagGrayScale {
                filename = fmt.Sprintf("%s/tiles-bw/%d/%d/%d", outPrefix, depth, xi, yi)
                DrawTile(graph, worldDim, worldDim, xi, yi, TILE_PIXEL_LEN, TILE_PIXEL_LEN, filename,COLOUR_GRAYSCALE)
            }
        }
    }
    channel <- 1 // signal that this set of tiles is done
}

func ParallelGenerateLabelZone(graph *Graph, outPrefix string, depth, scale, width, height, xiFirst, xiLast, yiFirst, yiLast int, showCategories, showRegions bool, channel chan int) {
    for xi := xiFirst; xi <= xiLast; xi++ {
        for yi := yiFirst; yi <= yiLast; yi++ {
            filename := fmt.Sprintf("%s/zones/%d/%d/%d", outPrefix, depth, xi, yi)
            GenerateLabelZone(graph, scale, width, height, depth, xi, yi, showCategories, showRegions, filename)
        }
    }
    channel <- 1 // signal that this set of tiles is done
}

func GenerateAllTiles(graph *Graph, w *bufio.Writer, outPrefix string) {

    fmt.Fprintf(w,",\"tilings\":[")

    //divisionSet := [...]int{4,8,24}
    //divisionSet := [...]int{4,8,24,72}
    //divisionSet := [...]int{4,8,24,72,216}
    //divisionSet := [...]int{4,8,16,32}
    divisionSet := [...]int{4,8,16,32,64}
    //divisionSet := [...]int{4,8,16,32,64,128}
    //divisionSet := [...]int{4,8,16,32,64,128,256}

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
    fmt.Fprintf(w,"]")
}

func GenerateAllLabelZones(graph *Graph, w *bufio.Writer, outPrefix string) {

    fmt.Fprintf(w,",\"zones\":[")

    // tile divisions, scale divisions
    depthSet := []struct {
        tdivs, sdivs uint
        showCats, showRegs bool
    }{
        {1,1,true,false},
        {1,2,true,false},
        {1,4,false,true},
        {1,8,false,true},
        {2,16,false,false},
        {4,32,false,false},
        {8,64,false,false},
        {16,128,false,false},
        {32,256,false,false},
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
                go ParallelGenerateLabelZone(
                    graph, outPrefix,
                    depth, scale, tile_width, tile_height, xi, xiLast, 1, int(labelDepth.tdivs),
                    labelDepth.showCats, labelDepth.showRegs,
                    channel)
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

func DrawSingleImage(graph *Graph, surfWidthInt, surfHeightInt int, zoomFactor float64, filename string, colourScheme int) {

    // convert width & height to floats for convenience
    surfWidth := float64(surfWidthInt)
    surfHeight := float64(surfHeightInt)

    // create surface to draw on
    surf := cairo.NewSurface(cairo.FORMAT_ARGB32, surfWidthInt, surfHeightInt)

    // a black background
    surf.SetSourceRGBA(0, 0, 0, 1)
    surf.Paint()

    // work out scaling so that the entire graph fits on the surface
    // multiple by zoom factor if the caller wants to zoom in (larger number is more zoomed in)
    scale := zoomFactor * math.Min(surfWidth / float64(graph.BoundsX), surfHeight / float64(graph.BoundsY));

    // make the transformation matrix for the papers
    matrix := new(cairo.Matrix)
    matrix.Xx = scale
    matrix.Yy = scale
    matrix.X0 = 0.5 * float64(surf.GetWidth())
    matrix.Y0 = 0.4 * float64(surf.GetHeight())
    matrixInv := *matrix
    matrixInv.Invert()
    surf.SetMatrix(*matrix)

    // the papers
    for _, paper := range graph.papers {
        pixelRadius, _ := matrix.TransformDistance(float64(paper.radius), float64(paper.radius))
        if pixelRadius < 0.09 {
            newRadius, _ := matrixInv.TransformDistance(0.09, 0.09)
            surf.Arc(float64(paper.x), float64(paper.y), newRadius, 0, 2 * math.Pi)
        } else {
            surf.Arc(float64(paper.x), float64(paper.y), float64(paper.radius), 0, 2 * math.Pi)
        }
        col := paper.GetColour(colourScheme)
        surf.SetSourceRGB(float64(col.r), float64(col.g), float64(col.b))
        surf.Fill()
    }

    if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
        fmt.Println(err)
        return
    }

    // save with full colours
    surf.WriteToPNG(filename + ".png")

    surf.Finish()
}

func DrawPoster(graph *Graph, surfWidthInt, surfHeightInt int, filename string, colourScheme int) {

    // convert width & height to floats for convenience
    surfWidth := float64(surfWidthInt)
    surfHeight := float64(surfHeightInt)

    // create surface to draw on
    surf := cairo.NewSurface(cairo.FORMAT_ARGB32, surfWidthInt, surfHeightInt)

    // a black background
    surf.SetSourceRGBA(0, 0, 0, 1)
    surf.Paint()

    // bands at top and bottom
    //surf.IdentityMatrix()
    //surf.Scale(float64(surf.GetWidth()), float64(surf.GetHeight()))
    //surf.SetSourceRGBA(0.26667, 0.33333, 0.4, 1) // #445566
    //surf.Rectangle(0, 0, 1, 0.1)
    //surf.Fill()
    //surf.Rectangle(0, 0.9, 1, 0.1)
    //surf.Fill()

    // load and paint our logo
    surfLogo := cairo.NewSurfaceFromPNG("../../boa/img/app/paperscapeTransparent.png")
    surf.IdentityMatrix()
    scale := 0.2 * surfWidth / float64(surfLogo.GetWidth())
    surf.Scale(scale, scale)
    surf.SetSourceSurface(surfLogo, 0.01 * surfWidth / scale, 0.01 * surfHeight / scale)
    surf.Paint()

    // load and draw the text
    surfText := cairo.NewSurfaceFromPNG("postertext.png")
    surfText.SetOperator(cairo.OPERATOR_IN)
    //surfText.SetSourceRGBA(1, 1, 1, 1) // text colour
    //surfText.Paint() // colour the text
    surf.IdentityMatrix()
    scale = 0.2 * surfWidth / float64(surfText.GetWidth())
    surf.Scale(scale, scale)
    surf.SetSourceSurface(surfText, 0.79 * surfWidth / scale, 0.67 * surfHeight / scale)
    surf.Paint() // draw the text to the main surface

    // work out scaling so that the entire graph fits on the surface
    scale = 1.6 * math.Min(surfWidth / float64(graph.BoundsX), surfHeight / float64(graph.BoundsY));

    // make the transformation matrix for the papers
    matrix := new(cairo.Matrix)
    matrix.Xx = scale
    matrix.Yy = scale
    matrix.X0 = 0.5 * float64(surf.GetWidth())
    matrix.Y0 = 0.4 * float64(surf.GetHeight())
    matrixInv := *matrix
    matrixInv.Invert()
    surf.SetMatrix(*matrix)

    // the papers
    for _, paper := range graph.papers {
        pixelRadius, _ := matrix.TransformDistance(float64(paper.radius), float64(paper.radius))
        if pixelRadius < 0.09 {
            newRadius, _ := matrixInv.TransformDistance(0.09, 0.09)
            surf.Arc(float64(paper.x), float64(paper.y), newRadius, 0, 2 * math.Pi)
        } else {
            surf.Arc(float64(paper.x), float64(paper.y), float64(paper.radius), 0, 2 * math.Pi)
        }
        col := paper.GetColour(colourScheme)
        surf.SetSourceRGB(float64(col.r), float64(col.g), float64(col.b))
        surf.Fill()
    }

    // category labels
    surf.SelectFontFace("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_NORMAL)
    surf.SetFontSize(650)
    surf.SetSourceRGBA(1, 1, 1, 1)
    for _, catLabel := range graph.catLabels {
        pieces := strings.Split(catLabel.label, ",")
        for i, piece := range pieces {
            extent := surf.TextExtents(piece)
            surf.MoveTo(float64(catLabel.X)-extent.Width/2, float64(catLabel.Y + i * 800)-extent.Height/2)
            surf.ShowText(piece)
        }
    }

    // region labels
    surf.SelectFontFace("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_NORMAL)
    surf.SetFontSize(500)
    surf.SetSourceRGBA(1, 1, 1, 1)
    for _, regLabel := range graph.regLabels {
        text := strings.TrimRight(regLabel.Label, ",")
        extent := surf.TextExtents(text)
        surf.MoveTo(float64(regLabel.X)-extent.Width/2, float64(regLabel.Y)-extent.Height/2)
        surf.ShowText(text)
    }

    if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
        fmt.Println(err)
        return
    }

    // save with full colours
    surf.WriteToPNG(filename + ".png")

    surf.Finish()
}
