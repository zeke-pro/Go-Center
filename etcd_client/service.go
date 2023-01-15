package etcd_client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	clientv3 "go.etcd.io/etcd/client/v3"
	"math/rand"
	"time"
)

type Endpoint struct {
	Scheme  string `json:"scheme"`
	Address string `json:"address"`
	Port    int    `json:"port"`
	Tag     string `json:"tag"`
}

type Service struct {
	Id        string               `json:"id"`
	Name      string               `json:"name"`
	Endpoints map[string]*Endpoint `json:"endpoints"`
}

func NewCurrentService() *Service {
	return &Service{
		Id:   envConfInstance.ServiceId,
		Name: envConfInstance.ServiceName,
	}
}

// Register 服务注册
func (r *Center) Register(service *Service) error {
	r.Deregister()
	if service == nil {
		return errors.New("no self service defined")
	}
	if service.Name == "" {
		return errors.New("current service name is not defined")
	}
	if service.Id == "" {
		return errors.New("current service id is not defined")
	}
	//服务kv
	key := fmt.Sprintf("%s/%s/%s/%s", envConfInstance.ServiceNamespace, "service", service.Name, service.Id)
	data, err := json.Marshal(service)
	if err != nil {
		return err
	}
	value := string(data)

	ctx, cancel := context.WithTimeout(r.client.Ctx(), time.Second*3)
	defer cancel()

	// 写入etcd
	leaseId, err := r.registerWithKV(ctx, key, value)
	if err != nil {
		return err
	}

	regCtx, regCancel := context.WithCancel(r.client.Ctx())

	r.leaseId = leaseId
	r.regCancel = regCancel
	r.regKey = key
	go r.heartBeat(regCtx, leaseId, key, value)
	return nil
}

// registerWithKV create a new lease, return current leaseID
func (r *Center) registerWithKV(ctx context.Context, key string, value string) (clientv3.LeaseID, error) {
	grant, err := r.client.Grant(ctx, int64(time.Second*3))
	if err != nil {
		return 0, err
	}
	_, err = r.client.Put(ctx, key, value, clientv3.WithLease(grant.ID))
	if err != nil {
		return 0, err
	}
	return grant.ID, nil
}

func (r *Center) heartBeat(ctx context.Context, leaseID clientv3.LeaseID, key string, value string) {
	curLeaseID := leaseID
	kac, err := r.client.KeepAlive(ctx, leaseID)
	if err != nil {
		curLeaseID = 0
	}
	rand.Seed(time.Now().Unix())

	for {
		if curLeaseID == 0 {
			for {
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
					r.leaseId = id
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

				//keepalive创建成功，跳出当前循环
				kac, err = r.client.KeepAlive(ctx, curLeaseID)
				if err == nil {
					break
				}
				time.Sleep(10 * time.Second)
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
		case <-ctx.Done():
			return
		case <-r.client.Ctx().Done():
			return
		}
	}
}

// Deregister the registration.
func (r *Center) Deregister() error {
	if r.regCancel != nil {
		r.regCancel()
	}
	ctx, cancel := context.WithTimeout(r.client.Ctx(), time.Second*3)
	defer cancel()
	if r.regKey != "" {
		_, err := r.client.Delete(ctx, r.regKey)
		if err != nil {
			return err
		}
		r.regKey = ""
	}
	if r.leaseId != 0 {
		_, err := r.client.Revoke(ctx, r.leaseId)
		if err != nil {
			return err
		}
		r.leaseId = 0
	}
	return nil
}
