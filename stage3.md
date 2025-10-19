# 🧩 Stage 3 — Request Line & URI Parsing

---

## 🧠 Why this matters

The first line of an HTTP/1.1 request looks like this:

```
METHOD SP request-target SP HTTP/1.1\r\n
```

Example:

```
GET /search?q=go HTTP/1.1\r\n
```

Your server will receive this line first — before any headers or body — and you must parse it safely and accurately.

The stdlib’s `net/http` uses a combination of:

* `textproto.Reader.ReadLine()` to read it
* a custom tokenizer to split it into parts
* `net/url` to parse the URI
  We’ll build our own minimal equivalents — faster, smaller, and testable.

You’ll need to produce:

* a `Request` struct holding the method, URL, protocol, and headers,
* a lightweight RFC3986 URL parser (`/internal/httpx/url.go`),
* and a parser function that reads and validates the first line from your `CRLFFastReader`.

---

## 🧩 What you’ll build

### 🗂 File layout

```
internal/httpx/
    request.go
    url.go
    parse_request_test.go
```

---

### 1️⃣ Define the Request struct

```go
type Request struct {
	Method        string
	URL           *URL
	Proto         string // e.g. "HTTP/1.1"
	ProtoMajor    int
	ProtoMinor    int
	Header        Header
	Host          string
	ContentLength int64
	Body          io.ReadCloser
	ctx           context.Context
}
```

For now, `Body` and `ContentLength` may stay zero/`nil` — you’ll wire them up in Stage 4.

---

### 2️⃣ Parse the request line

Create a function:

```go
func ParseRequestLine(line string) (method, target, proto string, major, minor int, err error)
```

Requirements:

* Split by spaces — must yield exactly 3 tokens.
* Validate `method`: uppercase letters, length ≤ 20.
* Validate `proto`:

    * must start with `"HTTP/"`,
    * followed by digits like `1.0`, `1.1`, `2.0`, etc.
    * parse into `major`, `minor` ints.
* Return structured parts or error.

---

### 3️⃣ Build a minimal URL parser

You’ll need a small struct for parsed URLs:

```go
type URL struct {
	Scheme   string
	Host     string
	Path     string
	RawQuery string
}
```

Implement:

```go
func ParseRequestURI(raw string) (*URL, error)
```

Rules:

* If `raw` starts with `/`, it’s an **origin-form** URI (typical request: `/path?x=1`).
* If it starts with `http://` or `https://`, parse as absolute URI: extract scheme, host, path, query.
* If it’s `*`, accept only for `OPTIONS * HTTP/1.1`.
* Do **not** allow spaces, `\r`, or `\n` in the URI.
* Split query string at the first `?`.
* Do not unescape yet (keep percent-encoded).

---

### 4️⃣ Put it together

Implement:

```go
func ParseRequest(r *netx.CRLFFastReader, limits ParseLimits) (*Request, error)
```

Where `ParseLimits` is:

```go
type ParseLimits struct {
	MaxLineBytes int // maximum allowed for request line
	MaxHeaderBytes int
}
```

Algorithm:

1. Read the first line with `r.ReadLine(limits.MaxLineBytes)`.
2. Parse it with `ParseRequestLine`.
3. Build `Request{Method, Proto, ProtoMajor, ProtoMinor, URL}`.
4. Initialize an empty `Header{}` — Stage 4 will fill it.
5. For `Host`:

    * If URL.Host is set, use that.
    * Otherwise, you’ll extract it later from headers (`Host:`).

Return the `*Request` and error if any parsing fails.

---

### 5️⃣ Context variant (for Stage 6 compatibility)

For later cancellation support, also stub this helper:

```go
func parseRequestWithContext(ctx context.Context, r *netx.CRLFFastReader, limits ParseLimits) (*Request, error)
```

It should:

* Immediately return `ctx.Err()` if cancelled,
* Otherwise just call `ParseRequest`.

---
