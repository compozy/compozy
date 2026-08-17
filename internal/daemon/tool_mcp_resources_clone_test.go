package daemon

import (
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

func TestCloneDaemonMCPServer(t *testing.T) {
	t.Run("Should Preserve Remote Metadata And Deep Copy", func(t *testing.T) {
		t.Parallel()

		original := compozyconfig.MCPServer{
			Name:      "github",
			Transport: compozyconfig.MCPServerTransportHTTP,
			Command:   "ignored",
			Args:      []string{"--stdio"},
			Env: map[string]string{
				"TOKEN_ENV": "GITHUB_TOKEN",
			},
			Headers:       map[string]string{"X-Tenant": "public"},
			SecretHeaders: map[string]string{"Authorization": "vault:extensions/github/env/GITHUB_TOKEN"},
			URL:           "https://mcp.example.test/mcp",
			Auth: compozyconfig.MCPAuthConfig{
				Registration:    compozyconfig.MCPAuthRegistrationPreRegistered,
				IssuerURL:       "https://issuer.example.test",
				ClientID:        "compozy-client",
				ClientSecretRef: "vault:mcp/global/github/client-secret",
				Scopes:          []string{"tools.read", "issues.write"},
			},
		}

		cloned := cloneDaemonMCPServer(original)
		original.Args[0] = "mutated"
		original.Env["TOKEN_ENV"] = "mutated"
		original.Headers["X-Tenant"] = "mutated"
		original.SecretHeaders["Authorization"] = "mutated"
		original.Auth.Scopes[0] = "mutated"

		if got, want := cloned.Transport, compozyconfig.MCPServerTransportHTTP; got != want {
			t.Fatalf("cloned.Transport = %q, want %q", got, want)
		}
		if got, want := cloned.URL, "https://mcp.example.test/mcp"; got != want {
			t.Fatalf("cloned.URL = %q, want %q", got, want)
		}
		if got, want := cloned.Auth.Registration, compozyconfig.MCPAuthRegistrationPreRegistered; got != want {
			t.Fatalf("cloned.Auth.Registration = %q, want %q", got, want)
		}
		if got, want := cloned.Auth.ClientSecretRef, "vault:mcp/global/github/client-secret"; got != want {
			t.Fatalf("cloned.Auth.ClientSecretRef = %q, want %q", got, want)
		}
		if got, want := cloned.Args[0], "--stdio"; got != want {
			t.Fatalf("cloned.Args[0] = %q, want %q", got, want)
		}
		if got, want := cloned.Env["TOKEN_ENV"], "GITHUB_TOKEN"; got != want {
			t.Fatalf("cloned.Env[TOKEN_ENV] = %q, want %q", got, want)
		}
		if got, want := cloned.Headers["X-Tenant"], "public"; got != want {
			t.Fatalf("cloned.Headers[X-Tenant] = %q, want %q", got, want)
		}
		if got, want := cloned.SecretHeaders["Authorization"], "vault:extensions/github/env/GITHUB_TOKEN"; got != want {
			t.Fatalf("cloned.SecretHeaders[Authorization] = %q, want %q", got, want)
		}
		if got, want := cloned.Auth.Scopes[0], "tools.read"; got != want {
			t.Fatalf("cloned.Auth.Scopes[0] = %q, want %q", got, want)
		}
	})
}
