//go:build integration

package contract_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

func TestDaemonHealthRouteDecodesIntoCanonicalContract(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(core.RequestIDMiddleware())
	engine.Use(core.ErrorMiddleware())
	core.RegisterRoutes(engine, core.NewHandlers(&core.HandlerConfig{
		TransportName: "integration",
		Daemon: integrationDaemonService{
			health: core.DaemonHealth{
				Ready:    false,
				Degraded: true,
				Details: []core.HealthDetail{{
					Code:     "daemon_not_ready",
					Message:  "replay still in progress",
					Severity: "warning",
				}},
			},
		},
	}))

	request := httptest.NewRequest(http.MethodGet, "/api/daemon/health", http.NoBody)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}

	var payload contract.DaemonHealthResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload.Health.Ready || !payload.Health.Degraded || len(payload.Health.Details) != 1 {
		t.Fatalf("decoded health payload = %#v", payload.Health)
	}
}

func TestRunSnapshotAndStreamDecodeIntoCanonicalContract(t *testing.T) {
	gin.SetMode(gin.TestMode)

	now := time.Date(2026, 4, 20, 14, 0, 0, 0, time.UTC)
	nextCursor := core.StreamCursor{Timestamp: now, Sequence: 3}
	stream := newIntegrationRunStream()
	sendOverflow := make(chan struct{})

	go func() {
		<-sendOverflow
		stream.events <- core.RunStreamItem{
			Overflow: &core.RunStreamOverflow{Reason: "slow consumer"},
		}
		close(stream.events)
		close(stream.errors)
	}()

	engine := gin.New()
	engine.Use(core.RequestIDMiddleware())
	engine.Use(core.ErrorMiddleware())
	core.RegisterRoutes(engine, core.NewHandlers(&core.HandlerConfig{
		TransportName:     "integration",
		HeartbeatInterval: 10 * time.Millisecond,
		Runs: integrationRunService{
			snapshot: core.RunSnapshot{
				Run: core.Run{
					RunID:            "run-1",
					WorkspaceID:      "ws-1",
					Mode:             "task",
					Status:           "running",
					PresentationMode: "stream",
					StartedAt:        now,
				},
				Jobs: []core.RunJobState{{
					Index:     1,
					JobID:     "job-1",
					Status:    "running",
					UpdatedAt: now,
					Summary: &core.RunJobSummary{
						IDE:   "codex",
						Model: "gpt-5.4",
					},
				}},
				Transcript: []core.RunTranscriptMessage{{
					Sequence:  1,
					Stream:    "session",
					Role:      "assistant",
					Content:   "hello",
					Timestamp: now,
				}},
				Usage: kinds.Usage{
					InputTokens:  4,
					OutputTokens: 6,
					TotalTokens:  10,
				},
				Shutdown: &core.RunShutdownState{
					Phase:       "draining",
					Source:      "signal",
					RequestedAt: now,
					DeadlineAt:  now.Add(30 * time.Second),
				},
				NextCursor: &nextCursor,
			},
			openStream: func(context.Context, string, core.StreamCursor) (core.RunStream, error) {
				return stream, nil
			},
		},
	}))

	t.Run("snapshot", func(t *testing.T) {
		t.Parallel()

		request := httptest.NewRequest(http.MethodGet, "/api/runs/run-1/snapshot", http.NoBody)
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)

		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", response.Code)
		}

		var payload contract.RunSnapshotResponse
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		snapshot, err := payload.Decode()
		if err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if len(snapshot.Jobs) != 1 || snapshot.Usage.TotalTokens != 10 || snapshot.Shutdown == nil {
			t.Fatalf("decoded snapshot = %#v", snapshot)
		}
	})

	t.Run("stream", func(t *testing.T) {
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

		framesCh := make(chan integrationSSEFrame, 8)
		errCh := make(chan error, 1)
		go collectIntegrationSSE(response.Body, framesCh, errCh)

		var heartbeat *contract.HeartbeatPayload
		var overflow *contract.OverflowPayload
		timeout := time.NewTimer(2 * time.Second)
		defer timeout.Stop()

		for heartbeat == nil || overflow == nil {
			select {
			case frame, ok := <-framesCh:
				if !ok {
					if err := <-errCh; err != nil {
						t.Fatalf("collectIntegrationSSE() error = %v", err)
					}
					t.Fatalf("stream closed before required frames; heartbeat=%#v overflow=%#v", heartbeat, overflow)
				}
				switch frame.Event {
				case "heartbeat":
					var payload contract.HeartbeatPayload
					if err := json.Unmarshal(frame.Data, &payload); err != nil {
						t.Fatalf("decode heartbeat payload: %v", err)
					}
					heartbeat = &payload
					close(sendOverflow)
				case "overflow":
					var payload contract.OverflowPayload
					if err := json.Unmarshal(frame.Data, &payload); err != nil {
						t.Fatalf("decode overflow payload: %v", err)
					}
					overflow = &payload
				}
			case err := <-errCh:
				if err != nil {
					t.Fatalf("collectIntegrationSSE() error = %v", err)
				}
			case <-timeout.C:
				t.Fatalf("timed out waiting for heartbeat/overflow; heartbeat=%#v overflow=%#v", heartbeat, overflow)
			}
		}

		if heartbeat.RunID != "run-1" || overflow.RunID != "run-1" || overflow.Reason != "slow consumer" {
			t.Fatalf("decoded frames heartbeat=%#v overflow=%#v", heartbeat, overflow)
		}
	})
}

type integrationDaemonService struct {
	health core.DaemonHealth
}

func (s integrationDaemonService) Status(context.Context) (core.DaemonStatus, error) {
	return core.DaemonStatus{}, nil
}

func (s integrationDaemonService) Health(context.Context) (core.DaemonHealth, error) {
	return s.health, nil
}

func (s integrationDaemonService) Metrics(context.Context) (core.MetricsPayload, error) {
	return core.MetricsPayload{}, nil
}

func (s integrationDaemonService) Stop(context.Context, bool) error {
	return nil
}

type integrationRunService struct {
	snapshot   core.RunSnapshot
	openStream func(context.Context, string, core.StreamCursor) (core.RunStream, error)
}

func (s integrationRunService) List(context.Context, core.RunListQuery) ([]core.Run, error) {
	return nil, nil
}

func (s integrationRunService) Get(context.Context, string) (core.Run, error) {
	return core.Run{}, nil
}

func (s integrationRunService) Snapshot(context.Context, string) (core.RunSnapshot, error) {
	return s.snapshot, nil
}

func (s integrationRunService) Events(context.Context, string, core.RunEventPageQuery) (core.RunEventPage, error) {
	return core.RunEventPage{}, nil
}

func (s integrationRunService) OpenStream(
	ctx context.Context,
	runID string,
	after core.StreamCursor,
) (core.RunStream, error) {
	if s.openStream == nil {
		return nil, errors.New("stream factory is required")
	}
	return s.openStream(ctx, runID, after)
}

func (s integrationRunService) Cancel(context.Context, string) error {
	return nil
}

type integrationRunStream struct {
	events chan core.RunStreamItem
	errors chan error
}

func newIntegrationRunStream() *integrationRunStream {
	return &integrationRunStream{
		events: make(chan core.RunStreamItem, 8),
		errors: make(chan error, 1),
	}
}

func (s *integrationRunStream) Events() <-chan core.RunStreamItem {
	return s.events
}

func (s *integrationRunStream) Errors() <-chan error {
	return s.errors
}

func (s *integrationRunStream) Close() error {
	return nil
}

type integrationSSEFrame struct {
	Event string
	Data  []byte
}

func collectIntegrationSSE(body io.Reader, out chan<- integrationSSEFrame, errs chan<- error) {
	defer close(out)

	scanner := bufio.NewScanner(body)
	frame := integrationSSEFrame{}
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "":
			if frame.Event != "" || len(frame.Data) > 0 {
				out <- frame
			}
			frame = integrationSSEFrame{}
		case strings.HasPrefix(line, "event:"):
			frame.Event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			frame.Data = []byte(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	errs <- scanner.Err()
}
