package core

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/agentidentity"
	profilepkg "github.com/compozy/compozy/internal/profile"
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

// archivedTerminalProfileService knows one archived profile. Resolve refuses it as an
// unavailable selection; the historical lookup still finds it.
type archivedTerminalProfileService struct{ ProfileService }

func (archivedTerminalProfileService) Resolve(
	context.Context,
	profilepkg.ResolveInput,
) (profilepkg.Resolution, error) {
	return profilepkg.Resolution{}, &profilepkg.Error{
		Code: "profile_archived", Message: "profile archived is archived", Cause: profilepkg.ErrArchived,
	}
}

func (archivedTerminalProfileService) GetByName(_ context.Context, name string) (profilepkg.Profile, error) {
	if name != "archived" {
		return profilepkg.Profile{}, profilepkg.ErrNotFound
	}
	return profilepkg.Profile{ID: "profile-archived", Name: "archived", State: profilepkg.StateArchived}, nil
}

func TestTerminalDownloadsShouldKeepRecordingsOfArchivedProfilesReadable(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	journal := &terminalDownloadJournalStub{}
	handlers := NewBaseHandlers(&BaseHandlerConfig{
		TransportName: "udsapi",
		Terminal:      &terminalProviderStub{Manager: terminalDownloadManagerStub{journal: journal}},
		Profiles:      archivedTerminalProfileService{},
	})
	router := gin.New()
	router.GET("/api/workspaces/:workspace_id/terminals/recordings/:id", handlers.DownloadTerminalRecording)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequestWithContext(
		testutil.Context(t),
		http.MethodGet,
		"/api/workspaces/workspace-a/terminals/recordings/recording-a?profile=archived",
		http.NoBody,
	))
	if response.Code != http.StatusOK || response.Body.String() != "asciicast" {
		t.Fatalf("archived recording status/body = %d/%s, want 200/asciicast", response.Code, response.Body.String())
	}
	if journal.recordingScope.ProfileID != "profile-archived" {
		t.Fatalf("recording scope = %#v, want the archived profile", journal.recordingScope)
	}
}

// ownerTerminalProfileService lists two owner profiles for an all-profiles read.
type ownerTerminalProfileService struct{ ProfileService }

func (ownerTerminalProfileService) ListIdentities(context.Context) ([]profilepkg.Profile, error) {
	return []profilepkg.Profile{{ID: store.DefaultProfileID, Name: "default"}, {ID: "profile-b", Name: "b"}}, nil
}

// perProfileTerminalJournal behaves like the User scope's journals: it opens files of one
// profile and refuses the aggregate scope with not-exist.
type perProfileTerminalJournal struct {
	terminalAgentJournalStub
	owner string
}

func (j *perProfileTerminalJournal) Recording(
	_ context.Context, _ string, scope store.ReadScope, id string,
) (*terminalpkg.RecordingRef, io.ReadCloser, error) {
	if scope.AllProfiles || scope.ProfileID != j.owner {
		return nil, nil, os.ErrNotExist
	}
	return &terminalpkg.RecordingRef{ID: id, Bytes: int64(len("asciicast"))},
		io.NopCloser(strings.NewReader("asciicast")), nil
}

func (j *perProfileTerminalJournal) Artifact(
	_ context.Context, _ string, scope store.ReadScope, _ string,
) (io.ReadCloser, error) {
	if scope.AllProfiles || scope.ProfileID != j.owner {
		return nil, os.ErrNotExist
	}
	return io.NopCloser(strings.NewReader("artifact bytes")), nil
}

// expiredTerminalManager lists nothing and reports one profile's tombstoned terminal as expired.
type expiredTerminalManager struct{ scopeRecordingTerminalManager }

func (*expiredTerminalManager) Get(_ context.Context, _, profileID string, _ terminalpkg.ID) (*terminalpkg.Info, error) {
	if profileID == "profile-b" {
		return nil, &terminalpkg.Error{Code: terminalpkg.ErrorCodeExpired, Message: "expired", Err: terminalpkg.ErrExpired}
	}
	return nil, &terminalpkg.Error{Code: terminalpkg.ErrorCodeNotFound, Message: "missing", Err: terminalpkg.ErrNotFound}
}

func TestTerminalAggregateReadsShouldAnswerLikeTheOwnerProfileRead(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	t.Run("Should download a file that only its owner profile's journal can open", func(t *testing.T) {
		t.Parallel()
		journal := &perProfileTerminalJournal{owner: "profile-b"}
		handlers := NewBaseHandlers(&BaseHandlerConfig{
			TransportName: "udsapi",
			Terminal:      &terminalProviderStub{Manager: terminalDownloadManagerStub{journal: journal}},
			Profiles:      ownerTerminalProfileService{},
		})
		router := gin.New()
		router.GET("/api/workspaces/:workspace_id/terminals/recordings/:id", handlers.DownloadTerminalRecording)
		router.GET("/api/workspaces/:workspace_id/terminals/artifacts/:id", handlers.DownloadTerminalArtifact)
		for path, want := range map[string]string{
			"/api/workspaces/@user/terminals/recordings/recording-b?all_profiles=true": "asciicast",
			"/api/workspaces/@user/terminals/artifacts/artifact-b?all_profiles=true":   "artifact bytes",
		} {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequestWithContext(
				testutil.Context(t), http.MethodGet, path, http.NoBody,
			))
			if response.Code != http.StatusOK || response.Body.String() != want {
				t.Fatalf("%s status/body = %d/%s, want 200/%s", path, response.Code, response.Body.String(), want)
			}
		}
	})
	t.Run("Should report an expired terminal as expired, not missing", func(t *testing.T) {
		t.Parallel()
		handlers := NewBaseHandlers(&BaseHandlerConfig{
			TransportName: "udsapi",
			Terminal:      &terminalProviderStub{Manager: &expiredTerminalManager{}},
			Profiles:      ownerTerminalProfileService{},
		})
		router := gin.New()
		router.GET("/api/workspaces/:workspace_id/terminals/:id", handlers.GetTerminal)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequestWithContext(
			testutil.Context(t), http.MethodGet,
			"/api/workspaces/workspace-a/terminals/term-gone?all_profiles=true", http.NoBody,
		))
		if response.Code != http.StatusGone || !strings.Contains(response.Body.String(), `"code":"terminal_expired"`) {
			t.Fatalf("aggregate expired status/body = %d/%s, want 410 terminal_expired", response.Code, response.Body.String())
		}
	})
}
