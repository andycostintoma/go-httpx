
# 🧩 Stage 1 — Building the CRLF Line Reader

---

## 🧠 Conceptual Background — *Why this matters*

Every HTTP/1.x message (both request and response) is a **sequence of text lines** terminated by **CRLF (`\r\n`)**, followed by an optional body.
When you read from a TCP connection, you don’t get neat lines — you get arbitrary chunks of bytes.

We need a component that can:

* efficiently read up to a line terminator across any number of `Read()` calls,
* handle both `\r\n` (correct) and lone `\n` (tolerated for robustness),
* stop before consuming body bytes,
* enforce safety limits (headers could be maliciously large),
* and integrate cleanly with context cancellation later on.

In the standard library, this logic lives in `net/textproto.Reader`.
We’ll write our own minimal, **production-grade equivalent** called `CRLFFastReader`.
This will power both:

* request parsing on the server side, and
* response parsing on the client side.

---

## 🧩 Implementation Objectives — *What to build now*

### Package layout

```
internal/netx/
    crlfreader.go
    crlfreader_test.go
```

### Core type

```go
type CRLFFastReader struct {
    br   *bufio.Reader  // buffered source for efficient small reads
    size int            // the internal buffer size (for bounds checks)
}
```

We wrap a `bufio.Reader` because it’s efficient and allows us to peek ahead.
We also track the configured buffer size to enforce safe peek limits.

### Required functions

1. **Constructor**

   ```go
   func NewCRLFFastReader(r io.Reader) *CRLFFastReader
   ```

    * Initializes a `bufio.Reader` with a reasonable default buffer (e.g. 4 KB).
    * Returns a pointer to the new struct.

2. **Line Reader**

   ```go
   func (r *CRLFFastReader) ReadLine(max int) (line []byte, isPrefix bool, err error)
   ```

    * Reads bytes until `\n` is found.
    * Trims trailing `\r\n` or bare `\n`.
    * If the line exceeds `max`, return `ErrLineTooLong`.
    * If only part of a line is returned because it exceeded the internal buffer, set `isPrefix = true`.
    * On EOF after partial data, return the data with `err = io.EOF`.

3. **Peeker**

   ```go
   func (r *CRLFFastReader) Peek(n int) ([]byte, error)
   ```

    * Returns the next `n` bytes without advancing the read position.
    * If `n` exceeds `r.size`, return `ErrPeekBeyondCap`.
    * Must never allocate more than its internal buffer.

4. **Error Sentinels**

   ```go
   var (
       ErrLineTooLong     = errors.New("crlf: line too long")
       ErrPeekBeyondCap   = errors.New("crlf: peek beyond internal capacity")
   )
   ```

### Behavioral subtleties

* Empty line (`\r\n`) → return `[]byte{}` with no error.
* `\r` alone should *not* terminate a line.
* The function must **not** strip spaces or modify data except for newline normalization.
* It should handle multiple consecutive calls correctly without skipping or re-reading data.

### Why `isPrefix` exists

`bufio.Reader.ReadLine` uses this pattern when a line is longer than its internal buffer — the caller can detect partial lines and reconstruct them.
We mimic this because the real `net/http` parser expects this exact semantics.

### Why the limits

Real HTTP servers must reject huge lines early to avoid memory exhaustion (DoS).
The `max` argument allows us to enforce per-line size safely before allocating.

---
