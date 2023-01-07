package store

import (
	"center/constant"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Endpoint struct {
	Scheme  string `json:"scheme"`
	Address string `json:"address"`
	Port    int    `json:"port"`
	Tag     string `json:"name"`
}

type Service struct {
	Id        string                 `json:"id"`
	Name      string                 `json:"name"`
	MateData  map[string]interface{} `json:"mateData"`
	Endpoints []*Endpoint            `json:"endpoints"`
}

type ServiceStore struct {
	serviceName string
	filePath    string //文件路径
	syncFile    bool   //是否将同步到本地文件
	list        []*Service
}

func NewDefaultServiceStore(name string) *ServiceStore {
	dir := constant.ConfigDir
	fp := filepath.Join(dir, fmt.Sprintf("service_%s.json", name))
	var list []*Service
	if data, err := os.ReadFile(fp); err == nil {
		json.Unmarshal(data, &list)
	}
	return &ServiceStore{name, fp, true, list}
}

func (s *ServiceStore) GetName() string {
	return s.serviceName
}

func (s *ServiceStore) GetList() []*Service {
	return s.list
}

func (s *ServiceStore) SetList(list []*Service) error {
	s.list = list
	if s.syncFile && s.filePath != "" {
		data, err := json.Marshal(list)
		if err != nil {
			return err
		}
		file, err := os.Create(s.filePath)
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
