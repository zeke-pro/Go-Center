package center

import (
	"center/constant"
	"center/store"
	"context"
	"errors"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"time"
)

// Option  扩展属性参数
type Option func(o *options)

type options struct {
	self             *store.Service
	ttl              time.Duration
	maxRetry         int // 最大重试次数
	addrStore        store.IEtcdAddrStore
	registrarTimeout time.Duration
	namespace        string
}

// AddressStore with center address store
func AddressStore(store store.IEtcdAddrStore) Option {
	return func(o *options) { o.addrStore = store }
}

// Namespace with center namespace
func Namespace(namespace string) Option {
	return func(o *options) { o.namespace = namespace }
}

func CurrentService(service *store.Service) Option {
	return func(o *options) { o.self = service }
}

// RegisterTTL with register ttl.
func RegisterTTL(ttl time.Duration) Option {
	return func(o *options) { o.ttl = ttl }
}

func MaxRetry(num int) Option {
	return func(o *options) { o.maxRetry = num }
}

type Center struct {
	opts   *options
	ctx    context.Context
	cancel func()
	client *clientv3.Client
	kv     clientv3.KV
	lease  clientv3.Lease // 租约
}

func NewCenter(opts ...Option) (*Center, error) {
	op := &options{
		self:             nil,
		ttl:              time.Second * 15,
		maxRetry:         5,
		addrStore:        nil,
		registrarTimeout: time.Second * 5,
		namespace:        constant.ServiceNamespace,
	}
	for _, o := range opts {
		o(op)
	}
	if op.addrStore == nil {
		store := store.NewDefaultEtcdAddrStore()
		if len(store.Get()) <= 0 {
			return nil, errors.New("center address is not define")
		}
		op.addrStore = store
	}
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   op.addrStore.Get(),
		DialTimeout: time.Second, DialOptions: []grpc.DialOption{grpc.WithBlock()},
	})
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Center{
		opts:   op,
		client: client,
		kv:     clientv3.NewKV(client),
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

func (r *Center) Close() error {
	r.Deregister()
	r.client.Close()
	r.cancel()
	return nil
}

func (r *Center) SetSelf(service *store.Service) {
	r.opts.self = service
}
func (r *Center) GetEtcdClient() *clientv3.Client {
	return r.client
}
