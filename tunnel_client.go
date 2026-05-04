package main

import (
	"context"
	"errors"
	"github.com/Li-giegie/netx"
	"log"
	"net"
	"net/http"
	"os"
	"time"
)

func RunTunnelClient(lAddr, rAddr string) error {
	l, err := net.Listen("tcp", lAddr)
	if err != nil {
		return err
	}
	log.Println("tunnel client listen on", lAddr, "forward to", rAddr)
	conn, err := netx.Dial("tcp", rAddr)
	if err != nil {
		return err
	}
	defer conn.Stop()
	var client TunnelClient
	return client.Serve(l, conn)
}

type TunnelClient struct {
	ctx    context.Context
	cancel context.CancelCauseFunc
}

func (s *TunnelClient) Serve(l net.Listener, upstream *netx.Conn) (err error) {
	defer func() {
		l.Close()
		upstream.Stop()
	}()
	s.ctx, s.cancel = context.WithCancelCause(context.TODO())
	go func() {
		if err = upstream.Serve(empty{}); err != nil {
			log.Println("upstream conn exit", err)
		}
		s.cancel(err)
	}()
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				log.Println("tunnel conn exit", err)
				s.cancel(err)
				return
			}
			session, err := upstream.Session()
			if err != nil {
				log.Println("open tunnel session err", err)
				s.cancel(err)
				return
			}
			go s.Handle(conn, session)
		}
	}()
	<-s.ctx.Done()
	if err = context.Cause(s.ctx); err != nil && errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

func (s *TunnelClient) Stop() {
	s.cancel(nil)
}

func (s *TunnelClient) Handle(conn net.Conn, session *netx.Session) {
	log.Println("session id", session.SessionWriter.Id())
	defer func() {
		conn.Close()
		session.SessionWriter.Close()
		session.SessionReader.Close()
		log.Println("session close", session.SessionWriter.Id())
	}()

	h, err := ParseHeader(conn, 4096, 4096)
	if err != nil {
		log.Println("failed to parse header:", err)
		return
	}
	host, ok := h.GetHost()
	if !ok {
		os.WriteFile("./no_host_"+time.Now().Format("20060102150405")+".txt", h.buffer, 0644)
		log.Println("failed to parse host header")
		return
	}
	if _, err = session.Write([]byte(host)); err != nil {
		log.Println("failed to write tunnel-server:", err)
		return
	}
	connectReply, err := session.ReadChunk()
	if err != nil {
		log.Println("读取上游响应CONNECT失败", err)
		return
	}
	switch code := StatusCode(connectReply); code {
	case Code200:
		if h.Method == http.MethodConnect {
			if _, err = conn.Write([]byte(code.String(h.Proto))); err != nil {
				log.Println("发送第一条HTTP报文失败：", err)
				return
			}
		} else {
			if _, err = session.Write(h.buffer); err != nil {
				log.Println("转发第一条HTTP报文失败", err)
				return
			}
		}
	case Code500, Code502, Code503:
		conn.Write([]byte(code.String(h.Proto)))
		log.Println("CONNECT 失败", code)
		return
	default:
		log.Println("CONNECT 失败：未知Code", code)
		return
	}
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()
	go func() {
		for {
			chunk, err := session.ReadChunk()
			if err != nil {
				log.Println("读取隧道服务端失败", err)
				cancel()
				return
			}
			if _, err = conn.Write(chunk); err != nil {
				log.Println("写入源失败", err)
				cancel()
				return
			}
		}
	}()
	go func() {
		for {
			data, err := h.ReadBytes(conn)
			if err != nil {
				log.Println("读取源失败", err)
				cancel()
				return
			}
			if _, err = session.Write(data); err != nil {
				log.Println("转发到隧道服务端失败：", err)
				cancel()
				return
			}
		}
	}()
	<-ctx.Done()
}

type empty struct{}

func (e empty) Handle(r *netx.SessionReader, w *netx.SessionWriter) {
	defer func() {
		r.Close()
		w.Close()
	}()
}
