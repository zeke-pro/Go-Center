package main_test

import (
	"fmt"
	"github.com/google/uuid"
	ec "github.com/zeke-pro/doraemon-go/etcd_client"
	"log"
	"net/http"
	_ "net/http/pprof"
	"testing"
	"time"
)

type Student struct {
	Name string
	Age  int
}

type Service struct {
	Id   interface{}
	Name interface{}
}

func TestStorePut(t *testing.T) {
	c, err := ec.NewCenter()
	if err != nil {
		panic(err)
	}
	store1 := ec.NewDefaultConfigStore[string]("put_config")
	store1.Set("test")

	store2 := ec.NewDefaultConfigStore[Student]("stu_config")

	err = c.SyncConfigs(
		store1,
		store2,
	)
	if err != nil {
		panic(err)
	}
}

func TestStoreRequirePut(t *testing.T) {

	c, err := ec.NewCenter()
	if err != nil {
		panic(err)
	}

	store1 := ec.NewConfigStore[string]("put_config", &ec.LocalConfig{Path: "config/put_config.json", RequireWrite: true}, &ec.RemoteConfig{Path: "put_config", Prefix: false, RequireWatch: false})

	store2 := ec.NewConfigStore[Student]("stu_config_watch", &ec.LocalConfig{Path: "config/stu_config_watch.json", RequireWrite: true}, &ec.RemoteConfig{Path: "stu_config_watch", RequirePut: true, Prefix: false, RequireWatch: false})
	//最好是local.RequireWatch = true,自动调用WatchLocal(c),调用需要Center，比较麻烦

	err = c.SyncConfigs(
		store1,
		store2,
	)
	if err != nil {
		panic(err)
	}

	store1.Set("test")
	store2.Set(Student{Name: "stu", Age: 18})

	//第一轮更新
	store1.Set("test-new")
	store2.Set(Student{Name: "stu-new", Age: 19})

	//第二轮更新
	store1.Set("test-new-again")
	store2.Set(Student{Name: "stu-new-again", Age: 20})

	time.Sleep(5 * time.Second)

}

func TestStoreRequireWatch(t *testing.T) {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	c, err := ec.NewCenter()
	if err != nil {
		panic(err)
	}

	store1 := ec.NewConfigStore[string]("require_config", &ec.LocalConfig{Path: "config/require_watch.json", RequireWrite: true}, &ec.RemoteConfig{Path: "require_watch", Prefix: false, RequireWatch: true, RequirePut: true})

	err = c.SyncConfigs(
		store1,
	)
	if err != nil {
		panic(err)
	}

	store1.Set("test1")
	store1.Set("test2")
	store1.Set("test3")
	var v string = store1.Get()
	fmt.Printf("v:%v\n", v)

	time.Sleep(3000 * time.Second)

}

func TestService(t *testing.T) {
	c, err := ec.NewCenter()
	if err != nil {
		panic(err)
	}

	store := ec.NewDefaultServiceStore("serviceX")

	err = c.DiscoverServices(store)
	if err != nil {
		panic(err)
	}
	fmt.Println("发现了服务")
	for _, v := range store.Get() {
		fmt.Println(v)
	}

	uid, err := uuid.NewUUID()
	if err != nil {
		panic(err)
	}

	testService := ec.Service{
		Id:   uid.String(),
		Name: "serviceX",
	}

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
	testService.Endpoints = endpoints

	c.SetSelf(&testService)
	c.Register()

	time.Sleep(time.Second * 10)

}
