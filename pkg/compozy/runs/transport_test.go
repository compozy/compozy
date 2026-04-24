package runs

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	apiclient "github.com/compozy/compozy/internal/api/client"
	apicore "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/pkg/compozy/events"
)

type stubAPIClientRunStream struct {
	items  chan apiclient.RunStreamItem
	errors chan error
	closed bool
}

func (s *stubAPIClientRunStream) Items() <-chan apiclient.RunStreamItem {
	if s == nil {
		return nil
	}
	return s.items
}

func (s *stubAPIClientRunStream) Errors() <-chan error {
	if s == nil {
		return nil
	}
	return s.errors
}

func (s *stubAPIClientRunStream) Close() error {
	if s != nil {
		s.closed = true
	}
	return nil
}

func TestNewDaemonClientAndReaderBootstrap(t *testing.T) {
	socketClient, err := newDaemonClient(daemonInfoRecord{SocketPath: "/tmp/compozy.sock"})
	if err != nil {
		t.Fatalf("newDaemonClient(socket) error = %v", err)
	}
	if target := socketClient.Target(); target.SocketPath != "/tmp/compozy.sock" || target.HTTPPort != 0 {
		t.Fatalf("newDaemonClient(socket) target = %#v", target)
	}

	httpClient, err := newDaemonClient(daemonInfoRecord{HTTPPort: 43123})
	if err != nil {
		t.Fatalf("newDaemonClient(http) error = %v", err)
	}
	if target := httpClient.Target(); target.SocketPath != "" || target.HTTPPort != 43123 {
		t.Fatalf("newDaemonClient(http) target = %#v", target)
	}

	if _, err := newDaemonClient(daemonInfoRecord{}); err == nil {
		t.Fatal("newDaemonClient(invalid) error = nil, want non-nil")
	}

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	infoPath := filepath.Join(homeDir, ".compozy", "daemon", "daemon.json")
	if err := os.MkdirAll(filepath.Dir(infoPath), 0o755); err != nil {
		t.Fatalf("mkdir daemon info dir: %v", err)
	}
	if err := os.WriteFile(infoPath, []byte(`{"http_port":43123}`), 0o600); err != nil {
		t.Fatalf("write daemon info: %v", err)
	}

	reader, err := newDefaultDaemonRunReader()
	if err != nil {
		t.Fatalf("newDefaultDaemonRunReader() error = %v", err)
	}
	defaultReader, ok := reader.(*defaultDaemonRunReader)
	if !ok {
		t.Fatalf("newDefaultDaemonRunReader() type = %T, want *defaultDaemonRunReader", reader)
	}
	if target := defaultReader.daemon.Target(); target.HTTPPort != 43123 {
		t.Fatalf("reader target = %#v, want http port 43123", target)
	}

	t.Setenv("HOME", t.TempDir())
	if _, err := newDefaultDaemonRunReader(); err == nil || !errors.Is(err, ErrDaemonUnavailable) {
		t.Fatalf("newDefaultDaemonRunReader() error = %v, want ErrDaemonUnavailable", err)
	}
}

func TestReadRunsDaemonInfoAndCursorHelpers(t *testing.T) {
	t.Parallel()

	infoPath := filepath.Join(t.TempDir(), "daemon.json")
	if err := os.WriteFile(infoPath, []byte(`{"socket_path":"/tmp/test.sock","http_port":1234}`), 0o600); err != nil {
		t.Fatalf("write daemon info: %v", err)
	}

	info, err := readRunsDaemonInfo(infoPath)
	if err != nil {
		t.Fatalf("readRunsDaemonInfo() error = %v", err)
	}
	if info.SocketPath != "/tmp/test.sock" || info.HTTPPort != 1234 {
		t.Fatalf("readRunsDaemonInfo() = %#v, want socket + port", info)
	}

	cursor := RemoteCursor{
		Timestamp: time.Date(2026, 4, 18, 3, 0, 0, 0, time.UTC),
		Sequence:  7,
	}
	raw := formatRemoteCursor(cursor)
	if raw == "" {
		t.Fatal("formatRemoteCursor() = empty, want encoded cursor")
	}
	parsed, err := parseRemoteCursor(raw)
	if err != nil {
		t.Fatalf("parseRemoteCursor() error = %v", err)
	}
	if parsed != cursor {
		t.Fatalf("parseRemoteCursor() = %#v, want %#v", parsed, cursor)
	}
	if pointer := remoteCursorPointerFromCore(
		&apicore.StreamCursor{Timestamp: cursor.Timestamp, Sequence: cursor.Sequence},
	); pointer == nil ||
		*pointer != cursor {
		t.Fatalf("remoteCursorPointerFromCore() = %#v, want %#v", pointer, cursor)
	}
}

func TestAdaptDaemonClientErrorAndDefaultStatus(t *testing.T) {
	t.Parallel()

	serviceUnavailable := adaptDaemonClientError("open run", &apiclient.RemoteError{
		StatusCode: 503,
		Envelope: apicore.TransportError{
			RequestID: "req-503",
			Code:      "service_unavailable",
			Message:   "daemon warming",
		},
	})
	if !errors.Is(serviceUnavailable, ErrDaemonUnavailable) {
		t.Fatalf("adaptDaemonClientError(503) = %v, want ErrDaemonUnavailable", serviceUnavailable)
	}

	conflict := adaptDaemonClientError("open run", &apiclient.RemoteError{
		StatusCode: 409,
		Envelope: apicore.TransportError{
			RequestID: "req-409",
			Code:      "conflict",
			Message:   "already running",
		},
	})
	if conflict == nil || !strings.Contains(conflict.Error(), "open run") ||
		!strings.Contains(conflict.Error(), "request_id=req-409") {
		t.Fatalf("adaptDaemonClientError(conflict) = %v, want op-prefixed request id", conflict)
	}

	netErr := &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}
	if err := adaptDaemonClientError("dial daemon", netErr); !errors.Is(err, ErrDaemonUnavailable) {
		t.Fatalf("adaptDaemonClientError(net) = %v, want ErrDaemonUnavailable", err)
	}
	if err := adaptDaemonClientError("dial daemon", context.DeadlineExceeded); !errors.Is(err, ErrDaemonUnavailable) {
		t.Fatalf("adaptDaemonClientError(deadline) = %v, want ErrDaemonUnavailable", err)
	}

	plainErr := errors.New("boom")
	if err := adaptDaemonClientError("open run", plainErr); !errors.Is(err, plainErr) {
		t.Fatalf("adaptDaemonClientError(plain) = %v, want original error wrapped", err)
	}

	if got := defaultRunStatus(); got != publicRunStatusRunning {
		t.Fatalf("defaultRunStatus() = %q, want %q", got, publicRunStatusRunning)
	}
}

func TestDaemonRunStreamAdapterConvertsClientItemsAndErrors(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 18, 3, 0, 0, 0, time.UTC)
	inner := &stubAPIClientRunStream{
		items:  make(chan apiclient.RunStreamItem, 4),
		errors: make(chan error, 1),
	}
	inner.items <- apiclient.RunStreamItem{
		Snapshot: &apiclient.RunStreamSnapshot{
			Snapshot: apicore.RunSnapshot{
				Run: apicore.Run{
					RunID:  "run-stream",
					Status: publicRunStatusRunning,
				},
				Incomplete:        true,
				IncompleteReasons: []string{"transcript_gap"},
				NextCursor:        &apicore.StreamCursor{Timestamp: now, Sequence: 3},
			},
		},
	}
	inner.items <- apiclient.RunStreamItem{
		Heartbeat: &apiclient.RunStreamHeartbeat{
			Cursor: apicore.StreamCursor{Timestamp: now, Sequence: 4},
		},
	}
	inner.items <- apiclient.RunStreamItem{
		Overflow: &apiclient.RunStreamOverflow{
			Cursor: apicore.StreamCursor{Timestamp: now.Add(time.Second), Sequence: 5},
			Reason: "lagging",
		},
	}
	inner.items <- apiclient.RunStreamItem{
		Event: &events.Event{
			SchemaVersion: events.SchemaVersion,
			RunID:         "run-stream",
			Seq:           6,
			Timestamp:     now.Add(2 * time.Second),
			Kind:          events.EventKindRunCompleted,
		},
	}
	close(inner.items)
	inner.errors <- errors.New("boom")
	close(inner.errors)

	stream := newDaemonRunStreamAdapter(inner)

	var got []RemoteRunStreamItem
	for item := range stream.Items() {
		got = append(got, item)
	}

	var streamErr error
	for err := range stream.Errors() {
		streamErr = err
	}

	if len(got) != 4 {
		t.Fatalf("stream items = %d, want 4", len(got))
	}
	if got[0].Snapshot == nil || got[0].Snapshot.Status != publicRunStatusRunning {
		t.Fatalf("snapshot item = %#v, want running snapshot", got[0])
	}
	if got[0].Snapshot.NextCursor == nil || got[0].Snapshot.NextCursor.Sequence != 3 {
		t.Fatalf("snapshot cursor = %#v, want seq 3", got[0].Snapshot)
	}
	if !got[0].Snapshot.Incomplete || len(got[0].Snapshot.IncompleteReasons) != 1 {
		t.Fatalf("snapshot completeness = %#v, want incomplete snapshot metadata", got[0].Snapshot)
	}
	if got[1].HeartbeatCursor == nil || got[1].HeartbeatCursor.Sequence != 4 {
		t.Fatalf("heartbeat item = %#v, want cursor seq 4", got[1])
	}
	if got[2].OverflowCursor == nil || got[2].OverflowCursor.Sequence != 5 {
		t.Fatalf("overflow item = %#v, want cursor seq 5", got[2])
	}
	if got[3].Event == nil || got[3].Event.Seq != 6 || got[3].Event.Kind != events.EventKindRunCompleted {
		t.Fatalf("event item = %#v, want completed event seq 6", got[3])
	}
	if streamErr == nil || streamErr.Error() != "boom" {
		t.Fatalf("stream.Errors() = %v, want boom", streamErr)
	}

	if err := stream.Close(); err != nil {
		t.Fatalf("stream.Close() error = %v", err)
	}
	if !inner.closed {
		t.Fatal("stream.Close() did not delegate to inner stream")
	}
}

func TestApplySummaryEventDetailsCoversQueuedAndJobPayloads(t *testing.T) {
	t.Parallel()

	summary := RunSummary{RunID: "run-transport"}
	applySummaryEventDetails(&summary, []events.Event{
		{
			SchemaVersion: events.SchemaVersion,
			RunID:         "run-transport",
			Seq:           1,
			Timestamp:     time.Unix(1, 0).UTC(),
			Kind:          events.EventKindRunQueued,
			Payload:       []byte(`{"workspace_root":"/workspace","ide":"codex","model":"gpt-5.5"}`),
		},
		{
			SchemaVersion: events.SchemaVersion,
			RunID:         "run-transport",
			Seq:           2,
			Timestamp:     time.Unix(2, 0).UTC(),
			Kind:          events.EventKindJobQueued,
			Payload:       []byte(`{"ide":"cursor","model":"gpt-5.5"}`),
		},
		{
			SchemaVersion: events.SchemaVersion,
			RunID:         "run-transport",
			Seq:           3,
			Timestamp:     time.Unix(3, 0).UTC(),
			Kind:          events.EventKindJobStarted,
			Payload:       []byte(`{"ide":"zed","model":"gpt-5.6"}`),
		},
	})

	if summary.WorkspaceRoot != "/workspace" {
		t.Fatalf("summary.WorkspaceRoot = %q, want /workspace", summary.WorkspaceRoot)
	}
	if summary.IDE != "codex" || summary.Model != "gpt-5.5" {
		t.Fatalf("summary IDE/model = %q/%q, want first non-empty codex/gpt-5.5", summary.IDE, summary.Model)
	}
}

func TestRunChannelHelpersRespectContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	eventsCh := make(chan events.Event, 1)
	errsCh := make(chan error, 1)
	if sendRunEvent(ctx, eventsCh, events.Event{}) {
		t.Fatal("sendRunEvent() returned true on canceled context")
	}
	if sendRunError(ctx, errsCh, errors.New("boom")) {
		t.Fatal("sendRunError() returned true on canceled context")
	}

	if !sendRunEvent(context.Background(), eventsCh, events.Event{Seq: 7}) {
		t.Fatal("sendRunEvent() returned false on active context")
	}
	if item := <-eventsCh; item.Seq != 7 {
		t.Fatalf("sendRunEvent() item.Seq = %d, want 7", item.Seq)
	}

	if !sendRunError(context.Background(), errsCh, errors.New("boom")) {
		t.Fatal("sendRunError() returned false on active context")
	}
	if got := (<-errsCh).Error(); got != "boom" {
		t.Fatalf("sendRunError() error = %q, want boom", got)
	}

	var nilCtx context.Context
	if !sendRunEvent(nilCtx, eventsCh, events.Event{Seq: 8}) {
		t.Fatal("sendRunEvent(nil) returned false, want true")
	}
	if item := <-eventsCh; item.Seq != 8 {
		t.Fatalf("sendRunEvent(nil) item.Seq = %d, want 8", item.Seq)
	}
	if !sendRunError(nilCtx, errsCh, nil) {
		t.Fatal("sendRunError(nil, nil) returned false, want true")
	}
}

func TestReplayReportsNilRunAndMissingClient(t *testing.T) {
	t.Parallel()

	var nilRun *Run
	for _, err := range nilRun.Replay(0) {
		if err == nil || !strings.Contains(err.Error(), "nil run") {
			t.Fatalf("nil Run.Replay() error = %v, want nil run error", err)
		}
		break
	}

	run := &Run{summary: RunSummary{RunID: "run-replay-missing-client"}}
	for _, err := range run.Replay(0) {
		if err == nil || !errors.Is(err, ErrDaemonUnavailable) {
			t.Fatalf("missing-client Replay() error = %v, want ErrDaemonUnavailable", err)
		}
		break
	}
}
