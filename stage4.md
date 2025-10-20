# 🧩 Stage 4 — Body Readers (fixed length, chunked, and close-delimited)

---

## 🧠 why this matters

after parsing the request line and headers, the next bytes on the connection belong to the message body.
how we decide *where the body ends* depends on the headers:

| Situation                                  | How to read                      |
| ------------------------------------------ | -------------------------------- |
| `Content-Length: N`                        | exactly `N` bytes                |
| `Transfer-Encoding: chunked`               | RFC 7230 § 4.1 chunked format    |
| neither (and not `CONNECT`)                | read until the connection closes |
| conflicting (`Content-Length` + `chunked`) | must reject                      |

a robust body reader must:

* enforce size limits,
* support trailers (`Trailer:` headers appended after the last chunk),
* unblock on context cancellation,
* detect premature EOFs and mismatched lengths.

---

## 🧩 what to implement

### 🗂 file layout

```
internal/httpx/
    body.go
    body_test.go
```

---

### 1️⃣ define error sentinels

```go
var (
    ErrBodyTooLarge     = errors.New("httpx: body too large")
    ErrBadChunk         = errors.New("httpx: invalid chunk encoding")
    ErrLengthMismatch   = errors.New("httpx: content-length mismatch")
    ErrUnexpectedTrailer = errors.New("httpx: unexpected trailer")
)
```

---

### 2️⃣ the entry function

```go
func NewBodyReader(ctx context.Context, req *Request, r io.Reader, maxSize int64) (io.ReadCloser, int64, error)
```

* picks the correct strategy based on headers:

    * if `Transfer-Encoding: chunked` → return a `chunkedReader`
    * else if `Content-Length` present → return a `fixedReader`
    * else → return a `closeReader`
* enforce `maxSize` (total bytes allowed across the entire body).
* on error, wrap with context info (e.g., `"body decode: %w"`).

---

### 3️⃣ implement the concrete readers

#### **fixedReader**

```go
type fixedReader struct {
    r     io.Reader
    n     int64 // remaining bytes
    limit int64
}
```

* read up to `n` bytes; return `io.EOF` when exhausted.
* if underlying reader ends early → `ErrLengthMismatch`.
* enforce `limit` if non-zero.

#### **chunkedReader**

```go
type chunkedReader struct {
    r      *bufio.Reader
    remain int64
    done   bool
    limit  int64
    header Header // optional: to attach trailers
}
```

Behavior:

* read `<hex-size>\r\n<data>\r\n` repeatedly.
* when size==0 → read optional trailer section (until `\r\n\r\n`).
* total size ≤ `limit`.
* invalid chunk size or missing CRLF → `ErrBadChunk`.
* optional: merge trailers into `req.Header` if they match declared `Trailer:` keys.

#### **closeReader**

Wraps a raw `io.Reader` that reads until EOF, optionally enforcing a `limit`.

---

### 4️⃣ context cancellation

every reader must select on `ctx.Done()` inside `Read(p)` loops:

```go
select {
case <-ctx.Done():
    return 0, ctx.Err()
default:
}
```

this ensures long uploads/downloads abort promptly.

---

### 5️⃣ integrate into `Request`

you don’t need to modify `Request` yet, but you will use:

```go
req.Body, req.ContentLength, err = NewBodyReader(req.Context(), req, r, limits.MaxBodyBytes)
```

in a later stage when parsing full messages.

