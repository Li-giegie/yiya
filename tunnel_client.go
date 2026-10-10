package main

import (
	"fmt"
	"github.com/Li-giegie/netx"
	"io"
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
	closeCh chan struct{}
}

func (s *TunnelClient) Serve(downstream net.Listener, upstream *netx.Conn) error {
	defer func() {
		downstream.Close()
		upstream.Stop()
		s.Stop()
	}()
	s.closeCh = make(chan struct{})

	upstreamDone := make(chan error, 1)
	go func() {
		defer close(upstreamDone)
		upstreamDone <- upstream.Serve(empty{})
	}()

	downstreamDone := make(chan error, 1)
	go func() {
		defer close(downstreamDone)
		for {
			conn, err := downstream.Accept()
			if err != nil {
				downstreamDone <- fmt.Errorf("downstream conn accept error: %v", err)
				return
			}
			session, err := upstream.Session()
			if err != nil {
				conn.Close()
				downstreamDone <- fmt.Errorf("open upstream session error: %v", err)
				return
			}
			go s.Handle(conn, session)
			//continue
			//rFile, err := os.OpenFile("./.trace/read_"+strings.ReplaceAll(conn.RemoteAddr().String(), ":", "_"), os.O_RDWR|os.O_CREATE, 0600)
			//if err != nil {
			//	conn.Close()
			//	log.Println("打开文件失败", err)
			//	return
			//}
			//wFile, err := os.OpenFile("./.trace/write_"+strings.ReplaceAll(conn.RemoteAddr().String(), ":", "_"), os.O_RDWR|os.O_CREATE, 0600)
			//if err != nil {
			//	conn.Close()
			//	log.Println("打开文件失败", err)
			//	return
			//}
			//log.Println("追踪文件", rFile.Name(), "----", wFile.Name())
			//go s.Handle(&TraceConn{
			//	Conn: conn,
			//	ReadFunc: func(bytes []byte, i int, err error) {
			//		rFile.Write(bytes[:i])
			//	},
			//	WriteFunc: func(bytes []byte, i int, err error) {
			//		wFile.Write(bytes[:i])
			//	},
			//	CloseFunc: func(err error) {
			//		rFile.Sync()
			//		rFile.Close()
			//		wFile.Sync()
			//		wFile.Close()
			//		log.Println("关闭文件")
			//	},
			//}, session)
		}
	}()

	select {
	case <-s.closeCh:
		return nil
	case err := <-downstreamDone:
		return err
	case err := <-upstreamDone:
		return err
	}
}

func (s *TunnelClient) Stop() {
	if s.closeCh != nil {
		close(s.closeCh)
		s.closeCh = nil
	}
}

func (s *TunnelClient) Handle(conn net.Conn, session *netx.Session) {
	now := time.Now()
	log.Println("session id", session.SessionWriter.Id())
	defer func() {
		conn.Close()
		session.SessionWriter.Close()
		session.SessionReader.Close()
		log.Println("session", session.SessionWriter.Id(), "close", session.SessionWriter.Id(), "持续时长", time.Since(now))
	}()

	headReader, err := ParseHeader(conn, 4096, 4096)
	if err != nil {
		log.Println("解析HTTP Header 失败", err)
		return
	}

	host, ok := headReader.GetHost()
	if !ok {
		os.WriteFile("./no_host_"+time.Now().Format("20060102150405")+".txt", headReader.buffer, 0644)
		log.Println("解析HTTP Header 失败 没有Host")
		return
	}

	if _, err = session.Write([]byte(host)); err != nil {
		log.Println("发送Host失败", err)
		return
	}

	connectReply, err := session.ReadChunk()
	if err != nil {
		log.Println("读取上游响应CONNECT失败", err)
		return
	}
	switch code := StatusCode(connectReply); code {
	case Code200:
		if headReader.Method == http.MethodConnect {
			if _, err = conn.Write([]byte(code.String(headReader.Proto))); err != nil {
				log.Println("发送第一条HTTP报文失败：", err)
				return
			}
		} else {
			if _, err = session.Write(headReader.buffer); err != nil {
				log.Println("转发第一条HTTP报文失败", err)
				return
			}
		}
	case Code500, Code502, Code503:
		conn.Write([]byte(code.String(headReader.Proto)))
		log.Println("CONNECT 失败", code)
		return
	default:
		log.Println("CONNECT 失败：未知Code", code)
		return
	}

	go func() {
		for {
			chunk, err := session.ReadChunk()
			if err != nil {
				if err != io.EOF {
					log.Println("读取上游失败", err)
				}
				return
			}
			if _, err = conn.Write(chunk); err != nil {
				log.Println("写入下游失败", err)
				return
			}
		}
	}()

	for {
		data, err := headReader.ReadBytes(conn)
		if err != nil {
			if err != io.EOF {
				log.Println("读取下游失败", err)
			}
			return
		}
		if _, err = session.Write(data); err != nil {
			log.Println("写入上游失败：", err)
			return
		}
	}
}

type empty struct{}

func (e empty) Handle(r *netx.SessionReader, w *netx.SessionWriter) {
	defer func() {
		r.Close()
		w.Close()
	}()
}
