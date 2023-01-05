package center

import (
	"center/datastore"
	"context"
	"errors"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"time"
)

// Option  扩展属性参数
type Option func(o *options)

type options struct {
	ctx              context.Context
	self             *datastore.Service
	ttl              time.Duration
	maxRetry         int // 最大重试次数
	addrStore        datastore.IEtcdAddrStore
	registrarTimeout time.Duration
}

// Context with registry context.
func Context(ctx context.Context) Option {
	return func(o *options) { o.ctx = ctx }
}

// AddressStore with center address store
func AddressStore(store datastore.IEtcdAddrStore) Option {
	return func(o *options) { o.addrStore = store }
}

func CurrentService(service *datastore.Service) Option {
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
	Client *clientv3.Client
	kv     clientv3.KV
	lease  clientv3.Lease // 租约
}

func NewCenter(opts ...Option) (*Center, error) {
	op := &options{
		ctx:              context.Background(),
		self:             nil,
		ttl:              time.Second * 15,
		maxRetry:         5,
		addrStore:        nil,
		registrarTimeout: time.Second * 5,
	}
	for _, o := range opts {
		o(op)
	}
	if op.addrStore == nil {
		store := datastore.NewEtcdAddrStoreFromEnv()
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
	return &Center{
		opts:   op,
		Client: client,
		kv:     clientv3.NewKV(client),
	}, nil
}

func (r *Center) Close() error {
	r.Deregister()
	r.Client.Close()
	r.opts.ctx.Done()
	return nil
}
