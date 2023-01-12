package etcd_client

import (
	"context"
	"encoding/json"
	"fmt"
)

// SyncConfigs 同步配置
// 根据store.RemoteConfig配置 开启远程同步本地
// 根据store.LocalConfig配置 开启本地同步远程
func (r *Center) SyncConfigs(stores ...IStore) error {
	for _, s := range stores {
		if remote := s.Remote(); remote != nil {
			key := fmt.Sprintf("%s/%s/%s", r.opts.namespace, "config", remote.Path)
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
			//如果需要put到远程
			if remote.RequirePut {
				if remote.SetChan == nil {
					remote.SetChan = make(chan interface{})
				}
				go func() {
					for {
						select {
						case setData := <-remote.SetChan:
							str, err := json.Marshal(setData)
							if err != nil {
								r.GetEtcdClient().Put(context.Background(), key, string(str))
							}
						}
					}
				}()
			}
		}
	}
	return nil
}
