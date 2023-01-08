package etcd_client

import (
	"fmt"
)

// SyncConfigs 同步配置.
func (r *Center) SyncConfigs(stores ...IStore) error {
	for _, s := range stores {
		if remote := s.Remote(); remote != nil {
			key := fmt.Sprintf("%s/%s/%s", r.opts.namespace, "config", remote.Path)
			err := r.requestKV(key, s)
			if err != nil {
				return err
			}
			if remote.RequireWatch {
				e := r.watchKV(key, s)
				if e != nil {
					return e
				}
			}
		}
	}
	return nil
}