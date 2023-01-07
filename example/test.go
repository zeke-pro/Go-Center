package main

import (
	"center"
	"center/store"
	"context"
	"encoding/json"
	"fmt"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// 配置
type TestConfig struct {
	Name string `json:"name"`
	Num  int    `json:"num"`
}

/**
环境变量
CENTER_ADDR=127.0.0.1:2379;SERVICE_NAME=test_service;SERVICE_NAMESPACE=center
*/

func main() {
	c, err := center.NewCenter()
	if err != nil {
		panic(err)
	}

	//向etcd添加测试数据
	//addTestData(c.GetEtcdClient())

	//服务发现
	service1 := store.NewDefaultServiceStore("service1")
	err = c.DiscoverServices(service1)
	if err != nil {
		panic(err)
	}
	fmt.Println("发现了服务")
	for _, v := range service1.GetList() {
		fmt.Println(v)
	}

	//配置
	config1 := store.NewDefaultConfigStore[*TestConfig]("config1")
	config2 := store.NewDefaultConfigStore[TestConfig]("config2")
	//字符串配置
	config3 := store.NewDefaultConfigStore[string]("config3")
	//Prefix 映射成数组
	config4 := store.NewConfigStore[[]*TestConfig]("config4", &store.LocalConfig{Path: "config/config4.json", SyncFile: true}, &store.RemoteConfig{Path: "config4", Prefix: true, RequireWatch: true})
	//Prefix 映射成map
	config5 := store.NewConfigStore[map[string]*TestConfig]("config5", &store.LocalConfig{Path: "config/config5.json", SyncFile: true}, &store.RemoteConfig{Path: "config4", Prefix: true, RequireWatch: true})
	//Prefix 映射成字符串数组
	config6 := store.NewConfigStore[[]string]("config6", &store.LocalConfig{Path: "config/config6.json", SyncFile: true}, &store.RemoteConfig{Path: "config6", Prefix: true, RequireWatch: true})
	err = c.SyncConfigs(
		config1,
		config2,
		config3,
		config4,
		config5,
		config6,
	)
	if err != nil {
		panic(err)
	}
	fmt.Println("config1", config1.Get())
	fmt.Println("config2", config2.Get())
	fmt.Println("config3", config3.Get())
	fmt.Println("config4", config4.Get())
	fmt.Println("config5", config5.Get())
	fmt.Println("config6", config6.Get())

	//注册
	//定义自己
	self := center.CreateCurrentServiceFromEnv()
	self.Endpoints = []*store.Endpoint{
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
	c.SetSelf(self)
	defer c.Close()
	err = c.Register()
	if err != nil {
		panic(err)
	}
	for {
	}
}
func addTestData(client *clientv3.Client) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	//服务发现数据
	for i := 1; i < 8; i++ {
		id := fmt.Sprintf("id-%d", i)
		name := fmt.Sprintf("name-%d", i)
		addr := fmt.Sprintf("192.168.0.%d", i)
		service := store.Service{Id: id, Name: name}
		service.Endpoints = []*store.Endpoint{
			{
				Scheme:  "http",
				Address: addr,
				Port:    12888,
			},
			{
				Scheme:  "https",
				Address: addr,
				Port:    12889,
			},
			{
				Scheme:  "grpc",
				Address: addr,
				Port:    12999,
			},
		}
		d, _ := json.Marshal(service)
		str := string(d)
		key := fmt.Sprintf("%s/%s/%s/%s", "center", "service", "service1", id)
		client.Put(ctx, key, str)
	}
	//配置
	//config1
	b, _ := json.Marshal(&TestConfig{Name: "test", Num: 1})
	config1Str := string(b)
	key := fmt.Sprintf("%s/%s/%s", "center", "config", "config1")
	client.Put(ctx, key, config1Str)
	//config2
	key = fmt.Sprintf("%s/%s/%s", "center", "config", "config2")
	client.Put(ctx, key, config1Str)
	//config3
	key = fmt.Sprintf("%s/%s/%s", "center", "config", "config3")
	client.Put(ctx, key, "config3-value")
	//config4,5
	for i := 1; i < 8; i++ {
		key = fmt.Sprintf("%s/%s/%s/key-%d", "center", "config", "config4", i)
		client.Put(ctx, key, config1Str)
	}
	//config6
	for i := 1; i < 8; i++ {
		key = fmt.Sprintf("%s/%s/%s/key-%d", "center", "config", "config6", i)
		value := fmt.Sprintf("value-%d", i)
		client.Put(ctx, key, value)
	}
}
