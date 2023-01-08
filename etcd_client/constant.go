package etcd_client

import (
	"github.com/google/uuid"
	"os"
)

var (
	EtcdAddr         string
	ServiceId        string
	ServiceName      string
	ServiceNamespace string
	ConfigDir        string
)

func init() {
	ConfigDir = os.Getenv("CONFIG_DIR")
	if ConfigDir == "" {
		ConfigDir = "./config/"
	}
	EtcdAddr = os.Getenv("ETCD_ADDR")
	if EtcdAddr == "" {
		EtcdAddr = "127.0.0.1:2379"
	}
	ServiceId = os.Getenv("SERVICE_ID")
	if ServiceId == "" {
		uid, err := uuid.NewUUID()
		if err != nil {
			panic(err)
		}
		ServiceId = uid.String()
	}
	ServiceName = os.Getenv("SERVICE_NAME")
	ServiceNamespace = os.Getenv("SERVICE_NAMESPACE")
	if ServiceNamespace == "" {
		ServiceNamespace = "center"
	}
}
