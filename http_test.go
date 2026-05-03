package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestParseHeader(t *testing.T) {
	p := []string{
		"GET / HTTP/1.1\r\nHost: example.com\r\nContent-Type: application/json\r\n\r\nname=test&other=xxx",
		"GET /index.html HTTP/1.1\r\nHost: 127.0.0.1:8080\r\nUser-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64)\r\nAccept: text/html,application/xhtml+xml\r\nAccept-Language: zh-CN,zh;q=0.9\r\nConnection: keep-alive\r\n\r\n",
		"GET /api/user?name=test&age=18&token=abc123xyz HTTP/1.1\r\nHost: localhost:8080\r\nCache-Control: no-cache\r\nReferer: http://localhost/\r\nConnection: close\r\n\r\n",
		"POST /api/login HTTP/1.1\r\nHost: 127.0.0.1:8080\r\nContent-Type: application/x-www-form-urlencoded\r\nContent-Length: 27\r\nConnection: keep-alive\r\n\r\nusername=admin&password=123456",
		"POST /api/user/add HTTP/1.1\r\nHost: 127.0.0.1:8080\r\nContent-Type: application/json;charset=utf-8\r\nContent-Length: 68\r\nAuthorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9\r\n\r\n{\"id\":1001,\"nickname\":\"测试用户\",\"role\":\"admin\",\"status\":1}",
		"GET /api/info HTTP/1.1\r\nHost: localhost:8080\r\nCookie: sessionid=987654321; uid=10086\r\nX-Request-ID: 20260502-test-001\r\nX-Custom-Token: abcdef123456\r\nConnection: close\r\n\r\n",
		"PUT /api/user/1001 HTTP/1.1\r\nHost: 127.0.0.1:8080\r\nContent-Type: application/json\r\nContent-Length: 45\r\n\r\n{\"nickname\":\"修改昵称\",\"phone\":\"13800138000\"}",
		"DELETE /api/user/1001 HTTP/1.1\r\nHost: 127.0.0.1:8080\r\nAuthorization: Basic YWRtaW46MTIzNDU2\r\nConnection: close\r\n\r\n",
		"HEAD /favicon.ico HTTP/1.1\r\nHost: localhost:8080\r\nConnection: keep-alive\r\n\r\n",
		"OPTIONS /api/* HTTP/1.1\r\nHost: 127.0.0.1:8080\r\nAccess-Control-Request-Method: POST\r\nAccess-Control-Request-Headers: content-type\r\nOrigin: http://127.0.0.1:3000\r\n\r\n",
		"GET /test HTTP/2.0.1\r\nHost: localhost\r\n\r\n",
		"GET /no-host HTTP/1.1\r\nUser-Agent: test-client\r\nConnection: close\r\n\r\n",
		"GET /a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a HTTP/1.1\r\nHost: localhost:8080\r\n\r\n",
		"POST /api/empty HTTP/1.1\r\nHost: 127.0.0.1:8080\r\nContent-Type: application/json\r\nContent-Length: 0\r\n\r\n",
		"CONNECT example.com:443 HTTP/1.1\r\nHost: example.com:443\r\nProxy-Connection: keep-alive\r\nUser-Agent: Mozilla/5.0\r\nAccept: */*\r\n\r\n",
	}

	for i, s := range p {
		h, err := ParseHeader(strings.NewReader(s), 512, -1)
		if err != nil {
			t.Fatal(i, err)
		}
		fmt.Println(h.String())
		fmt.Println(string(h.HeaderBytes()))
		host, ok := h.GetHost()
		if !ok {
			if strings.Contains(s, "\r\nHost: ") {
				t.Fatal("host not found in header", s)
			}
			continue
		}
		fmt.Println(host)
	}

}

func TestTLSServer(t *testing.T) {
	rootCrt, err := os.ReadFile("./x509/mTLS/ca.crt")
	if err != nil {
		t.Fatal(err)
	}
	p := x509.NewCertPool()
	p.AppendCertsFromPEM(rootCrt)
	serverCrt, err := tls.LoadX509KeyPair("./x509/mTLS/server.crt", "./x509/mTLS/server.key")
	if err != nil {
		t.Fatal(err)
	}

	cfg := &tls.Config{
		Certificates: []tls.Certificate{serverCrt},
		ClientCAs:    p,
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}
	l, err := tls.Listen("tcp", "127.0.0.1:9000", cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	for {
		conn, err := l.Accept()
		if err != nil {
			t.Fatal(err)
		}
		go func() {
			defer conn.Close()
			fmt.Println(io.Copy(os.Stdout, conn))
		}()
	}
}

func TestTLSClient(t *testing.T) {
	rootCrt, err := os.ReadFile("./x509/mTLS/ca.crt")
	if err != nil {
		t.Fatal(err)
	}
	p := x509.NewCertPool()
	p.AppendCertsFromPEM(rootCrt)
	clientCrt, err := tls.LoadX509KeyPair("./x509/mTLS/client.crt", "./x509/mTLS/client.key")
	if err != nil {
		t.Fatal(err)
	}
	cfg := &tls.Config{
		RootCAs:      p,
		Certificates: []tls.Certificate{clientCrt},
	}
	conn, err := tls.Dial("tcp", "127.0.0.1:9000", cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	for i := 0; i < 10; i++ {
		_, err = conn.Write([]byte("hello" + strconv.Itoa(i)))
		if err != nil {
			t.Error(err)
			break
		}
	}
}

func TestName(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:9000")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	go func() {
		conn, err := l.Accept()
		if err != nil {
			t.Fatal(err)
		}
		go func() {
			defer conn.Close()
			io.Copy(os.Stdout, conn)
		}()
	}()

	dialer := net.Dialer{
		Timeout:         0,
		Deadline:        time.Time{},
		LocalAddr:       nil,
		DualStack:       false,
		FallbackDelay:   0,
		KeepAlive:       time.Second * 5,
		KeepAliveConfig: net.KeepAliveConfig{},
		Resolver:        nil,
		Cancel:          nil,
		Control:         nil,
		ControlContext:  nil,
	}
	conn, err := dialer.Dial("tcp", "127.0.0.1:9000")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	time.Sleep(time.Minute)
}
