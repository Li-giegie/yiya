package main

import (
	"io"
	"net"
	"sync"
)

func NewXORReadWriter(rw io.ReadWriter, key []byte) *XORReadWriter {
	return &XORReadWriter{rw, key, 0, 0}
}

type XORReadWriter struct {
	rw  io.ReadWriter
	key []byte
	wi  uint
	ri  uint
}

func (x *XORReadWriter) Read(p []byte) (n int, err error) {
	n, err = x.rw.Read(p)
	for i := range p[:n] {
		p[i] = p[i] ^ x.key[x.ri%uint(len(x.key))]
		x.ri++
	}
	return
}

func (x *XORReadWriter) Write(p []byte) (n int, err error) {
	for i := range p {
		p[i] = p[i] ^ x.key[x.wi%uint(len(x.key))]
		x.wi++
	}
	return x.rw.Write(p)
}

func NewXorConn(conn net.Conn, key []byte) *XORConn {
	return &XORConn{
		Conn:          conn,
		XORReadWriter: NewXORReadWriter(conn, key),
	}
}

type XORConn struct {
	net.Conn
	*XORReadWriter
	sync.Mutex
}

func (c *XORConn) Write(p []byte) (n int, err error) {
	// 加锁保证顺序，不然无法还原
	c.Lock()
	defer c.Unlock()
	return c.XORReadWriter.Write(p)
}

func (c *XORConn) Read(p []byte) (n int, err error) {
	return c.XORReadWriter.Read(p)
}
