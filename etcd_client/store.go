package etcd_client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"log"
	"os"
	"path"
	"reflect"
)

type OnChange func(newValue any, oldValue any)

type RemoteConfig struct {
	Path         string
	Prefix       bool
	RequireWatch bool             //是否watch远程变化
	RequirePut   bool             //是否需要将set的数据put到远程
	PutChan      chan interface{} //本地put信号
	OnChange     *OnChange        //远程变化的回调方法（只有RequireWatch为true时有效）
}

type LocalConfig struct {
	Path         string
	RequireWrite bool //有更新是否写入本地配置文件
	//RequireWatch bool
	//Channel      chan *RemoteData
}

type RemoteData struct {
	Key   string
	Value []byte
}

type IStore interface {
	Remote() *RemoteConfig
	Get() any
	Local() *LocalConfig
	Parse([]*RemoteData) error
}

type Store[T any] struct {
	data   T
	name   string
	local  *LocalConfig
	remote *RemoteConfig
}

func NewDefaultConfigStore[T any](name string) *Store[T] {
	if _, err := os.Stat(ConfigDir); os.IsNotExist(err) {
		err = os.Mkdir(ConfigDir, os.ModePerm)
		if err != nil {
			panic(err)
		}
	}

	filePath := path.Join(ConfigDir, fmt.Sprintf("config_%s.json", name))
	var data T
	st := &Store[T]{
		data,
		name,
		&LocalConfig{filePath, true, false, nil},
		&RemoteConfig{name, false, true, make(chan *RemoteData)},
	}
	st.readFile()
	return st
}

func NewDefaultServiceStore(name string) *Store[[]*Service] {
	if _, err := os.Stat(ConfigDir); os.IsNotExist(err) {
		err = os.Mkdir(ConfigDir, os.ModePerm)
		if err != nil {
			panic(err)
		}
	}

	filePath := path.Join(ConfigDir, fmt.Sprintf("service_%s.json", name))
	var data []*Service
	sto := &Store[[]*Service]{
		data,
		name,
		&LocalConfig{filePath, true, false, nil},
		&RemoteConfig{name, true, true, make(chan *RemoteData)},
	}
	return sto
}

func NewConfigStore[T any](name string, local *LocalConfig, remote *RemoteConfig) *Store[T] {
	var data T
	if local.RequireWatch {
		if local.Channel == nil {
			local.Channel = make(chan *RemoteData)
		}
	}
	store := &Store[T]{
		data, name, local, remote,
	}

	return store
}

func (s *Store[T]) Get() T {
	return s.data
}

func (s *Store[T]) Set(data T) error {
	s.data = data
	err := s.saveFile()
	if err != nil {
		return err
	}
	if remote := s.Remote(); remote != nil {
		if remote.RequirePut && remote.PutChan != nil {
			//问题。。。。
			remote.PutChan <-
		}
	}
	return nil
}

// 更新到本地,如果Local开启RequireWatch，会触发本地监听，本地监听会更新到远端
func (s *Store[T]) Put(key string, data T, center *Center) {
	s.Set(data)
	s.saveFile()

	value, err := s.SerializeValue(data)
	if err != nil {
		return
	}

	if s.local.RequireWatch {
		s.local.Channel <- &RemoteData{Key: key, Value: []byte(value)}
	}

}

func (s *Store[T]) SerializeValue(value T) (string, error) {
	//if value == nil {
	//	return "", nil
	//}
	str, err := json.Marshal(value)

	return string(str), err
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

func (s *Store[T]) WatchRemote(center *Center) {
	if s.remote == nil || s.remote.Path == "" {
		return
	}

	ctx := context.Background()
	fullKey := getConfigKey(ServiceNamespace, s.remote.Path)
	watchChan := center.client.Watcher.Watch(ctx, fullKey, clientv3.WithPrefix())
	center.client.Watcher.RequestProgress(ctx)

	go func() {
		for {
			select {
			case resp := <-watchChan:
				for _, ev := range resp.Events {
					switch ev.Type {
					case mvccpb.PUT:
						rd := &RemoteData{Key: string(ev.Kv.Key), Value: ev.Kv.Value}
						rds := []*RemoteData{rd}
						s.Parse(rds) //解析更新到本地
						s.Remote().Channel <- rd
					case mvccpb.DELETE:
						//TODO 删除逻辑是否需要？
						fmt.Printf("DELETE key:%s\n", ev.Kv.Key)
					}
				}
			}
		}

	}()
}

func (s *Store[T]) WatchLocal(c *Center) {
	if s.local.RequireWatch {
		if s.local == nil || s.local.Path == "" {
			return
		}
		go func() {
			for {
				select {
				case rd := <-s.local.Channel:
					log.Printf("WatchLocal, s.name=%s, key=%s,value=%s \n", s.name, rd.Key, string(rd.Value))
					fullKey := getConfigKey(ServiceNamespace, s.local.Path)
					c.client.Put(context.Background(), fullKey, string(rd.Value))
				}
			}
		}()
	} else {
		log.Printf("WatchLocal, s.name=%s, 不需要监听本地变化 \n", s.name)
	}

}

func getConfigKey(namespace string, path string) string {
	key := fmt.Sprintf("%s/%s/%s", namespace, "config", path)
	return key
}

func (s *Store[T]) Parse(kvs []*RemoteData) error {
	var data T
	if len(kvs) > 0 {
		var parsedValue reflect.Value
		prefix := s.remote.Prefix
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
	s.data = data
	//写入文件
	return s.saveFile()
}

func (s *Store[T]) readFile() error {
	if s.local == nil || s.local.Path == "" {
		return nil
	}

	if b, err := os.ReadFile(s.local.Path); err == nil {
		json.Unmarshal(b, s.data)
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
