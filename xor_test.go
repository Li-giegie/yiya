package main

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"testing"
	"time"
)

func TestXOR(t *testing.T) {
	listen, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(listen.Addr().String())
	defer listen.Close()
	go func() {
		for {
			conn, err := listen.Accept()
			if err != nil {
				t.Fatal(err)
				return
			}
			go func() {
				conn = NewXorConn(conn, []byte("123456"))
				defer conn.Close()

				buf := make([]byte, 1024)
				for {
					n, err := conn.Read(buf)
					if err != nil {
						log.Println("read error:", err)
						return
					}
					println("server read", string(buf[:n]))
					conn.Write([]byte("server receive"))
					conn.Write(buf[:n])
				}
			}()
		}
	}()

	conn, err := net.Dial("tcp", listen.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn = NewXorConn(conn, []byte("123456"))
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := conn.Read(buf)
			if err != nil {
				log.Println("read error:", err)
				return
			}
			println("client read", string(buf[:n]))
		}
	}()
	for i := 0; i < 3; i++ {
		conn.Write([]byte(strconv.Itoa(i)))
		conn.Write([]byte(time.Now().String()))
		time.Sleep(time.Second)
	}

}
