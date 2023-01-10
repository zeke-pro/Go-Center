package etcd_client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	clientv3 "go.etcd.io/etcd/client/v3"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Register 服务注册
// 原理:
//
//	创建一个租约： clientv3.NewLease，
//	然后创建key写入值： r.registerWithKV(ctx, key, value)，
//	同时监听心跳： r.heartBeat(r.opts.ctx, leaseID, key, value)
func (r *Center) Register() error {
	self := r.opts.self
	if self == nil {
		return errors.New("no self service defined")
	}
	if self.Name == "" {
		return errors.New("current service name is not defined")
	}
	if self.Id == "" {
		return errors.New("current service id is not defined")
	}
	ctx, cancel := context.WithTimeout(r.ctx, r.opts.registrarTimeout)
	defer cancel()
	key := fmt.Sprintf("%s/%s/%s/%s", r.opts.namespace, "service", r.opts.self.Name, r.opts.self.Id)
	data, err := json.Marshal(r.opts.self)
	value := string(data)
	if err != nil {
		return err
	}
	if r.lease != nil {
		r.lease.Close()
	}
	r.lease = clientv3.NewLease(r.client)
	leaseID, err := r.registerWithKV(ctx, key, value)
	if err != nil {
		return err
	}

	go r.GraceShutdown()
	go r.heartBeat(r.ctx, leaseID, key, value)
	return nil
}

// GraceShutdown 程序正常退出，注销服务
func (r *Center) GraceShutdown() {
	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		select {
		case sig := <-c:
			{
				log.Printf("receive signal %s, deregister service %s", sig.String(), r.opts.self.Name)
				r.Deregister()
				os.Exit(1)
			}
		}
	}()
}

// Deregister the registration.
func (r *Center) Deregister() error {
	ctx, cancel := context.WithTimeout(r.ctx, r.opts.registrarTimeout)
	defer cancel()
	defer func() {
		if r.lease != nil {
			r.lease.Close()
		}
	}()
	key := fmt.Sprintf("%s/%s/%s/%s", r.opts.namespace, "service", r.opts.self.Name, r.opts.self.Id)
	_, err := r.client.Delete(ctx, key)
	return err
}

// registerWithKV 创建新租约，返回租约ID
// 原理:
//
//	lease.Grant(ctx, int64(r.opts.ttl.Seconds()))  设置租约过期时间为ttl
//	注意: 续租时间大概自动为租约的三分之一时间(官方)
//	client.Put(ctx, key, value, clientv3.WithLease(grant.ID))   创建key写入值
func (r *Center) registerWithKV(ctx context.Context, key string, value string) (clientv3.LeaseID, error) {
	grant, err := r.lease.Grant(ctx, int64(r.opts.ttl.Seconds()))
	if err != nil {
		return 0, err
	}
	_, err = r.client.Put(ctx, key, value, clientv3.WithLease(grant.ID))
	if err != nil {
		return 0, err
	}
	return grant.ID, nil
}

// heartBeat 监听心跳  也可以理解为租约的业务实现
func (r *Center) heartBeat(ctx context.Context, leaseID clientv3.LeaseID, key string, value string) {
	curLeaseID := leaseID
	kac, err := r.client.KeepAlive(ctx, leaseID)
	if err != nil {
		curLeaseID = 0
	}
	rand.Seed(time.Now().Unix())

	for {
		if curLeaseID == 0 {
			// try to registerWithKV
			var retreat []int
			for retryCnt := 0; retryCnt < r.opts.maxRetry; retryCnt++ {
				if ctx.Err() != nil {
					return
				}
				// prevent infinite blocking
				idChan := make(chan clientv3.LeaseID, 1)
				errChan := make(chan error, 1)
				cancelCtx, cancel := context.WithCancel(ctx)
				go func() {
					defer cancel()
					id, registerErr := r.registerWithKV(cancelCtx, key, value)
					if registerErr != nil {
						errChan <- registerErr
					} else {
						idChan <- id
					}
				}()

				select {
				case <-time.After(3 * time.Second):
					cancel()
					continue
				case <-errChan:
					continue
				case curLeaseID = <-idChan:
				}

				kac, err = r.client.KeepAlive(ctx, curLeaseID)
				if err == nil {
					break
				}
				retreat = append(retreat, 1<<retryCnt)
				time.Sleep(time.Duration(retreat[rand.Intn(len(retreat))]) * time.Second)
			}
			if _, ok := <-kac; !ok {
				// retry failed
				return
			}
		}

		select {
		case _, ok := <-kac:
			if !ok {
				if ctx.Err() != nil {
					// channel closed due to context cancel
					return
				}
				// need to retry registration
				curLeaseID = 0
				continue
			}
		case <-r.ctx.Done():
			return
		}
	}
}
