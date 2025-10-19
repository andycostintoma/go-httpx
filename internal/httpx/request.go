package httpx

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/andycostintoma/httpx/internal/netx"
)

// requestLine models the first line of an HTTP/1.x request.
type requestLine struct {
	Method     string
	RequestURI string
	Proto      string
	ProtoMajor int
	ProtoMinor int
}

// String returns the serialized form of the request line.
func (r requestLine) String() string {
	return fmt.Sprintf("%s %s %s", r.Method, r.RequestURI, r.Proto)
}

// Request represents a parsed HTTP/1.x request.
// Body handling and header parsing are added in Stage 4.
type Request struct {
	requestLine
	URL           *URL
	Header        Header
	Host          string
	ContentLength int64
	Body          io.ReadCloser
	ctx           context.Context
}

// ParseLimits controls how many bytes can be read from a request line or headers.
type ParseLimits struct {
	MaxLineBytes   int
	MaxHeaderBytes int
}

// ParseRequest reads and parses the request line from r.
// Headers and body are ignored at this stage.
func ParseRequest(r *netx.CRLFFastReader, limits ParseLimits) (*Request, error) {
	line, _, err := r.ReadLine(limits.MaxLineBytes)
	if err != nil {
		return nil, fmt.Errorf("read request line: %w", err)
	}
	if len(line) == 0 {
		return nil, errors.New("empty request line")
	}

	rl, err := parseRequestLine(string(line))
	if err != nil {
		return nil, err
	}

	u, err := ParseRequestURI(rl.RequestURI)
	if err != nil {
		return nil, err
	}

	req := &Request{
		requestLine: rl,
		URL:         u,
		Header:      make(Header),
		ctx:         context.Background(),
	}

	// For now, Host comes from URL if absolute-form.
	if u.Host != "" {
		req.Host = strings.ToLower(u.Host)
	}

	return req, nil
}

// parseRequestWithContext is the context-aware variant used in later stages.
func parseRequestWithContext(ctx context.Context, r *netx.CRLFFastReader, limits ParseLimits) (*Request, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	req, err := ParseRequest(r, limits)
	if err != nil {
		return nil, err
	}
	req.ctx = ctx
	return req, nil
}

// parseRequestLine parses "METHOD SP Request-URI SP HTTP/x.y".
func parseRequestLine(line string) (rl requestLine, err error) {
	// Be tolerant of multiple spaces or tabs.
	parts := strings.Fields(line)
	if len(parts) != 3 {
		return rl, fmt.Errorf("malformed request line: %q", line)
	}

	method := parts[0]
	target := parts[1]
	proto := parts[2]

	if len(method) == 0 || len(method) > 20 {
		return rl, fmt.Errorf("invalid method: %q", method)
	}
	for _, c := range method {
		if c < 'A' || c > 'Z' {
			return rl, fmt.Errorf("method must be uppercase A–Z: %q", method)
		}
	}

	if !strings.HasPrefix(proto, "HTTP/") {
		return rl, fmt.Errorf("invalid protocol: %q", proto)
	}
	ver := strings.TrimPrefix(proto, "HTTP/")
	dot := strings.IndexByte(ver, '.')
	if dot < 0 {
		return rl, fmt.Errorf("invalid HTTP version: %q", proto)
	}
	major, err1 := strconv.Atoi(ver[:dot])
	minor, err2 := strconv.Atoi(ver[dot+1:])
	if err1 != nil || err2 != nil {
		return rl, fmt.Errorf("invalid HTTP version numbers: %q", proto)
	}

	rl = requestLine{
		Method:     method,
		RequestURI: target,
		Proto:      proto,
		ProtoMajor: major,
		ProtoMinor: minor,
	}
	return rl, nil
}

// Context returns the request's context.
func (r *Request) Context() context.Context {
	if r == nil || r.ctx == nil {
		return context.Background()
	}
	return r.ctx
}

// WithContext returns a shallow copy of r with its context replaced by ctx.
func (r *Request) WithContext(ctx context.Context) *Request {
	if r == nil {
		return nil
	}
	cp := *r
	cp.ctx = ctx
	return &cp
}

// String returns a human-readable representation of the request line.
func (r *Request) String() string {
	if r == nil {
		return "<nil request>"
	}
	return r.requestLine.String()
}
