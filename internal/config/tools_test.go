package config

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestToolsConfigDefaults(t *testing.T) {
	t.Parallel()

	t.Run("Should load safe tools defaults", func(t *testing.T) {
		t.Parallel()

		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}

		cfg := DefaultWithHome(homePaths)
		if !cfg.Tools.Enabled {
			t.Fatal("DefaultWithHome() Tools.Enabled = false, want true")
		}
		if !cfg.Tools.HostedMCPEnabled {
			t.Fatal("DefaultWithHome() Tools.HostedMCPEnabled = false, want true")
		}
		if got, want := cfg.Tools.DefaultMaxResultBytes, DefaultToolsMaxResultBytes; got != want {
			t.Fatalf("DefaultWithHome() Tools.DefaultMaxResultBytes = %d, want %d", got, want)
		}
		if got, want := cfg.Tools.HostedMCP.BindNonceTTLSeconds, DefaultHostedMCPBindNonceTTLSeconds; got != want {
			t.Fatalf("DefaultWithHome() Tools.HostedMCP.BindNonceTTLSeconds = %d, want %d", got, want)
		}
		if got, want := cfg.Tools.HostedMCP.BindNonceTTL(), 30*time.Second; got != want {
			t.Fatalf("DefaultWithHome() Tools.HostedMCP.BindNonceTTL() = %s, want %s", got, want)
		}
		if got, want := cfg.Tools.Clarify.Timeout, DefaultToolsClarifyTimeout; got != want {
			t.Fatalf("DefaultWithHome() Tools.Clarify.Timeout = %s, want %s", got, want)
		}
		if got, want := cfg.Tools.Artifacts.MaxCount, DefaultToolsArtifactMaxCount; got != want {
			t.Fatalf("DefaultWithHome() Tools.Artifacts.MaxCount = %d, want %d", got, want)
		}
		if got, want := cfg.Tools.Artifacts.MaxBytes, DefaultToolsArtifactMaxBytes; got != want {
			t.Fatalf("DefaultWithHome() Tools.Artifacts.MaxBytes = %d, want %d", got, want)
		}
		if got, want := cfg.Tools.Artifacts.MaxAge, DefaultToolsArtifactMaxAge; got != want {
			t.Fatalf("DefaultWithHome() Tools.Artifacts.MaxAge = %s, want %s", got, want)
		}
		if got, want := cfg.Tools.Policy.ExternalDefault, ToolsExternalDefaultDisabled; got != want {
			t.Fatalf("DefaultWithHome() Tools.Policy.ExternalDefault = %q, want %q", got, want)
		}
		if got, want := cfg.Tools.Policy.ApprovalTimeoutSeconds, DefaultToolsApprovalTimeoutSeconds; got != want {
			t.Fatalf("DefaultWithHome() Tools.Policy.ApprovalTimeoutSeconds = %d, want %d", got, want)
		}
		if got, want := cfg.Tools.Policy.ApprovalTimeout(), 120*time.Second; got != want {
			t.Fatalf("DefaultWithHome() Tools.Policy.ApprovalTimeout() = %s, want %s", got, want)
		}
		if len(cfg.Tools.Policy.TrustedSources) != 0 {
			t.Fatalf(
				"DefaultWithHome() Tools.Policy.TrustedSources = %#v, want empty",
				cfg.Tools.Policy.TrustedSources,
			)
		}
	})
}

func TestToolsConfigValidation(t *testing.T) {
	t.Parallel()

	t.Run("Should accept valid trusted external sources", func(t *testing.T) {
		t.Parallel()

		cfg := defaultTestConfig(t)
		cfg.MCPServers = []MCPServer{{
			Name:    "github",
			Command: "npx",
		}}
		cfg.Tools.Policy.TrustedSources = []string{"mcp:github", "extension:linear"}

		if err := cfg.Validate(); err != nil {
			t.Fatalf("Validate() error = %v", err)
		}
	})

	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{
			name: "ShouldRejectNegativeDefaultResultBytes",
			mutate: func(cfg *Config) {
				cfg.Tools.DefaultMaxResultBytes = -1
			},
			wantErr: "tools.default_max_result_bytes",
		},
		{
			name: "ShouldRejectTooLargeDefaultResultBytes",
			mutate: func(cfg *Config) {
				cfg.Tools.DefaultMaxResultBytes = MaxToolsMaxResultBytes + 1
			},
			wantErr: "tools.default_max_result_bytes",
		},
		{
			name: "ShouldRejectInvalidExternalDefault",
			mutate: func(cfg *Config) {
				cfg.Tools.Policy.ExternalDefault = ToolsExternalDefault("maybe")
			},
			wantErr: "tools.policy.external_default",
		},
		{
			name: "ShouldRejectZeroApprovalTimeout",
			mutate: func(cfg *Config) {
				cfg.Tools.Policy.ApprovalTimeoutSeconds = 0
			},
			wantErr: "tools.policy.approval_timeout_seconds",
		},
		{
			name: "ShouldRejectTooLargeApprovalTimeout",
			mutate: func(cfg *Config) {
				cfg.Tools.Policy.ApprovalTimeoutSeconds = MaxToolsApprovalTimeoutSeconds + 1
			},
			wantErr: "tools.policy.approval_timeout_seconds",
		},
		{
			name: "ShouldRejectZeroHostedMCPBindNonceTTL",
			mutate: func(cfg *Config) {
				cfg.Tools.HostedMCP.BindNonceTTLSeconds = 0
			},
			wantErr: "tools.hosted_mcp.bind_nonce_ttl_seconds",
		},
		{
			name: "ShouldRejectTooLargeHostedMCPBindNonceTTL",
			mutate: func(cfg *Config) {
				cfg.Tools.HostedMCP.BindNonceTTLSeconds = MaxHostedMCPBindNonceTTLSeconds + 1
			},
			wantErr: "tools.hosted_mcp.bind_nonce_ttl_seconds",
		},
		{
			name: "ShouldRejectTooShortClarifyTimeout",
			mutate: func(cfg *Config) {
				cfg.Tools.Clarify.Timeout = MinToolsClarifyTimeout - time.Nanosecond
			},
			wantErr: "tools.clarify.timeout",
		},
		{
			name: "ShouldRejectTooLongClarifyTimeout",
			mutate: func(cfg *Config) {
				cfg.Tools.Clarify.Timeout = MaxToolsClarifyTimeout + time.Nanosecond
			},
			wantErr: "tools.clarify.timeout",
		},
		{
			name: "ShouldRejectZeroArtifactMaxCount",
			mutate: func(cfg *Config) {
				cfg.Tools.Artifacts.MaxCount = 0
			},
			wantErr: "tools.artifacts.max_count",
		},
		{
			name: "ShouldRejectZeroArtifactMaxBytes",
			mutate: func(cfg *Config) {
				cfg.Tools.Artifacts.MaxBytes = 0
			},
			wantErr: "tools.artifacts.max_bytes",
		},
		{
			name: "ShouldRejectZeroArtifactMaxAge",
			mutate: func(cfg *Config) {
				cfg.Tools.Artifacts.MaxAge = 0
			},
			wantErr: "tools.artifacts.max_age",
		},
		{
			name: "ShouldRejectBlankTrustedSource",
			mutate: func(cfg *Config) {
				cfg.Tools.Policy.TrustedSources = []string{""}
			},
			wantErr: "tools.policy.trusted_sources[0]",
		},
		{
			name: "ShouldRejectTrustedSourceWhitespace",
			mutate: func(cfg *Config) {
				cfg.Tools.Policy.TrustedSources = []string{" mcp:github "}
			},
			wantErr: "tools.policy.trusted_sources[0]",
		},
		{
			name: "ShouldRejectUnknownTrustedSourceKind",
			mutate: func(cfg *Config) {
				cfg.Tools.Policy.TrustedSources = []string{"builtin:agh"}
			},
			wantErr: "tools.policy.trusted_sources[0]",
		},
		{
			name: "ShouldRejectTrustedSourceWildcardOwner",
			mutate: func(cfg *Config) {
				cfg.Tools.Policy.TrustedSources = []string{"mcp:*"}
			},
			wantErr: "tools.policy.trusted_sources[0]",
		},
		{
			name: "ShouldRejectUnknownMCPTrustedSource",
			mutate: func(cfg *Config) {
				cfg.Tools.Policy.TrustedSources = []string{"mcp:missing"}
			},
			wantErr: "unknown MCP source",
		},
		{
			name: "ShouldRejectDuplicateTrustedSource",
			mutate: func(cfg *Config) {
				cfg.MCPServers = []MCPServer{{
					Name:    "github",
					Command: "npx",
				}}
				cfg.Tools.Policy.TrustedSources = []string{"mcp:github", "mcp:github"}
			},
			wantErr: "duplicates trusted source",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := defaultTestConfig(t)
			tt.mutate(&cfg)
			err := cfg.Validate()
			if err == nil {
				t.Fatal("Validate() error = nil, want tools config validation failure")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %q, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestApplyConfigOverlayFileAppliesToolsOverlay(t *testing.T) {
	t.Parallel()

	t.Run("Should apply tools overlay", func(t *testing.T) {
		t.Parallel()

		cfg := defaultTestConfig(t)
		overlayPath := filepath.Join(t.TempDir(), "overlay.toml")
		writeFile(t, overlayPath, `
[tools]
enabled = false
hosted_mcp_enabled = false
default_max_result_bytes = 131072

[tools.hosted_mcp]
bind_nonce_ttl_seconds = 45

[tools.clarify]
timeout = "2m"

[tools.artifacts]
max_count = 42
max_bytes = 8388608
max_age = "24h"

[tools.policy]
external_default = "ask"
approval_timeout_seconds = 90
trusted_sources = ["mcp:github", "extension:linear"]
`)

		if err := ApplyConfigOverlayFile(overlayPath, &cfg); err != nil {
			t.Fatalf("ApplyConfigOverlayFile() error = %v", err)
		}
		if cfg.Tools.Enabled || cfg.Tools.HostedMCPEnabled {
			t.Fatalf("ApplyConfigOverlayFile() Tools flags = %#v, want disabled", cfg.Tools)
		}
		if got, want := cfg.Tools.DefaultMaxResultBytes, int64(131072); got != want {
			t.Fatalf("ApplyConfigOverlayFile() DefaultMaxResultBytes = %d, want %d", got, want)
		}
		if got, want := cfg.Tools.HostedMCP.BindNonceTTLSeconds, 45; got != want {
			t.Fatalf("ApplyConfigOverlayFile() BindNonceTTLSeconds = %d, want %d", got, want)
		}
		if got, want := cfg.Tools.Clarify.Timeout, 2*time.Minute; got != want {
			t.Fatalf("ApplyConfigOverlayFile() Clarify.Timeout = %s, want %s", got, want)
		}
		if got, want := cfg.Tools.Artifacts.MaxCount, 42; got != want {
			t.Fatalf("ApplyConfigOverlayFile() Artifacts.MaxCount = %d, want %d", got, want)
		}
		if got, want := cfg.Tools.Artifacts.MaxBytes, int64(8388608); got != want {
			t.Fatalf("ApplyConfigOverlayFile() Artifacts.MaxBytes = %d, want %d", got, want)
		}
		if got, want := cfg.Tools.Artifacts.MaxAge, 24*time.Hour; got != want {
			t.Fatalf("ApplyConfigOverlayFile() Artifacts.MaxAge = %s, want %s", got, want)
		}
		if got, want := cfg.Tools.Policy.ExternalDefault, ToolsExternalDefaultAsk; got != want {
			t.Fatalf("ApplyConfigOverlayFile() ExternalDefault = %q, want %q", got, want)
		}
		if got, want := cfg.Tools.Policy.ApprovalTimeoutSeconds, 90; got != want {
			t.Fatalf("ApplyConfigOverlayFile() ApprovalTimeoutSeconds = %d, want %d", got, want)
		}
		if got, want := cfg.Tools.Policy.TrustedSources,
			[]string{"mcp:github", "extension:linear"}; !slices.Equal(got, want) {
			t.Fatalf("ApplyConfigOverlayFile() TrustedSources = %#v, want %#v", got, want)
		}
	})
}

func TestLoadMergesToolsConfigAcrossGlobalAndWorkspace(t *testing.T) {
	t.Run("Should merge tools config across global and workspace overlays", func(t *testing.T) {
		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		if err := EnsureHomeLayout(homePaths); err != nil {
			t.Fatalf("EnsureHomeLayout() error = %v", err)
		}
		workspaceRoot := t.TempDir()

		writeFile(t, homePaths.ConfigFile, `
[[mcp_servers]]
name = "global"
command = "npx"

[tools]
enabled = true
hosted_mcp_enabled = false
default_max_result_bytes = 131072

[tools.hosted_mcp]
bind_nonce_ttl_seconds = 45

[tools.clarify]
timeout = "4m"

[tools.artifacts]
max_count = 40
max_bytes = 4194304
max_age = "48h"

[tools.policy]
external_default = "ask"
approval_timeout_seconds = 90
trusted_sources = ["mcp:global"]
`)
		writeFile(t, workspaceConfigFile(workspaceRoot), `
[[mcp_servers]]
name = "workspace"
command = "node"

[tools]
default_max_result_bytes = 524288

[tools.hosted_mcp]
bind_nonce_ttl_seconds = 15

[tools.clarify]
timeout = "90s"

[tools.artifacts]
max_count = 20
max_age = "12h"

[tools.policy]
external_default = "enabled"
approval_timeout_seconds = 30
trusted_sources = ["mcp:workspace", "extension:linear"]
`)

		cfg, err := LoadForHome(homePaths, WithWorkspaceRoot(workspaceRoot), withoutDotEnv())
		if err != nil {
			t.Fatalf("LoadForHome() error = %v", err)
		}
		if !cfg.Tools.Enabled {
			t.Fatal("LoadForHome() Tools.Enabled = false, want inherited true")
		}
		if cfg.Tools.HostedMCPEnabled {
			t.Fatal("LoadForHome() Tools.HostedMCPEnabled = true, want inherited false")
		}
		if got, want := cfg.Tools.DefaultMaxResultBytes, int64(524288); got != want {
			t.Fatalf("LoadForHome() DefaultMaxResultBytes = %d, want %d", got, want)
		}
		if got, want := cfg.Tools.HostedMCP.BindNonceTTLSeconds, 15; got != want {
			t.Fatalf("LoadForHome() BindNonceTTLSeconds = %d, want %d", got, want)
		}
		if got, want := cfg.Tools.Clarify.Timeout, 90*time.Second; got != want {
			t.Fatalf("LoadForHome() Clarify.Timeout = %s, want %s", got, want)
		}
		if got, want := cfg.Tools.Artifacts.MaxCount, 20; got != want {
			t.Fatalf("LoadForHome() Artifacts.MaxCount = %d, want %d", got, want)
		}
		if got, want := cfg.Tools.Artifacts.MaxBytes, int64(4194304); got != want {
			t.Fatalf("LoadForHome() Artifacts.MaxBytes = %d, want inherited %d", got, want)
		}
		if got, want := cfg.Tools.Artifacts.MaxAge, 12*time.Hour; got != want {
			t.Fatalf("LoadForHome() Artifacts.MaxAge = %s, want %s", got, want)
		}
		if got, want := cfg.Tools.Policy.ExternalDefault, ToolsExternalDefaultEnabled; got != want {
			t.Fatalf("LoadForHome() ExternalDefault = %q, want %q", got, want)
		}
		if got, want := cfg.Tools.Policy.ApprovalTimeoutSeconds, 30; got != want {
			t.Fatalf("LoadForHome() ApprovalTimeoutSeconds = %d, want %d", got, want)
		}
		if got, want := cfg.Tools.Policy.TrustedSources,
			[]string{"mcp:workspace", "extension:linear"}; !slices.Equal(got, want) {
			t.Fatalf("LoadForHome() TrustedSources = %#v, want %#v", got, want)
		}
		if !hasMCPServer(cfg.MCPServers, "global") || !hasMCPServer(cfg.MCPServers, "workspace") {
			t.Fatalf("LoadForHome() MCPServers = %#v, want merged global and workspace servers", cfg.MCPServers)
		}
	})
}

func TestLoadRejectsUnknownToolsConfigKeys(t *testing.T) {
	t.Run("Should reject unknown tools config keys", func(t *testing.T) {
		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		if err := EnsureHomeLayout(homePaths); err != nil {
			t.Fatalf("EnsureHomeLayout() error = %v", err)
		}
		writeFile(t, homePaths.ConfigFile, `
[tools]
unknown = true
`)

		_, err = LoadForHome(homePaths, withoutDotEnv())
		if err == nil {
			t.Fatal("LoadForHome() error = nil, want unknown tools key failure")
		}
		if !strings.Contains(err.Error(), "tools.unknown") {
			t.Fatalf("LoadForHome() error = %q, want tools.unknown", err)
		}
	})
}

func TestLoadToolsConfigAndAgentGrammarThroughRuntimePaths(t *testing.T) {
	t.Run("Should load tools config and agent grammar through runtime paths", func(t *testing.T) {
		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		if err := EnsureHomeLayout(homePaths); err != nil {
			t.Fatalf("EnsureHomeLayout() error = %v", err)
		}
		writeFile(t, homePaths.ConfigFile, `
[[mcp_servers]]
name = "github"
command = "npx"

[tools]
enabled = true
hosted_mcp_enabled = true
default_max_result_bytes = 262144

[tools.hosted_mcp]
bind_nonce_ttl_seconds = 30

[tools.policy]
external_default = "ask"
approval_timeout_seconds = 120
trusted_sources = ["mcp:github", "extension:linear"]
`)
		agentPath := filepath.Join(homePaths.AgentsDir, "coder", agentDefName)
		writeFile(t, agentPath, `---
name: coder
provider: claude
tools: ["agh__skill_view", "mcp__github__*"]
toolsets: ["agh__catalog"]
deny_tools: ["agh__task_*"]
---

You are a code agent.
`)

		cfg, err := LoadForHome(homePaths, withoutDotEnv())
		if err != nil {
			t.Fatalf("LoadForHome() error = %v", err)
		}
		agent, err := LoadAgentDef("coder", homePaths)
		if err != nil {
			t.Fatalf("LoadAgentDef() error = %v", err)
		}
		resolved, err := cfg.ResolveAgent(agent)
		if err != nil {
			t.Fatalf("ResolveAgent() error = %v", err)
		}
		if got, want := resolved.Tools, []string{"agh__skill_view", "mcp__github__*"}; !slices.Equal(got, want) {
			t.Fatalf("ResolveAgent() Tools = %#v, want %#v", got, want)
		}
		if got, want := resolved.Toolsets, []string{"agh__catalog"}; !slices.Equal(got, want) {
			t.Fatalf("ResolveAgent() Toolsets = %#v, want %#v", got, want)
		}
		if got, want := resolved.DenyTools, []string{"agh__task_*"}; !slices.Equal(got, want) {
			t.Fatalf("ResolveAgent() DenyTools = %#v, want %#v", got, want)
		}
	})
}

func defaultTestConfig(t *testing.T) Config {
	t.Helper()

	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	return DefaultWithHome(homePaths)
}
