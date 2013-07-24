package main

import (
    "flag"
    "os"
    "encoding/json"
    "log"
    "xiwi"
)

var flagDB        = flag.String("db", "", "MySQL database to connect to")
var flagLabelTable = flag.String("label-table","label_data", "Name of tile table in db")

type LabelsJSON struct {
    LatestId   uint    `json:"latestid"`
    Xmin       int     `json:"xmin"`
    Ymin       int     `json:"ymin"`
    Xmax       int     `json:"xmax"`
    Ymax       int     `json:"ymax"`
    Zones    []LabelDepths `json:"zones"`
}

type LabelDepths struct {
    Depth      uint    `json:"z"`
    Worldw     uint    `json:"w"`
    Worldh     uint    `json:"h"`
    Numx       uint    `json:"nx"`
    Numy       uint    `json:"ny"`
}

func main() {
    // parse command line options
    flag.Parse()

    if flag.NArg() != 1 {
        log.Fatal("need to specify a tile_index.json file")
    }

    // connect to db
    db := xiwi.ConnectToDB(*flagDB)
    if db == nil {
        return
    }
    defer db.Close()

    // Open JSON map file
    file, err := os.Open(flag.Arg(0))
    if err != nil {
        log.Fatal(err)
    }

    // Decode JSON
    dec := json.NewDecoder(file)
    var jsonObj LabelsJSON
    if err := dec.Decode(&jsonObj); err != nil {
        log.Fatal(err)
    }
    file.Close()

    zones, _ := json.Marshal(jsonObj.Zones)

    db.Reconnect = true
    db.Lock()
    sql := "REPLACE INTO " + *flagLabelTable + " (latest_id,xmin,ymin,xmax,ymax,zones) VALUES (?,?,?,?,?,?)"
    stmt, _ := db.Prepare(sql)
    stmt.BindParams(jsonObj.LatestId,jsonObj.Xmin,jsonObj.Ymin,jsonObj.Xmax,jsonObj.Ymax,zones)
    stmt.Execute()
    db.Unlock()
    stmt.Close()

}
