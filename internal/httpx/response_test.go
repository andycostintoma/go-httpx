package httpx

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
)

// helper to normalize CRLFs in assertions if needed (kept simple here)
func mustEqual(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Fatalf("mismatch:\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}

// A reader that returns provided chunks one-by-one on successive Read calls.
// Used to get deterministic chunk sizes in tests.
type splitReader struct {
	chunks [][]byte
	i      int
}

func (s *splitReader) Read(p []byte) (int, error) {
	if s.i >= len(s.chunks) {
		return 0, io.EOF
	}
	ch := s.chunks[s.i]
	s.i++
	n := copy(p, ch)
	return n, nil
}

func TestWriteFixedLengthResponse(t *testing.T) {
	var buf bytes.Buffer

	resp := &Response{
		Proto:      "HTTP/1.1",
		StatusCode: 200,
		Status:     "OK",
		Header:     Header{},
		Body:       strings.NewReader("hello world"),
	}
	resp.Header.Set("Content-Type", "text/plain")
	resp.Header.Set("Content-Length", "11")

	if err := WriteResponse(context.Background(), &buf, resp); err != nil {
		t.Fatal(err)
	}

	got := buf.String()

	// check status line
	if !strings.HasPrefix(got, "HTTP/1.1 200 OK\r\n") {
		t.Fatalf("bad status line: %q", got)
	}

	// check that both headers exist (order not important)
	if !strings.Contains(got, "Content-Type: text/plain\r\n") {
		t.Fatalf("missing Content-Type header in:\n%s", got)
	}
	if !strings.Contains(got, "Content-Length: 11\r\n") {
		t.Fatalf("missing Content-Length header in:\n%s", got)
	}

	// ensure correct body after header section
	if !strings.HasSuffix(got, "\r\n\r\nhello world") {
		t.Fatalf("body missing or malformed, got:\n%s", got)
	}
}

func TestWriteChunkedResponse(t *testing.T) {
	var buf bytes.Buffer

	body := &splitReader{
		chunks: [][]byte{
			[]byte("Wiki"),
			[]byte("pedia"),
		},
	}

	resp := &Response{
		Proto:      "HTTP/1.1",
		StatusCode: 200,
		Status:     "OK",
		Header:     Header{},
		Body:       body,
	}
	resp.Header.Set("Transfer-Encoding", "chunked")

	if err := WriteResponse(context.Background(), &buf, resp); err != nil {
		t.Fatal(err)
	}

	want := "" +
		"HTTP/1.1 200 OK\r\n" +
		"Transfer-Encoding: chunked\r\n" +
		"\r\n" +
		"4\r\nWiki\r\n" +
		"5\r\npedia\r\n" +
		"0\r\n\r\n"
	mustEqual(t, buf.String(), want)
}

func TestWriteUntilCloseResponse(t *testing.T) {
	var buf bytes.Buffer

	resp := &Response{
		Proto:      "HTTP/1.1",
		StatusCode: 200,
		Status:     "OK",
		Header:     Header{},
		Body:       strings.NewReader("abc"),
	}
	resp.Header.Set("Content-Type", "text/plain")
	// No Content-Length, no Transfer-Encoding => until-close

	if err := WriteResponse(context.Background(), &buf, resp); err != nil {
		t.Fatal(err)
	}

	wantPrefix := "" +
		"HTTP/1.1 200 OK\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n"
	got := buf.String()
	if !strings.HasPrefix(got, wantPrefix) {
		t.Fatalf("headers mismatch:\n--- got ---\n%q\n--- want prefix ---\n%q", got, wantPrefix)
	}
	if got[len(wantPrefix):] != "abc" {
		t.Fatalf("body mismatch: got %q, want %q", got[len(wantPrefix):], "abc")
	}
}

func TestContextCancelDuringWrite(t *testing.T) {
	var buf bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before writing

	resp := &Response{
		StatusCode: 200,
		Status:     "OK",
		Header:     Header{},
		Body:       strings.NewReader("should-not-write"),
	}

	err := WriteResponse(ctx, &buf, resp)
	if err == nil {
		t.Fatal("expected context error, got nil")
	}
	if ctx.Err() == nil {
		t.Fatalf("expected ctx.Err() to be non-nil")
	}
}
