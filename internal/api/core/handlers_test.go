package core_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
	go func() {
		time.Sleep(30 * time.Millisecond)
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
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	text := string(body)
	if !strings.Contains(text, "event: heartbeat") {
		t.Fatalf("stream missing heartbeat frame:\n%s", text)
	}
	if !strings.Contains(text, "event: overflow") {
		t.Fatalf("stream missing overflow frame:\n%s", text)
	}
	if !strings.Contains(text, `"reason":"slow consumer"`) {
		t.Fatalf("stream missing overflow payload:\n%s", text)
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
	events chan core.RunStreamItem
	errors chan error
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
	return nil
}

func decodeJSON(t *testing.T, data []byte, dst any) {
	t.Helper()
	if err := json.Unmarshal(data, dst); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
}
