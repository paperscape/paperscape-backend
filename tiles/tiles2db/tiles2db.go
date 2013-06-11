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
    Filename   string  `json:"map_file"`
    LatestId   uint    `json:"latestid"`
    Xmin       int     `json:"xmin"`
    Ymin       int     `json:"ymin"`
    Xmax       int     `json:"xmax"`
    Ymax       int     `json:"ymax"`
    Pixelw     uint    `json:"pixelw"`
    Pixelh     uint    `json:"pixelh"`
    Tilings    []TileDepths `json:"tilings"`
}

type TileDepths struct {
    Depth      uint    `json:"z"`
    Worldw     uint    `json:"tw"`
    Worldh     uint    `json:"th"`
    Numx       uint    `json:"nx"`
    Numy       uint    `json:"ny"`
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

    tilings, _ := json.Marshal(jsonObj.Tilings)

    db.Reconnect = true
    db.Lock()
    sql := "REPLACE INTO " + *flagTileTable + " (latest_id,xmin,ymin,xmax,ymax,tile_pixel_w,tile_pixel_h,tilings) VALUES (?,?,?,?,?,?,?,?)"
    stmt, _ := db.Prepare(sql)
    stmt.BindParams(jsonObj.LatestId,jsonObj.Xmin,jsonObj.Ymin,jsonObj.Xmax,jsonObj.Ymax,jsonObj.Pixelw,jsonObj.Pixelh,tilings)
    stmt.Execute()
    db.Unlock()
    stmt.Close()

}
