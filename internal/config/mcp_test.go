package config

import (
	"encoding/hex"
	"maps"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMCPRenderHeaderInputs(t *testing.T) {
	t.Parallel()
	t.Run(
		"Should omit unset optional credentials and render bound headers only in the launch copy",
		func(t *testing.T) {
			t.Parallel()
			const configured = "fixture-key"
			const reference = "vault:extensions/global/fixture/env/DD_API_KEY"
			server := MCPServer{
				AgentPlugin: true,
				Headers: map[string]string{
					"DD_API_KEY": "${DD_API_KEY:-}", "DD_APPLICATION_KEY": "Prefix ${DD_APPLICATION_KEY:-}",
					"X-Fallback": "${FALLBACK_TOKEN:-package-default}", "X-Empty": "",
				},
				InputValues: map[string]MCPInputValue{
					"DD_API_KEY": {SecretRef: reference}, "DD_APPLICATION_KEY": {}, "FALLBACK_TOKEN": {},
				},
			}
			rendered, secrets, err := server.RenderInputs(func(ref string) (string, error) {
				if ref != reference {
					t.Fatal("unexpected credential reference")
				}
				return configured, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			want := map[string]string{"DD_API_KEY": configured, "X-Fallback": "package-default", "X-Empty": ""}
			if !maps.Equal(rendered.Headers, want) || len(secrets) != 2 {
				t.Fatal("unexpected credentials in rendered headers or redactions")
			}
			if server.Headers["DD_API_KEY"] != "${DD_API_KEY:-}" || len(server.Headers) != 4 {
				t.Fatal("launch rendering changed the published definition")
			}
		},
	)
}

func TestMCPStdioConfig(t *testing.T) {
	t.Parallel()

	t.Run("Should default to a validated stdio process reuse policy", func(t *testing.T) {
		t.Parallel()

		cfg := DefaultWithHome(HomePaths{})
		if got, want := cfg.MCP.Stdio, DefaultMCPStdioConfig(); got != want {
			t.Fatalf("DefaultWithHome() MCP.Stdio = %#v, want %#v", got, want)
		}
		if got, want := cfg.MCP.StatusProbeTimeout, DefaultMCPStatusProbeTimeout; got != want {
			t.Fatalf("DefaultWithHome() MCP.StatusProbeTimeout = %s, want %s", got, want)
		}
		if err := cfg.MCP.Validate(); err != nil {
			t.Fatalf("default MCP.Validate() error = %v", err)
		}
	})

	t.Run("Should overlay stdio bounds field by field", func(t *testing.T) {
		t.Parallel()

		cfg := defaultTestConfig(t)
		overlayPath := filepath.Join(t.TempDir(), "overlay.toml")
		writeFile(t, overlayPath, "[mcp]\nstatus_probe_timeout = \"45s\"\n\n[mcp.stdio]\nidle_timeout = \"2m\"\n")
		if err := ApplyConfigOverlayFile(overlayPath, &cfg); err != nil {
			t.Fatalf("ApplyConfigOverlayFile() error = %v", err)
		}
		want := DefaultMCPStdioConfig()
		want.IdleTimeout = 2 * time.Minute
		if cfg.MCP.Stdio != want {
			t.Fatalf("MCP.Stdio = %#v, want %#v", cfg.MCP.Stdio, want)
		}
		if got, want := cfg.MCP.StatusProbeTimeout, 45*time.Second; got != want {
			t.Fatalf("MCP.StatusProbeTimeout = %s, want %s", got, want)
		}
	})

	t.Run("Should reject unusable stdio bounds", func(t *testing.T) {
		t.Parallel()

		for name, mutate := range map[string]func(*MCPStdioConfig){
			"mcp.stdio.idle_timeout":    func(cfg *MCPStdioConfig) { cfg.IdleTimeout = 0 },
			"mcp.stdio.connect_timeout": func(cfg *MCPStdioConfig) { cfg.ConnectTimeout = -time.Second },
			"mcp.stdio.max_concurrent_calls": func(cfg *MCPStdioConfig) {
				cfg.MaxConcurrentCalls = MaxMCPStdioMaxConcurrentCalls + 1
			},
		} {
			cfg := MCPConfig{OAuth: DefaultWithHome(HomePaths{}).MCP.OAuth, Stdio: DefaultMCPStdioConfig()}
			mutate(&cfg.Stdio)
			cfg.StatusProbeTimeout = DefaultMCPStatusProbeTimeout
			err := cfg.Validate()
			if err == nil || !strings.Contains(err.Error(), name) {
				t.Fatalf("Validate() error = %v, want rejection naming %s", err, name)
			}
		}
		cfg := DefaultWithHome(HomePaths{}).MCP
		cfg.StatusProbeTimeout = 0
		if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "mcp.status_probe_timeout") {
			t.Fatalf("Validate() error = %v, want rejection naming mcp.status_probe_timeout", err)
		}
	})

	t.Run("Should keep stdio bounds out of workspace and profile layers", func(t *testing.T) {
		t.Parallel()

		for _, path := range [][]string{{"mcp", "stdio", "idle_timeout"}, {"mcp", "status_probe_timeout"}} {
			if err := ValidateConfigWriteScope(WriteScopeWorkspace, path); err == nil ||
				!strings.Contains(err.Error(), "global-only") {
				t.Fatalf("ValidateConfigWriteScope(workspace, %v) error = %v, want global-only rejection", path, err)
			}
			if err := ValidateConfigWriteScope(WriteScopeUser, path); err != nil {
				t.Fatalf("ValidateConfigWriteScope(user, %v) error = %v, want nil", path, err)
			}
		}
		_, err := loadProfileConfigOverlayBytes([]byte("[mcp.stdio]\nidle_timeout = \"5m\"\n"), "profile.toml")
		if err == nil || !strings.Contains(err.Error(), profileConfigKeyDeniedCode) &&
			!strings.Contains(err.Error(), "machine-only") {
			t.Fatalf("loadProfileConfigOverlayBytes(mcp.stdio) error = %v, want profile denial", err)
		}
	})
}

// Invariant (UT-007): the consent identity covers the full unresolved definition,
// while credentials behind stable Vault references and runtime allocation do not affect it.
// Owner: config definition normalization; canonical suite: TestMCPDefinitionDigest.
func TestMCPDefinitionDigest(t *testing.T) {
	t.Parallel()
	original := MCPServer{
		Name:          "remote",
		Transport:     "http",
		URL:           "https://example.com/mcp",
		Args:          []string{"a"},
		Env:           map[string]string{"FLAG": "one"},
		Headers:       map[string]string{"X-Mode": "one"},
		SecretEnv:     map[string]string{"TOKEN": "vault:one"},
		SecretHeaders: map[string]string{"X-Key": "vault:key"},
		ReadOnlyTools: []string{"read"},
		Auth:          MCPAuthConfig{Registration: MCPAuthRegistrationAuto},
	}
	// Invariant: public fingerprints are stable per installation and differ across installations; owner: config normalization.
	t.Run("Should bind the definition digest to the installation key", func(t *testing.T) {
		t.Parallel()
		first, err := MCPDefinitionDigest(original, []byte("installation one"))
		if err != nil {
			t.Fatal(err)
		}
		again, err := MCPDefinitionDigest(original, []byte("installation one"))
		if err != nil || again != first {
			t.Fatalf("unstable digest: %v", err)
		}
		other, err := MCPDefinitionDigest(original, []byte("installation two"))
		if err != nil || other == first {
			t.Fatalf("installation-independent digest: %v", err)
		}
	})

	t.Run("Should reject a missing installation key", func(t *testing.T) {
		t.Parallel()
		if digest, err := MCPDefinitionDigest(original, nil); err == nil || digest != "" {
			t.Fatalf("missing key produced digest %q: %v", digest, err)
		}
	})
	t.Run("Should keep literal credentials out of the digest", func(t *testing.T) {
		t.Parallel()
		server := cloneMCPServer(original)
		server.URL = "https://example.com/mcp?token=private-query-token"
		server.Env = map[string]string{"TOKEN": "private-env-token"}
		server.Headers = map[string]string{"Authorization": "Bearer private-header-token"}
		digest, err := MCPDefinitionDigest(server, []byte("installation one"))
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := hex.DecodeString(digest)
		if err != nil || len(decoded) != 32 {
			t.Fatalf("invalid digest %q: %v", digest, err)
		}
		for _, secret := range []string{"private-query-token", "private-env-token", "private-header-token"} {
			if strings.Contains(digest, secret) {
				t.Fatal("digest exposed a literal credential")
			}
		}
	})
	digest, err := MCPDefinitionDigest(original, []byte("installation one"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func(*MCPServer)
	}{
		{"args", func(s *MCPServer) { s.Args = []string{"b"} }},
		{"env value", func(s *MCPServer) { s.Env["FLAG"] = "two" }},
		{"header value", func(s *MCPServer) { s.Headers["X-Mode"] = "two" }},
		{"env reference", func(s *MCPServer) { s.SecretEnv["TOKEN"] = "vault:two" }},
		{"header reference", func(s *MCPServer) { s.SecretHeaders["X-Key"] = "vault:other" }},
		{"readonly", func(s *MCPServer) { s.ReadOnlyTools = []string{"other"} }},
		{"cwd", func(s *MCPServer) { s.CWD = "/other" }},
		{"auth", func(s *MCPServer) { s.Auth.ClientID = "other-client" }},
		{"catalog entry", func(s *MCPServer) { s.CatalogEntry = "other" }},
		{"catalog version", func(s *MCPServer) { s.CatalogVersion = "2" }},
		{"reference", func(s *MCPServer) { s.Reference = true }},
	} {
		t.Run("Should change for "+tc.name, func(t *testing.T) {
			t.Parallel()
			changed := cloneMCPServer(original)
			tc.change(&changed)
			got, err := MCPDefinitionDigest(changed, []byte("installation one"))
			if err != nil || got == digest {
				t.Fatalf("digest=%q, err=%v", got, err)
			}
		})
	}
	changed := cloneMCPServer(original)
	changed.Name = "other"
	changed.RuntimeName = "allocated"
	changed.DefinitionSource = "/other/file"
	got, err := MCPDefinitionDigest(changed, []byte("installation one"))
	if err != nil || got != digest {
		t.Fatalf("allocation changed digest: %q %v", got, err)
	}
}
