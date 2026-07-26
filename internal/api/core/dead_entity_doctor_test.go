// Suite: dead-entity doctor API composition
// Invariant: the public MCP filter exposes durable workspace marks without a mutation affordance.
// Boundary IN: BaseHandlers doctor registration, workspace fan-out, and filtering.
// Boundary OUT: dead-entity state transitions and redaction, owned by internal/deadentity and internal/doctor.
package core_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	core "github.com/compozy/agh/internal/api/core"
	apitestutil "github.com/compozy/agh/internal/api/testutil"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	"github.com/gin-gonic/gin"
)

func TestDoctorMCPFilterIncludesDurableDeadEntities(t *testing.T) {
	t.Run("Should expose a workspace dead mark without a revive command", func(t *testing.T) {
		t.Parallel()

		workspaceID := "ws-doctor-dead"
		handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
			Workspaces: apitestutil.StubWorkspaceService{
				ListFn: func(context.Context) ([]workspacepkg.Workspace, error) {
					return []workspacepkg.Workspace{{ID: workspaceID, Name: "Doctor Dead"}}, nil
				},
			},
			DeadEntities: deadEntityDoctorSource{entities: map[string][]store.DeadEntity{
				workspaceID: {
					{
						DeadEntityKey: store.DeadEntityKey{
							WorkspaceID: workspaceID,
							Kind:        store.DeadEntityKindMCPSidecar,
							EntityID:    "github",
						},
						Reason:   "invalid configuration",
						MarkedAt: time.Date(2026, 7, 15, 20, 0, 0, 0, time.UTC),
					},
				},
			}},
		})
		router := gin.New()
		router.GET("/doctor", handlers.GetDoctor)

		response := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(
			testutil.Context(t),
			http.MethodGet,
			"/doctor?only=mcp",
			http.NoBody,
		)
		router.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d body = %s, want 200", response.Code, response.Body.String())
		}
		var payload contract.DoctorPayload
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal(doctor) error = %v", err)
		}
		if len(payload.Items) != 1 {
			t.Fatalf("doctor items = %#v, want one durable MCP diagnostic", payload.Items)
		}
		item := payload.Items[0]
		if item.Category != contract.CategoryMCP || item.Code != contract.CodeMCPServerUnavailable {
			t.Fatalf("doctor item = %#v, want MCP unavailable", item)
		}
		if item.SuggestedCommand != "" {
			t.Fatalf("SuggestedCommand = %q, want no revive control", item.SuggestedCommand)
		}
	})
}

type deadEntityDoctorSource struct {
	entities map[string][]store.DeadEntity
}

func (s deadEntityDoctorSource) List(
	_ context.Context,
	workspaceID string,
) ([]store.DeadEntity, error) {
	return append([]store.DeadEntity(nil), s.entities[workspaceID]...), nil
}
