package main_test

import (
	"fmt"
	ec "github.com/zeke-pro/doraemon-go/etcd_client"
	_ "net/http/pprof"
	"testing"
	"time"
)

type Student struct {
	Name string
	Age  int
}

func TestStorePut(t *testing.T) {
	c, err := ec.NewCenter()
	if err != nil {
		panic(err)
	}
	store1 := ec.NewDefaultConfigStore[string]("put_config")
	store1.Put("test", "test", c)

	store2 := ec.NewDefaultConfigStore[Student]("stu_config")
	store2.Put("stu", Student{Name: "stu", Age: 18}, c)

	err = c.SyncConfigs(
		store1,
		store2,
	)
	if err != nil {
		panic(err)
	}
}

func TestStorePrefix(t *testing.T) {
	store1 := ec.NewConfigStore[string]("prefix_config", &ec.LocalStore{Path: "config/prefix_config.txt", SyncFile: true, RequireWatch: false}, &ec.RemoteStore{Path: "prefix_config", Prefix: false, RequireWatch: false})

	c, err := ec.NewCenter()
	if err != nil {
		panic(err)
	}

	fmt.Println("store1.Local().Path=", store1.Local().Path)
	fmt.Println("store1.Remote().Path=", store1.Remote().Path)

	store1.Set("Test")
	store1.Put("test", "test", c) //Local.RequireWatch=false 不会Put到远程
	c.SyncConfigs(store1)

	store2 := ec.NewConfigStore[string]("prefix_2_config", &ec.LocalStore{Path: "config/prefix_2_config.txt", SyncFile: true, RequireWatch: true}, &ec.RemoteStore{Path: "prefix_2_config", Prefix: false, RequireWatch: false})

	fmt.Println("store2.Local().Path=", store2.Local().Path)
	fmt.Println("store2.Remote().Path=", store2.Remote().Path)

	store2.Set("Test")
	store2.WatchLocal(c)
	store2.Put("test", "test", c) //Local.RequireWatch=false 不会Put到远程
	c.SyncConfigs(store2)

}

func TestStorePutAndWatchLocal(t *testing.T) {
	// go profile 功能
	//go func() {
	//	log.Println(http.ListenAndServe("localhost:6060", nil))
	//}()

	c, err := ec.NewCenter()
	if err != nil {
		panic(err)
	}

	store1 := ec.NewConfigStore[string]("put_config", &ec.LocalStore{Path: "config/put_config.json", SyncFile: true, RequireWatch: true}, &ec.RemoteStore{Path: "put_config", Prefix: false, RequireWatch: true})
	//store1 := ec.NewDefaultConfigStore[string]("put_watch")
	store1.WatchLocal(c)
	store1.Put("test", "test", c)

	//store2 := ec.NewDefaultConfigStore[Student]("stu_config_watch")
	store2 := ec.NewConfigStore[Student]("stu_config_watch", &ec.LocalStore{Path: "config/stu_config_watch.json", SyncFile: true, RequireWatch: true}, &ec.RemoteStore{Path: "stu_config_watch", Prefix: false, RequireWatch: true})
	//最好是local.RequireWatch = true,自动调用WatchLocal(c),调用需要Center，比较麻烦
	store2.WatchLocal(c) // watch local change，and sync to remote
	store2.Put("stu", Student{Name: "stu", Age: 18}, c)

	err = c.SyncConfigs(
		store1,
		store2,
	)
	if err != nil {
		panic(err)
	}

	//第一轮更新
	store1.Put("test", "test-new", c)
	store2.Put("stu", Student{Name: "stu-new", Age: 19}, c)

	//第二轮更新
	store1.Put("test", "test-new-again", c)
	store2.Put("stu", Student{Name: "stu-new-again", Age: 19}, c)

	time.Sleep(5 * time.Second)

}
