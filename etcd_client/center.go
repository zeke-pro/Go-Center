package etcd_client

import (
	"context"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"time"
)

// Option  扩展属性参数
type Option func(o *options)

type options struct {
	self             *Service
	ttl              time.Duration
	maxRetry         int    // 最大重试次数
	addr             string //暂时不做冗余，只用一个地址
	registrarTimeout time.Duration
	namespace        string
}

// Namespace with center namespace
func Namespace(namespace string) Option {
	return func(o *options) { o.namespace = namespace }
}

func CurrentService(service *Service) Option {
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
		addr:             EtcdAddr,
		registrarTimeout: time.Second * 5,
		namespace:        ServiceNamespace,
	}
	for _, o := range opts {
		o(op)
	}
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{op.addr},
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

func (r *Center) SetSelf(service *Service) {
	r.opts.self = service
}

func (r *Center) GetEtcdClient() *clientv3.Client {
	return r.client
}

// 监听
func (r *Center) watchKV(key string, store IStore) error {
	args := []clientv3.OpOption{
		clientv3.WithRev(0),
		clientv3.WithKeysOnly(),
	}
	withPrefix := store.Remote().Prefix
	if withPrefix {
		args = append(args, clientv3.WithPrefix())
	}
	watcher := clientv3.NewWatcher(r.client)
	ch := watcher.Watch(r.ctx, key, args...)
	err := watcher.RequestProgress(context.Background())
	if err != nil {
		return err
	}
	go func() {
		for {
			select {
			case <-r.ctx.Done():
				return
			case <-ch:
				//这里要第一次不执行
				//................
				r.requestKV(key, store)
			}
		}
	}()
	return nil
}

// 获取
func (r *Center) requestKV(key string, store IStore) error {
	ctx, cancel := context.WithTimeout(r.ctx, time.Second*5)
	defer cancel()
	args := make([]clientv3.OpOption, 0, 1)
	withPrefix := store.Remote().Prefix
	prefix := ""
	if withPrefix {
		args = append(args, clientv3.WithPrefix())
		prefix = key + "/"
	}
	res, err := r.kv.Get(ctx, key, args...)
	if err != nil {
		return err
	}
	kv := convertKv(res.Kvs, prefix)
	if len(kv) > 0 {
		return store.Parse(kv)
	}
	return nil

}

func convertKv(kvs []*mvccpb.KeyValue, prefix string) []*RemoteData {
	r := make([]*RemoteData, len(kvs), len(kvs))
	for i, vv := range kvs {
		key := string(vv.Key)
		key = key[len(prefix):]
		value := vv.Value
		r[i] = &RemoteData{key, value}
	}
	return r
}
