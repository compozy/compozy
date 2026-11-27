package apptoken

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeAppOrigin(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "Should lowercase the host", input: "http://LOCALHOST:3000", want: "http://localhost:3000"},
		{
			name:  "Should remove default HTTPS port",
			input: "https://dash.example.com:443",
			want:  "https://dash.example.com",
		},
		{name: "Should remove default HTTP port", input: "http://example.com:80", want: "http://example.com"},
		{name: "Should preserve bracketed IPv6", input: "http://[::1]:3000", want: "http://[::1]:3000"},
		{
			name:  "Should accept the maximum origin length",
			input: "https://" + strings.Repeat("a", 2040),
			want:  "https://" + strings.Repeat("a", 2040),
		},
		{name: "Should reject an oversized origin", input: "https://" + strings.Repeat("a", 2041)},
		{name: "Should reject a path", input: "http://localhost:3000/app"},
		{
			name:  "Should canonicalize a root slash and uppercase scheme",
			input: "HTTP://LocalHost:3000/",
			want:  "http://localhost:3000",
		},
		{name: "Should reject a query", input: "http://x.test?a=1"},
		{name: "Should reject an empty fragment", input: "http://x.test#"},
		{name: "Should reject null", input: "null"},
		{name: "Should reject a wildcard host", input: "https://*.example.com"},
		{name: "Should reject a wildcard", input: "*"},
		{name: "Should reject file URLs", input: "file:///tmp"},
		{name: "Should reject extension URLs", input: "chrome-extension://abc"},
		{name: "Should reject userinfo", input: "https://user@x.test"},
		{name: "Should reject a nonnumeric port", input: "https://x.test:abc"},
		{name: "Should reject an out of range port", input: "https://x.test:65536"},
		{name: "Should reject an empty port", input: "https://x.test:"},
		{name: "Should reject whitespace", input: " https://x.test"},
		{name: "Should reject an invalid IPv6 host", input: "https://[invalid]:3000"},
		{name: "Should reject a bracketed IPv4 host", input: "https://[127.0.0.1]:3000"},
		{name: "Should reject an unbracketed IPv6 host", input: "http://::1:3000"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := NormalizeOrigin(tc.input)
			if tc.want == "" {
				if !errors.Is(err, ErrOriginInvalid) {
					t.Fatalf("NormalizeOrigin(%q) = (%q, %v), want invalid origin", tc.input, got, err)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("NormalizeOrigin(%q) = (%q, %v), want %q", tc.input, got, err, tc.want)
			}
		})
	}
}
