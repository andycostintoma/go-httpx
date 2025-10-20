package httpx

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// -----------------------------------------------------------------------------
// Sentinel errors
// -----------------------------------------------------------------------------
var (
	ErrBodyTooLarge      = errors.New("httpx: body too large")
	ErrBadChunk          = errors.New("httpx: invalid chunk encoding")
	ErrLengthMismatch    = errors.New("httpx: content-length mismatch")
	ErrUnexpectedTrailer = errors.New("httpx: unexpected trailer")
)

// -----------------------------------------------------------------------------
// Public entrypoint
// -----------------------------------------------------------------------------

// NewBodyReader chooses the appropriate reader for the message body based on headers.
//
// It returns an io.ReadCloser representing the body stream and the expected
// Content-Length (if known; otherwise -1).
func NewBodyReader(ctx context.Context, req *Request, r io.Reader, maxSize int64) (io.ReadCloser, int64, error) {
	h := req.Header

	// 1. Transfer-Encoding: chunked
	if strings.EqualFold(h.Get("Transfer-Encoding"), "chunked") {
		return newChunkedReader(ctx, r, maxSize, h), -1, nil
	}

	// 2. Content-Length: fixed-length body
	if cl := h.Get("Content-Length"); cl != "" {
		n, err := strconv.ParseInt(strings.TrimSpace(cl), 10, 64)
		if err != nil || n < 0 {
			return nil, 0, ErrLengthMismatch
		}
		if maxSize > 0 && n > maxSize {
			return nil, 0, ErrBodyTooLarge
		}
		return newFixedReader(ctx, r, n, maxSize), n, nil
	}

	// 3. No length → read until close
	return newCloseReader(ctx, r, maxSize), -1, nil
}

// -----------------------------------------------------------------------------
// fixedReader (Content-Length)
// -----------------------------------------------------------------------------

type fixedReader struct {
	ctx       context.Context
	r         io.Reader
	n         int64 // remaining bytes (Content-Length)
	limit     int64 // global body cap
	readTotal int64
}

func newFixedReader(ctx context.Context, r io.Reader, n, limit int64) io.ReadCloser {
	return &fixedReader{
		ctx:   ctx,
		r:     r,
		n:     n,
		limit: limit,
	}
}

func (f *fixedReader) Read(p []byte) (int, error) {
	select {
	case <-f.ctx.Done():
		return 0, f.ctx.Err()
	default:
	}

	if f.n <= 0 {
		return 0, io.EOF
	}

	// Never read more than remaining bytes.
	if int64(len(p)) > f.n {
		p = p[:f.n]
	}

	n, err := f.r.Read(p)
	f.n -= int64(n)
	f.readTotal += int64(n)

	// Enforce maxSize (global cap).
	if f.limit > 0 && f.readTotal > f.limit {
		return n, ErrBodyTooLarge
	}

	// Short body: hit EOF before expected.
	if err == io.EOF && f.n > 0 {
		return n, ErrLengthMismatch
	}

	// Exactly finished.
	if f.n == 0 {
		return n, io.EOF
	}

	return n, err
}

func (f *fixedReader) Close() error { return nil }

// -----------------------------------------------------------------------------
// chunkedReader (Transfer-Encoding: chunked)
// -----------------------------------------------------------------------------

type chunkState int

const (
	stateChunkHeader chunkState = iota // waiting for "<hex-size>\r\n"
	stateChunkData                     // reading chunk data
	stateChunkCRLF                     // expecting "\r\n" after data
	stateTrailer                       // reading trailers
	stateDone                          // finished
)

type chunkedReader struct {
	ctx       context.Context
	r         *bufio.Reader
	state     chunkState
	remain    int64
	limit     int64
	readTotal int64
	header    Header
}

func newChunkedReader(ctx context.Context, src io.Reader, limit int64, hdr Header) io.ReadCloser {
	return &chunkedReader{
		ctx:    ctx,
		r:      bufio.NewReader(src),
		state:  stateChunkHeader,
		limit:  limit,
		header: hdr,
	}
}

func (c *chunkedReader) Read(p []byte) (int, error) {
	select {
	case <-c.ctx.Done():
		return 0, c.ctx.Err()
	default:
	}

	switch c.state {
	case stateDone:
		return 0, io.EOF

	case stateChunkHeader:
		size, err := c.nextChunkSize()
		if err != nil {
			return 0, err
		}
		if size == 0 {
			c.state = stateTrailer
			return 0, nil
		}
		c.remain = size
		c.state = stateChunkData
		return 0, nil

	case stateChunkData:
		if c.remain <= 0 {
			c.state = stateChunkCRLF
			return 0, nil
		}

		if int64(len(p)) > c.remain {
			p = p[:c.remain]
		}
		n, err := c.r.Read(p)
		c.remain -= int64(n)
		c.readTotal += int64(n)

		if c.limit > 0 && c.readTotal > c.limit {
			return n, ErrBodyTooLarge
		}

		if err != nil {
			return n, err
		}
		if c.remain == 0 {
			c.state = stateChunkCRLF
		}
		return n, nil

	case stateChunkCRLF:
		line, err := c.r.ReadString('\n')
		if err != nil {
			return 0, ErrBadChunk
		}
		if line != "\r\n" {
			return 0, ErrBadChunk
		}
		c.state = stateChunkHeader
		return 0, nil

	case stateTrailer:
		if err := c.readTrailers(); err != nil {
			return 0, err
		}
		c.state = stateDone
		return 0, io.EOF

	default:
		return 0, fmt.Errorf("httpx: invalid chunk reader state %d", c.state)
	}
}

func (c *chunkedReader) Close() error { return nil }

// nextChunkSize parses "<hex-size>\r\n"
func (c *chunkedReader) nextChunkSize() (int64, error) {
	line, err := c.r.ReadString('\n')
	if err != nil {
		return 0, err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return 0, ErrBadChunk
	}

	// ignore chunk extensions ("; name=value")
	if semi := strings.IndexByte(line, ';'); semi >= 0 {
		line = line[:semi]
	}

	size, err := strconv.ParseInt(line, 16, 64)
	if err != nil || size < 0 {
		return 0, ErrBadChunk
	}
	return size, nil
}

// readTrailers parses optional trailer headers after the final 0-sized chunk.
func (c *chunkedReader) readTrailers() error {
	for {
		line, err := c.r.ReadString('\n')
		if err != nil {
			return ErrUnexpectedTrailer
		}
		if line == "\r\n" {
			return nil // blank line terminates trailer section
		}
		line = strings.TrimSuffix(line, "\r\n")
		i := strings.IndexByte(line, ':')
		if i <= 0 {
			return ErrUnexpectedTrailer
		}
		key := CanonicalHeaderKey(line[:i])
		val := strings.TrimSpace(line[i+1:])
		c.header.Add(key, val)
	}
}

// -----------------------------------------------------------------------------
// closeReader (no length → read-until-close)
// -----------------------------------------------------------------------------

type closeReader struct {
	ctx       context.Context
	r         io.Reader
	limit     int64
	readTotal int64
}

func newCloseReader(ctx context.Context, r io.Reader, limit int64) io.ReadCloser {
	return &closeReader{ctx: ctx, r: r, limit: limit}
}

func (c *closeReader) Read(p []byte) (int, error) {
	select {
	case <-c.ctx.Done():
		return 0, c.ctx.Err()
	default:
	}

	if c.limit > 0 {
		remaining := c.limit - c.readTotal
		if remaining <= 0 {
			return 0, io.EOF
		}
		if int64(len(p)) > remaining {
			p = p[:remaining]
		}
	}

	n, err := c.r.Read(p)
	c.readTotal += int64(n)

	if c.limit > 0 && c.readTotal > c.limit {
		return n, ErrBodyTooLarge
	}

	return n, err
}

func (c *closeReader) Close() error { return nil }
