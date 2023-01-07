package center

import (
	"center/constant"
	"center/store"
	"context"
	"encoding/json"
	"fmt"
	clientv3 "go.etcd.io/etcd/client/v3"
	"time"
)

func marshal(si *store.Service) (string, error) {
	data, err := json.Marshal(si)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func unmarshal(data []byte) (si *store.Service, err error) {
	err = json.Unmarshal(data, &si)
	return
}

type IServiceStore interface {
	GetName() string
	SetList(list []*store.Service) error
}

func CreateCurrentServiceFromEnv() *store.Service {
	return &store.Service{
		Id:   constant.ServiceId,
		Name: constant.ServiceName,
	}
}

// 获取
func (r *Center) getService(key string, serviceStore IServiceStore) error {
	ctx, cancel := context.WithTimeout(r.ctx, time.Second*5)
	defer cancel()
	resp, err := r.kv.Get(ctx, key, clientv3.WithPrefix())
	if err != nil {
		return err
	}
	items := make([]*store.Service, 0)
	for _, kv := range resp.Kvs {
		si, err := unmarshal(kv.Value)
		if err != nil {
			return err
		}
		items = append(items, si)
	}
	serviceStore.SetList(items)
	return nil
}

func (r *Center) getAndWatchService(serviceStore IServiceStore) error {
	key := fmt.Sprintf("%s/%s/%s", r.opts.namespace, "service", serviceStore.GetName())
	err := r.getService(key, serviceStore)
	if err != nil {
		return err
	}
	//监听变化
	watcher := clientv3.NewWatcher(r.client)
	ch := watcher.Watch(r.ctx, key, clientv3.WithPrefix(), clientv3.WithRev(0), clientv3.WithKeysOnly())
	err = watcher.RequestProgress(context.Background())
	if err != nil {
		return err
	}
	go func() {
		for {
			select {
			case <-r.ctx.Done():
				return
			case <-ch:
				r.getService(key, serviceStore)
			}
		}
	}()
	return nil
}

// DiscoverServices 发现服务.
func (r *Center) DiscoverServices(serviceStore ...IServiceStore) error {
	for _, d := range serviceStore {
		err := r.getAndWatchService(d)
		if err != nil {
			return err
		}
	}
	return nil
}
