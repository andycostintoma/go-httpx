package httpx

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/andycostintoma/httpx/internal/netx"
)

func TestParseRequestLine(t *testing.T) {
	line := "GET /a/b?x=1 HTTP/1.1"
	rl, err := parseRequestLine(line)
	if err != nil {
		t.Fatal(err)
	}
	if rl.Method != "GET" || rl.RequestURI != "/a/b?x=1" || rl.Proto != "HTTP/1.1" {
		t.Fatalf("parsed wrong: %+v", rl)
	}
	if rl.ProtoMajor != 1 || rl.ProtoMinor != 1 {
		t.Fatalf("version wrong: %d.%d", rl.ProtoMajor, rl.ProtoMinor)
	}
}

func TestParseRequestLineBad(t *testing.T) {
	cases := []string{
		"G ET / HTTP/1.1",                     // space in method
		"GET / WTF/1.1",                       // proto missing HTTP/
		"GET / HTTP/x.y",                      // invalid version numbers
		"",                                    // empty
		"GET / HTTP/1",                        // missing minor version
		"TOOLONGMETHODNAMEFORHTTP / HTTP/1.1", // >20 chars
	}
	for _, c := range cases {
		if _, err := parseRequestLine(c); err == nil {
			t.Fatalf("expected error for %q", c)
		}
	}
}

func TestParseRequest(t *testing.T) {
	raw := "GET /a/b?x=1 HTTP/1.1\r\nHost: ex.com\r\n\r\n"
	rd := netx.NewCRLFFastReader(bytes.NewBufferString(raw))
	req, err := ParseRequest(rd, ParseLimits{MaxLineBytes: 4096})
	if err != nil {
		t.Fatal(err)
	}
	if req.Method != "GET" || req.Proto != "HTTP/1.1" {
		t.Fatalf("method/proto mismatch: %v %v", req.Method, req.Proto)
	}
	if req.URL.Path != "/a/b" || req.URL.RawQuery != "x=1" {
		t.Fatalf("url mismatch: %+v", req.URL)
	}
	if req.Host != "" {
		t.Fatalf("expected empty Host for origin-form URI, got %q", req.Host)
	}
}

func TestParseRequestAbsoluteForm(t *testing.T) {
	raw := "GET http://example.com/x?q=1 HTTP/1.1\r\n\r\n"
	rd := netx.NewCRLFFastReader(bytes.NewBufferString(raw))
	req, err := ParseRequest(rd, ParseLimits{MaxLineBytes: 4096})
	if err != nil {
		t.Fatal(err)
	}
	if req.URL.Host != "example.com" {
		t.Fatalf("expected host example.com, got %q", req.URL.Host)
	}
	if req.Host != "example.com" {
		t.Fatalf("Host not propagated from absolute URI, got %q", req.Host)
	}
}

func TestContextCancelDuringParse(t *testing.T) {
	raw := "GET / HTTP/1.1\r\nHost: x\r\n\r\n"
	rd := netx.NewCRLFFastReader(strings.NewReader(raw))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := parseRequestWithContext(ctx, rd, ParseLimits{MaxLineBytes: 4096})
	if err == nil {
		t.Fatal("expected ctx error")
	}
}
