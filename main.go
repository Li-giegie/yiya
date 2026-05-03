package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"github.com/Li-giegie/netx"
	"log"
	"net"
	"os"
	"time"
)

var (
	mode         = flag.String("mode", "server", "run mode")
	lAddr        = flag.String("laddr", "127.0.0.1:1080", "tunnel client listen address")
	pAddr        = flag.String("paddr", "127.0.0.1:1081", "tunnel server listen address")
	mTLS         = flag.Bool("mTLS", false, "enable mTls")
	rootCertFile = flag.String("rootCertFile", "", "CA root Certificate")
	certFile     = flag.String("certFile", "", "Certificate file path")
	keyFile      = flag.String("keyFile", "", "Private Key path")
	KeepAlive    = flag.Duration("keepAlive", time.Second*15, "tunnel keep alive interval")
)

func main() {
	flag.Parse()
	log.Printf("config:\nmode: \t%s\nladdr: \t%s\npAddr: \t%s\nmTLS: \t%v\nrootCertFile: \t%s\ncertFile: \t%s\nkeyFile: \t%s\n", *mode, *lAddr, *pAddr, *mTLS, *rootCertFile, *certFile, *keyFile)
	switch *mode {
	case "server":
		var listener net.Listener
		if *mTLS {
			pool, cert := loadCert()
			l, err := tls.Listen("tcp", *pAddr,
				&tls.Config{
					ClientCAs:    pool,
					ClientAuth:   tls.RequireAndVerifyClientCert,
					Certificates: []tls.Certificate{*cert},
				},
			)
			if err != nil {
				log.Fatal("tls listen err", err)
			}
			listener = l
		} else {
			l, err := net.Listen("tcp", *pAddr)
			if err != nil {
				log.Fatal("listen err", err)
			}
			listener = l
		}
		log.Println("tunnel server listen on", *pAddr)
		var s TunnelServer
		err := s.Serve(listener)
		log.Println("server exit", err)
	case "client":
		dialer := net.Dialer{KeepAlive: *KeepAlive}
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
			c, err := tlsDialer.Dial("tcp", *pAddr)
			if err != nil {
				log.Fatal("tls dial err", err)
			}
			conn = c
		} else {
			c, err := dialer.Dial("tcp", *pAddr)
			if err != nil {
				log.Fatal("dial err", err)
			}
			conn = c
			log.Println("connect tunnel server on", *pAddr)
		}
		listen, err := net.Listen("tcp", *lAddr)
		if err != nil {
			log.Fatal("listen err", err)
		}
		log.Println("client listen on", *lAddr)
		var client TunnelClient
		err = client.Serve(listen, netx.NewConn(conn, &netx.Config{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
		}))
		log.Println("tunnel client exit:", err)
	default:
		log.Fatalf("unknown mode: %s", *mode)
	}
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
