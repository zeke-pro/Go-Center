package center

import (
	"center/store"
	"context"
	"fmt"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"time"
)

type IConfigStore interface {
	Parse(kvs []*mvccpb.KeyValue) error
	Remote() *store.RemoteConfig
}

// SyncConfigs 同步配置.
func (r *Center) SyncConfigs(store ...IConfigStore) error {
	for _, s := range store {
		if s.Remote() != nil {
			err := r.syncConfig(s)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *Center) syncConfig(store IConfigStore) error {
	remote := store.Remote()
	key := fmt.Sprintf("%s/%s/%s", r.opts.namespace, "config", remote.Path)

	err := r.getConfig(key, store)
	if err != nil {
		return err
	}
	//监听变化
	if remote.RequireWatch {
		watcher := clientv3.NewWatcher(r.client)
		ch := watcher.Watch(r.ctx, key, clientv3.WithRev(0), clientv3.WithKeysOnly())
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
					r.getConfig(key, store)
				}
			}
		}()
	}
	return nil
}

// 获取
func (r *Center) getConfig(key string, store IConfigStore) error {
	ctx, cancel := context.WithTimeout(r.ctx, time.Second*5)
	defer cancel()
	args := make([]clientv3.OpOption, 0, 1)
	if store.Remote().Prefix {
		args = append(args, clientv3.WithPrefix())
	}
	resp, err := r.kv.Get(ctx, key, args...)
	if err != nil {
		return err
	}
	err = store.Parse(resp.Kvs)
	if err != nil {
		return err
	}
	return nil
}
