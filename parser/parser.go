package main

import (
	"fmt"
	"log"
	"strings"

	"golang.org/x/net/html"
)

func parseNode(n *html.Node) {
	if n.Type == html.ElementNode && n.Data == "a" {
		for _, a := range n.Attr {
			if a.Key == "href" {
				fmt.Println(a.Val)
				break
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		parseNode(c)
	}
}

func main() {
	// [TBU]
	// [1] store doc locally
	// [2] read into doc
	// [3] build parser
	// [4] build fetcher
	fmt.Println("parser online")
	s := `<p>Links:</p><ul><li><a href="foo">Foo</a><li><a href="/bar/baz">BarBaz</a></ul>`
	doc, err := html.Parse(strings.NewReader(s))
	if err != nil {
		log.Fatal(err)
	}
	parseNode(doc)
}
