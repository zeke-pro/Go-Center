package etcd_client

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"reflect"
)

type RemoteStore struct {
	Path         string
	Prefix       bool
	RequireWatch bool
}

type LocalStore struct {
	Path     string
	SyncFile bool
}

type RemoteData struct {
	Key   string
	Value []byte
}

type IStore interface {
	Remote() *RemoteStore
	Local() *LocalStore
	Parse([]*RemoteData) error
}

type Store[T any] struct {
	data   T
	name   string
	local  *LocalStore
	remote *RemoteStore
}

func NewDefaultConfigStore[T any](name string) *Store[T] {
	filePath := path.Join(ConfigDir, fmt.Sprintf("config_%s.json", name))
	var data T
	st := &Store[T]{
		data,
		name,
		&LocalStore{filePath, true},
		&RemoteStore{name, false, true},
	}
	st.readFile()
	return st
}

func NewDefaultServiceStore(name string) *Store[[]*Service] {
	filePath := path.Join(ConfigDir, fmt.Sprintf("service_%s.json", name))
	var data []*Service
	sto := &Store[[]*Service]{
		data,
		name,
		&LocalStore{filePath, true},
		&RemoteStore{name, true, true},
	}
	return sto
}

func NewConfigStore[T any](name string, local *LocalStore, remote *RemoteStore) *Store[T] {
	var data T
	return &Store[T]{
		data, name, local, remote,
	}
}

func (c *Store[T]) Get() T {
	return c.data
}

func (c *Store[T]) Set(data T) error {
	c.data = data
	return c.saveFile()
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
	case reflect.Struct, reflect.Map, reflect.Slice, reflect.Array:
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
func parseKV(kvs []*RemoteData, resType reflect.Type) reflect.Value {
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
			//k := string(kv.Key)
			//arr := strings.Split(k, "/")
			//if len(arr) < 3 {
			//	return n
			//}
			//arr = arr[3:]
			//k = strings.Join(arr, "/")
			key := reflect.ValueOf(kv.Key)
			value := parseBytes(kv.Value, resType.Elem()).Elem()
			mp.SetMapIndex(key, value)
		}
		n.Elem().Set(mp)
	default:
		return reflect.Value{}
	}
	return n
}

func (c *Store[T]) Parse(kvs []*RemoteData) error {
	var data T
	if len(kvs) > 0 {
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
	}
	c.data = data
	//写入文件
	return c.saveFile()
}

func (c *Store[T]) readFile() error {
	if c.local == nil || c.local.Path == "" {
		return nil
	}
	if b, err := os.ReadFile(c.local.Path); err == nil {
		json.Unmarshal(b, c.data)
	}
	return nil
}

// 保存到本地
func (c *Store[T]) saveFile() error {
	l := c.Local()
	if l == nil || !l.SyncFile || l.Path == "" {
		return nil
	}
	file, err := os.Create(l.Path)
	defer file.Close()
	if err != nil {
		return err
	}
	var b []byte
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

func (c *Store[T]) Remote() *RemoteStore {
	return c.remote
}

func (c *Store[T]) Local() *LocalStore {
	return c.local
}
