package etcd_client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"log"
	"os"
	"time"
)

type Center struct {
	ttl         int64 //服务注册keepalive断开删除的ttl
	client      *clientv3.Client
	registerMap map[*Service]*register
	maxRetry    int
}

type Endpoint struct {
	Scheme string `json:"scheme"`
	Host   string `json:"host"`
	Port   int    `json:"port"`
	Tag    string `json:"tag"`
}

func (e *Endpoint) GetUrl() string {
	return fmt.Sprintf("%s://%s:%d", e.Scheme, e.Host, e.Port)
}

type Service struct {
	Id        string      `json:"id"`
	Version   string      `json:"version"`
	Name      string      `json:"name"`
	Endpoints []*Endpoint `json:"endpoints"`
}

func NewCurrentService() *Service {
	return &Service{
		Id:   envConfInstance.ServiceId,
		Name: envConfInstance.ServiceName,
	}
}

type register struct {
	key     string
	value   string
	cancel  func()
	leaseId clientv3.LeaseID
}

type CenterConfig struct {
	RegisterTTL   int64       //服务注册keepalive断开删除的ttl
	EtcdAddr      []string    //etcd地址
	EtcdTlsConfig *tls.Config //etcd tls配置
	MaxRetry      int         //注册最大重试次数
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
		RegisterTTL:   3,
		EtcdAddr:      []string{envConfInstance.EtcdAddr},
		EtcdTlsConfig: tlsConfig,
		MaxRetry:      10,
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
		client:      client,
		ttl:         config.RegisterTTL,
		maxRetry:    config.MaxRetry,
		registerMap: make(map[*Service]*register),
	}, nil
}

func (c *Center) Close() error {
	for reg := range c.registerMap {
		err := c.Deregister(reg)
		if err != nil {
			return err
		}
	}
	return c.client.Close()
}

// Deregister the registration.
func (c *Center) Deregister(service *Service) error {
	r, has := c.registerMap[service]
	if !has {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()
	_, err := c.client.Delete(ctx, r.key)
	if err != nil {
		return err
	}
	if r.cancel != nil {
		r.cancel()
	}
	_, err = c.client.Revoke(ctx, r.leaseId)
	if err != nil {
		return err
	}
	delete(c.registerMap, service)
	return nil
}

// Register 服务注册
func (c *Center) Register(service *Service) error {
	if service == nil {
		return errors.New("no self service defined")
	}
	if service.Name == "" {
		return errors.New("current service name is not defined")
	}
	if service.Id == "" {
		return errors.New("current service id is not defined")
	}
	err := c.Deregister(service)
	if err != nil {
		return err
	}
	r := &register{}
	c.registerMap[service] = r
	//服务kv
	r.key = fmt.Sprintf("%s/%s/%s/%s", envConfInstance.ServiceNamespace, "service", service.Name, service.Id)
	data, err := json.Marshal(service)
	if err != nil {
		return err
	}
	r.value = string(data)
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	go func() {
		n := 0
		for {
			if n > c.maxRetry {
				fmt.Println("重试次数超过上限，注册失败")
				c.Deregister(service)
				return
			}
			n++
			leaseId, err := c.registerKV(ctx, r)
			if err != nil {
				fmt.Printf("第%d次注册失败，原因:%s\n", n, err)
				time.Sleep(time.Second * 3)
				continue
			}
			r.leaseId = leaseId
			alive, err := c.client.KeepAlive(ctx, leaseId)
			if err != nil {
				fmt.Printf("第%d次注册时，创建keepalive失败，原因:%s\n", n, err)
				time.Sleep(time.Second * 3)
				continue
			}
			fmt.Println("服务注册成功")
			n = 0
			stopped := c.keep(ctx, alive)
			if stopped {
				fmt.Printf("服务被注销\n")
				return
			}
			fmt.Printf("keepAlive连接断开，重新注册\n")
			continue
		}
	}()
	return nil
}

func (c *Center) keep(ctx context.Context, ch <-chan *clientv3.LeaseKeepAliveResponse) bool {
	for {
		select {
		case <-ctx.Done():
			return true
		case _, ok := <-ch:
			if !ok {
				if ctx.Err() != nil {
					return true
				}
				return false
			}
		}
	}
}

func (c *Center) registerKV(parentCtx context.Context, reg *register) (clientv3.LeaseID, error) {
	ctx, cancel := context.WithTimeout(parentCtx, time.Second*3)
	defer cancel()
	leaseResp, err := c.client.Grant(ctx, c.ttl)
	if err != nil {
		return 0, err
	}
	_, err = c.client.Put(ctx, reg.key, reg.value, clientv3.WithLease(leaseResp.ID))
	if err != nil {
		c.client.Revoke(ctx, leaseResp.ID)
		return 0, err
	}
	return leaseResp.ID, nil
}

func (c *Center) GetEtcdClient() *clientv3.Client {
	return c.client
}

// 监听
func (c *Center) watchKV(store IStore) {
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
	ch := c.client.Watch(c.client.Ctx(), key, args...)
	go func() {
		for wresp := range ch {
			version := wresp.Header.Revision
			if version != store.GetVersion() {
				err := c.requestKV(store)
				if err != nil {
					log.Printf("the request failed after listening to the data,current key is %s \n", key)
				}
			}
		}
	}()
}

// 获取
func (c *Center) requestKV(store IStore) error {
	key := store.Remote().Key
	ctx, cancel := context.WithTimeout(c.client.Ctx(), time.Second*5)
	defer cancel()
	args := make([]clientv3.OpOption, 0, 1)
	withPrefix := store.Remote().Prefix
	if withPrefix {
		args = append(args, clientv3.WithPrefix())
	}
	res, err := c.client.Get(ctx, key, args...)
	if err != nil {
		return err
	}
	return store.ReceiveData(res.Kvs, res.Header.Revision)
}

func (c *Center) SetStores(stores ...IStore) error {
	for _, s := range stores {
		if remote := s.Remote(); remote != nil {
			err := c.requestKV(s)
			if err != nil {
				fmt.Println(err)
			}
			if remote.RequireWatch {
				c.watchKV(s)
			}
		}
	}
	return nil
}
