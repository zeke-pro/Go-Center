package datastore

import (
	"center/constant"
	"encoding/json"
	"fmt"
	"os"
	"path"
)

type IConfigStore interface {
	Get() any
	Unmarshal(data []byte) error
	GetName() string
	GetRemotePath() string
	OnChange(func(newValue any))
}

type ConfigStore struct {
	data       any
	name       string
	remotePath string
	filePath   string
	syncFile   bool
	onChange   func(newValue any)
}

func NewConfigStore(name string, data any, remotePath string) *ConfigStore {
	filePath := path.Join(constant.ConfigDir, fmt.Sprintf("config_%s.json", name))
	return &ConfigStore{
		data, name, remotePath, filePath, true, nil,
	}
}

func (c *ConfigStore) Get() any {
	return c.data
}

func (c *ConfigStore) GetName() string {
	return c.name
}

func (c *ConfigStore) GetRemotePath() string {
	return c.remotePath
}

func (c *ConfigStore) Unmarshal(data []byte) error {

	err := json.Unmarshal(data, c.data)
	if err != nil {
		return err
	}
	if c.syncFile && c.filePath != "" {
		file, err := os.Create(c.filePath)
		defer file.Close()
		if err != nil {
			return err
		}
		_, err = file.Write(data)
		if err != nil {
			return err
		}
	}
	if c.onChange != nil {
		c.onChange(c.data)
	}
	return nil
}

func (c *ConfigStore) OnChange(onChange func(newValue any)) {
	c.onChange = onChange
}
