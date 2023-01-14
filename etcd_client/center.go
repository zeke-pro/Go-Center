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
	"path"
	"time"
)

// Option  扩展属性参数
type Option func(o *options)

type options struct {
	ttl              time.Duration
	maxRetry         int    // 最大重试次数
	addr             string //暂时不做冗余，只用一个地址
	registrarTimeout time.Duration
}

// RegisterTTL with register ttl.
func RegisterTTL(ttl time.Duration) Option {
	return func(o *options) { o.ttl = ttl }
}

func MaxRetry(num int) Option {
	return func(o *options) { o.maxRetry = num }
}

type Center struct {
	opts      *options
	regKey    string
	regCancel func()
	leaseId   clientv3.LeaseID // 服务注册的租约Id
	client    *clientv3.Client
}

func NewEtcdClientConfig() clientv3.Config {
	if !IsSSL {
		return clientv3.Config{
			Endpoints:   []string{ETCDAddr},
			DialTimeout: time.Second, DialOptions: []grpc.DialOption{grpc.WithBlock()},
		}
	} else {
		// 加载客户端证书
		cert, err := tls.LoadX509KeyPair(path.Join(CertDir, CertFile), path.Join(CertDir, CertKeyFile))
		if err != nil {
			panic(err)
		}

		// 加载 CA 证书
		caData, err := os.ReadFile(path.Join(CertDir, CertCAFile))
		if err != nil {
			panic(err)
		}

		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM(caData)

		_tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			RootCAs:      pool,
		}

		return clientv3.Config{
			Endpoints:   []string{ETCDAddr},
			DialTimeout: time.Second, DialOptions: []grpc.DialOption{grpc.WithBlock()},
			TLS: _tlsConfig,
		}
	}
}

func NewCenter(opts ...Option) (*Center, error) {
	op := &options{
		ttl:              time.Second * 15,
		maxRetry:         5,
		addr:             EtcdAddr,
		registrarTimeout: time.Second * 5,
	}
	for _, o := range opts {
		o(op)
	}
	client, err := clientv3.New(NewEtcdClientConfig())
	if err != nil {
		return nil, err
	}
	return &Center{
		opts:   op,
		client: client,
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
