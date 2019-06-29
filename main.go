package main

import (
	"context"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/gocolly/colly"
	"github.com/olivere/elastic"
	"github.com/orcaman/concurrent-map"
	"golang.org/x/net/html"
	"gopkg.in/cheggaaa/pb.v2"
)

type Grade struct {
	YDS     string `json:"YDS"`
	French  string `json:"French"`
	Ewbanks string `json:"Ewbanks"`
	UIAA    string `json:"UIAA"`
	ZA      string `json:"ZA"`
	British string `json:"British"`
}

type Location struct {
	Latitude  string `json:"Latitude"`
	Longitude string `json:"Longitude"`
}

type Route struct {
	Name     string   `json:"Name"`
	Grade    Grade    `json:"Grade"`
	Location Location `json:"Location"`
}

func parseLocation(loc string) Location {
	s := strings.Split(loc, ",")
	lat := s[0]
	lon := strings.Split(s[1], " ")[1]

	return Location{
		Latitude:  strings.Trim(lat, "\n"),
		Longitude: strings.Trim(lon, "\n"),
	}
}

func parseGrade(nodes []*html.Node) Grade {
	g := Grade{}
	for _, node := range nodes {
		for _, attr := range node.Attr {
			currGrade := strings.TrimSpace(node.FirstChild.Data)
			if attr.Key == "class" {
				switch attr.Val {
				case "rateYDS":
					g.YDS = currGrade
				case "rateEwbanks":
					g.Ewbanks = currGrade
				case "rateUIAA":
					g.UIAA = currGrade
				case "rateZA":
					g.ZA = currGrade
				case "rateBritish":
					g.British = currGrade
				}
			}
		}
	}
	return g
}

func main() {
	routeLocations := cmap.New()
	client, err := elastic.NewClient()
	if err != nil {
		panic(err)
	}

	exists, err := client.IndexExists("routes").Do(context.Background())
	if !exists {
		_, err = client.CreateIndex("routes").Do(context.Background())
		if err != nil {
			panic(err)
		}
	}
	bar := pb.StartNew(379)

	// TODO: command line args: --state --all-states --Limit
	// TODO: fetch # of routes from route page
	// TODO: persistent data store - redis/

	// TODO: make top level collector for each state
	c := colly.NewCollector(
		colly.AllowedDomains("www.mountainproject.com"),
	)

	c.Limit(&colly.LimitRule{
		RandomDelay: 2 * time.Second,
	})

	routeCollector := c.Clone()
	routeCollector.Async = true

	// Top-level areas page
	c.OnHTML("#route-guide strong a[href]", func(e *colly.HTMLElement) {
		l := e.Attr("href")
		c.Visit(l)
	})

	// Area page
	c.OnHTML(".mp-sidebar .max-height", func(e *colly.HTMLElement) {
		links := e.ChildAttrs(".lef-nav-row a", "href")
		for _, link := range links {
			c.Visit(link)
		}
	})

	// Found routes on page
	c.OnHTML(".pt-main-content", func(e *colly.HTMLElement) {
		links := e.ChildAttrs(".mp-sidebar table td a", "href")
		for _, link := range links {
			if strings.Contains(link, "route") {
				parsedLink, err := url.Parse(link)
				if err != nil {
					panic(err)
				}
				routeID := strings.Split(parsedLink.Path, "/")[2]
				rawLatLong := "0, 0"
				e.ForEach(".description-details tr", func(_ int, el *colly.HTMLElement) {
					switch el.ChildText("td:first-child") {
					case "GPS:":
						rawLatLong = el.ChildText("td:nth-child(2)")
					}
				})
				latLong := parseLocation(rawLatLong)
				routeLocations.Set(routeID, latLong)
				routeCollector.Visit(link)
			}
		}
	})

	routeCollector.OnHTML("#route-page > div > div:nth-child(1)", func(e *colly.HTMLElement) {
		routeID := strings.Split(e.Request.URL.Path, "/")[2]
		g := parseGrade(e.DOM.Find("#route-page > div > div:nth-child(1) h2 span").Nodes)
		l := Location{}
		if tmp, ok := routeLocations.Get(routeID); ok {
			l = tmp.(Location)
		}
		r := Route{
			Location: l,
			Name:     e.ChildText("h1"),
			Grade:    g,
		}
		_, err = client.Index().
			Index("routes").
			Type("route").
			BodyJson(r).
			Refresh("wait_for").
			Do(context.Background())
		bar.Increment()
	})

	start := time.Now()
	// c.Visit("https://www.mountainproject.com/route-guide")
	c.Visit("https://www.mountainproject.com/area/106374428/new-jersey")
	c.Wait()
	routeCollector.Wait()
	elapsed := time.Since(start)
	bar.Finish()

	// enc := json.NewEncoder(os.Stdout)
	// enc.SetIndent("", "  ")
	// enc.Encode(routes)

	// log.Println("Routes found: ", len(routes))
	log.Println("Execution time:", elapsed)
}
