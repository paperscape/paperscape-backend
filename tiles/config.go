package main

import (
    "os"
    "fmt"
    "log"
    "encoding/json"
    "github.com/yanatan16/GoMySQL"
)

type HeatmapSettings struct {
    SqlMetaField    string      `json:"sql_meta_field"`
    SqlMetaType     string      `json:"sql_meta_type"`
    ColdCol         []float32   `json:"cold_col"`
    WarmCol         []float32   `json:"warm_col"`
}

type TilesSettings struct {
    Heatmap             HeatmapSettings `json:"heatmap"`
    BackgroundCol       []float64       `json:"background_col"`
    DrawPaperOutline    bool            `json:"draw_paper_outline"`
    MaxTileDivision     uint            `json:"max_tile_division"`
    MaxLabelDivision    uint            `json:"max_label_division"`
}

type MetaTable struct {
    Name          string `json:"name"`
    WhereClause   string `json:"where_clause"`
    ExtraClause   string `json:"extra_clause"`
    FieldId       string `json:"field_id"`
    FieldMaincat  string `json:"field_maincat"`
    FieldAllcats  string `json:"field_allcats"`
    FieldTitle    string `json:"field_title"`
    FieldAuthors  string `json:"field_authors"`
    FieldKeywords string `json:"field_keywords"`
    FieldAgesort  string `json:"field_agesort"`
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

type SqlTables struct {
    Meta MetaTable `json:"meta_table"`
    Map  MapTable  `json:"map_table"`
    Date DateTable `json:"date_table"`
    Misc MiscTable `json:"misc_table"`
}

type Config struct {
    IdsTimeOrdered bool `json:"ids_time_ordered"`
    Tiles TilesSettings `json:"tiles"`
    Sql SqlTables `json:"sql"`
    db *mysql.Client `json:"-"` // ignored by JSON
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

