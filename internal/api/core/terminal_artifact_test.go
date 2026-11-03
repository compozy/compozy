package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
	"github.com/compozy/compozy/internal/testutil"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/compozy/compozy/internal/workspaceaccess"
	"github.com/gin-gonic/gin"
)

func TestTerminalHandlersShouldKeepProfileScopesClosed(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	manager := &scopeRecordingTerminalManager{infos: []terminalpkg.Info{
		{ID: "term-a", WS: "workspace-a", ProfileID: "profile-b"},
	}}
	provider := &terminalProviderStub{Manager: manager}
	handlers := NewBaseHandlers(&BaseHandlerConfig{TransportName: "udsapi", Terminal: provider})
	router := gin.New()
	router.GET("/api/workspaces/:workspace_id/terminals", handlers.ListTerminals)
	router.GET("/api/workspaces/:workspace_id/terminals/:id", handlers.GetTerminal)
	router.GET("/api/workspaces/:workspace_id/terminals/:id/read", handlers.ReadTerminal)
	router.POST("/api/workspaces/:workspace_id/terminals", handlers.CreateTerminal)

	list := httptest.NewRecorder()
	router.ServeHTTP(
		list,
		httptest.NewRequestWithContext(
			testutil.Context(t),
			http.MethodGet,
			"/api/workspaces/workspace-a/terminals?all_profiles=true",
			http.NoBody,
		),
	)
	if list.Code != http.StatusOK || !manager.scope.AllProfiles {
		t.Fatalf("aggregate list status/scope = %d/%#v", list.Code, manager.scope)
	}
	get := httptest.NewRecorder()
	router.ServeHTTP(
		get,
		httptest.NewRequestWithContext(testutil.Context(t),
			http.MethodGet,
			"/api/workspaces/workspace-a/terminals/term-a?all_profiles=true",
			http.NoBody),
	)
	// Invariant: the aggregate detail finds the terminal in whichever profile owns it.
	if get.Code != http.StatusOK || !strings.Contains(get.Body.String(), `"profile_id":"profile-b"`) {
		t.Fatalf("aggregate get status/body = %d/%s, want the owner-labeled terminal", get.Code, get.Body.String())
	}
	missing := httptest.NewRecorder()
	router.ServeHTTP(missing, httptest.NewRequestWithContext(testutil.Context(t), http.MethodGet,
		"/api/workspaces/workspace-a/terminals/term-gone?all_profiles=true", http.NoBody))
	if missing.Code != http.StatusNotFound || !strings.Contains(missing.Body.String(), `"code":"terminal_not_found"`) {
		t.Fatalf("aggregate missing get status/body = %d/%s, want typed 404", missing.Code, missing.Body.String())
	}
	read := httptest.NewRecorder()
	router.ServeHTTP(read, httptest.NewRequestWithContext(testutil.Context(t), http.MethodGet,
		"/api/workspaces/workspace-a/terminals/term-a/read?all_profiles=true", http.NoBody))
	if read.Code != http.StatusBadRequest ||
		!strings.Contains(read.Body.String(), `"code":"profile_selection_conflict"`) {
		t.Fatalf("aggregate interaction status/body = %d/%s, want typed 400", read.Code, read.Body.String())
	}
	create := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(testutil.Context(t),
		http.MethodPost,
		"/api/workspaces/workspace-a/terminals?all_profiles=true",
		strings.NewReader(`{}`))

	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(create, request)
	if create.Code != http.StatusBadRequest {
		t.Fatalf("mutation status = %d, want 400; body=%s", create.Code, create.Body.String())
	}
}

func TestTerminalDownloadsShouldStreamOnlyProfileScopedArtifacts(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	journal := &terminalDownloadJournalStub{}
	provider := &terminalProviderStub{Manager: terminalDownloadManagerStub{journal: journal}}
	handlers := NewBaseHandlers(&BaseHandlerConfig{TransportName: "udsapi", Terminal: provider})
	router := gin.New()
	router.GET("/api/workspaces/:workspace_id/terminals/recordings/:id", handlers.DownloadTerminalRecording)
	router.GET("/api/workspaces/:workspace_id/terminals/artifacts/:id", handlers.DownloadTerminalArtifact)

	recording := httptest.NewRecorder()
	router.ServeHTTP(
		recording,
		httptest.NewRequestWithContext(testutil.Context(t),
			http.MethodGet,
			"/api/workspaces/workspace-a/terminals/recordings/recording-a",
			http.NoBody),
	)
	if recording.Code != http.StatusOK || recording.Body.String() != "asciicast" {
		t.Fatalf("recording status/body = %d/%q, want 200/asciicast", recording.Code, recording.Body.String())
	}
	if got := recording.Header().Get("Content-Type"); got != "application/x-asciicast" {
		t.Fatalf("recording Content-Type = %q", got)
	}
	if journal.recordingScope.ProfileID != store.DefaultProfileID {
		t.Fatalf("recording scope = %#v, want default profile", journal.recordingScope)
	}

	artifact := httptest.NewRecorder()
	router.ServeHTTP(
		artifact,
		httptest.NewRequestWithContext(
			testutil.Context(t),
			http.MethodGet,
			"/api/workspaces/workspace-a/terminals/artifacts/artifact-a",
			http.NoBody,
		),
	)
	if artifact.Code != http.StatusOK || artifact.Body.String() != "artifact bytes" {
		t.Fatalf("artifact status/body = %d/%q, want 200/artifact bytes", artifact.Code, artifact.Body.String())
	}
	if journal.artifactScope.ProfileID != store.DefaultProfileID {
		t.Fatalf("artifact scope = %#v, want default profile", journal.artifactScope)
	}

	missing := httptest.NewRecorder()
	router.ServeHTTP(
		missing,
		httptest.NewRequestWithContext(
			testutil.Context(t),
			http.MethodGet,
			"/api/workspaces/workspace-a/terminals/recordings/foreign",
			http.NoBody,
		),
	)
	if missing.Code != http.StatusNotFound || !strings.Contains(missing.Body.String(), `"code":"terminal_not_found"`) {
		t.Fatalf("foreign recording status/body = %d/%s, want typed 404", missing.Code, missing.Body.String())
	}
}

type terminalAggregatePolicy bool

func (allowed terminalAggregatePolicy) Authorize(
	context.Context,
	workspaceaccess.Request,
) (workspaceaccess.Decision, error) {
	return workspaceaccess.Decision{Allowed: bool(allowed), Source: workspaceaccess.SourceAgentGrant}, nil
}

func TestTerminalReadsShouldApplyTheSharedProfileReadPolicyToAgents(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name    string
		allowed bool
		status  int
		body    string
	}{
		{"Should admit an agent the workspace-access policy admits", true, http.StatusOK, `"profile_id":"profile-b"`},
		{"Should refuse a denied agent with the typed selection conflict", false, http.StatusBadRequest,
			`"code":"profile_selection_conflict"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			manager := &scopeRecordingTerminalManager{infos: []terminalpkg.Info{
				{ID: "term-a", WS: "workspace-a", ProfileID: "profile-b"},
			}}
			handlers := NewBaseHandlers(&BaseHandlerConfig{
				TransportName: "udsapi",
				Terminal:      &terminalProviderStub{Manager: manager},
				Sessions: sessionManagerStub{status: func(context.Context, string) (*session.Info, error) {
					return &session.Info{
						ID: "caller", AgentName: "coder", ProfileID: store.DefaultProfileID,
						WorkspaceID: "workspace-a", State: session.StateActive,
					}, nil
				}},
				Workspaces: workspaceResolveServiceStub{
					resolve: func(_ context.Context, id string) (workspacepkg.ResolvedWorkspace, error) {
						return workspacepkg.ResolvedWorkspace{ID: id, Name: id}, nil
					},
				},
				WorkspaceAccess: terminalAggregatePolicy(tc.allowed),
			})
			router := gin.New()
			router.GET("/api/workspaces/:workspace_id/terminals", handlers.ListTerminals)
			router.GET("/api/workspaces/:workspace_id/terminals/:id", handlers.GetTerminal)
			// Invariant: every terminal read answers an agent's all_profiles through one policy.
			for _, path := range []string{
				"/api/workspaces/workspace-a/terminals?all_profiles=true",
				"/api/workspaces/workspace-a/terminals/term-a?all_profiles=true",
			} {
				request := httptest.NewRequestWithContext(testutil.Context(t), http.MethodGet, path, http.NoBody)
				request.Header.Set(agentidentity.HeaderSessionID, "caller")
				request.Header.Set(agentidentity.HeaderAgent, "coder")
				response := httptest.NewRecorder()
				router.ServeHTTP(response, request)
				if response.Code != tc.status || !strings.Contains(response.Body.String(), tc.body) {
					t.Fatalf("%s status/body = %d/%s, want %d with %s",
						path, response.Code, response.Body.String(), tc.status, tc.body)
				}
			}
		})
	}
}
