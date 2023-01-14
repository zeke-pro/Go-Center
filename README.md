# 说明
## ETCD key路径规划
| 类型     | 一层路径  | 二层路径 | 三层路径 | 四层路径         |
| -------- | --------- | -------- | -------- |--------------|
| 服务 | 命名空间(统一为center) | service | store名称 | id,一般用UUID生成 | 
| 配置 | 命名空间(统一为center) | config |  store名称 | key名称        |

```
# 配置的key名称
center/config/myconfig/put_config.json	

# 服务的key名称
center/service/test_service/ce302a06-90e2-11ed-8cdb-8656d13e4381
```

## 环境变量
| 名称                | 作用           | 默认值            |
|-------------------|--------------|----------------|
| CONFIG_DIR        | 配置文件路径       | ./config       |
| ETCD_ADDR         | ETCD地址(带端口号) |127.0.0.1:2379|
| SERVICE_ID        | 唯一标识         | uuid           |
| SERVICE_NAME      | 服务名称       |                |
| SERVICE_NAMESPACE | 命名空间         | center         |
| IS_SSL             | 连接ETCD 是否启用TLS 证书 | false      |
| CERT_DIR           | 证书存储目录          | cert/      |
| CERT_KEY_FILE      | 证书存储目录          | client.key |
| CERT_FILE          | 证书存储目录          | client.crt |
| CERT_CA_FILE       | CA证书文件名称        | ca.crt     |


## 证书生成和运行配置
修改 script/pki.sh  机器IP地址清单
```bash 
# 生成证书,讲证书复制到certs/目录下
./pki.sh
```

etcd.yml配置文件
```yaml
name: etcd01
data-dir: data/etcd/default.etcd
listen-peer-urls: https://192.168.31.17:2380
listen-client-urls: https://192.168.31.17:2379,https://127.0.0.1:2379
initial-advertise-peer-urls: https://192.168.31.17:2380
advertise-client-url: https://192.168.31.17:2379,https://127.0.0.1:2379
initial-cluster: etcd01=https://192.168.31.17:2380
initial-cluster-token: etcd-cluster
initial-cluster-state: new
client-transport-security: 
  cert-file:  certs/client.crt
  key-file:   certs/client.key
  client-cert-auth: false
  trusted-ca-file: certs/ca.crt

peer-transport-security:
  cert-file: certs/peer.crt
  key-file:  certs/peer.key
  client-cert-auth: false
  trusted-ca-file: certs/ca.crt
  auto-tls: false
```

运行etcd
```
etcd  --config-file etcd.yml
```

center程序配置运行，添加环境变量```IS_SSL=true ``` ，center程序运行时证书相关变量如表格所示

