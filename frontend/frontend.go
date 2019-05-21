package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"strconv"

	"github.com/golang/groupcache"
	"github.com/gravaman/groupmkt/api"
	"github.com/gravaman/groupmkt/client"
)

type Frontend struct {
	cacheGroup *groupcache.Group
}

func (fe *Frontend) Get(args *api.Load, reply *api.ValueResult) error {
	var data []byte
	log.Printf("client request for :%s\n", args.Key)
	err := fe.cacheGroup.Get(nil, args.Key, groupcache.AllocatingByteSliceSink(&data))
	reply.Value = string(data)
	return err
}

func Start(fe *Frontend, port string) {
	rpc.Register(fe)
	rpc.HandleHTTP()
	if l, err := net.Listen("tcp", port); err != nil {
		log.Fatal(err)
	} else {
		http.Serve(l, nil)
	}
}

func NewServer(cacheGroup *groupcache.Group) *Frontend {
	server := new(Frontend)
	server.cacheGroup = cacheGroup
	return server
}

func main() {
	port := flag.String("port", "8001", "groupcache port")
	flag.Parse()

	peers := groupcache.NewHTTPPool("http://localhost:" + *port)
	client := new(client.Client)

	stringcache := groupcache.NewGroup("Cache", 64<<20, groupcache.GetterFunc(
		func(ctx groupcache.Context, key string, dest groupcache.Sink) error {
			result := client.Get(key)
			log.Printf("Client requesting %s\n", key)
			dest.SetBytes([]byte(result))
			return nil
		}))

	peers.Set("http://localhost:8001", "http://localhost:8002", "http://localhost:8003")
	fes := NewServer(stringcache)

	i, err := strconv.Atoi(*port)
	if err != nil {
		log.Fatal(err)
	}

	fep := ":" + strconv.Itoa(i+1000)
	go Start(fes, fep)

	log.Println(stringcache)
	log.Printf("cachegroup slave starting on port %d", i)
	log.Printf("frontend starting on port %d", i)

	http.ListenAndServe("127.0.0.1:"+*port, http.HandlerFunc(peers.ServeHTTP))
}
