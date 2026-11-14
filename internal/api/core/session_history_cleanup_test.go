package core_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	apitestutil "github.com/compozy/compozy/internal/api/testutil"
	"github.com/compozy/compozy/internal/sessioncleanup"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/workspaceaccess"
	"github.com/gin-gonic/gin"
)

type historyCleanupStubManager struct {
	apitestutil.StubSessionManager
	previews []sessioncleanup.Request
	starts   []sessioncleanup.Request
	startErr error
}

var _ core.SessionHistoryCleanupManager = (*historyCleanupStubManager)(nil)

func (m *historyCleanupStubManager) PreviewHistoryCleanup(
	_ context.Context,
	request sessioncleanup.Request,
) (sessioncleanup.Plan, error) {
	m.previews = append(m.previews, request)
	return sessioncleanup.Plan{
		CountedAt: historyCleanupNow,
		Counts:    sessioncleanup.Counts{Sessions: 2, Children: 1},
		Threads:   []sessioncleanup.Thread{{RootID: "root"}, {RootID: "other"}},
	}, nil
}

func (m *historyCleanupStubManager) StartHistoryCleanup(
	_ context.Context,
	request sessioncleanup.Request,
) (sessioncleanup.Run, sessioncleanup.Plan, error) {
	m.starts = append(m.starts, request)
	if m.startErr != nil {
		return sessioncleanup.Run{}, sessioncleanup.Plan{}, m.startErr
	}
	return sessioncleanup.Run{
		ID:        "run-1",
		ProfileID: request.Scope.ProfileID,
		Scope:     request.Scope,
		State:     sessioncleanup.RunRunning,
		Total:     3,
	}, sessioncleanup.Plan{
		Counts: sessioncleanup.Counts{Sessions: 2, Children: 1},
	}, nil
}

func (m *historyCleanupStubManager) HistoryCleanupRun(string) (sessioncleanup.Run, bool, error) {
	return sessioncleanup.Run{}, false, nil
}

func (m *historyCleanupStubManager) StopHistoryCleanup(context.Context, string, string) (sessioncleanup.Run, error) {
	return sessioncleanup.Run{}, sessioncleanup.ErrNoActiveRun
}

func performHistoryCleanup(
	t *testing.T,
	manager *historyCleanupStubManager,
	body string,
	withIdentity bool,
) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
		TransportName: "uds-test", Sessions: manager, StreamDone: make(chan struct{}),
		Now: func() time.Time { return historyCleanupNow },
	})
	router.POST("/api/sessions/history-cleanup", handlers.PlanSessionHistoryCleanup)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(
		t.Context(), http.MethodPost, "/api/sessions/history-cleanup", bytes.NewReader([]byte(body)),
	)
	req.Header.Set("Content-Type", "application/json")
	if withIdentity {
		req.Header.Set(agentidentity.HeaderSessionID, "sess-agent")
		req.Header.Set(agentidentity.HeaderAgent, "coder")
	}
	router.ServeHTTP(recorder, req)
	return recorder
}

var historyCleanupNow = time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)

func historyCleanupBody(fields string) string {
	return `{"workspace_id":"` + workspaceaccess.UserSelector + `","delete":{"everything":true}` + fields + `}`
}

func TestBaseHandlersPlanSessionHistoryCleanup(t *testing.T) {
	t.Parallel()

	t.Run("Should preview a dry run without starting a cleanup", func(t *testing.T) {
		t.Parallel()
		manager := &historyCleanupStubManager{}
		recorder := performHistoryCleanup(t, manager, historyCleanupBody(`,"older_than_days":7,"dry_run":true`), false)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
		}
		var response contract.SessionHistoryCleanupPreviewResponse
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode preview: %v", err)
		}
		if response.Plan.Counts.Total != 3 || response.Plan.Threads != 2 || response.Plan.ArchivedBefore == nil {
			t.Fatalf("plan = %+v, want 3 sessions in 2 threads with a cutoff", response.Plan)
		}
		if len(manager.starts) != 0 || len(manager.previews) != 1 {
			t.Fatalf("starts = %d previews = %d, want a preview only", len(manager.starts), len(manager.previews))
		}
		request := manager.previews[0]
		if request.Scope.ProfileID != store.DefaultProfileID ||
			request.Scope.WorkspaceID != workspaceaccess.UserSelector {
			t.Fatalf("scope = %+v, want the default profile in User scope", request.Scope)
		}
		if want := historyCleanupNow.Add(-7 * 24 * time.Hour); !request.ArchivedBefore.Equal(want) {
			t.Fatalf("cutoff = %v, want %v", request.ArchivedBefore, want)
		}
	})

	t.Run("Should report the instant a dry run counted up to and fence a start to it", func(t *testing.T) {
		t.Parallel()
		manager := &historyCleanupStubManager{}
		recorder := performHistoryCleanup(t, manager, historyCleanupBody(`,"dry_run":true`), false)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
		}
		var response contract.SessionHistoryCleanupPreviewResponse
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode preview: %v", err)
		}
		// The daemon's clock, so a start sent with it deletes only what was counted.
		if response.Plan.ArchivedBefore != nil || !response.Plan.CountedAt.Equal(historyCleanupNow) {
			t.Fatalf("plan = %+v, want no cutoff, counted at %v", response.Plan, historyCleanupNow)
		}
		counted := response.Plan.CountedAt.Format(time.RFC3339Nano)
		recorder = performHistoryCleanup(t, manager, historyCleanupBody(`,"counted_at":"`+counted+`"`), false)
		if recorder.Code != http.StatusAccepted {
			t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
		}
		if len(manager.starts) != 1 || manager.starts[0].CountedAt == nil ||
			!manager.starts[0].CountedAt.Equal(historyCleanupNow) {
			t.Fatalf("starts = %+v, want one fenced to the dry run's count", manager.starts)
		}
	})

	t.Run("Should start the cleanup in the background when not a dry run", func(t *testing.T) {
		t.Parallel()
		manager := &historyCleanupStubManager{}
		recorder := performHistoryCleanup(t, manager, historyCleanupBody(""), false)
		if recorder.Code != http.StatusAccepted {
			t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
		}
		var response contract.SessionHistoryCleanupStartResponse
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode start: %v", err)
		}
		if response.Run.ID != "run-1" || response.Run.State != "running" || response.Run.Total != 3 {
			t.Fatalf("run = %+v, want the started run", response.Run)
		}
		if len(manager.starts) != 1 || manager.starts[0].ArchivedBefore != nil {
			t.Fatalf("starts = %+v, want one start over every archived thread", manager.starts)
		}
	})

	t.Run("Should refuse both range fields at once", func(t *testing.T) {
		t.Parallel()
		manager := &historyCleanupStubManager{}
		body := historyCleanupBody(`,"older_than_days":7,"archived_before":"2026-11-01T00:00:00Z"`)
		recorder := performHistoryCleanup(t, manager, body, false)
		if recorder.Code != http.StatusBadRequest ||
			!strings.Contains(recorder.Body.String(), "older_than_days and archived_before") {
			t.Fatalf("status = %d, body = %s, want 400 naming both fields", recorder.Code, recorder.Body.String())
		}
		if len(manager.starts) != 0 {
			t.Fatal("refused request started a cleanup")
		}
	})

	t.Run("Should report a cleanup already running as a conflict", func(t *testing.T) {
		t.Parallel()
		manager := &historyCleanupStubManager{startErr: sessioncleanup.ErrRunActive}
		recorder := performHistoryCleanup(t, manager, historyCleanupBody(""), false)
		if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), "already running") {
			t.Fatalf("status = %d, body = %s, want 409", recorder.Code, recorder.Body.String())
		}
	})

	t.Run("Should keep agents off the operator surface", func(t *testing.T) {
		t.Parallel()
		manager := &historyCleanupStubManager{}
		recorder := performHistoryCleanup(t, manager, historyCleanupBody(""), true)
		if recorder.Code != http.StatusForbidden || !strings.Contains(recorder.Body.String(), "operator surface") {
			t.Fatalf("status = %d, body = %s, want 403", recorder.Code, recorder.Body.String())
		}
		if len(manager.starts) != 0 {
			t.Fatal("agent request started a cleanup")
		}
	})
}
