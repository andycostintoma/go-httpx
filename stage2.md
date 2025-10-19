# 🧩 Stage 2 — Header Map with Canonicalization & Limits

---

## 🧠 Why this matters

HTTP headers are a multi-map: one key may have **multiple values**. The stdlib models this as `map[string][]string` and relies on **canonicalized keys** (e.g., `Content-Type`) to ensure case-insensitive lookups and consistent emission on the wire. Accurate canonicalization and strict **safety limits** (field count/size) are critical to prevent parsing ambiguities and DoS via oversized inputs.

In the standard library (`net/http.Header`), the API behaves as follows:

* **Get** returns the **first** value associated with a key (not a join). It’s case-insensitive and assumes keys are stored canonically. ([Go Packages][1])
* **Values** returns **all** values associated with a key; the **returned slice is not a copy** (mutating it affects the header). ([Go Packages][1])
* Keys should be in **canonical MIME header form** (first letter & letters after “-” uppercase, others lowercase), as with `textproto.CanonicalMIMEHeaderKey`. ([Go Packages][1])

You’ll build a `Header` type mirroring stdlib behavior, plus **hardening** helpers to validate header names/values and enforce limits.

---

## 🧱 What to implement (API & structure)

Create/extend package: `internal/httpx`.

### 1) Core type

```go
type Header map[string][]string
```

### 2) Canonicalization

A helper that mirrors stdlib semantics (you may name it as you wish, e.g.):

```go
func CanonicalHeaderKey(s string) string
```

Rules:

* Convert ASCII letters to **Title-Case** chunks separated by `-`.
  Example: `content-type` → `Content-Type`, `HOST` → `Host`.
* Reject invalid bytes for a field-name (enforced during validation; canonicalizer may assume a “likely valid” input).

### 3) Mutators & accessors

Implement **exactly** these methods to mirror stdlib behavior:

```go
func (h Header) Add(key, value string)
func (h Header) Set(key, value string)
func (h Header) Get(key string) string      // returns FIRST value only
func (h Header) Values(key string) []string // returns underlying slice; NOT a copy
func (h Header) Del(key string)
```

Semantics:

* All public methods must treat `key` case-insensitively by canonicalizing it.
* `Get` returns the **first** value or `""` if none (no join). ([Go Packages][1])
* `Values` returns the **original slice** (not a copy). ([Go Packages][1])
* `Set` replaces all existing values for `key` with exactly one element.

### 4) Limits & validation

Define a limits struct and validation function:

```go
type HeaderLimits struct {
    MaxFields          int   // maximum distinct header keys allowed
    MaxKeyBytes        int   // maximum length of a single header field-name (bytes)
    MaxValueBytes      int   // maximum length of a single header field-value (bytes)
    MaxTotalValuesBytes int  // cap on sum of all value lengths (optional hard cap)
}

func ValidateHeader(h Header, lim HeaderLimits) error
```

Validation requirements (keep tight but practical):

* **Count**: `len(h)` must not exceed `MaxFields`.
* **Field-name** chars: only RFC 7230 tchar set is allowed (`A–Z a–z 0–9 ! # $ % & ' * + - . ^ _ ` | ~`). Spaces, control chars, or `:` are **invalid**.
* **Field-name** length ≤ `MaxKeyBytes`.
* **Each value** must be ASCII without CTLs except **HTAB**; length ≤ `MaxValueBytes`.
* If `MaxTotalValuesBytes > 0`, the sum of lengths across all values must not exceed it.

> Note: This stage is about **data structure correctness and safety**. Wire formatting (`Write`, `WriteSubset`) and parser integration come later.

---



