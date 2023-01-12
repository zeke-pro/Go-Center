package etcd_client

import (
	"fmt"
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

func CreateCurrentServiceFromEnv() *Service {
	return &Service{
		Id:   ServiceId,
		Name: ServiceName,
	}
}

// DiscoverServices 发现服务.
func (r *Center) DiscoverServices(stores ...IStore) error {
	for _, s := range stores {
		if remote := s.Remote(); remote != nil {
			key := fmt.Sprintf("%s/%s/%s", r.opts.namespace, "service", remote.Path)
			err := r.requestKV(key, s)
			if err != nil {
				fmt.Println(err)
			}
			if remote.RequireWatch {
				e := r.watchKV(key, s)
				if e != nil {
					fmt.Println(err)
				}
			}

		}
	}
	return nil
}
