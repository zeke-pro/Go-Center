package center

import (
	"center/datastore"
	"context"
	"fmt"
	clientv3 "go.etcd.io/etcd/client/v3"
	"time"
)

// TakeOverConfig 接管配置.
func (r *Center) TakeOverConfig(store datastore.IConfigStore) error {
	key := fmt.Sprintf("%s/%s/%s", r.opts.self.Namespace, "config", store.GetRemotePath())
	ctx, _ := context.WithTimeout(r.opts.ctx, time.Second*5)
	err := r.getConfig(ctx, key, store)
	if err != nil {
		return err
	}
	//监听变化
	lCtx := r.opts.ctx
	watcher := clientv3.NewWatcher(r.Client)
	ch := watcher.Watch(lCtx, key, clientv3.WithRev(0), clientv3.WithKeysOnly())
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
				ctx1, _ := context.WithTimeout(r.opts.ctx, time.Second*5)
				r.getConfig(ctx1, key, store)
			}
		}
	}()
	return nil
}

// 获取
func (r *Center) getConfig(ctx context.Context, key string, store datastore.IConfigStore) error {
	resp, err := r.kv.Get(ctx, key, clientv3.WithPrefix())
	if err != nil {
		return err
	}
	if len(resp.Kvs) == 1 {
		store.Unmarshal(resp.Kvs[0].Value)
	}
	return nil
}
