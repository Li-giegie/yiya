package main

import (
	"encoding/binary"
	"errors"
	"io"
	"math/rand"
	"net"
	"sync"
)

func NewXorConn(conn net.Conn, key []byte) *XORConn {
	return &XORConn{
		Conn: conn,
		rd:   NewXORReader(conn, key),
		wr:   NewXORWriter(conn, key),
	}
}

type XORConn struct {
	rd *XORReader
	wr *XORWriter
	net.Conn
	sync.Mutex
}

func (c *XORConn) Write(p []byte) (n int, err error) {
	return c.wr.Write(p)
}

func (c *XORConn) Read(p []byte) (n int, err error) {
	return c.rd.Read(p)
}

func NewXORWriter(w io.Writer, key []byte) *XORWriter {
	return &XORWriter{
		w: w,
		p: sync.Pool{
			New: func() any {
				b := make([]byte, 8, 1024)
				return &b
			},
		},
		key: key,
	}
}

type XORWriter struct {
	w   io.Writer
	p   sync.Pool
	key []byte
}

var ErrWriteLong = errors.New("write too large: maximum size is 4GB (uint32 limit)")

func (x *XORWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if uint64(len(p)) > 0xffffffff {
		return 0, ErrWriteLong
	}
	offset := rand.Uint32()
	pBuf := x.p.Get().(*[]byte)
	*pBuf = (*pBuf)[:8]
	binary.LittleEndian.PutUint32((*pBuf)[:4], uint32(len(p)))
	binary.LittleEndian.PutUint32((*pBuf)[4:8], offset)
	*pBuf = append(*pBuf, p...)
	for i := 0; i < len(p); i++ {
		(*pBuf)[i+8] = (*pBuf)[i+8] ^ x.key[(uint32(i)+offset)%uint32(len(x.key))]
	}
	n, err := x.w.Write(*pBuf)
	if n -= 8; n < 0 {
		n = 0
	}
	x.p.Put(pBuf)
	return n, err
}

func NewXORReader(r io.Reader, key []byte) *XORReader {
	return &XORReader{
		r:   r,
		key: key,
		hb:  make([]byte, 8),
	}
}

type XORReader struct {
	r      io.Reader
	key    []byte
	hb     []byte
	size   uint32
	offset uint32
}

func (x *XORReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if x.size <= 0 {
		if _, err := io.ReadFull(x.r, x.hb); err != nil {
			return 0, err
		}
		x.offset = binary.LittleEndian.Uint32(x.hb[4:])
		if x.size = binary.LittleEndian.Uint32(x.hb[:4]); x.size == 0 {
			return 0, nil
		}
	}
	if uint32(len(p)) > x.size {
		p = p[:x.size]
	}
	n, err := x.r.Read(p)
	x.size -= uint32(n)
	for i := range p[:n] {
		p[i] ^= x.key[x.offset%uint32(len(x.key))]
		x.offset++
	}
	return n, err
}
