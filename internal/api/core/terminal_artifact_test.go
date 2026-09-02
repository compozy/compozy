package core

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/gin-gonic/gin"
)

func TestTerminalHandlersShouldKeepProfileScopesClosed(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	manager := &scopeRecordingTerminalManager{}
	provider := &terminalProviderStub{Manager: manager}
	handlers := NewBaseHandlers(&BaseHandlerConfig{TransportName: "udsapi", Terminal: provider})
	router := gin.New()
	router.GET("/api/workspaces/:workspace_id/terminals", handlers.ListTerminals)
	router.GET("/api/workspaces/:workspace_id/terminals/:id", handlers.GetTerminal)
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
	if get.Code != http.StatusBadRequest {
		t.Fatalf("single-owner get status = %d, want 400; body=%s", get.Code, get.Body.String())
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
