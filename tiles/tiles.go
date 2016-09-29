package main

import (
    "flag"
    "os"
    "bufio"
    "fmt"
    "path/filepath"
    "strings"
    "strconv"
    "runtime"
    "math"
    "sort"
    "log"
    //"time"
    "encoding/json"
    //"GoMySQL"
    "github.com/yanatan16/GoMySQL"
    "github.com/ungerik/go-cairo"
)

var GRAPH_PADDING      = 100 // what to pad graph by on each side
var TILE_PIXEL_LEN     = 512

var COLOUR_NORMAL      = 0
var COLOUR_HEATMAP     = 1
var COLOUR_GRAYSCALE   = 2

var flagSettingsFile   = flag.String("settings", "../config/arxiv-settings.json", "Read settings from JSON file")
var flagCatsFile       = flag.String("categories", "../config/arxiv-categories.json", "Read categories from JSON file")
var flagLayoutFile     = flag.String("layout", "", "Read paper locations from JSON file instead of DB")
var flagRegionsFile    = flag.String("regions", "", "Read region labels from JSON file")

var flagGrayScale      = flag.Bool("gs", false, "Generate grayscale tiles")
var flagHeatMap        = flag.Bool("hm", false, "Generate heatmap tiles")

var flagNoTiles        = flag.Bool("no-tiles", false, "Do not generate normal tiles (still generates index information)")
var flagNoLabels       = flag.Bool("no-labels", false, "Do not generate labels (still generates index information)")

var flagCentreGraph    = flag.Bool("centre", false, "Whether to centre the graph on the total centre of mass")

var flagDoSingle       = flag.String("single-image", "", "Generate a large single image with <WxHxZoom> parameters, eg 100x100x2.5")
var flagDoPoster       = flag.Bool("poster", false, "Generate an image suitable for printing as a poster")


var flagMaxCores       = flag.Int("cores", -1, "Max number of system cores to use, default is all of them")

func main() {
    // parse command line options
    flag.Parse()

    if flag.NArg() != 1 {
        log.Fatal("need to specify output prefix")
    }
    outPrefix := flag.Arg(0)

    // read in settings
    config := ReadConfigFromJSON(*flagSettingsFile)
    if config == nil {
        log.Fatal("Could not read in config settings")
        return
    }

    // connect to the db
    config.db = ConnectToDB()
    if config.db == nil {
        log.Fatal("Could not connect to DB")
        return
    }
    defer config.db.Close()

    // read in the categories
    catSet := ReadCategoriesFromJSON(*flagCatsFile)
    if catSet == nil {
        return
    }

    // read in the graph
    graph := ReadGraph(config, *flagLayoutFile, catSet)

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
        colourScheme := COLOUR_NORMAL
        if *flagHeatMap {
            colourScheme = COLOUR_HEATMAP
        } else if *flagGrayScale {
            colourScheme = COLOUR_GRAYSCALE
        }
        DrawSingleImage(config, graph, int(resx), int(resy), zoomFactor, outPrefix, colourScheme)
    } else if *flagDoPoster {
        // A0 at 300 dpi: 9933 x 14043
        // A0 at 72 dpi: 2348 x 3370 
        // A1 at 300 dpi: 7016 x 9933
        // A1 at 72 dpi: 1648 x 2384 
        // A3 at 300 dpi: 3508 x 4961
        // A3 at 72 dpi: 842 x 1191 
        // A4 at 300 dpi: 2480 x 3508
        // A4 at 72 dpi: 595 x 842 
        //resy := 9933; resx := 14043
        resy := 7016; resx := 9933
        DrawPoster(config, graph, resx, resy, outPrefix, COLOUR_NORMAL)
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
        
        // dbSuffix was used for timelapse maps 
        // assumed that map tables have prefix 'map_data_'
        dbSuffix := ""
        if len(config.Sql.Map.Name) > 9 && config.Sql.Map.Name[:9] == "map_data_" {
            dbSuffix = config.Sql.Map.Name[9:]
        }

        fmt.Fprintf(w,"world_index({\"dbsuffix\":\"%s\",\"latestid\":%d,\"numpapers\":%d,\"xmin\":%d,\"ymin\":%d,\"xmax\":%d,\"ymax\":%d,\"pixelw\":%d,\"pixelh\":%d",dbSuffix,graph.LatestId,num_papers,graph.MinX,graph.MinY,graph.MaxX,graph.MaxY,TILE_PIXEL_LEN,TILE_PIXEL_LEN)

        graph.QueryNewPapersId(config)
        if graph.NewPapersId != 0 {
            fmt.Fprintf(w,",\"newid\":%d",graph.NewPapersId)
        }

        graph.QueryLastMetaDownload(config)
        if graph.LastMetaDownload != "" {
            fmt.Fprintf(w,",\"lastdl\":\"%s\"",graph.LastMetaDownload)
        }

        GenerateAllTiles(config, graph, w, outPrefix)
        runtime.GC()

        GenerateAllLabelZones(graph, w, outPrefix)

        fmt.Fprintf(w,"})")
        w.Flush()
    }
}

type PaperSortId []*Paper
func (p PaperSortId) Len() int           { return len(p) }
func (p PaperSortId) Less(i, j int) bool { return p[i].id < p[j].id }
func (p PaperSortId) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

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

// read layout in form [[id,x,y,r],...] from JSON file
func ReadPaperLocationFromJSON(config *Config, filename string) []*Paper {
    fmt.Printf("reading paper layout from JSON file %v\n", filename)

    // open JSON file
    file, err := os.Open(filename)
    if err != nil {
        log.Println(err)
        return nil
    }

    // decode JSON
    dec := json.NewDecoder(file)
    var layout [][]int64
    if err := dec.Decode(&layout); err != nil {
        log.Println(err)
        return nil
    }

    // close file
    file.Close()

    // create default category for newly loaded papers
    defaultCat := MakeDefaultCategory("unknown")

    // build paper array
    papers := make([]*Paper, 0)
    for _, item := range layout {
        papers = append(papers, MakePaper(uint(item[0]), int(item[1]), int(item[2]), int(item[3]), defaultCat))
    }

    // make sure papers are sorted!
    sort.Sort(PaperSortId(papers))

    // print info
    fmt.Printf("read %v paper positions\n", len(layout))

    return papers
}

// Query database for papers to include
func QueryPapers(config *Config) []*Paper {
    
    // count number of papers
    query := fmt.Sprintf("SELECT count(%s) FROM %s", config.Sql.Map.FieldId, config.Sql.Map.Name)
    //err := config.db.Query("SELECT count(id) FROM " + loc_table)
    err := config.db.Query(query)
    if err != nil {
        fmt.Println("MySQL query error;", err)
        return nil
    }

    // get result set
    result, err := config.db.UseResult()
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
    config.db.FreeResult()

    // allocate paper array
    //papers := make([]*Paper, numPapers)
    papers := make([]*Paper, 0)

    // create default category for newly loaded papers
    defaultCat := MakeDefaultCategory("unknown")

    // execute the query
    //err = config.db.Query("SELECT map_data.id,map_data.x,map_data.y,map_data.r,keywords.keywords FROM map_data,keywords WHERE map_data.id = keywords.id")
    query = fmt.Sprintf("SELECT %s,%s,%s,%s FROM %s", config.Sql.Map.FieldId, config.Sql.Map.FieldX, config.Sql.Map.FieldY, config.Sql.Map.FieldR, config.Sql.Map.Name)
    err = config.db.Query(query)
    if err != nil {
        fmt.Println("MySQL query error;", err)
        return nil
    }

    // get result set
    result, err = config.db.UseResult()
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

        papers = append(papers,MakePaper(uint(id), int(x), int(y), int(r), defaultCat))
    }

    config.db.FreeResult()

    // make sure papers are sorted!
    sort.Sort(PaperSortId(papers))

    if len(papers) != int(numPapers) {
        fmt.Println("could not read all papers from",config.Sql.Map.Name,"; wanted", numPapers, "got", len(papers))
        return nil
    }

    return papers
}

func MakePaper(id uint, x int, y int, radius int, maincat *Category) *Paper {
    paper := new(Paper)
    paper.id = id
    paper.x = x
    paper.y = y
    paper.radius = radius
    paper.maincat = maincat
    return paper
}

func ReadGraph(config *Config, jsonLocationFile string, catSet *CategorySet) *Graph {
    graph := new(Graph)

    if jsonLocationFile != "" {
        // load positions from JSON layout file
        graph.papers = ReadPaperLocationFromJSON(config, jsonLocationFile)
        if graph.papers == nil {
            log.Fatal("could not read papers from JSON file")
        }
    } else {
        // load positions from the data base
        graph.papers = QueryPapers(config)
        if graph.papers == nil {
            log.Fatal("could not read papers from db")
        }
        fmt.Printf("read %v papers from db\n", len(graph.papers))
    }

    graph.QueryCategories(config, catSet)

    if !*flagNoLabels {
        graph.QueryLabels(config)
        graph.CalculateCategoryLabels(catSet)
        if *flagRegionsFile != "" {
            graph.ReadRegionLabels(*flagRegionsFile)
        }
    }

    // determine labels to use for each paper
    //for _, paper := range graph.papers {
    //    paper.DetermineLabel()
    //}

    if !*flagNoTiles {
        graph.ComputeAges(config)
    }

    // Only if using heat calc
    if *flagHeatMap {
        graph.QueryHeat(config)
    }
    
    var sumMass, sumMassX, sumMassY int64

    for _, paper := range graph.papers {
        if paper.x - paper.radius < graph.MinX { graph.MinX = paper.x - paper.radius }
        if paper.y - paper.radius < graph.MinY { graph.MinY = paper.y - paper.radius }
        if paper.x + paper.radius > graph.MaxX { graph.MaxX = paper.x + paper.radius }
        if paper.y + paper.radius > graph.MaxY { graph.MaxY = paper.y + paper.radius }
        mass := int64(paper.radius*paper.radius)
        sumMass += mass
        sumMassX += mass*int64(paper.x)
        sumMassY += mass*int64(paper.y)
    }

    graph.MinX -= GRAPH_PADDING
    graph.MaxX += GRAPH_PADDING
    graph.MinY -= GRAPH_PADDING
    graph.MaxY += GRAPH_PADDING

    // centre graph on total centre of mass
    if *flagCentreGraph && sumMass > 0 {
        centreX := int(sumMassX/sumMass)
        centreY := int(sumMassY/sumMass)
        fmt.Printf("Adjusting for centre of mass: (%d,%d)\n",centreX,centreY)
        if (centreX - graph.MinX) > (graph.MaxX - centreX) {
            graph.MaxX = 2.*centreX - graph.MinX
        } else {
            graph.MinX = 2.*centreX - graph.MaxX
        }
        if (centreY - graph.MinY) > (graph.MaxY - centreY) {
            graph.MaxY = 2.*centreY - graph.MinY
        } else {
            graph.MinY = 2.*centreY - graph.MaxY
        }
    }

    graph.BoundsX = graph.MaxX - graph.MinX
    graph.BoundsY = graph.MaxY - graph.MinY

    graph.LatestId = graph.papers[len(graph.papers)-1].id

    //for _, paper := range graph.papers {
    //    paper.SetColour()
    //}

    fmt.Printf("graph has %v papers; min=(%v,%v), max=(%v,%v)\n", len(graph.papers), graph.MinX, graph.MinY, graph.MaxX, graph.MaxY)

    return graph
}


func DrawTile(config *Config, graph *Graph, worldWidth, worldHeight, xi, yi, surfWidth, surfHeight int, filename string, colourScheme int) {

    surf := cairo.NewSurface(cairo.FORMAT_ARGB32, surfWidth, surfHeight)
    surf.SetSourceRGBA(0, 0, 0, 0) // transparent background
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
    surf.SetLineWidth(5)
    // Need to add largest radius to dimensions to ensure we don't miss any papers

    // foreground
    graph.qt.ApplyIfWithin(int(x), int(y), int(rx)+graph.qt.MaxR, int(ry)+graph.qt.MaxR, func(paper *Paper) {
        r := float64(paper.radius)
        pixelRadius, _ := matrix.TransformDistance(r, r)
        if pixelRadius < 0.09 {
            newRadius, _ := matrixInv.TransformDistance(0.09, 0.09)
            r = newRadius
        }
        col := paper.GetColour(config,colourScheme,true)
        //surf.SetSourceRGB(float64(paper.col.r), float64(paper.col.g), float64(paper.col.b))
        surf.SetSourceRGB(float64(col.r), float64(col.g), float64(col.b))
        surf.Arc(float64(paper.x), float64(paper.y), r, 0, 2 * math.Pi)
        surf.Fill()
        if config.Tiles.DrawPaperOutline {
            col := paper.GetColour(config,colourScheme,false)
            surf.SetSourceRGB(float64(col.r), float64(col.g), float64(col.b))
            surf.Arc(float64(paper.x), float64(paper.y), r, 0, 2 * math.Pi)
            surf.Stroke()
        }
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

func ParallelDrawTile(config *Config, graph *Graph, outPrefix string, depth, worldDim, xiFirst, xiLast, yiFirst, yiLast int, channel chan int) {
    var filename string
    for xi := xiFirst; xi <= xiLast; xi++ {
        for yi := yiFirst; yi <= yiLast; yi++ {
            // Draw normal tile
            if !*flagNoTiles {
                filename = fmt.Sprintf("%s/tiles/%d/%d/%d", outPrefix, depth, xi, yi)
                DrawTile(config, graph, worldDim, worldDim, xi, yi, TILE_PIXEL_LEN, TILE_PIXEL_LEN, filename,COLOUR_NORMAL)
            }
            // Draw heatmap tile
            if *flagHeatMap {
                filename = fmt.Sprintf("%s/tiles-hm/%d/%d/%d", outPrefix, depth, xi, yi)
                DrawTile(config, graph, worldDim, worldDim, xi, yi, TILE_PIXEL_LEN, TILE_PIXEL_LEN, filename,COLOUR_HEATMAP)
            }
            // Draw grayscale tile
            if *flagGrayScale {
                filename = fmt.Sprintf("%s/tiles-bw/%d/%d/%d", outPrefix, depth, xi, yi)
                DrawTile(config, graph, worldDim, worldDim, xi, yi, TILE_PIXEL_LEN, TILE_PIXEL_LEN, filename,COLOUR_GRAYSCALE)
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

func GenerateAllTiles(config *Config, graph *Graph, w *bufio.Writer, outPrefix string) {

    fmt.Fprintf(w,",\"tilings\":[")

    //divisionSet := [...]int{4,8,16,32,64}
    //divisionSet := [...]int{4,8,16,32,64,128}
    divisionSet := [...]int{4,8,16,32,64,128,256}

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
            go ParallelDrawTile(config, graph, outPrefix, depth, worldDim, xi, xiLast, 1, divs, channel)
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

        if !*flagNoLabels {
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

func DrawSingleImage(config *Config, graph *Graph, surfWidthInt, surfHeightInt int, zoomFactor float64, filename string, colourScheme int) {

    // convert width & height to floats for convenience
    surfWidth := float64(surfWidthInt)
    surfHeight := float64(surfHeightInt)

    // create surface to draw on
    surf := cairo.NewSurface(cairo.FORMAT_ARGB32, surfWidthInt, surfHeightInt)
    surf.SetLineWidth(5)

    // a black background
    surf.SetSourceRGBA(config.Tiles.BackgroundCol[0], config.Tiles.BackgroundCol[1], config.Tiles.BackgroundCol[2], 1)
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
        r := float64(paper.radius)
        pixelRadius, _ := matrix.TransformDistance(r, r)
        if pixelRadius < 0.09 {
            newRadius, _ := matrixInv.TransformDistance(0.09, 0.09)
            r = newRadius
        }
        col := paper.GetColour(config,colourScheme,true)
        surf.SetSourceRGB(float64(col.r), float64(col.g), float64(col.b))
        surf.Arc(float64(paper.x), float64(paper.y), r, 0, 2 * math.Pi)
        surf.Fill()
        if config.Tiles.DrawPaperOutline {
            col := paper.GetColour(config,colourScheme,false)
            surf.SetSourceRGB(float64(col.r), float64(col.g), float64(col.b))
            surf.Arc(float64(paper.x), float64(paper.y), r, 0, 2 * math.Pi)
            surf.Stroke()
        }
    }

    if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
        fmt.Println(err)
        return
    }

    // save with full colours
    surf.WriteToPNG(filename + ".png")

    surf.Finish()
}

func DrawPoster(config *Config, graph *Graph, surfWidthInt, surfHeightInt int, filename string, colourScheme int) {

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
    surfLogo, _ := cairo.NewSurfaceFromPNG("poster/paperscapeTransparent.png")
    surf.IdentityMatrix()
    scale := 0.2 * surfWidth / float64(surfLogo.GetWidth())
    surf.Scale(scale, scale)
    surf.SetSourceSurface(surfLogo, 0.01 * surfWidth / scale, 0.01 * surfHeight / scale)
    surf.Paint()

    // load and draw the text
    surfText, _ := cairo.NewSurfaceFromPNG("poster/postertext.png")
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
        col := paper.GetColour(config,colourScheme,true)
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

func ConnectToDB() *mysql.Client {
    // connect to MySQL database; using a socket is preferred since it's faster
    var db *mysql.Client
    var err error

    mysql_host := os.Getenv("PSCP_MYSQL_HOST")
    mysql_user := os.Getenv("PSCP_MYSQL_USER")
    mysql_pwd  := os.Getenv("PSCP_MYSQL_PWD")
    mysql_db   := os.Getenv("PSCP_MYSQL_DB")
    mysql_sock := os.Getenv("PSCP_MYSQL_SOCKET")

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
