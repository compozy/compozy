package core

import (
	"context"
	"net/http"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/modelcatalog"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

// pendingUserWorkspaceStub answers user-scope resolution with a desired config that still holds
// a pending, unapplied edit.
type pendingUserWorkspaceStub struct {
	workspaceResolveServiceStub
	desired compozyconfig.Config
}

func (s *pendingUserWorkspaceStub) ResolveUser(context.Context, string) (workspacepkg.ResolvedWorkspace, error) {
	return workspacepkg.ResolvedWorkspace{Config: s.desired}, nil
}

func TestModelCatalogSelectionConfig(t *testing.T) {
	t.Parallel()

	t.Run("Should filter default-profile reads with the active config, not a pending disable", func(t *testing.T) {
		t.Parallel()
		active := compozyconfig.DefaultWithHome(compozyconfig.HomePaths{})
		desired := compozyconfig.CloneConfig(&active)
		codex := desired.Providers["codex"]
		codex.Enabled = new(false)
		desired.Providers["codex"] = codex
		service := &modelCatalogServiceSpy{
			listModelsFn: func(context.Context, modelcatalog.ListOptions) ([]modelcatalog.Model, error) {
				return []modelcatalog.Model{seedModelCatalogModel("codex", "gpt-5.4")}, nil
			},
		}
		engine := newModelCatalogCoreEngineWithConfig(t, &BaseHandlerConfig{
			Config:       active,
			ModelCatalog: service,
			Workspaces:   &pendingUserWorkspaceStub{desired: desired},
		})
		recorder := performModelCatalogRequest(t, engine, http.MethodGet, "/model-catalog/providers/codex/models", nil)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
		}
		var payload contract.ProviderModelListResponse
		decodeModelCatalogResponse(t, recorder, &payload)
		if len(payload.Models) != 1 {
			t.Fatalf("models = %#v, want codex kept while its disable is still pending", payload.Models)
		}
	})
}
