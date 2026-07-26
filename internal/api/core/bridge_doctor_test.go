// Suite: bridge doctor API integration
// Invariant: the public doctor filter delegates to provider checks and returns only bridge diagnostics.
// Boundary IN: BaseHandlers doctor registration and filtering.
// Boundary OUT: provider check semantics, owned by adapter control suites.
package core_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/api/contract"
	core "github.com/compozy/agh/internal/api/core"
	apitestutil "github.com/compozy/agh/internal/api/testutil"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/testutil"
	"github.com/gin-gonic/gin"
)

func TestDoctorBridgeFilterRunsProviderVerification(t *testing.T) {
	t.Run("Should return only bridge diagnostics with secret-slot remediation", func(t *testing.T) {
		t.Parallel()

		homePaths, err := aghconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		bridgeChecks := 0
		bridges := apitestutil.StubBridgeService{
			ListInstancesFn: func(context.Context) ([]bridgepkg.BridgeInstance, error) {
				return []bridgepkg.BridgeInstance{{
					ID:            "brg-slack",
					Platform:      "slack",
					ExtensionName: "slack",
				}}, nil
			},
			CheckBridgeFn: func(
				_ context.Context,
				extensionName string,
				request bridgepkg.BridgeCheckRequest,
			) (bridgepkg.BridgeCheckResponse, error) {
				bridgeChecks++
				if extensionName != "slack" || request.BridgeInstanceID != "brg-slack" {
					t.Fatalf("CheckBridge() extension=%q request=%#v", extensionName, request)
				}
				return bridgepkg.BridgeCheckResponse{Checks: []bridgepkg.BridgeCheckRecord{{
					Check:       "provider.identity",
					Status:      bridgepkg.BridgeCheckStatusFail,
					Remediation: "Bind the required bot_token bridge secret and retry.",
				}}}, nil
			},
		}
		handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
			Config:   aghconfig.DefaultWithHome(homePaths),
			Observer: apitestutil.StubObserver{},
			Sessions: apitestutil.StubSessionManager{},
			Bridges:  bridges,
		})
		router := gin.New()
		router.GET("/doctor", handlers.GetDoctor)

		response := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(
			testutil.Context(t),
			http.MethodGet,
			"/doctor?only=bridge",
			http.NoBody,
		)
		router.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", response.Code, response.Body.String())
		}
		if bridgeChecks != 1 {
			t.Fatalf("bridge check calls = %d, want 1", bridgeChecks)
		}

		var payload contract.DoctorPayload
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal(doctor) error = %v", err)
		}
		if len(payload.Items) != 1 {
			t.Fatalf("doctor items = %#v, want one bridge result", payload.Items)
		}
		item := payload.Items[0]
		if item.Category != contract.CategoryBridge || item.Severity != contract.SeverityError ||
			!strings.Contains(item.Message, "bot_token") {
			t.Fatalf("doctor item = %#v, want failing bot_token remediation", item)
		}
	})
}
