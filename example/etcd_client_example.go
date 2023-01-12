package main

import (
	"context"
	"encoding/json"
	"fmt"
	ec "github.com/zeke-pro/doraemon-go/etcd_client"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// 配置
type TestConfig struct {
	Name string `json:"name"`
	Num  int    `json:"num"`
}

/**
环境变量
ETCD_ADDR=127.0.0.1:2379;SERVICE_NAME=test_service;SERVICE_NAMESPACE=center
*/

func main() {
	c, err := ec.NewCenter()
	if err != nil {
		panic(err)
	}

	//向etcd添加测试数据
	addTestData(c.GetEtcdClient())

	//服务发现
	service1 := ec.NewDefaultServiceStore("service1")
	err = c.DiscoverServices(service1)
	if err != nil {
		panic(err)
	}
	fmt.Println("发现了服务")
	for _, v := range service1.Get() {
		fmt.Println(v)
	}

	//配置
	store1 := ec.NewDefaultConfigStore[*TestConfig]("config1")
	store2 := ec.NewDefaultConfigStore[TestConfig]("config2")
	//字符串配置
	store3 := ec.NewDefaultConfigStore[string]("config3")
	//Prefix 映射成数组
	store4 := ec.NewConfigStore[[]*TestConfig]("config4", &ec.LocalConfig{Path: "config/config4.json", RequireWrite: true}, &ec.RemoteConfig{Path: "config4", Prefix: true, RequireWatch: true})
	//Prefix 映射成map
	store5 := ec.NewConfigStore[map[string]*TestConfig]("config5", &ec.LocalConfig{Path: "config/config5.json", RequireWrite: true}, &ec.RemoteConfig{Path: "config4", Prefix: true, RequireWatch: true})
	//Prefix 映射成字符串数组
	store6 := ec.NewConfigStore[[]string]("config6", &ec.LocalConfig{Path: "config/config6.json", RequireWrite: true}, &ec.RemoteConfig{Path: "config6", Prefix: true, RequireWatch: true})
	config9_remote := &ec.RemoteConfig{Path: "config9", Prefix: true, RequireWatch: true, RequirePut: false, SetChan: make(chan interface{})}
	store9 := ec.NewConfigStore[[]string]("config9", &ec.LocalConfig{Path: "config/config9.json", RequireWrite: true}, config9_remote)
	err = c.SyncConfigs(
		store1,
		store2,
		store3,
		store4,
		store5,
		store6,
		store9,
	)
	//.........

	if err != nil {
		panic(err)
	}
	fmt.Println("config1", store1.Get())
	fmt.Println("config2", store2.Get())
	fmt.Println("config3", store3.Get())
	fmt.Println("config4", store4.Get())
	fmt.Println("config5", store5.Get())
	fmt.Println("config6", store6.Get())

	//注册
	//定义自己
	self := ec.CreateCurrentServiceFromEnv()
	endpoints := []*ec.Endpoint{
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
	self.Endpoints = endpoints

	c.SetSelf(self)
	defer c.Close()
	err = c.Register()
	if err != nil {
		panic(err)
	}

	//注册第二个服务
	//self2 := ec.CreateCurrentServiceFromEnv()
	//uid, err := uuid.NewUUID()
	//if err != nil {
	//	panic(err)
	//}
	//
	//self2.Id = uid.String()
	//self2.Endpoints = endpoints
	//c.SetSelf(self2)
	//
	//err = c.Register()
	//if err != nil {
	//	panic(err)
	//}

	for {

	}
}
func addTestData(client *clientv3.Client) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	//服务发现数据
	for i := 1; i < 3; i++ {
		id := fmt.Sprintf("id-%d", i)
		name := fmt.Sprintf("name-%d", i)
		addr := fmt.Sprintf("192.168.0.%d", i)
		service := ec.Service{Id: id, Name: name}
		service.Endpoints = []*ec.Endpoint{
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
	//b, _ := json.Marshal(&TestConfig{Name: "test", Num: 1})
	//config1Str := string(b)
	//key := fmt.Sprintf("%s/%s/%s", "center", "config", "config1")
	//client.Put(ctx, key, config1Str)
	////config2
	//key = fmt.Sprintf("%s/%s/%s", "center", "config", "config2")
	//client.Put(ctx, key, config1Str)
	////config3
	//key = fmt.Sprintf("%s/%s/%s", "center", "config", "config3")
	//client.Put(ctx, key, "config3-value")
	////config4,5
	//for i := 1; i < 8; i++ {
	//	key = fmt.Sprintf("%s/%s/%s/key-%d", "center", "config", "config4", i)
	//	client.Put(ctx, key, config1Str)
	//}
	////config6
	//for i := 1; i < 8; i++ {
	//	key = fmt.Sprintf("%s/%s/%s/key-%d", "center", "config", "config6", i)
	//	value := fmt.Sprintf("value-%d", i)
	//	client.Put(ctx, key, value)
	//}

	key := fmt.Sprintf("%s/%s/%s/key-%d", "center", "config", "config9", 1)
	value := fmt.Sprintf("value-%d", 1)
	client.Put(ctx, key, value)
}
