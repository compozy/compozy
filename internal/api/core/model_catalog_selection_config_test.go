package core

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
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
	t.Run("Should apply the default profile's own provider disable", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		homePaths := compozyconfig.HomePaths{
			ConfigFile:  filepath.Join(root, compozyconfig.ConfigName),
			ProfilesDir: filepath.Join(root, "profiles"),
		}
		profileDir := filepath.Join(homePaths.ProfilesDir, "default")
		if err := os.MkdirAll(profileDir, 0o700); err != nil {
			t.Fatalf("mkdir profile: %v", err)
		}
		profileFile := filepath.Join(profileDir, compozyconfig.ConfigName)
		if err := os.WriteFile(profileFile, []byte("[providers.codex]\nenabled = false\n"), 0o600); err != nil {
			t.Fatalf("write profile config: %v", err)
		}
		service := &modelCatalogServiceSpy{
			listModelsFn: func(context.Context, modelcatalog.ListOptions) ([]modelcatalog.Model, error) {
				return []modelcatalog.Model{seedModelCatalogModel("codex", "gpt-5.4")}, nil
			},
		}
		engine := newModelCatalogCoreEngineWithConfig(t, &BaseHandlerConfig{
			Config:       compozyconfig.DefaultWithHome(homePaths),
			HomePaths:    homePaths,
			ModelCatalog: service,
		})
		recorder := performModelCatalogRequest(t, engine, http.MethodGet, "/model-catalog/providers/codex/models", nil)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
		}
		var payload contract.ProviderModelListResponse
		decodeModelCatalogResponse(t, recorder, &payload)
		if len(payload.Models) != 0 {
			t.Fatalf("models = %#v, want codex dropped by the default profile's disable", payload.Models)
		}
	})
	t.Run("Should drop models of a provider the profile defines and disables on its own", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		homePaths := compozyconfig.HomePaths{
			ConfigFile:  filepath.Join(root, compozyconfig.ConfigName),
			ProfilesDir: filepath.Join(root, "profiles"),
		}
		profileDir := filepath.Join(homePaths.ProfilesDir, "default")
		if err := os.MkdirAll(profileDir, 0o700); err != nil {
			t.Fatalf("mkdir profile: %v", err)
		}
		profileFile := filepath.Join(profileDir, compozyconfig.ConfigName)
		profileTOML := "[providers.work]\ncommand = \"work-acp\"\nenabled = false\n"
		if err := os.WriteFile(profileFile, []byte(profileTOML), 0o600); err != nil {
			t.Fatalf("write profile config: %v", err)
		}
		service := &modelCatalogServiceSpy{
			listModelsFn: func(context.Context, modelcatalog.ListOptions) ([]modelcatalog.Model, error) {
				return []modelcatalog.Model{
					seedModelCatalogModel("codex", "gpt-5.4"),
					seedModelCatalogModel("work", "work-large"),
				}, nil
			},
		}
		engine := newModelCatalogCoreEngineWithConfig(t, &BaseHandlerConfig{
			Config:       compozyconfig.DefaultWithHome(homePaths),
			HomePaths:    homePaths,
			ModelCatalog: service,
		})
		recorder := performModelCatalogRequest(t, engine, http.MethodGet, "/model-catalog/models", nil)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
		}
		var payload contract.ProviderModelListResponse
		decodeModelCatalogResponse(t, recorder, &payload)
		if len(payload.Models) != 1 || payload.Models[0].ProviderID != "codex" {
			t.Fatalf("models = %#v, want only codex once the profile disables its own work provider", payload.Models)
		}
	})
	t.Run(
		"Should apply the selected profile's disable to a workspace read without profile-aware resolution",
		func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			homePaths := compozyconfig.HomePaths{
				ConfigFile:  filepath.Join(root, compozyconfig.ConfigName),
				ProfilesDir: filepath.Join(root, "profiles"),
			}
			profileDir := filepath.Join(homePaths.ProfilesDir, "default")
			if err := os.MkdirAll(profileDir, 0o700); err != nil {
				t.Fatalf("mkdir profile: %v", err)
			}
			profileFile := filepath.Join(profileDir, compozyconfig.ConfigName)
			if err := os.WriteFile(profileFile, []byte("[providers.codex]\nenabled = false\n"), 0o600); err != nil {
				t.Fatalf("write profile config: %v", err)
			}
			workspaceConfig := compozyconfig.DefaultWithHome(homePaths)
			service := &modelCatalogServiceSpy{
				listModelsFn: func(context.Context, modelcatalog.ListOptions) ([]modelcatalog.Model, error) {
					return []modelcatalog.Model{seedModelCatalogModel("codex", "gpt-5.4")}, nil
				},
			}
			engine := newModelCatalogCoreEngineWithConfig(t, &BaseHandlerConfig{
				Config:       compozyconfig.DefaultWithHome(homePaths),
				HomePaths:    homePaths,
				ModelCatalog: service,
				Workspaces: workspaceResolveServiceStub{
					resolve: func(context.Context, string) (workspacepkg.ResolvedWorkspace, error) {
						return workspacepkg.ResolvedWorkspace{ID: "ws-lab", Config: workspaceConfig}, nil
					},
				},
			})
			recorder := performModelCatalogRequest(
				t, engine, http.MethodGet, "/model-catalog/providers/codex/models?workspace_id=ws-lab", nil,
			)
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
			}
			var payload contract.ProviderModelListResponse
			decodeModelCatalogResponse(t, recorder, &payload)
			if len(payload.Models) != 0 {
				t.Fatalf("models = %#v, want codex dropped by the selected profile's disable", payload.Models)
			}
		},
	)
}
