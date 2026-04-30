package core_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/store/globaldb"
)

func TestNon2xxResponsesIncludeRequestIDAndEnvelope(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	handlers := core.NewHandlers(&core.HandlerConfig{
		TransportName: "test",
		Runs: &fakeRunService{
			getErr: globaldb.ErrRunNotFound,
		},
	})
	engine := gin.New()
	engine.Use(core.RequestIDMiddleware())
	engine.Use(core.ErrorMiddleware())
	core.RegisterRoutes(engine, handlers)

	request := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/api/runs/missing",
		http.NoBody,
	)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}

	requestID := strings.TrimSpace(response.Header().Get(core.HeaderRequestID))
	if requestID == "" {
		t.Fatal("X-Request-Id header = empty, want non-empty")
	}

	var payload core.TransportError
	decodeJSON(t, response.Body.Bytes(), &payload)
	if payload.RequestID != requestID {
		t.Fatalf("payload.RequestID = %q, want %q", payload.RequestID, requestID)
	}
	if payload.Code != "not_found" {
		t.Fatalf("payload.Code = %q, want not_found", payload.Code)
	}
	if payload.Message == "" {
		t.Fatal("payload.Message = empty, want non-empty")
	}
}

func TestStreamRunRejectsInvalidLastEventID(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	handlers := core.NewHandlers(&core.HandlerConfig{
		TransportName: "test",
		Runs:          &fakeRunService{},
	})
	engine := gin.New()
	engine.Use(core.RequestIDMiddleware())
	engine.Use(core.ErrorMiddleware())
	core.RegisterRoutes(engine, handlers)

	request := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/api/runs/run-1/stream",
		http.NoBody,
	)
	request.Header.Set("Last-Event-ID", "bad")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)

	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnprocessableEntity)
	}

	var payload core.TransportError
	decodeJSON(t, response.Body.Bytes(), &payload)
	if payload.Code != "invalid_cursor" {
		t.Fatalf("payload.Code = %q, want invalid_cursor", payload.Code)
	}
	if payload.RequestID == "" {
		t.Fatal("payload.RequestID = empty, want non-empty")
	}
}

func TestStreamRunEmitsHeartbeatAndOverflowFrames(t *testing.T) {
	gin.SetMode(gin.TestMode)

	stream := newFakeRunStream()
	sendOverflow := make(chan struct{})
	go func() {
		<-sendOverflow
		stream.events <- core.RunStreamItem{
			Overflow: &core.RunStreamOverflow{Reason: "slow consumer"},
		}
		close(stream.events)
		close(stream.errors)
	}()

	handlers := core.NewHandlers(&core.HandlerConfig{
		TransportName:     "test",
		HeartbeatInterval: 10 * time.Millisecond,
		Runs: &fakeRunService{
			openStream: func(_ context.Context, _ string, _ core.StreamCursor) (core.RunStream, error) {
				return stream, nil
			},
		},
	})

	engine := gin.New()
	engine.Use(core.RequestIDMiddleware())
	engine.Use(core.ErrorMiddleware())
	core.RegisterRoutes(engine, handlers)

	server := httptest.NewServer(engine)
	defer server.Close()

	request, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		server.URL+"/api/runs/run-1/stream",
		http.NoBody,
	)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	text, err := func() (string, error) {
		defer response.Body.Close()

		linesCh := make(chan string, 32)
		scanErrCh := make(chan error, 1)
		go func() {
			scanner := bufio.NewScanner(response.Body)
			for scanner.Scan() {
				linesCh <- scanner.Text()
			}
			scanErrCh <- scanner.Err()
			close(linesCh)
		}()

		lines := make([]string, 0, 16)
		overflowTriggered := false
		for {
			select {
			case line, ok := <-linesCh:
				if !ok {
					if err := <-scanErrCh; err != nil {
						return "", fmt.Errorf("scanner error: %w", err)
					}
					return strings.Join(lines, "\n"), nil
				}
				lines = append(lines, line)
				if line == "event: "+core.RunHeartbeatSSEEvent && !overflowTriggered {
					overflowTriggered = true
					close(sendOverflow)
				}
			case err := <-scanErrCh:
				if err != nil {
					return "", fmt.Errorf("scanner error: %w", err)
				}
				return strings.Join(lines, "\n"), nil
			case <-time.After(time.Second):
				return "", fmt.Errorf("timeout reading stream; collected lines=%v", lines)
			}
		}
	}()
	if err != nil {
		t.Fatalf("read stream text: %v", err)
	}

	if !strings.Contains(text, "event: "+core.RunHeartbeatSSEEvent) {
		t.Fatalf("stream missing heartbeat frame:\n%s", text)
	}
	if !strings.Contains(text, "event: "+core.RunOverflowSSEEvent) {
		t.Fatalf("stream missing overflow frame:\n%s", text)
	}
}

func TestStreamWorkspaceSocketEmitsEventHeartbeatAndOverflowMessages(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	stream := newFakeWorkspaceEventStream()
	sendOverflow := make(chan struct{})
	overflowOnce := sync.Once{}
	feederCtx, stopFeeder := context.WithCancel(context.Background())
	t.Cleanup(stopFeeder)
	var feederWG sync.WaitGroup
	feederWG.Add(1)
	go func() {
		defer feederWG.Done()
		defer close(stream.events)
		defer close(stream.errors)
		select {
		case stream.events <- core.WorkspaceStreamItem{
			Event: &core.WorkspaceEvent{
				Seq:          42,
				TS:           time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC),
				WorkspaceID:  "workspace-1",
				WorkflowSlug: "demo",
				RunID:        "run-1",
				Mode:         "task",
				Status:       "running",
				Kind:         core.WorkspaceEventKindRunStatusChanged,
			},
		}:
		case <-feederCtx.Done():
			return
		}
		select {
		case <-sendOverflow:
		case <-feederCtx.Done():
			return
		}
		select {
		case stream.events <- core.WorkspaceStreamItem{
			Overflow: &core.WorkspaceStreamOverflow{Reason: "slow consumer"},
		}:
		case <-feederCtx.Done():
			return
		}
	}()
	t.Cleanup(func() {
		stopFeeder()
		feederWG.Wait()
	})

	handlers := core.NewHandlers(&core.HandlerConfig{
		TransportName:     "test",
		HeartbeatInterval: 10 * time.Millisecond,
		WorkspaceEvents: &fakeWorkspaceEventService{
			openStream: func(_ context.Context, workspaceID string) (core.WorkspaceEventStream, error) {
				if workspaceID != "workspace-1" {
					t.Fatalf("workspace id = %q, want workspace-1", workspaceID)
				}
				return stream, nil
			},
		},
	})

	engine := gin.New()
	engine.Use(core.RequestIDMiddleware())
	engine.Use(core.ErrorMiddleware())
	core.RegisterRoutes(engine, handlers)

	server := httptest.NewServer(engine)
	defer server.Close()

	socketURL := strings.Replace(server.URL, "http://", "ws://", 1) + "/api/workspaces/workspace-1/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, resp, err := websocket.Dial(ctx, socketURL, nil)
	if resp != nil && resp.Body != nil {
		t.Cleanup(func() {
			if closeErr := resp.Body.Close(); closeErr != nil {
				t.Errorf("close websocket response body: %v", closeErr)
			}
		})
	}
	if err != nil {
		t.Fatalf("websocket.Dial() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := conn.CloseNow(); closeErr != nil {
			t.Errorf("close websocket client transport: %v", closeErr)
		}
	})

	seen := make(map[string]core.WorkspaceSocketMessage)
	for len(seen) < 3 {
		var message core.WorkspaceSocketMessage
		if err := wsjson.Read(ctx, conn, &message); err != nil {
			t.Fatalf("wsjson.Read() error = %v; seen=%v", err, seen)
		}
		seen[message.Type] = message
		if message.Type == core.WorkspaceHeartbeatSocketType {
			overflowOnce.Do(func() { close(sendOverflow) })
		}
	}

	eventMessage, ok := seen[core.WorkspaceEventSocketType]
	if !ok {
		t.Fatalf("missing workspace event message: %#v", seen)
	}
	if eventMessage.ID != "42" {
		t.Fatalf("workspace event id = %q, want 42", eventMessage.ID)
	}
	var event core.WorkspaceEvent
	decodeJSON(t, eventMessage.Payload, &event)
	if event.Kind != core.WorkspaceEventKindRunStatusChanged || event.RunID != "run-1" {
		t.Fatalf("workspace event payload = %#v", event)
	}

	heartbeatMessage, ok := seen[core.WorkspaceHeartbeatSocketType]
	if !ok {
		t.Fatalf("missing workspace heartbeat message: %#v", seen)
	}
	var heartbeat core.WorkspaceSocketHeartbeatPayload
	decodeJSON(t, heartbeatMessage.Payload, &heartbeat)
	if heartbeat.WorkspaceID != "workspace-1" {
		t.Fatalf("heartbeat workspace id = %q, want workspace-1", heartbeat.WorkspaceID)
	}

	overflowMessage, ok := seen[core.WorkspaceOverflowSocketType]
	if !ok {
		t.Fatalf("missing workspace overflow message: %#v", seen)
	}
	var overflow core.WorkspaceSocketOverflowPayload
	decodeJSON(t, overflowMessage.Payload, &overflow)
	if overflow.Reason != "slow consumer" {
		t.Fatalf("overflow reason = %q, want slow consumer", overflow.Reason)
	}
}

func TestStreamRunLogsCloseErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuffer, nil))
	stream := newFakeRunStream()
	stream.closeErr = errors.New("close failed")
	close(stream.events)
	close(stream.errors)

	handlers := core.NewHandlers(&core.HandlerConfig{
		TransportName: "test",
		Logger:        logger,
		Runs: &fakeRunService{
			openStream: func(_ context.Context, _ string, _ core.StreamCursor) (core.RunStream, error) {
				return stream, nil
			},
		},
	})

	engine := gin.New()
	engine.Use(core.RequestIDMiddleware())
	engine.Use(core.ErrorMiddleware())
	core.RegisterRoutes(engine, handlers)

	request := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/api/runs/run-1/stream",
		http.NoBody,
	)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)

	logs := logBuffer.String()
	if !strings.Contains(logs, "close run stream") {
		t.Fatalf("logs missing close warning:\n%s", logs)
	}
	if !strings.Contains(logs, "run_id=run-1") {
		t.Fatalf("logs missing run id:\n%s", logs)
	}
}

type fakeRunService struct {
	getErr     error
	openStream func(context.Context, string, core.StreamCursor) (core.RunStream, error)
}

func (f *fakeRunService) List(context.Context, core.RunListQuery) ([]core.Run, error) {
	return nil, nil
}

func (f *fakeRunService) Get(context.Context, string) (core.Run, error) {
	return core.Run{}, f.getErr
}

func (f *fakeRunService) Snapshot(context.Context, string) (core.RunSnapshot, error) {
	return core.RunSnapshot{}, nil
}

func (f *fakeRunService) Transcript(context.Context, string) (core.RunTranscript, error) {
	return core.RunTranscript{}, nil
}

func (f *fakeRunService) RunDetail(context.Context, string) (core.RunDetailPayload, error) {
	return core.RunDetailPayload{}, nil
}

func (f *fakeRunService) Events(context.Context, string, core.RunEventPageQuery) (core.RunEventPage, error) {
	return core.RunEventPage{}, nil
}

func (f *fakeRunService) OpenStream(
	ctx context.Context,
	runID string,
	after core.StreamCursor,
) (core.RunStream, error) {
	if f.openStream != nil {
		return f.openStream(ctx, runID, after)
	}
	return newFakeRunStream(), nil
}

func (f *fakeRunService) Cancel(context.Context, string) error {
	return nil
}

type fakeRunStream struct {
	events   chan core.RunStreamItem
	errors   chan error
	closeErr error
}

func newFakeRunStream() *fakeRunStream {
	return &fakeRunStream{
		events: make(chan core.RunStreamItem, 8),
		errors: make(chan error, 1),
	}
}

func (f *fakeRunStream) Events() <-chan core.RunStreamItem {
	return f.events
}

func (f *fakeRunStream) Errors() <-chan error {
	return f.errors
}

func (f *fakeRunStream) Close() error {
	return f.closeErr
}

type fakeWorkspaceEventService struct {
	openStream func(context.Context, string) (core.WorkspaceEventStream, error)
}

func (f *fakeWorkspaceEventService) OpenWorkspaceStream(
	ctx context.Context,
	workspaceID string,
) (core.WorkspaceEventStream, error) {
	if f.openStream != nil {
		return f.openStream(ctx, workspaceID)
	}
	return newFakeWorkspaceEventStream(), nil
}

type fakeWorkspaceEventStream struct {
	events   chan core.WorkspaceStreamItem
	errors   chan error
	closeErr error
}

func newFakeWorkspaceEventStream() *fakeWorkspaceEventStream {
	return &fakeWorkspaceEventStream{
		events: make(chan core.WorkspaceStreamItem, 8),
		errors: make(chan error, 1),
	}
}

func (f *fakeWorkspaceEventStream) Events() <-chan core.WorkspaceStreamItem {
	return f.events
}

func (f *fakeWorkspaceEventStream) Errors() <-chan error {
	return f.errors
}

func (f *fakeWorkspaceEventStream) Close() error {
	return f.closeErr
}

func decodeJSON(t *testing.T, data []byte, dst any) {
	t.Helper()
	if err := json.Unmarshal(data, dst); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
}
