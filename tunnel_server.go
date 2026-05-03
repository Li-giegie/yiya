package main

import (
	"context"
	"github.com/Li-giegie/netx"
	"log"
	"net"
)

func RunTunnelServer(addr string) error {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer l.Close()
	var s TunnelServer
	log.Println("tunnel server listen on", addr)
	return s.Serve(l)
}

var (
	handshakeRequest  = "Are you ok?"
	handshakeResponse = "I'm fine, thanks."
)

type TunnelServer struct{}

func (t TunnelServer) Serve(l net.Listener) error {
	srv := netx.NewServer(l)
	return srv.Serve(t)
}

func (t TunnelServer) Handle(r *netx.SessionReader, w *netx.SessionWriter) {
	defer func() {
		r.Close()
		w.Close()
		log.Println("session closed", r.Id())
	}()
	data, err := r.ReadChunk()
	if err != nil {
		log.Println("读取Host失败：", err)
		return
	}
	upstream, err := net.Dial("tcp", string(data))
	if err != nil {
		w.WriteClose([]byte("503"))
		log.Println("dial upstream error", string(data), err)
		return
	}
	defer upstream.Close()
	if _, err = w.Write([]byte("200")); err != nil {
		log.Println("response connect 200 err", err)
		return
	}
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()
	go func() {
		for {
			data, err = r.ReadChunk()
			if err != nil {
				log.Println("read down stream err", err)
				cancel()
				return
			}
			if _, err = upstream.Write(data); err != nil {
				log.Println("write up stream err", err)
				cancel()
				return
			}
		}
	}()
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := upstream.Read(buf)
			if err != nil {
				log.Println("读取上游失败", err)
				cancel()
				return
			}
			if _, err = w.Write(buf[:n]); err != nil {
				log.Println("写入下游失败", err)
				cancel()
				return
			}
		}
	}()
	<-ctx.Done()
}
