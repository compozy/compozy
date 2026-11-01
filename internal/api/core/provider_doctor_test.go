package core_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	apitestutil "github.com/compozy/compozy/internal/api/testutil"
	compozyconfig "github.com/compozy/compozy/internal/config"
	authproviders "github.com/compozy/compozy/internal/providers"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/gin-gonic/gin"
)

func TestDoctorProviderFilterIncludesProviderDiagnostics(t *testing.T) {
	t.Parallel()

	t.Run("Should include provider diagnostics when filtering doctor output", func(t *testing.T) {
		t.Parallel()

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
		handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
			Config:   cfg,
			Observer: apitestutil.StubObserver{},
			Sessions: apitestutil.StubSessionManager{},
		})
		router := gin.New()
		router.GET("/doctor", handlers.GetDoctor)

		response := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(
			testutil.Context(t),
			http.MethodGet,
			"/doctor?only=provider",
			http.NoBody,
		)
		router.ServeHTTP(response, req)

		if response.Code != http.StatusOK {
			t.Fatalf("status = %d body = %s, want 200", response.Code, response.Body.String())
		}
		var payload contract.DoctorPayload
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal(doctor) error = %v", err)
		}
		for _, item := range payload.Items {
			if item.Category != contract.CategoryProvider {
				t.Fatalf("Doctor item category = %q, want provider-only items", item.Category)
			}
			if item.ID == "doctor.provider.public" &&
				item.Message == authproviders.ProviderAuthNoAuthRequiredMessage {
				return
			}
		}
		t.Fatalf("Doctor items = %#v, want provider public no-auth diagnostic", payload.Items)
	})
}

// Invariant: doctor includes unresolved adapter startup failures even when auth is ready.
func TestDoctorNPXStartupFailure(t *testing.T) {
	t.Parallel()
	t.Run("Should show cached adapter recovery guidance", func(t *testing.T) {
		t.Parallel()
		var scans atomic.Int64
		handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
			Config: compozyconfig.DefaultWithHome(compozyconfig.HomePaths{}), Observer: apitestutil.StubObserver{},
			Sessions: apitestutil.StubSessionManager{ListAllFn: func(context.Context) ([]*session.Info, error) {
				scans.Add(1)
				return []*session.Info{{ID: "failed-start", State: session.StateStopped, Failure: &store.SessionFailure{
					Kind:       store.FailureHandshake,
					ReasonCode: store.FailureReasonNPXInitialize,
					Summary:    "ACP adapter initialize failed: npx launched but ACP initialize timed out.",
				}}, {ID: "recovered", State: session.StateActive, Failure: &store.SessionFailure{
					Kind: store.FailureHandshake, ReasonCode: store.FailureReasonNPXInitialize,
				}}}, nil
			}},
		})
		router := gin.New()
		router.GET("/doctor", handlers.GetDoctor)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet,
			"/doctor", http.NoBody))
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
		}
		if scans.Load() != 1 {
			t.Fatalf("session inventory scanned %d times", scans.Load())
		}
		var payload contract.DoctorPayload
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		for _, item := range payload.Items {
			if item.ID == "doctor.session.recovered.adapter" {
				t.Fatalf("recovered session reported: %#v", item)
			}
		}
		for _, item := range payload.Items {
			if item.ID == "doctor.session.failed-start.adapter" {
				if item.Code != contract.CodeProviderTransientFailure ||
					item.DataFreshness != contract.FreshnessStale || !strings.Contains(item.Message, "retry") {
					t.Fatalf("diagnostic = %#v", item)
				}
				return
			}
		}
		t.Fatalf("missing adapter diagnostic: %#v", payload.Items)
	})
}

// Invariant: session inventory failure cannot replace valid provider readiness diagnostics.
// Owner: doctor HTTP composition; canonical provider doctor suite.
func TestDoctorProviderSurvivesSessionInventoryFailure(t *testing.T) {
	t.Parallel()
	t.Run("Should retain provider diagnostics alongside the adapter probe error", func(t *testing.T) {
		t.Parallel()
		cfg := compozyconfig.DefaultWithHome(compozyconfig.HomePaths{})
		cfg.Providers = map[string]compozyconfig.ProviderConfig{"public": {
			Command: "public-provider acp", AuthMode: compozyconfig.ProviderAuthModeNone,
			NoneSecurity: compozyconfig.ProviderNoneSecurityPublicReadonly,
		}}
		handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
			Config: cfg, Observer: apitestutil.StubObserver{},
			Sessions: apitestutil.StubSessionManager{ListAllFn: func(context.Context) ([]*session.Info, error) {
				return nil, errors.New("session inventory unavailable")
			}},
		})
		router := gin.New()
		router.GET("/doctor", handlers.GetDoctor)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet,
			"/doctor?only=provider", http.NoBody))
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
		}
		var payload contract.DoctorPayload
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		var providerReady, inventoryFailed bool
		for _, item := range payload.Items {
			providerReady = providerReady || item.ID == "doctor.provider.public" &&
				item.Code == contract.CodeProviderAuthenticated
			inventoryFailed = inventoryFailed || item.Code == contract.CodeProbeFailed &&
				item.Severity == contract.SeverityError &&
				strings.Contains(item.Message, "session inventory unavailable")
		}
		if !providerReady || !inventoryFailed {
			t.Fatalf("missing independent diagnostics: %#v", payload.Items)
		}
	})
}
