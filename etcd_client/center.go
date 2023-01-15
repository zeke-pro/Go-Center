package etcd_client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"log"
	"os"
	"time"
)

type Center struct {
	ttl       time.Duration //服务注册keepalive断开删除的ttl
	client    *clientv3.Client
	regKey    string
	regCancel func()
	leaseId   clientv3.LeaseID
}

type CenterConfig struct {
	RegisterTTL   time.Duration //服务注册keepalive断开删除的ttl
	EtcdAddr      []string      //etcd地址
	EtcdTlsConfig *tls.Config   //etcd tls配置
}

func NewDefaultCenterConfig() (*CenterConfig, error) {
	var tlsConfig *tls.Config
	if envConfInstance.EtcdSslEnable {
		cert, err := tls.LoadX509KeyPair(envConfInstance.EtcdCertPath, envConfInstance.EtcdPriPath)
		if err != nil {
			return nil, err
		}

		// 加载 CA 证书
		caData, err := os.ReadFile(envConfInstance.EtcdCaPath)
		if err != nil {
			return nil, err
		}
		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM(caData)
		tlsConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
			RootCAs:      pool,
		}
	}
	return &CenterConfig{
		RegisterTTL:   time.Second * 15,
		EtcdAddr:      []string{envConfInstance.EtcdAddr},
		EtcdTlsConfig: tlsConfig,
	}, nil
}

func NewCenter(config *CenterConfig) (*Center, error) {
	clientCfg := clientv3.Config{
		Endpoints:   config.EtcdAddr,
		DialTimeout: time.Second, DialOptions: []grpc.DialOption{grpc.WithBlock()},
		TLS: config.EtcdTlsConfig,
	}
	client, err := clientv3.New(clientCfg)
	if err != nil {
		return nil, err
	}
	return &Center{
		client: client,
		ttl:    config.RegisterTTL,
	}, nil
}

func (r *Center) Close() error {
	err := r.Deregister()
	if err != nil {
		return err
	}
	err = r.client.Close()
	if err != nil {
		return err
	}
	return nil
}

func (r *Center) GetEtcdClient() *clientv3.Client {
	return r.client
}

// 监听
func (r *Center) watchKV(store IStore) {
	key := store.Remote().Key
	args := []clientv3.OpOption{
		clientv3.WithRev(0),
		clientv3.WithKeysOnly(),
	}
	remoteConfig := store.Remote()
	withPrefix := remoteConfig.Prefix
	if withPrefix {
		args = append(args, clientv3.WithPrefix())
	}
	ch := r.client.Watch(r.client.Ctx(), key, args...)
	go func() {
		for wresp := range ch {
			version := wresp.Header.Revision
			if version != store.GetVersion() {
				err := r.requestKV(store)
				if err != nil {
					log.Printf("the request failed after listening to the data,current key is %s \n", key)
				}
			}
			//for _, ev := range wresp.Events {
			//	switch ev.Type {
			//	case mvccpb.PUT:
			//		fmt.Printf("%s %q : %q isModify:%s \n", ev.Type, ev.Kv.Key, ev.Kv.Value, ev.IsModify())
			//	case mvccpb.DELETE:
			//	}
			//}
		}
	}()
}

// 获取
func (r *Center) requestKV(store IStore) error {
	key := store.Remote().Key
	ctx, cancel := context.WithTimeout(r.client.Ctx(), time.Second*5)
	defer cancel()
	args := make([]clientv3.OpOption, 0, 1)
	withPrefix := store.Remote().Prefix
	if withPrefix {
		args = append(args, clientv3.WithPrefix())
	}
	res, err := r.client.Get(ctx, key, args...)
	if err != nil {
		return err
	}
	return store.ReceiveData(res.Kvs, res.Header.Revision)
}

func (r *Center) SetStores(stores ...IStore) error {
	for _, s := range stores {
		if remote := s.Remote(); remote != nil {
			err := r.requestKV(s)
			if err != nil {
				fmt.Println(err)
			}
			if remote.RequireWatch {
				r.watchKV(s)
			}
		}
	}
	return nil
}
