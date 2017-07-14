package main

import (
	"net/http"

	"github.com/PuerkitoBio/gocrawl"
	"github.com/PuerkitoBio/goquery"
)

// State type
type State struct {
	area Area
	name string
}

// Area type
type Area struct {
	area  *Area
	route Route
}

// Route type
type Route struct {
	name, grade string
	rating      int
}

// MountainProjectExtender Default Extender
type MountainProjectExtender struct {
	gocrawl.DefaultExtender
}

// Visit page
func (x *MountainProjectExtender) Visit(ctx *gocrawl.URLContext, res *http.Response, doc *goquery.Document) (interface{}, bool) {

	return nil, true
}

// Filter pages to visit
func (x *MountainProjectExtender) Filter(ctx *gocrawl.URLContext, isVisted bool) bool {
	return !isVisted
}

func main() {
	opts := gocrawl.NewOptions(new(MountainProjectExtender))

	opts.LogFlags = gocrawl.LogAll

	c := gocrawl.NewCrawlerWithOptions(opts)
	c.Run("https://www.mountainproject.com/destinations")
}
