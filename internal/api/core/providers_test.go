package core

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	authproviders "github.com/compozy/compozy/internal/providers"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/gin-gonic/gin"
)

func TestProviderAuthHandlers(t *testing.T) {
	t.Parallel()

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
				SandboxID: "sandbox", SandboxBackend: "local", SandboxProfile: "default",
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

	t.Run("Should expose provider-managed runtime strategy", func(t *testing.T) {
		t.Parallel()

		cfg := providerAuthTestConfig(t)
		router := providerAuthTestRouter(t, &cfg, nil)
		response := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(
			testutil.Context(t),
			http.MethodGet,
			"/providers/openclaw",
			http.NoBody,
		)
		router.ServeHTTP(response, req)

		if response.Code != http.StatusOK {
			t.Fatalf("status = %d body = %s, want 200", response.Code, response.Body.String())
		}
		var payload contract.ProviderSummaryPayload
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal(provider summary) error = %v", err)
		}
		if got, want := payload.RuntimeStrategy, contract.ProviderRuntimeStrategyProviderManaged; got != want {
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
				Stdout:   "access_token=stdout-secret",
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
		for _, leaked := range []string{"stdout-secret", "stderr-secret", "compozy_claim_sensitive"} {
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
		var resolveCalls int
		var resolvedEnv []string
		resolver := func(_ context.Context, command string, env []string, dir string) (string, error) {
			resolveCalls++
			if command != "provider-cli" {
				t.Fatalf("resolver command = %q, want provider-cli", command)
			}
			if dir != "" {
				t.Fatalf("resolver dir = %q, want empty", dir)
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
			if spec.Dir != "" {
				t.Fatalf("Dir = %q, want empty", spec.Dir)
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
			ID:       "doctor.network.status",
			Code:     contract.CodeNetworkDisabled,
			Category: contract.CategoryNetwork,
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
