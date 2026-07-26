package cli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/testutil"
)

func TestProviderAuthStatusCommand(t *testing.T) {
	t.Parallel()

	t.Run("Should report native CLI auth without requiring credentials", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, nil)
		stdout, _, err := executeRootCommand(t, deps, "provider", "auth", "status", "claude", "-o", "json")
		if err != nil {
			t.Fatalf("provider auth status error = %v", err)
		}

		var record providerAuthStatusRecord
		if err := json.Unmarshal([]byte(stdout), &record); err != nil {
			t.Fatalf("json.Unmarshal(provider auth status) error = %v", err)
		}
		if got, want := record.Provider, "claude"; got != want {
			t.Fatalf("Provider = %q, want %q", got, want)
		}
		if got, want := record.AuthMode, "native_cli"; got != want {
			t.Fatalf("AuthMode = %q, want %q", got, want)
		}
		if len(record.Credentials) != 0 {
			t.Fatalf("Credentials = %#v, want none for native CLI provider", record.Credentials)
		}
		if record.NativeCLI == nil || record.NativeCLI.Command != "claude" {
			t.Fatalf("NativeCLI = %#v, want claude login command presence", record.NativeCLI)
		}
	})

	t.Run("Should report native CLI lookup errors without failing status", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, nil)
		deps.loadConfig = func() (aghconfig.Config, error) {
			cfg := aghconfig.DefaultWithHome(mustTestHomePaths(t))
			cfg.Providers["local"] = aghconfig.ProviderConfig{
				Command:  "local-agent acp",
				AuthMode: aghconfig.ProviderAuthModeNativeCLI,
			}
			return cfg, nil
		}
		deps.lookPath = func(name string) (string, error) {
			if name != "local-agent" {
				t.Fatalf("lookPath(%q), want local-agent", name)
			}
			return "", errors.New("permission denied scanning PATH")
		}

		stdout, _, err := executeRootCommand(t, deps, "provider", "auth", "status", "local", "-o", "json")
		if err != nil {
			t.Fatalf("provider auth status error = %v", err)
		}

		var record providerAuthStatusRecord
		if err := json.Unmarshal([]byte(stdout), &record); err != nil {
			t.Fatalf("json.Unmarshal(provider auth status) error = %v", err)
		}
		if got, want := record.State, "missing_cli"; got != want {
			t.Fatalf("State = %q, want %q", got, want)
		}
		if record.NativeCLI == nil || record.NativeCLI.Error == "" {
			t.Fatalf("NativeCLI = %#v, want lookup error", record.NativeCLI)
		}
	})

	t.Run("Should report missing required bound secret credentials", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, nil)
		deps.loadConfig = func() (aghconfig.Config, error) {
			cfg := aghconfig.DefaultWithHome(mustTestHomePaths(t))
			cfg.Providers["custom"] = aghconfig.ProviderConfig{
				Command:  "custom-agent --acp",
				AuthMode: aghconfig.ProviderAuthModeBoundSecret,
				CredentialSlots: []aghconfig.ProviderCredentialSlot{
					{
						Name:      "api_key",
						TargetEnv: "CUSTOM_API_KEY",
						SecretRef: "env:CUSTOM_API_KEY",
						Kind:      "api_key",
						Required:  true,
					},
				},
			}
			return cfg, nil
		}

		stdout, _, err := executeRootCommand(t, deps, "provider", "auth", "status", "custom", "-o", "json")
		if err != nil {
			t.Fatalf("provider auth status error = %v", err)
		}

		var record providerAuthStatusRecord
		if err := json.Unmarshal([]byte(stdout), &record); err != nil {
			t.Fatalf("json.Unmarshal(provider auth status) error = %v", err)
		}
		if got, want := record.State, "missing_credential"; got != want {
			t.Fatalf("State = %q, want %q", got, want)
		}
		if len(record.Credentials) != 1 || record.Credentials[0].Present {
			t.Fatalf("Credentials = %#v, want one missing credential", record.Credentials)
		}
	})

	t.Run("Should run configured native status command", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, nil)
		deps.loadConfig = func() (aghconfig.Config, error) {
			cfg := aghconfig.DefaultWithHome(mustTestHomePaths(t))
			cfg.Providers["claude"] = aghconfig.ProviderConfig{
				AuthStatusCmd: "claude auth status",
			}
			return cfg, nil
		}
		deps.lookPath = func(name string) (string, error) {
			if name != "claude" {
				t.Fatalf("lookPath(%q), want claude", name)
			}
			return "/usr/local/bin/claude", nil
		}
		deps.runProviderAuthCommand = func(
			_ context.Context,
			spec providerAuthCommandSpec,
		) (providerAuthCommandResult, error) {
			if !strings.Contains(spec.Command, "claude auth status") {
				t.Fatalf("Command = %q, want claude status command", spec.Command)
			}
			if providerTestEnvValue(spec.Env, "AGH_PROVIDER_AUTH_MODE") != "native_cli" {
				t.Fatalf("AGH_PROVIDER_AUTH_MODE missing from provider auth command env: %#v", spec.Env)
			}
			return providerAuthCommandResult{ExitCode: 0, Stdout: "logged in"}, nil
		}

		stdout, _, err := executeRootCommand(t, deps, "provider", "auth", "status", "claude", "-o", "json")
		if err != nil {
			t.Fatalf("provider auth status error = %v", err)
		}

		var record providerAuthStatusRecord
		if err := json.Unmarshal([]byte(stdout), &record); err != nil {
			t.Fatalf("json.Unmarshal(provider auth status) error = %v", err)
		}
		if got, want := record.State, "authenticated"; got != want {
			t.Fatalf("State = %q, want %q", got, want)
		}
		if record.Probe == nil || record.Probe.Stdout != "logged in" {
			t.Fatalf("Probe = %#v, want status command result", record.Probe)
		}
		if record.NativeCLI == nil || !record.NativeCLI.Present || record.NativeCLI.Source != "auth_status_command" {
			t.Fatalf("NativeCLI = %#v, want present auth_status_command", record.NativeCLI)
		}
	})

	t.Run("Should report missing native CLI before probing status command", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, nil)
		deps.loadConfig = func() (aghconfig.Config, error) {
			cfg := aghconfig.DefaultWithHome(mustTestHomePaths(t))
			cfg.Providers["local"] = aghconfig.ProviderConfig{
				Command:       "missing-agent acp",
				AuthMode:      aghconfig.ProviderAuthModeNativeCLI,
				AuthStatusCmd: "missing-agent auth status",
				AuthLoginCmd:  "missing-agent auth login",
			}
			return cfg, nil
		}
		deps.lookPath = func(name string) (string, error) {
			if name != "missing-agent" {
				t.Fatalf("lookPath(%q), want missing-agent", name)
			}
			return "", exec.ErrNotFound
		}
		deps.runProviderAuthCommand = func(
			_ context.Context,
			spec providerAuthCommandSpec,
		) (providerAuthCommandResult, error) {
			t.Fatalf("runProviderAuthCommand(%q) called, want missing CLI to stop before probe", spec.Command)
			return providerAuthCommandResult{}, nil
		}

		stdout, _, err := executeRootCommand(t, deps, "provider", "auth", "status", "local", "-o", "json")
		if err != nil {
			t.Fatalf("provider auth status error = %v", err)
		}

		var record providerAuthStatusRecord
		if err := json.Unmarshal([]byte(stdout), &record); err != nil {
			t.Fatalf("json.Unmarshal(provider auth status) error = %v", err)
		}
		if got, want := record.State, "missing_cli"; got != want {
			t.Fatalf("State = %q, want %q", got, want)
		}
		if record.NativeCLI == nil || record.NativeCLI.Present || record.NativeCLI.Command != "missing-agent" {
			t.Fatalf("NativeCLI = %#v, want missing missing-agent", record.NativeCLI)
		}
		if record.Probe != nil {
			t.Fatalf("Probe = %#v, want no probe when native CLI is missing", record.Probe)
		}
		if !strings.Contains(record.Message, "missing-agent auth login") {
			t.Fatalf("Message = %q, want login command guidance", record.Message)
		}
	})

	t.Run("Should include login command when native status probe needs login", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, nil)
		deps.loadConfig = func() (aghconfig.Config, error) {
			cfg := aghconfig.DefaultWithHome(mustTestHomePaths(t))
			cfg.Providers["local"] = aghconfig.ProviderConfig{
				Command:       "claude acp",
				AuthMode:      aghconfig.ProviderAuthModeNativeCLI,
				AuthStatusCmd: "claude auth status",
				AuthLoginCmd:  "claude auth login",
			}
			return cfg, nil
		}
		deps.lookPath = func(name string) (string, error) {
			if name != "claude" {
				t.Fatalf("lookPath(%q), want claude", name)
			}
			return "/usr/local/bin/claude", nil
		}
		deps.runProviderAuthCommand = func(
			_ context.Context,
			spec providerAuthCommandSpec,
		) (providerAuthCommandResult, error) {
			if spec.Command != "claude auth status" {
				t.Fatalf("Command = %q, want claude auth status", spec.Command)
			}
			return providerAuthCommandResult{ExitCode: 1, Stderr: "not logged in"}, nil
		}

		stdout, _, err := executeRootCommand(t, deps, "provider", "auth", "status", "local", "-o", "json")
		if err != nil {
			t.Fatalf("provider auth status error = %v", err)
		}

		var record providerAuthStatusRecord
		if err := json.Unmarshal([]byte(stdout), &record); err != nil {
			t.Fatalf("json.Unmarshal(provider auth status) error = %v", err)
		}
		if got, want := record.State, "needs_login"; got != want {
			t.Fatalf("State = %q, want %q", got, want)
		}
		if !strings.Contains(record.Message, "claude auth login") {
			t.Fatalf("Message = %q, want login command guidance", record.Message)
		}
		if record.NativeCLI == nil || !record.NativeCLI.Present || record.NativeCLI.Path != "/usr/local/bin/claude" {
			t.Fatalf("NativeCLI = %#v, want present claude path", record.NativeCLI)
		}
	})
}

func TestProviderDaemonCommands(t *testing.T) {
	t.Parallel()

	t.Run("Should list providers through daemon client", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{
			listProvidersFn: func(_ context.Context) (contract.ProviderListResponse, error) {
				return contract.ProviderListResponse{
					Providers: []contract.ProviderSummaryPayload{
						{
							Name:        "public",
							DisplayName: "Public",
							Default:     true,
							AuthStatus: contract.ProviderAuthStatusPayload{
								Mode:    "none",
								State:   contract.ProviderAuthStateNone,
								Message: "No auth required.",
							},
						},
					},
				}, nil
			},
		}
		deps := newTestDeps(t, client)

		stdout, _, err := executeRootCommand(t, deps, "provider", "list", "-o", "json")
		if err != nil {
			t.Fatalf("provider list error = %v", err)
		}

		var record contract.ProviderListResponse
		if err := json.Unmarshal([]byte(stdout), &record); err != nil {
			t.Fatalf("json.Unmarshal(provider list) error = %v", err)
		}
		if got, want := record.Providers[0].AuthStatus.State, contract.ProviderAuthStateNone; got != want {
			t.Fatalf("AuthStatus.State = %q, want %q", got, want)
		}
	})

	t.Run("Should run remote auth status through daemon client", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{
			probeProviderAuthFn: func(_ context.Context, providerID string) (contract.ProviderAuthProbeResponse, error) {
				if providerID != "codex" {
					t.Fatalf("providerID = %q, want codex", providerID)
				}
				return contract.ProviderAuthProbeResponse{
					Provider: "codex",
					AuthStatus: contract.ProviderAuthStatusPayload{
						Mode:  "native_cli",
						State: contract.ProviderAuthStateNeedsLogin,
						Code:  contract.CodeProviderNotAuthenticated,
					},
					Probe: &contract.ProviderAuthProbeResult{ExitCode: 1, Stderr: "not logged in"},
				}, nil
			},
		}
		deps := newTestDeps(t, client)

		stdout, _, err := executeRootCommand(t, deps, "provider", "auth", "status", "codex", "--remote", "-o", "json")
		if err != nil {
			t.Fatalf("provider auth status --remote error = %v", err)
		}

		var record contract.ProviderAuthProbeResponse
		if err := json.Unmarshal([]byte(stdout), &record); err != nil {
			t.Fatalf("json.Unmarshal(provider auth probe) error = %v", err)
		}
		if got, want := record.AuthStatus.Code, contract.CodeProviderNotAuthenticated; got != want {
			t.Fatalf("AuthStatus.Code = %q, want %q", got, want)
		}
	})
}

// This test mutates process environment and must stay outside the parallel
// provider auth status suite.
func TestProviderAuthStatusCommandHermeticEnv(t *testing.T) {
	t.Run("Should hide operator credentials from provider auth checks", func(t *testing.T) {
		setProviderTestEnv(t, "CUSTOM_API_KEY", "sk-operator")
		hermetic := testutil.ApplyHermeticEnv(t)

		deps := newTestDeps(t, nil)
		deps.getenv = os.Getenv
		deps.loadConfig = func() (aghconfig.Config, error) {
			cfg := aghconfig.DefaultWithHome(mustTestHomePaths(t))
			cfg.Providers["custom"] = aghconfig.ProviderConfig{
				Command:  "custom-agent --acp",
				AuthMode: aghconfig.ProviderAuthModeBoundSecret,
				CredentialSlots: []aghconfig.ProviderCredentialSlot{
					{
						Name:      "api_key",
						TargetEnv: "CUSTOM_API_KEY",
						SecretRef: "env:CUSTOM_API_KEY",
						Kind:      "api_key",
						Required:  true,
					},
				},
			}
			return cfg, nil
		}

		stdout, _, err := executeRootCommand(t, deps, "provider", "auth", "status", "custom", "-o", "json")
		if err != nil {
			t.Fatalf("provider auth status error = %v", err)
		}

		var record providerAuthStatusRecord
		if err := json.Unmarshal([]byte(stdout), &record); err != nil {
			t.Fatalf("json.Unmarshal(provider auth status) error = %v", err)
		}
		if got, want := record.State, "missing_credential"; got != want {
			t.Fatalf("State = %q, want %q", got, want)
		}
		if len(record.Credentials) != 1 || record.Credentials[0].Present {
			t.Fatalf("Credentials = %#v, want hermetic env to hide operator credential", record.Credentials)
		}
		if got, want := os.Getenv("AGH_HOME"), hermetic.HomeDir; got != want {
			t.Fatalf("AGH_HOME = %q, want %q", got, want)
		}
	})
}

func TestProviderAuthLoginCommand(t *testing.T) {
	t.Parallel()

	t.Run("Should run configured native login command locally", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, nil)
		deps.loadConfig = func() (aghconfig.Config, error) {
			cfg := aghconfig.DefaultWithHome(mustTestHomePaths(t))
			cfg.Providers["codex"] = aghconfig.ProviderConfig{
				AuthLoginCmd: "codex login",
			}
			return cfg, nil
		}
		deps.lookPath = func(name string) (string, error) {
			if name != "codex" {
				t.Fatalf("lookPath(%q), want codex", name)
			}
			return "/usr/local/bin/codex", nil
		}
		deps.runProviderAuthLoginCommand = func(
			_ context.Context,
			spec providerAuthCommandSpec,
		) (providerAuthCommandResult, error) {
			if spec.Command != "codex login" {
				t.Fatalf("Login command = %q, want codex login", spec.Command)
			}
			return providerAuthCommandResult{ExitCode: 0}, nil
		}

		stdout, _, err := executeRootCommand(t, deps, "provider", "auth", "login", "codex", "-o", "json")
		if err != nil {
			t.Fatalf("provider auth login error = %v", err)
		}

		var record providerAuthStatusRecord
		if err := json.Unmarshal([]byte(stdout), &record); err != nil {
			t.Fatalf("json.Unmarshal(provider auth login) error = %v", err)
		}
		if got, want := record.State, "unknown"; got != want {
			t.Fatalf("State = %q, want %q", got, want)
		}
		if got, want := record.LoginCommand, "codex login"; got != want {
			t.Fatalf("LoginCommand = %q, want %q", got, want)
		}
		if record.Code != "provider_classification_unknown" {
			t.Fatalf("Code = %q, want provider_classification_unknown", record.Code)
		}
		if record.NativeCLI == nil || !record.NativeCLI.Present || record.NativeCLI.Command != "codex" {
			t.Fatalf("NativeCLI = %#v, want present codex", record.NativeCLI)
		}
	})

	t.Run("Should print only the resolved native login command", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, nil)
		deps.loadConfig = func() (aghconfig.Config, error) {
			cfg := aghconfig.DefaultWithHome(mustTestHomePaths(t))
			cfg.Providers["codex"] = aghconfig.ProviderConfig{
				AuthLoginCmd: "codex login",
			}
			return cfg, nil
		}
		deps.lookPath = func(name string) (string, error) {
			if name != "codex" {
				t.Fatalf("lookPath(%q), want codex", name)
			}
			return "/usr/local/bin/codex", nil
		}
		deps.runProviderAuthCommand = func(
			_ context.Context,
			spec providerAuthCommandSpec,
		) (providerAuthCommandResult, error) {
			t.Fatalf("runProviderAuthCommand(%q) called, want print-only login command", spec.Command)
			return providerAuthCommandResult{}, nil
		}

		stdout, _, err := executeRootCommand(t, deps, "provider", "auth", "login", "codex", "--print-command")
		if err != nil {
			t.Fatalf("provider auth login --print-command error = %v", err)
		}
		if got, want := stdout, "codex login\n"; got != want {
			t.Fatalf("stdout = %q, want %q", got, want)
		}
	})

	t.Run("Should expose builtin Pi login against the isolated Pi auth directory", func(t *testing.T) {
		t.Parallel()

		homePaths := mustTestHomePaths(t)
		deps := newTestDeps(t, nil)
		deps.resolveHome = func() (aghconfig.HomePaths, error) {
			return homePaths, nil
		}
		deps.loadConfig = func() (aghconfig.Config, error) {
			cfg := aghconfig.DefaultWithHome(homePaths)
			cfg.Providers["pi"] = aghconfig.ProviderConfig{
				EnvPolicy:  aghconfig.ProviderEnvPolicyIsolated,
				HomePolicy: aghconfig.ProviderHomePolicyIsolated,
			}
			return cfg, nil
		}
		deps.lookPath = func(name string) (string, error) {
			if name != "npx" {
				t.Fatalf("lookPath(%q), want npx", name)
			}
			return "/usr/local/bin/npx", nil
		}
		deps.runProviderAuthLoginCommand = func(
			_ context.Context,
			spec providerAuthCommandSpec,
		) (providerAuthCommandResult, error) {
			if spec.Command != "npx -y pi-acp@latest --terminal-login" {
				t.Fatalf("Login command = %q, want Pi terminal login", spec.Command)
			}
			if providerTestEnvValue(spec.Env, "PI_CODING_AGENT_DIR") == "" {
				t.Fatalf("Login env = %#v, want PI_CODING_AGENT_DIR", spec.Env)
			}
			return providerAuthCommandResult{ExitCode: 0}, nil
		}
		deps.runProviderAuthCommand = func(
			_ context.Context,
			spec providerAuthCommandSpec,
		) (providerAuthCommandResult, error) {
			t.Fatalf("runProviderAuthCommand(%q) called, want no status command after Pi login", spec.Command)
			return providerAuthCommandResult{}, nil
		}

		stdout, _, err := executeRootCommand(t, deps, "provider", "auth", "login", "pi", "-o", "json")
		if err != nil {
			t.Fatalf("provider auth login error = %v", err)
		}

		var record providerAuthStatusRecord
		if err := json.Unmarshal([]byte(stdout), &record); err != nil {
			t.Fatalf("json.Unmarshal(provider auth login) error = %v", err)
		}
		if got, want := record.AuthMode, "native_cli"; got != want {
			t.Fatalf("AuthMode = %q, want %q", got, want)
		}
		if got, want := record.LoginCommand, "npx -y pi-acp@latest --terminal-login"; got != want {
			t.Fatalf("LoginCommand = %q, want %q", got, want)
		}
		if providerTestEnvValue(record.LoginEnv, "HOME") == "" {
			t.Fatalf("LoginEnv = %#v, want isolated HOME", record.LoginEnv)
		}
		if providerTestEnvValue(record.LoginEnv, "PI_CODING_AGENT_DIR") == "" {
			t.Fatalf("LoginEnv = %#v, want Pi auth directory", record.LoginEnv)
		}
		if got, want := record.State, "unknown"; got != want {
			t.Fatalf("State = %q, want %q", got, want)
		}
		if record.NativeCLI == nil || !record.NativeCLI.Present || record.NativeCLI.Command != "npx" {
			t.Fatalf("NativeCLI = %#v, want present npx", record.NativeCLI)
		}
	})

	t.Run("Should fail before printing when native login CLI is missing", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, nil)
		deps.loadConfig = func() (aghconfig.Config, error) {
			cfg := aghconfig.DefaultWithHome(mustTestHomePaths(t))
			cfg.Providers["local"] = aghconfig.ProviderConfig{
				Command:      "local-agent acp",
				AuthMode:     aghconfig.ProviderAuthModeNativeCLI,
				AuthLoginCmd: "missing-agent auth login",
			}
			return cfg, nil
		}
		deps.lookPath = func(name string) (string, error) {
			if name != "missing-agent" {
				t.Fatalf("lookPath(%q), want missing-agent", name)
			}
			return "", exec.ErrNotFound
		}
		deps.runProviderAuthCommand = func(
			_ context.Context,
			spec providerAuthCommandSpec,
		) (providerAuthCommandResult, error) {
			t.Fatalf("runProviderAuthCommand(%q) called, want missing CLI to stop first", spec.Command)
			return providerAuthCommandResult{}, nil
		}

		_, _, err := executeRootCommand(t, deps, "provider", "auth", "login", "local", "--print-command")
		if err == nil {
			t.Fatal("provider auth login missing CLI error = nil, want missing CLI error")
		}
		if !strings.Contains(err.Error(), `native CLI "missing-agent" was not found on PATH`) {
			t.Fatalf("provider auth login missing CLI error = %v, want missing CLI guidance", err)
		}
	})

	t.Run("Should explain native login boundary when login command is missing", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, nil)
		deps.loadConfig = func() (aghconfig.Config, error) {
			cfg := aghconfig.DefaultWithHome(mustTestHomePaths(t))
			cfg.Providers["local"] = aghconfig.ProviderConfig{
				Command:  "local-agent acp",
				AuthMode: aghconfig.ProviderAuthModeNativeCLI,
			}
			return cfg, nil
		}
		deps.runProviderAuthCommand = func(
			_ context.Context,
			spec providerAuthCommandSpec,
		) (providerAuthCommandResult, error) {
			t.Fatalf("runProviderAuthCommand(%q) called, want missing login command to stop first", spec.Command)
			return providerAuthCommandResult{}, nil
		}

		_, _, err := executeRootCommand(t, deps, "provider", "auth", "login", "local", "-o", "json")
		if err == nil {
			t.Fatal("provider auth login local error = nil, want missing auth_login_command error")
		}
		if !strings.Contains(err.Error(), "set providers.local.auth_login_command") {
			t.Fatalf("provider auth login local error = %v, want config guidance", err)
		}
	})

	t.Run("Should reject builtin wrapped provider login without running Pi terminal auth", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, nil)
		deps.runProviderAuthCommand = func(
			_ context.Context,
			spec providerAuthCommandSpec,
		) (providerAuthCommandResult, error) {
			t.Fatalf("runProviderAuthCommand(%q) called, want no native login for wrapped provider", spec.Command)
			return providerAuthCommandResult{}, nil
		}

		_, _, err := executeRootCommand(t, deps, "provider", "auth", "login", "openrouter", "-o", "json")
		if err == nil {
			t.Fatal("provider auth login openrouter error = nil, want missing auth_login_command error")
		}
		if !strings.Contains(err.Error(), `provider "openrouter" does not define auth_login_command`) {
			t.Fatalf("provider auth login openrouter error = %v, want missing auth_login_command", err)
		}
	})

	t.Run("Should print isolated Pi login command with provider home environment", func(t *testing.T) {
		t.Parallel()

		homePaths := mustTestHomePaths(t)
		deps := newTestDeps(t, nil)
		deps.resolveHome = func() (aghconfig.HomePaths, error) {
			return homePaths, nil
		}
		deps.loadConfig = func() (aghconfig.Config, error) {
			cfg := aghconfig.DefaultWithHome(homePaths)
			cfg.Providers["pi"] = aghconfig.ProviderConfig{
				EnvPolicy:  aghconfig.ProviderEnvPolicyIsolated,
				HomePolicy: aghconfig.ProviderHomePolicyIsolated,
			}
			return cfg, nil
		}
		deps.lookPath = func(name string) (string, error) {
			if name != "npx" {
				t.Fatalf("lookPath(%q), want npx", name)
			}
			return "/usr/local/bin/npx", nil
		}
		deps.runProviderAuthCommand = func(
			_ context.Context,
			spec providerAuthCommandSpec,
		) (providerAuthCommandResult, error) {
			t.Fatalf("runProviderAuthCommand(%q) called, want print-only login command", spec.Command)
			return providerAuthCommandResult{}, nil
		}

		stdout, _, err := executeRootCommand(t, deps, "provider", "auth", "login", "pi", "--print-command")
		if err != nil {
			t.Fatalf("provider auth login pi --print-command error = %v", err)
		}
		if !strings.Contains(stdout, " HOME=") {
			t.Fatalf("stdout = %q, want env HOME prefix", stdout)
		}
		if !strings.Contains(stdout, "PI_CODING_AGENT_DIR=") {
			t.Fatalf("stdout = %q, want Pi auth directory", stdout)
		}
		if !strings.Contains(stdout, "npx -y pi-acp@latest --terminal-login") {
			t.Fatalf("stdout = %q, want Pi terminal login command", stdout)
		}
	})

	t.Run("Should reject print command with explicit output format", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, nil)
		deps.loadConfig = func() (aghconfig.Config, error) {
			cfg := aghconfig.DefaultWithHome(mustTestHomePaths(t))
			cfg.Providers["codex"] = aghconfig.ProviderConfig{
				AuthLoginCmd: "codex login",
			}
			return cfg, nil
		}
		deps.lookPath = func(name string) (string, error) {
			if name != "codex" {
				t.Fatalf("lookPath(%q), want codex", name)
			}
			return "/usr/local/bin/codex", nil
		}
		deps.runProviderAuthCommand = func(
			_ context.Context,
			spec providerAuthCommandSpec,
		) (providerAuthCommandResult, error) {
			t.Fatalf("runProviderAuthCommand(%q) called, want print-only login command", spec.Command)
			return providerAuthCommandResult{}, nil
		}

		_, _, err := executeRootCommand(t, deps, "provider", "auth", "login", "codex", "--print-command", "-o", "json")
		if err == nil {
			t.Fatal("provider auth login --print-command -o json error = nil, want conflict")
		}
		if !strings.Contains(err.Error(), "--print-command emits raw shell text") {
			t.Fatalf("provider auth login output conflict error = %v, want raw shell text guidance", err)
		}
	})
}

func mustTestHomePaths(t *testing.T) aghconfig.HomePaths {
	t.Helper()

	homePaths, err := aghconfig.ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	return homePaths
}

func setProviderTestEnv(t *testing.T, key string, value string) {
	t.Helper()

	original, hadOriginal := os.LookupEnv(key)
	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("Setenv(%q) error = %v", key, err)
	}
	t.Cleanup(func() {
		var err error
		if hadOriginal {
			err = os.Setenv(key, original)
		} else {
			err = os.Unsetenv(key)
		}
		if err != nil {
			t.Fatalf("restore env %q error = %v", key, err)
		}
	})
}

func providerTestEnvValue(env []string, key string) string {
	prefix := key + "="
	for _, entry := range env {
		if value, ok := strings.CutPrefix(entry, prefix); ok {
			return value
		}
	}
	return ""
}
