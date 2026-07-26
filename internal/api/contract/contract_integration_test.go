//go:build integration

package contract_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/api/testutil"
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
				Ready:               false,
				Degraded:            true,
				UptimeSeconds:       12,
				ActiveRunCount:      1,
				ActiveRunsByMode:    []core.DaemonModeCount{{Mode: "task", Count: 1}},
				WorkspaceCount:      2,
				IntegrityIssueCount: 1,
				Databases: core.DaemonDatabaseDiagnostics{
					GlobalBytes: 100,
					RunDBBytes:  200,
				},
				Reconcile: core.DaemonReconcileDiagnostics{
					ReconciledRuns:     3,
					CrashEventAppended: 2,
					CrashEventMissing:  1,
					LastRunID:          "run-last",
				},
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
	if payload.Health.Ready || !payload.Health.Degraded || payload.Health.IntegrityIssueCount != 1 ||
		len(payload.Health.ActiveRunsByMode) != 1 || len(payload.Health.Details) != 1 {
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
			run: core.Run{
				RunID:            "run-1",
				WorkspaceID:      "ws-1",
				Mode:             "task",
				Status:           "running",
				PresentationMode: "stream",
				StartedAt:        now,
			},
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
						Model: "gpt-5.5",
						Speed: kinds.SpeedFast,
						SpeedResolution: &kinds.SpeedResolution{
							Requested: kinds.SpeedFast,
							Status:    kinds.SpeedResolutionStatusUnsupported,
							Reason:    kinds.SpeedResolutionReasonCapabilityAbsent,
						},
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
				Incomplete:        true,
				IncompleteReasons: []string{"transcript_gap"},
				NextCursor:        &nextCursor,
			},
			openStream: func(context.Context, string, core.StreamCursor) (core.RunStream, error) {
				return stream, nil
			},
		},
	}))

	t.Run("list and detail remain shape compatible", func(t *testing.T) {
		for _, testCase := range []struct {
			path string
			body any
		}{
			{
				path: "/api/runs",
				body: &contract.RunListResponse{},
			},
			{
				path: "/api/runs/run-1",
				body: &contract.RunResponse{},
			},
		} {
			request := httptest.NewRequest(http.MethodGet, testCase.path, http.NoBody)
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("%s status = %d, want 200", testCase.path, response.Code)
			}
			if err := json.Unmarshal(response.Body.Bytes(), testCase.body); err != nil {
				t.Fatalf("%s json.Unmarshal() error = %v", testCase.path, err)
			}
			switch payload := testCase.body.(type) {
			case *contract.RunListResponse:
				if len(payload.Runs) != 1 || payload.Runs[0].RunID != "run-1" {
					t.Fatalf("%s payload = %#v, want run-1 list entry", testCase.path, payload)
				}
			case *contract.RunResponse:
				if payload.Run.RunID != "run-1" {
					t.Fatalf("%s payload = %#v, want run-1 detail", testCase.path, payload)
				}
			}
			if strings.Contains(response.Body.String(), `"speed"`) {
				t.Fatalf(
					"%s response unexpectedly changed run summary shape: %s",
					testCase.path,
					response.Body.String(),
				)
			}
		}
	})

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
		summary := snapshot.Jobs[0].Summary
		if summary == nil || summary.Speed != kinds.SpeedFast || summary.SpeedResolution == nil {
			t.Fatalf("decoded snapshot speed summary = %#v", summary)
		}
		wantResolution := kinds.SpeedResolution{
			Requested: kinds.SpeedFast,
			Status:    kinds.SpeedResolutionStatusUnsupported,
			Reason:    kinds.SpeedResolutionReasonCapabilityAbsent,
		}
		if !reflect.DeepEqual(*summary.SpeedResolution, wantResolution) {
			t.Fatalf("decoded speed resolution = %#v, want %#v", *summary.SpeedResolution, wantResolution)
		}

		var responseJSON map[string]any
		if err := json.Unmarshal(response.Body.Bytes(), &responseJSON); err != nil {
			t.Fatalf("json.Unmarshal(response body) error = %v", err)
		}
		jobs, ok := responseJSON["jobs"].([]any)
		if !ok || len(jobs) != 1 {
			t.Fatalf("response jobs = %#v, want one job", responseJSON["jobs"])
		}
		job, ok := jobs[0].(map[string]any)
		if !ok {
			t.Fatalf("response job = %#v, want object", jobs[0])
		}
		responseSummary, ok := job["summary"].(map[string]any)
		if !ok {
			t.Fatalf("response summary = %#v, want object", job["summary"])
		}
		if got := responseSummary["speed"]; got != "fast" {
			t.Fatalf("response speed = %#v, want fast", got)
		}
		wantResolutionJSON := map[string]any{
			"requested": "fast",
			"status":    "unsupported",
			"reason":    "capability_absent",
		}
		if got := responseSummary["speed_resolution"]; !reflect.DeepEqual(got, wantResolutionJSON) {
			t.Fatalf("response speed_resolution = %#v, want %#v", got, wantResolutionJSON)
		}
		if got, want := snapshot.IncompleteReasons, []string{"transcript_gap"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("decoded snapshot incomplete reasons = %#v, want %#v", got, want)
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

		var heartbeat *contract.HeartbeatPayload
		var overflow *contract.OverflowPayload
		overflowRequested := false
		frames, err := testutil.ReadSSEFramesUntil(response.Body, 2*time.Second, func(frames []testutil.SSEFrame) bool {
			for _, frame := range frames {
				switch frame.Event {
				case core.RunHeartbeatSSEEvent:
					if heartbeat == nil {
						var payload contract.HeartbeatPayload
						if err := json.Unmarshal(frame.Data, &payload); err != nil {
							t.Fatalf("decode heartbeat payload: %v", err)
						}
						heartbeat = &payload
					}
					if !overflowRequested {
						close(sendOverflow)
						overflowRequested = true
					}
				case core.RunOverflowSSEEvent:
					if overflow == nil {
						var payload contract.OverflowPayload
						if err := json.Unmarshal(frame.Data, &payload); err != nil {
							t.Fatalf("decode overflow payload: %v", err)
						}
						overflow = &payload
					}
				}
			}
			return heartbeat != nil && overflow != nil
		})
		if err != nil {
			t.Fatalf("ReadSSEFramesUntil() error = %v", err)
		}
		if heartbeat == nil || overflow == nil {
			t.Fatalf(
				"stream closed before required frames; heartbeat=%#v overflow=%#v frames=%#v",
				heartbeat,
				overflow,
				frames,
			)
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
	run        core.Run
	snapshot   core.RunSnapshot
	openStream func(context.Context, string, core.StreamCursor) (core.RunStream, error)
}

func (s integrationRunService) List(context.Context, core.RunListQuery) ([]core.Run, error) {
	return []core.Run{s.run}, nil
}

func (s integrationRunService) Get(context.Context, string) (core.Run, error) {
	return s.run, nil
}

func (s integrationRunService) Snapshot(context.Context, string) (core.RunSnapshot, error) {
	return s.snapshot, nil
}

func (s integrationRunService) Transcript(context.Context, string) (core.RunTranscript, error) {
	return core.RunTranscript{
		RunID:      s.snapshot.Run.RunID,
		Messages:   []core.RunUIMessage{},
		NextCursor: s.snapshot.NextCursor,
	}, nil
}

func (s integrationRunService) RunDetail(context.Context, string) (core.RunDetailPayload, error) {
	return core.RunDetailPayload{
		Run:      s.run,
		Snapshot: s.snapshot,
	}, nil
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

func (s integrationRunService) PauseRunJob(
	_ context.Context,
	runID string,
	jobID string,
) (core.RunJobControlResponse, error) {
	return core.RunJobControlResponse{
		RunID:  runID,
		JobID:  jobID,
		Status: "pausing",
	}, nil
}

func (s integrationRunService) SendRunJobMessage(
	_ context.Context,
	runID string,
	jobID string,
	_ core.RunJobMessageRequest,
) (core.RunJobControlResponse, error) {
	return core.RunJobControlResponse{
		RunID:  runID,
		JobID:  jobID,
		Status: "resumed",
	}, nil
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
