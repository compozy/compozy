package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	diagcontract "github.com/compozy/compozy/internal/diagnosticcontract"
	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/vault"
)

type fakeProviderSecretResolver struct {
	values map[string]string
	errs   map[string]error
}

type mutableProviderSecretResolver struct {
	mu     sync.RWMutex
	values map[string]string
}

func (r *mutableProviderSecretResolver) ResolveRef(ctx context.Context, ref string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	value, ok := r.values[ref]
	if !ok {
		return "", vault.ErrSecretNotFound
	}
	return value, nil
}

func (r *mutableProviderSecretResolver) delete(ref string) {
	r.mu.Lock()
	delete(r.values, ref)
	r.mu.Unlock()
}

type profileNameResolverMap map[string]string

func (r profileNameResolverMap) ProfileName(ctx context.Context, profileID string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	name, ok := r[profileID]
	if !ok {
		return "", fmt.Errorf("unknown profile id %q", profileID)
	}
	return name, nil
}

func TestProviderRuntimeEnvironmentLookupUsesPlatformSemantics(t *testing.T) {
	t.Parallel()
	t.Run("Should use platform key rules for provider home and credential lookups", func(t *testing.T) {
		t.Parallel()

		env := []string{
			"HOME=/exact/home",
			"home=/mixed/home",
			"USERPROFILE=C:\\exact-home",
			"UserProfile=C:\\mixed-home",
			"PROVIDER_TOKEN=exact-secret",
			"Provider_Token=mixed-secret",
		}
		lookup := providerLookupEnv(env)
		wantHome := "/exact/home"
		wantToken := "exact-secret"
		if runtime.GOOS == "windows" {
			wantHome = "/mixed/home"
			wantToken = "mixed-secret"
		}
		if got, ok := lookup("HOME"); !ok || got != wantHome {
			t.Fatalf("provider LookupEnv(HOME) = %q, %t; want %q, true", got, ok, wantHome)
		}
		if got, ok := lookup("PROVIDER_TOKEN"); !ok || got != wantToken {
			t.Fatalf("provider LookupEnv(PROVIDER_TOKEN) = %q, %t; want %q, true", got, ok, wantToken)
		}
		if got := providerHomeIdentity(env); got != wantHome {
			t.Fatalf("providerHomeIdentity() = %q, want %q", got, wantHome)
		}

		userProfileEnv := []string{
			"USERPROFILE=C:\\exact-home",
			"UserProfile=C:\\mixed-home",
		}
		wantUserProfile := "C:\\exact-home"
		if runtime.GOOS == "windows" {
			wantUserProfile = "C:\\mixed-home"
		}
		if got := providerHomeIdentity(userProfileEnv); got != wantUserProfile {
			t.Fatalf("providerHomeIdentity(USERPROFILE) = %q, want %q", got, wantUserProfile)
		}
	})

	t.Run("Should retain workspace and profile ownership without a sandbox", func(t *testing.T) {
		t.Parallel()

		const profileID = "01PROFILEMARKETING000000000"
		scope := providerPreStartScopeForSession(&Session{
			WorkspaceID: "ws-marketing",
			ProfileID:   profileID,
		}, []string{"HOME=/provider-home"})
		if scope.WorkspaceID != "ws-marketing" || scope.ProfileID != profileID ||
			scope.HomeIdentity != "/provider-home" {
			t.Fatalf("provider pre-start scope = %#v, want workspace/profile/home ownership", scope)
		}
		if scope.SandboxID != "" || scope.SandboxBackend != "" || scope.SandboxProfile != "" {
			t.Fatalf("provider pre-start sandbox scope = %#v, want empty no-sandbox fields", scope)
		}
	})
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

func TestProviderCredentialOverrideSurvivesInflightRemovalAndFallsBackOnNextRunIT047IT048(t *testing.T) {
	t.Parallel()
	t.Run("Should preserve in-flight profile credentials and fall back on the next run", func(t *testing.T) {
		t.Parallel()
		testProviderCredentialOverrideSurvivesInflightRemovalAndFallsBackOnNextRunIT047IT048(t)
	})
}

func testProviderCredentialOverrideSurvivesInflightRemovalAndFallsBackOnNextRunIT047IT048(t *testing.T) {
	const marketingProfileID = "01PROFILEMARKETING000000000"
	profileRef := "vault:profiles/marketing/providers/openrouter/api_key"
	userRef := "vault:providers/openrouter/api_key"
	secrets := &mutableProviderSecretResolver{values: map[string]string{
		profileRef: "profile-secret",
		userRef:    "user-secret",
	}}
	h := newHarness(
		t,
		WithProviderSecretResolver(secrets),
		WithProfileNameResolver(profileNameResolverMap{
			store.DefaultProfileID: "default",
			marketingProfileID:     "marketing",
		}),
	)
	resolved, err := h.resolver.Resolve(t.Context(), h.workspaceID)
	if err != nil {
		t.Fatalf("Resolve(workspace) error = %v", err)
	}
	resolved.Config.Providers["openrouter"] = compozyconfig.ProviderConfig{
		Command:  "acpmock-openrouter",
		Harness:  compozyconfig.ProviderHarnessACP,
		AuthMode: compozyconfig.ProviderAuthModeBoundSecret,
		CredentialSlots: []compozyconfig.ProviderCredentialSlot{{
			Name: "api_key", TargetEnv: "OPENROUTER_API_KEY", SecretRef: userRef, Required: true,
		}},
	}
	resolved.Agents = []compozyconfig.AgentDef{{
		Name: "credential-agent", Provider: "openrouter", Prompt: "Use the configured credential.",
	}}
	h.resolver.upsert(&resolved)

	startOptions := make(chan acp.StartOpts, 3)
	releaseFirstStart := make(chan struct{})
	h.driver.startHook = func(opts acp.StartOpts, sequence int) (*fakeProcess, error) {
		startOptions <- opts
		if sequence == 1 {
			<-releaseFirstStart
		}
		return newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, fmt.Sprintf("acp-credential-%d", sequence)), nil
	}
	type createResult struct {
		session *Session
		err     error
	}
	firstResult := make(chan createResult, 1)
	go func() {
		session, createErr := h.manager.Create(t.Context(), CreateOpts{
			ProfileID: marketingProfileID,
			AgentName: "credential-agent",
			Name:      "marketing-inflight",
			Workspace: h.workspaceID,
		})
		firstResult <- createResult{session: session, err: createErr}
	}()
	firstOpts := <-startOptions
	if got := envValue(firstOpts.Env, "OPENROUTER_API_KEY"); got != "profile-secret" {
		t.Fatalf("marketing in-flight credential = %q, want profile-secret", got)
	}
	secrets.delete(profileRef)
	close(releaseFirstStart)
	first := <-firstResult
	if first.err != nil {
		t.Fatalf("Create(marketing in-flight) error = %v", first.err)
	}
	if first.session.ProfileID != marketingProfileID {
		t.Fatalf("marketing in-flight owner = %q, want %q", first.session.ProfileID, marketingProfileID)
	}
	if err := h.manager.Stop(t.Context(), first.session.ID); err != nil {
		t.Fatalf("Stop(marketing in-flight) error = %v", err)
	}

	next, err := h.manager.Create(t.Context(), CreateOpts{
		ProfileID: marketingProfileID,
		AgentName: "credential-agent",
		Name:      "marketing-fallback",
		Workspace: h.workspaceID,
	})
	if err != nil {
		t.Fatalf("Create(marketing fallback) error = %v", err)
	}
	nextOpts := <-startOptions
	if got := envValue(nextOpts.Env, "OPENROUTER_API_KEY"); got != "user-secret" {
		t.Fatalf("marketing next-run credential = %q, want user-secret", got)
	}
	if next.ProfileID != marketingProfileID {
		t.Fatalf("marketing fallback owner = %q, want %q", next.ProfileID, marketingProfileID)
	}
	if err := h.manager.Stop(t.Context(), next.ID); err != nil {
		t.Fatalf("Stop(marketing fallback) error = %v", err)
	}

	defaultSession, err := h.manager.Create(t.Context(), CreateOpts{
		ProfileID: store.DefaultProfileID,
		AgentName: "credential-agent",
		Name:      "default-user-credential",
		Workspace: h.workspaceID,
	})
	if err != nil {
		t.Fatalf("Create(default credential) error = %v", err)
	}
	defaultOpts := <-startOptions
	if got := envValue(defaultOpts.Env, "OPENROUTER_API_KEY"); got != "user-secret" {
		t.Fatalf("default credential = %q, want user-secret", got)
	}
	if defaultSession.ProfileID != store.DefaultProfileID {
		t.Fatalf("default owner = %q, want %q", defaultSession.ProfileID, store.DefaultProfileID)
	}
	if err := h.manager.Stop(t.Context(), defaultSession.ID); err != nil {
		t.Fatalf("Stop(default credential) error = %v", err)
	}
}

func TestPrepareProviderForStartExposesAuthMetadataAndIsolatedHome(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should expose provider auth metadata without injecting native credentials",
		func(t *testing.T) {
			t.Parallel()

			manager := &Manager{providerSecrets: fakeProviderSecretResolver{}}
			resolved := compozyconfig.ResolvedAgent{
				Provider:   "claude",
				Model:      "claude-sonnet-4-6",
				Harness:    compozyconfig.ProviderHarnessACP,
				AuthMode:   compozyconfig.ProviderAuthModeNativeCLI,
				EnvPolicy:  compozyconfig.ProviderEnvPolicyFiltered,
				HomePolicy: compozyconfig.ProviderHomePolicyOperator,
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
			if got := envValue(opts.Env, "COMPOZY_MODEL"); got != "claude-sonnet-4-6" {
				t.Fatalf("COMPOZY_MODEL = %q, want claude-sonnet-4-6", got)
			}
			if got := envValue(opts.Env, "COMPOZY_PROVIDER_AUTH_MODE"); got != "native_cli" {
				t.Fatalf("COMPOZY_PROVIDER_AUTH_MODE = %q, want native_cli", got)
			}
			if got := envValue(opts.Env, "COMPOZY_PROVIDER_ENV_POLICY"); got != "filtered" {
				t.Fatalf("COMPOZY_PROVIDER_ENV_POLICY = %q, want filtered", got)
			}
			if got := envValue(opts.Env, "COMPOZY_PROVIDER_HOME_POLICY"); got != "operator" {
				t.Fatalf("COMPOZY_PROVIDER_HOME_POLICY = %q, want operator", got)
			}
		},
	)

	t.Run(
		"Should set an Compozy-owned provider home when isolated home policy is selected",
		func(t *testing.T) {
			t.Parallel()

			homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
			if err != nil {
				t.Fatalf("ResolveHomePathsFrom() error = %v", err)
			}
			manager := &Manager{
				homePaths:       homePaths,
				providerSecrets: fakeProviderSecretResolver{},
			}
			resolved := compozyconfig.ResolvedAgent{
				Provider:   "codex",
				Model:      "gpt-5.4",
				Harness:    compozyconfig.ProviderHarnessACP,
				AuthMode:   compozyconfig.ProviderAuthModeNativeCLI,
				EnvPolicy:  compozyconfig.ProviderEnvPolicyIsolated,
				HomePolicy: compozyconfig.ProviderHomePolicyIsolated,
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

	// Invariant: isolated provider homes override each native provider's state directory.
	// Owning layer: session provider-runtime integration coverage.
	// Canonical suite: TestPrepareProviderForStartExposesAuthMetadataAndIsolatedHome.
	t.Run("Should override native provider directories for isolated homes", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			provider string
			envKey   string
		}{
			{provider: "claude", envKey: "CLAUDE_CONFIG_DIR"},
			{provider: "hermes", envKey: "HERMES_HOME"},
			{provider: "openclaw", envKey: "OPENCLAW_STATE_DIR"},
		}
		for _, test := range tests {
			t.Run("Should override "+test.provider+" native directory for isolated homes", func(t *testing.T) {
				t.Parallel()

				homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
				if err != nil {
					t.Fatalf("ResolveHomePathsFrom() error = %v", err)
				}
				manager := &Manager{
					homePaths:       homePaths,
					providerSecrets: fakeProviderSecretResolver{},
				}
				resolved := compozyconfig.ResolvedAgent{
					Provider:   test.provider,
					Harness:    compozyconfig.ProviderHarnessACP,
					AuthMode:   compozyconfig.ProviderAuthModeNativeCLI,
					EnvPolicy:  compozyconfig.ProviderEnvPolicyIsolated,
					HomePolicy: compozyconfig.ProviderHomePolicyIsolated,
				}

				opts, err := manager.prepareProviderForStart(
					testutil.Context(t),
					&Session{},
					resolved,
					acp.StartOpts{Env: []string{
						"HOME=/Users/operator",
						test.envKey + "=/Users/operator/native",
					}},
				)
				if err != nil {
					t.Fatalf("prepareProviderForStart(%s) error = %v", test.provider, err)
				}

				providerHome := filepath.Join(homePaths.HomeDir, "providers", test.provider)
				wantNativeDir := filepath.Join(providerHome, test.provider)
				if got := envValue(opts.Env, test.envKey); got != wantNativeDir {
					t.Fatalf("%s = %q, want %q", test.envKey, got, wantNativeDir)
				}
				assertProviderRuntimeFileMode(t, wantNativeDir, 0o700)
			})
		}
	})

	t.Run("Should preserve operator codex home for regular codex sessions", func(t *testing.T) {
		t.Parallel()

		manager := &Manager{providerSecrets: fakeProviderSecretResolver{}}
		resolved := compozyconfig.ResolvedAgent{
			Name:       "coder",
			Provider:   "codex",
			Model:      "gpt-5.5",
			Harness:    compozyconfig.ProviderHarnessACP,
			AuthMode:   compozyconfig.ProviderAuthModeNativeCLI,
			EnvPolicy:  compozyconfig.ProviderEnvPolicyFiltered,
			HomePolicy: compozyconfig.ProviderHomePolicyOperator,
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
		resolved := compozyconfig.ResolvedAgent{
			Provider:        "pi",
			Model:           "claude-opus-4-7",
			Harness:         compozyconfig.ProviderHarnessPiACP,
			RuntimeProvider: "anthropic",
			AuthMode:        compozyconfig.ProviderAuthModeNativeCLI,
			EnvPolicy:       compozyconfig.ProviderEnvPolicyFiltered,
			HomePolicy:      compozyconfig.ProviderHomePolicyOperator,
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
		if got := envValue(opts.Env, "COMPOZY_PROVIDER_AUTH_MODE"); got != "native_cli" {
			t.Fatalf("COMPOZY_PROVIDER_AUTH_MODE = %q, want native_cli", got)
		}
		assertNoPath(t, filepath.Join(session.sessionDir, "provider-runtime", "pi"))
	})

	t.Run(
		"Should isolate Pi auth directory when isolated home policy is selected",
		func(t *testing.T) {
			t.Parallel()

			homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
			if err != nil {
				t.Fatalf("ResolveHomePathsFrom() error = %v", err)
			}
			manager := &Manager{
				homePaths:       homePaths,
				providerSecrets: fakeProviderSecretResolver{},
			}
			session := &Session{sessionDir: t.TempDir()}
			resolved := compozyconfig.ResolvedAgent{
				Provider:        "pi",
				Model:           "claude-opus-4-7",
				Harness:         compozyconfig.ProviderHarnessPiACP,
				RuntimeProvider: "anthropic",
				AuthMode:        compozyconfig.ProviderAuthModeNativeCLI,
				EnvPolicy:       compozyconfig.ProviderEnvPolicyIsolated,
				HomePolicy:      compozyconfig.ProviderHomePolicyIsolated,
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

func TestResolveProviderNativeCLIUsesFinalLaunchEnvironment(t *testing.T) {
	t.Run("Should resolve the native CLI from the final launch environment", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS == "windows" {
			t.Skip("executable symlink fixture requires Unix semantics")
		}

		binDir := t.TempDir()
		testExecutable, err := os.Executable()
		if err != nil {
			t.Fatalf("os.Executable() error = %v", err)
		}
		claudeExecutable := filepath.Join(binDir, "claude")
		if err := os.Symlink(testExecutable, claudeExecutable); err != nil {
			t.Fatalf("Symlink(claude executable) error = %v", err)
		}

		opts, err := resolveProviderNativeCLI(
			testutil.Context(t),
			compozyconfig.ResolvedAgent{
				Provider: "claude",
				Command:  "npx -y @agentclientprotocol/claude-agent-acp@latest",
			},
			acp.StartOpts{
				Command: "npx -y @agentclientprotocol/claude-agent-acp@latest",
				Cwd:     t.TempDir(),
				Env:     []string{"PATH=" + binDir},
			},
		)
		if err != nil {
			t.Fatalf("resolveProviderNativeCLI() error = %v", err)
		}
		if got := envValue(opts.Env, "CLAUDE_CODE_EXECUTABLE"); got != claudeExecutable {
			t.Fatalf("CLAUDE_CODE_EXECUTABLE = %q, want %q", got, claudeExecutable)
		}
	})
}

func TestPrepareProviderForStartInjectsSecretsAndMaterializesPiRuntime(t *testing.T) {
	t.Parallel()

	t.Run("Should prefer the active profile credential and retain user fallback semantics", func(t *testing.T) {
		t.Parallel()

		profileRef := "vault:profiles/marketing/providers/openrouter/api_key"
		userRef := "vault:providers/openrouter/api_key"
		manager := &Manager{providerSecrets: fakeProviderSecretResolver{values: map[string]string{
			profileRef: "profile-secret",
			userRef:    "user-secret",
		}}}
		resolved := compozyconfig.ResolvedAgent{
			ProfileName: "marketing",
			Provider:    "openrouter",
			Harness:     compozyconfig.ProviderHarnessACP,
			AuthMode:    compozyconfig.ProviderAuthModeBoundSecret,
			CredentialSlots: []compozyconfig.ProviderCredentialSlot{{
				Name: "api_key", TargetEnv: "OPENROUTER_API_KEY", SecretRef: userRef, Required: true,
			}},
		}
		opts, err := manager.prepareProviderForStart(
			t.Context(),
			&Session{sessionDir: t.TempDir()},
			resolved,
			acp.StartOpts{},
		)
		if err != nil {
			t.Fatalf("prepareProviderForStart(profile override) error = %v", err)
		}
		if got := envValue(opts.Env, "OPENROUTER_API_KEY"); got != "profile-secret" {
			t.Fatalf("OPENROUTER_API_KEY = %q, want profile-secret", got)
		}
		if opts.ProviderConfig == nil || opts.ProviderConfig.CredentialSlots[0].SecretRef != profileRef {
			t.Fatalf("ProviderConfig credentials = %#v, want profile ref %q", opts.ProviderConfig, profileRef)
		}
	})

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

		resolved := compozyconfig.ResolvedAgent{
			Provider:        "openrouter",
			Model:           "openai/gpt-5.4",
			Harness:         compozyconfig.ProviderHarnessPiACP,
			RuntimeProvider: "openrouter",
			BaseURL:         "https://openrouter.ai/api/v1",
			Transport:       "openai",
			AuthMode:        compozyconfig.ProviderAuthModeBoundSecret,
			CredentialSlots: []compozyconfig.ProviderCredentialSlot{
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
		if provider.BaseURL != "https://openrouter.ai/api/v1" || provider.API != "openai" {
			t.Fatalf("models.json provider = %#v, want base URL and API transport", provider)
		}
		assertProviderRuntimeFileMode(t, filepath.Join(runtimeDir, "models.json"), 0o600)
		assertPiRuntimeEnvContract(t, opts, "openrouter", "OPENROUTER_API_KEY", secret)
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
			resolved := compozyconfig.ResolvedAgent{
				Provider:        "openrouter",
				Model:           "openai/gpt-5.4",
				Harness:         compozyconfig.ProviderHarnessPiACP,
				RuntimeProvider: "openrouter",
				Transport:       "openai",
				AuthMode:        compozyconfig.ProviderAuthModeBoundSecret,
				CredentialSlots: []compozyconfig.ProviderCredentialSlot{{
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
		resolved := compozyconfig.ResolvedAgent{
			Provider:        "openrouter",
			Model:           "openai/gpt-5.4",
			Harness:         compozyconfig.ProviderHarnessPiACP,
			RuntimeProvider: "openrouter",
			AuthMode:        compozyconfig.ProviderAuthModeBoundSecret,
			CredentialSlots: []compozyconfig.ProviderCredentialSlot{
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
			resolved := compozyconfig.ResolvedAgent{
				Provider: "openrouter",
				Model:    "openai/gpt-5.4",
				Harness:  compozyconfig.ProviderHarnessACP,
				AuthMode: compozyconfig.ProviderAuthModeBoundSecret,
				CredentialSlots: []compozyconfig.ProviderCredentialSlot{{
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
			failure, failureMatched := errors.AsType[*acp.FailureError](err)
			if !failureMatched || failure == nil {
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
			resolved.Agents = []compozyconfig.AgentDef{
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
				if got := envValue(opts.Env, "COMPOZY_PROVIDER_AUTH_MODE"); got != "native_cli" {
					t.Fatalf("COMPOZY_PROVIDER_AUTH_MODE = %q, want native_cli", got)
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

			got := preferredACPModel(compozyconfig.ResolvedAgent{
				Provider:        test.provider,
				RuntimeProvider: test.provider,
				Harness:         compozyconfig.ProviderHarnessACP,
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
		resolved compozyconfig.ResolvedAgent
		ref      string
		slot     compozyconfig.ProviderCredentialSlot
		err      error
		want     bool
	}{
		{
			name:     "Should skip optional missing pi vault secret",
			resolved: compozyconfig.ResolvedAgent{Harness: compozyconfig.ProviderHarnessPiACP},
			ref:      "vault:providers/openrouter/api-key",
			slot:     compozyconfig.ProviderCredentialSlot{Required: false},
			err:      vault.ErrMissingSecret,
			want:     true,
		},
		{
			name:     "Should require missing pi vault secret when slot is required",
			resolved: compozyconfig.ResolvedAgent{Harness: compozyconfig.ProviderHarnessPiACP},
			ref:      "vault:providers/openrouter/api-key",
			slot:     compozyconfig.ProviderCredentialSlot{Required: true},
			err:      vault.ErrMissingSecret,
			want:     false,
		},
		{
			name:     "Should require missing env refs for direct acp providers when slot is required",
			resolved: compozyconfig.ResolvedAgent{Harness: compozyconfig.ProviderHarnessACP},
			ref:      "env:ANTHROPIC_API_KEY",
			slot:     compozyconfig.ProviderCredentialSlot{Required: true},
			err:      vault.ErrMissingSecret,
			want:     false,
		},
		{
			name:     "Should skip optional missing env refs for direct acp providers",
			resolved: compozyconfig.ResolvedAgent{Harness: compozyconfig.ProviderHarnessACP},
			ref:      "env:ANTHROPIC_API_KEY",
			slot:     compozyconfig.ProviderCredentialSlot{Required: false},
			err:      vault.ErrMissingSecret,
			want:     true,
		},
		{
			name:     "Should skip optional missing secret refs for direct acp providers",
			resolved: compozyconfig.ResolvedAgent{Harness: compozyconfig.ProviderHarnessACP},
			ref:      "vault:providers/custom/api-key",
			slot:     compozyconfig.ProviderCredentialSlot{Required: false},
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
	targetEnv string,
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
	wantRef := "$" + targetEnv
	if provider.APIKey != wantRef {
		t.Fatalf("models.json providers[%q].apiKey = %q, want %q", providerName, provider.APIKey, wantRef)
	}
	if got := envValue(opts.Env, targetEnv); got != wantSecret {
		t.Fatalf("child env %s = %q, want resolved provider secret", targetEnv, got)
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
