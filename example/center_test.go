package main_test

import (
	_ "net/http/pprof"
	"testing"
)

type Student struct {
	Name string
	Age  int
}

type Service struct {
	Id   interface{}
	Name interface{}
}

func TestConfigPutWatch(t *testing.T) {

}
