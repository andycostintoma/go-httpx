package httpx

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"
)

type Header map[string][]string

// Sentinel errors for higher-level handling.
var (
	ErrInvalidFieldName    = errors.New("httpx: invalid header field name")
	ErrInvalidValue        = errors.New("httpx: invalid header value")
	ErrHeaderTooLarge      = errors.New("httpx: too many header fields")
	ErrKeyTooLarge         = errors.New("httpx: header key too long")
	ErrValueTooLarge       = errors.New("httpx: header value too long")
	ErrTotalValuesTooLarge = errors.New("httpx: total header values too large")
)

// CanonicalHeaderKey returns the canonical format of the HTTP header key,
// identical to textproto.CanonicalMIMEHeaderKey from the stdlib.
func CanonicalHeaderKey(s string) string {
	if s == "" {
		return ""
	}
	parts := strings.Split(s, "-")
	for i, p := range parts {
		if p == "" {
			continue
		}
		runes := []rune(p)
		runes[0] = unicode.ToUpper(runes[0])
		for j := 1; j < len(runes); j++ {
			runes[j] = unicode.ToLower(runes[j])
		}
		parts[i] = string(runes)
	}
	return strings.Join(parts, "-")
}

// Add appends a value to the header key, canonicalizing the key first.
func (h Header) Add(key, value string) {
	k := CanonicalHeaderKey(key)
	h[k] = append(h[k], value)
}

// Set replaces any existing values for key with a single value.
func (h Header) Set(key, value string) {
	k := CanonicalHeaderKey(key)
	h[k] = []string{value}
}

// Get returns the first value associated with key, or "" if none.
func (h Header) Get(key string) string {
	k := CanonicalHeaderKey(key)
	if v, ok := h[k]; ok && len(v) > 0 {
		return v[0]
	}
	return ""
}

// Values returns all values associated with key (the original slice, not a copy).
func (h Header) Values(key string) []string {
	return h[CanonicalHeaderKey(key)]
}

// Del deletes the header key (case-insensitive).
func (h Header) Del(key string) {
	delete(h, CanonicalHeaderKey(key))
}

// Clone returns a deep copy of the header map.
// Used by client and server to duplicate headers safely.
func (h Header) Clone() Header {
	if h == nil {
		return nil
	}
	c := make(Header, len(h))
	for k, v := range h {
		vv := make([]string, len(v))
		copy(vv, v)
		c[k] = vv
	}
	return c
}

// Write serializes headers to wire format: "Key: Value\r\n...".
func (h Header) Write(w io.Writer) error {
	for k, vals := range h {
		for _, v := range vals {
			if _, err := fmt.Fprintf(w, "%s: %s\r\n", k, v); err != nil {
				return err
			}
		}
	}
	_, err := io.WriteString(w, "\r\n")
	return err
}

// -----------------------------------------------------------------------------
// Validation
// -----------------------------------------------------------------------------

type HeaderLimits struct {
	MaxFields           int // maximum distinct header keys allowed
	MaxKeyBytes         int // maximum length of a single header field-name (bytes)
	MaxValueBytes       int // maximum length of a single header field-value (bytes)
	MaxTotalValuesBytes int // cap on sum of all value lengths (optional hard cap)
}

// isValidFieldName reports whether s is a valid HTTP header field name per RFC 7230 §3.2.6.
// Allowed characters: A–Z a–z 0–9 ! # $ % & ' * + - . ^ _ ` | ~
func isValidFieldName(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z',
			c >= 'a' && c <= 'z',
			c >= '0' && c <= '9',
			c == '!', c == '#', c == '$', c == '%', c == '&', c == '\'',
			c == '*', c == '+', c == '-', c == '.', c == '^', c == '_',
			c == '`', c == '|', c == '~':
			continue
		default:
			return false
		}
	}
	return true
}

// isValidValue checks that a value contains only printable ASCII or HTAB,
// per RFC 7230 §3.2.6 (no CTL except HTAB).
func isValidValue(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\t' {
			continue
		}
		if c < 32 || c == 127 {
			return false
		}
	}
	return true
}

// ValidateHeader enforces field counts, key/value size limits, and valid chars.
func ValidateHeader(h Header, lim HeaderLimits) error {
	if lim.MaxFields > 0 && len(h) > lim.MaxFields {
		return fmt.Errorf("%w: %d fields", ErrHeaderTooLarge, len(h))
	}

	totalBytes := 0
	for k, vals := range h {
		if !isValidFieldName(k) {
			return fmt.Errorf("%w: %q", ErrInvalidFieldName, k)
		}
		if lim.MaxKeyBytes > 0 && len(k) > lim.MaxKeyBytes {
			return fmt.Errorf("%w: %s", ErrKeyTooLarge, k)
		}
		for _, v := range vals {
			if lim.MaxValueBytes > 0 && len(v) > lim.MaxValueBytes {
				return fmt.Errorf("%w: %s", ErrValueTooLarge, k)
			}
			if !isValidValue(v) {
				return fmt.Errorf("%w: %q", ErrInvalidValue, v)
			}
			totalBytes += len(v)
		}
	}
	if lim.MaxTotalValuesBytes > 0 && totalBytes > lim.MaxTotalValuesBytes {
		return fmt.Errorf("%w: %d bytes", ErrTotalValuesTooLarge, totalBytes)
	}
	return nil
}
