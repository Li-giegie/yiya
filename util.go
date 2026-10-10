package main

import "net"

type TraceConn struct {
	net.Conn
	ReadFunc  func([]byte, int, error)
	WriteFunc func([]byte, int, error)
	CloseFunc func(error)
}

func (conn *TraceConn) Read(b []byte) (n int, err error) {
	n, err = conn.Conn.Read(b)
	if conn.ReadFunc != nil {
		conn.ReadFunc(b, n, err)
	}
	return
}

func (conn *TraceConn) Write(b []byte) (n int, err error) {
	n, err = conn.Conn.Write(b)
	if conn.WriteFunc != nil {
		conn.WriteFunc(b, n, err)
	}
	return
}

func (conn *TraceConn) Close() (err error) {
	err = conn.Conn.Close()
	if conn.CloseFunc != nil {
		conn.CloseFunc(err)
	}
	return
}
