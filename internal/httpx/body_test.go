package httpx

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
)

// -----------------------------------------------------------------------------
// fixedReader tests
// -----------------------------------------------------------------------------

func TestFixedLengthBody(t *testing.T) {
	raw := "hello world"
	r := strings.NewReader(raw)

	// Use constructor with a valid context to avoid nil panic
	fr := newFixedReader(context.Background(), r, int64(len(raw)), 0)

	data, err := io.ReadAll(fr)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != raw {
		t.Fatalf("got %q, want %q", data, raw)
	}

	// reading again must return EOF
	n, err := fr.Read(make([]byte, 1))
	if n != 0 || err != io.EOF {
		t.Fatalf("expected EOF, got n=%d err=%v", n, err)
	}
}

func TestFixedLengthTooShort(t *testing.T) {
	r := strings.NewReader("abc")
	fr := newFixedReader(context.Background(), r, 5, 0)

	_, err := io.ReadAll(fr)
	if err == nil {
		t.Fatal("expected ErrLengthMismatch for short body")
	}
}

// -----------------------------------------------------------------------------
// chunkedReader tests
// -----------------------------------------------------------------------------

func TestChunkedBody(t *testing.T) {
	raw := "" +
		"4\r\nWiki\r\n" +
		"5\r\npedia\r\n" +
		"0\r\nX-T: v\r\n\r\n"

	r := bytes.NewBufferString(raw)
	ctx := context.Background()

	cr := newChunkedReader(ctx, r, 1<<20, Header{})
	data, err := io.ReadAll(cr)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "Wikipedia" {
		t.Fatalf("got %q, want %q", data, "Wikipedia")
	}

	// Type assert to concrete type to inspect trailers
	hdr := cr.(*chunkedReader)
	if hdr.header.Get("X-T") != "v" {
		t.Fatalf("missing or invalid trailer, got %#v", hdr.header)
	}
}

func TestChunkedBadEncoding(t *testing.T) {
	raw := "ZZZ\r\nbad\r\n"
	r := bytes.NewBufferString(raw)
	cr := newChunkedReader(context.Background(), r, 1024, Header{})

	_, err := io.ReadAll(cr)
	if err == nil {
		t.Fatal("expected ErrBadChunk for invalid encoding")
	}
}

// -----------------------------------------------------------------------------
// closeReader tests
// -----------------------------------------------------------------------------

func TestCloseReaderEOF(t *testing.T) {
	r := strings.NewReader("abc")
	cr := newCloseReader(context.Background(), r, 0)

	data, err := io.ReadAll(cr)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "abc" {
		t.Fatalf("got %q, want %q", data, "abc")
	}
}

// -----------------------------------------------------------------------------
// context cancellation test
// -----------------------------------------------------------------------------

func TestContextCancelDuringRead(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediately cancel

	r := strings.NewReader("abc")
	fr := newFixedReader(ctx, r, 3, 0)

	buf := make([]byte, 2)
	_, err := fr.Read(buf)

	if err == nil {
		t.Fatal("expected context cancellation error")
	}
	if ctx.Err() == nil {
		t.Fatal("expected ctx.Err() to be non-nil")
	}
}
