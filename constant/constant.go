package constant

import (
	"github.com/google/uuid"
	"os"
)

var (
	CenterAddr       string
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
	CenterAddr = os.Getenv("CENTER_ADDR")
	if CenterAddr == "" {
		CenterAddr = "127.0.0.1:2379"
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
	if ServiceName == "" {
		panic("service name is not define")
	}
	ServiceNamespace = os.Getenv("SERVICE_NAMESPACE")
	if ServiceNamespace == "" {
		ServiceNamespace = "center"
	}
}
