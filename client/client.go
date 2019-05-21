// Package client provides cli interface for frontend nodes.
package client

import (
	"log"
	"net/rpc"

	"github.com/gravaman/groupmkt/api"
)

// Client provides the service entry interface.
type Client struct{}

// Get makes the rpc request for the key.
func (c *Client) Get(key string) string {
	client, err := rpc.DialHTTP("tcp", "localhost:8080")
	if err != nil {
		log.Fatal(err)
	}
	args := &api.Load{key}
	var reply api.ValueResult
	if err := client.Call("DB.Get", args, &reply); err != nil {
		log.Fatal(err)
	}

	return string(reply.Value)
}

// Set makes the rpc store request for the key value pair.
func (c *Client) Set(key, val string) int {
	client, err := rpc.DialHTTP("tcp", "localhost:8080")
	if err != nil {
		log.Fatal(err)
	}
	args := &api.Store{key, val}
	var reply int
	if err := client.Call("DB.Set", args, &reply); err != nil {
		log.Fatal(err)
	}
	return reply
}
