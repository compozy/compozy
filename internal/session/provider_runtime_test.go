package session

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/acp"
	aghconfig "github.com/compozy/agh/internal/config"
	diagcontract "github.com/compozy/agh/internal/diagnosticcontract"
	"github.com/compozy/agh/internal/diagnostics"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
	"github.com/compozy/agh/internal/vault"
)

type fakeProviderSecretResolver struct {
	values map[string]string
	errs   map[string]error
}

func (r fakeProviderSecretResolver) ResolveRef(ctx context.Context, ref string) (string, error) {
	if ctx == nil {
		return "", errors.New("test resolver context is required")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err, ok := r.errs[ref]; ok {
		return "", err
	}
	if value, ok := r.values[ref]; ok {
		return value, nil
	}
	return "", vault.ErrSecretNotFound
}

func TestPrepareProviderForStartExposesAuthMetadataAndIsolatedHome(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should expose provider auth metadata without injecting native credentials",
		func(t *testing.T) {
			t.Parallel()

			manager := &Manager{providerSecrets: fakeProviderSecretResolver{}}
			resolved := aghconfig.ResolvedAgent{
				Provider:   "claude",
				Model:      "claude-sonnet-4-6",
				Harness:    aghconfig.ProviderHarnessACP,
				AuthMode:   aghconfig.ProviderAuthModeNativeCLI,
				EnvPolicy:  aghconfig.ProviderEnvPolicyFiltered,
				HomePolicy: aghconfig.ProviderHomePolicyOperator,
			}

			opts, err := manager.prepareProviderForStart(
				testutil.Context(t),
				&Session{},
				resolved,
				acp.StartOpts{
					Env: []string{"ANTHROPIC_API_KEY=parent-secret", "KEEP=1"},
				},
			)
			if err != nil {
				t.Fatalf("prepareProviderForStart(native auth) error = %v", err)
			}
			if got := envValue(opts.Env, "ANTHROPIC_API_KEY"); got != "parent-secret" {
				t.Fatalf("ANTHROPIC_API_KEY = %q, want untouched native CLI env", got)
			}
			if got := envValue(opts.Env, "ANTHROPIC_MODEL"); got != "claude-sonnet-4-6" {
				t.Fatalf("ANTHROPIC_MODEL = %q, want claude-sonnet-4-6", got)
			}
			if got := envValue(opts.Env, "AGH_MODEL"); got != "claude-sonnet-4-6" {
				t.Fatalf("AGH_MODEL = %q, want claude-sonnet-4-6", got)
			}
			if got := envValue(opts.Env, "AGH_PROVIDER_AUTH_MODE"); got != "native_cli" {
				t.Fatalf("AGH_PROVIDER_AUTH_MODE = %q, want native_cli", got)
			}
			if got := envValue(opts.Env, "AGH_PROVIDER_ENV_POLICY"); got != "filtered" {
				t.Fatalf("AGH_PROVIDER_ENV_POLICY = %q, want filtered", got)
			}
			if got := envValue(opts.Env, "AGH_PROVIDER_HOME_POLICY"); got != "operator" {
				t.Fatalf("AGH_PROVIDER_HOME_POLICY = %q, want operator", got)
			}
		},
	)

	t.Run(
		"Should set an AGH-owned provider home when isolated home policy is selected",
		func(t *testing.T) {
			t.Parallel()

			homePaths, err := aghconfig.ResolveHomePathsFrom(t.TempDir())
			if err != nil {
				t.Fatalf("ResolveHomePathsFrom() error = %v", err)
			}
			manager := &Manager{
				homePaths:       homePaths,
				providerSecrets: fakeProviderSecretResolver{},
			}
			resolved := aghconfig.ResolvedAgent{
				Provider:   "codex",
				Model:      "gpt-5.4",
				Harness:    aghconfig.ProviderHarnessACP,
				AuthMode:   aghconfig.ProviderAuthModeNativeCLI,
				EnvPolicy:  aghconfig.ProviderEnvPolicyIsolated,
				HomePolicy: aghconfig.ProviderHomePolicyIsolated,
			}

			opts, err := manager.prepareProviderForStart(
				testutil.Context(t),
				&Session{},
				resolved,
				acp.StartOpts{
					Env: []string{"HOME=/Users/operator", "CODEX_HOME=/Users/operator/.codex"},
				},
			)
			if err != nil {
				t.Fatalf("prepareProviderForStart(isolated home) error = %v", err)
			}
			wantHome := filepath.Join(homePaths.HomeDir, "providers", "codex")
			if got := envValue(opts.Env, "PROVIDER_HOME"); got != wantHome {
				t.Fatalf("PROVIDER_HOME = %q, want %q", got, wantHome)
			}
			if got := envValue(opts.Env, "HOME"); got != wantHome {
				t.Fatalf("HOME = %q, want %q", got, wantHome)
			}
			if got := envValue(opts.Env, "CODEX_HOME"); got != filepath.Join(wantHome, "codex") {
				t.Fatalf("CODEX_HOME = %q, want isolated codex home", got)
			}
			if got := envValue(opts.Env, "ANTHROPIC_MODEL"); got != "" {
				t.Fatalf("ANTHROPIC_MODEL = %q, want empty for codex provider", got)
			}
			assertProviderRuntimeFileMode(t, wantHome, 0o700)
			assertProviderRuntimeFileMode(t, filepath.Join(wantHome, "codex"), 0o700)
		},
	)

	t.Run("Should preserve operator codex home for regular codex sessions", func(t *testing.T) {
		t.Parallel()

		manager := &Manager{providerSecrets: fakeProviderSecretResolver{}}
		resolved := aghconfig.ResolvedAgent{
			Name:       "coder",
			Provider:   "codex",
			Model:      "gpt-5.5",
			Harness:    aghconfig.ProviderHarnessACP,
			AuthMode:   aghconfig.ProviderAuthModeNativeCLI,
			EnvPolicy:  aghconfig.ProviderEnvPolicyFiltered,
			HomePolicy: aghconfig.ProviderHomePolicyOperator,
		}

		opts, err := manager.prepareProviderForStart(testutil.Context(t), &Session{
			AgentName:   "coder",
			WorkspaceID: "ws_test",
		}, resolved, acp.StartOpts{
			Env: []string{"CODEX_HOME=/Users/operator/.codex"},
		})
		if err != nil {
			t.Fatalf("prepareProviderForStart(regular codex) error = %v", err)
		}
		if got := envValue(opts.Env, "CODEX_HOME"); got != "/Users/operator/.codex" {
			t.Fatalf("CODEX_HOME = %q, want operator codex home", got)
		}
	})

	t.Run("Should preserve operator Pi auth directory for native Pi providers", func(t *testing.T) {
		t.Parallel()

		manager := &Manager{providerSecrets: fakeProviderSecretResolver{}}
		session := &Session{sessionDir: t.TempDir()}
		resolved := aghconfig.ResolvedAgent{
			Provider:        "pi",
			Model:           "claude-opus-4-7",
			Harness:         aghconfig.ProviderHarnessPiACP,
			RuntimeProvider: "anthropic",
			AuthMode:        aghconfig.ProviderAuthModeNativeCLI,
			EnvPolicy:       aghconfig.ProviderEnvPolicyFiltered,
			HomePolicy:      aghconfig.ProviderHomePolicyOperator,
		}

		opts, err := manager.prepareProviderForStart(
			testutil.Context(t),
			session,
			resolved,
			acp.StartOpts{
				Env: []string{"KEEP=1"},
			},
		)
		if err != nil {
			t.Fatalf("prepareProviderForStart(native pi operator home) error = %v", err)
		}
		if got := envValue(opts.Env, "PI_CODING_AGENT_DIR"); got != "" {
			t.Fatalf("PI_CODING_AGENT_DIR = %q, want operator Pi auth path untouched", got)
		}
		if got := envValue(opts.Env, "AGH_PROVIDER_AUTH_MODE"); got != "native_cli" {
			t.Fatalf("AGH_PROVIDER_AUTH_MODE = %q, want native_cli", got)
		}
		assertNoPath(t, filepath.Join(session.sessionDir, "provider-runtime", "pi"))
	})

	t.Run(
		"Should isolate Pi auth directory when isolated home policy is selected",
		func(t *testing.T) {
			t.Parallel()

			homePaths, err := aghconfig.ResolveHomePathsFrom(t.TempDir())
			if err != nil {
				t.Fatalf("ResolveHomePathsFrom() error = %v", err)
			}
			manager := &Manager{
				homePaths:       homePaths,
				providerSecrets: fakeProviderSecretResolver{},
			}
			session := &Session{sessionDir: t.TempDir()}
			resolved := aghconfig.ResolvedAgent{
				Provider:        "pi",
				Model:           "claude-opus-4-7",
				Harness:         aghconfig.ProviderHarnessPiACP,
				RuntimeProvider: "anthropic",
				AuthMode:        aghconfig.ProviderAuthModeNativeCLI,
				EnvPolicy:       aghconfig.ProviderEnvPolicyIsolated,
				HomePolicy:      aghconfig.ProviderHomePolicyIsolated,
			}

			opts, err := manager.prepareProviderForStart(
				testutil.Context(t),
				session,
				resolved,
				acp.StartOpts{
					Env: []string{
						"HOME=/Users/operator",
						"PI_CODING_AGENT_DIR=/Users/operator/.pi/agent",
					},
				},
			)
			if err != nil {
				t.Fatalf("prepareProviderForStart(native pi isolated home) error = %v", err)
			}
			wantHome := filepath.Join(homePaths.HomeDir, "providers", "pi")
			wantAgentDir := filepath.Join(wantHome, ".pi", "agent")
			if got := envValue(opts.Env, "HOME"); got != wantHome {
				t.Fatalf("HOME = %q, want %q", got, wantHome)
			}
			if got := envValue(opts.Env, "PI_CODING_AGENT_DIR"); got != wantAgentDir {
				t.Fatalf("PI_CODING_AGENT_DIR = %q, want %q", got, wantAgentDir)
			}
			assertProviderRuntimeFileMode(t, wantAgentDir, 0o700)
			assertNoPath(t, filepath.Join(session.sessionDir, "provider-runtime", "pi"))
		},
	)
}

func TestPrepareProviderForStartInjectsSecretsAndMaterializesPiRuntime(t *testing.T) {
	t.Parallel()

	t.Run("Should inject resolved provider secrets and redact dynamic values", func(t *testing.T) {
		t.Parallel()

		secret := "sk-provider-runtime-secret-123456"
		manager := &Manager{
			providerSecrets: fakeProviderSecretResolver{
				values: map[string]string{
					"vault:providers/openrouter/api-key":   secret,
					"vault:providers/openrouter/secondary": "secondary-provider-runtime-secret",
				},
			},
		}
		session := &Session{sessionDir: t.TempDir()}
		t.Cleanup(session.clearProviderSecretRedactions)

		resolved := aghconfig.ResolvedAgent{
			Provider:        "openrouter",
			Model:           "openai/gpt-5.4",
			Harness:         aghconfig.ProviderHarnessPiACP,
			RuntimeProvider: "openrouter",
			BaseURL:         "https://openrouter.ai/api/v1",
			Transport:       "openai",
			AuthMode:        aghconfig.ProviderAuthModeBoundSecret,
			CredentialSlots: []aghconfig.ProviderCredentialSlot{
				{
					Name:      "secondary",
					TargetEnv: "OPENROUTER_SECONDARY_TOKEN",
					SecretRef: "vault:providers/openrouter/secondary",
					Kind:      "oauth",
					Required:  false,
				},
				{
					Name:      "api_key",
					TargetEnv: "OPENROUTER_API_KEY",
					SecretRef: "vault:providers/openrouter/api-key",
					Kind:      "api_key",
					Required:  true,
				},
			},
		}

		opts, err := manager.prepareProviderForStart(
			testutil.Context(t),
			session,
			resolved,
			acp.StartOpts{
				Env: []string{"OPENROUTER_API_KEY=old", "KEEP=1"},
			},
		)
		if err != nil {
			t.Fatalf("prepareProviderForStart() error = %v", err)
		}

		if got := envValue(opts.Env, "OPENROUTER_API_KEY"); got != secret {
			t.Fatalf("OPENROUTER_API_KEY = %q, want injected secret", got)
		}
		runtimeDir := envValue(opts.Env, "PI_CODING_AGENT_DIR")
		if runtimeDir == "" {
			t.Fatal("PI_CODING_AGENT_DIR = empty, want materialized pi runtime directory")
		}
		assertProviderRuntimeFileMode(t, runtimeDir, 0o700)
		settings := readProviderJSON[piSettingsFile](t, filepath.Join(runtimeDir, "settings.json"))
		if settings.DefaultProvider != "openrouter" || settings.DefaultModel != "openai/gpt-5.4" {
			t.Fatalf("settings.json = %#v, want openrouter defaults", settings)
		}
		assertProviderRuntimeFileMode(t, filepath.Join(runtimeDir, "settings.json"), 0o600)
		models := readProviderJSON[piModelsFile](t, filepath.Join(runtimeDir, "models.json"))
		provider := models.Providers["openrouter"]
		if provider.APIKey != "OPENROUTER_API_KEY" {
			t.Fatalf("models.json apiKey = %q, want injected env name", provider.APIKey)
		}
		if provider.BaseURL != "https://openrouter.ai/api/v1" || provider.API != "openai" {
			t.Fatalf("models.json provider = %#v, want base URL and API transport", provider)
		}
		assertProviderRuntimeFileMode(t, filepath.Join(runtimeDir, "models.json"), 0o600)
		assertPiRuntimeEnvContract(t, opts, "openrouter", secret)
		payload, err := os.ReadFile(filepath.Join(runtimeDir, "models.json"))
		if err != nil {
			t.Fatalf("ReadFile(models.json) error = %v", err)
		}
		if strings.Contains(string(payload), secret) {
			t.Fatalf("models.json leaked secret payload: %s", payload)
		}
		redacted := diagnostics.RedactAndBound("stderr leaked "+secret, 256)
		if strings.Contains(redacted, secret) {
			t.Fatalf("RedactAndBound(dynamic provider secret) = %q leaked secret", redacted)
		}
	})

	t.Run(
		"Should replace stale pi runtime files with private file and directory modes",
		func(t *testing.T) {
			t.Parallel()

			secret := "sk-provider-runtime-replacement-secret"
			manager := &Manager{
				providerSecrets: fakeProviderSecretResolver{
					values: map[string]string{
						"vault:providers/openrouter/api-key": secret,
					},
				},
			}
			sessionDir := t.TempDir()
			runtimeDir := filepath.Join(sessionDir, "provider-runtime", "pi")
			if err := os.MkdirAll(runtimeDir, 0o755); err != nil {
				t.Fatalf("MkdirAll(runtimeDir) error = %v", err)
			}
			if err := os.Chmod(runtimeDir, 0o755); err != nil {
				t.Fatalf("Chmod(runtimeDir) error = %v", err)
			}
			settingsPath := filepath.Join(runtimeDir, "settings.json")
			if err := os.WriteFile(settingsPath, []byte("{\"defaultProvider\":\"stale\"}"), 0o644); err != nil {
				t.Fatalf("WriteFile(stale settings) error = %v", err)
			}
			session := &Session{sessionDir: sessionDir}
			t.Cleanup(session.clearProviderSecretRedactions)
			resolved := aghconfig.ResolvedAgent{
				Provider:        "openrouter",
				Model:           "openai/gpt-5.4",
				Harness:         aghconfig.ProviderHarnessPiACP,
				RuntimeProvider: "openrouter",
				Transport:       "openai",
				AuthMode:        aghconfig.ProviderAuthModeBoundSecret,
				CredentialSlots: []aghconfig.ProviderCredentialSlot{{
					Name:      "api_key",
					TargetEnv: "OPENROUTER_API_KEY",
					SecretRef: "vault:providers/openrouter/api-key",
					Kind:      "api_key",
					Required:  true,
				}},
			}

			opts, err := manager.prepareProviderForStart(
				testutil.Context(t),
				session,
				resolved,
				acp.StartOpts{},
			)
			if err != nil {
				t.Fatalf("prepareProviderForStart() error = %v", err)
			}

			if got := envValue(opts.Env, "PI_CODING_AGENT_DIR"); got != runtimeDir {
				t.Fatalf("PI_CODING_AGENT_DIR = %q, want %q", got, runtimeDir)
			}
			assertProviderRuntimeFileMode(t, runtimeDir, 0o700)
			assertProviderRuntimeFileMode(t, settingsPath, 0o600)
			settings := readProviderJSON[piSettingsFile](t, settingsPath)
			if settings.DefaultProvider != "openrouter" ||
				settings.DefaultModel != "openai/gpt-5.4" {
				t.Fatalf("settings.json = %#v, want replacement runtime config", settings)
			}
		},
	)

	t.Run("Should omit pi apiKey when optional credential is missing", func(t *testing.T) {
		t.Parallel()

		manager := &Manager{
			providerSecrets: fakeProviderSecretResolver{
				errs: map[string]error{
					"vault:providers/openrouter/api-key": vault.ErrMissingSecret,
				},
			},
		}
		session := &Session{sessionDir: t.TempDir()}
		resolved := aghconfig.ResolvedAgent{
			Provider:        "openrouter",
			Model:           "openai/gpt-5.4",
			Harness:         aghconfig.ProviderHarnessPiACP,
			RuntimeProvider: "openrouter",
			AuthMode:        aghconfig.ProviderAuthModeBoundSecret,
			CredentialSlots: []aghconfig.ProviderCredentialSlot{
				{
					Name:      "api_key",
					TargetEnv: "OPENROUTER_API_KEY",
					SecretRef: "vault:providers/openrouter/api-key",
					Kind:      "api_key",
					Required:  false,
				},
			},
		}

		opts, err := manager.prepareProviderForStart(
			testutil.Context(t),
			session,
			resolved,
			acp.StartOpts{
				Env: []string{"OPENROUTER_API_KEY=parent-shell-secret"},
			},
		)
		if err != nil {
			t.Fatalf("prepareProviderForStart(optional missing) error = %v", err)
		}
		if got := envValue(opts.Env, "OPENROUTER_API_KEY"); got != "" {
			t.Fatalf("OPENROUTER_API_KEY = %q, want scrubbed env for missing vault secret", got)
		}
		runtimeDir := envValue(opts.Env, "PI_CODING_AGENT_DIR")
		if runtimeDir == "" {
			t.Fatal("PI_CODING_AGENT_DIR = empty, want materialized pi runtime")
		}
		models := readProviderJSON[piModelsFile](t, filepath.Join(runtimeDir, "models.json"))
		if got := models.Providers["openrouter"].APIKey; got != "" {
			t.Fatalf(
				"models.json apiKey = %q, want omitted apiKey for missing optional secret",
				got,
			)
		}
		payload, err := os.ReadFile(filepath.Join(runtimeDir, "models.json"))
		if err != nil {
			t.Fatalf("ReadFile(models.json) error = %v", err)
		}
		if strings.Contains(string(payload), `"apiKey"`) {
			t.Fatalf("models.json contains apiKey for missing optional secret: %s", payload)
		}
	})

	t.Run(
		"Should classify missing required bound secret as provider auth failure",
		func(t *testing.T) {
			t.Parallel()

			manager := &Manager{
				providerSecrets: fakeProviderSecretResolver{
					errs: map[string]error{
						"vault:providers/openrouter/api-key": vault.ErrMissingSecret,
					},
				},
			}
			resolved := aghconfig.ResolvedAgent{
				Provider: "openrouter",
				Model:    "openai/gpt-5.4",
				Harness:  aghconfig.ProviderHarnessACP,
				AuthMode: aghconfig.ProviderAuthModeBoundSecret,
				CredentialSlots: []aghconfig.ProviderCredentialSlot{{
					Name:      "api_key",
					TargetEnv: "OPENROUTER_API_KEY",
					SecretRef: "vault:providers/openrouter/api-key",
					Kind:      "api_key",
					Required:  true,
				}},
			}

			_, err := manager.prepareProviderForStart(
				testutil.Context(t),
				&Session{sessionDir: t.TempDir()},
				resolved,
				acp.StartOpts{},
			)
			if err == nil {
				t.Fatal(
					"prepareProviderForStart(missing required secret) error = nil, want provider auth failure",
				)
			}
			var failure *acp.FailureError
			if !errors.As(err, &failure) || failure == nil {
				t.Fatalf(
					"prepareProviderForStart(missing required secret) error = %v, want FailureError",
					err,
				)
			}
			if got, want := failure.Kind, store.FailureProviderAuth; got != want {
				t.Fatalf("FailureError.Kind = %q, want %q", got, want)
			}
			item, ok := diagnostics.ItemFromError(err)
			if !ok {
				t.Fatalf("diagnostics.ItemFromError() ok = false for %v", err)
			}
			if got, want := item.Code, diagcontract.CodeProviderCredentialUnresolved; got != want {
				t.Fatalf("Diagnostic.Code = %q, want %q", got, want)
			}
		},
	)

	t.Run(
		"Should pass native Pi model selection through session start options",
		func(t *testing.T) {
			t.Parallel()

			h := newHarness(t)
			resolved, err := h.resolver.Resolve(testutil.Context(t), h.workspaceID)
			if err != nil {
				t.Fatalf("Resolve(workspace) error = %v", err)
			}
			resolved.Agents = []aghconfig.AgentDef{
				{
					Name:     "coder",
					Provider: "pi",
					Model:    "claude-opus-4-7",
					Prompt:   "You are a coding assistant.",
				},
			}
			h.resolver.upsert(&resolved)
			h.driver.startHook = func(opts acp.StartOpts, _ int) (*fakeProcess, error) {
				if !strings.Contains(opts.Command, "pi-acp@latest") {
					t.Fatalf("StartOpts.Command = %q, want pi-acp command", opts.Command)
				}
				if got := opts.PreferredModel; got != "anthropic/claude-opus-4-7" {
					t.Fatalf("StartOpts.PreferredModel = %q, want anthropic/claude-opus-4-7", got)
				}
				if got := envValue(opts.Env, "PI_CODING_AGENT_DIR"); got != "" {
					t.Fatalf("PI_CODING_AGENT_DIR = %q, want operator Pi auth path untouched", got)
				}
				if got := envValue(opts.Env, "AGH_PROVIDER_AUTH_MODE"); got != "native_cli" {
					t.Fatalf("AGH_PROVIDER_AUTH_MODE = %q, want native_cli", got)
				}
				return newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, "acp-pi-runtime"), nil
			}

			session, err := h.manager.Create(testutil.Context(t), CreateOpts{
				AgentName: "coder",
				Name:      "pi-native-model-contract",
				Workspace: h.workspaceID,
			})
			if err != nil {
				t.Fatalf("Create(pi native model contract) error = %v", err)
			}
			t.Cleanup(func() {
				if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
					t.Fatalf("Stop(pi native model contract cleanup) error = %v", err)
				}
			})
		},
	)
}

func TestPreferredACPModelUsesProviderTransportValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		provider string
		model    string
		want     string
	}{
		{
			name:     "Should translate canonical Claude Sonnet ID at the ACP boundary",
			provider: runtimeProviderClaude,
			model:    "claude-sonnet-5",
			want:     "sonnet",
		},
		{
			name:     "Should translate canonical Claude Opus ID at the ACP boundary",
			provider: runtimeProviderClaude,
			model:    "claude-opus-4-8",
			want:     "opus[1m]",
		},
		{
			name:     "Should preserve an unknown Claude model for live validation",
			provider: runtimeProviderClaude,
			model:    "custom-claude-model",
			want:     "custom-claude-model",
		},
		{
			name:     "Should preserve the canonical Codex model value",
			provider: runtimeProviderCodex,
			model:    "gpt-5.6-sol",
			want:     "gpt-5.6-sol",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := preferredACPModel(aghconfig.ResolvedAgent{
				Provider:        test.provider,
				RuntimeProvider: test.provider,
				Harness:         aghconfig.ProviderHarnessACP,
				Model:           test.model,
			}, true)
			if got != test.want {
				t.Fatalf("preferredACPModel() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestShouldSkipMissingProviderSecret(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		resolved aghconfig.ResolvedAgent
		ref      string
		slot     aghconfig.ProviderCredentialSlot
		err      error
		want     bool
	}{
		{
			name:     "Should skip optional missing pi vault secret",
			resolved: aghconfig.ResolvedAgent{Harness: aghconfig.ProviderHarnessPiACP},
			ref:      "vault:providers/openrouter/api-key",
			slot:     aghconfig.ProviderCredentialSlot{Required: false},
			err:      vault.ErrMissingSecret,
			want:     true,
		},
		{
			name:     "Should require missing pi vault secret when slot is required",
			resolved: aghconfig.ResolvedAgent{Harness: aghconfig.ProviderHarnessPiACP},
			ref:      "vault:providers/openrouter/api-key",
			slot:     aghconfig.ProviderCredentialSlot{Required: true},
			err:      vault.ErrMissingSecret,
			want:     false,
		},
		{
			name:     "Should require missing env refs for direct acp providers when slot is required",
			resolved: aghconfig.ResolvedAgent{Harness: aghconfig.ProviderHarnessACP},
			ref:      "env:ANTHROPIC_API_KEY",
			slot:     aghconfig.ProviderCredentialSlot{Required: true},
			err:      vault.ErrMissingSecret,
			want:     false,
		},
		{
			name:     "Should skip optional missing env refs for direct acp providers",
			resolved: aghconfig.ResolvedAgent{Harness: aghconfig.ProviderHarnessACP},
			ref:      "env:ANTHROPIC_API_KEY",
			slot:     aghconfig.ProviderCredentialSlot{Required: false},
			err:      vault.ErrMissingSecret,
			want:     true,
		},
		{
			name:     "Should skip optional missing secret refs for direct acp providers",
			resolved: aghconfig.ResolvedAgent{Harness: aghconfig.ProviderHarnessACP},
			ref:      "vault:providers/custom/api-key",
			slot:     aghconfig.ProviderCredentialSlot{Required: false},
			err:      vault.ErrSecretNotFound,
			want:     true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := shouldSkipMissingProviderSecret(tc.resolved, tc.ref, tc.slot, tc.err); got != tc.want {
				t.Fatalf("shouldSkipMissingProviderSecret() = %t, want %t", got, tc.want)
			}
		})
	}
}

func envValue(env []string, key string) string {
	prefix := key + "="
	for _, entry := range env {
		if value, ok := strings.CutPrefix(entry, prefix); ok {
			return value
		}
	}
	return ""
}

func assertProviderRuntimeFileMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(%q) error = %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("mode(%q) = %#o, want %#o", path, got, want)
	}
}

func assertNoPath(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); err == nil {
		t.Fatalf("path %q exists, want absent", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Stat(%q) error = %v, want not exist", path, err)
	}
}

func assertPiRuntimeEnvContract(
	t *testing.T,
	opts acp.StartOpts,
	providerName string,
	wantSecret string,
) {
	t.Helper()

	runtimeDir := envValue(opts.Env, "PI_CODING_AGENT_DIR")
	if runtimeDir == "" {
		t.Fatal("PI_CODING_AGENT_DIR = empty, want materialized pi runtime directory")
	}
	models := readProviderJSON[piModelsFile](t, filepath.Join(runtimeDir, "models.json"))
	provider, ok := models.Providers[providerName]
	if !ok {
		t.Fatalf("models.json providers = %#v, want provider %q", models.Providers, providerName)
	}
	if provider.APIKey == "" {
		t.Fatalf("models.json providers[%q].apiKey = empty, want env var reference", providerName)
	}
	if got := envValue(opts.Env, provider.APIKey); got != wantSecret {
		t.Fatalf("child env %s = %q, want resolved provider secret", provider.APIKey, got)
	}
}

func readProviderJSON[T any](t *testing.T, path string) T {
	t.Helper()

	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	var value T
	if err := json.Unmarshal(payload, &value); err != nil {
		t.Fatalf("Unmarshal(%q) error = %v", path, err)
	}
	return value
}
