package main

import (
	"bytes"
	"io"
	"testing"
)

func TestXOR(t *testing.T) {
	buf := new(bytes.Buffer)
	w := NewXORWriter(buf, []byte("123456"))
	n, err := w.Write([]byte("123"))
	if n != 3 || err != nil {
		t.Fatal("fail", n, err)
	}

	n, err = w.Write([]byte("hahaha"))
	if n != 6 || err != nil {
		t.Fatal("fail", n, err)
	}

	r := NewXORReader(buf, []byte("123456"))

	b := make([]byte, 0, 64)
	for {
		n, err = r.Read(b[len(b):cap(b)])
		b = b[:len(b)+n]
		if err != nil {
			if err != io.EOF {
				t.Fatal("fail", err)
			}
			break
		}
		if len(b) == cap(b) {
			b = append(b, 0)[:cap(b)]
		}
	}
	if string(b) != "123hahaha" {
		t.Fatal("fail", string(b))
	}
}
