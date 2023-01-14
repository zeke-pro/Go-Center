package main

import (
	ec "github.com/zeke-pro/doraemon-go/etcd_client"
)

func main() {
	client, err := ec.NewCenter()
	if err != nil {
		panic(err)
	}
	defer client.Close()
	self := ec.NewCurrentService()
	self.Name = "test-name"
	self.Id = "test-id"
	client.Register(self)
	for {
	}
}
