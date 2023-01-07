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

// 不带Prefix的解析，data必须是引用类型
func parseBytes(b []byte, data any) error {
	tp := reflect.TypeOf(data).Elem()
	switch tp.Kind() {
	case reflect.String:
		v := data.(*string)
		*v = string(b)
	case reflect.Struct:
		err := json.Unmarshal(b, data)
		if err != nil {
			return err
		}
	default:
		return errors.New("type not supported")
	}
	return nil
}

// 带Prefix的解析，data必须是引用类型
func parseKV(kvs []*mvccpb.KeyValue, data any) error {
	if len(kvs) == 0 {
		return nil
	}
	tp := reflect.TypeOf(data).Elem()
	switch tp.Kind() {
	case reflect.Slice:
		slTp := tp.Elem()
		//是否指针
		isPoint := slTp.Kind() == reflect.Ptr
		if isPoint {
			slTp = slTp.Elem()
		}
		sl := reflect.MakeSlice(tp, len(kvs), len(kvs))
		for i, kv := range kvs {
			d := reflect.New(slTp)
			parseFunc := reflect.ValueOf(parseBytes)
			args := []reflect.Value{
				reflect.ValueOf(kv.Value),
				d,
			}
			res := parseFunc.Call(args)[0].Interface()
			if res != nil {
				return res.(error)
			}
			if isPoint {
				sl.Index(i).Set(d)
			} else {
				sl.Index(i).Set(d.Elem())
			}
		}
		a := sl.Interface()
		d := data
		*d = a
	case reflect.Map:
		slTp := tp.Elem()
		//是否指针
		isPoint := slTp.Kind() == reflect.Ptr
		if isPoint {
			slTp = slTp.Elem()
		}
		if slTp.Key().Kind() != reflect.String {
			return errors.New("only map with string key are supported")
		}
		mp := reflect.MakeMapWithSize(tp, len(kvs))
		for _, kv := range kvs {
			key := string(kv.Key)
			d := reflect.New(slTp)
			parseFunc := reflect.ValueOf(parseBytes)
			args := []reflect.Value{
				reflect.ValueOf(kv.Value),
				d,
			}
			res := parseFunc.Call(args)[0].Interface()
			if res != nil {
				return res.(error)
			}
			if isPoint {
				mp.SetMapIndex(reflect.ValueOf(key), d)
			} else {
				mp.SetMapIndex(reflect.ValueOf(key), d.Elem())
			}
		}
		d := data.(*any)
		*d = mp.Interface()
	default:
		return errors.New("type not supported")
	}
	return nil
}

func (c *ConfigStore[T]) Parse(kvs []*mvccpb.KeyValue) error {
	var data T
	var tpr any
	isPoint := reflect.TypeOf(data).Kind() == reflect.Ptr
	if isPoint {
		tpr = data
	} else {
		tpr = &data
	}
	prefix := c.remote.Prefix
	if prefix {
		err := parseKV(kvs, tpr)
		if err != nil {
			return err
		}
	} else {
		if len(kvs) > 1 {
			return errors.New("the remote config format is incorrect")
		}
		if len(kvs) < 1 {
			return errors.New("remote configuration does not exist")
		}
		b := kvs[0].Value
		err := parseBytes(b, tpr)
		if err != nil {
			return err
		}
	}

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
