package main

import (
    "os"
    "fmt"
    "log"
    "encoding/json"
)

type Settings struct {
    ServeMyPscp   bool   `json:"serve_mypscp"`
}

type MetaTable struct {
    Name          string `json:"name"`
    FieldId       string `json:"field_id"`
    FieldMaincat  string `json:"field_maincat"`
    FieldAllcats  string `json:"field_allcats"`
    FieldTitle    string `json:"field_title"`
    FieldAuthors  string `json:"field_authors"`
    FieldKeywords string `json:"field_keywords"`
    FieldPubl     string `json:"field_publ"`
    FieldArxiv    string `json:"field_arxiv"`
    FieldInspire  string `json:"field_inspire"`
}

type RefsTable struct {
    Name            string `json:"name"`
    FieldId         string `json:"field_id"`
    FieldRefs       string `json:"field_refs"`
    FieldNumRefs    string `json:"field_numrefs"`
    FieldCites      string `json:"field_cites"`
    FieldNumCites   string `json:"field_numcites"`
    FieldDNumCites1 string `json:"field_dnc1"`
    FieldDNumCites5 string `json:"field_dnc5"`
    RblobOrder      bool   `json:"rblob_order"`
    RblobFreq       bool   `json:"rblob_freq"`
    RblobCites      bool   `json:"rblob_cites"`
}

type MapTable struct {
    Name    string `json:"name"`
    FieldId string `json:"field_id"`
    FieldX  string `json:"field_x"`
    FieldY  string `json:"field_y"`
    FieldR  string `json:"field_r"`
}

type DateTable struct {
    Name      string `json:"name"`
    FieldDays string `json:"field_days"`
    FieldId   string `json:"field_id"`
}

type MiscTable struct {
    Name       string `json:"name"`
    FieldField string `json:"field_field"`
    FieldValue string `json:"field_value"`
}

type AbstractTable struct {
    Name          string `json:"name"`
    FieldId       string `json:"field_id"`
    FieldAbstract string `json:"field_abstract"`
}

type SqlTables struct {
    Meta MetaTable     `json:"meta_table"`
    Refs RefsTable     `json:"refs_table"`
    Map  MapTable      `json:"map_table"`
    Date DateTable     `json:"date_table"`
    Misc MiscTable     `json:"misc_table"`
    Abst AbstractTable `json:"misc_table"`
}

type Config struct {
    Settings Settings `json:"webserver"`
    IdsTimeOrdered bool `json:"ids_time_ordered"`
    Sql SqlTables `json:"sql"`
}

func ReadConfigFromJSON(filename string) *Config {
    fmt.Printf("reading configuration settings from JSON file %v\n", filename)

    // open JSON file
    file, err := os.Open(filename)
    if err != nil {
        log.Println(err)
        return nil
    }
    defer file.Close()

    // decode JSON
    dec := json.NewDecoder(file)
    config := new(Config)
    if err := dec.Decode(&config); err != nil {
        log.Println(err)
        return nil
    }

    return config
}

