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
	"sync"
	"time"
)

type Center struct {
	ttl         int64 //服务注册keepalive断开删除的ttl
	client      *clientv3.Client
	registerMap map[*Service]*register
	maxRetry    int
	cancelLease func()
	reqTimeout  time.Duration
	leaseId     clientv3.LeaseID
	mu          sync.Mutex
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
	Id        string            `json:"id"`
	Version   string            `json:"version"`
	Name      string            `json:"name"`
	Metadata  map[string]string `json:"metadata"`
	Endpoints []*Endpoint       `json:"endpoints"`
}

func NewCurrentService() *Service {
	return &Service{
		Id:   envConfInstance.ServiceId,
		Name: envConfInstance.ServiceName,
	}
}

type register struct {
	key   string
	value string
}

type CenterConfig struct {
	RegisterTTL    int64         //服务注册keepalive断开删除的ttl
	EtcdAddr       []string      //etcd地址
	EtcdTlsConfig  *tls.Config   //etcd tls配置
	MaxRetry       int           //注册最大重试次数
	RequestTimeout time.Duration //每次请求的超时时间
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
		RegisterTTL:    3,
		EtcdAddr:       []string{envConfInstance.EtcdAddr},
		EtcdTlsConfig:  tlsConfig,
		MaxRetry:       10,
		RequestTimeout: time.Second * 3,
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
		reqTimeout:  config.RequestTimeout,
	}, nil
}

func (c *Center) Close() error {
	for k, _ := range c.registerMap {
		err := c.Deregister(k)
		if err != nil {
			return err
		}
	}
	if c.cancelLease != nil {
		c.cancelLease()
	}
	return c.client.Close()
}

func (c *Center) startLease() error {
	if c.leaseId != 0 {
		return errors.New("已存在lease id")
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.cancelLease = cancel
	kaCh, err := c.createLease(ctx)
	if err != nil {
		cancel()
		return err
	}
	go func() {
		for {
			if ctx.Err() != nil {
				return
			}
			//是否主动停止
			stopped := c.keepLease(ctx, kaCh)
			c.mu.Lock()
			c.leaseId = 0
			c.mu.Unlock()
			if stopped {
				fmt.Printf("租约被注销\n")
				return
			} else {
				fmt.Printf("连接断开，开始重新连接\n")
				kaCh, err = c.createLease(ctx)
				if err != nil {
					fmt.Printf("创建lease失败，错误：%s\n", err)
					return
				}
				go c.rePutServices()
				fmt.Println("重新创建lease成功")
			}
		}
	}()
	return nil
}

func (c *Center) createLease(ctx context.Context) (<-chan *clientv3.LeaseKeepAliveResponse, error) {
	if c.leaseId != 0 {
		return nil, errors.New("lease id 已经存在")
	}
	retryNum := 0
	for {
		if ctx.Err() != nil {
			return nil, nil
		}
		if retryNum > c.maxRetry {
			break
		}
		retryNum++
		reqCtx, cancel := context.WithTimeout(ctx, c.reqTimeout)
		leaseResp, err := c.client.Grant(reqCtx, c.ttl)
		if err != nil {
			fmt.Printf("创建lease失败，错误 %s", err)
			cancel()
			continue
		}
		ch, err := c.client.KeepAlive(ctx, leaseResp.ID)
		if err != nil {
			fmt.Printf("创建keepalive失败，错误 %s", err)
			cancel()
			continue
		}
		c.leaseId = leaseResp.ID
		cancel()
		return ch, nil
	}
	return nil, errors.New("重试次数超过上限，创建lease失败")
}

func (c *Center) keepLease(ctx context.Context, ch <-chan *clientv3.LeaseKeepAliveResponse) bool {
	for {
		select {
		case <-ctx.Done():
			return true
		case _, isOpen := <-ch:
			if !isOpen {
				if ctx.Err() != nil {
					return true
				}
				return false
			}
		}
	}
}

// rePutServices 重新put服务发现，通常因为leaseId变化
func (c *Center) rePutServices() {
	if len(c.registerMap) > 0 && c.leaseId != 0 {
		for _, s := range c.registerMap {
			c.client.Put(context.TODO(), s.key, s.value, clientv3.WithLease(c.leaseId))
		}
	}
}

// Deregister the registration.
func (c *Center) Deregister(service *Service) error {
	if service == nil {
		return errors.New("service is not defined")
	}
	r, has := c.registerMap[service]
	if !has {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), c.reqTimeout)
	defer cancel()
	_, err := c.client.Delete(ctx, r.key)
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
	//如果没有租约，先创建租约
	c.mu.Lock()
	if c.leaseId == 0 {
		c.startLease()
	}
	c.mu.Unlock()
	data, err := json.Marshal(service)
	if err != nil {
		return err
	}
	re := &register{
		key:   fmt.Sprintf("%s/%s/%s/%s", envConfInstance.ServiceNamespace, "service", service.Name, service.Id),
		value: string(data),
	}
	if ore, has := c.registerMap[service]; has {
		if ore.key != re.key {
			c.Deregister(service)
		}
	}
	ctx, _ := context.WithTimeout(context.Background(), c.reqTimeout)
	_, err = c.client.Put(ctx, re.key, re.value, clientv3.WithLease(c.leaseId))
	if err != nil {
		return err
	}
	c.registerMap[service] = re
	return nil
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
