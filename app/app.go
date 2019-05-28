package main

import (
	"flag"
	"log"
	"net/http"
)

var (
	debug = flag.Bool("d", false, "debug mode")
	port  = flag.String("p", "8080", "port")
)

func main() {
	flag.Parse()
	if *debug {
		log.Println("debug mode turned on")
	}

	fs := http.FileServer(http.Dir("static"))
	http.Handle("/", fs)

	log.Printf("server listening at localhost:%s", *port)
	log.Fatal(http.ListenAndServe(":"+*port, nil))
}
