package main

import (
    "os"
    "fmt"
    "strings"
    "strconv"
    "math"
    "log"
    "time"
    "encoding/json"
    //"GoMySQL"
    "github.com/yanatan16/GoMySQL"
)

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
       
        // bit of hack at the moment
        if *flagSubCats {
            if paper.maincat == "astro-ph.CO" {
                col.r, col.g, col.b = 0.3, 0.3, 1 // blue
            } else if paper.maincat == "astro-ph.EP" {
                col.r, col.g, col.b = 0.3, 1, 0.3 // green
            } else if paper.maincat == "astro-ph.GA" {
                col.r, col.g, col.b = 1, 1, 0.3 // yellow
            } else if paper.maincat == "astro-ph.HE" {
                col.r, col.g, col.b = 0.3, 1, 1 // cyan
            } else if paper.maincat == "astro-ph.IM" {
                col.r, col.g, col.b = 0.7, 0.36, 0.2 // tan brown
            } else if paper.maincat == "astro-ph.SR" {
                col.r, col.g, col.b = 1, 0.3, 0.3 // red
            } else if paper.maincat == "astro-ph" {
                col.r, col.g, col.b = 0.89, 0.53, 0.6 // skin pink
                //col.r, col.g, col.b = 1, 1, 1 // white
            }
        }

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
        //} else if (!*flagSubCats) {
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
    //err := db.Query("SELECT id,maincat FROM meta_data")
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
        var allcats string
        if id, ok = row[0].(uint64); !ok { continue }
        if maincat, ok = row[1].(string); !ok { continue }
        allcats, ok = row[2].(string)

        paper := graph.GetPaperById(uint(id))
        if paper != nil {
            paper.maincat = maincat
            // code for if we want to distinguish sub-cats
            if *flagSubCats {
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
            }
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
