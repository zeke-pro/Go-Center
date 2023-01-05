package main

import (
	"center/constant"
	"center/datastore"
	"context"
	"encoding/json"
	"fmt"
	clientv3 "go.etcd.io/etcd/client/v3"
	"time"
)

func marshal(si *datastore.Service) (string, error) {
	data, err := json.Marshal(si)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func unmarshal(data []byte) (si *datastore.Service, err error) {
	err = json.Unmarshal(data, &si)
	return
}

func CreateCurrentServiceFromEnv() *datastore.Service {
	return &datastore.Service{
		Id:        constant.ServiceId,
		Name:      constant.ServiceName,
		Namespace: constant.ServiceNamespace,
	}
}

// 获取
func (r *Center) getServices(ctx context.Context, key string, store datastore.IServiceStore) error {

	resp, err := r.kv.Get(ctx, key, clientv3.WithPrefix())
	if err != nil {
		return err
	}
	items := make([]*datastore.Service, 0)
	for _, kv := range resp.Kvs {
		si, err := unmarshal(kv.Value)
		if err != nil {
			return err
		}
		items = append(items, si)
	}
	store.SetList(items)
	return nil
}

// TakeOverServiceList 接管服务列表.
func (r *Center) TakeOverServiceList(store datastore.IServiceStore) error {
	key := fmt.Sprintf("%s/%s/%s", r.opts.self.Namespace, "service", store.GetName())
	ctx, _ := context.WithTimeout(r.opts.ctx, time.Second*5)
	err := r.getServices(ctx, key, store)
	if err != nil {
		return err
	}
	//监听变化
	lCtx := r.opts.ctx
	watcher := clientv3.NewWatcher(r.Client)
	ch := watcher.Watch(lCtx, key, clientv3.WithPrefix(), clientv3.WithRev(0), clientv3.WithKeysOnly())
	err = watcher.RequestProgress(context.Background())
	if err != nil {
		return err
	}
	go func() {
		for {
			select {
			case <-lCtx.Done():
				watcher.Close()
				return
			case <-ch:
				//刚启动，没有变化也会走到这里来
				ctx1, _ := context.WithTimeout(r.opts.ctx, time.Second*5)
				r.getServices(ctx1, key, store)
			}
		}
	}()
	return nil
}
