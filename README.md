# 说明
## 环境变量

| 名称                 | 作用              | 默认值        |
|--------------------|-----------------|------------|
| CONFIG_DIR         | 配置文件路径          | ./config   |
| CENTER_ADDR        | 中心地址(带端口号)      | 127.0.0.1:2379 |
| SERVICE_ID         | 唯一标识            | uuid       |
| SERVICE_NAME       | * 服务名称          |            |
| SERVICE_NAMESPACE  | 命名空间            | center     |
| IS_SSL             | 连接ETCD 是否启用TLS 证书 | false      |
| CERT_DIR           | 证书存储目录          | cert/      |
| CERT_KEY_FILE      | 证书存储目录          | client.key |
| CERT_FILE          | 证书存储目录          | client.crt |
| CERT_CA_FILE       | CA证书文件名称        | ca.crt     |