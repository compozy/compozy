package core_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	apitestutil "github.com/compozy/compozy/internal/api/testutil"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/gin-gonic/gin"
)

type archiveStoppedStubManager struct {
	apitestutil.StubSessionManager
	requests []session.ArchiveStoppedRequest
	report   session.ArchiveStoppedReport
}

var _ core.SessionArchiveStoppedManager = (*archiveStoppedStubManager)(nil)

func (m *archiveStoppedStubManager) ArchiveStopped(
	_ context.Context,
	req session.ArchiveStoppedRequest,
) (session.ArchiveStoppedReport, error) {
	m.requests = append(m.requests, req)
	report := m.report
	report.DryRun = req.DryRun
	return report, nil
}

func archiveStoppedWorkspaces() apitestutil.StubWorkspaceService {
	archivedAt := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	return apitestutil.StubWorkspaceService{GetFn: func(_ context.Context, ref string) (workspacepkg.Workspace, error) {
		switch ref {
		case "platform", "ws-platform":
			return workspacepkg.Workspace{ID: "ws-platform", Name: "platform"}, nil
		case "closed":
			return workspacepkg.Workspace{ID: "ws-closed", Name: "closed", ArchivedAt: &archivedAt}, nil
		default:
			return workspacepkg.Workspace{}, workspacepkg.ErrWorkspaceNotFound
		}
	}}
}

func performArchiveStopped(
	t *testing.T,
	manager *archiveStoppedStubManager,
	body string,
	withIdentity bool,
) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
		TransportName: "uds-test", Sessions: manager, Workspaces: archiveStoppedWorkspaces(),
		StreamDone: make(chan struct{}),
	})
	router.POST("/api/sessions/archive-stopped", handlers.ArchiveStoppedSessions)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(
		t.Context(), http.MethodPost, "/api/sessions/archive-stopped", bytes.NewReader([]byte(body)),
	)
	req.Header.Set("Content-Type", "application/json")
	if withIdentity {
		req.Header.Set(agentidentity.HeaderSessionID, "sess-agent")
		req.Header.Set(agentidentity.HeaderAgent, "coder")
	}
	router.ServeHTTP(recorder, req)
	return recorder
}

func decodeArchiveStoppedError(t *testing.T, recorder *httptest.ResponseRecorder) contract.ErrorPayload {
	t.Helper()
	var payload contract.ErrorPayload
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error payload: %v (%s)", err, recorder.Body.String())
	}
	return payload
}

func TestBaseHandlersArchiveStoppedSessions(t *testing.T) {
	t.Parallel()

	t.Run("Should archive one workspace of the active profile and return the report shape", func(t *testing.T) {
		t.Parallel()
		manager := &archiveStoppedStubManager{report: session.ArchiveStoppedReport{
			Archived: []session.ArchiveStoppedThread{
				{
					RootID:      "sess-root",
					WorkspaceID: "ws-platform",
					Title:       "Done",
					SessionIDs:  []string{"sess-root", "sess-child"},
				},
			},
			SkipCounts: map[session.ArchiveStoppedSkipReason]int{session.ArchiveStoppedSkipWaitingOnYou: 1},
			Skipped: []session.ArchiveStoppedSkip{{
				RootID: "sess-wait", WorkspaceID: "ws-platform", Reason: session.ArchiveStoppedSkipWaitingOnYou,
			}},
		}}
		recorder := performArchiveStopped(t, manager, `{"workspace_id":"platform","dry_run":true}`, false)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
		}
		if len(manager.requests) != 1 {
			t.Fatalf("requests = %d, want 1", len(manager.requests))
		}
		if req := manager.requests[0]; req.ProfileID != store.DefaultProfileID || req.WorkspaceID != "ws-platform" ||
			req.AllWorkspaces || !req.DryRun || req.CallerSessionID != "" {
			t.Fatalf("request = %+v, want the default profile, resolved workspace and dry run", req)
		}
		var response contract.SessionArchiveStoppedResponse
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if !response.DryRun || response.ArchivedThreadCount != 1 || response.ArchivedSessionCount != 2 ||
			response.Archived[0].SessionIDs[0] != "sess-root" || response.SkipCounts["waiting_on_you"] != 1 ||
			len(response.Skipped) != 1 || response.Skipped[0].Reason != "waiting_on_you" ||
			response.Failures == nil || response.SkippedTruncated || response.FailuresTruncated {
			t.Fatalf("response = %+v", response)
		}
	})

	t.Run("Should pass all workspaces and the truncation flags through", func(t *testing.T) {
		t.Parallel()
		skipped := make([]session.ArchiveStoppedSkip, 0, session.ArchiveStoppedListLimit)
		for i := range session.ArchiveStoppedListLimit {
			skipped = append(skipped, session.ArchiveStoppedSkip{
				RootID: "sess-" + strconv.Itoa(i), Reason: session.ArchiveStoppedSkipRunning,
			})
		}
		manager := &archiveStoppedStubManager{report: session.ArchiveStoppedReport{
			SkipCounts:       map[session.ArchiveStoppedSkipReason]int{session.ArchiveStoppedSkipRunning: 150},
			Skipped:          skipped,
			SkippedTruncated: true,
			FailureCount:     120,
		}}
		recorder := performArchiveStopped(t, manager, `{"all_workspaces":true}`, false)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
		}
		if req := manager.requests[0]; !req.AllWorkspaces || req.WorkspaceID != "" || req.DryRun {
			t.Fatalf("request = %+v, want all workspaces", req)
		}
		var response contract.SessionArchiveStoppedResponse
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if response.SkipCounts["running"] != 150 || len(response.Skipped) != 100 || !response.SkippedTruncated ||
			response.Archived == nil || response.ArchivedThreadCount != 0 || response.FailureCount != 120 {
			t.Fatalf("response = %+v, want complete counts and a truncated list", response)
		}
	})

	t.Run("Should refuse a request without exactly one scope", func(t *testing.T) {
		t.Parallel()
		for _, body := range []string{`{}`, `{"workspace_id":"platform","all_workspaces":true}`, `{"workspace_id":`} {
			manager := &archiveStoppedStubManager{}
			recorder := performArchiveStopped(t, manager, body, false)
			if recorder.Code != http.StatusBadRequest ||
				decodeArchiveStoppedError(t, recorder).Code != "invalid_request" {
				t.Fatalf("body %s: status = %d, payload = %s", body, recorder.Code, recorder.Body.String())
			}
			if len(manager.requests) != 0 {
				t.Fatalf("body %s reached the manager", body)
			}
		}
	})

	t.Run("Should report an unknown or archived workspace as not found", func(t *testing.T) {
		t.Parallel()
		for _, ref := range []string{"missing", "closed"} {
			manager := &archiveStoppedStubManager{}
			recorder := performArchiveStopped(t, manager, `{"workspace_id":"`+ref+`"}`, false)
			if recorder.Code != http.StatusNotFound ||
				decodeArchiveStoppedError(t, recorder).Code != "workspace_not_found" {
				t.Fatalf("workspace %s: status = %d, payload = %s", ref, recorder.Code, recorder.Body.String())
			}
			if len(manager.requests) != 0 {
				t.Fatalf("workspace %s reached the manager", ref)
			}
		}
	})

	t.Run("Should refuse an agent caller on the operator surface", func(t *testing.T) {
		t.Parallel()
		manager := &archiveStoppedStubManager{}
		recorder := performArchiveStopped(t, manager, `{"all_workspaces":true}`, true)
		if recorder.Code != http.StatusForbidden || len(manager.requests) != 0 {
			t.Fatalf("status = %d, requests = %d, want 403 before the manager", recorder.Code, len(manager.requests))
		}
	})
}
