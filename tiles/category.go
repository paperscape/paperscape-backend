package main

import (
    "os"
    "fmt"
    "log"
    "encoding/json"
)

// this struct is used only for parsing the JSON
// the names must match the JSON format
type CategoryJSON struct {
    Cat     string
    Col     []float32
}

// this structure is exposed to the user
type Category struct {
    name    string
    r, g, b float32
}

type CategorySet struct {
    cats    []*Category
}

func MakeDefaultCategory(name string) *Category {
    c := new(Category)
    *c = Category{name, 0.7, 1, 0.3}
    return c
}

// read categories in form [{"cat":"name","col":[r,g,b]},...] from JSON file
func ReadCategoriesFromJSON(filename string) *CategorySet {
    fmt.Printf("reading categories from JSON file %v\n", filename)

    // open JSON file
    file, err := os.Open(filename)
    if err != nil {
        log.Println(err)
        return nil
    }

    // decode JSON
    dec := json.NewDecoder(file)
    var cats []CategoryJSON
    if err := dec.Decode(&cats); err != nil {
        log.Println(err)
        return nil
    }

    // close file
    file.Close()

    // create the category set object
    catSet := new(CategorySet)
    for _, c := range cats {
        c2 := new(Category)
        c2.name = c.Cat
        c2.r = c.Col[0]
        c2.g = c.Col[1]
        c2.b = c.Col[2]
        catSet.cats = append(catSet.cats, c2)

    }

    // print info
    fmt.Printf("read %v categories\n", len(catSet.cats))

    return catSet
}

func (catSet *CategorySet) Lookup(name string) *Category {
    for _, cat := range catSet.cats {
        if cat.name == name {
            return cat
        }
    }

    // not found! create a new category with default colour
    c := MakeDefaultCategory(name)
    catSet.cats = append(catSet.cats, c)
    return c
}
