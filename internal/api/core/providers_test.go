package core

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	authproviders "github.com/compozy/compozy/internal/providers"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/gin-gonic/gin"
)

func TestProviderAuthHandlers(t *testing.T) {
	t.Parallel()

	t.Run("Should signal same profile provider recovery after a successful auth probe", func(t *testing.T) {
		t.Parallel()
		cfg := providerAuthTestConfig(t)
		var signaled bool
		sessions := &providerAuthRecoverySessions{ready: func(profile, provider, fingerprint string) bool {
			signaled = true
			if profile != store.DefaultProfileID || provider != "native" ||
				fingerprint != authproviders.AuthenticationRouteFingerprint("native", cfg.Providers["native"]) {
				t.Errorf("unexpected recovery route: %s %s %s", profile, provider, fingerprint)
			}
			return true
		}}
		handlers := NewBaseHandlers(&BaseHandlerConfig{
			Config:                      cfg,
			Sessions:                    sessions,
			ProviderAuthCommandResolver: func(context.Context, string, []string, string) (string, error) { return "/test/bin/provider-cli", nil },
			ProviderAuthRunner: func(context.Context, authproviders.ProviderAuthCommandSpec) (authproviders.ProviderAuthCommandResult, error) {
				return authproviders.ProviderAuthCommandResult{Stdout: "logged in"}, nil
			},
		})
		router := gin.New()
		router.POST("/providers/:provider_id/auth/probe", handlers.ProbeProviderAuth)
		response := httptest.NewRecorder()
		router.ServeHTTP(
			response,
			httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/providers/native/auth/probe", http.NoBody),
		)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "authenticated") || !signaled {
			t.Fatalf("probe = %d %s signaled=%t", response.Code, response.Body.String(), signaled)
		}
	})

	t.Run("Should use the active provider generation for inventory auth and agent admission", func(t *testing.T) {
		t.Parallel()
		homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		settings := &activeProviderSettings{config: providerAuthTestConfig(t)}
		handlers := NewBaseHandlers(&BaseHandlerConfig{
			Config:    compozyconfig.CloneConfig(&settings.config),
			Settings:  settings,
			HomePaths: homePaths,
		})
		router := gin.New()
		router.GET("/providers", handlers.ListProviders)
		router.GET("/providers/:provider_id", handlers.GetProvider)
		router.POST("/providers/:provider_id/auth/probe", handlers.ProbeProviderAuth)
		settings.config.Providers["local-overlay"] = compozyconfig.ProviderConfig{
			Command:         "sh",
			RuntimeProvider: "claude",
			ReviewerMode:    "auto",
			AuthMode:        compozyconfig.ProviderAuthModeNone,
			NoneSecurity:    compozyconfig.ProviderNoneSecurityLocalTransport,
		}
		for _, endpoint := range []struct{ method, path string }{
			{http.MethodGet, "/providers"},
			{http.MethodGet, "/providers/local-overlay"},
			{http.MethodPost, "/providers/local-overlay/auth/probe"},
		} {
			t.Run("Should expose the applied overlay through "+endpoint.method+" "+endpoint.path, func(t *testing.T) {
				response := httptest.NewRecorder()
				router.ServeHTTP(
					response,
					httptest.NewRequestWithContext(t.Context(), endpoint.method, endpoint.path, http.NoBody),
				)
				if endpoint.path == "/providers" &&
					!strings.Contains(response.Body.String(), `"reviewer_mode":"auto"`) {
					t.Fatalf("reviewer metadata: %s", response.Body.String())
				}
				if endpoint.path == "/providers" {
					// FE-1: a provider without a reviewer still sends reviewer_mode, empty.
					var inventory struct {
						Providers []map[string]any `json:"providers"`
					}
					if err := json.Unmarshal(response.Body.Bytes(), &inventory); err != nil {
						t.Fatal(err)
					}
					found := false
					for _, provider := range inventory.Providers {
						if provider["name"] != "native" {
							continue
						}
						found = true
						if mode, ok := provider["reviewer_mode"]; !ok || mode != "" {
							t.Fatalf("native reviewer_mode = %v present=%v, want explicit empty", mode, ok)
						}
					}
					if !found {
						t.Fatalf("native provider missing: %s", response.Body.String())
					}
				}
				if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "local-overlay") {
					t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
				}
			})
		}
		_, _, _, cfg, err := handlers.createAgentDraftAndPath(t.Context(), contract.CreateAgentRequest{
			Scope: contract.AgentCreateScopeGlobal,
			Agent: contract.CreateAgentPayload{
				Name:     "writer",
				Provider: "local-overlay",
				Prompt:   "Write project outlines.",
			},
		})
		if err != nil || cfg.Providers["local-overlay"].RuntimeProvider != "claude" {
			t.Fatalf("agent admission config = %#v, error = %v", cfg.Providers, err)
		}
		delete(settings.config.Providers, "local-overlay")
		response := httptest.NewRecorder()
		router.ServeHTTP(
			response,
			httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/providers/local-overlay", http.NoBody),
		)
		if response.Code != http.StatusNotFound || !strings.Contains(response.Body.String(), "provider_not_installed") {
			t.Fatalf("removed provider status = %d, body = %s", response.Code, response.Body.String())
		}
		settings.err = errors.New("active generation unavailable")
		response = httptest.NewRecorder()
		router.ServeHTTP(
			response,
			httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/providers", http.NoBody),
		)
		if response.Code != http.StatusInternalServerError ||
			!strings.Contains(response.Body.String(), settings.err.Error()) {
			t.Fatalf("unavailable generation status = %d, body = %s", response.Code, response.Body.String())
		}
	})

	t.Run("Should discard cached pre-start failure after successful reauthentication", func(t *testing.T) {
		t.Parallel()

		cfg := providerAuthTestConfig(t)
		provider := cfg.Providers["native"]
		starter := authproviders.NewPreStarter()
		authenticated := false
		calls := 0
		runner := func(context.Context, authproviders.ProviderAuthCommandSpec) (authproviders.ProviderAuthCommandResult, error) {
			calls++
			if authenticated {
				return authproviders.ProviderAuthCommandResult{Stdout: "logged in"}, nil
			}
			return authproviders.ProviderAuthCommandResult{ExitCode: 1, Stderr: "not logged in"}, nil
		}
		resolver := func(context.Context, string, []string, string) (string, error) {
			return "/test/bin/provider-cli", nil
		}
		env := &authproviders.ProbeEnv{
			ProviderName: "native",
			PreStartScope: authproviders.PreStartScope{
				WorkspaceID: "workspace", ProfileID: "profile", HomeIdentity: t.TempDir(),
			},
			LookPath:       func(string) (string, error) { return "/test/bin/provider-cli", nil },
			ResolveCommand: resolver,
			RunCommand:     runner,
		}
		for range 2 {
			report := starter.PreStart(t.Context(), provider, env)
			if report.Cause != nil || report.Item == nil || report.Item.Code != contract.CodeProviderNotAuthenticated {
				t.Fatalf("pre-start report = %#v, want cached authentication failure", report)
			}
		}
		if calls != 1 {
			t.Fatalf("initial probe calls = %d, want 1", calls)
		}
		authenticated = true
		handlers := NewBaseHandlers(&BaseHandlerConfig{
			Config: cfg, ProviderAuthRunner: runner, ProviderAuthCommandResolver: resolver,
			OnProviderAuthSuccess: starter.Clear,
		})
		router := gin.New()
		router.POST("/providers/:provider_id/auth/probe", handlers.ProbeProviderAuth)
		response := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			"/providers/native/auth/probe",
			http.NoBody,
		)
		router.ServeHTTP(response, request)
		var payload contract.ProviderAuthProbeResponse
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode probe response: %v", err)
		}
		if response.Code != http.StatusOK || payload.AuthStatus.State != contract.ProviderAuthStateAuthenticated {
			t.Fatalf("probe status = %d, body = %s", response.Code, response.Body.String())
		}
		if report := starter.PreStart(t.Context(), provider, env); report.Cause != nil || report.Item != nil {
			t.Fatalf("pre-start after reauthentication = %#v, want success", report)
		}
		if calls != 3 {
			t.Fatalf("probe calls = %d, want original failure, live auth and refreshed pre-start", calls)
		}
	})

	t.Run("Should report the last live sign-in verdict on provider reads", func(t *testing.T) {
		t.Parallel()

		verdicts := authproviders.NewAuthVerdicts()
		var resolverErr error
		cfg := providerAuthTestConfig(t)
		handlers := NewBaseHandlers(&BaseHandlerConfig{
			Config: cfg,
			ProviderAuthCommandResolver: func(context.Context, string, []string, string) (string, error) {
				if resolverErr != nil {
					return "", resolverErr
				}
				return "/test/bin/provider-cli", nil
			},
			ProviderAuthRunner: func(context.Context, authproviders.ProviderAuthCommandSpec) (authproviders.ProviderAuthCommandResult, error) {
				return authproviders.ProviderAuthCommandResult{Stdout: "logged in"}, nil
			},
			ProviderAuthVerdicts: verdicts,
		})
		router := gin.New()
		router.GET("/providers/:provider_id", handlers.GetProvider)
		router.POST("/providers/:provider_id/auth/probe", handlers.ProbeProviderAuth)
		read := func() contract.ProviderSummaryPayload {
			t.Helper()
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequestWithContext(
				t.Context(), http.MethodGet, "/providers/native", http.NoBody,
			))
			var payload contract.ProviderSummaryPayload
			if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &payload) != nil {
				t.Fatalf("get provider status = %d, body = %s", response.Code, response.Body.String())
			}
			return payload
		}
		if before := read(); before.AuthStatus.State != contract.ProviderAuthStateUnknown ||
			before.AuthStatus.LastProbeAt != nil {
			t.Fatalf("auth status before probe = %#v, want unverified", before.AuthStatus)
		}
		native, err := cfg.ResolveProvider("native")
		if err != nil {
			t.Fatalf("ResolveProvider(native) error = %v", err)
		}
		verdicts.Record("native", native, []string{"HOME=/session-seat"}, authproviders.Classification{
			State: authproviders.ProviderAuthStateAuthenticated,
		}, time.Now())
		if foreign := read(); foreign.AuthStatus.State != contract.ProviderAuthStateUnknown ||
			foreign.AuthStatus.LastProbeAt != nil {
			t.Fatalf("auth status for another auth home = %#v, want unverified", foreign.AuthStatus)
		}

		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequestWithContext(
			t.Context(), http.MethodPost, "/providers/native/auth/probe", http.NoBody,
		))
		if response.Code != http.StatusOK {
			t.Fatalf("probe status = %d, body = %s", response.Code, response.Body.String())
		}

		after := read()
		if after.AuthStatus.State != contract.ProviderAuthStateAuthenticated || after.AuthStatus.LastProbeAt == nil {
			t.Fatalf("auth status after probe = %#v, want authenticated with a check time", after.AuthStatus)
		}

		resolverErr = errors.New("open /test/bin/provider-cli: permission denied")
		failed := read()
		if failed.AuthStatus.State != contract.ProviderAuthStateUnknown || failed.AuthStatus.LastProbeAt != nil ||
			!strings.Contains(failed.AuthStatus.Message, "permission denied") {
			t.Fatalf(
				"auth status after resolver failure = %#v, want the declared error without a check time",
				failed.AuthStatus,
			)
		}
	})

	for _, tc := range []struct {
		name        string
		cancelProbe bool
	}{
		{name: "Should retain pre-start verdicts after a failed auth probe"},
		{name: "Should retain pre-start verdicts after a canceled auth probe", cancelProbe: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			invalidated := false
			handlers := NewBaseHandlers(&BaseHandlerConfig{
				Config: providerAuthTestConfig(t),
				ProviderAuthCommandResolver: func(context.Context, string, []string, string) (string, error) {
					return "/test/bin/provider-cli", nil
				},
				ProviderAuthRunner: func(context.Context, authproviders.ProviderAuthCommandSpec) (authproviders.ProviderAuthCommandResult, error) {
					if tc.cancelProbe {
						cancel()
						return authproviders.ProviderAuthCommandResult{Stdout: "logged in"}, nil
					}
					return authproviders.ProviderAuthCommandResult{ExitCode: 1, Stderr: "not logged in"}, nil
				},
				OnProviderAuthSuccess: func() { invalidated = true },
			})
			router := gin.New()
			router.POST("/providers/:provider_id/auth/probe", handlers.ProbeProviderAuth)
			response := httptest.NewRecorder()
			request := httptest.NewRequestWithContext(ctx, http.MethodPost, "/providers/native/auth/probe", http.NoBody)
			router.ServeHTTP(response, request)
			var payload contract.ProviderAuthProbeResponse
			if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode probe response: %v", err)
			}
			if response.Code != http.StatusOK || payload.Provider != "native" {
				t.Fatalf("probe status = %d, body = %s", response.Code, response.Body.String())
			}
			if invalidated {
				t.Fatal("failed or canceled probe invalidated the pre-start cache")
			}
		})
	}

	t.Run("Should report explicit no auth provider state", func(t *testing.T) {
		t.Parallel()

		cfg := providerAuthTestConfig(t)
		router := providerAuthTestRouter(t, &cfg, nil)
		response := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(testutil.Context(t), http.MethodGet, "/providers/public", http.NoBody)
		router.ServeHTTP(response, req)

		if response.Code != http.StatusOK {
			t.Fatalf("status = %d body = %s, want 200", response.Code, response.Body.String())
		}
		var payload contract.ProviderSummaryPayload
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal(provider summary) error = %v", err)
		}
		if got, want := payload.AuthStatus.State, contract.ProviderAuthStateNone; got != want {
			t.Fatalf("AuthStatus.State = %q, want %q", got, want)
		}
		if got, want := payload.AuthStatus.Message, authproviders.ProviderAuthNoAuthRequiredMessage; got != want {
			t.Fatalf("AuthStatus.Message = %q, want %q", got, want)
		}
		if got, want := payload.RuntimeStrategy, contract.ProviderRuntimeStrategySessionConfig; got != want {
			t.Fatalf("RuntimeStrategy = %q, want %q", got, want)
		}
	})

	t.Run("Should classify remote probe auth failure", func(t *testing.T) {
		t.Parallel()

		runner := func(
			_ context.Context,
			spec authproviders.ProviderAuthCommandSpec,
		) (authproviders.ProviderAuthCommandResult, error) {
			if spec.Command != "provider-cli auth status" {
				t.Fatalf("Command = %q, want provider status command", spec.Command)
			}
			if !spec.NoTTY {
				t.Fatal("NoTTY = false, want daemon probe to be non-interactive")
			}
			return authproviders.ProviderAuthCommandResult{
				ExitCode: 1,
				Stdout:   `{"loggedIn":false,"email":"private@example.test","orgId":"private-org-id","orgName":"Private Organization","access_token":"stdout-secret"}`,
				Stderr:   "HTTP 401 unauthorized token=stderr-secret compozy_claim_sensitive",
			}, nil
		}
		cfg := providerAuthTestConfig(t)
		router := providerAuthTestRouter(t, &cfg, runner)
		response := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(
			testutil.Context(t),
			http.MethodPost,
			"/providers/native/auth/probe",
			http.NoBody,
		)
		router.ServeHTTP(response, req)

		if response.Code != http.StatusOK {
			t.Fatalf("status = %d body = %s, want 200", response.Code, response.Body.String())
		}
		var payload contract.ProviderAuthProbeResponse
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal(provider probe) error = %v", err)
		}
		if got, want := payload.AuthStatus.State, contract.ProviderAuthStateNeedsLogin; got != want {
			t.Fatalf("AuthStatus.State = %q, want %q", got, want)
		}
		if got, want := payload.AuthStatus.Code, contract.CodeProviderNotAuthenticated; got != want {
			t.Fatalf("AuthStatus.Code = %q, want %q", got, want)
		}
		if payload.Probe == nil {
			t.Fatal("Probe = nil, want redacted probe output")
		}
		probeOutput := payload.Probe.Stdout + payload.Probe.Stderr
		for _, leaked := range []string{"stdout-secret", "stderr-secret", "compozy_claim_sensitive", "private@example.test", "private-org-id", "Private Organization"} {
			if strings.Contains(probeOutput, leaked) {
				t.Fatalf("probe output = %#v leaked %q", payload.Probe, leaked)
			}
		}
		if !strings.Contains(probeOutput, "[REDACTED]") {
			t.Fatalf("probe output = %#v, want redaction marker", payload.Probe)
		}
	})

	t.Run("Should resolve the remote probe command once with its final runtime", func(t *testing.T) {
		t.Parallel()

		cfg := providerAuthTestConfig(t)
		wantDir, err := os.Getwd()
		if err != nil {
			t.Fatalf("os.Getwd() error = %v", err)
		}
		var resolveCalls int
		var resolvedEnv []string
		resolver := func(_ context.Context, command string, env []string, dir string) (string, error) {
			resolveCalls++
			if command != "provider-cli" {
				t.Fatalf("resolver command = %q, want provider-cli", command)
			}
			if dir != wantDir {
				t.Fatalf("resolver dir = %q, want daemon working directory %q", dir, wantDir)
			}
			resolvedEnv = append([]string(nil), env...)
			if resolveCalls > 1 {
				return "", errors.New("second auth status resolution must not occur")
			}
			return "/final/bin/provider-cli", nil
		}
		runner := func(
			_ context.Context,
			spec authproviders.ProviderAuthCommandSpec,
		) (authproviders.ProviderAuthCommandResult, error) {
			if got, want := spec.Executable, "/final/bin/provider-cli"; got != want {
				t.Fatalf("Executable = %q, want %q", got, want)
			}
			if !slices.Equal(spec.Args, []string{"auth", "status"}) {
				t.Fatalf("Args = %#v, want auth status", spec.Args)
			}
			if spec.Dir != wantDir {
				t.Fatalf("Dir = %q, want daemon working directory %q", spec.Dir, wantDir)
			}
			if !slices.Equal(spec.Env, resolvedEnv) {
				t.Fatalf("Env = %#v, want resolver env %#v", spec.Env, resolvedEnv)
			}
			return authproviders.ProviderAuthCommandResult{ExitCode: 0, Stdout: "logged in"}, nil
		}
		router := providerAuthTestRouterWithResolver(t, &cfg, runner, resolver)
		response := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(
			testutil.Context(t),
			http.MethodPost,
			"/providers/native/auth/probe",
			http.NoBody,
		)
		router.ServeHTTP(response, req)

		if response.Code != http.StatusOK {
			t.Fatalf("status = %d body = %s, want 200", response.Code, response.Body.String())
		}
		if got, want := resolveCalls, 1; got != want {
			t.Fatalf("auth status resolver calls = %d, want %d", got, want)
		}
		var payload contract.ProviderAuthProbeResponse
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal(provider auth probe) error = %v", err)
		}
		if got, want := payload.AuthStatus.State, contract.ProviderAuthStateAuthenticated; got != want {
			t.Fatalf("AuthStatus.State = %q, want %q", got, want)
		}
		if payload.Probe == nil || payload.Probe.Stdout != "logged in" {
			t.Fatalf("Probe = %#v, want final command result", payload.Probe)
		}
	})

	t.Run("Should pin a missing remote probe command without running it", func(t *testing.T) {
		t.Parallel()

		cfg := providerAuthTestConfig(t)
		resolveCalls := 0
		resolver := func(context.Context, string, []string, string) (string, error) {
			resolveCalls++
			if resolveCalls == 1 {
				return "", exec.ErrNotFound
			}
			return "/later/bin/provider-cli", nil
		}
		runner := func(
			_ context.Context,
			spec authproviders.ProviderAuthCommandSpec,
		) (authproviders.ProviderAuthCommandResult, error) {
			t.Fatalf("ProviderAuthRunner(%q) called after missing resolution", spec.Command)
			return authproviders.ProviderAuthCommandResult{}, nil
		}
		router := providerAuthTestRouterWithResolver(t, &cfg, runner, resolver)
		response := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(
			testutil.Context(t),
			http.MethodPost,
			"/providers/native/auth/probe",
			http.NoBody,
		)
		router.ServeHTTP(response, req)

		if response.Code != http.StatusOK {
			t.Fatalf("status = %d body = %s, want 200", response.Code, response.Body.String())
		}
		if got, want := resolveCalls, 1; got != want {
			t.Fatalf("auth status resolver calls = %d, want %d", got, want)
		}
		var payload contract.ProviderAuthProbeResponse
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal(provider auth probe) error = %v", err)
		}
		if got, want := payload.AuthStatus.State, contract.ProviderAuthStateMissingCLI; got != want {
			t.Fatalf("AuthStatus.State = %q, want %q", got, want)
		}
		if got, want := payload.AuthStatus.Code, contract.CodeProviderCLIMissing; got != want {
			t.Fatalf("AuthStatus.Code = %q, want %q", got, want)
		}
		if payload.Probe != nil {
			t.Fatalf("Probe = %#v, want no probe after missing resolution", payload.Probe)
		}
	})

	t.Run("Should keep an operational remote probe resolution failure unknown", func(t *testing.T) {
		t.Parallel()

		cfg := providerAuthTestConfig(t)
		resolveCalls := 0
		resolver := func(context.Context, string, []string, string) (string, error) {
			resolveCalls++
			if resolveCalls == 1 {
				return "", errors.New("resolver failed token=api-resolution-secret")
			}
			return "/later/bin/provider-cli", nil
		}
		runner := func(
			_ context.Context,
			spec authproviders.ProviderAuthCommandSpec,
		) (authproviders.ProviderAuthCommandResult, error) {
			t.Fatalf("ProviderAuthRunner(%q) called after resolver failure", spec.Command)
			return authproviders.ProviderAuthCommandResult{}, nil
		}
		router := providerAuthTestRouterWithResolver(t, &cfg, runner, resolver)
		response := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(
			testutil.Context(t),
			http.MethodPost,
			"/providers/native/auth/probe",
			http.NoBody,
		)
		router.ServeHTTP(response, req)

		if response.Code != http.StatusOK {
			t.Fatalf("status = %d body = %s, want 200", response.Code, response.Body.String())
		}
		if got, want := resolveCalls, 1; got != want {
			t.Fatalf("auth status resolver calls = %d, want %d", got, want)
		}
		var payload contract.ProviderAuthProbeResponse
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal(provider auth probe) error = %v", err)
		}
		if got, want := payload.AuthStatus.State, contract.ProviderAuthStateUnknown; got != want {
			t.Fatalf("AuthStatus.State = %q, want %q", got, want)
		}
		if got, want := payload.AuthStatus.Code, contract.CodeProviderClassificationUnknown; got != want {
			t.Fatalf("AuthStatus.Code = %q, want %q", got, want)
		}
		if payload.Probe != nil {
			t.Fatalf("Probe = %#v, want no probe after resolver failure", payload.Probe)
		}
		if strings.Contains(response.Body.String(), "api-resolution-secret") {
			t.Fatalf("body = %s, leaked resolver secret", response.Body.String())
		}
		if !strings.Contains(payload.AuthStatus.Message, "[REDACTED]") {
			t.Fatalf("AuthStatus.Message = %q, want redaction marker", payload.AuthStatus.Message)
		}
	})

	t.Run("Should distinguish missing providers from invalid provider configuration", func(t *testing.T) {
		t.Parallel()

		cfg := providerAuthTestConfig(t)
		cfg.Providers["bad"] = compozyconfig.ProviderConfig{
			Command:  "bad-provider acp",
			AuthMode: compozyconfig.ProviderAuthModeNone,
			CredentialSlots: []compozyconfig.ProviderCredentialSlot{{
				Name:      "api",
				TargetEnv: "BAD_API_KEY",
				SecretRef: "env:BAD_API_KEY",
				Required:  true,
			}},
		}
		router := providerAuthTestRouter(t, &cfg, nil)

		missing := httptest.NewRecorder()
		missingReq := httptest.NewRequestWithContext(
			testutil.Context(t),
			http.MethodGet,
			"/providers/missing",
			http.NoBody,
		)
		router.ServeHTTP(missing, missingReq)
		if missing.Code != http.StatusNotFound {
			t.Fatalf("missing provider status = %d body = %s, want 404", missing.Code, missing.Body.String())
		}

		invalid := httptest.NewRecorder()
		invalidReq := httptest.NewRequestWithContext(
			testutil.Context(t),
			http.MethodGet,
			"/providers/bad",
			http.NoBody,
		)
		router.ServeHTTP(invalid, invalidReq)
		if invalid.Code != http.StatusInternalServerError {
			t.Fatalf("invalid provider status = %d body = %s, want 500", invalid.Code, invalid.Body.String())
		}
	})

	t.Run("Should report no auth required for remote none probe", func(t *testing.T) {
		t.Parallel()

		runner := func(
			_ context.Context,
			spec authproviders.ProviderAuthCommandSpec,
		) (authproviders.ProviderAuthCommandResult, error) {
			t.Fatalf("ProviderAuthRunner(%q) called, want no subprocess for auth_mode none", spec.Command)
			return authproviders.ProviderAuthCommandResult{}, nil
		}
		cfg := providerAuthTestConfig(t)
		router := providerAuthTestRouter(t, &cfg, runner)
		response := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(
			testutil.Context(t),
			http.MethodPost,
			"/providers/public/auth/probe",
			http.NoBody,
		)
		router.ServeHTTP(response, req)

		if response.Code != http.StatusOK {
			t.Fatalf("status = %d body = %s, want 200", response.Code, response.Body.String())
		}
		var payload contract.ProviderAuthProbeResponse
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal(provider none probe) error = %v", err)
		}
		if got, want := payload.AuthStatus.State, contract.ProviderAuthStateNone; got != want {
			t.Fatalf("AuthStatus.State = %q, want %q", got, want)
		}
		if got, want := payload.AuthStatus.Message, authproviders.ProviderAuthNoAuthRequiredMessage; got != want {
			t.Fatalf("AuthStatus.Message = %q, want %q", got, want)
		}
		if payload.Probe != nil {
			t.Fatalf("Probe = %#v, want nil for auth_mode none", payload.Probe)
		}
	})
}

func TestDiagnosticStatus(t *testing.T) {
	t.Parallel()

	t.Run("Should keep informational diagnostics from changing aggregate status", func(t *testing.T) {
		t.Parallel()

		got := diagnosticStatus([]contract.DiagnosticItem{{
			ID:       "doctor.provider.status",
			Code:     "provider.info",
			Category: contract.CategoryProvider,
			Severity: contract.SeverityInfo,
		}})
		if got != statusStateOK {
			t.Fatalf("diagnosticStatus(info) = %q, want %q", got, statusStateOK)
		}
	})
}

func providerAuthTestRouter(
	t *testing.T,
	cfg *compozyconfig.Config,
	runner authproviders.ProviderAuthCommandRunner,
) *gin.Engine {
	return providerAuthTestRouterWithResolver(t, cfg, runner, nil)
}

func providerAuthTestRouterWithResolver(
	t *testing.T,
	cfg *compozyconfig.Config,
	runner authproviders.ProviderAuthCommandRunner,
	resolver authproviders.ProviderAuthCommandResolver,
) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)
	if cfg == nil {
		cfg = &compozyconfig.Config{}
	}
	if resolver == nil {
		resolver = func(_ context.Context, command string, _ []string, _ string) (string, error) {
			return "/test/bin/" + command, nil
		}
	}
	handlers := NewBaseHandlers(&BaseHandlerConfig{
		Config:                      *cfg,
		ProviderAuthRunner:          runner,
		ProviderAuthCommandResolver: resolver,
	})
	router := gin.New()
	router.GET("/providers/:provider_id", handlers.GetProvider)
	router.POST("/providers/:provider_id/auth/probe", handlers.ProbeProviderAuth)
	return router
}

func providerAuthTestConfig(t *testing.T) compozyconfig.Config {
	t.Helper()

	homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	cfg := compozyconfig.DefaultWithHome(homePaths)
	cfg.Providers["public"] = compozyconfig.ProviderConfig{
		Command:      "public-provider acp",
		AuthMode:     compozyconfig.ProviderAuthModeNone,
		NoneSecurity: compozyconfig.ProviderNoneSecurityPublicReadonly,
	}
	cfg.Providers["native"] = compozyconfig.ProviderConfig{
		Command:       "provider-cli acp",
		AuthMode:      compozyconfig.ProviderAuthModeNativeCLI,
		AuthStatusCmd: "provider-cli auth status",
		AuthLoginCmd:  "provider-cli login",
	}
	return cfg
}

type activeProviderSettings struct {
	SettingsService
	config compozyconfig.Config
	err    error
}

func (s *activeProviderSettings) ActiveConfig(context.Context) (compozyconfig.Config, error) {
	return compozyconfig.CloneConfig(&s.config), s.err
}

func TestProviderInventoryEnabled(t *testing.T) {
	t.Parallel()

	t.Run("Should list a disabled provider with enabled false next to enabled ones", func(t *testing.T) {
		t.Parallel()
		homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		settings := &activeProviderSettings{config: providerAuthTestConfig(t)}
		native := settings.config.Providers["native"]
		native.Enabled = new(false)
		settings.config.Providers["native"] = native
		handlers := NewBaseHandlers(&BaseHandlerConfig{
			Config:    compozyconfig.CloneConfig(&settings.config),
			Settings:  settings,
			HomePaths: homePaths,
		})
		router := gin.New()
		router.GET("/providers", handlers.ListProviders)
		response := httptest.NewRecorder()
		router.ServeHTTP(
			response,
			httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/providers", http.NoBody),
		)
		var inventory contract.ProviderListResponse
		if err := json.Unmarshal(response.Body.Bytes(), &inventory); err != nil {
			t.Fatalf("decode inventory: %v; body=%s", err, response.Body.String())
		}
		enabled := map[string]bool{}
		for _, provider := range inventory.Providers {
			enabled[provider.Name] = provider.Enabled
		}
		if got, listed := enabled["native"]; !listed || got {
			t.Fatalf("native enabled = %v listed = %v, want listed and disabled", got, listed)
		}
		if !enabled["public"] {
			t.Fatalf("public enabled = false, want true; inventory=%v", enabled)
		}
	})

	for _, tc := range []struct {
		name          string
		globalEnabled *bool
		profileTOML   string
		want          bool
	}{
		{
			name:          "Should report a profile re-enable over a global disable",
			globalEnabled: new(false),
			profileTOML:   "[providers.native]\nenabled = true\n",
			want:          true,
		},
		{
			name:        "Should report a profile disable over an enabled global",
			profileTOML: "[providers.native]\nenabled = false\n",
			want:        false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			profileDir := filepath.Join(homePaths.ProfilesDir, compozyconfig.DefaultProfileDirName)
			if err := os.MkdirAll(profileDir, 0o700); err != nil {
				t.Fatalf("mkdir profile: %v", err)
			}
			profileFile := filepath.Join(profileDir, compozyconfig.ConfigName)
			if err := os.WriteFile(profileFile, []byte(tc.profileTOML), 0o600); err != nil {
				t.Fatalf("write profile config: %v", err)
			}
			settings := &activeProviderSettings{config: providerAuthTestConfig(t)}
			native := settings.config.Providers["native"]
			native.Enabled = tc.globalEnabled
			settings.config.Providers["native"] = native
			handlers := NewBaseHandlers(&BaseHandlerConfig{
				Config:    compozyconfig.CloneConfig(&settings.config),
				Settings:  settings,
				HomePaths: homePaths,
			})
			router := gin.New()
			router.GET("/providers", handlers.ListProviders)
			router.GET("/providers/:provider_id", handlers.GetProvider)

			var inventory contract.ProviderListResponse
			serveProviderJSON(t, router, "/providers", &inventory)
			var listed *contract.ProviderSummaryPayload
			for i := range inventory.Providers {
				if inventory.Providers[i].Name == "native" {
					listed = &inventory.Providers[i]
				}
			}
			if listed == nil || listed.Enabled != tc.want {
				t.Fatalf("list native = %+v, want enabled %v", listed, tc.want)
			}
			var single contract.ProviderSummaryPayload
			serveProviderJSON(t, router, "/providers/native", &single)
			if single.Enabled != tc.want {
				t.Fatalf("get native enabled = %v, want %v", single.Enabled, tc.want)
			}
		})
	}
}

func serveProviderJSON(t *testing.T, router http.Handler, path string, out any) {
	t.Helper()
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, http.NoBody))
	if response.Code != http.StatusOK {
		t.Fatalf("GET %s status = %d; body=%s", path, response.Code, response.Body.String())
	}
	if err := json.Unmarshal(response.Body.Bytes(), out); err != nil {
		t.Fatalf("decode %s: %v; body=%s", path, err, response.Body.String())
	}
}

type providerAuthRecoverySessions struct {
	sessionManagerStub
	ready func(string, string, string) bool
}

func (s *providerAuthRecoverySessions) ProviderAuthenticationReady(profile, provider, fingerprint string) bool {
	return s.ready(profile, provider, fingerprint)
}
