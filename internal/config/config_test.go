package config

import (
	"bytes"
	"errors"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/sandbox"
)

func TestLoadValidTOMLConfigWithAllSections(t *testing.T) {
	workspaceRoot := t.TempDir()
	homeRoot := filepath.Join(t.TempDir(), "home")
	t.Setenv("AGH_HOME", homeRoot)

	homePaths, err := ResolveHomePaths()
	if err != nil {
		t.Fatalf("ResolveHomePaths() error = %v", err)
	}
	if err := EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}

	writeFile(t, homePaths.ConfigFile, `
[daemon]
socket = "~/.agh/custom.sock"

[http]
host = "127.0.0.1"
port = 3030

[defaults]
agent = "researcher"
provider = "claude"

[agents.soul]
enabled = false
max_body_bytes = 16384
context_projection_bytes = 1024

[agents.heartbeat]
enabled = true
max_body_bytes = 24576
context_projection_bytes = 2048
min_interval = "10m"
default_interval = "45m"
wake_cooldown = "2m"
max_wakes_per_cycle = 8
active_session_only = false
allow_active_hours_preferences = false
wake_event_retention = "72h"
session_health_stale_after = "3m"
session_health_hook_min_interval = "90s"

[limits]
max_concurrent_agents = 22

[session.limits]
timeout = "30m"

[permissions]
mode = "approve-all"

[[mcp_servers]]
name = "linear"
command = "linear-mcp"

[tools]
enabled = true
hosted_mcp_enabled = false
default_max_result_bytes = 131072

[tools.hosted_mcp]
bind_nonce_ttl_seconds = 45

[tools.policy]
external_default = "ask"
approval_timeout_seconds = 90
trusted_sources = ["mcp:linear", "extension:linear"]

	[providers.claude]
	auth_mode = "bound_secret"
	[providers.claude.models]
	default = "claude-opus"
	[[providers.claude.credential_slots]]
	name = "api_key"
	target_env = "ANTHROPIC_KEY"
	secret_ref = "env:ANTHROPIC_KEY"
	kind = "api_key"
	required = true
	[[providers.claude.mcp_servers]]
	name = "github"
command = "npx"
args = ["-y", "@modelcontextprotocol/server-github"]
secret_env = { GITHUB_TOKEN = "env:GITHUB_TOKEN" }

[observability]
enabled = true
retention_days = 14
max_global_bytes = 2048
agent_probe_timeout = "9s"

[observability.transcripts]
enabled = true
segment_bytes = 512
max_bytes_per_session = 4096

[log]
level = "debug"
max_size_mb = 20
max_backups = 4
max_age_days = 14
compress_backups = true

[skills]
enabled = false
disabled_skills = ["code-review", "agh"]
poll_interval = "5s"
allowed_marketplace_mcp = ["@registry/skill-a", "@registry/skill-b"]
allowed_marketplace_hooks = ["@registry/hook-a", "@registry/hook-b"]

[skills.marketplace]
registry = "clawhub"
base_url = "https://registry.example.test/api/v1"

[extensions.marketplace]
registry = "github"
base_url = "https://api.github.example.test"

[memory]
enabled = true
global_dir = "~/agh-memory-test"

[memory.dream]
min_hours = 48
min_sessions = 5
check_interval = "45m"

[roles.dream]
agent = "claude"

[network]
enabled = true
max_replay_age = 600

[network.live.defaults]
max_wakes = 12

[network.live.limits]
max_wakes = 80
`)

	cfg, err := Load(WithWorkspaceRoot(workspaceRoot))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Daemon.Socket == "~/.agh/custom.sock" {
		t.Fatalf("Load() did not normalize daemon socket path: %q", cfg.Daemon.Socket)
	}
	if cfg.HTTP.Host != "127.0.0.1" || cfg.HTTP.Port != 3030 {
		t.Fatalf("Load() HTTP = %#v", cfg.HTTP)
	}
	if cfg.Defaults.Agent != "researcher" {
		t.Fatalf("Load() Defaults.Agent = %q, want %q", cfg.Defaults.Agent, "researcher")
	}
	if cfg.Defaults.Provider != "claude" {
		t.Fatalf("Load() Defaults.Provider = %q, want %q", cfg.Defaults.Provider, "claude")
	}
	if cfg.Agents.Soul.Enabled {
		t.Fatal("Load() Agents.Soul.Enabled = true, want false")
	}
	if got, want := cfg.Agents.Soul.MaxBodyBytes, int64(16384); got != want {
		t.Fatalf("Load() Agents.Soul.MaxBodyBytes = %d, want %d", got, want)
	}
	if got, want := cfg.Agents.Soul.ContextProjectionBytes, int64(1024); got != want {
		t.Fatalf("Load() Agents.Soul.ContextProjectionBytes = %d, want %d", got, want)
	}
	if !cfg.Agents.Heartbeat.Enabled {
		t.Fatal("Load() Agents.Heartbeat.Enabled = false, want true")
	}
	if got, want := cfg.Agents.Heartbeat.MaxBodyBytes, int64(24576); got != want {
		t.Fatalf("Load() Agents.Heartbeat.MaxBodyBytes = %d, want %d", got, want)
	}
	if got, want := cfg.Agents.Heartbeat.ContextProjectionBytes, int64(2048); got != want {
		t.Fatalf("Load() Agents.Heartbeat.ContextProjectionBytes = %d, want %d", got, want)
	}
	if got, want := cfg.Agents.Heartbeat.MinInterval, 10*time.Minute; got != want {
		t.Fatalf("Load() Agents.Heartbeat.MinInterval = %s, want %s", got, want)
	}
	if got, want := cfg.Agents.Heartbeat.DefaultInterval, 45*time.Minute; got != want {
		t.Fatalf("Load() Agents.Heartbeat.DefaultInterval = %s, want %s", got, want)
	}
	if got, want := cfg.Agents.Heartbeat.WakeCooldown, 2*time.Minute; got != want {
		t.Fatalf("Load() Agents.Heartbeat.WakeCooldown = %s, want %s", got, want)
	}
	if got, want := cfg.Agents.Heartbeat.MaxWakesPerCycle, 8; got != want {
		t.Fatalf("Load() Agents.Heartbeat.MaxWakesPerCycle = %d, want %d", got, want)
	}
	if cfg.Agents.Heartbeat.ActiveSessionOnly {
		t.Fatal("Load() Agents.Heartbeat.ActiveSessionOnly = true, want false")
	}
	if cfg.Agents.Heartbeat.AllowActiveHoursPreferences {
		t.Fatal("Load() Agents.Heartbeat.AllowActiveHoursPreferences = true, want false")
	}
	if got, want := cfg.Agents.Heartbeat.WakeEventRetention, 72*time.Hour; got != want {
		t.Fatalf("Load() Agents.Heartbeat.WakeEventRetention = %s, want %s", got, want)
	}
	if got, want := cfg.Agents.Heartbeat.SessionHealthStaleAfter, 3*time.Minute; got != want {
		t.Fatalf("Load() Agents.Heartbeat.SessionHealthStaleAfter = %s, want %s", got, want)
	}
	if got, want := cfg.Agents.Heartbeat.SessionHealthHookMinInterval, 90*time.Second; got != want {
		t.Fatalf("Load() Agents.Heartbeat.SessionHealthHookMinInterval = %s, want %s", got, want)
	}
	if cfg.Limits.MaxConcurrentAgents != 22 {
		t.Fatalf("Load() Limits = %#v", cfg.Limits)
	}
	if got, want := cfg.Session.Limits.Timeout, 30*time.Minute; got != want {
		t.Fatalf("Load() Session.Limits.Timeout = %s, want %s", got, want)
	}
	if cfg.Permissions.Mode != PermissionModeApproveAll {
		t.Fatalf("Load() Permissions.Mode = %q, want %q", cfg.Permissions.Mode, PermissionModeApproveAll)
	}
	if cfg.Tools.HostedMCPEnabled {
		t.Fatal("Load() Tools.HostedMCPEnabled = true, want false")
	}
	if got, want := cfg.Tools.DefaultMaxResultBytes, int64(131072); got != want {
		t.Fatalf("Load() Tools.DefaultMaxResultBytes = %d, want %d", got, want)
	}
	if got, want := cfg.Tools.HostedMCP.BindNonceTTLSeconds, 45; got != want {
		t.Fatalf("Load() Tools.HostedMCP.BindNonceTTLSeconds = %d, want %d", got, want)
	}
	if got, want := cfg.Tools.Policy.ExternalDefault, ToolsExternalDefaultAsk; got != want {
		t.Fatalf("Load() Tools.Policy.ExternalDefault = %q, want %q", got, want)
	}
	if got, want := cfg.Tools.Policy.ApprovalTimeoutSeconds, 90; got != want {
		t.Fatalf("Load() Tools.Policy.ApprovalTimeoutSeconds = %d, want %d", got, want)
	}
	if got, want := cfg.Tools.Policy.TrustedSources, []string{
		"mcp:linear",
		"extension:linear",
	}; !slices.Equal(
		got,
		want,
	) {
		t.Fatalf("Load() Tools.Policy.TrustedSources = %#v, want %#v", got, want)
	}
	if cfg.Observability.RetentionDays != 14 || cfg.Observability.MaxGlobalBytes != 2048 ||
		cfg.Observability.AgentProbeTimeout != 9*time.Second {
		t.Fatalf("Load() Observability = %#v", cfg.Observability)
	}
	if cfg.Observability.Transcripts.SegmentBytes != 512 || cfg.Observability.Transcripts.MaxBytesPerSession != 4096 {
		t.Fatalf("Load() Transcript config = %#v", cfg.Observability.Transcripts)
	}
	if cfg.Log.Level != "debug" {
		t.Fatalf("Load() Log.Level = %q, want %q", cfg.Log.Level, "debug")
	}
	if cfg.Log.MaxSizeMB != 20 || cfg.Log.MaxBackups != 4 ||
		cfg.Log.MaxAgeDays != 14 || !cfg.Log.CompressBackups {
		t.Fatalf("Load() Log rotation = %#v", cfg.Log)
	}
	if cfg.Skills.Enabled {
		t.Fatal("Load() Skills.Enabled = true, want false")
	}
	if got, want := cfg.Skills.PollInterval, 5*time.Second; got != want {
		t.Fatalf("Load() Skills.PollInterval = %s, want %s", got, want)
	}
	if got, want := cfg.Skills.DisabledSkills, []string{"code-review", "agh"}; !slices.Equal(got, want) {
		t.Fatalf("Load() Skills.DisabledSkills = %#v, want %#v", got, want)
	}
	if got, want := cfg.Skills.AllowedMarketplaceMCP, []string{
		"@registry/skill-a",
		"@registry/skill-b",
	}; !slices.Equal(
		got,
		want,
	) {
		t.Fatalf("Load() Skills.AllowedMarketplaceMCP = %#v, want %#v", got, want)
	}
	if got, want := cfg.Skills.AllowedMarketplaceHooks, []string{
		"@registry/hook-a",
		"@registry/hook-b",
	}; !slices.Equal(
		got,
		want,
	) {
		t.Fatalf("Load() Skills.AllowedMarketplaceHooks = %#v, want %#v", got, want)
	}
	if got, want := cfg.Skills.Marketplace.Registry, "clawhub"; got != want {
		t.Fatalf("Load() Skills.Marketplace.Registry = %q, want %q", got, want)
	}
	if got, want := cfg.Skills.Marketplace.BaseURL, "https://registry.example.test/api/v1"; got != want {
		t.Fatalf("Load() Skills.Marketplace.BaseURL = %q, want %q", got, want)
	}
	if got, want := cfg.Extensions.Marketplace.Registry, "github"; got != want {
		t.Fatalf("Load() Extensions.Marketplace.Registry = %q, want %q", got, want)
	}
	if got, want := cfg.Extensions.Marketplace.BaseURL, "https://api.github.example.test"; got != want {
		t.Fatalf("Load() Extensions.Marketplace.BaseURL = %q, want %q", got, want)
	}
	userHome, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir() error = %v", err)
	}
	if !cfg.Memory.Enabled {
		t.Fatal("Load() Memory.Enabled = false, want true")
	}
	if got, want := cfg.Memory.GlobalDir, filepath.Join(userHome, "agh-memory-test"); got != want {
		t.Fatalf("Load() Memory.GlobalDir = %q, want %q", got, want)
	}
	if got, want := cfg.Roles.Dream.Agent, "claude"; got != want {
		t.Fatalf("Load() Roles.Dream.Agent = %q, want %q", got, want)
	}
	if got, want := cfg.Memory.Dream.MinHours, 48.0; got != want {
		t.Fatalf("Load() Memory.Dream.MinHours = %v, want %v", got, want)
	}
	if got, want := cfg.Memory.Dream.MinSessions, 5; got != want {
		t.Fatalf("Load() Memory.Dream.MinSessions = %d, want %d", got, want)
	}
	if got, want := cfg.Memory.Dream.CheckInterval, 45*time.Minute; got != want {
		t.Fatalf("Load() Memory.Dream.CheckInterval = %s, want %s", got, want)
	}
	if !cfg.Network.Enabled {
		t.Fatal("Load() Network.Enabled = false, want true")
	}
	if got, want := cfg.Network.MaxReplayAge, 600; got != want {
		t.Fatalf("Load() Network.MaxReplayAge = %d, want %d", got, want)
	}
	if got, want := cfg.Network.Live.Defaults.MaxWakes, 12; got != want {
		t.Fatalf("Load() Network.Live.Defaults.MaxWakes = %d, want %d", got, want)
	}
	if got, want := cfg.Network.Live.Limits.MaxWakes, 80; got != want {
		t.Fatalf("Load() Network.Live.Limits.MaxWakes = %d, want %d", got, want)
	}

	claude, err := cfg.ResolveProvider("claude")
	if err != nil {
		t.Fatalf("ResolveProvider() error = %v", err)
	}
	if claude.Command == "" || claude.Models.Default != "claude-opus" {
		t.Fatalf("ResolveProvider() = %#v", claude)
	}
	if slots := claude.EffectiveCredentialSlots(); len(slots) != 1 ||
		slots[0].TargetEnv != "ANTHROPIC_KEY" ||
		slots[0].SecretRef != "env:ANTHROPIC_KEY" {
		t.Fatalf("ResolveProvider() CredentialSlots = %#v, want ANTHROPIC_KEY slot", slots)
	}
	if len(claude.MCPServers) != 1 || claude.MCPServers[0].Name != "github" {
		t.Fatalf("ResolveProvider() MCPServers = %#v", claude.MCPServers)
	}
}

func TestLogConfigDefaultsAndValidation(t *testing.T) {
	t.Parallel()

	t.Run("ShouldExposeRotationDefaults", func(t *testing.T) {
		t.Parallel()

		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		cfg := DefaultWithHome(homePaths)
		if cfg.Log.MaxSizeMB != 10 || cfg.Log.MaxBackups != 5 || cfg.Log.MaxAgeDays != 30 {
			t.Fatalf("DefaultWithHome() Log = %#v, want rotation defaults", cfg.Log)
		}
		if cfg.Log.CompressBackups {
			t.Fatal("DefaultWithHome() Log.CompressBackups = true, want false")
		}
	})

	tests := []struct {
		name    string
		config  LogConfig
		wantErr string
	}{
		{
			name:   "ShouldAcceptValidRotationConfig",
			config: LogConfig{Level: "info", MaxSizeMB: 1, MaxBackups: 0, MaxAgeDays: 0},
		},
		{
			name:    "ShouldRejectNonPositiveMaxSize",
			config:  LogConfig{Level: "info", MaxSizeMB: 0, MaxBackups: 1, MaxAgeDays: 1},
			wantErr: "log.max_size_mb",
		},
		{
			name:    "ShouldRejectNegativeMaxBackups",
			config:  LogConfig{Level: "info", MaxSizeMB: 1, MaxBackups: -1, MaxAgeDays: 1},
			wantErr: "log.max_backups",
		},
		{
			name:    "ShouldRejectNegativeMaxAge",
			config:  LogConfig{Level: "info", MaxSizeMB: 1, MaxBackups: 1, MaxAgeDays: -1},
			wantErr: "log.max_age_days",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.config.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatal("Validate() error = nil, want non-nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestDaemonMemoryReportIntervalDefaultsAndValidation(t *testing.T) {
	t.Parallel()

	t.Run("Should expose the five minute default", func(t *testing.T) {
		t.Parallel()

		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		cfg := DefaultWithHome(homePaths)
		if got, want := cfg.Daemon.MemoryReportInterval, 5*time.Minute; got != want {
			t.Fatalf("DefaultWithHome() Daemon.MemoryReportInterval = %s, want %s", got, want)
		}
	})

	t.Run("Should expose the subprocess health escalation default", func(t *testing.T) {
		t.Parallel()

		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		cfg := DefaultWithHome(homePaths)
		if got, want := cfg.Daemon.SubprocessHealthEscalationThreshold, 3; got != want {
			t.Fatalf("DefaultWithHome() threshold = %d, want %d", got, want)
		}
	})

	t.Run("Should allow zero to disable reports", func(t *testing.T) {
		t.Parallel()

		cfg := DaemonConfig{
			Socket:         "/tmp/agh.sock",
			ReloadTimeouts: DefaultDaemonReloadTimeoutsConfig(),
		}
		if err := cfg.Validate(); err != nil {
			t.Fatalf("Validate() error = %v, want nil", err)
		}
	})

	t.Run("Should allow zero to disable subprocess health escalation", func(t *testing.T) {
		t.Parallel()

		cfg := DaemonConfig{
			Socket:         "/tmp/agh.sock",
			ReloadTimeouts: DefaultDaemonReloadTimeoutsConfig(),
		}
		if err := cfg.Validate(); err != nil {
			t.Fatalf("Validate() error = %v, want nil", err)
		}
	})

	t.Run("Should reject a negative interval", func(t *testing.T) {
		t.Parallel()

		cfg := DaemonConfig{
			Socket:               "/tmp/agh.sock",
			MemoryReportInterval: -time.Second,
			ReloadTimeouts:       DefaultDaemonReloadTimeoutsConfig(),
		}
		err := cfg.Validate()
		if err == nil {
			t.Fatal("Validate() error = nil, want negative interval rejection")
		}
		if !strings.Contains(err.Error(), daemonMemoryReportIntervalPath) {
			t.Fatalf("Validate() error = %v, want %q", err, daemonMemoryReportIntervalPath)
		}
	})

	t.Run("Should reject a negative subprocess health escalation threshold", func(t *testing.T) {
		t.Parallel()

		cfg := DaemonConfig{
			Socket:                              "/tmp/agh.sock",
			SubprocessHealthEscalationThreshold: -1,
			ReloadTimeouts:                      DefaultDaemonReloadTimeoutsConfig(),
		}
		err := cfg.Validate()
		if err == nil {
			t.Fatal("Validate() error = nil, want negative threshold rejection")
		}
		if !strings.Contains(err.Error(), daemonSubprocessHealthEscalationThresholdPath) {
			t.Fatalf("Validate() error = %v, want %q", err, daemonSubprocessHealthEscalationThresholdPath)
		}
	})
}

func TestLoadSandboxProfilesFromTOML(t *testing.T) {
	workspaceRoot := t.TempDir()
	homeRoot := filepath.Join(t.TempDir(), "home")
	t.Setenv("AGH_HOME", homeRoot)

	homePaths, err := ResolveHomePaths()
	if err != nil {
		t.Fatalf("ResolveHomePaths() error = %v", err)
	}
	if err := EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}

	writeFile(t, homePaths.ConfigFile, `
[defaults]
sandbox = "daytona-dev"

[sandboxes.local]
backend = "local"

[sandboxes.daytona-dev]
backend = "daytona"
sync_mode = "session-bidirectional"
persistence = "reuse"
runtime_root = "/home/daytona/workspace"

[sandboxes.daytona-dev.env]
NODE_ENV = "development"
AGH_PROFILE = "daytona"

[sandboxes.daytona-dev.network]
allow_public_ingress = false
allow_outbound = true
allow_list = ["api.example.test"]
deny_list = ["metadata.google.internal"]

[sandboxes.daytona-dev.daytona]
api_url = "https://app.daytona.io/api"
target = "team-default"
image = "ubuntu:24.04"
snapshot = "snap-agent-base"
class = "cpu-2"
auto_stop = "30m"
auto_archive = "24h"
`)

	cfg, err := Load(WithWorkspaceRoot(workspaceRoot))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got, want := cfg.Defaults.Sandbox, "daytona-dev"; got != want {
		t.Fatalf("Defaults.Sandbox = %q, want %q", got, want)
	}
	profile := cfg.Sandboxes["daytona-dev"]
	if profile.Backend != "daytona" || profile.Daytona.Snapshot != "snap-agent-base" {
		t.Fatalf("daytona profile = %#v, want parsed profile", profile)
	}
	if got, want := profile.Env["NODE_ENV"], "development"; got != want {
		t.Fatalf("profile Env[NODE_ENV] = %q, want %q", got, want)
	}

	resolved, err := cfg.ResolveSandbox(cfg.Defaults.Sandbox)
	if err != nil {
		t.Fatalf("ResolveSandbox() error = %v", err)
	}
	if resolved.Backend != sandbox.BackendDaytona {
		t.Fatalf("resolved.Backend = %q, want %q", resolved.Backend, sandbox.BackendDaytona)
	}
	if resolved.SyncMode != sandbox.SyncModeSessionBidirectional ||
		resolved.Persistence != sandbox.PersistenceReuse {
		t.Fatalf("resolved sync/persistence = %q/%q", resolved.SyncMode, resolved.Persistence)
	}
	if resolved.Daytona == nil {
		t.Fatal("resolved.Daytona = nil, want profile")
	}
	if got, want := resolved.Daytona.StartupSource, sandbox.DaytonaStartupSourceSnapshot; got != want {
		t.Fatalf("resolved Daytona startup source = %q, want %q", got, want)
	}
	if got, want := resolved.Daytona.StartupRef, "snap-agent-base"; got != want {
		t.Fatalf("resolved Daytona startup ref = %q, want %q", got, want)
	}
}

func TestResolveSandboxErrorContract(t *testing.T) {
	t.Parallel()

	t.Run("Should expose missing sandbox profiles through a sentinel error", func(t *testing.T) {
		t.Parallel()

		cfg := Config{Sandboxes: map[string]SandboxProfile{}}
		_, err := cfg.ResolveSandbox("missing-profile")
		if !errors.Is(err, ErrSandboxProfileNotFound) {
			t.Fatalf("ResolveSandbox() error = %v, want ErrSandboxProfileNotFound", err)
		}
	})
}

func TestDaytonaSnapshotWinsOverImageInResolvedProfile(t *testing.T) {
	t.Parallel()

	resolved, err := (SandboxProfile{
		Backend: "daytona",
		Daytona: DaytonaProfile{
			Image:    "ubuntu:24.04",
			Snapshot: "snap-prebuilt",
		},
	}).Resolve("daytona-dev")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolved.Daytona == nil {
		t.Fatal("resolved.Daytona = nil, want profile")
	}
	if got, want := resolved.Daytona.StartupSource, sandbox.DaytonaStartupSourceSnapshot; got != want {
		t.Fatalf("StartupSource = %q, want %q", got, want)
	}
	if got, want := resolved.Daytona.StartupRef, "snap-prebuilt"; got != want {
		t.Fatalf("StartupRef = %q, want %q", got, want)
	}
	if got, want := resolved.Daytona.Image, "ubuntu:24.04"; got != want {
		t.Fatalf("Image = %q, want preserved fallback %q", got, want)
	}
}

func TestSandboxProfileValidationRejectsInvalidBackend(t *testing.T) {
	t.Parallel()

	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	cfg := DefaultWithHome(homePaths)
	cfg.Sandboxes["bad"] = SandboxProfile{Backend: "docker"}

	err = cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want invalid backend")
	}
	if !strings.Contains(err.Error(), "sandboxes.bad.backend") {
		t.Fatalf("Validate() error = %v, want sandboxes.bad.backend", err)
	}
}

func TestDefaultWithHomeIncludesSoulDefaults(t *testing.T) {
	t.Parallel()

	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}

	cfg := DefaultWithHome(homePaths)
	if !cfg.Agents.Soul.Enabled {
		t.Fatal("DefaultWithHome() Agents.Soul.Enabled = false, want true")
	}
	if got, want := cfg.Agents.Soul.MaxBodyBytes, int64(32768); got != want {
		t.Fatalf("DefaultWithHome() Agents.Soul.MaxBodyBytes = %d, want %d", got, want)
	}
	if got, want := cfg.Agents.Soul.ContextProjectionBytes, int64(2048); got != want {
		t.Fatalf("DefaultWithHome() Agents.Soul.ContextProjectionBytes = %d, want %d", got, want)
	}
}

func TestSoulConfigValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  SoulConfig
		wantErr string
	}{
		{
			name:    "Should reject zero max body bytes",
			config:  SoulConfig{Enabled: true, MaxBodyBytes: 0, ContextProjectionBytes: 1},
			wantErr: "agents.soul.max_body_bytes",
		},
		{
			name:    "Should reject zero context projection bytes",
			config:  SoulConfig{Enabled: true, MaxBodyBytes: 1, ContextProjectionBytes: 0},
			wantErr: "agents.soul.context_projection_bytes",
		},
		{
			name:    "Should reject context projection above max body bytes",
			config:  SoulConfig{Enabled: true, MaxBodyBytes: 10, ContextProjectionBytes: 11},
			wantErr: "agents.soul.context_projection_bytes must be <=",
		},
		{
			name:   "Should accept disabled soul with valid limits",
			config: SoulConfig{Enabled: false, MaxBodyBytes: 10, ContextProjectionBytes: 5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.config.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatal("Validate() error = nil, want non-nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestHeartbeatConfigDefaultsAndValidation(t *testing.T) {
	t.Parallel()

	t.Run("Should include built in heartbeat defaults", func(t *testing.T) {
		t.Parallel()

		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}

		cfg := DefaultWithHome(homePaths)
		if !cfg.Agents.Heartbeat.Enabled {
			t.Fatal("DefaultWithHome() Agents.Heartbeat.Enabled = false, want true")
		}
		if got, want := cfg.Agents.Heartbeat.MaxBodyBytes, int64(32768); got != want {
			t.Fatalf("DefaultWithHome() Agents.Heartbeat.MaxBodyBytes = %d, want %d", got, want)
		}
		if got, want := cfg.Agents.Heartbeat.ContextProjectionBytes, int64(4096); got != want {
			t.Fatalf("DefaultWithHome() Agents.Heartbeat.ContextProjectionBytes = %d, want %d", got, want)
		}
		if got, want := cfg.Agents.Heartbeat.MinInterval, 5*time.Minute; got != want {
			t.Fatalf("DefaultWithHome() Agents.Heartbeat.MinInterval = %s, want %s", got, want)
		}
		if got, want := cfg.Agents.Heartbeat.DefaultInterval, 30*time.Minute; got != want {
			t.Fatalf("DefaultWithHome() Agents.Heartbeat.DefaultInterval = %s, want %s", got, want)
		}
		if got, want := cfg.Agents.Heartbeat.WakeCooldown, time.Minute; got != want {
			t.Fatalf("DefaultWithHome() Agents.Heartbeat.WakeCooldown = %s, want %s", got, want)
		}
		if got, want := cfg.Agents.Heartbeat.MaxWakesPerCycle, 25; got != want {
			t.Fatalf("DefaultWithHome() Agents.Heartbeat.MaxWakesPerCycle = %d, want %d", got, want)
		}
		if !cfg.Agents.Heartbeat.ActiveSessionOnly {
			t.Fatal("DefaultWithHome() Agents.Heartbeat.ActiveSessionOnly = false, want true")
		}
		if !cfg.Agents.Heartbeat.AllowActiveHoursPreferences {
			t.Fatal("DefaultWithHome() Agents.Heartbeat.AllowActiveHoursPreferences = false, want true")
		}
		if got, want := cfg.Agents.Heartbeat.WakeEventRetention, 168*time.Hour; got != want {
			t.Fatalf("DefaultWithHome() Agents.Heartbeat.WakeEventRetention = %s, want %s", got, want)
		}
		if got, want := cfg.Agents.Heartbeat.SessionHealthStaleAfter, 2*time.Minute; got != want {
			t.Fatalf("DefaultWithHome() Agents.Heartbeat.SessionHealthStaleAfter = %s, want %s", got, want)
		}
		if got, want := cfg.Agents.Heartbeat.SessionHealthHookMinInterval, time.Minute; got != want {
			t.Fatalf("DefaultWithHome() Agents.Heartbeat.SessionHealthHookMinInterval = %s, want %s", got, want)
		}
	})

	base := DefaultHeartbeatConfig()
	tests := []struct {
		name    string
		mutate  func(*HeartbeatConfig)
		wantErr string
	}{
		{
			name:    "Should reject zero max body bytes",
			mutate:  func(cfg *HeartbeatConfig) { cfg.MaxBodyBytes = 0 },
			wantErr: "agents.heartbeat.max_body_bytes",
		},
		{
			name:    "Should reject unbounded max body bytes",
			mutate:  func(cfg *HeartbeatConfig) { cfg.MaxBodyBytes = 2 << 20 },
			wantErr: "agents.heartbeat.max_body_bytes must be <=",
		},
		{
			name:    "Should reject zero context projection bytes",
			mutate:  func(cfg *HeartbeatConfig) { cfg.ContextProjectionBytes = 0 },
			wantErr: "agents.heartbeat.context_projection_bytes",
		},
		{
			name: "Should reject context projection above max body bytes",
			mutate: func(cfg *HeartbeatConfig) {
				cfg.MaxBodyBytes = 128
				cfg.ContextProjectionBytes = 256
			},
			wantErr: "agents.heartbeat.context_projection_bytes must be <=",
		},
		{
			name:    "Should reject zero min interval",
			mutate:  func(cfg *HeartbeatConfig) { cfg.MinInterval = 0 },
			wantErr: "agents.heartbeat.min_interval",
		},
		{
			name: "Should reject min interval above default interval",
			mutate: func(cfg *HeartbeatConfig) {
				cfg.MinInterval = time.Hour
				cfg.DefaultInterval = time.Minute
			},
			wantErr: "agents.heartbeat.min_interval must be <=",
		},
		{
			name:    "Should reject zero wake cooldown",
			mutate:  func(cfg *HeartbeatConfig) { cfg.WakeCooldown = 0 },
			wantErr: "agents.heartbeat.wake_cooldown",
		},
		{
			name:    "Should reject zero max wakes per cycle",
			mutate:  func(cfg *HeartbeatConfig) { cfg.MaxWakesPerCycle = 0 },
			wantErr: "agents.heartbeat.max_wakes_per_cycle",
		},
		{
			name:    "Should reject wake event retention below one hour",
			mutate:  func(cfg *HeartbeatConfig) { cfg.WakeEventRetention = 59 * time.Minute },
			wantErr: "agents.heartbeat.wake_event_retention",
		},
		{
			name:    "Should reject zero session health stale interval",
			mutate:  func(cfg *HeartbeatConfig) { cfg.SessionHealthStaleAfter = 0 },
			wantErr: "agents.heartbeat.session_health_stale_after",
		},
		{
			name:    "Should reject zero session health hook interval",
			mutate:  func(cfg *HeartbeatConfig) { cfg.SessionHealthHookMinInterval = 0 },
			wantErr: "agents.heartbeat.session_health_hook_min_interval",
		},
		{
			name:   "Should accept disabled heartbeat with valid limits",
			mutate: func(cfg *HeartbeatConfig) { cfg.Enabled = false },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := base
			tt.mutate(&cfg)
			err := cfg.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatal("Validate() error = nil, want non-nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestSandboxProfileValidationRejectsInvalidSyncMode(t *testing.T) {
	t.Parallel()

	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	cfg := DefaultWithHome(homePaths)
	cfg.Sandboxes["bad"] = SandboxProfile{
		Backend:  "daytona",
		SyncMode: "continuous",
	}

	err = cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want invalid sync_mode")
	}
	if !strings.Contains(err.Error(), "sandboxes.bad.sync_mode") {
		t.Fatalf("Validate() error = %v, want sandboxes.bad.sync_mode", err)
	}
}

func TestSandboxProfileValidationRejectsInvalidPersistence(t *testing.T) {
	t.Parallel()

	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	cfg := DefaultWithHome(homePaths)
	cfg.Sandboxes["bad"] = SandboxProfile{
		Backend:     "daytona",
		Persistence: "forever",
	}

	err = cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want invalid persistence")
	}
	if !strings.Contains(err.Error(), "sandboxes.bad.persistence") {
		t.Fatalf("Validate() error = %v, want sandboxes.bad.persistence", err)
	}
}

func TestLoadWorkspaceOverridesGlobalValues(t *testing.T) {
	workspaceRoot := t.TempDir()
	homeRoot := filepath.Join(t.TempDir(), "home")
	t.Setenv("AGH_HOME", homeRoot)

	homePaths, err := ResolveHomePaths()
	if err != nil {
		t.Fatalf("ResolveHomePaths() error = %v", err)
	}
	if err := EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}

	writeFile(t, homePaths.ConfigFile, `
[http]
host = "localhost"
port = 2123

	[providers.claude]
	auth_mode = "bound_secret"
	[providers.claude.models]
	default = "global-model"
	[[providers.claude.models.curated]]
	id = "global-model"
	display_name = "Global Model"
	[[providers.claude.credential_slots]]
	name = "api_key"
	target_env = "GLOBAL_KEY"
	secret_ref = "env:GLOBAL_KEY"
	kind = "api_key"
	required = true

[roles.auto_title]
enabled = false

[session.limits]
timeout = "20m"

[session.compaction]
enabled = true
pressure_threshold = 0.80
max_attempts_per_turn = 2
failure_cooldown = "5m"

[skills]
enabled = true
disabled_skills = ["global-skill"]
poll_interval = "3s"
allowed_marketplace_mcp = ["@global/skill"]
allowed_marketplace_hooks = ["@global/hook"]

[skills.marketplace]
registry = "clawhub"
base_url = "https://global.example.test/api/v1"
`)
	writeFile(t, filepath.Join(workspaceRoot, DirName, ConfigName), `
[http]
port = 4242

[providers.claude]
[providers.claude.models]
default = "workspace-model"
[[providers.claude.models.curated]]
id = "workspace-model"
display_name = "Workspace Model"
reasoning_efforts = ["low", "high"]
default_reasoning_effort = "high"

[roles.auto_title]
enabled = true

[session.limits]
timeout = "45m"

[session.compaction]
pressure_threshold = 0.90

[skills]
enabled = false
disabled_skills = ["workspace-skill"]
poll_interval = "9s"
allowed_marketplace_mcp = ["@workspace/skill"]
allowed_marketplace_hooks = ["@workspace/hook"]

[skills.marketplace]
registry = "clawhub"
base_url = "https://workspace.example.test/api/v1"
`)

	cfg, err := Load(WithWorkspaceRoot(workspaceRoot))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.HTTP.Host != "localhost" || cfg.HTTP.Port != 4242 {
		t.Fatalf("Load() HTTP = %#v", cfg.HTTP)
	}
	if got, want := cfg.Session.Limits.Timeout, 45*time.Minute; got != want {
		t.Fatalf("Load() Session.Limits.Timeout = %s, want %s", got, want)
	}
	if !cfg.Roles.AutoTitle.Enabled {
		t.Fatal("Load() Roles.AutoTitle.Enabled = false, want workspace override true")
	}
	if got, want := cfg.Session.Compaction.PressureThreshold, 0.90; got != want {
		t.Fatalf("Load() Session.Compaction.PressureThreshold = %v, want %v", got, want)
	}
	if !cfg.Session.Compaction.Enabled || cfg.Session.Compaction.MaxAttemptsPerTurn != 2 ||
		cfg.Session.Compaction.FailureCooldown != 5*time.Minute {
		t.Fatalf("Load() Session.Compaction = %#v, want inherited global guards", cfg.Session.Compaction)
	}

	claude, err := cfg.ResolveProvider("claude")
	if err != nil {
		t.Fatalf("ResolveProvider() error = %v", err)
	}
	if claude.Models.Default != "workspace-model" {
		t.Fatalf("ResolveProvider() Models.Default = %q, want %q", claude.Models.Default, "workspace-model")
	}
	if len(claude.Models.Curated) != 1 {
		t.Fatalf("ResolveProvider() Models.Curated = %#v, want one workspace model", claude.Models.Curated)
	}
	if got, want := claude.Models.Curated[0].ID, "workspace-model"; got != want {
		t.Fatalf("ResolveProvider() Models.Curated[0].ID = %q, want %q", got, want)
	}
	if got, want := claude.Models.Curated[0].DisplayName, "Workspace Model"; got != want {
		t.Fatalf("ResolveProvider() Models.Curated[0].DisplayName = %q, want %q", got, want)
	}
	if got, want := claude.Models.Curated[0].DefaultReasoningEffort, "high"; got != want {
		t.Fatalf("ResolveProvider() Models.Curated[0].DefaultReasoningEffort = %q, want %q", got, want)
	}
	if slots := claude.EffectiveCredentialSlots(); len(slots) != 1 ||
		slots[0].TargetEnv != "GLOBAL_KEY" ||
		slots[0].SecretRef != "env:GLOBAL_KEY" {
		t.Fatalf("ResolveProvider() CredentialSlots = %#v, want GLOBAL_KEY slot", slots)
	}
	if cfg.Skills.Enabled {
		t.Fatal("Load() Skills.Enabled = true, want false")
	}
	if got, want := cfg.Skills.AllowedMarketplaceMCP, []string{"@workspace/skill"}; !slices.Equal(got, want) {
		t.Fatalf("Load() Skills.AllowedMarketplaceMCP = %#v, want %#v", got, want)
	}
	if got, want := cfg.Skills.AllowedMarketplaceHooks, []string{"@workspace/hook"}; !slices.Equal(got, want) {
		t.Fatalf("Load() Skills.AllowedMarketplaceHooks = %#v, want %#v", got, want)
	}
	if got, want := cfg.Skills.Marketplace.Registry, "clawhub"; got != want {
		t.Fatalf("Load() Skills.Marketplace.Registry = %q, want %q", got, want)
	}
	if got, want := cfg.Skills.Marketplace.BaseURL, "https://workspace.example.test/api/v1"; got != want {
		t.Fatalf("Load() Skills.Marketplace.BaseURL = %q, want %q", got, want)
	}
	if got, want := cfg.Skills.PollInterval, 9*time.Second; got != want {
		t.Fatalf("Load() Skills.PollInterval = %s, want %s", got, want)
	}
	if got, want := cfg.Skills.DisabledSkills, []string{"workspace-skill"}; !slices.Equal(got, want) {
		t.Fatalf("Load() Skills.DisabledSkills = %#v, want %#v", got, want)
	}
}

func TestLoadWorkspaceOverridesAgentsSoulConfig(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	if err := EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}

	writeFile(t, homePaths.ConfigFile, `
[agents.soul]
enabled = false
max_body_bytes = 40000
context_projection_bytes = 2000
`)
	writeFile(t, filepath.Join(workspaceRoot, DirName, ConfigName), `
[agents.soul]
context_projection_bytes = 4096
`)

	cfg, err := LoadForHome(homePaths, WithWorkspaceRoot(workspaceRoot))
	if err != nil {
		t.Fatalf("LoadForHome() error = %v", err)
	}

	if cfg.Agents.Soul.Enabled {
		t.Fatal("LoadForHome() Agents.Soul.Enabled = true, want global false")
	}
	if got, want := cfg.Agents.Soul.MaxBodyBytes, int64(40000); got != want {
		t.Fatalf("LoadForHome() Agents.Soul.MaxBodyBytes = %d, want %d", got, want)
	}
	if got, want := cfg.Agents.Soul.ContextProjectionBytes, int64(4096); got != want {
		t.Fatalf("LoadForHome() Agents.Soul.ContextProjectionBytes = %d, want %d", got, want)
	}
}

func TestLoadWorkspaceOverridesAgentsHeartbeatConfig(t *testing.T) {
	t.Parallel()

	t.Run("Should merge global and workspace heartbeat overlays", func(t *testing.T) {
		t.Parallel()

		workspaceRoot := t.TempDir()
		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		if err := EnsureHomeLayout(homePaths); err != nil {
			t.Fatalf("EnsureHomeLayout() error = %v", err)
		}

		writeFile(t, homePaths.ConfigFile, `
[agents.heartbeat]
enabled = false
max_body_bytes = 60000
context_projection_bytes = 3000
min_interval = "15m"
default_interval = "45m"
wake_cooldown = "3m"
max_wakes_per_cycle = 4
active_session_only = true
allow_active_hours_preferences = true
wake_event_retention = "48h"
session_health_stale_after = "4m"
session_health_hook_min_interval = "2m"
`)
		writeFile(t, filepath.Join(workspaceRoot, DirName, ConfigName), `
[agents.heartbeat]
context_projection_bytes = 4096
default_interval = "1h"
allow_active_hours_preferences = false
`)

		cfg, err := LoadForHome(homePaths, WithWorkspaceRoot(workspaceRoot))
		if err != nil {
			t.Fatalf("LoadForHome() error = %v", err)
		}

		if cfg.Agents.Heartbeat.Enabled {
			t.Fatal("LoadForHome() Agents.Heartbeat.Enabled = true, want global false")
		}
		if got, want := cfg.Agents.Heartbeat.MaxBodyBytes, int64(60000); got != want {
			t.Fatalf("LoadForHome() Agents.Heartbeat.MaxBodyBytes = %d, want %d", got, want)
		}
		if got, want := cfg.Agents.Heartbeat.ContextProjectionBytes, int64(4096); got != want {
			t.Fatalf("LoadForHome() Agents.Heartbeat.ContextProjectionBytes = %d, want %d", got, want)
		}
		if got, want := cfg.Agents.Heartbeat.MinInterval, 15*time.Minute; got != want {
			t.Fatalf("LoadForHome() Agents.Heartbeat.MinInterval = %s, want %s", got, want)
		}
		if got, want := cfg.Agents.Heartbeat.DefaultInterval, time.Hour; got != want {
			t.Fatalf("LoadForHome() Agents.Heartbeat.DefaultInterval = %s, want %s", got, want)
		}
		if got, want := cfg.Agents.Heartbeat.WakeCooldown, 3*time.Minute; got != want {
			t.Fatalf("LoadForHome() Agents.Heartbeat.WakeCooldown = %s, want %s", got, want)
		}
		if got, want := cfg.Agents.Heartbeat.MaxWakesPerCycle, 4; got != want {
			t.Fatalf("LoadForHome() Agents.Heartbeat.MaxWakesPerCycle = %d, want %d", got, want)
		}
		if !cfg.Agents.Heartbeat.ActiveSessionOnly {
			t.Fatal("LoadForHome() Agents.Heartbeat.ActiveSessionOnly = false, want true")
		}
		if cfg.Agents.Heartbeat.AllowActiveHoursPreferences {
			t.Fatal("LoadForHome() Agents.Heartbeat.AllowActiveHoursPreferences = true, want workspace false")
		}
		if got, want := cfg.Agents.Heartbeat.WakeEventRetention, 48*time.Hour; got != want {
			t.Fatalf("LoadForHome() Agents.Heartbeat.WakeEventRetention = %s, want %s", got, want)
		}
		if got, want := cfg.Agents.Heartbeat.SessionHealthStaleAfter, 4*time.Minute; got != want {
			t.Fatalf("LoadForHome() Agents.Heartbeat.SessionHealthStaleAfter = %s, want %s", got, want)
		}
		if got, want := cfg.Agents.Heartbeat.SessionHealthHookMinInterval, 2*time.Minute; got != want {
			t.Fatalf("LoadForHome() Agents.Heartbeat.SessionHealthHookMinInterval = %s, want %s", got, want)
		}
	})
}

func TestLoadMergesTopLevelMCPServersAcrossConfigAndJSONSidecars(t *testing.T) {
	workspaceRoot := t.TempDir()
	homeRoot := filepath.Join(t.TempDir(), "home")
	t.Setenv("AGH_HOME", homeRoot)

	homePaths, err := ResolveHomePaths()
	if err != nil {
		t.Fatalf("ResolveHomePaths() error = %v", err)
	}
	if err := EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}

	writeFile(t, homePaths.ConfigFile, `
[[mcp_servers]]
name = "partial"
command = "global-command"

[[mcp_servers]]
name = "sidecar"
command = "global-inline"
args = ["--inline"]
`)
	writeFile(t, filepath.Join(homePaths.HomeDir, MCPJSONName), `{
  "mcpServers": {
    "sidecar": {
      "command": "global-sidecar"
    }
  }
}`)
	writeFile(t, filepath.Join(workspaceRoot, DirName, ConfigName), `
[[mcp_servers]]
name = "partial"
args = ["--workspace"]
env = { WORKSPACE = "1" }

[[mcp_servers]]
name = "workspace-only"
command = "workspace-command"

[[mcp_servers]]
name = "replace-me"
command = "workspace-inline"
`)
	writeFile(t, filepath.Join(workspaceRoot, DirName, MCPJSONName), `{
  "mcp_servers": {
    "replace-me": {
      "command": "workspace-sidecar"
    },
    "workspace-json": {
      "command": "workspace-json-command"
    }
  }
}`)

	cfg, err := Load(WithWorkspaceRoot(workspaceRoot))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got, want := len(cfg.MCPServers), 5; got != want {
		t.Fatalf("Load() MCPServers len = %d, want %d (%#v)", got, want, cfg.MCPServers)
	}
	if got, want := cfg.MCPServers[0].Name, "partial"; got != want {
		t.Fatalf("Load() MCPServers[0].Name = %q, want %q", got, want)
	}
	if got, want := cfg.MCPServers[0].Command, "global-command"; got != want {
		t.Fatalf("Load() partial.Command = %q, want %q", got, want)
	}
	if got, want := cfg.MCPServers[0].Args[0], "--workspace"; got != want {
		t.Fatalf("Load() partial.Args = %#v, want workspace field merge", cfg.MCPServers[0].Args)
	}
	if got, want := cfg.MCPServers[0].Env["WORKSPACE"], "1"; got != want {
		t.Fatalf("Load() partial.Env[WORKSPACE] = %q, want %q", got, want)
	}
	if got, want := cfg.MCPServers[1].Command, "global-sidecar"; got != want {
		t.Fatalf("Load() sidecar.Command = %q, want %q", got, want)
	}
	if got := len(cfg.MCPServers[1].Args); got != 0 {
		t.Fatalf("Load() sidecar.Args = %#v, want same-scope sidecar replacement", cfg.MCPServers[1].Args)
	}
	if got, want := cfg.MCPServers[2].Command, "workspace-command"; got != want {
		t.Fatalf("Load() workspace-only.Command = %q, want %q", got, want)
	}
	if got, want := cfg.MCPServers[3].Command, "workspace-sidecar"; got != want {
		t.Fatalf("Load() replace-me.Command = %q, want %q", got, want)
	}
	if got, want := cfg.MCPServers[4].Command, "workspace-json-command"; got != want {
		t.Fatalf("Load() workspace-json.Command = %q, want %q", got, want)
	}
}

func TestLoadSupportsRemoteMCPAuthFieldsInTOML(t *testing.T) {
	workspaceRoot := t.TempDir()
	homeRoot := filepath.Join(t.TempDir(), "home")
	t.Setenv("AGH_HOME", homeRoot)

	homePaths, err := ResolveHomePaths()
	if err != nil {
		t.Fatalf("ResolveHomePaths() error = %v", err)
	}
	if err := EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}

	writeFile(t, homePaths.ConfigFile, `
[[mcp_servers]]
name = "linear"
transport = "sse"
url = "https://mcp.example/sse"

[mcp_servers.auth]
type = "oauth2_pkce"
authorization_url = "https://auth.example/authorize"
token_url = "https://auth.example/token"
client_id = "client-id"
client_secret_ref = "env:LINEAR_CLIENT_SECRET"
scopes = ["read"]

[[providers.codex.mcp_servers]]
name = "remote-provider"
transport = "http"
url = "https://provider.example/mcp"

[providers.codex.mcp_servers.auth]
type = "oauth2_pkce"
issuer_url = "https://issuer.example"
client_id = "provider-client"
scopes = ["tools"]
`)

	cfg, err := Load(WithWorkspaceRoot(workspaceRoot))
	if err != nil {
		t.Fatalf("Load(remote MCP TOML) error = %v", err)
	}

	if got, want := len(cfg.MCPServers), 1; got != want {
		t.Fatalf("Load() MCPServers len = %d, want %d (%#v)", got, want, cfg.MCPServers)
	}
	linear := cfg.MCPServers[0]
	if got, want := linear.Transport, MCPServerTransportSSE; got != want {
		t.Fatalf("Load() linear.Transport = %q, want %q", got, want)
	}
	if got, want := linear.URL, "https://mcp.example/sse"; got != want {
		t.Fatalf("Load() linear.URL = %q, want %q", got, want)
	}
	if got, want := linear.Auth.Type, MCPAuthTypeOAuth2PKCE; got != want {
		t.Fatalf("Load() linear.Auth.Type = %q, want %q", got, want)
	}
	if got, want := linear.Auth.AuthorizationURL, "https://auth.example/authorize"; got != want {
		t.Fatalf("Load() linear.Auth.AuthorizationURL = %q, want %q", got, want)
	}
	if got, want := linear.Auth.TokenURL, "https://auth.example/token"; got != want {
		t.Fatalf("Load() linear.Auth.TokenURL = %q, want %q", got, want)
	}
	if got, want := linear.Auth.ClientSecretRef, "env:LINEAR_CLIENT_SECRET"; got != want {
		t.Fatalf("Load() linear.Auth.ClientSecretRef = %q, want %q", got, want)
	}
	if got, want := linear.Auth.Scopes, []string{"read"}; !slices.Equal(got, want) {
		t.Fatalf("Load() linear.Auth.Scopes = %#v, want %#v", got, want)
	}

	codex, err := cfg.ResolveProvider("codex")
	if err != nil {
		t.Fatalf("ResolveProvider(codex) error = %v", err)
	}
	if got, want := len(codex.MCPServers), 1; got != want {
		t.Fatalf("ResolveProvider(codex) MCPServers len = %d, want %d (%#v)", got, want, codex.MCPServers)
	}
	providerRemote := codex.MCPServers[0]
	if got, want := providerRemote.Transport, MCPServerTransportHTTP; got != want {
		t.Fatalf("provider remote Transport = %q, want %q", got, want)
	}
	if got, want := providerRemote.Auth.IssuerURL, "https://issuer.example"; got != want {
		t.Fatalf("provider remote IssuerURL = %q, want %q", got, want)
	}
	if got, want := providerRemote.Auth.ClientID, "provider-client"; got != want {
		t.Fatalf("provider remote ClientID = %q, want %q", got, want)
	}
}

func TestSessionLimitsConfigValidateRejectsNegativeTimeout(t *testing.T) {
	t.Run("Should reject negative timeout", func(t *testing.T) {
		t.Parallel()

		cfg := SessionLimitsConfig{Timeout: -time.Second}
		err := cfg.Validate()
		if err == nil {
			t.Fatal("SessionLimitsConfig.Validate() error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "session.limits.timeout") {
			t.Fatalf("SessionLimitsConfig.Validate() error = %v, want session.limits.timeout context", err)
		}
	})
}

func TestSessionCompactionConfigDefaultsAndValidation(t *testing.T) {
	t.Parallel()

	t.Run("Should expose the pressure compaction defaults", func(t *testing.T) {
		t.Parallel()

		cfg := DefaultSessionCompactionConfig()
		if !cfg.Enabled || cfg.PressureThreshold != 0.85 || cfg.MaxAttemptsPerTurn != 1 ||
			cfg.FailureCooldown != 10*time.Minute {
			t.Fatalf("DefaultSessionCompactionConfig() = %#v", cfg)
		}
		if err := cfg.Validate(); err != nil {
			t.Fatalf("DefaultSessionCompactionConfig().Validate() error = %v", err)
		}
	})

	for _, tc := range []struct {
		name   string
		mutate func(*SessionCompactionConfig)
		field  string
	}{
		{
			name: "Should reject pressure above one",
			mutate: func(cfg *SessionCompactionConfig) {
				cfg.PressureThreshold = 1.01
			},
			field: "pressure_threshold",
		},
		{
			name: "Should reject negative pressure",
			mutate: func(cfg *SessionCompactionConfig) {
				cfg.PressureThreshold = -0.01
			},
			field: "pressure_threshold",
		},
		{
			name: "Should reject NaN pressure",
			mutate: func(cfg *SessionCompactionConfig) {
				cfg.PressureThreshold = math.NaN()
			},
			field: "pressure_threshold",
		},
		{
			name: "Should reject infinite pressure",
			mutate: func(cfg *SessionCompactionConfig) {
				cfg.PressureThreshold = math.Inf(1)
			},
			field: "pressure_threshold",
		},
		{
			name: "Should reject a zero attempt cap",
			mutate: func(cfg *SessionCompactionConfig) {
				cfg.MaxAttemptsPerTurn = 0
			},
			field: "max_attempts_per_turn",
		},
		{
			name: "Should reject a negative failure cooldown",
			mutate: func(cfg *SessionCompactionConfig) {
				cfg.FailureCooldown = -time.Second
			},
			field: "failure_cooldown",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := DefaultSessionCompactionConfig()
			tc.mutate(&cfg)
			err := cfg.Validate()
			if err == nil || !strings.Contains(err.Error(), "session.compaction."+tc.field) {
				t.Fatalf("SessionCompactionConfig.Validate() error = %v, want %s context", err, tc.field)
			}
		})
	}

	t.Run("Should accept zero pressure as an explicit disable switch", func(t *testing.T) {
		t.Parallel()

		cfg := DefaultSessionCompactionConfig()
		cfg.PressureThreshold = 0
		if err := cfg.Validate(); err != nil {
			t.Fatalf("SessionCompactionConfig.Validate(zero pressure) error = %v", err)
		}
	})
}

func TestSessionSupervisionConfigValidateRejectsWarningAfterTimeout(t *testing.T) {
	t.Run("Should reject warning threshold after timeout", func(t *testing.T) {
		t.Parallel()

		cfg := DefaultSessionSupervisionConfig()
		cfg.InactivityWarningAfter = 2 * time.Minute
		cfg.InactivityTimeout = time.Minute

		err := cfg.Validate()
		if err == nil {
			t.Fatal("SessionSupervisionConfig.Validate() error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "session.supervision.inactivity_warning_after") ||
			!strings.Contains(err.Error(), "session.supervision.inactivity_timeout") {
			t.Fatalf("SessionSupervisionConfig.Validate() error = %v, want threshold context", err)
		}
	})
}

func TestSessionSupervisionConfigValidateRejectsNegativePromptDeadline(t *testing.T) {
	t.Run("Should reject negative prompt deadline", func(t *testing.T) {
		t.Parallel()

		cfg := DefaultSessionSupervisionConfig()
		cfg.PromptDeadline = -time.Second

		err := cfg.Validate()
		if err == nil {
			t.Fatal("SessionSupervisionConfig.Validate() error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "session.supervision.prompt_deadline") {
			t.Fatalf("SessionSupervisionConfig.Validate() error = %v, want prompt deadline context", err)
		}
	})
}

func TestObservabilityConfigValidateRetentionDays(t *testing.T) {
	t.Run("Should allow zero as keep history", func(t *testing.T) {
		t.Parallel()

		cfg := validObservabilityConfigForTest()
		cfg.RetentionDays = 0

		if err := cfg.Validate(); err != nil {
			t.Fatalf("ObservabilityConfig.Validate() error = %v, want nil for keep-history retention", err)
		}
	})

	t.Run("Should reject negative retention days", func(t *testing.T) {
		t.Parallel()

		cfg := validObservabilityConfigForTest()
		cfg.RetentionDays = -1

		err := cfg.Validate()
		if err == nil {
			t.Fatal("ObservabilityConfig.Validate() error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "observability.retention_days must be zero or positive") {
			t.Fatalf("ObservabilityConfig.Validate() error = %v, want retention_days context", err)
		}
	})

	t.Run("Should reject negative agent probe timeout", func(t *testing.T) {
		t.Parallel()

		cfg := validObservabilityConfigForTest()
		cfg.AgentProbeTimeout = -time.Second

		err := cfg.Validate()
		if err == nil {
			t.Fatal("ObservabilityConfig.Validate() error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "observability.agent_probe_timeout must be zero or positive") {
			t.Fatalf("ObservabilityConfig.Validate() error = %v, want agent_probe_timeout context", err)
		}
	})
}

func validObservabilityConfigForTest() ObservabilityConfig {
	return ObservabilityConfig{
		Enabled:        true,
		MaxGlobalBytes: 1,
		Transcripts: ObservabilityTranscriptConfig{
			Enabled:            true,
			SegmentBytes:       1,
			MaxBytesPerSession: 1,
		},
	}
}

func TestLoadWorkspaceAddsValuesWithoutClobberingGlobal(t *testing.T) {
	workspaceRoot := t.TempDir()
	homeRoot := filepath.Join(t.TempDir(), "home")
	t.Setenv("AGH_HOME", homeRoot)

	homePaths, err := ResolveHomePaths()
	if err != nil {
		t.Fatalf("ResolveHomePaths() error = %v", err)
	}
	if err := EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}

	writeFile(t, homePaths.ConfigFile, `
[observability]
enabled = true
retention_days = 7
max_global_bytes = 1000

[observability.transcripts]
enabled = true
segment_bytes = 128
max_bytes_per_session = 2048

[providers.claude]
auth_mode = "bound_secret"
[providers.claude.models]
default = "global-model"
`)
	writeFile(t, filepath.Join(workspaceRoot, DirName, ConfigName), `
[observability.transcripts]
segment_bytes = 256

	[providers.claude]
	auth_mode = "bound_secret"
	[[providers.claude.credential_slots]]
	name = "api_key"
	target_env = "WORKSPACE_KEY"
	secret_ref = "env:WORKSPACE_KEY"
	kind = "api_key"
	required = true
	`)

	cfg, err := Load(WithWorkspaceRoot(workspaceRoot))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Observability.Enabled != true || cfg.Observability.RetentionDays != 7 ||
		cfg.Observability.MaxGlobalBytes != 1000 {
		t.Fatalf("Load() Observability = %#v", cfg.Observability)
	}
	if cfg.Observability.Transcripts.SegmentBytes != 256 || cfg.Observability.Transcripts.MaxBytesPerSession != 2048 {
		t.Fatalf("Load() Transcripts = %#v", cfg.Observability.Transcripts)
	}

	claude, err := cfg.ResolveProvider("claude")
	if err != nil {
		t.Fatalf("ResolveProvider() error = %v", err)
	}
	if claude.Models.Default != "global-model" {
		t.Fatalf("ResolveProvider() = %#v", claude)
	}
	if slots := claude.EffectiveCredentialSlots(); len(slots) != 1 ||
		slots[0].TargetEnv != "WORKSPACE_KEY" ||
		slots[0].SecretRef != "env:WORKSPACE_KEY" {
		t.Fatalf("ResolveProvider() CredentialSlots = %#v, want WORKSPACE_KEY slot", slots)
	}
}

func TestLoadWithoutWorkspaceRootIgnoresCurrentDirectoryWorkspaceFiles(t *testing.T) {
	// Intentionally not parallel: this test mutates process-global cwd via os.Chdir.
	homeRoot := filepath.Join(t.TempDir(), "home")
	dotenvHome := filepath.Join(t.TempDir(), "dotenv-home")
	cwd := t.TempDir()

	t.Setenv("AGH_HOME", homeRoot)

	homePaths, err := ResolveHomePaths()
	if err != nil {
		t.Fatalf("ResolveHomePaths() error = %v", err)
	}
	if err := EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}
	writeFile(t, homePaths.ConfigFile, `
[http]
port = 3030
`)

	dotenvPaths, err := ResolveHomePathsFrom(dotenvHome)
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	if err := EnsureHomeLayout(dotenvPaths); err != nil {
		t.Fatalf("EnsureHomeLayout(dotenv) error = %v", err)
	}
	writeFile(t, dotenvPaths.ConfigFile, `
[http]
port = 9090
`)

	writeFile(t, filepath.Join(cwd, ".env"), "AGH_HOME="+dotenvHome+"\n")
	writeFile(t, filepath.Join(cwd, DirName, ConfigName), `
[http]
port = 4242
`)

	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(cwd); err != nil {
		t.Fatalf("Chdir(%q) error = %v", cwd, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previousWD); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got, want := cfg.HTTP.Port, 3030; got != want {
		t.Fatalf("Load() HTTP.Port = %d, want %d", got, want)
	}
	if got, want := cfg.Daemon.Socket, homePaths.DaemonSocket; got != want {
		t.Fatalf("Load() Daemon.Socket = %q, want %q", got, want)
	}
}

func TestLoadWithWorkspaceRootUsesExplicitRootOnly(t *testing.T) {
	workspaceRoot := t.TempDir()
	otherWorkspace := t.TempDir()
	homeRoot := filepath.Join(t.TempDir(), "home")

	t.Setenv("AGH_HOME", homeRoot)

	homePaths, err := ResolveHomePaths()
	if err != nil {
		t.Fatalf("ResolveHomePaths() error = %v", err)
	}
	if err := EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}
	writeFile(t, homePaths.ConfigFile, `
[http]
host = "localhost"
port = 2123
`)
	writeFile(t, filepath.Join(workspaceRoot, DirName, ConfigName), `
[http]
port = 4242
`)
	writeFile(t, filepath.Join(otherWorkspace, DirName, ConfigName), `
[http]
port = 9999
`)

	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(otherWorkspace); err != nil {
		t.Fatalf("Chdir(%q) error = %v", otherWorkspace, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previousWD); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	})

	cfg, err := Load(WithWorkspaceRoot(workspaceRoot))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got, want := cfg.HTTP.Port, 4242; got != want {
		t.Fatalf("Load() HTTP.Port = %d, want %d", got, want)
	}

	writeFile(t, filepath.Join(workspaceRoot, DirName, ConfigName), `
[marketplace.catalog]
ttl = "30m"
`)
	_, err = Load(WithWorkspaceRoot(workspaceRoot))
	if err == nil || !strings.Contains(err.Error(), "marketplace catalog settings are global-only") {
		t.Fatalf("Load(workspace Marketplace config) error = %v, want global-only rejection", err)
	}
}

func TestLoadRejectsUnknownConfigKeys(t *testing.T) {
	workspaceRoot := t.TempDir()
	homeRoot := filepath.Join(t.TempDir(), "home")
	t.Setenv("AGH_HOME", homeRoot)

	homePaths, err := ResolveHomePaths()
	if err != nil {
		t.Fatalf("ResolveHomePaths() error = %v", err)
	}
	if err := EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}

	writeFile(t, homePaths.ConfigFile, `
[http]
port = 2123
unknown = true
`)

	if _, err := Load(WithWorkspaceRoot(workspaceRoot)); err == nil {
		t.Fatal("Load() error = nil, want non-nil")
	}
}

func TestLoadRejectsTimeoutOnSessionBackedRoles(t *testing.T) {
	t.Parallel()

	for _, role := range []RoleName{
		RoleCoordinator,
		RoleDream,
		RoleCheckpointSummary,
		RoleMemoryExtractor,
		RoleAutoTitle,
	} {
		t.Run("Should reject timeout on "+string(role), func(t *testing.T) {
			t.Parallel()

			contents := []byte("[roles." + string(role) + "]\ntimeout = \"1s\"\n")
			_, err := loadConfigOverlayBytes(contents, "roles.toml")
			if err == nil {
				t.Fatal("loadConfigOverlayBytes() error = nil, want unknown-key rejection")
			}
			path := "roles." + string(role) + ".timeout"
			if !strings.Contains(err.Error(), path) {
				t.Fatalf("loadConfigOverlayBytes() error = %v, want %s", err, path)
			}
		})
	}
}

func TestLoadRejectsRemovedRoleConfigKeys(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		config  string
		wantKey string
	}{
		{
			name:    "Should reject the autonomy coordinator table",
			config:  "[autonomy.coordinator]\nenabled = true\n",
			wantKey: "autonomy.coordinator",
		},
		{
			name:    "Should reject the memory dream agent",
			config:  "[memory.dream]\nagent = \"curator\"\n",
			wantKey: "memory.dream.agent",
		},
		{
			name:    "Should reject the memory dream enabled flag",
			config:  "[memory.dream]\nenabled = false\n",
			wantKey: "memory.dream.enabled",
		},
		{
			name:    "Should reject the memory extractor model",
			config:  "[memory.extractor]\nmodel = \"model-a\"\n",
			wantKey: "memory.extractor.model",
		},
		{
			name:    "Should reject the memory extractor enabled flag",
			config:  "[memory.extractor]\nenabled = false\n",
			wantKey: "memory.extractor.enabled",
		},
		{
			name:    "Should reject the memory controller LLM table",
			config:  "[memory.controller.llm]\nenabled = true\n",
			wantKey: "memory.controller.llm",
		},
		{
			name:    "Should reject the session auto title flag",
			config:  "[session]\nauto_title_enabled = false\n",
			wantKey: "session.auto_title_enabled",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := loadConfigOverlayBytes([]byte(tc.config), "removed-role-key.toml")
			if err == nil || !strings.Contains(err.Error(), tc.wantKey) {
				t.Fatalf("loadConfigOverlayBytes() error = %v, want removed key %s", err, tc.wantKey)
			}
		})
	}
}

func TestLoadRejectsUnknownSkillsConfigKeys(t *testing.T) {
	workspaceRoot := t.TempDir()
	homeRoot := filepath.Join(t.TempDir(), "home")
	t.Setenv("AGH_HOME", homeRoot)

	homePaths, err := ResolveHomePaths()
	if err != nil {
		t.Fatalf("ResolveHomePaths() error = %v", err)
	}
	if err := EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}

	writeFile(t, homePaths.ConfigFile, `
[skills]
poll_interval = "3s"
unknown = true
`)

	_, err = Load(WithWorkspaceRoot(workspaceRoot))
	if err == nil {
		t.Fatal("Load() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "skills.unknown") {
		t.Fatalf("Load() error = %v, want skills.unknown in message", err)
	}
}

func TestDefaultWithHomeLeavesMarketplaceConfigEmpty(t *testing.T) {
	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}

	cfg := DefaultWithHome(homePaths)
	if cfg.Skills.AllowedMarketplaceMCP != nil {
		t.Fatalf(
			"DefaultWithHome() Skills.AllowedMarketplaceMCP = %#v, want nil/empty",
			cfg.Skills.AllowedMarketplaceMCP,
		)
	}
	if cfg.Skills.AllowedMarketplaceHooks != nil {
		t.Fatalf(
			"DefaultWithHome() Skills.AllowedMarketplaceHooks = %#v, want nil/empty",
			cfg.Skills.AllowedMarketplaceHooks,
		)
	}
	if cfg.Skills.Marketplace != (MarketplaceConfig{}) {
		t.Fatalf("DefaultWithHome() Skills.Marketplace = %#v, want zero value", cfg.Skills.Marketplace)
	}
	if cfg.Extensions.Marketplace != (ExtensionsMarketplaceConfig{}) {
		t.Fatalf("DefaultWithHome() Extensions.Marketplace = %#v, want zero value", cfg.Extensions.Marketplace)
	}
	if cfg.Extensions.Marketplace.AllowUnverified {
		t.Fatal("DefaultWithHome() Extensions.Marketplace.AllowUnverified = true, want false")
	}
	if len(cfg.Extensions.Resources.AllowedKinds) != 0 ||
		cfg.Extensions.Resources.MaxScope != "" ||
		cfg.Extensions.Resources.SnapshotRateLimit != (ExtensionsResourceRateLimitConfig{}) ||
		cfg.Extensions.Resources.OperatorWriteRateLimit != (ExtensionsResourceRateLimitConfig{}) {
		t.Fatalf("DefaultWithHome() Extensions.Resources = %#v, want zero value", cfg.Extensions.Resources)
	}
}

func TestSkillsConfigValidateMarketplaceConfig(t *testing.T) {
	t.Parallel()

	base := SkillsConfig{
		Enabled:      true,
		PollInterval: time.Second,
	}

	t.Run("ShouldAcceptValidMarketplaceConfig", func(t *testing.T) {
		cfg := base
		cfg.Marketplace = MarketplaceConfig{
			Registry: "clawhub",
			BaseURL:  "https://registry.example.test/api/v1",
		}

		if err := cfg.Validate(); err != nil {
			t.Fatalf("SkillsConfig.Validate() error = %v", err)
		}
	})

	t.Run("ShouldRejectEmptyRegistryWhenMarketplaceConfigured", func(t *testing.T) {
		cfg := base
		cfg.Marketplace = MarketplaceConfig{
			BaseURL: "https://registry.example.test/api/v1",
		}

		err := cfg.Validate()
		if err == nil {
			t.Fatal("SkillsConfig.Validate() error = nil, want registry validation failure")
		}
		if !strings.Contains(err.Error(), "skills.marketplace.registry") {
			t.Fatalf("SkillsConfig.Validate() error = %v, want marketplace registry context", err)
		}
	})

	t.Run("ShouldRejectInvalidMarketplaceBaseURL", func(t *testing.T) {
		cfg := base
		cfg.Marketplace = MarketplaceConfig{
			Registry: "clawhub",
			BaseURL:  "ftp://registry.example.test/api/v1",
		}

		err := cfg.Validate()
		if err == nil {
			t.Fatal("SkillsConfig.Validate() error = nil, want marketplace base_url validation failure")
		}
		if !strings.Contains(err.Error(), "skills.marketplace.base_url") {
			t.Fatalf("SkillsConfig.Validate() error = %v, want marketplace base_url context", err)
		}
	})
}

func TestExtensionsConfigValidateMarketplaceConfig(t *testing.T) {
	tests := []struct {
		name        string
		cfg         ExtensionsConfig
		wantErrPath string
	}{
		{
			name: "ShouldAcceptValidMarketplaceConfig",
			cfg: ExtensionsConfig{
				Marketplace: ExtensionsMarketplaceConfig{
					Registry: "github",
					BaseURL:  "https://api.github.example.test",
				},
			},
		},
		{
			name: "ShouldAcceptEmptyMarketplaceConfig",
			cfg:  ExtensionsConfig{},
		},
		{
			name: "ShouldRejectMarketplaceBaseURLWithoutHost",
			cfg: ExtensionsConfig{
				Marketplace: ExtensionsMarketplaceConfig{
					Registry: "github",
					BaseURL:  "https://",
				},
			},
			wantErrPath: "extensions.marketplace.base_url",
		},
		{
			name: "ShouldRejectUnknownMarketplaceRegistry",
			cfg: ExtensionsConfig{
				Marketplace: ExtensionsMarketplaceConfig{
					Registry: "unknown",
					BaseURL:  "https://api.github.example.test",
				},
			},
			wantErrPath: "extensions.marketplace.registry",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErrPath == "" {
				if err != nil {
					t.Fatalf("ExtensionsConfig.Validate() error = %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("ExtensionsConfig.Validate() error = nil, want marketplace validation failure")
			}
			if !strings.Contains(err.Error(), tt.wantErrPath) {
				t.Fatalf("ExtensionsConfig.Validate() error = %v, want %s context", err, tt.wantErrPath)
			}
		})
	}

	t.Run("ShouldWarnForHTTPBaseURL", func(t *testing.T) {
		// This subtest swaps slog.Default(), so the parent test must remain
		// non-parallel to avoid cross-test interference.
		var logs bytes.Buffer
		original := slog.Default()
		slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelWarn})))
		defer slog.SetDefault(original)

		cfg := ExtensionsConfig{
			Marketplace: ExtensionsMarketplaceConfig{
				Registry: "github",
				BaseURL:  "http://api.github.example.test",
			},
		}

		if err := cfg.Validate(); err != nil {
			t.Fatalf("ExtensionsConfig.Validate(http) error = %v", err)
		}
		if !strings.Contains(logs.String(), "insecure http scheme") {
			t.Fatalf("ExtensionsConfig.Validate(http) logs = %q, want insecure http scheme warning", logs.String())
		}
	})
}

func TestExtensionsConfigValidateResourcesConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		cfg         ExtensionsConfig
		wantErrPath string
	}{
		{
			name: "ShouldAcceptValidResourcePolicy",
			cfg: ExtensionsConfig{
				Resources: ExtensionsResourcesConfig{
					AllowedKinds: []resources.ResourceKind{
						resources.ResourceKind("tool"),
						resources.ResourceKind("mcp_server"),
					},
					MaxScope: resources.ResourceScopeKindWorkspace,
					SnapshotRateLimit: ExtensionsResourceRateLimitConfig{
						Requests: 2,
						Window:   5 * time.Second,
						Queue:    1,
					},
					OperatorWriteRateLimit: ExtensionsResourceRateLimitConfig{
						Requests: 10,
						Window:   time.Minute,
						Queue:    0,
					},
				},
			},
		},
		{
			name: "ShouldRejectDaemonOnlyAllowedKind",
			cfg: ExtensionsConfig{
				Resources: ExtensionsResourcesConfig{
					AllowedKinds: []resources.ResourceKind{resources.ResourceKind("bridge.instance")},
				},
			},
			wantErrPath: "extensions.resources.allowed_kinds",
		},
		{
			name: "ShouldRejectInvalidResourceMaxScope",
			cfg: ExtensionsConfig{
				Resources: ExtensionsResourcesConfig{
					MaxScope: resources.ResourceScopeKind("session"),
				},
			},
			wantErrPath: "extensions.resources.max_scope",
		},
		{
			name: "ShouldRejectInvalidSnapshotRateLimit",
			cfg: ExtensionsConfig{
				Resources: ExtensionsResourcesConfig{
					SnapshotRateLimit: ExtensionsResourceRateLimitConfig{
						Requests: 0,
						Window:   time.Second,
					},
				},
			},
			wantErrPath: "extensions.resources.snapshot_rate_limit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErrPath == "" {
				if err != nil {
					t.Fatalf("ExtensionsConfig.Validate() error = %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("ExtensionsConfig.Validate() error = nil, want validation failure")
			}
			if !strings.Contains(err.Error(), tt.wantErrPath) {
				t.Fatalf("ExtensionsConfig.Validate() error = %v, want %s context", err, tt.wantErrPath)
			}
		})
	}
}

func TestLoadRoundTripsExtensionsResourcePolicy(t *testing.T) {
	workspaceRoot := t.TempDir()
	homeRoot := filepath.Join(t.TempDir(), "home")
	t.Setenv("AGH_HOME", homeRoot)

	homePaths, err := ResolveHomePaths()
	if err != nil {
		t.Fatalf("ResolveHomePaths() error = %v", err)
	}
	if err := EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}

	writeFile(t, homePaths.ConfigFile, `
[extensions.resources]
allowed_kinds = ["tool", "mcp_server"]
max_scope = "global"

[extensions.resources.snapshot_rate_limit]
requests = 3
window = "15s"
queue = 1

[extensions.resources.operator_write_rate_limit]
requests = 12
window = "1m"
queue = 0
`)

	writeFile(t, filepath.Join(workspaceRoot, DirName, ConfigName), `
[extensions.resources]
allowed_kinds = ["tool"]
max_scope = "workspace"

[extensions.resources.snapshot_rate_limit]
requests = 1
window = "5s"
queue = 1
`)

	cfg, err := Load(WithWorkspaceRoot(workspaceRoot))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got, want := cfg.Extensions.Resources.AllowedKinds, []resources.ResourceKind{
		resources.ResourceKind("tool"),
	}; !slices.Equal(
		got,
		want,
	) {
		t.Fatalf("Load() AllowedKinds = %#v, want %#v", got, want)
	}
	if got, want := cfg.Extensions.Resources.MaxScope, resources.ResourceScopeKindWorkspace; got != want {
		t.Fatalf("Load() MaxScope = %q, want %q", got, want)
	}
	if got, want := cfg.Extensions.Resources.SnapshotRateLimit.Requests, 1; got != want {
		t.Fatalf("Load() SnapshotRateLimit.Requests = %d, want %d", got, want)
	}
	if got, want := cfg.Extensions.Resources.SnapshotRateLimit.Window, 5*time.Second; got != want {
		t.Fatalf("Load() SnapshotRateLimit.Window = %s, want %s", got, want)
	}
	if got, want := cfg.Extensions.Resources.OperatorWriteRateLimit.Requests, 12; got != want {
		t.Fatalf("Load() OperatorWriteRateLimit.Requests = %d, want %d", got, want)
	}
	if got, want := cfg.Extensions.Resources.OperatorWriteRateLimit.Window, time.Minute; got != want {
		t.Fatalf("Load() OperatorWriteRateLimit.Window = %s, want %s", got, want)
	}
}

func TestValidateRejectsInvalidPorts(t *testing.T) {
	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}

	tests := []struct {
		name string
		port int
	}{
		{name: "zero", port: 0},
		{name: "too high", port: 65536},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultWithHome(homePaths)
			cfg.HTTP.Port = tt.port

			if err := cfg.Validate(); err == nil {
				t.Fatalf("Validate() error = nil for port %d", tt.port)
			}
		})
	}
}

func TestValidateRejectsUnknownPermissionMode(t *testing.T) {
	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}

	cfg := DefaultWithHome(homePaths)
	cfg.Permissions.Mode = PermissionMode("maybe")
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want non-nil")
	}
}

func TestValidateWrapsHooksConfigErrors(t *testing.T) {
	t.Parallel()

	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}

	cfg := DefaultWithHome(homePaths)
	cfg.Hooks.Declarations = []hookspkg.HookDecl{{
		Name:   "broken-hook",
		Event:  "bad.event",
		Source: hookspkg.HookSourceConfig,
	}}

	err = cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "validate hooks config") {
		t.Fatalf("Validate() error = %q, want hooks config context", err)
	}
}

func TestDreamConfigValidateRejectsNonPositiveThresholds(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		patch func(*DreamConfig)
	}{
		{
			name: "min hours",
			patch: func(cfg *DreamConfig) {
				cfg.MinHours = 0
			},
		},
		{
			name: "min sessions",
			patch: func(cfg *DreamConfig) {
				cfg.MinSessions = 0
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := DreamConfig{
				MinHours:      24,
				MinSessions:   3,
				CheckInterval: 30 * time.Minute,
			}
			tc.patch(&cfg)

			if err := cfg.Validate(); err == nil {
				t.Fatalf("Validate() error = nil for %s", tc.name)
			}
		})
	}
}

func TestLoadRejectsNonPositiveSkillsPollInterval(t *testing.T) {
	workspaceRoot := t.TempDir()
	homeRoot := filepath.Join(t.TempDir(), "home")
	t.Setenv("AGH_HOME", homeRoot)

	homePaths, err := ResolveHomePaths()
	if err != nil {
		t.Fatalf("ResolveHomePaths() error = %v", err)
	}
	if err := EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}

	writeFile(t, homePaths.ConfigFile, `
[skills]
poll_interval = "0s"
`)

	_, err = Load(WithWorkspaceRoot(workspaceRoot))
	if err == nil {
		t.Fatal("Load() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "skills.poll_interval") {
		t.Fatalf("Load() error = %v, want skills.poll_interval in message", err)
	}
}

func TestLoadUsesDotEnvForAGHHome(t *testing.T) {
	workspaceRoot := t.TempDir()
	homeRoot := filepath.Join(t.TempDir(), "dotenv-home")

	writeFile(t, filepath.Join(workspaceRoot, ".env"), "AGH_HOME="+homeRoot+"\n")

	homePaths, err := ResolveHomePathsFrom(homeRoot)
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	if err := EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}
	writeFile(t, homePaths.ConfigFile, `
[defaults]
agent = "dotenv-agent"
`)

	cfg, err := Load(WithWorkspaceRoot(workspaceRoot))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Defaults.Agent != "dotenv-agent" {
		t.Fatalf("Load() Defaults.Agent = %q, want %q", cfg.Defaults.Agent, "dotenv-agent")
	}
}

func TestLoadUsesDotEnvForAGHHomeWithoutMutatingProcessEnv(t *testing.T) {
	workspaceRoot := t.TempDir()
	homeRoot := filepath.Join(t.TempDir(), "dotenv-home")

	unsetEnvForTest(t, "AGH_HOME")
	writeFile(t, filepath.Join(workspaceRoot, ".env"), "AGH_HOME="+homeRoot+"\n")

	homePaths, err := ResolveHomePathsFrom(homeRoot)
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	if err := EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}
	writeFile(t, homePaths.ConfigFile, `
[defaults]
agent = "dotenv-agent"
`)

	cfg, err := Load(WithWorkspaceRoot(workspaceRoot))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Defaults.Agent != "dotenv-agent" {
		t.Fatalf("Load() Defaults.Agent = %q, want %q", cfg.Defaults.Agent, "dotenv-agent")
	}
	if _, ok := os.LookupEnv("AGH_HOME"); ok {
		t.Fatal("Load() mutated process AGH_HOME, want workspace dotenv scoped to the current load only")
	}
}

func TestLoadForHomeKeepsWebhookSecretRefsUnresolvedAcrossWorkspaceLoads(t *testing.T) {
	const secretEnv = "AGH_CONFIG_TASK09_WEBHOOK_SECRET"

	unsetEnvForTest(t, secretEnv)

	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	if err := EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}

	writeFile(t, homePaths.ConfigFile, `
[automation]
timezone = "UTC"
max_concurrent_jobs = 1
default_fire_limit = { max = 1, window = "5m" }

[[automation.triggers]]
scope = "global"
name = "deploy"
event = "webhook"
endpoint_slug = "deploy-review"
agent = "summarizer"
prompt = "Review {{ index .Data \"payload\" }}"
	webhook_secret_ref = "env:`+secretEnv+`"
	`)

	workspaceWithEnv := t.TempDir()
	writeFile(t, filepath.Join(workspaceWithEnv, ".env"), secretEnv+"=workspace-only-secret\n")

	cfg, err := LoadForHome(homePaths, WithWorkspaceRoot(workspaceWithEnv))
	if err != nil {
		t.Fatalf("LoadForHome(workspaceWithEnv) error = %v", err)
	}
	if got, want := cfg.Automation.Triggers[0].WebhookSecretRef, "env:"+secretEnv; got != want {
		t.Fatalf("LoadForHome(workspaceWithEnv) WebhookSecretRef = %q, want %q", got, want)
	}

	workspaceWithoutEnv := t.TempDir()
	cfg, err = LoadForHome(homePaths, WithWorkspaceRoot(workspaceWithoutEnv))
	if err != nil {
		t.Fatalf("LoadForHome(workspaceWithoutEnv) error = %v", err)
	}
	if got, want := cfg.Automation.Triggers[0].WebhookSecretRef, "env:"+secretEnv; got != want {
		t.Fatalf("LoadForHome(workspaceWithoutEnv) WebhookSecretRef = %q, want %q", got, want)
	}
}

func TestLoadWithoutDotEnvOptionIgnoresDotEnv(t *testing.T) {
	workspaceRoot := t.TempDir()
	envHome := filepath.Join(t.TempDir(), "dotenv-home")
	overrideHome := filepath.Join(t.TempDir(), "override-home")

	t.Setenv("AGH_HOME", overrideHome)
	writeFile(t, filepath.Join(workspaceRoot, ".env"), "AGH_HOME="+envHome+"\n")

	overridePaths, err := ResolveHomePathsFrom(overrideHome)
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}

	cfg, err := Load(WithWorkspaceRoot(workspaceRoot), withoutDotEnv())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Daemon.Socket != overridePaths.DaemonSocket {
		t.Fatalf("Load() Daemon.Socket = %q, want %q", cfg.Daemon.Socket, overridePaths.DaemonSocket)
	}
}

func TestLoadWithoutValidationReturnsMergedConfig(t *testing.T) {
	workspaceRoot := t.TempDir()
	homeRoot := filepath.Join(t.TempDir(), "home")
	t.Setenv("AGH_HOME", homeRoot)

	homePaths, err := ResolveHomePaths()
	if err != nil {
		t.Fatalf("ResolveHomePaths() error = %v", err)
	}
	if err := EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}

	writeFile(t, homePaths.ConfigFile, `
[http]
host = "localhost"
port = 0
`)

	cfg, err := Load(WithWorkspaceRoot(workspaceRoot), withoutValidation())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.HTTP.Port != 0 {
		t.Fatalf("Load() HTTP.Port = %d, want 0", cfg.HTTP.Port)
	}
}

func TestLoadMissingConfigReturnsDefaults(t *testing.T) {
	workspaceRoot := t.TempDir()
	homeRoot := filepath.Join(t.TempDir(), "home")
	t.Setenv("AGH_HOME", homeRoot)

	homePaths, err := ResolveHomePaths()
	if err != nil {
		t.Fatalf("ResolveHomePaths() error = %v", err)
	}
	want := DefaultWithHome(homePaths)

	cfg, err := Load(WithWorkspaceRoot(workspaceRoot))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.HTTP != want.HTTP || cfg.Defaults != want.Defaults || cfg.Limits != want.Limits ||
		cfg.Permissions != want.Permissions {
		t.Fatalf("Load() = %#v, want defaults %#v", cfg, want)
	}
	if cfg.Daemon.Socket != want.Daemon.Socket {
		t.Fatalf("Load() Daemon.Socket = %q, want %q", cfg.Daemon.Socket, want.Daemon.Socket)
	}
	if !reflect.DeepEqual(cfg.Memory, want.Memory) {
		t.Fatalf("Load() Memory = %#v, want %#v", cfg.Memory, want.Memory)
	}
	if cfg.Skills.Enabled != want.Skills.Enabled || cfg.Skills.PollInterval != want.Skills.PollInterval ||
		!slices.Equal(cfg.Skills.DisabledSkills, want.Skills.DisabledSkills) {
		t.Fatalf("Load() Skills = %#v, want %#v", cfg.Skills, want.Skills)
	}
	if cfg.Network != want.Network {
		t.Fatalf("Load() Network = %#v, want %#v", cfg.Network, want.Network)
	}
	if !cfg.Network.Enabled {
		t.Fatal("Load() Network.Enabled = false, want true by default")
	}
	if !cfg.Roles.AutoTitle.Enabled {
		t.Fatal("Load() Roles.AutoTitle.Enabled = false, want true by default")
	}
}

func TestDefaultConfigUsesResolvedHomePaths(t *testing.T) {
	t.Setenv("AGH_HOME", "")

	cfg, err := defaultConfig()
	if err != nil {
		t.Fatalf("defaultConfig() error = %v", err)
	}
	if cfg.HTTP.Port != 2123 || cfg.Defaults.Agent != DefaultAgentName {
		t.Fatalf("defaultConfig() = %#v", cfg)
	}
	if cfg.Permissions.Mode != PermissionModeApproveAll {
		t.Fatalf("defaultConfig() Permissions.Mode = %q, want %q", cfg.Permissions.Mode, PermissionModeApproveAll)
	}
	if !cfg.Roles.Dream.Enabled || cfg.Roles.Dream.Agent != "" {
		t.Fatalf("defaultConfig() Roles.Dream = %#v, want enabled builtin routing", cfg.Roles.Dream)
	}
	if !cfg.Skills.Enabled {
		t.Fatal("defaultConfig() Skills.Enabled = false, want true")
	}
	if got, want := cfg.Skills.PollInterval, 3*time.Second; got != want {
		t.Fatalf("defaultConfig() Skills.PollInterval = %s, want %s", got, want)
	}
	if !cfg.Network.Enabled {
		t.Fatal("defaultConfig() Network.Enabled = false, want true")
	}
	if got, want := cfg.Network.Live.Defaults.MaxWakes, 8; got != want {
		t.Fatalf("defaultConfig() Network.Live.Defaults.MaxWakes = %d, want %d", got, want)
	}
	if got, want := cfg.Network.Live.Limits.MaxWakes, 64; got != want {
		t.Fatalf("defaultConfig() Network.Live.Limits.MaxWakes = %d, want %d", got, want)
	}
}

func TestLoadRespectsExplicitNetworkDisable(t *testing.T) {
	workspaceRoot := t.TempDir()
	homeRoot := filepath.Join(t.TempDir(), "home")
	t.Setenv("AGH_HOME", homeRoot)

	homePaths, err := ResolveHomePaths()
	if err != nil {
		t.Fatalf("ResolveHomePaths() error = %v", err)
	}
	if err := EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}

	writeFile(t, homePaths.ConfigFile, `
[network]
enabled = false
`)

	cfg, err := Load(WithWorkspaceRoot(workspaceRoot))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Network.Enabled {
		t.Fatal("Load() Network.Enabled = true, want explicit false override to win")
	}
}

func TestNetworkConfigValidateRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}

	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{
			name: "Should reject invalid live default max wakes",
			mutate: func(cfg *Config) {
				cfg.Network.Live.Defaults.MaxWakes = 0
			},
			wantErr: "network.live.defaults",
		},
		{
			name: "Should reject live default above ceiling",
			mutate: func(cfg *Config) {
				cfg.Network.Live.Defaults.MaxWakes = cfg.Network.Live.Limits.MaxWakes + 1
			},
			wantErr: networkLiveLimitsMaxWakesPath,
		},
		{
			name: "Should reject invalid coalesce range",
			mutate: func(cfg *Config) {
				cfg.Network.Live.Limits.MinCoalesceWindow = "6s"
			},
			wantErr: networkLiveLimitsMinCoalesceWindowPath,
		},
		{
			name: "Should reject invalid replay age",
			mutate: func(cfg *Config) {
				cfg.Network.MaxReplayAge = 0
			},
			wantErr: "network.max_replay_age",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := DefaultWithHome(homePaths)
			tc.mutate(&cfg)

			err := cfg.Validate()
			if err == nil {
				t.Fatalf("Validate() error = nil for %s", tc.name)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Validate() error = %q, want substring %q", err, tc.wantErr)
			}
		})
	}
}

func TestLoadGlobalConfigRejectsRemovedNetworkKeys(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		key   string
		value string
	}{
		{name: "Should reject default_channel", key: "default_channel", value: `"builders"`},
		{name: "Should reject port", key: "port", value: "4222"},
		{name: "Should reject max_payload", key: "max_payload", value: "65536"},
		{name: "Should reject activation_top_k", key: "activation_top_k", value: "3"},
		{name: "Should reject digest_flush_interval", key: "digest_flush_interval", value: `"250ms"`},
		{name: "Should reject digest_max_envelopes", key: "digest_max_envelopes", value: "10"},
		{name: "Should reject response_guidance_max_bytes", key: "response_guidance_max_bytes", value: "512"},
		{
			name:  "Should reject delivery_structured_body_max_bytes",
			key:   "delivery_structured_body_max_bytes",
			value: "4096",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
			if err != nil {
				t.Fatalf("ResolveHomePathsFrom() error = %v", err)
			}
			writeFile(t, homePaths.ConfigFile, "[network]\n"+tc.key+" = "+tc.value+"\n")

			_, err = LoadGlobalConfig(homePaths)
			if err == nil {
				t.Fatalf("LoadGlobalConfig() error = nil, want removed key %q rejected", tc.key)
			}
			if !strings.Contains(err.Error(), "network."+tc.key) {
				t.Fatalf("LoadGlobalConfig() error = %q, want path network.%s", err, tc.key)
			}
		})
	}
}

func writeFile(t *testing.T, path string, contents string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
	if err := os.WriteFile(path, []byte(strings.TrimLeft(contents, "\n")), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func unsetEnvForTest(t *testing.T, key string) {
	t.Helper()

	value, hadValue := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("Unsetenv(%q) error = %v", key, err)
	}

	t.Cleanup(func() {
		var err error
		if hadValue {
			err = os.Setenv(key, value)
		} else {
			err = os.Unsetenv(key)
		}
		if err != nil {
			t.Fatalf("restore env %q error = %v", key, err)
		}
	})
}
