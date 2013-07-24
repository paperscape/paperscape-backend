package main

import (
    "flag"
    "os"
    "encoding/json"
    "log"
    "xiwi"
)

var flagDB       = flag.String("db", "", "MySQL database to connect to")
var flagMapTable = flag.String("map-table","map_data", "Name of map table in db")

func main() {
    // parse command line options
    flag.Parse()

    if flag.NArg() != 1 {
        log.Fatal("need to specify a maps.json file")
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
    var papers [][]int
    if err := dec.Decode(&papers); err != nil {
        log.Fatal(err)
    }
    file.Close()

    db.Reconnect = true
    db.Lock()
    sql := "REPLACE INTO " + *flagMapTable + " (id,x,y,r) VALUES (?,?,?,?)"
    stmt, _ := db.Prepare(sql)
    db.Unlock()

    for _, paper := range papers {
        id := uint(paper[0])
        x  := paper[1]
        y  := paper[2]
        r  := paper[3]
        
        db.Lock()
        stmt.BindParams(id,x,y,r)
        stmt.Execute()
        db.Unlock()
    }
    stmt.Close()

}
