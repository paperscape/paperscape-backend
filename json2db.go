package main

import (
    "flag"
    "os"
    "fmt"
    "encoding/json"
    "GoMySQL"
    "log"
)

var flagDB       = flag.String("db", "localhost", "MySQL database to connect to")
var flagMapFile  = flag.String("map-file", "", "JSON file to read map data from")
var flagMapTable = flag.String("map-table","map_data", "Name of map table in db")

func main() {
    // parse command line options
    flag.Parse()
    
    // Connect to MySQL
    db, err := mysql.DialTCP(*flagDB, "hidden", "hidden", "xiwi")
    if err != nil {
        fmt.Println("cannot connect to database;", err)
        return
    }
    fmt.Println("connect to database", *flagDB)
    defer db.Close()

    // Open JSON map file
    file, err := os.Open(*flagMapFile)
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
