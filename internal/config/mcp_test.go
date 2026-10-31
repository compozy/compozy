package config

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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
