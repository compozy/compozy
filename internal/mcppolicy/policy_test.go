package mcppolicy

import (
	"errors"
	"testing"
)

func TestValidateHeaders(t *testing.T) {
	t.Parallel()

	t.Run("Should accept ordinary fixed and operator headers", func(t *testing.T) {
		t.Parallel()

		err := ValidateHeaders(
			map[string]string{"X-Tenant": "public"},
			map[string]string{"Authorization": "vault://tenant/token"},
			SourceOperatorSecret,
			false,
		)
		if err != nil {
			t.Fatalf("ValidateHeaders() error = %v", err)
		}
	})

	tests := []struct {
		name        string
		fixed       map[string]string
		secret      map[string]string
		source      HeaderSource
		authEnabled bool
		wantCode    Code
	}{
		{
			name:     "Should reject content type for packages",
			fixed:    map[string]string{"Content-Type": "application/json"},
			source:   SourcePackageFixed,
			wantCode: CodeReservedHeader,
		},
		{
			name:     "Should reject MCP owned headers for operators",
			secret:   map[string]string{"MCP-Session-Id": "id"},
			source:   SourceOperatorSecret,
			wantCode: CodeReservedHeader,
		},
		{
			name:     "Should reject MCP owned headers for OAuth",
			secret:   map[string]string{"mcp-protocol-version": "v"},
			source:   SourceOAuthOwned,
			wantCode: CodeReservedHeader,
		},
		{
			name:     "Should reject package authorization",
			fixed:    map[string]string{"Authorization": "Bearer embedded"},
			source:   SourcePackageFixed,
			wantCode: CodePackageCredential,
		},
		{
			name:     "Should reject package cookies",
			fixed:    map[string]string{"Cookie": "embedded"},
			source:   SourcePackageFixed,
			wantCode: CodePackageCredential,
		},
		{
			name:        "Should reject operator authorization when OAuth owns it",
			secret:      map[string]string{"Authorization": "vault://ref"},
			source:      SourceOperatorSecret,
			authEnabled: true,
			wantCode:    CodeOAuthAuthorizationOwned,
		},
		{
			name:     "Should reject other operator credentials",
			secret:   map[string]string{"Proxy-Authorization": "vault://ref"},
			source:   SourceOperatorSecret,
			wantCode: CodeOperatorCredential,
		},
		{
			name:     "Should reject OAuth cookies",
			secret:   map[string]string{"Set-Cookie": "value"},
			source:   SourceOAuthOwned,
			wantCode: CodeOperatorCredential,
		},
		{
			name:     "Should reject duplicate names across maps",
			fixed:    map[string]string{"X-Tenant": "a"},
			secret:   map[string]string{"x-tenant": "b"},
			source:   SourceOperatorSecret,
			wantCode: CodeDuplicateHeader,
		},
		{
			name:     "Should reject duplicate names within a map",
			fixed:    map[string]string{"X-Tenant": "a", "x-tenant": "b"},
			source:   SourcePackageFixed,
			wantCode: CodeDuplicateHeader,
		},
		{
			name:     "Should reject non-token names",
			fixed:    map[string]string{"Bad Header": "value"},
			source:   SourcePackageFixed,
			wantCode: CodeInvalidHeaderName,
		},
		{
			name:     "Should reject carriage returns",
			fixed:    map[string]string{"X-Test": "a\rb"},
			source:   SourcePackageFixed,
			wantCode: CodeInvalidHeaderValue,
		},
		{
			name:     "Should reject line feeds",
			fixed:    map[string]string{"X-Test": "a\nb"},
			source:   SourcePackageFixed,
			wantCode: CodeInvalidHeaderValue,
		},
		{
			name:     "Should reject unknown sources",
			secret:   map[string]string{"X-Test": "value"},
			source:   HeaderSource(99),
			wantCode: CodeInvalidHeaderSource,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateHeaders(test.fixed, test.secret, test.source, test.authEnabled)
			var policyErr *Error
			if !errors.As(err, &policyErr) {
				t.Fatalf("ValidateHeaders() error = %v, want *Error", err)
			}
			if policyErr.Code != test.wantCode {
				t.Fatalf("ValidateHeaders() code = %q, want %q", policyErr.Code, test.wantCode)
			}
		})
	}
}

func TestValidateRemoteURL(t *testing.T) {
	t.Parallel()

	valid := []string{
		"https://api.example.com/mcp",
		"http://localhost:3000/mcp",
		"http://127.0.0.1/mcp",
		"http://[::1]/mcp",
	}
	for _, endpoint := range valid {
		t.Run("Should accept "+endpoint, func(t *testing.T) {
			t.Parallel()

			if err := ValidateRemoteURL(endpoint); err != nil {
				t.Fatalf("ValidateRemoteURL(%q) error = %v", endpoint, err)
			}
		})
	}

	invalid := []string{
		"http://api.example.com/mcp",
		"https://user:pw@example.com/mcp",
		"https://example.com/mcp#fragment",
		"https://example.com/mcp#",
		"ftp://example.com/mcp",
		"/relative",
	}
	wantMessages := []string{
		"non-loopback MCP endpoints must use https",
		"MCP endpoint URL must not contain user information",
		"MCP endpoint URL must not contain a fragment",
		"MCP endpoint URL must not contain a fragment",
		"MCP endpoint URL must use http or https",
		"MCP endpoint URL must use http or https",
	}
	for index, endpoint := range invalid {
		t.Run("Should reject "+endpoint, func(t *testing.T) {
			t.Parallel()

			err := ValidateRemoteURL(endpoint)
			if err == nil || err.Error() != wantMessages[index] {
				t.Fatalf("ValidateRemoteURL(%q) error = %v, want %q", endpoint, err, wantMessages[index])
			}
		})
	}
}
