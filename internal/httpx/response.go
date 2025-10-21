package httpx

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Response represents a minimal HTTP/1.x response to serialize.
type Response struct {
	Proto      string    // e.g. "HTTP/1.1" (defaults to "HTTP/1.1" if empty)
	StatusCode int       // e.g. 200
	Status     string    // e.g. "OK"
	Header     Header    // response headers
	Body       io.Reader // may be nil
}

// WriteResponse serializes an HTTP/1.x response (status line, headers, body).
// It selects transfer semantics by inspecting headers:
//   - Content-Length present -> write exactly that many bytes
//   - Transfer-Encoding: chunked -> write chunked body
//   - else -> stream until EOF (caller manages connection close semantics)
func WriteResponse(ctx context.Context, w io.Writer, resp *Response) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	bw := bufio.NewWriter(w)

	proto := resp.Proto
	if proto == "" {
		proto = "HTTP/1.1"
	}
	if resp.Status == "" {
		// best-effort: if missing, synthesize "NNN"
		resp.Status = strconv.Itoa(resp.StatusCode)
	}

	// Status line: "HTTP/1.1 200 OK\r\n"
	if _, err := bw.WriteString(fmt.Sprintf("%s %d %s\r\n", proto, resp.StatusCode, resp.Status)); err != nil {
		return err
	}

	// Emit headers (each value on its own line).
	for k, vals := range resp.Header {
		ck := CanonicalHeaderKey(k)
		for _, v := range vals {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			if _, err := bw.WriteString(ck + ": " + v + "\r\n"); err != nil {
				return err
			}
		}
	}

	// End of header section.
	if _, err := bw.WriteString("\r\n"); err != nil {
		return err
	}
	if err := bw.Flush(); err != nil {
		return err
	}

	// If no body, we're done.
	if resp.Body == nil {
		return nil
	}

	// Body strategy:
	if clStr := resp.Header.Get("Content-Length"); clStr != "" {
		// Fixed length
		n, err := strconv.ParseInt(strings.TrimSpace(clStr), 10, 64)
		if err != nil || n < 0 {
			return ErrLengthMismatch
		}
		// copy exactly N bytes
		if _, err := io.CopyN(bw, resp.Body, n); err != nil {
			return err
		}
		return bw.Flush()
	}

	if strings.EqualFold(resp.Header.Get("Transfer-Encoding"), "chunked") {
		// Chunked writer
		cw := newChunkedWriter(ctx, bw)
		// Stream body in reasonable chunks; io.Copy will call Write on cw.
		if _, err := io.Copy(cw, resp.Body); err != nil {
			_ = cw.Close() // attempt to close trailer even on error
			return err
		}
		if err := cw.Close(); err != nil {
			return err
		}
		return bw.Flush()
	}

	// Until-close: just stream everything.
	if _, err := io.Copy(bw, resp.Body); err != nil {
		return err
	}
	return bw.Flush()
}

// -----------------------------------------------------------------------------
// chunkedWriter: mirror of chunked transfer encoding (writer side)
// -----------------------------------------------------------------------------

type chunkedWriter struct {
	ctx context.Context
	w   *bufio.Writer
}

func newChunkedWriter(ctx context.Context, w *bufio.Writer) *chunkedWriter {
	return &chunkedWriter{ctx: ctx, w: w}
}

// Write emits one chunk for p: "<hex>\r\n<p>\r\n".
// A Write with len(p)==0 is a no-op (final "0\r\n\r\n" is written by Close).
func (cw *chunkedWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	select {
	case <-cw.ctx.Done():
		return 0, cw.ctx.Err()
	default:
	}

	// chunk size line
	if _, err := cw.w.WriteString(strconv.FormatInt(int64(len(p)), 16)); err != nil {
		return 0, err
	}
	if _, err := cw.w.WriteString("\r\n"); err != nil {
		return 0, err
	}

	// data
	n, err := cw.w.Write(p)
	if err != nil {
		return n, err
	}

	// trailing CRLF
	if _, err := cw.w.WriteString("\r\n"); err != nil {
		return n, err
	}
	return n, nil
}

// Close writes the terminating zero-sized chunk: "0\r\n\r\n".
func (cw *chunkedWriter) Close() error {
	select {
	case <-cw.ctx.Done():
		return cw.ctx.Err()
	default:
	}
	if _, err := cw.w.WriteString("0\r\n\r\n"); err != nil {
		return err
	}
	return nil
}
