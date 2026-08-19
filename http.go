package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"strings"
)

type StatusCode string

func (s StatusCode) Valid() bool {
	switch s {
	case Code200, Code500, Code502, Code503:
		return true
	default:
		return false
	}
}

func (s StatusCode) String(proto string) string {
	switch s {
	case Code200:
		return proto + " 200 Connection Established\r\n\r\n"
	case Code502:
		return proto + " 502 Bad Gateway\r\nConnection: close\r\n\r\nDNS resolution failed"
	case Code503:
		return proto + " 503 Service Unavailable\r\nConnection: close\r\n\r\nProxy could not connect to destination"
	default:
		return proto + " 500 Internal Server Error\r\nConnection: close\r\n\r\n"
	}
}

const (
	Code200 StatusCode = "200"
	Code500 StatusCode = "500"
	Code502 StatusCode = "502"
	Code503 StatusCode = "503"
)

type Header struct {
	Method       string
	URI          string
	Proto        string
	protoOffset  int
	headerOffset int
	buffer       []byte
}

func (h *Header) String() string {
	return fmt.Sprintf(`method: "%s" uri: "%s" proto: "%s"`, h.Method, h.URI, h.Proto)
}

func (h *Header) Get(k string) string {
	header := h.HeaderBytes()
	k = "\r\n" + k + ": "
	n := bytes.Index(header, []byte(k))
	if n == -1 {
		return ""
	}
	n2 := bytes.IndexByte(header[n+len(k):], '\n')
	if n2 == -1 {
		return ""
	}
	if header[n+len(k)+n2-1] == '\r' {
		n2--
	}
	return string(header[n+len(k) : n+len(k)+n2])
}

func (h *Header) GetHost() (string, bool) {
	host := h.Get("Host")
	if len(host) == 0 {
		// 如果是 Method 是 CONNECT 在检查一下URI是否是 Host
		if h.Method == http.MethodConnect {
			address := strings.TrimSpace(h.URI)
			if !strings.Contains(address, ":") {
				address += ":443"
			}
			if ap, err := netip.ParseAddrPort(address); err == nil {
				return ap.String(), true
			}
		}
		return "", false
	}
	if !strings.Contains(host, ":") {
		if h.Method == http.MethodConnect {
			return host + ":443", true
		} else {
			return host + ":80", true
		}
	}
	return host, true
}

func (h *Header) HeaderBytes() []byte {
	return h.buffer[h.protoOffset-1 : h.headerOffset+4]
}

func (h *Header) ReadBytes(r io.Reader) ([]byte, error) {
	n, err := r.Read(h.buffer)
	return h.buffer[:n], err
}

func ParseHeader(r io.Reader, bufferSize, maxUriSize int) (*Header, error) {
	var h Header
	h.buffer = make([]byte, 0, bufferSize)
	// 解析第一行
	for {
		n, err := r.Read(h.buffer[len(h.buffer):cap(h.buffer)])
		h.buffer = h.buffer[:len(h.buffer)+n]
		if err != nil {
			return nil, err
		}
		if maxUriSize > 0 && len(h.buffer) >= maxUriSize {
			return nil, fmt.Errorf("too much first line limit")
		}
		if len(h.buffer) == cap(h.buffer) {
			h.buffer = append(h.buffer, 0)[:len(h.buffer)]
		}
		if h.protoOffset = bytes.IndexByte(h.buffer, '\n'); h.protoOffset == -1 {
			continue
		}
		line := h.buffer[:h.protoOffset]
		if line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}
		method, other, ok := bytes.Cut(line, []byte{32})
		if !ok {
			return nil, fmt.Errorf("parse method err: %s", line)
		}
		uri, proto, ok := bytes.Cut(other, []byte{32})
		if !ok {
			return nil, fmt.Errorf("parse uri proto err: %s", other)
		}
		h.Method = string(method)
		h.URI = string(uri)
		h.Proto = string(proto)
		break
	}
	for h.headerOffset = bytes.Index(h.buffer, []byte{13, 10, 13, 10}); h.headerOffset == -1; h.headerOffset = bytes.Index(h.buffer, []byte{13, 10, 13, 10}) {
		n, err := r.Read(h.buffer[len(h.buffer):cap(h.buffer)])
		h.buffer = h.buffer[:len(h.buffer)+n]
		if err != nil {
			return nil, err
		}
		if len(h.buffer) == cap(h.buffer) {
			h.buffer = append(h.buffer, 0)[:len(h.buffer)]
		}
		if h.headerOffset = bytes.Index(h.buffer, []byte{13, 10, 13, 10}); h.headerOffset != -1 {
			break
		}
	}
	return &h, nil
}
