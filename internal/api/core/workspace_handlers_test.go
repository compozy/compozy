package core_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/workspaceaccess"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/api/testutil"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/gin-gonic/gin"
)

func TestWorkspaceUnknownConfigKeys(t *testing.T) {
	t.Parallel()
	for _, route := range []struct{ method, path, body string }{
		{http.MethodPost, "/workspaces", `{"root_dir":"/project"}`},
		{http.MethodPost, "/workspaces/resolve", `{"path":"/project"}`},
		{http.MethodGet, "/workspaces/ws_alpha", ""},
	} {
		t.Run("Should return actionable config rejection for "+route.method+" "+route.path, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			configPath := filepath.Join(root, ".compozy", "config.toml")
			if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(configPath, []byte("[defaults]\nide = 'codex'\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			home := testutil.NewTestHomePaths(t)
			_, configErr := compozyconfig.LoadForHome(home, compozyconfig.WithWorkspaceRoot(root))
			if configErr == nil {
				t.Fatal("expected invalid workspace configuration")
			}
			fixture := newHandlerFixture(t, &testutil.StubSessionManager{}, testutil.StubObserver{},
				testutil.StubWorkspaceService{
					RegisterFn: func(context.Context, workspacepkg.RegisterOptions) (workspacepkg.Workspace, error) {
						return workspacepkg.Workspace{}, configErr
					},
					ResolveFn: func(context.Context, string) (workspacepkg.ResolvedWorkspace, error) {
						return workspacepkg.ResolvedWorkspace{}, configErr
					},
					ResolveOrRegisterFn: func(context.Context, string) (workspacepkg.ResolvedWorkspace, error) {
						return workspacepkg.ResolvedWorkspace{}, configErr
					},
				})
			resp := performRequest(t, fixture.Engine, route.method, route.path, []byte(route.body))
			var payload contract.ErrorPayload
			testutil.DecodeJSONResponse(t, resp, &payload)
			if resp.Code != http.StatusUnprocessableEntity || payload.Code != "workspace_config_invalid" ||
				payload.ConfigPath != configPath || !slices.Equal(payload.UnknownKeys, []string{"defaults.ide"}) {
				t.Fatalf("response = %d %#v", resp.Code, payload)
			}
			if !strings.Contains(payload.Error, "rename or remove this file") {
				t.Fatalf("missing remediation: %s", payload.Error)
			}
			masked := core.ErrorPayloadForStatus(http.StatusInternalServerError, configErr, true)
			if !reflect.DeepEqual(masked, payload) {
				t.Fatalf("HTTP masked payload = %#v, want shared payload %#v", masked, payload)
			}
		})
	}
}

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
			Workspace: workspace, WorkspaceID: workspace.ID,
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
		manager := &testutil.StubSessionManager{
			ListForWorkspaceFn: func(_ context.Context, workspaceID string) ([]*session.Info, error) {
				if workspaceID != workspace.ID {
					t.Fatalf("ListForWorkspace id = %q, want %q", workspaceID, workspace.ID)
				}
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
			&testutil.StubSessionManager{},
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

	// Registry details must remain readable for cleanup without loading missing project files.
	t.Run("Should retain missing workspace details and sessions", func(t *testing.T) {
		t.Parallel()
		fixture, workspace, _, _, _, _, _, _ := setup(t)
		fixture.Handlers.Workspaces = testutil.StubWorkspaceService{
			GetFn: func(context.Context, string) (workspacepkg.Workspace, error) { return workspace, nil },
			ResolveFn: func(context.Context, string) (workspacepkg.ResolvedWorkspace, error) {
				return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceRootMissing
			},
		}
		response := performRequest(t, fixture.Engine, http.MethodGet, "/workspaces/"+workspace.ID, nil)
		if response.Code != http.StatusOK {
			t.Fatalf("missing workspace = %d %s", response.Code, response.Body)
		}
		var payload contract.WorkspaceDetailPayload
		testutil.DecodeJSONResponse(t, response, &payload)
		if !payload.Workspace.RootMissing || len(payload.Sessions) != 1 || len(payload.Agents) != 0 {
			t.Fatalf("missing details = %#v", payload)
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
		fixture.Handlers.Sessions = &testutil.StubSessionManager{
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
		fixture := newHandlerFixture(t, &testutil.StubSessionManager{}, testutil.StubObserver{}, workspaces)

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

func TestUserWorkspaceReferenceGuard(t *testing.T) {
	t.Parallel()
	t.Run("Should admit User loops goals and terminals while rejecting worktrees", func(t *testing.T) {
		t.Parallel()
		fixture := newHandlerFixture(
			t,
			&testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
		)
		router := gin.New()
		router.Use(fixture.Handlers.WorkspaceReferenceGuard)
		for _, surface := range []string{"loops", "loop-runs", "loop-requests", "terminals", "goal", "session-order"} {
			path := "/workspaces/:workspace_id/" + surface
			router.GET(path, func(c *gin.Context) { c.Status(http.StatusNoContent) })
			for _, suffix := range []string{"", "?worktree=feature"} {
				resp := performRequest(t, router, http.MethodGet, "/workspaces/@user/"+surface+suffix, nil)
				expected := http.StatusNoContent
				if suffix != "" {
					expected = http.StatusBadRequest
				}
				if resp.Code != expected {
					t.Fatalf("%s%s = %d", surface, suffix, resp.Code)
				}
			}
		}
	})

	t.Run("Should admit User selectors on agent, task, automation and Home reads", func(t *testing.T) {
		t.Parallel()
		fixture := newHandlerFixture(
			t,
			&testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
		)
		router := gin.New()
		router.Use(fixture.Handlers.WorkspaceReferenceGuard)
		for _, path := range []string{"/agents", "/agents/:name", "/agent-instances", "/tasks", "/automation/runs", "/observe/overview"} {
			router.GET(path, func(c *gin.Context) { c.Status(http.StatusNoContent) })
		}
		for _, target := range []string{
			"/agents?workspace=@user", "/agents/coder?workspace=@user", "/agent-instances?workspace=@user", "/tasks?workspace=@user",
			"/automation/runs?workspace_id=@user", "/observe/overview?workspace=@user",
		} {
			if resp := performRequest(t, router, http.MethodGet, target, nil); resp.Code != http.StatusNoContent {
				t.Fatalf("%s = %d, want the User selector admitted", target, resp.Code)
			}
		}
	})

	for _, tc := range []struct{ path, code string }{
		{"/workspaces/@user", "user_scope_unsupported"},
		{"/workspaces/@user/worktrees", "user_scope_worktree_unsupported"},
		{"/catalog?workspace=@user", "user_scope_unsupported"},
		{"/catalog?workspace_id=@user&worktree=feature", "user_scope_worktree_unsupported"},
	} {
		t.Run("Should reject unsupported User reference "+tc.path, func(t *testing.T) {
			t.Parallel()
			fixture := newHandlerFixture(
				t,
				&testutil.StubSessionManager{},
				testutil.StubObserver{},
				testutil.StubWorkspaceService{},
			)
			router := gin.New()
			router.Use(fixture.Handlers.WorkspaceReferenceGuard)
			for _, path := range []string{"/workspaces/:workspace_id", "/workspaces/:workspace_id/worktrees", "/catalog"} {
				router.GET(path, func(c *gin.Context) { c.Status(http.StatusNoContent) })
			}
			resp := performRequest(t, router, http.MethodGet, tc.path, nil)
			var payload contract.ErrorPayload
			testutil.DecodeJSONResponse(t, resp, &payload)
			if resp.Code != http.StatusBadRequest || payload.Code != tc.code {
				t.Fatalf("response = %d %#v", resp.Code, payload)
			}
		})
	}
}

func TestUserSessionWorktreeReference(t *testing.T) {
	t.Parallel()
	t.Run("Should reject a User worktree before resolving the workspace", func(t *testing.T) {
		t.Parallel()
		fixture := newHandlerFixture(
			t,
			&testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
		)
		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/sessions",
			[]byte(`{"workspace":"@user","agent_name":"general","worktree":"feature"}`),
		)
		var payload contract.ErrorPayload
		testutil.DecodeJSONResponse(t, resp, &payload)
		if resp.Code != http.StatusBadRequest || payload.Code != "user_scope_worktree_unsupported" {
			t.Fatalf("response = %d %#v", resp.Code, payload)
		}
	})
}

func TestUserSessionScopeConflict(t *testing.T) {
	t.Run("Should reject conflicting scope before workspace or worktree resolution", func(t *testing.T) {
		t.Parallel()
		fixture := newHandlerFixture(
			t,
			&testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
		)
		router := gin.New()
		router.POST("/api/sessions", fixture.Handlers.CreateSession)
		for _, body := range []string{`{"scope":"user","workspace":"ws-a"}`, `{"scope":"user","worktree":"wt-a"}`, `{"scope":"user","new_worktree":{}}`, `{"scope":"user","workspace_path":"/tmp"}`} {
			resp := performRequest(t, router, http.MethodPost, "/api/sessions", []byte(body))
			var payload contract.ErrorPayload
			testutil.DecodeJSONResponse(t, resp, &payload)
			if resp.Code != http.StatusBadRequest || payload.Code != "session_create_scope_conflict" {
				t.Fatalf("response=%d %+v", resp.Code, payload)
			}
		}
	})
}

func TestUserSessionCreate(t *testing.T) {
	t.Run("Should create a User session without resolving a registered workspace", func(t *testing.T) {
		t.Parallel()
		manager := &testutil.StubSessionManager{
			CreateAcceptedFn: func(_ context.Context, opts session.CreateAcceptedOpts) (*session.Info, error) {
				if opts.Session.Scope != workspacepkg.ScopeUser || opts.Session.Workspace != "" ||
					opts.Session.WorkspacePath != "" {
					t.Fatalf("opts=%+v", opts)
				}
				return &session.Info{
					ID:        "sess-user",
					ProfileID: store.DefaultProfileID,
					State:     session.StateActive,
				}, nil
			},
		}
		fixture := newHandlerFixture(t, manager, testutil.StubObserver{}, testutil.StubWorkspaceService{})
		router := gin.New()
		router.POST("/api/sessions", fixture.Handlers.CreateSession)
		resp := performRequest(
			t,
			router,
			http.MethodPost,
			"/api/sessions",
			[]byte(`{"scope":"user","agent_name":"general"}`),
		)
		var payload contract.SessionResponse
		testutil.DecodeJSONResponse(t, resp, &payload)
		if resp.Code != http.StatusCreated || payload.Session.Scope != "user" || payload.Session.WorkspaceID != "" {
			t.Fatalf("response=%d %+v", resp.Code, payload)
		}
	})
}

func TestUserLogsReferenceGuard(t *testing.T) {
	t.Parallel()
	t.Run("Should admit explicit User logs and preserve the selector", func(t *testing.T) {
		t.Parallel()
		fixture := newHandlerFixture(
			t,
			&testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
		)
		router := gin.New()
		router.Use(fixture.Handlers.WorkspaceReferenceGuard)
		router.GET("/logs", func(c *gin.Context) {
			query, err := core.ParseLogsQuery(c)
			if err != nil {
				t.Fatal(err)
			}
			c.JSON(http.StatusOK, gin.H{"workspace": query.WorkspaceID})
		})
		resp := performRequest(t, router, http.MethodGet, "/logs?workspace_id=@user", nil)
		var payload struct {
			Workspace string `json:"workspace"`
		}
		testutil.DecodeJSONResponse(t, resp, &payload)
		if resp.Code != http.StatusOK || payload.Workspace != workspaceaccess.UserSelector {
			t.Fatalf("response = %d %#v", resp.Code, payload)
		}
	})
}

func TestUserSessionRouteAllProfilesRead(t *testing.T) {
	t.Parallel()
	t.Run("Should read one User session under the all-profiles lens through its own profile", func(t *testing.T) {
		t.Parallel()
		manager := &testutil.StubSessionManager{StatusFn: func(_ context.Context, id string) (*session.Info, error) {
			if id != "sess-user" {
				return nil, session.ErrSessionNotFound
			}
			return &session.Info{
				ID:        id,
				ProfileID: store.DefaultProfileID,
				AgentName: "coder",
				State:     session.StateStopped,
			}, nil
		}}
		fixture := newHandlerFixture(t, manager, testutil.StubObserver{}, testutil.StubWorkspaceService{})
		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/@user/sessions/sess-user?all_profiles=true",
			nil,
		)
		if resp.Code != http.StatusOK {
			t.Fatalf("GET User session (all_profiles) = %d %s, want 200", resp.Code, resp.Body.String())
		}
	})
}

func TestWorkspaceArchiveHandlers(t *testing.T) {
	t.Run("Should return typed blockers without changing the workspace", func(t *testing.T) {
		t.Parallel()
		fixture := newHandlerFixture(
			t,
			&testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{
				SetArchivedFn: func(_ context.Context, ref string, archived bool) (workspacepkg.Workspace, error) {
					if ref != "ws_archive" || !archived {
						t.Fatalf("archive target = %q %v", ref, archived)
					}
					return workspacepkg.Workspace{}, &workspacepkg.BusyError{Blockers: []workspacepkg.ArchiveBlocker{
						{Kind: "session", ID: "sess_busy", Name: "Working"},
					}}
				},
			},
		)
		resp := performRequest(t, fixture.Engine, http.MethodPost, "/workspaces/ws_archive/archive", nil)
		var payload contract.ErrorPayload
		testutil.DecodeJSONResponse(t, resp, &payload)
		if resp.Code != http.StatusConflict || payload.Code != "workspace_busy" ||
			len(payload.Blockers) != 1 || payload.Blockers[0].ID != "sess_busy" {
			t.Fatalf("response = %d %#v", resp.Code, payload)
		}
	})
	t.Run("Should separate active and archived catalogs and restore the same identity", func(t *testing.T) {
		t.Parallel()
		at := time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC)
		fixture := newHandlerFixture(
			t,
			&testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{
				ListFn: func(context.Context) ([]workspacepkg.Workspace, error) {
					return []workspacepkg.Workspace{{ID: "ws_active"}, {ID: "ws_archive", ArchivedAt: &at}}, nil
				},
				SetArchivedFn: func(_ context.Context, ref string, archived bool) (workspacepkg.Workspace, error) {
					if archived {
						t.Fatal("unarchive requested archive")
					}
					return workspacepkg.Workspace{ID: ref}, nil
				},
			},
		)
		for _, test := range []struct{ path, id string }{
			{"/workspaces", "ws_active"}, {"/workspaces?archived=true", "ws_archive"},
		} {
			resp := performRequest(t, fixture.Engine, http.MethodGet, test.path, nil)
			var payload contract.WorkspacesResponse
			testutil.DecodeJSONResponse(t, resp, &payload)
			if resp.Code != http.StatusOK || len(payload.Workspaces) != 1 || payload.Workspaces[0].ID != test.id {
				t.Fatalf("catalog = %d %#v", resp.Code, payload)
			}
		}
		resp := performRequest(t, fixture.Engine, http.MethodPost, "/workspaces/ws_archive/unarchive", nil)
		var payload contract.WorkspaceResponse
		testutil.DecodeJSONResponse(t, resp, &payload)
		if resp.Code != http.StatusOK || payload.Workspace.ID != "ws_archive" || payload.Workspace.ArchivedAt != nil {
			t.Fatalf("unarchive = %d %#v", resp.Code, payload)
		}
	})
}
