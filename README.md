# YIYA 是一个隧道代理工具，提供安全（TLS）隧道代理功能

一共两种代理模式，分别是 不加密代理隧道，此模式请求明文从代理客户端传输到代理服务端可能会被防火墙拦截、自签名双向证书mTLS加代理密隧道，此模式请求会被加密传输到代理服务端，只要不泄露根证书100%安全，无懈可击

## 安装
```shell
go install github.com/Li-giegie/yiya
```

## 快速开始
### 1. 不加密代理隧道
1. 启动隧道 服务端 默认服务端侦听 ``127.0.0.1:1081``
    ```shell
    go run ./
    ```
2. 启动隧道 客户端 默认侦听 ``127.0.0.1:1080``
    ```shell
    go run ./ -mode client
    ```
### 2. 自签名双向证书mTLS加代理密隧道
1. 启动隧道服务端
    ```shell
    go run ./ -mTLS -rootCertFile .\x509\m
    TLS\ca.crt -certFile .\x509\mTLS\server.crt -keyFile .\x509\mTLS\server.key
    ```
2. 启动隧道客户端
    ```shell
    go run ./ -mode client -mTLS -rootCert
    File .\x509\mTLS\ca.crt -certFile .\x509\mTLS\client.crt  -keyFile .\x509\mTLS\client.key
    ```
## 生成自签双向认证mTLS证书
在执行命令前，请确保已经安装Openssl
### 第1步：生成根 CA（用于签发所有证书）
```shell
# 生成 CA 私钥
openssl genrsa -out ca.key 2048

# 生成 CA 证书（有效期10年）
openssl req -x509 -new -nodes -sha256 -key ca.key -out ca.crt -days 3650 -subj "/CN=My Root CA"
```
### 第2步：生成服务端证书
```shell
# 生成服务端私钥
openssl genrsa -out server.key 2048

# 生成服务端 CSR
openssl req -new -key server.key -out server.csr -subj "/CN=localhost" -addext "subjectAltName = DNS:localhost, DNS:*.example.com, IP:127.0.0.1"

# 使用 CA 签发服务端证书 Windows 系统该命令不可用
openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out server.crt -days 365 -sha256 -extfile <(echo "subjectAltName = DNS:localhost, DNS:*.example.com, IP:127.0.0.1")

# 如果是Windows系统，按下面步骤生成
1. 创建 san.ext 文件并写入: "subjectAltName = DNS:localhost, DNS:*.example.com, IP:127.0.0.1" | Out-File san.ext -Encoding utf8
2. 生成: openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out server.crt -days 365 -sha256 -extfile san.ext
```

### 第3步：生成客户端证书
```shell
# 生成客户端私钥
openssl genrsa -out client.key 2048

# 生成客户端 CSR（CN 用于标识客户端身份）
openssl req -new -key client.key -out client.csr -subj "/CN=client1"

# 使用 CA 签发客户端证书
openssl x509 -req -in client.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out client.crt -days 365 -sha256
```