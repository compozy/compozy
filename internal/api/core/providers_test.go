package core

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"
	authproviders "github.com/compozy/agh/internal/providers"
	"github.com/compozy/agh/internal/testutil"
	"github.com/gin-gonic/gin"
)

func TestProviderAuthHandlers(t *testing.T) {
	t.Parallel()

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
				Stderr:   "HTTP 401 unauthorized token=stderr-secret agh_claim_sensitive",
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
		for _, leaked := range []string{"stdout-secret", "stderr-secret", "agh_claim_sensitive"} {
			if strings.Contains(probeOutput, leaked) {
				t.Fatalf("probe output = %#v leaked %q", payload.Probe, leaked)
			}
		}
		if !strings.Contains(probeOutput, "[REDACTED]") {
			t.Fatalf("probe output = %#v, want redaction marker", payload.Probe)
		}
	})

	t.Run("Should distinguish missing providers from invalid provider configuration", func(t *testing.T) {
		t.Parallel()

		cfg := providerAuthTestConfig(t)
		cfg.Providers["bad"] = aghconfig.ProviderConfig{
			Command:  "bad-provider acp",
			AuthMode: aghconfig.ProviderAuthModeNone,
			CredentialSlots: []aghconfig.ProviderCredentialSlot{{
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
	cfg *aghconfig.Config,
	runner authproviders.ProviderAuthCommandRunner,
) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)
	if cfg == nil {
		cfg = &aghconfig.Config{}
	}
	handlers := NewBaseHandlers(&BaseHandlerConfig{
		Config:             *cfg,
		ProviderAuthRunner: runner,
	})
	router := gin.New()
	router.GET("/providers/:provider_id", handlers.GetProvider)
	router.POST("/providers/:provider_id/auth/probe", handlers.ProbeProviderAuth)
	return router
}

func providerAuthTestConfig(t *testing.T) aghconfig.Config {
	t.Helper()

	homePaths, err := aghconfig.ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	cfg := aghconfig.DefaultWithHome(homePaths)
	cfg.Providers["public"] = aghconfig.ProviderConfig{
		Command:      "public-provider acp",
		AuthMode:     aghconfig.ProviderAuthModeNone,
		NoneSecurity: aghconfig.ProviderNoneSecurityPublicReadonly,
	}
	cfg.Providers["native"] = aghconfig.ProviderConfig{
		Command:       "provider-cli acp",
		AuthMode:      aghconfig.ProviderAuthModeNativeCLI,
		AuthStatusCmd: "provider-cli auth status",
		AuthLoginCmd:  "provider-cli login",
	}
	return cfg
}
