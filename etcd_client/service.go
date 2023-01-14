package etcd_client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type Endpoint struct {
	Scheme  string `json:"scheme"`
	Address string `json:"address"`
	Port    int    `json:"port"`
	Tag     string `json:"name"`
}

type Service struct {
	Id        string               `json:"id"`
	Name      string               `json:"name"`
	Endpoints map[string]*Endpoint `json:"endpoints"`
}

func NewCurrentService() *Service {
	return &Service{
		Id:   ServiceId,
		Name: ServiceName,
	}
}

// Register 服务注册
func (r *Center) Register(service *Service) error {
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
	key := fmt.Sprintf("%s/%s/%s/%s", ServiceNamespace, "service", service.Name, service.Id)
	data, err := json.Marshal(service)
	if err != nil {
		return err
	}
	value := string(data)

	ctx, cancel := context.WithTimeout(r.client.Ctx(), r.opts.registrarTimeout)
	defer cancel()

	//已注册过
	if r.leaseId != 0 {
		_, err = r.client.Put(ctx, key, value, clientv3.WithLease(r.leaseId))
		if err != nil {
			return err
		}
		r.regKey = key
	}

	// 创建租约
	leaseResp, er := r.client.Grant(ctx, int64(r.opts.ttl.Seconds()))
	if er != nil {
		return er
	}

	leaseId := leaseResp.ID

	// 写入etcd
	_, err = r.client.Put(ctx, key, value, clientv3.WithLease(leaseId))
	if err != nil {
		return err
	}
	regCtx, regCancel := context.WithCancel(r.client.Ctx())
	alive, err := r.client.KeepAlive(regCtx, leaseId)
	go func() {
		for {
			select {
			case <-regCtx.Done():
				return
			case <-r.client.Ctx().Done():
				return
			case _, ok := <-alive:
				if !ok {
					return
				}
			}
		}
	}()

	r.leaseId = leaseId
	r.regCancel = regCancel
	r.regKey = key
	return nil
}

// Deregister the registration.
func (r *Center) Deregister() error {
	ctx, cancel := context.WithTimeout(r.client.Ctx(), r.opts.registrarTimeout)
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
	r.regCancel()
	return nil
}
