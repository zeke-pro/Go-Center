package main_test

import (
	ec "github.com/zeke-pro/doraemon-go/etcd_client"
	"testing"
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
