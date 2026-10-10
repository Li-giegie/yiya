package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"flag"
	"github.com/Li-giegie/netx"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"runtime"
	"time"
)

var (
	server = flag.Bool("server", false, "true run server,false run client")

	lAddr = flag.String("laddr", "0.0.0.0:1080", "tunnel client listen address")
	pAddr = flag.String("paddr", "0.0.0.0:1081", "tunnel server listen address")

	mTLS         = flag.Bool("mTLS", false, "enable mTls encrypt tunnel")
	rootCertFile = flag.String("rootCertFile", "", "CA root Certificate")
	certFile     = flag.String("certFile", "", "Certificate file path")
	keyFile      = flag.String("keyFile", "", "Private Key path")
	KeepAlive    = flag.Duration("keepAlive", time.Second*15, "tunnel keep alive interval")

	xor = flag.Bool("xor", false, "enable xor encrypt tunnel")
	key = flag.String("key", "", "xor key")

	forwardProxy                     = flag.Bool("forwardProxy", false, "enable forward proxy 正向代理模式 支持HTTP/S，示例 yiya -forwardProxy -laddr 0.0.0.0:1080")
	forwardProxyEnableAuth           = flag.Bool("forwardProxyEnableAuth", false, "enable basic auth")
	forwardProxyUserAuthFile         = flag.String("forwardProxyUserAuthFile", "./forwardProxyUserAuthFile.json", "forwardProxy user auth json file")
	forwardProxyUserAuthFileGenerate = flag.Bool("forwardProxyUserAuthFileGenerate", false, "generate forward proxy user auth json file template")
)

func main() {
	flag.Parse()
	if runtime.GOOS == "windows" {
		go io.Copy(io.Discard, os.Stdin)
	}
	log.Printf("config:\nserver: \t%v\nladdr: \t%s\npAddr: \t%s\nmTLS: \t%v\nrootCertFile: \t%s\ncertFile: \t%s\nkeyFile: \t%s\nxor: \t%v\nkey: \t%s\nforwardProxy: \t%v\nforwardProxyEnableAuth: \t%v\nforwardProxyUserAuthFile: \t%v\nforwardProxyUserAuthFileGenerate: \t%v\n", *server, *lAddr, *pAddr, *mTLS, *rootCertFile, *certFile, *keyFile, *xor, *key, *forwardProxy, *forwardProxyEnableAuth, *forwardProxyUserAuthFile, *forwardProxyUserAuthFileGenerate)
	if *forwardProxy {
		users := []*User{}
		if *forwardProxyEnableAuth && len(*forwardProxyUserAuthFile) > 0 {
			if len(*forwardProxyUserAuthFile) == 0 {
				log.Fatal("forward proxy user auth json file is empty")
			}
			f, err := os.Open(*forwardProxyUserAuthFile)
			if err != nil {
				log.Fatal(err)
			}
			err = json.NewDecoder(f).Decode(&users)
			f.Close()
			if err != nil {
				log.Fatal(err)
			}

		}
		var err error
		if len(*certFile) > 0 && len(*keyFile) > 0 {
			err = http.ListenAndServeTLS(*lAddr, *certFile, *keyFile, &ForwardProxyHandler{EnableAuth: *forwardProxyEnableAuth, Users: users})
		} else {
			err = http.ListenAndServe(*lAddr, &ForwardProxyHandler{EnableAuth: *forwardProxyEnableAuth, Users: users})
		}
		if err != nil {
			log.Fatal("http listen err", err)
		}
		return
	} else if *server {
		runServer()
		return
	} else if *forwardProxyUserAuthFileGenerate {
		data, err := json.MarshalIndent([]User{
			{
				Name:       "admin",
				Password:   "admin",
				ExpireTime: Time{Time: time.Now().Add(time.Hour * 24 * 8)},
			},
		}, "", "  ")
		if err != nil {
			log.Fatal("json marshal err", err)
		}
		if err = os.WriteFile("./forwardProxyUserAuthFile.json", data, 0644); err != nil {
			log.Fatal("write file err", err)
		}
		return
	}
	runClient()
}

func runServer() {
	var srv *netx.Server
	var err error
	if *mTLS {
		pool, cert := loadCert()
		l, err := tls.Listen("tcp", *pAddr, &tls.Config{
			ClientCAs:    pool,
			ClientAuth:   tls.RequireAndVerifyClientCert,
			Certificates: []tls.Certificate{*cert},
		})
		if err != nil {
			log.Fatal("tls listen err", err)
		}
		srv = netx.NewServer(l)
	} else if *xor {
		if len(*key) == 0 {
			log.Fatal("xor key is required")
		}
		if srv, err = netx.Listen("tcp", *pAddr); err != nil {
			log.Fatal("listen err", err)
		}
		srv.AcceptFunc = func(ctx context.Context, listener net.Listener) (net.Conn, error) {
			conn, err := listener.Accept()
			if err != nil {
				return nil, err
			}
			return NewXorConn(conn, []byte(*key)), nil
		}
	} else {
		if srv, err = netx.Listen("tcp", *pAddr); err != nil {
			log.Fatal("listen err", err)
		}
	}
	err = srv.Serve(TunnelServer{})
	log.Println("tunnel server exit", err)
}

func runClient() {
	l, err := net.Listen("tcp", *lAddr)
	if err != nil {
		log.Fatal("listen err", err)
	}
	defer l.Close()
	log.Println("tunnel client listen:", l.Addr().String())
	var dialer = net.Dialer{KeepAlive: *KeepAlive}
	var conn net.Conn
	if *mTLS {
		pool, cert := loadCert()
		tlsDialer := tls.Dialer{
			NetDialer: &dialer,
			Config: &tls.Config{
				RootCAs:      pool,
				ClientAuth:   tls.RequireAndVerifyClientCert,
				Certificates: []tls.Certificate{*cert},
			},
		}
		conn, err = tlsDialer.Dial("tcp", *pAddr)
		if err != nil {
			log.Fatal("tls dial err", err)
		}
	} else if *xor {
		if len(*key) == 0 {
			log.Fatal("xor key is required")
		}
		conn, err = net.Dial("tcp", *pAddr)
		if err != nil {
			log.Fatal("dial err", err)
		}
		conn = NewXorConn(conn, []byte(*key))
	} else {
		conn, err = dialer.Dial("tcp", *pAddr)
		if err != nil {
			log.Fatal("dial err", err)
		}
	}
	log.Println("tunnel client connected:", conn.RemoteAddr().String())
	upstream := netx.NewConn(conn, &netx.Config{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
	})
	var client TunnelClient
	err = client.Serve(l, upstream)
	log.Fatal("tunnel server exit", err)
}

func loadCert() (*x509.CertPool, *tls.Certificate) {
	rootCert, err := os.ReadFile(*rootCertFile)
	if err != nil {
		log.Fatal("read rootCertFile err", err)
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(rootCert)
	cert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
	if err != nil {
		log.Fatal("read certFile、keyFile err", err)
	}
	return pool, &cert
}
