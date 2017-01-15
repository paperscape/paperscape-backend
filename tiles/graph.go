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
    //"github.com/yanatan16/GoMySQL"
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
    maincat     *Category
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

func (paper *Paper) DetermineLabel(authors, keywords, title string) {
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
   
    var kwStr string
    if keywords != "" {
        // work out keyword string; maximum 2 keywords
        kws := strings.SplitN(keywords, ",", 3)
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
    } else if title != "" {
        // truncate title string to some reasonable length
        // hack: client uses commas to split string, so replace
        // all comas with semicolons
        title = strings.Replace(title,",",";",-1)
        if len(title) > 32 {
            title = title[:32]
            i := strings.LastIndex(title," ")
            if i != -1 {
                title = title[:i]
            }
        }
        kwStr = title + "...,"
    }

    paper.label = cleanJsonString(kwStr + "," + auStr)
}

func idToDaysAgo(id uint) uint {
    tId := time.Date((int(id / 10000000) + 1800),time.Month(((int(id % 10000000) / 625000) + 1)),((int(id % 625000) / 15625) + 1),0,0,0,0,time.UTC)
    days := uint(time.Now().Sub(tId).Hours()/24)
    return days
}

func (paper *Paper) GetColour(cfg *Config, colourScheme int, saturateByAge bool) *CairoColor {
    // basic colour of paper
    col := new(CairoColor)

    if colourScheme == COLOUR_HEATMAP {
        
        // Try pure heatmap instead
        //var coldR, coldG, coldB, hotR, hotG, hotB, dim float32
        //dim = 0.10
        //coldR, coldG, coldB = dim, dim, dim 
        //hotR, hotG, hotB = 1, dim, dim
        coldR := cfg.Tiles.Heatmap.ColdCol[0]
        coldG := cfg.Tiles.Heatmap.ColdCol[1]
        coldB := cfg.Tiles.Heatmap.ColdCol[2]
        warmR := cfg.Tiles.Heatmap.WarmCol[0]
        warmG := cfg.Tiles.Heatmap.WarmCol[1]
        warmB := cfg.Tiles.Heatmap.WarmCol[2]
        col.r = (warmR - coldR)*paper.heat + coldR
        col.g = (warmG - coldG)*paper.heat + coldG
        col.b = (warmB - coldB)*paper.heat + coldB

        //heat := float32(math.Exp(-math.Pow(float64(idToDaysAgo(paper.id))/730.,2)/2.))
        // voigt distro
        // Moved to QueryHeat
        //gamma := float64(0.0001)
        //sigma := float64(365*4)
        //t := float64(idToDaysAgo(paper.id))
        //heat := float32(math.Exp(-math.Pow(t/sigma,2)/2. - gamma*t))
        //col.r = (hotR - coldR)*heat + coldR
        //col.g = (hotG - coldG)*heat + coldG
        //col.b = (hotB - coldB)*heat + coldB
    
    } else {

        col.r = paper.maincat.Col[0]
        col.g = paper.maincat.Col[1]
        col.b = paper.maincat.Col[2]

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
        } else if (saturateByAge) {
            // older papers are saturated and dark, newer papers are coloured and bright
            var saturation float32 = 0.1 + 0.3 * (1 - paper.age)
            //var saturation float32 = 0.0
            //var dim_factor float32 = 0.4 + 0.6 * float32(math.Exp(float64(-10*age*age)))
            dim_factor := paper.maincat.DimFacs[0] + paper.maincat.DimFacs[1] * float32(math.Exp(float64(-4*(1-paper.age)*(1-paper.age))))
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

func (graph *Graph) CalculateCategoryLabels(catSet *CategorySet) {

    for _, category := range(catSet.Cats) {
        if category.Display == "" { continue } 

        var sumMass, sumMassX, sumMassY int64
        var minX,minY,maxX,maxY int
        for _, paper := range(graph.papers) {
            if paper.maincat.Name == category.Name {
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
            label.label = category.Display

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

func (graph *Graph) QueryLastMetaDownload(config *Config) {

    // construct query
    if config.Sql.Misc.Name == "" {
        fmt.Println("MySQL no misc table specified so can't query last meta download")
        return
    }
    query := fmt.Sprintf("SELECT %s FROM %s WHERE %s = \"lastmetadownload\"", config.Sql.Misc.FieldValue, config.Sql.Misc.Name, config.Sql.Misc.FieldField)
    // execute the query
    err := config.db.Query(query)
    if err != nil {
        fmt.Println("MySQL query error;", err)
        return
    }

    // get result set
    result, err := config.db.UseResult()
    if err != nil {
        fmt.Println("MySQL use result error;", err)
        return
    }

    defer config.db.FreeResult()

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

func (graph *Graph) QueryNewPapersId(config *Config) {

    // construct query
    if config.Sql.Date.Name == "" {
        fmt.Println("MySQL no date table specified so can't query new papers")
        return
    }
    //query := fmt.Sprintf("SELECT max(datebdry.id) FROM datebdry WHERE datebdry.id < %d",graph.LatestId)
    query := fmt.Sprintf("SELECT max(%s) FROM %s WHERE %s < %d", config.Sql.Date.FieldId, config.Sql.Date.Name, config.Sql.Date.FieldId, graph.LatestId)
    // execute the query
    err := config.db.Query(query)
    if err != nil {
        fmt.Println("MySQL query error;", err)
        return
    }

    // get result set
    result, err := config.db.UseResult()
    if err != nil {
        fmt.Println("MySQL use result error;", err)
        return
    }

    defer config.db.FreeResult()

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


func (graph *Graph) QueryHeat(cfg *Config) {

    if cfg.IdsTimeOrdered && cfg.Tiles.Heatmap.SqlMetaField == "" {
        // Calculate heat based on time ordered IDs
        for _, paper := range (graph.papers) {
            gamma := float64(0.0001)
            sigma := float64(365*4)
            t := float64(idToDaysAgo(paper.id))
            paper.heat = float32(math.Exp(-math.Pow(t/sigma,2)/2. - gamma*t))
        }
    } else if cfg.Tiles.Heatmap.SqlMetaField != "" && cfg.Tiles.Heatmap.SqlMetaType != "" {
        
        fmt.Printf("Querying heat using %s field from %s\n",cfg.Tiles.Heatmap.SqlMetaField,cfg.Sql.Meta.Name)
        // construct query
        query := "SELECT " + cfg.Sql.Meta.FieldId
        query += "," + cfg.Tiles.Heatmap.SqlMetaField
        query += " FROM " + cfg.Sql.Meta.Name

        // execute the query
        err := cfg.db.Query(query)
        if err != nil {
            fmt.Println("MySQL query error;", err)
            return
        }

        // get result set
        result, err := cfg.db.UseResult()
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

            var id uint64
            var ok bool
            var heat float32
            if id, ok = row[0].(uint64); !ok { continue }
            if cfg.Tiles.Heatmap.SqlMetaType == "uint" {
                var res uint64
                if res, ok = row[1].(uint64); !ok { continue }
                heat = float32(res)
            } else if cfg.Tiles.Heatmap.SqlMetaType == "int" {
                var res int64
                if res, ok = row[1].(int64); !ok { continue }
                heat = float32(res)
            } else if cfg.Tiles.Heatmap.SqlMetaType == "real" {
                var res float64
                if res, ok = row[1].(float64); !ok { continue }
                heat = float32(res)
            } else {
                fmt.Println("ERROR: invalid heatmap SqlMetaType")
                break
            }

            
            if paper := graph.GetPaperById(uint(id)); paper != nil {
                paper.heat = heat
                if heat > maxHeat { maxHeat = heat }
            }
        }

        // normalise
        if maxHeat > 0 {
            for _, paper := range graph.papers {
                paper.heat /= maxHeat
            }
        }

        // hack
        for _, paper := range graph.papers {
            if paper.heat > 0 {
                paper.heat = 1
            } else {
                paper.heat = 0
            }
        }


    } else {
        fmt.Println("ERROR: could not query paper heat")
    }


}

/* **OBSOLETE** old version
func (graph *Graph) QueryHeat(config *Config) {

    // execute the query
    err := config.db.Query("SELECT id,numCites,cites FROM pcite")
    if err != nil {
        fmt.Println("MySQL query error;", err)
        return
    }

    // get result set
    result, err := config.db.UseResult()
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

    config.db.FreeResult()

    fmt.Println("read heat from cites")
}
*/

func (graph *Graph) QueryCategories(config *Config, catSet *CategorySet) {

    // construct query
    //query := fmt.Sprintf("SELECT %s,%s,%s FROM %s",config.sqlMetaFieldId, config.sqlMetaFieldMaincat, config.sqlMetaFieldAllcats, config.sqlMetaTable)
    query := fmt.Sprintf("SELECT %s,%s FROM %s",config.Sql.Meta.FieldId, config.Sql.Meta.FieldMaincat, config.Sql.Meta.Name)
    // execute the query
    err := config.db.Query(query)
    if err != nil {
        fmt.Println("MySQL query error;", err)
        return
    }

    // get result set
    result, err := config.db.UseResult()
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
            paper.maincat = catSet.Lookup(maincat)
        }
    }

    config.db.FreeResult()

}

func (graph *Graph) QueryLabels(config *Config) {
    // construct query
    query := "SELECT " + config.Sql.Meta.FieldId
    keywordsLoaded := false
    authorsLoaded  := false
    titleLoaded   := false
    // Load only keywords or title, not both
    if config.Sql.Meta.FieldKeywords != "" {
        query += "," + config.Sql.Meta.FieldKeywords
        keywordsLoaded = true
    } else if config.Sql.Meta.FieldTitle != "" {
        query += "," + config.Sql.Meta.FieldTitle
        titleLoaded = true
    }
    if config.Sql.Meta.FieldAuthors != "" {
        query += "," + config.Sql.Meta.FieldAuthors
        authorsLoaded = true
    }
    query += " FROM " + config.Sql.Meta.Name
    if config.Sql.Meta.WhereClause != "" {
        query += fmt.Sprintf(" WHERE (%s)",config.Sql.Meta.WhereClause)
    }
    if config.Sql.Meta.ExtraClause != "" {
        query += " " + config.Sql.Meta.ExtraClause
    }
    if !((keywordsLoaded || titleLoaded) && authorsLoaded) {
        fmt.Println("Insufficient keywords or title and authors data available to make labels")
        return
    }

    // execute the query
    err := config.db.Query(query)
    if err != nil {
        fmt.Println("MySQL query error;", err)
        return
    }

    // get result set
    result, err := config.db.UseResult()
    if err != nil {
        fmt.Println("MySQL use result error;", err)
        return
    }
    defer config.db.FreeResult()

    // get each row from the result
    for {
        row := result.FetchRow()
        if row == nil {
            break
        }

        var ok bool
        var id uint64
        var keywords []byte
        var title string
        //var allcats string
        var authors []byte
        if id, ok = row[0].(uint64); !ok { continue }
        if row[1] != nil {
            if keywordsLoaded {
                if keywords, ok = row[1].([]byte); !ok { continue }
            } else if titleLoaded {
                if title, ok = row[1].(string); !ok { continue }
            } else { continue }
        } else { continue }
        if row[2] != nil {
            if authors, ok = row[2].([]byte); !ok { continue }
        } else { continue }
        paper := graph.GetPaperById(uint(id))
        if paper != nil {
            paper.DetermineLabel(string(authors),string(keywords),title)
        }
    }
}

func (graph *Graph) ComputeAges(config *Config) {
   
    fmt.Println("Computing paper ages")
    numPapers := len(graph.papers)
    if config.IdsTimeOrdered {
        // calculate age - papers should be sorted by id already
        for index, paper := range(graph.papers) {
            paper.age = float32(index) / float32(numPapers)
        }
    } else {
        // sort ids according to agesort field specified in settings
        // use this sorting to compute age
        var query string
        if *flagLayoutFile == "" {
            // loaded layout map from DB, so limit query to ids from map table
            query = fmt.Sprintf("SELECT %[1]s.%[2]s,%[3]s.%[5]s FROM %[1]s,%[3]s WHERE %[1]s.%[2]s = %[3]s.%[4]s ORDER BY %[3]s.%[5]s", config.Sql.Map.Name, config.Sql.Map.FieldId, config.Sql.Meta.Name, config.Sql.Meta.FieldId, config.Sql.Meta.FieldAgesort)
        } else {
            // loaded layout map from file, so get all ids from meta table to match against
            // (note: not using ExtraClause as it probably clashes with ORDER BY ...)
            query = fmt.Sprintf("SELECT %s FROM %s WHERE (%s) ORDER BY %s", config.Sql.Meta.FieldId, config.Sql.Meta.Name, config.Sql.Meta.WhereClause, config.Sql.Meta.FieldAgesort)
        }
        err := config.db.Query(query)
        if err != nil {
            fmt.Println("MySQL query error;", err)
            return
        }

        // get result set
        result, err := config.db.UseResult()
        if err != nil {
            fmt.Println("MySQL use result error;", err)
            return 
        }

        index := 0
        for {
            row := result.FetchRow()
            if row == nil {
                break
            }

            var ok bool
            var id uint64
            //var agesort uint64
            if id, ok = row[0].(uint64); !ok { continue }
            //if agesort, ok = row[1].(uint64); !ok { continue }
            paper := graph.GetPaperById(uint(id))
            if paper != nil {
                paper.age = float32(index) / float32(numPapers)
                index += 1
            }
        }

        config.db.FreeResult()

        if index != numPapers {
            log.Fatal("ERROR: Mismatch between number of papers and age index count:",index,numPapers)
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
    var insertErrors int
    for _, paper := range graph.papers {
        QuadTreeInsertPaper(nil, &qt.Root, paper, qt.MinX, qt.MinY, qt.MaxX, qt.MaxY, &insertErrors)
    }
    if (insertErrors > 0) {
        log.Printf("ERROR: QuadTreeInsertPaper hit minimum cell size %d time(s)\n",insertErrors)
    }

    fmt.Printf("quad tree bounding box: (%v,%v) -- (%v,%v)\n", qt.MinX, qt.MinY, qt.MaxX, qt.MaxY)

    // store the quad tree in the graph object
    graph.qt = qt
}
