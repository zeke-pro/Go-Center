package main

import (
	"context"
	ec "github.com/zeke-pro/doraemon-go/etcd_client"
	"time"
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
	time.Sleep(time.Second * 2)
	self.Name = "edit-name"
	client.Register(self)
	context.TODO().Done()
	for {
	}
}
