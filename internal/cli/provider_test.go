package cli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	authproviders "github.com/compozy/compozy/internal/providers"
	"github.com/compozy/compozy/internal/testutil"
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
		if !record.Login.Configured || record.Login.Executable != "claude" {
			t.Fatalf("Login = %#v, want configured claude descriptor", record.Login)
		}
	})

	t.Run("Should report native CLI lookup errors without failing status", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, nil)
		deps.loadConfig = func() (compozyconfig.Config, error) {
			cfg := compozyconfig.DefaultWithHome(mustTestHomePaths(t))
			cfg.Providers["local"] = compozyconfig.ProviderConfig{
				Command:       "local-agent acp",
				AuthMode:      compozyconfig.ProviderAuthModeNativeCLI,
				AuthStatusCmd: "status-helper auth status",
			}
			return cfg, nil
		}
		resolveCalls := 0
		deps.resolveProviderAuthCommand = func(_ context.Context, name string, _ []string, _ string) (string, error) {
			resolveCalls++
			if name != "status-helper" {
				t.Fatalf("resolveProviderAuthCommand(%q), want status-helper", name)
			}
			if resolveCalls == 1 {
				return "", errors.New("permission denied scanning PATH token=cli-resolution-secret")
			}
			return "/later/bin/status-helper", nil
		}
		deps.runProviderAuthCommand = func(
			_ context.Context,
			spec providerAuthCommandSpec,
		) (providerAuthCommandResult, error) {
			t.Fatalf("runProviderAuthCommand(%q) called after resolver failure", spec.Command)
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
		if got, want := record.State, "unknown"; got != want {
			t.Fatalf("State = %q, want %q", got, want)
		}
		if got, want := record.Code, contract.CodeProviderClassificationUnknown; got != want {
			t.Fatalf("Code = %q, want %q", got, want)
		}
		if strings.Contains(record.Message, "cli-resolution-secret") {
			t.Fatalf("message = %q, leaked resolver secret", record.Message)
		}
		if !strings.Contains(record.Message, "[REDACTED]") {
			t.Fatalf("message = %q, want redaction marker", record.Message)
		}
		if got := record.Login.Executable; got != "" {
			t.Fatalf("login executable = %q, want no configured login descriptor", got)
		}
		if got, want := resolveCalls, 1; got != want {
			t.Fatalf("auth status resolver calls = %d, want %d", got, want)
		}
		if strings.Contains(stdout, "cli-resolution-secret") {
			t.Fatalf("stdout leaked resolver secret: %s", stdout)
		}
	})

	t.Run("Should report missing required bound secret credentials", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, nil)
		deps.loadConfig = func() (compozyconfig.Config, error) {
			cfg := compozyconfig.DefaultWithHome(mustTestHomePaths(t))
			cfg.Providers["custom"] = compozyconfig.ProviderConfig{
				Command:  "custom-agent --acp",
				AuthMode: compozyconfig.ProviderAuthModeBoundSecret,
				CredentialSlots: []compozyconfig.ProviderCredentialSlot{
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
		deps.loadConfig = func() (compozyconfig.Config, error) {
			cfg := compozyconfig.DefaultWithHome(mustTestHomePaths(t))
			cfg.Providers["claude"] = compozyconfig.ProviderConfig{
				AuthStatusCmd: "claude auth status",
			}
			return cfg, nil
		}
		deps.resolveProviderAuthCommand = func(_ context.Context, name string, _ []string, _ string) (string, error) {
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
			if providerTestEnvValue(spec.Env, "COMPOZY_PROVIDER_AUTH_MODE") != "native_cli" {
				t.Fatalf("COMPOZY_PROVIDER_AUTH_MODE missing from provider auth command env: %#v", spec.Env)
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
		if !record.Login.Configured || record.Login.Executable != "claude" ||
			record.Login.Presence != authproviders.ProviderLoginPresencePresent {
			t.Fatalf("Login = %#v, want present claude descriptor", record.Login)
		}
	})

	t.Run("Should resolve the native status command once with its final runtime", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, nil)
		deps.loadConfig = func() (compozyconfig.Config, error) {
			cfg := compozyconfig.DefaultWithHome(mustTestHomePaths(t))
			cfg.Providers["native"] = compozyconfig.ProviderConfig{
				Command:       "native-agent acp",
				AuthMode:      compozyconfig.ProviderAuthModeNativeCLI,
				AuthStatusCmd: "status-helper auth status",
			}
			return cfg, nil
		}

		var resolveCalls int
		var resolvedEnv []string
		deps.resolveProviderAuthCommand = func(
			_ context.Context,
			command string,
			env []string,
			dir string,
		) (string, error) {
			resolveCalls++
			if command != "status-helper" {
				t.Fatalf("resolver command = %q, want status-helper", command)
			}
			if dir != "" {
				t.Fatalf("resolver dir = %q, want empty", dir)
			}
			resolvedEnv = append([]string(nil), env...)
			if resolveCalls > 1 {
				return "", errors.New("second auth status resolution must not occur")
			}
			return "/final/bin/status-helper", nil
		}
		deps.runProviderAuthCommand = func(
			_ context.Context,
			spec providerAuthCommandSpec,
		) (providerAuthCommandResult, error) {
			if got, want := spec.Executable, "/final/bin/status-helper"; got != want {
				t.Fatalf("Executable = %q, want %q", got, want)
			}
			if !slices.Equal(spec.Args, []string{"auth", "status"}) {
				t.Fatalf("Args = %#v, want auth status", spec.Args)
			}
			if spec.Dir != "" {
				t.Fatalf("Dir = %q, want empty", spec.Dir)
			}
			if !slices.Equal(spec.Env, resolvedEnv) {
				t.Fatalf("Env = %#v, want resolver env %#v", spec.Env, resolvedEnv)
			}
			return providerAuthCommandResult{ExitCode: 0, Stdout: "logged in"}, nil
		}

		stdout, _, err := executeRootCommand(t, deps, "provider", "auth", "status", "native", "-o", "json")
		if err != nil {
			t.Fatalf("provider auth status error = %v", err)
		}
		if got, want := resolveCalls, 1; got != want {
			t.Fatalf("auth status resolver calls = %d, want %d", got, want)
		}

		var record providerAuthStatusRecord
		if err := json.Unmarshal([]byte(stdout), &record); err != nil {
			t.Fatalf("json.Unmarshal(provider auth status) error = %v", err)
		}
		if got, want := record.State, "authenticated"; got != want {
			t.Fatalf("State = %q, want %q", got, want)
		}
		if record.Login.Configured {
			t.Fatalf("Login = %#v, want no configured login command", record.Login)
		}
		if strings.Contains(stdout, "/final/bin/status-helper") {
			t.Fatalf("stdout leaked resolved CLI path: %s", stdout)
		}
	})

	t.Run("Should report missing native CLI before probing status command", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, nil)
		deps.loadConfig = func() (compozyconfig.Config, error) {
			cfg := compozyconfig.DefaultWithHome(mustTestHomePaths(t))
			cfg.Providers["local"] = compozyconfig.ProviderConfig{
				Command:       "missing-agent acp",
				AuthMode:      compozyconfig.ProviderAuthModeNativeCLI,
				AuthStatusCmd: "missing-agent auth status",
				AuthLoginCmd:  "missing-agent auth login",
			}
			return cfg, nil
		}
		resolveCalls := 0
		deps.resolveProviderAuthCommand = func(_ context.Context, name string, _ []string, _ string) (string, error) {
			resolveCalls++
			if name != "missing-agent" {
				t.Fatalf("lookPath(%q), want missing-agent", name)
			}
			if resolveCalls == 1 {
				return "", exec.ErrNotFound
			}
			return "/later/bin/missing-agent", nil
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
		if got, want := record.Code, contract.CodeProviderCLIMissing; got != want {
			t.Fatalf("Code = %q, want %q", got, want)
		}
		if !record.Login.Configured || record.Login.Executable != "missing-agent" ||
			record.Login.Presence != authproviders.ProviderLoginPresenceMissing {
			t.Fatalf("Login = %#v, want missing missing-agent descriptor", record.Login)
		}
		if record.Probe != nil {
			t.Fatalf("Probe = %#v, want no probe when native CLI is missing", record.Probe)
		}
		if strings.Contains(record.Message, "missing-agent auth login") {
			t.Fatalf("Message = %q, leaked full login command", record.Message)
		}
		if got, want := record.Login.RecommendedAction, authproviders.ProviderFailureActionInstallCLI; got != want {
			t.Fatalf("Login.RecommendedAction = %q, want %q", got, want)
		}
		if got, want := resolveCalls, 1; got != want {
			t.Fatalf("auth status resolver calls = %d, want %d", got, want)
		}
	})

	t.Run("Should expose a safe login descriptor when native status probe needs login", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, nil)
		deps.loadConfig = func() (compozyconfig.Config, error) {
			cfg := compozyconfig.DefaultWithHome(mustTestHomePaths(t))
			cfg.Providers["local"] = compozyconfig.ProviderConfig{
				Command:       "claude acp",
				AuthMode:      compozyconfig.ProviderAuthModeNativeCLI,
				AuthStatusCmd: "claude auth status",
				AuthLoginCmd:  "claude auth login",
			}
			return cfg, nil
		}
		deps.resolveProviderAuthCommand = func(_ context.Context, name string, _ []string, _ string) (string, error) {
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
		if strings.Contains(record.Message, "claude auth login") {
			t.Fatalf("Message = %q, leaked full login command", record.Message)
		}
		if !record.Login.Configured || record.Login.Executable != "claude" ||
			record.Login.Presence != authproviders.ProviderLoginPresencePresent ||
			record.Login.RecommendedAction != authproviders.ProviderFailureActionLogin {
			t.Fatalf("Login = %#v, want present claude login action", record.Login)
		}
		if strings.Contains(stdout, "/usr/local/bin/claude") {
			t.Fatalf("stdout leaked resolved CLI path: %s", stdout)
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
		deps.loadConfig = func() (compozyconfig.Config, error) {
			cfg := compozyconfig.DefaultWithHome(mustTestHomePaths(t))
			cfg.Providers["custom"] = compozyconfig.ProviderConfig{
				Command:  "custom-agent --acp",
				AuthMode: compozyconfig.ProviderAuthModeBoundSecret,
				CredentialSlots: []compozyconfig.ProviderCredentialSlot{
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
		if got, want := os.Getenv("COMPOZY_HOME"), hermetic.HomeDir; got != want {
			t.Fatalf("COMPOZY_HOME = %q, want %q", got, want)
		}
	})
}

func TestProviderAuthLoginCommand(t *testing.T) {
	t.Parallel()

	t.Run("Should run configured native login command locally", func(t *testing.T) {
		t.Parallel()

		const loginCommand = "QA_LOGIN_TOKEN=raw-env-secret codex login --tenant hidden-account"
		deps := newTestDeps(t, nil)
		deps.loadConfig = func() (compozyconfig.Config, error) {
			cfg := compozyconfig.DefaultWithHome(mustTestHomePaths(t))
			cfg.Providers["codex"] = compozyconfig.ProviderConfig{
				AuthLoginCmd: loginCommand,
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
			if spec.Command != loginCommand {
				t.Fatalf("Login command = %q, want configured command", spec.Command)
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
		if record.Code != "provider_classification_unknown" {
			t.Fatalf("Code = %q, want provider_classification_unknown", record.Code)
		}
		if !record.Login.Configured || record.Login.Executable != "codex" ||
			record.Login.Presence != authproviders.ProviderLoginPresencePresent {
			t.Fatalf("Login = %#v, want present codex descriptor", record.Login)
		}
		for _, forbidden := range []string{
			"QA_LOGIN_TOKEN",
			"raw-env-secret",
			"hidden-account",
			"codex login",
			`"login_command":`,
		} {
			if strings.Contains(stdout, forbidden) {
				t.Fatalf("stdout leaked login detail %q: %s", forbidden, stdout)
			}
		}
	})

	t.Run("Should reject the removed print-command flag", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, nil)
		deps.runProviderAuthLoginCommand = func(
			_ context.Context,
			spec providerAuthCommandSpec,
		) (providerAuthCommandResult, error) {
			t.Fatalf("runProviderAuthLoginCommand(%q) called for removed flag", spec.Command)
			return providerAuthCommandResult{}, nil
		}

		stdout, _, err := executeRootCommand(t, deps, "provider", "auth", "login", "codex", "--print-command")
		if err == nil {
			t.Fatal("provider auth login --print-command error = nil, want unknown flag")
		}
		if !strings.Contains(err.Error(), "unknown flag: --print-command") {
			t.Fatalf("provider auth login --print-command error = %v, want unknown flag", err)
		}
		if strings.Contains(stdout, "codex login") {
			t.Fatalf("stdout leaked raw login command: %s", stdout)
		}
	})

	t.Run("Should expose builtin Pi login against the isolated Pi auth directory", func(t *testing.T) {
		t.Parallel()

		homePaths := mustTestHomePaths(t)
		deps := newTestDeps(t, nil)
		deps.resolveHome = func() (compozyconfig.HomePaths, error) {
			return homePaths, nil
		}
		deps.loadConfig = func() (compozyconfig.Config, error) {
			cfg := compozyconfig.DefaultWithHome(homePaths)
			cfg.Providers["pi"] = compozyconfig.ProviderConfig{
				EnvPolicy:  compozyconfig.ProviderEnvPolicyIsolated,
				HomePolicy: compozyconfig.ProviderHomePolicyIsolated,
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
		if got, want := record.State, "unknown"; got != want {
			t.Fatalf("State = %q, want %q", got, want)
		}
		if !record.Login.Configured || record.Login.Executable != "npx" ||
			record.Login.Presence != authproviders.ProviderLoginPresencePresent {
			t.Fatalf("Login = %#v, want present npx descriptor", record.Login)
		}
		for _, forbidden := range []string{
			"npx -y pi-acp@latest --terminal-login",
			"PI_CODING_AGENT_DIR=",
			`"login_env":`,
			`"native_cli":`,
		} {
			if strings.Contains(stdout, forbidden) {
				t.Fatalf("stdout leaked forbidden login detail %q: %s", forbidden, stdout)
			}
		}
	})

	t.Run("Should fail safely when native login CLI is missing", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, nil)
		deps.loadConfig = func() (compozyconfig.Config, error) {
			cfg := compozyconfig.DefaultWithHome(mustTestHomePaths(t))
			cfg.Providers["local"] = compozyconfig.ProviderConfig{
				Command:      "local-agent acp",
				AuthMode:     compozyconfig.ProviderAuthModeNativeCLI,
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

		_, _, err := executeRootCommand(t, deps, "provider", "auth", "login", "local")
		if err == nil {
			t.Fatal("provider auth login missing CLI error = nil, want missing CLI error")
		}
		if !strings.Contains(err.Error(), `native CLI "missing-agent" was not found on PATH`) {
			t.Fatalf("provider auth login missing CLI error = %v, want missing CLI guidance", err)
		}
		if strings.Contains(err.Error(), "missing-agent auth login") {
			t.Fatalf("provider auth login missing CLI error leaked raw command: %v", err)
		}
	})

	t.Run("Should explain native login boundary when login command is missing", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, nil)
		deps.loadConfig = func() (compozyconfig.Config, error) {
			cfg := compozyconfig.DefaultWithHome(mustTestHomePaths(t))
			cfg.Providers["local"] = compozyconfig.ProviderConfig{
				Command:  "local-agent acp",
				AuthMode: compozyconfig.ProviderAuthModeNativeCLI,
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
}

func mustTestHomePaths(t *testing.T) compozyconfig.HomePaths {
	t.Helper()

	homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
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
