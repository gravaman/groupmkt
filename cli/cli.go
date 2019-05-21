package main

import (
	"flag"
	"log"
	"net/rpc"

	"github.com/gravaman/groupmkt/api"
	"github.com/gravaman/groupmkt/client"
)

func main() {
	port := flag.String("port", "9001", "frontend port")
	set := flag.Bool("set", false, "set a key-value pair")
	get := flag.Bool("get", false, "get a value for key")
	cget := flag.Bool("cget", false, "get a value for key using group cache")
	key := flag.String("key", "foo", "key to get")
	val := flag.String("val", "bar", "value to set")
	flag.Parse()

	client := new(client.Client)

	switch {
	case *cget:
		client, err := rpc.DialHTTP("tcp", "localhost:"+*port)
		if err != nil {
			log.Fatal(err)
		}
		args := &api.Load{*key}
		var reply api.ValueResult
		if err := client.Call("Frontend.Get", args, &reply); err != nil {
			log.Fatal(err)
		}
		log.Printf("Client cget result: %s", string(reply.Value))
	case *get:
		log.Printf("Client get result: %s", client.Get(*key))
	case *set:
		log.Printf("Client set result: %d", client.Set(*key, *val))
	default:
		flag.PrintDefaults()
	}
}
