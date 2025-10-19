package netx

import (
	"bytes"
	"testing"
)

func TestReadLineCRLF(t *testing.T) {
	r := NewCRLFFastReader(bytes.NewBufferString("GET / HTTP/1.1\r\nHost: x\r\n\r\n"))
	l, _, _ := r.ReadLine(4096)
	if string(l) != "GET / HTTP/1.1" {
		t.Fatal("first line mismatch")
	}
	l, _, _ = r.ReadLine(4096)
	if string(l) != "Host: x" {
		t.Fatal("header line mismatch")
	}
	l, _, _ = r.ReadLine(4096)
	if len(l) != 0 {
		t.Fatal("expected empty line before body")
	}
}

func TestReadLineMax(t *testing.T) {
	big := bytes.Repeat([]byte("a"), 10<<20)
	r := NewCRLFFastReader(bytes.NewReader(append(big, '\r', '\n')))
	_, _, err := r.ReadLine(1024)
	if err == nil {
		t.Fatal("expected ErrLineTooLong")
	}
}

func TestTolerateBareLF(t *testing.T) {
	r := NewCRLFFastReader(bytes.NewBufferString("Host: x\n\n"))
	l, _, _ := r.ReadLine(1024)
	if string(l) != "Host: x" {
		t.Fatalf("got %q", string(l))
	}
	l, _, _ = r.ReadLine(1024)
	if len(l) != 0 {
		t.Fatal("expected empty")
	}
}

func TestPeekBound(t *testing.T) {
	r := NewCRLFFastReader(bytes.NewBufferString("abc\r\n"))
	p, err := r.Peek(2)
	if err != nil {
		t.Fatal(err)
	}
	if string(p) != "ab" {
		t.Fatal(string(p))
	}
}
