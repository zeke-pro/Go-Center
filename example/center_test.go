package main_test

import (
	ec "github.com/zeke-pro/doraemon-go/etcd_client"
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

func TestStorePutAndWatchLocal(t *testing.T) {
	c, err := ec.NewCenter()
	if err != nil {
		panic(err)
	}

	store1 := ec.NewConfigStore[string]("put_config", &ec.LocalStore{Path: "config/put_config.json", SyncFile: true, RequireWatch: true}, &ec.RemoteStore{Path: "put_config", Prefix: false, RequireWatch: true})
	//store1 := ec.NewDefaultConfigStore[string]("put_watch")
	store1.Put("test", "test", c)

	//store2 := ec.NewDefaultConfigStore[Student]("stu_config_watch")
	store2 := ec.NewConfigStore[Student]("stu_config_watch", &ec.LocalStore{Path: "config/stu_config_watch.json", SyncFile: true, RequireWatch: true}, &ec.RemoteStore{Path: "stu_config_watch", Prefix: false, RequireWatch: true})
	store2.Put("stu", Student{Name: "stu", Age: 18}, c)

	err = c.SyncConfigs(
		store1,
		store2,
	)
	if err != nil {
		panic(err)
	}

	store1.Put("test", "test-new", c)
	store2.Put("stu", Student{Name: "stu-new", Age: 19}, c)

	store1.Put("test", "test-new-again", c)
	store2.Put("stu", Student{Name: "stu-new-again", Age: 19}, c)

	time.Sleep(5 * time.Second)

}
