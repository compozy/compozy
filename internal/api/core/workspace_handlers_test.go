package core_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/api/testutil"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func TestWorkspaceHandlersDelegateToService(t *testing.T) {
	t.Parallel()

	setup := func(t *testing.T) (handlerFixture, workspacepkg.Workspace, workspacepkg.ResolvedWorkspace, *bool, *bool, *bool, string, string) {
		t.Helper()

		rootDir := filepath.Join(t.TempDir(), "root dir")
		addDir := filepath.Join(t.TempDir(), "add dir")
		if err := os.MkdirAll(rootDir, 0o755); err != nil {
			t.Fatalf("MkdirAll(rootDir) error = %v", err)
		}
		if err := os.MkdirAll(addDir, 0o755); err != nil {
			t.Fatalf("MkdirAll(addDir) error = %v", err)
		}
		workspace := workspacepkg.Workspace{
			ID:             "ws_alpha",
			RootDir:        rootDir,
			AdditionalDirs: []string{addDir},
			Name:           "alpha",
			DefaultAgent:   "coder",

			CreatedAt: time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, 4, 3, 12, 1, 0, 0, time.UTC),
		}
		resolved := workspacepkg.ResolvedWorkspace{
			Workspace: workspace,
			Config: compozyconfig.Config{
				Providers: map[string]compozyconfig.ProviderConfig{
					"alpha": {Command: "alpha --acp"},
				},
			},
			Agents: []compozyconfig.AgentDef{{
				Name:     "coder",
				Provider: "fake",
				Prompt:   "hello",
			}},
			Skills: []workspacepkg.SkillPath{{
				Dir:    filepath.Join(rootDir, ".skills", "build"),
				Source: "workspace",
			}},
		}
		updateCalled := false
		deleteCalled := false
		resolveCalled := false
		workspaces := testutil.StubWorkspaceService{
			RegisterFn: func(_ context.Context, opts workspacepkg.RegisterOptions) (workspacepkg.Workspace, error) {
				if opts.RootDir != rootDir || len(opts.AdditionalDirs) != 1 ||
					opts.AdditionalDirs[0] != addDir || opts.DefaultAgent != "coder" {
					t.Fatalf("Register opts = %#v", opts)
				}
				return workspace, nil
			},
			ListFn: func(context.Context) ([]workspacepkg.Workspace, error) {
				return []workspacepkg.Workspace{workspace}, nil
			},
			GetFn: func(context.Context, string) (workspacepkg.Workspace, error) {
				return workspace, nil
			},
			ResolveFn: func(context.Context, string) (workspacepkg.ResolvedWorkspace, error) {
				return resolved, nil
			},
			UpdateFn: func(_ context.Context, id string, opts workspacepkg.UpdateOptions) error {
				updateCalled = true
				if id != workspace.ID || opts.Name == nil || *opts.Name != "beta" {
					t.Fatalf("Update call = %q %#v", id, opts)
				}

				return nil
			},
			UnregisterFn: func(_ context.Context, id string) error {
				deleteCalled = true
				if id != workspace.ID {
					t.Fatalf("Unregister id = %q, want %q", id, workspace.ID)
				}
				return nil
			},
			ResolveOrRegisterFn: func(_ context.Context, path string) (workspacepkg.ResolvedWorkspace, error) {
				resolveCalled = true
				if path != rootDir {
					t.Fatalf("ResolveOrRegister path = %q, want %q", path, rootDir)
				}
				return resolved, nil
			},
		}
		manager := testutil.StubSessionManager{
			ListAllFn: func(context.Context) ([]*session.Info, error) {
				info := testutil.NewSessionInfo("sess-a")
				info.WorkspaceID = workspace.ID
				info.State = session.StateStopped
				return []*session.Info{info}, nil
			},
		}

		return newHandlerFixture(
			t,
			manager,
			testutil.StubObserver{},
			workspaces,
		), workspace, resolved, &updateCalled, &deleteCalled, &resolveCalled, rootDir, addDir
	}

	t.Run("Should create a workspace", func(t *testing.T) {
		t.Parallel()

		fixture, _, _, _, _, _, rootDir, addDir := setup(t)
		createBody, err := json.Marshal(contract.CreateWorkspaceRequest{
			RootDir:      rootDir,
			AddDirs:      []string{addDir},
			Name:         "alpha",
			DefaultAgent: "coder",
		})
		if err != nil {
			t.Fatalf("json.Marshal(create workspace request) error = %v", err)
		}
		createResp := performRequest(t, fixture.Engine, http.MethodPost, "/workspaces", createBody)
		if createResp.Code != http.StatusCreated {
			t.Fatalf("create workspace status = %d, want %d", createResp.Code, http.StatusCreated)
		}

		var payload struct {
			Workspace contract.WorkspacePayload `json:"workspace"`
		}
		testutil.DecodeJSONResponse(t, createResp, &payload)
		if payload.Workspace.RootDir != rootDir || len(payload.Workspace.AddDirs) != 1 ||
			payload.Workspace.AddDirs[0] != addDir {
			t.Fatalf("create workspace payload = %#v", payload.Workspace)
		}
	})

	t.Run("Should return default agent validation as a bad request", func(t *testing.T) {
		t.Parallel()

		rootDir := t.TempDir()
		validationMessage := `agent name "audio designer" must start with a lowercase letter, use only lowercase letters, numbers, hyphens, or underscores, and be at most 106 characters`
		validationErr := fmt.Errorf(
			"%w: %w",
			workspacepkg.ErrWorkspaceValidation,
			compozyconfig.ValidationError{
				Path:    "workspace.default_agent",
				Message: validationMessage,
			},
		)
		registerCalled := false
		workspaces := testutil.StubWorkspaceService{
			RegisterFn: func(context.Context, workspacepkg.RegisterOptions) (workspacepkg.Workspace, error) {
				registerCalled = true
				return workspacepkg.Workspace{}, validationErr
			},
		}
		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			workspaces,
		)

		response := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/workspaces",
			[]byte(`{"root_dir":"`+rootDir+`","name":"invalid-default-agent","default_agent":"audio designer"}`),
		)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusBadRequest, response.Body.String())
		}
		if !registerCalled {
			t.Fatal("Register() was not called")
		}

		var payload contract.ErrorPayload
		testutil.DecodeJSONResponse(t, response, &payload)
		wantError := "workspace validation failed: workspace.default_agent " + validationMessage
		if payload.Error != wantError {
			t.Fatalf("error = %q, want %q", payload.Error, wantError)
		}
	})

	t.Run("Should list workspaces", func(t *testing.T) {
		t.Parallel()

		fixture, _, _, _, _, _, _, _ := setup(t)
		listResp := performRequest(t, fixture.Engine, http.MethodGet, "/workspaces", nil)
		if listResp.Code != http.StatusOK {
			t.Fatalf("list workspaces status = %d, want %d", listResp.Code, http.StatusOK)
		}

		var payload struct {
			Workspaces []contract.WorkspacePayload `json:"workspaces"`
		}
		testutil.DecodeJSONResponse(t, listResp, &payload)
		if len(payload.Workspaces) != 1 || payload.Workspaces[0].ID != "ws_alpha" {
			t.Fatalf("list workspaces payload = %#v", payload.Workspaces)
		}
	})

	t.Run("Should get a workspace with sessions", func(t *testing.T) {
		t.Parallel()

		fixture, workspace, resolved, _, _, _, _, _ := setup(t)
		getResp := performRequest(t, fixture.Engine, http.MethodGet, "/workspaces/"+workspace.ID, nil)
		if getResp.Code != http.StatusOK {
			t.Fatalf("get workspace status = %d, want %d", getResp.Code, http.StatusOK)
		}

		var getPayload contract.WorkspaceDetailPayload
		testutil.DecodeJSONResponse(t, getResp, &getPayload)
		if len(getPayload.Sessions) != 1 || getPayload.Sessions[0].WorkspaceID != workspace.ID {
			t.Fatalf("sessions payload = %#v", getPayload.Sessions)
		}
		if sessionPayload := getPayload.Sessions[0]; sessionPayload.ProfileID != store.DefaultProfileID ||
			sessionPayload.ProfileName != "default" {
			t.Fatalf("workspace session profile owner = %#v, want default identity", sessionPayload)
		}
		expectedProviders := core.SessionProviderOptionPayloadsFromConfig(&resolved.Config)
		if got, want := len(getPayload.Providers), len(expectedProviders); got != want {
			t.Fatalf("len(providers) = %d, want %d (%#v)", got, want, getPayload.Providers)
		}
		for i, want := range expectedProviders {
			if got := getPayload.Providers[i]; !reflect.DeepEqual(got, want) {
				t.Fatalf("providers[%d] = %#v, want %#v", i, got, want)
			}
		}
	})

	t.Run("Should merge projected catalog agents into workspace detail", func(t *testing.T) {
		t.Parallel()

		fixture, workspace, _, _, _, _, _, _ := setup(t)
		fixture.Handlers.AgentCatalog = stubAgentCatalog{
			agents: []compozyconfig.AgentDef{
				{
					Name:     "coder",
					Provider: "catalog-should-not-win",
					Prompt:   "global duplicate",
				},
				{
					Name:     "qa-extension-agent",
					Provider: "codex",
					Prompt:   "extension agent",
				},
				{
					Name:     "onboarding",
					Provider: "codex",
					Prompt:   "internal onboarding",
				},
			},
		}

		getResp := performRequest(t, fixture.Engine, http.MethodGet, "/workspaces/"+workspace.ID, nil)
		if getResp.Code != http.StatusOK {
			t.Fatalf("get workspace status = %d, want %d", getResp.Code, http.StatusOK)
		}

		var getPayload contract.WorkspaceDetailPayload
		testutil.DecodeJSONResponse(t, getResp, &getPayload)
		if got, want := len(getPayload.Agents), 3; got != want {
			t.Fatalf("len(agents) = %d, want %d: %#v", got, want, getPayload.Agents)
		}
		if got, want := getPayload.Agents[0].Name, "coder"; got != want {
			t.Fatalf("agents[0].name = %q, want %q", got, want)
		}
		if got, want := getPayload.Agents[0].Provider, "fake"; got != want {
			t.Fatalf("agents[0].provider = %q, want workspace-scoped provider %q", got, want)
		}
		if got, want := getPayload.Agents[1].Name, "onboarding"; got != want {
			t.Fatalf("agents[1].name = %q, want %q", got, want)
		}
		if got, want := getPayload.Agents[2].Name, "qa-extension-agent"; got != want {
			t.Fatalf("agents[2].name = %q, want %q", got, want)
		}
	})

	t.Run("Should update a workspace via the service", func(t *testing.T) {
		t.Parallel()

		fixture, workspace, _, updateCalled, _, _, _, _ := setup(t)
		updateResp := performRequest(
			t,
			fixture.Engine,
			http.MethodPatch,
			"/workspaces/"+workspace.ID,
			[]byte(`{"name":"beta"}`),
		)
		if updateResp.Code != http.StatusOK || !*updateCalled {
			t.Fatalf("update status=%d called=%v", updateResp.Code, *updateCalled)
		}

		var payload struct {
			Workspace contract.WorkspacePayload `json:"workspace"`
		}
		testutil.DecodeJSONResponse(t, updateResp, &payload)
		if payload.Workspace.Name != "alpha" {
			t.Fatalf("update workspace payload = %#v", payload.Workspace)
		}
	})

	t.Run("Should delete a workspace via the service", func(t *testing.T) {
		t.Parallel()

		fixture, workspace, _, _, deleteCalled, _, _, _ := setup(t)
		fixture.Handlers.Sessions = testutil.StubSessionManager{
			ListAllFn: func(context.Context) ([]*session.Info, error) {
				t.Fatal("DeleteWorkspace() called Sessions.ListAll")
				return nil, nil
			},
			DeleteFn: func(context.Context, string) error {
				t.Fatal("DeleteWorkspace() called Sessions.Delete")
				return nil
			},
		}
		deleteResp := performRequest(t, fixture.Engine, http.MethodDelete, "/workspaces/"+workspace.ID, nil)
		if deleteResp.Code != http.StatusNoContent || !*deleteCalled {
			t.Fatalf("delete status=%d called=%v", deleteResp.Code, *deleteCalled)
		}
	})

	t.Run("Should resolve a workspace path via the service", func(t *testing.T) {
		t.Parallel()

		fixture, _, _, _, _, resolveCalled, rootDir, _ := setup(t)
		resolveBody, err := json.Marshal(contract.ResolveWorkspaceRequest{Path: rootDir})
		if err != nil {
			t.Fatalf("json.Marshal(resolve workspace request) error = %v", err)
		}
		resolveResp := performRequest(t, fixture.Engine, http.MethodPost, "/workspaces/resolve", resolveBody)
		if resolveResp.Code != http.StatusOK || !*resolveCalled {
			t.Fatalf("resolve status=%d called=%v", resolveResp.Code, *resolveCalled)
		}

		var payload struct {
			Workspace contract.WorkspacePayload `json:"workspace"`
		}
		testutil.DecodeJSONResponse(t, resolveResp, &payload)
		if payload.Workspace.RootDir != rootDir {
			t.Fatalf("resolve workspace payload = %#v", payload.Workspace)
		}
	})
}

func TestWorkspaceUpdateSupportsAddDirsAndDefaultAgent(t *testing.T) {
	t.Parallel()

	t.Run("Should update add_dirs and default_agent", func(t *testing.T) {
		t.Parallel()

		rootDir := t.TempDir()
		addDir := t.TempDir()
		workspace := workspacepkg.Workspace{ID: "ws_alpha", RootDir: rootDir, Name: "alpha"}
		var captured workspacepkg.UpdateOptions
		workspaces := testutil.StubWorkspaceService{
			GetFn: func(context.Context, string) (workspacepkg.Workspace, error) {
				return workspace, nil
			},
			UpdateFn: func(_ context.Context, _ string, opts workspacepkg.UpdateOptions) error {
				captured = opts
				return nil
			},
		}
		fixture := newHandlerFixture(t, testutil.StubSessionManager{}, testutil.StubObserver{}, workspaces)

		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodPatch,
			"/workspaces/ws_alpha",
			[]byte(`{"add_dirs":["`+addDir+`"],"default_agent":"coder"}`),
		)
		if resp.Code != http.StatusOK {
			t.Fatalf("update add_dirs/default_agent status = %d, want %d", resp.Code, http.StatusOK)
		}
		if captured.AdditionalDirs == nil || len(*captured.AdditionalDirs) != 1 ||
			(*captured.AdditionalDirs)[0] != addDir {
			t.Fatalf("captured add dirs = %#v", captured.AdditionalDirs)
		}
		if captured.DefaultAgent == nil || *captured.DefaultAgent != "coder" {
			t.Fatalf("captured default agent = %#v", captured.DefaultAgent)
		}
	})
}
