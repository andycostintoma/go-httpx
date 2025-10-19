package httpx

import "testing"

func TestHeaderCanonicalAndAddSetGet(t *testing.T) {
	h := Header{}
	h.Add("content-type", "text/plain")
	h.Add("Content-Type", "charset=utf-8")
	h.Add("HOST", "example.com")
	h.Set("x-powered-by", "go")

	// Keys must be stored/accessible in canonical form.
	if got := h.Get("CONTENT-TYPE"); got != "text/plain" { // FIRST value only
		t.Fatalf("Get(Content-Type) = %q, want %q", got, "text/plain")
	}
	if got := h.Get("host"); got != "example.com" {
		t.Fatalf("Get(Host) = %q", got)
	}
	// Set replaces previous values.
	h.Set("X-Powered-By", "rust? no, go")
	if got := h.Get("x-powered-by"); got != "rust? no, go" {
		t.Fatalf("Get after Set = %q", got)
	}
}

func TestHeaderValuesAndDel(t *testing.T) {
	h := Header{}
	h.Add("Accept", "text/html")
	h.Add("ACCEPT", "application/json")

	vals := h.Values("accept")
	if len(vals) != 2 || vals[0] != "text/html" || vals[1] != "application/json" {
		t.Fatalf("Values = %#v", vals)
	}

	// Values must NOT be a copy (mutations reflect in map),
	// mirroring stdlib's documented behavior.
	vals[0] = "text/plain"
	if got := h.Values("Accept")[0]; got != "text/plain" {
		t.Fatalf("Values slice should reflect underlying map change, got %q", got)
	}

	h.Del("ACCEPT")
	if got := len(h.Values("Accept")); got != 0 {
		t.Fatalf("Del failed, still %d values", got)
	}
}

func TestHeaderValidationLimits(t *testing.T) {
	h := Header{}
	// Prepare many fields quickly.
	for i := 0; i < 5; i++ {
		h.Add("X-K"+string(rune('A'+i)), "v")
	}
	lim := HeaderLimits{
		MaxFields:           4,
		MaxKeyBytes:         32,
		MaxValueBytes:       8,
		MaxTotalValuesBytes: 32,
	}
	if err := ValidateHeader(h, lim); err == nil {
		t.Fatal("expected error for too many fields")
	}

	// Invalid name (space) should fail.
	h = Header{"Bad Name": {"v"}}
	if err := ValidateHeader(h, lim); err == nil {
		t.Fatal("expected invalid field-name error")
	}

	// Invalid value (control characters other than HTAB).
	h = Header{"X-K": {"ok\tbut\u0007bell"}} // \a is control char → invalid
	if err := ValidateHeader(h, lim); err == nil {
		t.Fatal("expected invalid value error")
	}

	// Value too long.
	h = Header{"X-K": {"123456789"}} // 9 bytes > MaxValueBytes(8)
	if err := ValidateHeader(h, lim); err == nil {
		t.Fatal("expected value too long error")
	}

	// Sum of values too large.
	h = Header{"A": {"12345678"}, "B": {"12345678"}, "C": {"1"}}
	// total = 8+8+1 = 17 > MaxTotalValuesBytes(16) when set so:
	lim.MaxTotalValuesBytes = 16
	if err := ValidateHeader(h, lim); err == nil {
		t.Fatal("expected total values size error")
	}

	// Valid case.
	h = Header{"Content-Type": {"text/plain"}, "Host": {"ex.com"}}
	lim = HeaderLimits{MaxFields: 8, MaxKeyBytes: 64, MaxValueBytes: 64, MaxTotalValuesBytes: 0}
	if err := ValidateHeader(h, lim); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestCanonicalHeaderKeyBehavior(t *testing.T) {
	// Your CanonicalHeaderKey must match stdlib's semantics.
	cases := map[string]string{
		"content-type": "Content-Type",
		"HOST":         "Host",
		"etag":         "Etag",
		"x-custom-id":  "X-Custom-Id",
		"r":            "R",
		"":             "",
	}
	for in, want := range cases {
		if got := CanonicalHeaderKey(in); got != want {
			t.Fatalf("CanonicalHeaderKey(%q)=%q, want %q", in, got, want)
		}
	}
}
