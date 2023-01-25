package main

import (
	ec "github.com/zeke-pro/doraemon-go/etcd_client"
	"time"
)

func main() {
	conf, err := ec.NewDefaultCenterConfig()
	if err != nil {
		panic(err)
	}
	client, err := ec.NewCenter(conf)
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
	for {
	}
}
