package main

import (
    "flag"
    "os"
    "fmt"
    "encoding/json"
    "GoMySQL"
    "log"
)

var flagDB        = flag.String("db", "localhost", "MySQL database to connect to")
var flagTileTable = flag.String("tile-table","tile_data", "Name of tile table in db")

type TilesJSON struct {
    Filename   string  `json:"map_filename"`
    Tilings    []TileDepths `json:"tilings"`
}

type TileDepths struct {
    Depth      uint    `json:"depth"`
    Worldw     uint    `json:"worldw"`
    Worldh     uint    `json:"worldh"`
    Pixelw     uint    `json:"pixelw"`
    Pixelh     uint    `json:"pixelh"`
    Numx       uint    `json:"numx"`
    Numy       uint    `json:"numy"`
    Padding    uint    `json:"padding"`
}

func main() {
    // parse command line options
    flag.Parse()
   
    if flag.NArg() != 1 {
        log.Fatal("need to specify a tile_index.json file")
    }

    // Connect to MySQL
    db, err := mysql.DialTCP(*flagDB, "hidden", "hidden", "xiwi")
    if err != nil {
        fmt.Println("cannot connect to database;", err)
        return
    }
    fmt.Println("connect to database", *flagDB)
    defer db.Close()

    // Open JSON map file
    file, err := os.Open(flag.Arg(0))
    if err != nil {
        log.Fatal(err)
    }

    // Decode JSON
    dec := json.NewDecoder(file)
    var jsonObj TilesJSON
    if err := dec.Decode(&jsonObj); err != nil {
        log.Fatal(err)
    }
    file.Close()

    db.Reconnect = true
    db.Lock()
    sql := "REPLACE INTO " + *flagTileTable + " (depth,worldwidth,worldheight,pixelwidth,pixelheight,numtotalx,numtotaly,worldpadding) VALUES (?,?,?,?,?,?,?,?)"
    stmt, _ := db.Prepare(sql)
    db.Unlock()

    for _, tileDepth := range jsonObj.Tilings {
        db.Lock()
        stmt.BindParams(tileDepth.Depth,tileDepth.Worldw,tileDepth.Worldh,tileDepth.Pixelw,tileDepth.Pixelh,tileDepth.Numx,tileDepth.Numy,tileDepth.Padding)
        stmt.Execute()
        db.Unlock()
    }
    stmt.Close()

}
