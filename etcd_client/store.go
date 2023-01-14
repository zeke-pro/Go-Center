package etcd_client

import (
	"encoding/json"
	"errors"
	"fmt"
	"go.etcd.io/etcd/api/v3/mvccpb"
	"log"
	"os"
	"path"
	"reflect"
	"strings"
)

type RemoteConfig struct {
	Key          string //完整配置文件key
	Prefix       bool
	RequireWatch bool //是否watch远程变化
}

type LocalConfig struct {
	Path         string //完整路径
	RequireWrite bool   //有更新是否写入本地配置文件
}

type ReceivedData[T any] struct {
	OldData T
	NewData T
}

type IStore interface {
	Remote() *RemoteConfig
	Local() *LocalConfig
	GetVersion() int64
	ReceiveData(kvs []*mvccpb.KeyValue, version int64) error
}

type Store[T any] struct {
	data          T
	version       int64
	local         *LocalConfig
	remote        *RemoteConfig
	recNotifyList []chan *ReceivedData[T]
}

func NewDefaultConfigStore[T any](name string) *Store[T] {
	filePath := path.Join(ConfigDir, fmt.Sprintf("config_%s.json", name))
	key := fmt.Sprintf("%s/%s/%s", ServiceNamespace, "config", name)
	var data T
	st := &Store[T]{
		data:   data,
		local:  &LocalConfig{filePath, true},
		remote: &RemoteConfig{key, false, true},
	}
	return st
}

type ServiceStore = Store[[]*Service]

func NewDefaultServiceStore(name string) *ServiceStore {
	filePath := path.Join(ConfigDir, fmt.Sprintf("service_%s.json", name))
	key := fmt.Sprintf("%s/%s/%s", ServiceNamespace, "service", name)
	var data []*Service
	sto := &ServiceStore{
		data:   data,
		local:  &LocalConfig{filePath, true},
		remote: &RemoteConfig{key, true, true},
	}
	return sto
}

func (s *Store[T]) ReceivedNotify() <-chan *ReceivedData[T] {
	ch := make(chan *ReceivedData[T])
	s.recNotifyList = append(s.recNotifyList, ch)
	return ch
}

func (s *Store[T]) GetVersion() int64 {
	return s.version
}

func (s *Store[T]) GetData() T {
	return s.data
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
		str := string(b)
		str = strings.Trim(str, "\"") //去掉双引号,否则 "test3" 会变成  "\"test3\""
		n.Elem().SetString(str)
	default:
		log.Printf("cannot resolve type %s \n", resType.Kind().String())
		return reflect.Value{}
	}
	return n
}

// 带Prefix的解析，data必须是引用类型
func parseKV(kvs []*mvccpb.KeyValue, resType reflect.Type, remoteKey string) reflect.Value {
	n := reflect.New(resType)
	if len(kvs) == 0 {
		return n
	}
	switch resType.Kind() {
	case reflect.Pointer:
		child := parseKV(kvs, resType.Elem(), remoteKey)
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
			keyStr := string(kv.Key)
			keyStr = keyStr[len(remoteKey):]
			key := reflect.ValueOf(kv.Key)
			value := parseBytes(kv.Value, resType.Elem()).Elem()
			mp.SetMapIndex(key, value)
		}
		n.Elem().Set(mp)
	default:
		log.Printf("cannot resolve type %s \n", resType.Kind().String())
		return reflect.Value{}
	}
	return n
}

// 解析远程数据
func (s *Store[T]) ReceiveData(kvs []*mvccpb.KeyValue, version int64) error {
	if s.version == version {
		return nil
	}
	var data T
	if len(kvs) > 0 {
		var parsedValue reflect.Value
		prefix := s.remote.Prefix
		if prefix {
			prefixStr := fmt.Sprintf("%s/", s.Remote().Key)
			parsedValue = parseKV(kvs, reflect.TypeOf(data), prefixStr)
		} else {
			parsedValue = parseBytes(kvs[0].Value, reflect.TypeOf(data))
		}
		if !parsedValue.IsValid() {
			return errors.New("parse failed")
		}
		data = parsedValue.Elem().Interface().(T)
	}
	//原始版本非0
	if len(s.recNotifyList) > 0 && s.version != 0 {
		for _, notify := range s.recNotifyList {
			notify <- &ReceivedData[T]{
				NewData: data,
				OldData: s.data,
			}
		}
	}
	s.data = data
	s.version = version
	//写入文件
	if local := s.Local(); local != nil && local.RequireWrite {
		err := s.saveFile()
		if err != nil {
			return err
		}
	}
	return nil
}

// 从本地读取
func (s *Store[T]) ReadFile() error {
	if s.local == nil || s.local.Path == "" {
		return nil
	}
	if b, err := os.ReadFile(s.local.Path); err == nil {
		er := json.Unmarshal(b, s.data)
		if er != nil {
			return er
		}
	}
	return nil
}

// 保存到本地
func (s *Store[T]) saveFile() error {
	l := s.Local()
	if l == nil || !l.RequireWrite || l.Path == "" {
		return nil
	}
	file, err := os.Create(l.Path)
	defer file.Close()
	if err != nil {
		return err
	}
	var b []byte
	b, err = json.Marshal(s.data)
	if err != nil {
		return err
	}
	_, err = file.Write(b)
	if err != nil {
		return err
	}
	return nil
}

func (s *Store[T]) Remote() *RemoteConfig {
	return s.remote
}

func (s *Store[T]) Local() *LocalConfig {
	return s.local
}
