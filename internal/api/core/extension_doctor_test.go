// Suite: extension doctor API composition
// Invariant: crash-loop, missing environment, and stale development origin remain distinct actionable diagnostics.
// Boundary IN: BaseHandlers extension doctor registration and filtering.
// Boundary OUT: extension runtime state transitions, owned by internal/extension suites.
package core_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	apitestutil "github.com/compozy/compozy/internal/api/testutil"
	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/gin-gonic/gin"
)

func TestDoctorExtensionFilterReportsDistinctRuntimeFailures(t *testing.T) {
	t.Parallel()

	t.Run("Should report each extension failure as a distinct item", func(t *testing.T) {
		t.Parallel()

		const doctorOriginCanary = "/private/DOCTOR-WORKSPACE-ROOT-CANARY/DOCTOR-ORIGIN-CANARY"
		homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
			Config:   compozyconfig.DefaultWithHome(homePaths),
			Observer: apitestutil.StubObserver{},
			Sessions: apitestutil.StubSessionManager{},
			Extensions: extensionServiceStub{listFn: func(context.Context) ([]contract.ExtensionPayload, error) {
				return []contract.ExtensionPayload{
					{
						Name:                "crashy",
						ConsecutiveFailures: extensionpkg.DefaultRestartFailureThreshold,
						RestartBackoffMS:    8000,
					},
					{Name: "missing-env", MissingEnv: []string{"API_TOKEN"}},
					{Name: "stale-dev", Dev: true, FailureCode: "missing_origin", OriginPath: doctorOriginCanary},
				}, nil
			}},
		})
		router := gin.New()
		router.GET("/doctor", handlers.GetDoctor)

		response := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(
			testutil.Context(t),
			http.MethodGet,
			"/doctor?only=extension",
			http.NoBody,
		)
		router.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", response.Code, response.Body.String())
		}
		if strings.Contains(response.Body.String(), doctorOriginCanary) ||
			strings.Contains(response.Body.String(), `"origin_path"`) {
			t.Fatalf("doctor body leaked development origin: %s", response.Body.String())
		}

		var payload contract.DoctorPayload
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal(doctor) error = %v", err)
		}
		ids := make([]string, 0, len(payload.Items))
		var staleOriginItem *contract.DiagnosticItem
		for index := range payload.Items {
			item := payload.Items[index]
			if item.Category != contract.CategoryExtension {
				t.Fatalf("doctor item category = %q, want extension", item.Category)
			}
			ids = append(ids, item.ID)
			if item.ID == "doctor.extension.missing-env.missing_env" &&
				item.SuggestedCommand != "compozy extension secrets set missing-env --env API_TOKEN" {
				t.Fatalf("missing-env suggested command = %q, want exact secrets set command", item.SuggestedCommand)
			}
			if item.ID == "doctor.extension.stale-dev.stale_dev_origin" {
				staleOriginItem = &payload.Items[index]
			}
		}
		want := []string{
			"doctor.extension.crashy.crash_loop",
			"doctor.extension.missing-env.missing_env",
			"doctor.extension.stale-dev.stale_dev_origin",
		}
		for _, id := range want {
			if !slices.Contains(ids, id) {
				t.Fatalf("doctor item ids = %#v, want %q", ids, id)
			}
		}
		if staleOriginItem == nil {
			t.Fatal("stale development-origin diagnostic is missing")
		}
		if staleOriginItem.SuggestedCommand != "compozy extension status stale-dev" {
			t.Fatalf("stale origin command = %q, want safe status command", staleOriginItem.SuggestedCommand)
		}
		if _, exists := staleOriginItem.Evidence["origin_path"]; exists {
			t.Fatalf("stale origin evidence = %#v, must not include origin_path", staleOriginItem.Evidence)
		}
		if got := staleOriginItem.Evidence["failure_code"]; got != "missing_origin" {
			t.Fatalf("stale origin failure_code = %#v, want missing_origin", got)
		}
	})
}
