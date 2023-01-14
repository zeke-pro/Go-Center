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
	ETCDAddr         string
	ConfigDir        string
	IsSSL            bool
	CertDir          string
	CertKeyFile      string
	CertFile         string
	CertCAFile       string
)

func init() {
	ETCDAddr = os.Getenv("ETCD_ADDR")
	if ETCDAddr == "" {
		ETCDAddr = "127.0.0.1:2379"
	}

	ConfigDir = os.Getenv("CONFIG_DIR")
	if ConfigDir == "" {
		ConfigDir = "./config/"
	}
	if _, err := os.Stat(ConfigDir); os.IsNotExist(err) {
		err = os.Mkdir(ConfigDir, os.ModePerm)
		if err != nil {
			panic(err)
		}
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
	IsSSL = os.Getenv("IS_SSL") == "true"

	if IsSSL {
		CertDir = os.Getenv("CERT_DIR")
		if CertDir == "" {
			CertDir = "./cert/"
		}
		CertKeyFile = os.Getenv("CERT_KEY_FILE")
		if CertKeyFile == "" {
			CertKeyFile = "client.key"
		}
		CertFile = os.Getenv("CERT_FILE")
		if CertFile == "" {
			CertFile = "client.crt"
		}
		CertCAFile = os.Getenv("CERT_CA_FILE")
		if CertCAFile == "" {
			CertCAFile = "ca.crt"
		}
	}
}
