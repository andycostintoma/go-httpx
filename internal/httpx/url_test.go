package httpx

import "testing"

func TestParseRequestURI_OriginForm(t *testing.T) {
	u, err := ParseRequestURI("/index.html?x=1")
	if err != nil {
		t.Fatal(err)
	}
	if u.Scheme != "" || u.Host != "" {
		t.Fatalf("unexpected scheme/host: %+v", u)
	}
	if u.Path != "/index.html" || u.RawQuery != "x=1" {
		t.Fatalf("wrong origin-form parse: %+v", u)
	}
}

func TestParseRequestURI_AbsoluteForm(t *testing.T) {
	cases := []struct {
		raw, wantScheme, wantHost, wantPath, wantQuery string
	}{
		{"http://example.com/a/b?y=2", "http", "example.com", "/a/b", "y=2"},
		{"https://foo/bar", "https", "foo", "/bar", ""},
		{"http://example.com", "http", "example.com", "/", ""},
	}
	for _, c := range cases {
		u, err := ParseRequestURI(c.raw)
		if err != nil {
			t.Fatalf("parse %q: %v", c.raw, err)
		}
		if u.Scheme != c.wantScheme || u.Host != c.wantHost ||
			u.Path != c.wantPath || u.RawQuery != c.wantQuery {
			t.Fatalf("%q → got %+v", c.raw, u)
		}
	}
}

func TestParseRequestURI_AsteriskForm(t *testing.T) {
	u, err := ParseRequestURI("*")
	if err != nil {
		t.Fatal(err)
	}
	if u.Path != "*" {
		t.Fatalf("expected * path, got %q", u.Path)
	}
}

func TestParseRequestURI_Invalid(t *testing.T) {
	cases := []string{
		"",
		" bad",
		"/path with space",
		"http://exa mple.com/",
	}
	for _, raw := range cases {
		if _, err := ParseRequestURI(raw); err == nil {
			t.Fatalf("expected error for %q", raw)
		}
	}
}
