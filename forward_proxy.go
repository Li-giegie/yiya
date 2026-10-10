package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

type Time struct {
	time.Time
}

func (t Time) MarshalJSON() ([]byte, error) {
	b := make([]byte, 0, len(time.DateTime)+len(`""`))
	b = append(b, '"')
	b = append(b, t.Time.Format(time.DateTime)...)
	b = append(b, '"')
	return b, nil
}

func (t *Time) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	// TODO(https://go.dev/issue/47353): Properly unescape a JSON string.
	if len(data) < 2 || data[0] != '"' || data[len(data)-1] != '"' {
		return errors.New("Time.UnmarshalJSON: input is not a JSON string")
	}
	data = data[len(`"`) : len(data)-len(`"`)]
	var err error
	for _, str := range []string{time.RFC3339, time.RFC3339Nano, time.DateTime, time.DateOnly} {
		t.Time, err = time.ParseInLocation(str, string(data), time.Local)
		if err == nil {
			return nil
		}
	}
	return err
}

type User struct {
	Name       string `json:"name"`
	Password   string `json:"password"`
	ExpireTime Time
}
type ForwardProxyHandler struct {
	EnableAuth bool
	Users      []*User
}

func (p *ForwardProxyHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if p.EnableAuth {
		username, password, ok := p.getProxyAuth(req)
		if !ok || len(username) == 0 || len(password) == 0 {
			rw.Header().Set("Proxy-Authenticate", `Basic realm="Please input proxy username and password"`)
			rw.WriteHeader(http.StatusProxyAuthRequired)
			log.Println("获取授权信息为空")
			return
		}
		for _, user := range p.Users {
			if user.Name == username {
				if password != user.Password {
					rw.Header().Set("Proxy-Authenticate", `Basic realm="Please input proxy username and password"`)
					rw.WriteHeader(http.StatusProxyAuthRequired)
					log.Println("用户授权失败")
					return
				}
				if time.Now().After(user.ExpireTime.Time) {
					rw.Header().Set("Proxy-Authenticate", `Basic realm="Proxy login required"`)
					rw.WriteHeader(http.StatusProxyAuthRequired)
					log.Println("用户授权超期")
					return
				}
				fmt.Println("授权成功", user.Name, user.Password, user.ExpireTime)
				break
			}
		}
	}
	log.Println("写入数据")
	if req.Method == http.MethodConnect {
		host := req.Host
		if !strings.Contains(host, ":") {
			host += ":443"
		}
		dst, err := net.Dial("tcp", host)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusBadGateway)
			log.Println("dial error:", host, err)
			return
		}
		defer dst.Close()
		src, brw, err := rw.(http.Hijacker).Hijack()
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			log.Println("劫持连接失败", err)
			return
		}
		defer src.Close()

		// 告诉客户端隧道建立成功
		_, err = brw.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))
		if err != nil {
			log.Println("隧道建立成功响应失败", err)
			return
		}
		err = brw.Flush()
		if err != nil {
			return
		}
		// 双向拷贝数据
		go io.Copy(dst, src)
		io.Copy(src, dst)
		return
	}
	req.RequestURI = ""
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		rw.WriteHeader(http.StatusBadGateway)
		log.Println("do error:", err)
		return
	}
	defer resp.Body.Close()
	rw.WriteHeader(resp.StatusCode)
	io.Copy(rw, resp.Body)
	return
}

func (p *ForwardProxyHandler) getProxyAuth(r *http.Request) (username string, password string, exist bool) {
	authHeader := r.Header.Get("Proxy-Authorization")
	if authHeader == "" {
		return
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Basic" {
		return
	}

	// base64解码 user:pass
	raw, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return
	}
	up := strings.SplitN(string(raw), ":", 2)
	if len(up) != 2 {
		return
	}

	return up[0], up[1], true
}
