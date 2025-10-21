# **Stage 5 — HTTP Response Writer**

## 🧠 Purpose

You’ve now built the input side of HTTP (request and body parsing).
In this stage you’ll implement the **output side**: a minimal, production-grade **HTTP/1.x response writer** that mirrors the standard library’s behavior.

Your writer will:

* Serialize a status line and headers correctly,
* Select the right transfer encoding (`Content-Length`, `chunked`, or close-delimited),
* Stream the body efficiently,
* Support context cancellation and error propagation.

---

## 📚 Background

Every HTTP response on the wire looks like this:

```
HTTP/1.1 200 OK\r\n
Content-Type: text/plain\r\n
Content-Length: 11\r\n
\r\n
hello world
```

If the body length isn’t known ahead of time, HTTP uses **chunked encoding**:

```
HTTP/1.1 200 OK\r\n
Transfer-Encoding: chunked\r\n
\r\n
4\r\nWiki\r\n
5\r\npedia\r\n
0\r\n\r\n
```

Or, if neither header is set, the body is simply streamed until the connection closes.

Your job: implement a writer that produces exactly these formats.

---

## 🧩 What to implement

### **File**

Create: `internal/httpx/response.go`

### **Types & functions**

1. **`Response` struct**

   ```go
   type Response struct {
       Proto      string // "HTTP/1.1"
       StatusCode int
       Status     string
       Header     Header
       Body       io.Reader // may be nil
   }
   ```

2. **`WriteResponse(ctx, w, resp)`**

   ```go
   func WriteResponse(ctx context.Context, w io.Writer, resp *Response) error
   ```

    * Writes the status line: `HTTP/1.1 200 OK\r\n`
    * Writes headers, each as `Key: Value\r\n`
    * Terminates headers with an extra `\r\n`
    * Chooses one of:

        * **Fixed-length**: if `Content-Length` header exists → copy exactly that many bytes.
        * **Chunked**: if header `Transfer-Encoding: chunked` → wrap body with `chunkedWriter`.
        * **Until close**: otherwise → stream body until EOF.
    * Flushes after headers and after each chunk.
    * Aborts early if `ctx.Done()` fires.

3. **`chunkedWriter`**

   ```go
   type chunkedWriter struct {
       ctx context.Context
       w   *bufio.Writer
   }
   func (cw *chunkedWriter) Write(p []byte) (int, error)
   func (cw *chunkedWriter) Close() error
   ```

    * For every `Write(p)`:

      ```
      <hex length>\r\n
      <data>\r\n
      ```
    * On `Close()`:

      ```
      0\r\n\r\n
      ```
    * Respect context cancellation on every write.

---

## ✅ Passing conditions

You may move to Stage 6 once all these tests pass.

```go
func TestWriteFixedLengthResponse(t *testing.T)
func TestWriteChunkedResponse(t *testing.T)
func TestWriteUntilCloseResponse(t *testing.T)
func TestContextCancelDuringWrite(t *testing.T)
```

Each verifies:

| Behavior           | Expected output                                   |
| ------------------ | ------------------------------------------------- |
| **Fixed length**   | Proper status line + headers + body of exact size |
| **Chunked**        | Proper status + headers + correctly framed chunks |
| **Until close**    | Proper status + headers + raw body (no length)    |
| **Context cancel** | Write stops and returns `ctx.Err()`               |

---

## 🧠 Conceptual tips

* Use `fmt.Fprintf(w, "HTTP/1.1 %d %s\r\n", ...)` for the status line.
* Canonicalize headers via your existing `CanonicalHeaderKey`.
* Terminate header section with one empty line (`\r\n`).
* Wrap `w` in `bufio.NewWriter` for efficiency.
* Use `io.CopyN` or `io.Copy` for body streaming.
* Always check:

  ```go
  select {
  case <-ctx.Done():
      return ctx.Err()
  default:
  }
  ```

  before each write.

---

## 📈 Goal

By the end of Stage 5, you’ll have a fully working `httpx.Response` serializer that mirrors Go’s `net/http` response writer semantics and completes your HTTP/1.x transport layer.