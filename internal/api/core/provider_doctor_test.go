package core_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/compozy/agh/internal/api/contract"
	core "github.com/compozy/agh/internal/api/core"
	apitestutil "github.com/compozy/agh/internal/api/testutil"
	aghconfig "github.com/compozy/agh/internal/config"
	authproviders "github.com/compozy/agh/internal/providers"
	"github.com/compozy/agh/internal/testutil"
	"github.com/gin-gonic/gin"
)

func TestDoctorProviderFilterIncludesProviderDiagnostics(t *testing.T) {
	t.Parallel()

	t.Run("Should include provider diagnostics when filtering doctor output", func(t *testing.T) {
		t.Parallel()

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
