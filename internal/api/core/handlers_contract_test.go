package core_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/api/testutil"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/pkg/compozy/events"
)

func TestDaemonHealthReturnsCanonicalEnvelopeForReadyAndDegradedStates(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	testCases := []struct {
		name       string
		health     core.DaemonHealth
		wantStatus int
	}{
		{
			name: "degraded",
			health: core.DaemonHealth{
				Ready:    false,
				Degraded: true,
				Details: []core.HealthDetail{{
					Code:     "daemon_not_ready",
					Message:  "daemon still reconciling",
					Severity: "warning",
				}},
			},
			wantStatus: http.StatusServiceUnavailable,
		},
		{
			name: "ready",
			health: core.DaemonHealth{
				Ready: true,
				Details: []core.HealthDetail{{
					Code:    "healthy",
					Message: "daemon is ready",
				}},
			},
			wantStatus: http.StatusOK,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			engine := newCanonicalHandlersEngine(core.NewHandlers(&core.HandlerConfig{
				TransportName: "test",
				Daemon: &smokeDaemonService{
					health: tc.health,
				},
			}))

			request := httptest.NewRequestWithContext(
				context.Background(),
				http.MethodGet,
				"/api/daemon/health",
				http.NoBody,
			)
			request.Header.Set(core.HeaderRequestID, "req-health")
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, request)

			if response.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, tc.wantStatus, response.Body.String())
			}
			if got := strings.TrimSpace(response.Header().Get(core.HeaderRequestID)); got != "req-health" {
				t.Fatalf("X-Request-Id = %q, want req-health", got)
			}

			var payload contract.DaemonHealthResponse
			decodeJSON(t, response.Body.Bytes(), &payload)
			if !reflect.DeepEqual(payload.Health, tc.health) {
				t.Fatalf("payload.Health = %#v, want %#v", payload.Health, tc.health)
			}
		})
	}
}

func TestRunStartEndpointsReturnCanonicalRunEnvelopes(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	now := time.Date(2026, 4, 20, 18, 0, 0, 0, time.UTC)
	taskRun := core.Run{
		RunID:            "task-run-1",
		WorkspaceID:      "ws-1",
		WorkflowSlug:     "daemon",
		Mode:             "task",
		Status:           "running",
		PresentationMode: "stream",
		StartedAt:        now,
		RequestID:        "run-req-task",
	}
	reviewRun := core.Run{
		RunID:            "review-run-1",
		WorkspaceID:      "ws-1",
		WorkflowSlug:     "daemon",
		Mode:             "review",
		Status:           "running",
		PresentationMode: "stream",
		StartedAt:        now,
		RequestID:        "run-req-review",
	}
	reviewWatchRun := core.Run{
		RunID:            "review-watch-1",
		WorkspaceID:      "ws-1",
		WorkflowSlug:     "daemon",
		Mode:             "review_watch",
		Status:           "running",
		PresentationMode: "stream",
		StartedAt:        now,
		RequestID:        "run-req-review-watch",
	}

	engine := newCanonicalHandlersEngine(core.NewHandlers(&core.HandlerConfig{
		TransportName: "test",
		Tasks: &smokeTaskService{
			run: taskRun,
		},
		Reviews: &smokeReviewService{
			run:      reviewRun,
			watchRun: reviewWatchRun,
		},
	}))

	testCases := []struct {
		name       string
		target     string
		body       string
		requestID  string
		wantStatus int
		wantRun    core.Run
	}{
		{
			name:       "task run",
			target:     "/api/tasks/daemon/runs",
			body:       `{"workspace":"ws-1","presentation_mode":"stream"}`,
			requestID:  "req-task",
			wantStatus: http.StatusCreated,
			wantRun:    taskRun,
		},
		{
			name:       "review run",
			target:     "/api/reviews/daemon/rounds/1/runs",
			body:       `{"workspace":"ws-1","presentation_mode":"stream"}`,
			requestID:  "req-review",
			wantStatus: http.StatusCreated,
			wantRun:    reviewRun,
		},
		{
			name:       "review watch",
			target:     "/api/reviews/daemon/watch",
			body:       `{"workspace":"ws-1","provider":"coderabbit","pr_ref":"85","auto_push":true}`,
			requestID:  "req-review-watch",
			wantStatus: http.StatusCreated,
			wantRun:    reviewWatchRun,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			request := httptest.NewRequestWithContext(
				context.Background(),
				http.MethodPost,
				tc.target,
				strings.NewReader(tc.body),
			)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set(core.HeaderRequestID, tc.requestID)

			response := httptest.NewRecorder()
			engine.ServeHTTP(response, request)

			if response.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, tc.wantStatus, response.Body.String())
			}
			if got := strings.TrimSpace(response.Header().Get(core.HeaderRequestID)); got != tc.requestID {
				t.Fatalf("X-Request-Id = %q, want %q", got, tc.requestID)
			}

			var payload contract.RunResponse
			decodeJSON(t, response.Body.Bytes(), &payload)
			if payload.Run != tc.wantRun {
				t.Fatalf("payload.Run = %#v, want %#v", payload.Run, tc.wantRun)
			}
		})
	}
}

func TestStreamRunEmitsCanonicalEventHeartbeatAndOverflowPayloads(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	stream := newFakeRunStream()
	now := time.Date(2026, 4, 20, 18, 30, 0, 0, time.UTC)
	runID := "run-1"
	sendOverflow := make(chan struct{})
	var overflowOnce sync.Once
	producerCtx, cancelProducer := context.WithCancel(context.Background())
	defer cancelProducer()

	go func() {
		event := events.Event{
			SchemaVersion: events.SchemaVersion,
			RunID:         runID,
			Seq:           7,
			Timestamp:     now,
			Kind:          events.EventKindSessionUpdate,
			Payload:       json.RawMessage(`{"delta":"hello"}`),
		}
		stream.events <- core.RunStreamItem{Event: &event}
		select {
		case <-sendOverflow:
		case <-producerCtx.Done():
			close(stream.events)
			close(stream.errors)
			return
		}
		stream.events <- core.RunStreamItem{Overflow: &core.RunStreamOverflow{Reason: "slow consumer"}}
		close(stream.events)
		close(stream.errors)
	}()

	engine := newCanonicalHandlersEngine(core.NewHandlers(&core.HandlerConfig{
		TransportName:     "test",
		HeartbeatInterval: 5 * time.Millisecond,
		Now: func() time.Time {
			return now.Add(2 * time.Second)
		},
		Runs: &fakeRunService{
			openStream: func(context.Context, string, core.StreamCursor) (core.RunStream, error) {
				return stream, nil
			},
		},
	}))

	server := httptest.NewServer(engine)
	defer server.Close()

	request, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		server.URL+"/api/runs/"+runID+"/stream",
		http.NoBody,
	)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	request.Header.Set(core.HeaderRequestID, "req-stream")

	client := server.Client()
	client.Timeout = 5 * time.Second

	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer response.Body.Close()

	frames, err := testutil.ReadSSEFramesUntil(response.Body, 2*time.Second, func(items []testutil.SSEFrame) bool {
		for _, item := range items {
			if item.Event == core.RunHeartbeatSSEEvent {
				overflowOnce.Do(func() { close(sendOverflow) })
			}
		}
		return hasSSEFrame(items, core.RunOverflowSSEEvent)
	})
	if err != nil {
		t.Fatalf("ReadSSEFramesUntil() error = %v", err)
	}

	eventFrame, ok := firstSSEFrame(frames, core.RunEventSSEEvent)
	if !ok {
		t.Fatalf("stream frames missing %q: %#v", core.RunEventSSEEvent, frames)
	}
	heartbeatFrame, ok := firstSSEFrame(frames, core.RunHeartbeatSSEEvent)
	if !ok {
		t.Fatalf("stream frames missing %q: %#v", core.RunHeartbeatSSEEvent, frames)
	}
	overflowFrame, ok := firstSSEFrame(frames, core.RunOverflowSSEEvent)
	if !ok {
		t.Fatalf("stream frames missing %q: %#v", core.RunOverflowSSEEvent, frames)
	}

	var eventPayload events.Event
	if err := json.Unmarshal(eventFrame.Data, &eventPayload); err != nil {
		t.Fatalf("json.Unmarshal(event) error = %v", err)
	}
	if eventPayload.Kind != events.EventKindSessionUpdate || eventPayload.Seq != 7 || eventPayload.RunID != runID {
		t.Fatalf("event payload = %#v", eventPayload)
	}
	if eventFrame.ID != core.FormatCursor(now, 7) {
		t.Fatalf("event frame ID = %q, want %q", eventFrame.ID, core.FormatCursor(now, 7))
	}

	var heartbeatPayload contract.HeartbeatPayload
	if err := json.Unmarshal(heartbeatFrame.Data, &heartbeatPayload); err != nil {
		t.Fatalf("json.Unmarshal(heartbeat) error = %v", err)
	}
	if heartbeatPayload.RunID != runID || heartbeatPayload.Cursor != core.FormatCursor(now, 7) {
		t.Fatalf("heartbeat payload = %#v", heartbeatPayload)
	}

	var overflowPayload contract.OverflowPayload
	if err := json.Unmarshal(overflowFrame.Data, &overflowPayload); err != nil {
		t.Fatalf("json.Unmarshal(overflow) error = %v", err)
	}
	if overflowPayload.RunID != runID ||
		overflowPayload.Cursor != core.FormatCursor(now, 7) ||
		overflowPayload.Reason != "slow consumer" {
		t.Fatalf("overflow payload = %#v", overflowPayload)
	}
}

func TestTransportErrorsUseCanonicalCodeAndRequestIDFields(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	archiveConflict := globaldb.WorkflowActiveRunsError{
		WorkspaceID: "ws-1",
		WorkflowID:  "wf-1",
		Slug:        "daemon",
		ActiveRuns:  1,
	}

	engine := newCanonicalHandlersEngine(core.NewHandlers(&core.HandlerConfig{
		TransportName: "test",
		Runs: &fakeRunService{
			getErr: globaldb.ErrRunNotFound,
		},
		Tasks: &errorTaskService{err: archiveConflict},
	}))

	testCases := []struct {
		name       string
		method     string
		target     string
		body       string
		requestID  string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "validation",
			method:     http.MethodGet,
			target:     "/api/runs/run-1/events?after=bad",
			requestID:  "req-validation",
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "invalid_cursor",
		},
		{
			name:       "not found",
			method:     http.MethodGet,
			target:     "/api/runs/missing",
			requestID:  "req-not-found",
			wantStatus: http.StatusNotFound,
			wantCode:   string(contract.CodeNotFound),
		},
		{
			name:       "conflict",
			method:     http.MethodPost,
			target:     "/api/tasks/daemon/archive",
			body:       `{"workspace":"ws-1"}`,
			requestID:  "req-conflict",
			wantStatus: http.StatusConflict,
			wantCode:   string(contract.CodeConflict),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			request := httptest.NewRequestWithContext(
				context.Background(),
				tc.method,
				tc.target,
				strings.NewReader(tc.body),
			)
			if tc.body != "" {
				request.Header.Set("Content-Type", "application/json")
			}
			request.Header.Set(core.HeaderRequestID, tc.requestID)

			response := httptest.NewRecorder()
			engine.ServeHTTP(response, request)

			if response.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, tc.wantStatus, response.Body.String())
			}
			if got := strings.TrimSpace(response.Header().Get(core.HeaderRequestID)); got != tc.requestID {
				t.Fatalf("X-Request-Id = %q, want %q", got, tc.requestID)
			}

			var payload contract.TransportError
			decodeJSON(t, response.Body.Bytes(), &payload)
			if payload.Code != tc.wantCode {
				t.Fatalf("payload.Code = %q, want %q", payload.Code, tc.wantCode)
			}
			if payload.RequestID != tc.requestID {
				t.Fatalf("payload.RequestID = %q, want %q", payload.RequestID, tc.requestID)
			}
		})
	}
}

func newCanonicalHandlersEngine(handlers *core.Handlers) *gin.Engine {
	engine := gin.New()
	engine.Use(core.RequestIDMiddleware())
	engine.Use(core.ErrorMiddleware())
	core.RegisterRoutes(engine, handlers)
	return engine
}
func hasSSEFrame(frames []testutil.SSEFrame, event string) bool {
	for _, frame := range frames {
		if frame.Event == event {
			return true
		}
	}
	return false
}

func firstSSEFrame(frames []testutil.SSEFrame, event string) (testutil.SSEFrame, bool) {
	for _, frame := range frames {
		if frame.Event == event {
			return frame, true
		}
	}
	return testutil.SSEFrame{}, false
}
