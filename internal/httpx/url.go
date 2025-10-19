package httpx

import (
	"errors"
	"strings"
)

// URL is a minimal representation of a parsed request URI.
type URL struct {
	Scheme   string
	Host     string
	Path     string
	RawQuery string
}

// ParseRequestURI parses the request-target per RFC 7230 §5.3.
//
// Supported forms:
//   - origin-form:   /path?query
//   - absolute-form: http://host/path?query
//   - asterisk-form: * (for OPTIONS *)
func ParseRequestURI(raw string) (*URL, error) {
	if raw == "" {
		return nil, errors.New("empty request-target")
	}
	if strings.ContainsAny(raw, " \r\n") {
		return nil, errors.New("invalid characters in request-target")
	}

	// OPTIONS * form
	if raw == "*" {
		return &URL{Path: "*"}, nil
	}

	u := &URL{}
	switch {
	case strings.HasPrefix(raw, "http://"):
		u.Scheme = "http"
		rest := strings.TrimPrefix(raw, "http://")
		slash := strings.IndexByte(rest, '/')
		if slash == -1 {
			u.Host = strings.ToLower(rest)
			u.Path = "/"
			return u, nil
		}
		u.Host = strings.ToLower(rest[:slash])
		raw = rest[slash:]

	case strings.HasPrefix(raw, "https://"):
		u.Scheme = "https"
		rest := strings.TrimPrefix(raw, "https://")
		slash := strings.IndexByte(rest, '/')
		if slash == -1 {
			u.Host = strings.ToLower(rest)
			u.Path = "/"
			return u, nil
		}
		u.Host = strings.ToLower(rest[:slash])
		raw = rest[slash:]

	default:
		// origin-form (/path?query)
	}

	// Split query
	qmark := strings.IndexByte(raw, '?')
	if qmark >= 0 {
		u.Path = raw[:qmark]
		u.RawQuery = raw[qmark+1:]
	} else {
		u.Path = raw
	}
	if u.Path == "" {
		u.Path = "/"
	}
	return u, nil
}
