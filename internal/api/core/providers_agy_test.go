package core

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	authproviders "github.com/compozy/compozy/internal/providers"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/gin-gonic/gin"
)

func agyProbeTestConfig(t *testing.T) compozyconfig.Config {
	t.Helper()

	cfg := providerAuthTestConfig(t)
	cfg.Providers["agy"] = compozyconfig.ProviderConfig{
		Command:       "agy_acp_server.par --uid=",
		AuthMode:      compozyconfig.ProviderAuthModeNativeCLI,
		AuthACPMethod: "oauth-personal",
	}
	cfg.Providers["plain"] = compozyconfig.ProviderConfig{
		Command:  "plain-provider acp",
		AuthMode: compozyconfig.ProviderAuthModeNativeCLI,
	}
	return cfg
}

func agyProbeTestRouter(
	t *testing.T,
	cfg *compozyconfig.Config,
	probe func(context.Context, acp.SessionInspectionRequest) error,
	resolver authproviders.ProviderAuthCommandResolver,
	invalidated *bool,
) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)
	if resolver == nil {
		resolver = func(_ context.Context, command string, _ []string, _ string) (string, error) {
			return "/test/bin/" + command, nil
		}
	}
	handlers := NewBaseHandlers(&BaseHandlerConfig{
		Config:                      *cfg,
		ProviderAuthCommandResolver: resolver,
		ProviderACPProbe:            probe,
		OnProviderAuthSuccess: func() {
			*invalidated = true
		},
	})
	router := gin.New()
	router.GET("/providers/:provider_id", handlers.GetProvider)
	router.POST("/providers/:provider_id/auth/probe", handlers.ProbeProviderAuth)
	return router
}

// Invariant: remote ACP probes share the local classification, omit the
// command-result object, and invalidate cached startup failures only on
// success. Owner: shared provider handlers (IT-005).
func TestProviderAGYACPProbe(t *testing.T) {
	t.Parallel()

	t.Run("Should report ACP readiness without a probe object", func(t *testing.T) {
		t.Parallel()

		cfg := agyProbeTestConfig(t)
		var seen acp.SessionInspectionRequest
		invalidated := false
		router := agyProbeTestRouter(t, &cfg, func(
			_ context.Context,
			req acp.SessionInspectionRequest,
		) error {
			seen = req
			return nil
		}, nil, &invalidated)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequestWithContext(
			testutil.Context(t), http.MethodPost, "/providers/agy/auth/probe", http.NoBody,
		))

		if response.Code != http.StatusOK {
			t.Fatalf("status = %d body = %s, want 200", response.Code, response.Body.String())
		}
		var payload contract.ProviderAuthProbeResponse
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal(provider auth probe) error = %v", err)
		}
		if got, want := payload.AuthStatus.State, contract.ProviderAuthStateAuthenticated; got != want {
			t.Fatalf("AuthStatus.State = %q, want %q", got, want)
		}
		if got, want := payload.AuthStatus.Message, "ACP session creation succeeded."; got != want {
			t.Fatalf("AuthStatus.Message = %q, want %q", got, want)
		}
		if payload.Probe != nil {
			t.Fatalf("Probe = %#v, want omitted probe for ACP operations", payload.Probe)
		}
		if got, want := payload.AuthStatus.Login.Source, contract.ProviderLoginSourceAuthACPMethod; got != want {
			t.Fatalf("Login.Source = %q, want %q", got, want)
		}
		if !invalidated {
			t.Fatal("successful ACP probe did not invalidate cached startup failures")
		}
		if !strings.HasPrefix(strings.TrimSpace(seen.Command), "agy_acp_server.par") {
			t.Fatalf("probe Command = %q, want effective builtin server command", seen.Command)
		}
		if !seenHasEnv(seen.Env, "COMPOZY_PROVIDER=agy") {
			t.Fatalf("probe Env = %#v, want daemon provider scope", seen.Env)
		}
		if got := strings.TrimSpace(seen.Cwd); got == "" {
			t.Fatal("probe Cwd is empty, want the daemon working directory")
		}
		if info, err := os.Stat(seen.Cwd); err != nil || !info.IsDir() {
			t.Fatalf("probe Cwd = %q, want an existing directory (%v)", seen.Cwd, err)
		}
		// The production-built request must pass the real ACP base
		// validation that rejected D-001 before launch.
		if err := (acp.StartOpts{
			AgentName: seen.AgentName,
			Command:   seen.Command,
			Cwd:       seen.Cwd,
			Env:       seen.Env,
		}).Validate(); err != nil {
			t.Fatalf("production probe request Validate() error = %v", err)
		}
	})

	t.Run("Should classify ACP auth refusal without invalidating the cache", func(t *testing.T) {
		t.Parallel()

		cfg := agyProbeTestConfig(t)
		invalidated := false
		router := agyProbeTestRouter(t, &cfg, func(
			context.Context,
			acp.SessionInspectionRequest,
		) error {
			return errors.New("ACP session/new failed: not authenticated")
		}, nil, &invalidated)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequestWithContext(
			testutil.Context(t), http.MethodPost, "/providers/agy/auth/probe", http.NoBody,
		))

		if response.Code != http.StatusOK {
			t.Fatalf("status = %d body = %s, want 200", response.Code, response.Body.String())
		}
		var payload contract.ProviderAuthProbeResponse
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal(provider auth probe) error = %v", err)
		}
		if got, want := payload.AuthStatus.State, contract.ProviderAuthStateNeedsLogin; got != want {
			t.Fatalf("AuthStatus.State = %q, want %q", got, want)
		}
		if got, want := payload.AuthStatus.Code, contract.CodeProviderNotAuthenticated; got != want {
			t.Fatalf("AuthStatus.Code = %q, want %q", got, want)
		}
		if payload.Probe != nil {
			t.Fatalf("Probe = %#v, want omitted probe for ACP operations", payload.Probe)
		}
		if invalidated {
			t.Fatal("failed ACP probe invalidated the pre-start cache")
		}
	})

	t.Run("Should pin a missing ACP executable without probing", func(t *testing.T) {
		t.Parallel()

		cfg := agyProbeTestConfig(t)
		invalidated := false
		router := agyProbeTestRouter(t, &cfg, func(
			context.Context,
			acp.SessionInspectionRequest,
		) error {
			t.Fatal("ProviderACPProbe called with missing executable")
			return nil
		}, func(context.Context, string, []string, string) (string, error) {
			return "", exec.ErrNotFound
		}, &invalidated)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequestWithContext(
			testutil.Context(t), http.MethodPost, "/providers/agy/auth/probe", http.NoBody,
		))

		if response.Code != http.StatusOK {
			t.Fatalf("status = %d body = %s, want 200", response.Code, response.Body.String())
		}
		var payload contract.ProviderAuthProbeResponse
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal(provider auth probe) error = %v", err)
		}
		if got, want := payload.AuthStatus.State, contract.ProviderAuthStateMissingCLI; got != want {
			t.Fatalf("AuthStatus.State = %q, want %q", got, want)
		}
		if payload.Probe != nil {
			t.Fatalf("Probe = %#v, want no probe after missing resolution", payload.Probe)
		}
		if invalidated {
			t.Fatal("missing executable invalidated the pre-start cache")
		}
	})

	t.Run("Should keep the unavailable-probe error without a probe path", func(t *testing.T) {
		t.Parallel()

		cfg := agyProbeTestConfig(t)
		invalidated := false
		router := agyProbeTestRouter(t, &cfg, func(
			context.Context,
			acp.SessionInspectionRequest,
		) error {
			t.Fatal("ProviderACPProbe called without a probe path")
			return nil
		}, nil, &invalidated)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequestWithContext(
			testutil.Context(t), http.MethodPost, "/providers/plain/auth/probe", http.NoBody,
		))

		if response.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d body = %s, want 422", response.Code, response.Body.String())
		}
		if invalidated {
			t.Fatal("unavailable probe invalidated the pre-start cache")
		}
	})

	t.Run("Should expose agy inventory with the method descriptor", func(t *testing.T) {
		t.Parallel()

		cfg := agyProbeTestConfig(t)
		delete(cfg.Providers, "agy")
		invalidated := false
		router := agyProbeTestRouter(t, &cfg, nil, nil, &invalidated)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequestWithContext(
			testutil.Context(t), http.MethodGet, "/providers/agy", http.NoBody,
		))

		if response.Code != http.StatusOK {
			t.Fatalf("status = %d body = %s, want 200", response.Code, response.Body.String())
		}
		var payload contract.ProviderSummaryPayload
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal(provider summary) error = %v", err)
		}
		if got, want := payload.DisplayName, "Google Antigravity"; got != want {
			t.Fatalf("DisplayName = %q, want %q", got, want)
		}
		if got, want := payload.AuthStatus.Login.Source, contract.ProviderLoginSourceAuthACPMethod; got != want {
			t.Fatalf("Login.Source = %q, want %q", got, want)
		}
		executable := "agy_acp_server.par"
		if runtime.GOOS == "windows" {
			executable = "agy_acp_server.exe"
		}
		if got, want := payload.AuthStatus.Login.Executable, executable; got != want {
			t.Fatalf("Login.Executable = %q, want %q", got, want)
		}
	})
}

func seenHasEnv(env []string, want string) bool {
	return slices.Contains(env, want)
}

// Invariant: the daemon probe env carries its working directory so the ACP
// probe request passes base validation and reaches the server. Owner: shared
// provider handlers (IT-005, D-001).
func TestProviderProbeEnvCarriesWorkingDirectory(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve the daemon working directory", func(t *testing.T) {
		t.Parallel()

		cfg := agyProbeTestConfig(t)
		handlers := NewBaseHandlers(&BaseHandlerConfig{Config: cfg})
		provider, err := cfg.ResolveProvider("agy")
		if err != nil {
			t.Fatalf("ResolveProvider(agy) error = %v", err)
		}
		env, err := handlers.providerProbeEnv("agy", provider)
		if err != nil {
			t.Fatalf("providerProbeEnv() error = %v", err)
		}
		if got := strings.TrimSpace(env.CommandDir); got == "" {
			t.Fatal("CommandDir is empty, want the daemon working directory")
		}
		if info, err := os.Stat(env.CommandDir); err != nil || !info.IsDir() {
			t.Fatalf("CommandDir = %q, want an existing directory (%v)", env.CommandDir, err)
		}
		if want, err := os.Getwd(); err != nil || env.CommandDir != want {
			t.Fatalf("CommandDir = %q, want daemon working directory %q (%v)", env.CommandDir, want, err)
		}
	})
}

func TestProviderHandlersWithoutWorkingDirectory(t *testing.T) {
	const helperEnv = "COMPOZY_TEST_PROVIDER_REMOVED_CWD"
	if os.Getenv(helperEnv) != "1" {
		t.Parallel()
		t.Run("Should keep provider endpoints available after the launch directory is removed", func(t *testing.T) {
			t.Parallel()
			if runtime.GOOS == "windows" {
				t.Skip("Windows does not allow removing the current working directory")
			}
			cmd := exec.CommandContext(
				t.Context(),
				os.Args[0],
				"-test.run=^TestProviderHandlersWithoutWorkingDirectory$",
			)
			cmd.Env = append(os.Environ(), helperEnv+"=1", "TMPDIR="+t.TempDir())
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("removed working directory subprocess: %v\n%s", err, output)
			}
		})
		return
	}

	t.Run("Should use the temporary directory for inventory and native and ACP probes", func(t *testing.T) {
		// not parallel: the isolated helper subprocess removes its own working directory.
		tempDir := os.TempDir()
		cfg := agyProbeTestConfig(t)
		handlers := NewBaseHandlers(&BaseHandlerConfig{
			Config: cfg,
			ProviderAuthCommandResolver: func(_ context.Context, command string, _ []string, dir string) (string, error) {
				if dir != tempDir {
					t.Fatalf("resolver dir = %q, want %q", dir, tempDir)
				}
				return "/test/bin/" + command, nil
			},
			ProviderAuthRunner: func(_ context.Context, spec authproviders.ProviderAuthCommandSpec) (authproviders.ProviderAuthCommandResult, error) {
				if spec.Dir != tempDir {
					t.Fatalf("native probe Dir = %q, want %q", spec.Dir, tempDir)
				}
				return authproviders.ProviderAuthCommandResult{Stdout: "logged in"}, nil
			},
			ProviderACPProbe: func(_ context.Context, req acp.SessionInspectionRequest) error {
				if req.Cwd != tempDir {
					t.Fatalf("ACP probe Cwd = %q, want %q", req.Cwd, tempDir)
				}
				return nil
			},
		})
		router := gin.New()
		router.GET("/providers", handlers.ListProviders)
		router.GET("/providers/:provider_id", handlers.GetProvider)
		router.POST("/providers/:provider_id/auth/probe", handlers.ProbeProviderAuth)

		workdir := t.TempDir()
		t.Chdir(workdir)
		if err := os.Remove(workdir); err != nil {
			t.Fatalf("remove working directory: %v", err)
		}
		if _, err := os.Stat(workdir); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("removed working directory still exists: %v", err)
		}
		for _, endpoint := range []struct{ method, path, body string }{
			{http.MethodGet, "/providers", `"providers":[`},
			{http.MethodGet, "/providers/public", `"name":"public"`},
			{http.MethodPost, "/providers/native/auth/probe", `"state":"authenticated"`},
			{http.MethodPost, "/providers/agy/auth/probe", `"state":"authenticated"`},
		} {
			t.Run("Should serve "+endpoint.method+" "+endpoint.path, func(t *testing.T) {
				response := httptest.NewRecorder()
				router.ServeHTTP(response, httptest.NewRequestWithContext(
					t.Context(), endpoint.method, endpoint.path, http.NoBody,
				))
				if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), endpoint.body) {
					t.Fatalf(
						"status = %d body = %s, want 200 containing %s",
						response.Code,
						response.Body.String(),
						endpoint.body,
					)
				}
			})
		}
	})
}
