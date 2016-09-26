package main

import (
    "flag"
    "fmt"
    "os"
    "strings"
    "encoding/json"
    "log"
    "github.com/yanatan16/GoMySQL"
)

var flagMapTable = flag.String("map-table","map_data", "Name of map table in db")
var flagMapTableSuffix = flag.String("suffix","", "Suffix to append to name of map table in db")

func main() {
    // parse command line options
    flag.Parse()

    if flag.NArg() != 1 {
        log.Fatal("need to specify a maps.json file")
    }

    // connect to db
    db := ConnectToDB()
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

    loc_table := *flagMapTable
    if *flagMapTableSuffix != "" {
        loc_table += "_" + *flagMapTableSuffix
    }

    // Create map table if it doesn't exist
    sql := "CREATE TABLE " + loc_table + " (id INT UNSIGNED PRIMARY KEY, x INT, y INT, r INT) ENGINE = MyISAM;"
    err = db.Query(sql)
    if err != nil {
        fmt.Println("MySQL statement error;", err)
    }

    // Insert new values into table
    sql = "REPLACE INTO " + loc_table + " (id,x,y,r) VALUES (?,?,?,?)"
    stmt, err := db.Prepare(sql)
    if err != nil {
        fmt.Println("MySQL statement error;", err)
    }
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

func ConnectToDB() *mysql.Client {
    // connect to MySQL database; using a socket is preferred since it's faster
    var db *mysql.Client
    var err error

    mysql_host := os.Getenv("PSCP_MYSQL_HOST")
    mysql_user := os.Getenv("PSCP_MYSQL_USER")
    mysql_pwd  := os.Getenv("PSCP_MYSQL_PWD")
    mysql_db   := os.Getenv("PSCP_MYSQL_DB")
    mysql_sock := os.Getenv("PSCP_MYSQL_SOCKET")

    // if nothing requested, default to something sensible
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

