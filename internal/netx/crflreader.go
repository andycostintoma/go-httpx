package netx

import (
	"bufio"
	"errors"
	"io"
)

// ErrLineTooLong indicates that a line exceeded the configured maximum length.
var ErrLineTooLong = errors.New("crlf: line too long")

// ErrPeekBeyondCap indicates an attempt to peek beyond the internal buffer capacity.
var ErrPeekBeyondCap = errors.New("crlf: peek beyond internal capacity")

// DefaultBufSize defines the buffer size used by NewCRLFFastReader.
const DefaultBufSize = 4096

// CRLFFastReader provides efficient, safe CRLF line reading semantics for HTTP parsing.
// It behaves similarly to net/textproto.Reader, enforcing hard caps and RFC-compliant trimming.
type CRLFFastReader struct {
	br      *bufio.Reader // buffered source for efficient small reads
	bufSize int           // internal buffer size (for bounds checks)
}

// NewCRLFFastReader wraps r with a buffered reader of DefaultBufSize.
func NewCRLFFastReader(r io.Reader) *CRLFFastReader {
	return &CRLFFastReader{
		br:      bufio.NewReaderSize(r, DefaultBufSize),
		bufSize: DefaultBufSize,
	}
}

// Reset allows reusing the reader with a new underlying source.
func (r *CRLFFastReader) Reset(src io.Reader) {
	if r.br == nil {
		r.br = bufio.NewReaderSize(src, DefaultBufSize)
		r.bufSize = DefaultBufSize
		return
	}
	r.br.Reset(src)
}

// ReadLine reads a single logical line, trimming the trailing CRLF or LF.
//
// It enforces a maximum total line length (max). If the accumulated line exceeds
// that limit, it returns ErrLineTooLong. The isPrefix flag mirrors bufio.Reader.ReadLine
// semantics: true means the internal buffer filled before a newline was found.
func (r *CRLFFastReader) ReadLine(max int) (line []byte, isPrefix bool, err error) {
	if max <= 0 {
		return nil, false, errors.New("crlf: invalid max value")
	}

	var buf []byte
	for {
		part, perr := r.br.ReadSlice('\n')
		// enforce limit before appending large chunks
		if len(buf)+len(part) > max {
			return nil, true, ErrLineTooLong
		}
		buf = append(buf, part...)

		switch {
		case perr == nil:
			// found newline
			n := len(buf)
			if n > 0 && buf[n-1] == '\n' {
				n--
				if n > 0 && buf[n-1] == '\r' {
					n--
				}
			}
			return buf[:n], false, nil

		case errors.Is(perr, bufio.ErrBufferFull):
			// continue accumulating until newline found or max exceeded
			continue

		case errors.Is(perr, io.EOF):
			if len(buf) == 0 {
				return nil, false, io.EOF
			}
			return buf, false, io.EOF

		default:
			return buf, false, perr
		}
	}
}

// Peek returns the next n bytes without advancing the reader.
//
// The returned slice is backed by the internal buffer and must not be modified.
// If n exceeds the buffer size or cannot be satisfied without growing it,
// ErrPeekBeyondCap is returned.
func (r *CRLFFastReader) Peek(n int) ([]byte, error) {
	if n > r.bufSize {
		return nil, ErrPeekBeyondCap
	}
	b, err := r.br.Peek(n)
	if err != nil && errors.Is(err, bufio.ErrBufferFull) {
		return nil, ErrPeekBeyondCap
	}
	return b, err
}
