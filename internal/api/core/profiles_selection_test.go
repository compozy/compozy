package core_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	profilepkg "github.com/compozy/compozy/internal/profile"
	"github.com/gin-gonic/gin"
)

func (sessionProfileServiceStub) ListSelections(context.Context) ([]profilepkg.Selection, error) {
	return []profilepkg.Selection{{
		Lens: profilepkg.SelectionLensWorkspace, WorkspaceID: "ws-marketing", ProfileID: "profile-marketing",
	}}, nil
}

func TestGetProfileSelectionsReturnsOneStableShape(t *testing.T) {
	t.Parallel()

	t.Run("Should return arrays for the full map and filtered lenses", func(t *testing.T) {
		t.Parallel()

		handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{Profiles: sessionProfileServiceStub{}})
		engine := gin.New()
		engine.GET("/profiles/selection", handlers.GetProfileSelections)

		tests := []struct {
			name        string
			path        string
			wantScope   contract.ProfileSelectionScope
			wantProfile string
		}{
			{
				name: "full selection map", path: "/profiles/selection",
				wantScope: contract.ProfileSelectionScopeWorkspace, wantProfile: "marketing",
			},
			{
				name: "stored workspace lens", path: "/profiles/selection?scope=workspace&workspace_id=ws-marketing",
				wantScope: contract.ProfileSelectionScopeWorkspace, wantProfile: "marketing",
			},
			{
				name: "unstored global lens", path: "/profiles/selection?scope=global",
				wantScope: contract.ProfileSelectionScopeGlobal, wantProfile: "default",
			},
		}
		for _, test := range tests {
			t.Run("Should return an array for "+test.name, func(t *testing.T) {
				response := performRequest(t, engine, http.MethodGet, test.path, nil)
				if response.Code != http.StatusOK {
					t.Fatalf(
						"GET %s status = %d, want %d; body=%s",
						test.path,
						response.Code,
						http.StatusOK,
						response.Body.String(),
					)
				}
				var payload []contract.ProfileSelection
				decodeJSON(t, response.Body.Bytes(), &payload)
				if len(payload) != 1 || payload[0].Scope != test.wantScope || payload[0].Profile != test.wantProfile {
					t.Fatalf(
						"GET %s payload = %#v, want one %s/%s selection",
						test.path,
						payload,
						test.wantScope,
						test.wantProfile,
					)
				}
			})
		}
	})
}
