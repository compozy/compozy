package daemon

import (
	"testing"

	aghconfig "github.com/compozy/agh/internal/config"
)

func TestCloneDaemonMCPServer(t *testing.T) {
	t.Run("Should Preserve Remote Metadata And Deep Copy", func(t *testing.T) {
		t.Parallel()

		original := aghconfig.MCPServer{
			Name:      "github",
			Transport: aghconfig.MCPServerTransportSSE,
			Command:   "ignored",
			Args:      []string{"--stdio"},
			Env: map[string]string{
				"TOKEN_ENV": "GITHUB_TOKEN",
			},
			URL: "https://mcp.example.test/sse",
			Auth: aghconfig.MCPAuthConfig{
				Type:             aghconfig.MCPAuthTypeOAuth2PKCE,
				IssuerURL:        "https://issuer.example.test",
				MetadataURL:      "https://issuer.example.test/.well-known/oauth-authorization-server",
				AuthorizationURL: "https://issuer.example.test/authorize",
				TokenURL:         "https://issuer.example.test/token",
				RevocationURL:    "https://issuer.example.test/revoke",
				ClientID:         "agh-client",
				ClientSecretRef:  "env:GITHUB_MCP_CLIENT_SECRET",
				Scopes:           []string{"tools.read", "issues.write"},
			},
		}

		cloned := cloneDaemonMCPServer(original)
		original.Args[0] = "mutated"
		original.Env["TOKEN_ENV"] = "mutated"
		original.Auth.Scopes[0] = "mutated"

		if got, want := cloned.Transport, aghconfig.MCPServerTransportSSE; got != want {
			t.Fatalf("cloned.Transport = %q, want %q", got, want)
		}
		if got, want := cloned.URL, "https://mcp.example.test/sse"; got != want {
			t.Fatalf("cloned.URL = %q, want %q", got, want)
		}
		if got, want := cloned.Auth.Type, aghconfig.MCPAuthTypeOAuth2PKCE; got != want {
			t.Fatalf("cloned.Auth.Type = %q, want %q", got, want)
		}
		if got, want := cloned.Auth.ClientSecretRef, "env:GITHUB_MCP_CLIENT_SECRET"; got != want {
			t.Fatalf("cloned.Auth.ClientSecretRef = %q, want %q", got, want)
		}
		if got, want := cloned.Args[0], "--stdio"; got != want {
			t.Fatalf("cloned.Args[0] = %q, want %q", got, want)
		}
		if got, want := cloned.Env["TOKEN_ENV"], "GITHUB_TOKEN"; got != want {
			t.Fatalf("cloned.Env[TOKEN_ENV] = %q, want %q", got, want)
		}
		if got, want := cloned.Auth.Scopes[0], "tools.read"; got != want {
			t.Fatalf("cloned.Auth.Scopes[0] = %q, want %q", got, want)
		}
	})
}
