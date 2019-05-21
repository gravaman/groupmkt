package main

import (
	"log"
	"net"
	"net/http"
	"net/rpc"

	"github.com/gravaman/groupmkt/api"
)

type DB map[string]string

func (db *DB) Get(args *api.Load, reply *api.ValueResult) error {
	log.Printf("DB getting %s", args.Key)
	reply.Value = (*db)[args.Key]
	return nil
}

func (db *DB) Set(args *api.Store, reply *api.NullResult) error {
	log.Printf("DB setting %s=%s", args.Key, args.Value)
	(*db)[args.Key] = args.Value
	*reply = 0
	return nil
}

type DBS struct {
	db *DB
}

func (dbs *DBS) Start(port string) {
	rpc.Register(dbs.db)
	rpc.HandleHTTP()
	if l, err := net.Listen("tcp", port); err != nil {
		log.Fatal(err)
	} else {
		http.Serve(l, nil)
	}
}

func NewDBS() *DBS {
	db := make(DB)
	dbs := new(DBS)
	dbs.db = &db
	return dbs
}

func main() {
	dbs := NewDBS()
	log.Printf("db server starting on port 8080")
	dbs.Start(":8080")
}
