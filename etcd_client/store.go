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
	"sync"
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
	data        T
	version     int64
	local       *LocalConfig
	remote      *RemoteConfig
	m           sync.RWMutex                   //读写锁
	subscribers map[chan *ReceivedData[T]]bool //订阅列表,key是channel，value是是否一次性
	modify      func(data T) T
}

func NewDefaultConfigStore[T any](name string) *Store[T] {
	filePath := path.Join(envConfInstance.ConfigDir, fmt.Sprintf("config_%s.json", name))
	key := fmt.Sprintf("%s/%s/%s", envConfInstance.ServiceNamespace, "config", name)
	var data T
	st := &Store[T]{
		m:           sync.RWMutex{},
		subscribers: make(map[chan *ReceivedData[T]]bool),
		data:        data,
		local:       &LocalConfig{filePath, true},
		remote:      &RemoteConfig{key, false, true},
	}
	return st
}

type ServiceStore = Store[[]*Service]

// NewVersionServiceStore 带版本号的Service
func NewVersionServiceStore(name string, version string) *ServiceStore {
	filePath := path.Join(envConfInstance.ConfigDir, fmt.Sprintf("service_%s.json", name))
	key := fmt.Sprintf("%s/%s/%s", envConfInstance.ServiceNamespace, "service", name)
	var data []*Service
	sto := &ServiceStore{
		m:           sync.RWMutex{},
		subscribers: make(map[chan *ReceivedData[[]*Service]]bool),
		data:        data,
		local:       &LocalConfig{filePath, true},
		remote:      &RemoteConfig{key, true, true},
		modify: func(list []*Service) []*Service {
			if list == nil || len(list) == 0 {
				return list
			}
			res := make([]*Service, 0)
			for _, s := range list {
				if s.Version == version {
					res = append(res, s)
				}
			}
			return res
		},
	}
	return sto
}

func NewDefaultServiceStore(name string) *ServiceStore {
	filePath := path.Join(envConfInstance.ConfigDir, fmt.Sprintf("service_%s.json", name))
	key := fmt.Sprintf("%s/%s/%s", envConfInstance.ServiceNamespace, "service", name)
	var data []*Service
	sto := &ServiceStore{
		m:           sync.RWMutex{},
		subscribers: make(map[chan *ReceivedData[[]*Service]]bool),
		data:        data,
		local:       &LocalConfig{filePath, true},
		remote:      &RemoteConfig{key, true, true},
	}
	return sto
}

func (s *Store[T]) RegisterModify(f func(data T) T) {
	s.modify = f
}

// OnceSubscribe 一次性订阅，收到信息后将channel删除
func (s *Store[T]) OnceSubscribe() chan *ReceivedData[T] {
	ch := make(chan *ReceivedData[T])
	s.m.Lock()
	s.subscribers[ch] = true
	s.m.Unlock()
	return ch
}

// Subscribe 普通订阅
func (s *Store[T]) Subscribe() chan *ReceivedData[T] {
	ch := make(chan *ReceivedData[T])
	s.m.Lock()
	s.subscribers[ch] = false
	s.m.Unlock()
	return ch
}

// Unsubscribe 取消订阅
func (s *Store[T]) Unsubscribe(ch chan *ReceivedData[T]) {
	s.m.Lock()
	defer s.m.Unlock()
	_, has := s.subscribers[ch]
	if !has {
		return
	}
	close(ch)
	delete(s.subscribers, ch)
}

// publish 发布
func (s *Store[T]) publish(dt *ReceivedData[T]) {
	s.m.Lock()
	defer s.m.Unlock()
	for ch, once := range s.subscribers {
		select {
		case ch <- dt:
		//成功
		default:
			log.Println("推送失败")
		}
		if once {
			close(ch)
			delete(s.subscribers, ch)
		}
	}
}

func (s *Store[T]) GetVersion() int64 {
	return s.version
}

func (s *Store[T]) GetData() T {
	return s.data
}

// ReceiveData 解析远程数据
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
	if s.modify != nil {
		data = s.modify(data)
	}
	//原始版本非0
	if s.version != 0 {
		rd := &ReceivedData[T]{
			NewData: data,
			OldData: s.data,
		}
		go s.publish(rd)
	}
	s.data = data
	s.version = version
	//写入文件
	if local := s.Local(); local != nil && local.RequireWrite {
		go func() {
			err := s.saveFile()
			if err != nil {
				log.Printf("failed to write update to file.error:%s\n", err)
			}
		}()
	}
	return nil
}

// ReadFile 从本地读取
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

// 不带Prefix的解析
func parseBytes(b []byte, resType reflect.Type) reflect.Value {
	defer func() {
		if err := recover(); err != nil {
			fmt.Printf("解析失败，错误 %s\n", err)
		}
	}()
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
	defer func() {
		if err := recover(); err != nil {
			fmt.Printf("解析失败，错误 %s\n", err)
		}
	}()
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
			key := reflect.ValueOf(keyStr)
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
