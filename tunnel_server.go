package main

import (
	"io"
	"log"
	"net"

	"github.com/Li-giegie/netx"
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
		log.Println("与目的地址建立连接，响应失败", err)
		return
	}
	go func() {
		for {
			data, err = r.ReadChunk()
			if err != nil {
				log.Println("读取下游失败", err)
				return
			}
			if _, err = upstream.Write(data); err != nil {
				log.Println("写入上游失败", err)
				return
			}
		}
	}()
	buf := make([]byte, 4096)
	for {
		n, err := upstream.Read(buf)
		if err != nil {
			if err != io.EOF {
				log.Println("读取上游失败", err)
			}
			return
		}
		if _, err = w.Write(buf[:n]); err != nil {
			log.Println("写入下游失败", err)
			return
		}
	}
}
