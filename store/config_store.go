package store

import (
	"center/constant"
	"encoding/json"
	"errors"
	"fmt"
	"go.etcd.io/etcd/api/v3/mvccpb"
	"os"
	"path"
	"reflect"
	"strings"
)

type RemoteConfig struct {
	Path         string
	Prefix       bool
	RequireWatch bool
}

type LocalConfig struct {
	Path     string
	SyncFile bool
}

type ConfigStore[T any] struct {
	data   T
	name   string
	local  *LocalConfig
	remote *RemoteConfig
}

func NewDefaultConfigStore[T any](name string) *ConfigStore[T] {
	filePath := path.Join(constant.ConfigDir, fmt.Sprintf("config_%s.json", name))
	var data T
	st := &ConfigStore[T]{
		data, name, &LocalConfig{filePath, true}, &RemoteConfig{name, false, true},
	}
	st.readFile()
	return st
}

func NewConfigStore[T any](name string, local *LocalConfig, remote *RemoteConfig) *ConfigStore[T] {
	var data T
	return &ConfigStore[T]{
		data, name, local, remote,
	}
}

func (c *ConfigStore[T]) Get() T {
	return c.data
}

func (c *ConfigStore[T]) GetRemotePath() string {
	if c.remote == nil {
		return ""
	}
	return c.remote.Path
}

func newElemWhenNil(data any) {
	if reflect.TypeOf(data).Kind() != reflect.Ptr {
		return
	}
	if !reflect.ValueOf(data).Elem().IsNil() {
		return
	}

}

// 不带Prefix的解析
func parseBytes(b []byte, resType reflect.Type) reflect.Value {
	n := reflect.New(resType)
	if b == nil || len(b) == 0 {
		return n
	}
	switch resType.Kind() {
	case reflect.Pointer:
		child := parseBytes(b, resType.Elem())
		if !child.IsValid() {
			return reflect.Value{}
		}
		n.Elem().Set(child)
	case reflect.Struct:
		unmarshal := reflect.ValueOf(json.Unmarshal)
		args := []reflect.Value{
			reflect.ValueOf(b),
			n,
		}
		r := unmarshal.Call(args)
		if !r[0].IsNil() {
			return reflect.Value{}
		}
	case reflect.String:
		n.Elem().SetString(string(b))
	default:
		return reflect.Value{}
	}
	return n
}

// 带Prefix的解析，data必须是引用类型
func parseKV(kvs []*mvccpb.KeyValue, resType reflect.Type) reflect.Value {
	n := reflect.New(resType)
	if len(kvs) == 0 {
		return n
	}
	switch resType.Kind() {
	case reflect.Pointer:
		child := parseKV(kvs, resType.Elem())
		if !child.IsValid() {
			return reflect.Value{}
		}
		n.Elem().Set(child)
	case reflect.Slice:
		sl := reflect.MakeSlice(resType, len(kvs), len(kvs))
		for i, kv := range kvs {
			v := parseBytes(kv.Value, resType.Elem()).Elem()
			sl.Index(i).Set(v)
		}
		n.Elem().Set(sl)
	case reflect.Map:
		mp := reflect.MakeMapWithSize(resType, len(kvs))
		for _, kv := range kvs {
			k := string(kv.Key)
			arr := strings.Split(k, "/")
			if len(arr) < 3 {
				return n
			}
			arr = arr[3:]
			k = strings.Join(arr, "/")
			key := reflect.ValueOf(k)
			value := parseBytes(kv.Value, resType.Elem()).Elem()
			mp.SetMapIndex(key, value)
		}
		n.Elem().Set(mp)
	default:
		return reflect.Value{}
	}
	return n
}

func (c *ConfigStore[T]) Parse(kvs []*mvccpb.KeyValue) error {
	if len(kvs) == 0 {
		return nil
	}
	var data T
	var parsedValue reflect.Value
	prefix := c.remote.Prefix
	if prefix {
		parsedValue = parseKV(kvs, reflect.TypeOf(data))
	} else {
		parsedValue = parseBytes(kvs[0].Value, reflect.TypeOf(data))
	}
	if !parsedValue.IsValid() {
		return errors.New("parse failed")
	}
	data = parsedValue.Elem().Interface().(T)
	c.data = data
	//写入文件
	if c.local != nil && c.local.SyncFile && c.local.Path != "" {
		err := c.save()
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *ConfigStore[T]) readFile() error {
	if c.local == nil || c.local.Path == "" {
		return nil
	}
	if b, err := os.ReadFile(c.local.Path); err == nil {
		json.Unmarshal(b, c.data)
	}
	return nil
}

func (c *ConfigStore[T]) save() error {
	if c.local == nil || !c.local.SyncFile || c.local.Path == "" {
		return nil
	}
	file, err := os.Create(c.local.Path)
	defer file.Close()
	if err != nil {
		return err
	}
	var b []byte
	//tp := reflect.TypeOf(c.data)
	//if tp.Kind() == reflect.Ptr {
	//	tp = tp.Elem()
	//}
	//switch tp.Kind() {
	//case reflect.String:
	//
	//	file.WriteString(c.data.(string))
	//}

	b, err = json.Marshal(c.data)
	if err != nil {
		return err
	}
	_, err = file.Write(b)
	if err != nil {
		return err
	}
	return nil
}

func (c *ConfigStore[T]) Remote() *RemoteConfig {
	return c.remote
}
