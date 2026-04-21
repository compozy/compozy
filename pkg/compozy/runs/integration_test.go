package runs

import (
	"context"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync"
	"testing"
	"time"

	apiclient "github.com/compozy/compozy/internal/api/client"
	apicore "github.com/compozy/compozy/internal/api/core"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/daemon"
	"github.com/compozy/compozy/pkg/compozy/events"
	"github.com/gin-gonic/gin"
)

func TestOpenAndListUseDaemonBackedHTTPTransport(t *testing.T) {
	base := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
	service := &integrationRunService{
		runs: []apicore.Run{
			{
				RunID:       "run-1",
				Mode:        "exec",
				Status:      "failed",
				StartedAt:   base,
				EndedAt:     timePointer(base.Add(time.Minute)),
				WorkspaceID: "ws-1",
			},
			{
				RunID:       "run-2",
				Mode:        "prd-tasks",
				Status:      "completed",
				StartedAt:   base.Add(time.Hour),
				EndedAt:     timePointer(base.Add(time.Hour + time.Minute)),
				WorkspaceID: "ws-1",
			},
		},
		snapshots: map[string]apicore.RunSnapshot{
			"run-2": {
				Run: apicore.Run{
					RunID:       "run-2",
					Mode:        "prd-tasks",
					Status:      "completed",
					StartedAt:   base.Add(time.Hour),
					EndedAt:     timePointer(base.Add(time.Hour + time.Minute)),
					WorkspaceID: "ws-1",
				},
				Jobs: []apicore.RunJobState{{
					Summary: &apicore.RunJobSummary{
						IDE:   "codex",
						Model: "gpt-5.4",
					},
				}},
			},
		},
		events: map[string]apicore.RunEventPage{
			"run-2": {
				Events: []events.Event{
					{
						SchemaVersion: events.SchemaVersion,
						RunID:         "run-2",
						Seq:           1,
						Timestamp:     base.Add(time.Hour),
						Kind:          events.EventKindRunStarted,
						Payload: []byte(
							`{"workspace_root":"/workspace","artifacts_dir":"/tmp/home/.compozy/runs/run-2","ide":"codex","model":"gpt-5.4"}`,
						),
					},
				},
			},
		},
	}
	withIntegrationDaemonServer(t, service, func(_ int) {
		got, err := List("/workspace", ListOptions{})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(got) != 2 || got[0].RunID != "run-2" || got[1].RunID != "run-1" {
			t.Fatalf("List() = %#v, want run-2 then run-1", got)
		}

		run, err := Open("/workspace", "run-2")
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		summary := run.Summary()
		if summary.WorkspaceRoot != "/workspace" {
			t.Fatalf("Summary().WorkspaceRoot = %q, want /workspace", summary.WorkspaceRoot)
		}
		if summary.IDE != "codex" || summary.Model != "gpt-5.4" {
			t.Fatalf("Summary() IDE/model = %q/%q, want codex/gpt-5.4", summary.IDE, summary.Model)
		}
	})
}

func TestTailAndWatchWorkspaceUseDaemonBackedHTTPTransport(t *testing.T) {
	now := time.Date(2026, 4, 17, 13, 0, 0, 0, time.UTC)
	service := &integrationRunService{
		runs: []apicore.Run{},
		snapshots: map[string]apicore.RunSnapshot{
			"run-tail": {
				Run: apicore.Run{
					RunID:     "run-tail",
					Mode:      "exec",
					Status:    "running",
					StartedAt: now,
				},
				NextCursor: &apicore.StreamCursor{
					Timestamp: now.Add(time.Second),
					Sequence:  1,
				},
			},
		},
		events: map[string]apicore.RunEventPage{
			"run-tail": {
				Events: []events.Event{{
					SchemaVersion: events.SchemaVersion,
					RunID:         "run-tail",
					Seq:           1,
					Timestamp:     now.Add(time.Second),
					Kind:          events.EventKindRunStarted,
				}},
				NextCursor: &apicore.StreamCursor{
					Timestamp: now.Add(time.Second),
					Sequence:  1,
				},
			},
		},
		streamFactory: func(runID string, _ apicore.StreamCursor) apicore.RunStream {
			stream := newIntegrationRunStream()
			go func() {
				stream.events <- apicore.RunStreamItem{
					Event: &events.Event{
						SchemaVersion: events.SchemaVersion,
						RunID:         runID,
						Seq:           2,
						Timestamp:     now.Add(2 * time.Second),
						Kind:          events.EventKindRunCompleted,
					},
				}
				close(stream.events)
			}()
			return stream
		},
		listSequence: [][]apicore.Run{
			nil,
			{{
				RunID:     "run-watch",
				Status:    "running",
				StartedAt: now,
			}},
			{{
				RunID:     "run-watch",
				Status:    "failed",
				StartedAt: now,
			}},
		},
	}
	withIntegrationDaemonServer(t, service, func(_ int) {
		run, err := Open("/workspace", "run-tail")
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		eventsCh, errsCh := run.Tail(ctx, 2)
		got := collectTailEvents(t, eventsCh, errsCh, 1, 2*time.Second)
		if len(got) != 1 || got[0].Seq != 2 {
			t.Fatalf("Tail() = %#v, want live seq 2", got)
		}

		watchCtx, watchCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer watchCancel()
		runEvents, runErrs := WatchWorkspace(watchCtx, "/workspace")
		created := awaitRunEvent(t, runEvents, runErrs, 2*time.Second)
		if created.Kind != RunEventCreated {
			t.Fatalf("created event = %#v, want created", created)
		}
		changed := awaitRunEvent(t, runEvents, runErrs, 2*time.Second)
		if changed.Kind != RunEventStatusChanged || changed.Summary == nil ||
			changed.Summary.Status != publicRunStatusFailed {
			t.Fatalf("changed event = %#v, want failed status change", changed)
		}
	})
}

func TestOpenMatchesInternalClientSnapshotMetadata(t *testing.T) {
	base := time.Date(2026, 4, 17, 14, 0, 0, 0, time.UTC)
	service := &integrationRunService{
		snapshots: map[string]apicore.RunSnapshot{
			"run-compare": {
				Run: apicore.Run{
					RunID:     "run-compare",
					Mode:      "review",
					Status:    "completed",
					StartedAt: base,
					EndedAt:   timePointer(base.Add(2 * time.Minute)),
				},
				Jobs: []apicore.RunJobState{{
					Summary: &apicore.RunJobSummary{
						IDE:   "codex",
						Model: "gpt-5.4",
					},
				}},
			},
		},
		events: map[string]apicore.RunEventPage{
			"run-compare": {
				Events: []events.Event{{
					SchemaVersion: events.SchemaVersion,
					RunID:         "run-compare",
					Seq:           1,
					Timestamp:     base,
					Kind:          events.EventKindRunStarted,
					Payload: []byte(
						`{"workspace_root":"/workspace","artifacts_dir":"/tmp/home/.compozy/runs/run-compare","ide":"codex","model":"gpt-5.4"}`,
					),
				}},
			},
		},
	}

	withIntegrationDaemonServer(t, service, func(port int) {
		client, err := apiclient.New(apiclient.Target{HTTPPort: port})
		if err != nil {
			t.Fatalf("apiclient.New() error = %v", err)
		}

		snapshot, err := client.GetRunSnapshot(context.Background(), "run-compare")
		if err != nil {
			t.Fatalf("client.GetRunSnapshot() error = %v", err)
		}

		run, err := Open("/workspace", "run-compare")
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}

		summary := run.Summary()
		if summary.RunID != snapshot.Run.RunID || summary.Status != snapshot.Run.Status ||
			summary.Mode != snapshot.Run.Mode {
			t.Fatalf("summary = %#v, snapshot run = %#v", summary, snapshot.Run)
		}
		if !summary.StartedAt.Equal(snapshot.Run.StartedAt) {
			t.Fatalf("summary.StartedAt = %v, want %v", summary.StartedAt, snapshot.Run.StartedAt)
		}
		if summary.IDE != "codex" || summary.Model != "gpt-5.4" {
			t.Fatalf("summary IDE/model = %q/%q, want codex/gpt-5.4", summary.IDE, summary.Model)
		}
	})
}

func TestRemoteWatchAndClientStreamSurviveHeartbeatIdlePeriod(t *testing.T) {
	now := time.Date(2026, 4, 17, 15, 0, 0, 0, time.UTC)
	service := &integrationRunService{
		snapshots: map[string]apicore.RunSnapshot{
			"run-heartbeat": {
				Run: apicore.Run{
					RunID:     "run-heartbeat",
					Mode:      "exec",
					Status:    "running",
					StartedAt: now,
				},
			},
		},
		streamFactory: func(runID string, _ apicore.StreamCursor) apicore.RunStream {
			stream := newIntegrationRunStream()
			go func() {
				time.Sleep(40 * time.Millisecond)
				stream.events <- apicore.RunStreamItem{
					Event: &events.Event{
						SchemaVersion: events.SchemaVersion,
						RunID:         runID,
						Seq:           1,
						Timestamp:     now.Add(time.Second),
						Kind:          events.EventKindRunCompleted,
					},
				}
				close(stream.events)
			}()
			return stream
		},
	}

	withIntegrationDaemonServer(t, service, func(port int) {
		client, err := apiclient.New(apiclient.Target{HTTPPort: port})
		if err != nil {
			t.Fatalf("apiclient.New() error = %v", err)
		}

		stream, err := client.OpenRunStream(context.Background(), "run-heartbeat", apicore.StreamCursor{})
		if err != nil {
			t.Fatalf("client.OpenRunStream() error = %v", err)
		}
		clientEvent := awaitClientStreamEvent(t, stream.Items(), stream.Errors(), time.Second)
		if clientEvent.Seq != 1 || clientEvent.Kind != events.EventKindRunCompleted {
			t.Fatalf("client stream event = %#v, want completed seq 1", clientEvent)
		}
		if err := stream.Close(); err != nil {
			t.Fatalf("client stream close error = %v", err)
		}

		reader, err := newDefaultDaemonRunReader()
		if err != nil {
			t.Fatalf("newDefaultDaemonRunReader() error = %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		runEvents, runErrs := WatchRemote(ctx, reader, "run-heartbeat")
		publicEvent := awaitRunEventPayload(t, runEvents, runErrs, time.Second)
		if publicEvent.Seq != 1 || publicEvent.Kind != events.EventKindRunCompleted {
			t.Fatalf("WatchRemote() event = %#v, want completed seq 1", publicEvent)
		}
	})
}

func withIntegrationDaemonServer(t *testing.T, runs apicore.RunService, fn func(port int)) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	apicore.RegisterRoutes(engine, apicore.NewHandlers(&apicore.HandlerConfig{
		TransportName:     "http-test",
		HeartbeatInterval: 10 * time.Millisecond,
		Runs:              runs,
	}))

	server := httptest.NewServer(engine)
	defer server.Close()

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	port, err := strconv.Atoi(serverURL.Port())
	if err != nil {
		t.Fatalf("parse server port: %v", err)
	}

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	paths, err := compozyconfig.ResolveHomePaths()
	if err != nil {
		t.Fatalf("ResolveHomePaths() error = %v", err)
	}
	if err := daemon.WriteInfo(paths.InfoPath, daemon.Info{
		PID:        1,
		HTTPPort:   port,
		StartedAt:  time.Now().UTC(),
		State:      daemon.ReadyStateReady,
		SocketPath: "",
	}); err != nil {
		t.Fatalf("WriteInfo() error = %v", err)
	}

	previous := resolveRunsDaemonReader
	resolveRunsDaemonReader = newDefaultDaemonRunReader
	defer func() {
		resolveRunsDaemonReader = previous
	}()

	fn(port)
}

type integrationRunService struct {
	mu sync.Mutex

	runs          []apicore.Run
	snapshots     map[string]apicore.RunSnapshot
	events        map[string]apicore.RunEventPage
	streamFactory func(string, apicore.StreamCursor) apicore.RunStream
	listSequence  [][]apicore.Run
}

func (s *integrationRunService) List(_ context.Context, _ apicore.RunListQuery) ([]apicore.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.listSequence) > 0 {
		current := append([]apicore.Run(nil), s.listSequence[0]...)
		if len(s.listSequence) > 1 {
			s.listSequence = s.listSequence[1:]
		}
		return current, nil
	}
	return append([]apicore.Run(nil), s.runs...), nil
}

func (s *integrationRunService) Get(_ context.Context, runID string) (apicore.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.runs {
		if s.runs[i].RunID == runID {
			return s.runs[i], nil
		}
	}
	if snapshot, ok := s.snapshots[runID]; ok {
		return snapshot.Run, nil
	}
	return apicore.Run{}, apicore.NewProblem(404, "run_not_found", "run not found", nil, nil)
}

func (s *integrationRunService) Snapshot(_ context.Context, runID string) (apicore.RunSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snapshot, ok := s.snapshots[runID]
	if !ok {
		return apicore.RunSnapshot{}, apicore.NewProblem(404, "run_not_found", "run not found", nil, nil)
	}
	return snapshot, nil
}

func (s *integrationRunService) RunDetail(_ context.Context, runID string) (apicore.RunDetailPayload, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot, ok := s.snapshots[runID]
	if !ok {
		return apicore.RunDetailPayload{}, apicore.NewProblem(404, "run_not_found", "run not found", nil, nil)
	}

	for i := range s.runs {
		if s.runs[i].RunID == runID {
			return apicore.RunDetailPayload{
				Run:      s.runs[i],
				Snapshot: snapshot,
			}, nil
		}
	}

	return apicore.RunDetailPayload{
		Run:      snapshot.Run,
		Snapshot: snapshot,
	}, nil
}

func (s *integrationRunService) Events(
	_ context.Context,
	runID string,
	_ apicore.RunEventPageQuery,
) (apicore.RunEventPage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.events[runID], nil
}

func (s *integrationRunService) OpenStream(
	_ context.Context,
	runID string,
	after apicore.StreamCursor,
) (apicore.RunStream, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.streamFactory == nil {
		stream := newIntegrationRunStream()
		close(stream.events)
		return stream, nil
	}
	return s.streamFactory(runID, after), nil
}

func (*integrationRunService) Cancel(context.Context, string) error {
	return nil
}

type integrationRunStream struct {
	events chan apicore.RunStreamItem
	errs   chan error
}

func newIntegrationRunStream() *integrationRunStream {
	return &integrationRunStream{
		events: make(chan apicore.RunStreamItem, 8),
		errs:   make(chan error, 1),
	}
}

func (s *integrationRunStream) Events() <-chan apicore.RunStreamItem { return s.events }
func (s *integrationRunStream) Errors() <-chan error                 { return s.errs }
func (s *integrationRunStream) Close() error {
	close(s.errs)
	return nil
}

func awaitClientStreamEvent(
	t *testing.T,
	items <-chan apiclient.RunStreamItem,
	errs <-chan error,
	timeout time.Duration,
) events.Event {
	t.Helper()

	deadline := time.After(timeout)
	for {
		select {
		case item, ok := <-items:
			if !ok {
				t.Fatal("client stream closed before delivering an event")
			}
			if item.Event != nil {
				return *item.Event
			}
		case err, ok := <-errs:
			if ok && err != nil {
				t.Fatalf("client stream error = %v", err)
			}
		case <-deadline:
			t.Fatal("timed out waiting for client stream event")
		}
	}
}

func awaitRunEventPayload(
	t *testing.T,
	eventsCh <-chan events.Event,
	errsCh <-chan error,
	timeout time.Duration,
) events.Event {
	t.Helper()

	select {
	case event := <-eventsCh:
		return event
	case err := <-errsCh:
		t.Fatalf("WatchRemote() error = %v", err)
		return events.Event{}
	case <-time.After(timeout):
		t.Fatal("timed out waiting for WatchRemote event")
		return events.Event{}
	}
}
