package etcd_client

import (
	"github.com/google/uuid"
	"os"
	"path"
)

type envConf struct {
	ConfigDir        string
	EtcdAddr         string
	ServiceId        string
	ServiceName      string
	ServiceNamespace string
	EtcdSslEnable    bool
	EtcdCertDir      string
	EtcdCertPath     string
	EtcdPriPath      string
	EtcdCaPath       string
}

var envConfInstance *envConf

func init() {
	e := envConf{}
	e.ConfigDir = os.Getenv("CONFIG_DIR")
	if e.ConfigDir == "" {
		e.ConfigDir = "./config/"
	}
	if _, err := os.Stat(e.ConfigDir); os.IsNotExist(err) {
		err = os.Mkdir(e.ConfigDir, os.ModePerm)
		if err != nil {
			panic(err)
		}
	}

	e.EtcdAddr = os.Getenv("ETCD_ADDR")
	if e.EtcdAddr == "" {
		e.EtcdAddr = "127.0.0.1:2379"
	}
	e.ServiceId = os.Getenv("SERVICE_ID")
	if e.ServiceId == "" {
		uid, err := uuid.NewUUID()
		if err != nil {
			panic(err)
		}
		e.ServiceId = uid.String()
	}
	e.ServiceName = os.Getenv("SERVICE_NAME")
	e.ServiceNamespace = os.Getenv("SERVICE_NAMESPACE")
	if e.ServiceNamespace == "" {
		e.ServiceNamespace = "center"
	}
	e.EtcdSslEnable = os.Getenv("ETCD_SSL_ENABLE") == "true"

	if e.EtcdSslEnable {
		e.EtcdCertDir = os.Getenv("ETCD_CERT_DIR")
		if e.EtcdCertDir == "" {
			e.EtcdCertDir = path.Join(e.ConfigDir, "etcd_ssl")
		}
		e.EtcdCertPath = os.Getenv("ETCD_CERT_PATH")
		if e.EtcdCertPath == "" {
			e.EtcdCertPath = path.Join(e.EtcdCertDir, "cert.crt")
		}
		e.EtcdPriPath = os.Getenv("ETCD_PRI_PATH")
		if e.EtcdPriPath == "" {
			e.EtcdPriPath = path.Join(e.EtcdCertDir, "cert.key")
		}
		e.EtcdCaPath = os.Getenv("CERT_CA_FILE")
		if e.EtcdCaPath == "" {
			e.EtcdCaPath = path.Join(e.EtcdCertDir, "ca.crt")
		}
	}
	envConfInstance = &e
}
