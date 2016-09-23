package main

import (
    "os"
    "fmt"
    "log"
    "encoding/json"
)

type Category struct {
    Name    string      `json:"cat"`
    Col     []float32   `json:"col"`
    DimFacs []float32   `json:"dim_facs"`
}

type CategorySet struct {
    DefaultCol      []float32   `json:"default_col"`
    DefaultDimFacs  []float32   `json:"default_dim_facs"`
    Cats            []Category  `json:"cats"`
}

func MakeDefaultCategory(name string) *Category {
    c := new(Category)
    *c = Category{name, []float32{0.7, 1, 0.3}, []float32{0.5, 0.5}}
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
    catSet := new(CategorySet)
    if err := dec.Decode(catSet); err != nil {
        log.Println(err)
        return nil
    }

    // close file
    file.Close()

    // set defaults for categories that don't provide the values
    for i := range catSet.Cats {
        if len(catSet.Cats[i].Col) != 3 {
            catSet.Cats[i].Col = catSet.DefaultCol
        }
        if len(catSet.Cats[i].DimFacs) != 2 {
            catSet.Cats[i].DimFacs = catSet.DefaultDimFacs
        }
    }

    // print info
    fmt.Printf("read %v categories\n", len(catSet.Cats))

    return catSet
}

func (catSet *CategorySet) Lookup(name string) *Category {
    for _, cat := range catSet.Cats {
        if cat.Name == name {
            return &cat
        }
    }

    // not found! create a new category with default colour
    c := Category{name, catSet.DefaultCol, catSet.DefaultDimFacs}
    catSet.Cats = append(catSet.Cats, c)
    return &catSet.Cats[len(catSet.Cats) - 1]
}
