package datastore

import (
	"center/constant"
	"encoding/json"
	"os"
	"path/filepath"
)

type IEtcdAddrStore interface {
	Get() []string
	Set([]string) error
}

type EtcdAddrStore struct {
	list        []string
	filePath    string
	syncFile    bool
	defaultAddr string
}

const fileName string = "center.json"

func NewEtcdAddrStoreFromEnv() *EtcdAddrStore {
	dir := constant.ConfigDir
	fp := filepath.Join(dir, fileName)
	var list []string
	// 从配置文件读取初始化list
	if data, err := os.ReadFile(fp); err == nil {
		json.Unmarshal(data, &list)
	}
	addr := constant.CenterAddr
	return &EtcdAddrStore{
		list, fp, true, addr,
	}
}

func (e *EtcdAddrStore) Get() []string {
	list := e.list
	d := e.defaultAddr
	if d != "" {
		dup := func() bool {
			for _, v := range list {
				if v == d {
					return true
				}
			}
			return false
		}()
		if !dup {
			list = append([]string{d}, list...)
		}
	}
	return list
}
func (e *EtcdAddrStore) Set(list []string) error {
	e.list = list
	if e.syncFile && e.filePath != "" {
		data, err := json.Marshal(list)
		if err != nil {
			return err
		}
		file, err := os.Create(e.filePath)
		defer file.Close()
		if err != nil {
			return err
		}
		_, err = file.Write(data)
		if err != nil {
			return err
		}
	}
	return nil
}
