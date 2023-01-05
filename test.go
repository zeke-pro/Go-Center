package main

import (
	"center/datastore"
	"fmt"
)

func main() {
	//定义
	self := CreateCurrentServiceFromEnv()
	self.Endpoints = []*datastore.Endpoint{
		{
			Scheme:  "http",
			Address: "127.0.0.1",
			Port:    12888,
		},
		{
			Scheme:  "https",
			Address: "127.0.0.1",
			Port:    12889,
		},
		{
			Scheme:  "grpc",
			Address: "127.0.0.1",
			Port:    12999,
		},
	}
	//服务发现,从本地文件读取
	service1Store := datastore.NewServiceStore("service1")

	//配置
	type TestConfig struct {
		Name  string `json:"name"`
		Param string `json:"param"`
	}
	config1Store := datastore.NewConfigStore("config1", &TestConfig{}, "testConfig")

	if center, err := NewCenter(CurrentService(self)); err == nil {
		defer center.Close()
		//注册
		err = center.Register()
		if err != nil {
			panic(err)
		}
		//接管服务发现
		center.TakeOverServiceList(service1Store)
		//接管配置
		center.TakeOverConfig(config1Store)
		config1Store.OnChange(func(newValue any) {
			conf := newValue.(*TestConfig)
			fmt.Printf(conf.Name)
		})
		//center.Close()
	}

	for {
	}
}
