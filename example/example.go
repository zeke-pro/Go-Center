package main

import (
	"fmt"
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

	config1 := ec.NewDefaultConfigStore[string]("testconfig")
	discover := ec.NewDefaultServiceStore("testservice")
	client.SetStores(config1, discover)
	config1.OnReceived(func(nv string, ov string) {
		fmt.Println(nv)
		fmt.Println(ov)
	})
	for {
	}
}
