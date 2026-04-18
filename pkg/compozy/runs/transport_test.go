package runs

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/pkg/compozy/events"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestNewDaemonHTTPClientAndReaderBootstrap(t *testing.T) {
	client, baseURL, err := newDaemonHTTPClient(daemonInfoRecord{SocketPath: "/tmp/compozy.sock"})
	if err != nil {
		t.Fatalf("newDaemonHTTPClient(socket) error = %v", err)
	}
	if client == nil || baseURL != "http://daemon" {
		t.Fatalf("newDaemonHTTPClient(socket) = (%v, %q), want client + daemon base URL", client, baseURL)
	}

	client, baseURL, err = newDaemonHTTPClient(daemonInfoRecord{HTTPPort: 43123})
	if err != nil {
		t.Fatalf("newDaemonHTTPClient(http) error = %v", err)
	}
	if client == nil || baseURL != "http://127.0.0.1:43123" {
		t.Fatalf("newDaemonHTTPClient(http) = (%v, %q), want localhost base URL", client, baseURL)
	}

	if _, _, err := newDaemonHTTPClient(daemonInfoRecord{}); err == nil {
		t.Fatal("newDaemonHTTPClient(invalid) error = nil, want non-nil")
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
	if defaultReader.baseURL != "http://127.0.0.1:43123" {
		t.Fatalf("reader.baseURL = %q, want http://127.0.0.1:43123", defaultReader.baseURL)
	}

	t.Setenv("HOME", t.TempDir())
	if _, err := newDefaultDaemonRunReader(); err == nil || !errors.Is(err, ErrDaemonUnavailable) {
		t.Fatalf("newDefaultDaemonRunReader() error = %v, want ErrDaemonUnavailable", err)
	}
}

func TestReadRunsDaemonInfoAndRequestPathValidation(t *testing.T) {
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

	reader := &defaultDaemonRunReader{baseURL: "http://127.0.0.1:43123"}
	request, err := reader.newRequest(context.Background(), "/api/runs?limit=2")
	if err != nil {
		t.Fatalf("newRequest() error = %v", err)
	}
	if request.URL.Path != "/api/runs" || request.URL.RawQuery != "limit=2" {
		t.Fatalf("request URL = %q?%q, want /api/runs?limit=2", request.URL.Path, request.URL.RawQuery)
	}

	if _, err := reader.newRequest(context.Background(), "http://example.com/evil"); err == nil {
		t.Fatal("newRequest() accepted non-daemon path")
	}
}

func TestDecodeTransportErrorAndWrapDaemonRequestError(t *testing.T) {
	t.Parallel()

	reader := &defaultDaemonRunReader{}
	err := reader.decodeTransportError(
		"/api/runs",
		http.StatusServiceUnavailable,
		[]byte(`{"error":{"message":"daemon warming"}}`),
	)
	if !errors.Is(err, ErrDaemonUnavailable) {
		t.Fatalf("decodeTransportError(503) error = %v, want ErrDaemonUnavailable", err)
	}

	err = reader.decodeTransportError(
		"/api/runs",
		http.StatusBadRequest,
		[]byte(`{"error":{"code":"bad_request","message":"invalid filter"}}`),
	)
	if err == nil || !strings.Contains(err.Error(), "invalid filter") {
		t.Fatalf("decodeTransportError(400) error = %v, want message", err)
	}

	err = reader.decodeTransportError("/api/runs", http.StatusBadGateway, []byte(`not-json`))
	if err == nil || !strings.Contains(err.Error(), "status 502") {
		t.Fatalf("decodeTransportError(invalid) error = %v, want status fallback", err)
	}

	netErr := &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}
	if err := wrapDaemonRequestError("dial daemon", netErr); !errors.Is(err, ErrDaemonUnavailable) {
		t.Fatalf("wrapDaemonRequestError(net) error = %v, want ErrDaemonUnavailable", err)
	}
	if err := wrapDaemonRequestError("dial daemon", context.DeadlineExceeded); !errors.Is(err, ErrDaemonUnavailable) {
		t.Fatalf("wrapDaemonRequestError(deadline) error = %v, want ErrDaemonUnavailable", err)
	}

	plainErr := errors.New("boom")
	if err := wrapDaemonRequestError("dial daemon", plainErr); !errors.Is(err, plainErr) {
		t.Fatalf("wrapDaemonRequestError(plain) error = %v, want original error", err)
	}

	if got := defaultRunStatus(); got != publicRunStatusRunning {
		t.Fatalf("defaultRunStatus() = %q, want %q", got, publicRunStatusRunning)
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
			Payload:       []byte(`{"workspace_root":"/workspace","ide":"codex","model":"gpt-5.4"}`),
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
	if summary.IDE != "codex" || summary.Model != "gpt-5.4" {
		t.Fatalf("summary IDE/model = %q/%q, want first non-empty codex/gpt-5.4", summary.IDE, summary.Model)
	}
}

func TestOpenRunStreamParsesHeartbeatOverflowAndEvents(t *testing.T) {
	t.Parallel()

	cursorTS := time.Date(2026, 4, 18, 3, 0, 0, 0, time.UTC)
	reader := &defaultDaemonRunReader{
		baseURL: "http://daemon",
		httpClient: &http.Client{
			Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				if got := request.Header.Get("Last-Event-ID"); got != formatRemoteCursor(RemoteCursor{
					Timestamp: cursorTS,
					Sequence:  3,
				}) {
					t.Fatalf("Last-Event-ID = %q, want resume cursor", got)
				}
				body := strings.Join([]string{
					"event: heartbeat",
					`data: {"cursor":"2026-04-18T03:00:01Z|4","ts":"2026-04-18T03:00:01Z"}`,
					"",
					"event: overflow",
					`data: {"cursor":"2026-04-18T03:00:02Z|5","reason":"lagging","ts":"2026-04-18T03:00:02Z"}`,
					"",
					`data: {"schema_version":"1.0","run_id":"run-stream","seq":6,"ts":"2026-04-18T03:00:03Z","kind":"run.completed"}`,
					"",
				}, "\n")
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     make(http.Header),
				}, nil
			}),
		},
	}

	stream, err := reader.OpenRunStream(context.Background(), "run-stream", RemoteCursor{
		Timestamp: cursorTS,
		Sequence:  3,
	})
	if err != nil {
		t.Fatalf("OpenRunStream() error = %v", err)
	}
	defer func() {
		_ = stream.Close()
	}()

	var got []RemoteRunStreamItem
	for item := range stream.Items() {
		got = append(got, item)
	}
	for err := range stream.Errors() {
		if err != nil {
			t.Fatalf("stream.Errors() unexpected error = %v", err)
		}
	}

	if len(got) != 3 {
		t.Fatalf("stream items = %d, want 3", len(got))
	}
	if got[0].HeartbeatCursor == nil || got[0].HeartbeatCursor.Sequence != 4 {
		t.Fatalf("heartbeat item = %#v, want cursor seq 4", got[0])
	}
	if got[1].OverflowCursor == nil || got[1].OverflowCursor.Sequence != 5 {
		t.Fatalf("overflow item = %#v, want cursor seq 5", got[1])
	}
	if got[2].Event == nil || got[2].Event.Seq != 6 || got[2].Event.Kind != events.EventKindRunCompleted {
		t.Fatalf("event item = %#v, want completed event seq 6", got[2])
	}
}

func TestOpenRunStreamAndSendHelpersHandleFailures(t *testing.T) {
	t.Parallel()

	reader := &defaultDaemonRunReader{
		baseURL: "http://daemon",
		httpClient: &http.Client{
			Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusServiceUnavailable,
					Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"daemon unavailable"}}`)),
					Header:     make(http.Header),
				}, nil
			}),
		},
	}
	stream, err := reader.OpenRunStream(context.Background(), "run-stream", RemoteCursor{})
	if stream != nil {
		t.Fatal("OpenRunStream() stream != nil on failure")
	}
	if err == nil || !errors.Is(err, ErrDaemonUnavailable) {
		t.Fatalf("OpenRunStream() error = %v, want ErrDaemonUnavailable", err)
	}

	testStream := &daemonRunStream{
		items:  make(chan RemoteRunStreamItem, 1),
		errors: make(chan error, 1),
	}
	testStream.items <- RemoteRunStreamItem{}
	if err := testStream.sendItem(RemoteRunStreamItem{}); err == nil {
		t.Fatal("sendItem(full buffer) error = nil, want non-nil")
	}

	testStream.errors <- errors.New("existing")
	testStream.sendError(errors.New("dropped"))
	if got := (<-testStream.errors).Error(); got != "existing" {
		t.Fatalf("sendError() replaced buffered error with %q", got)
	}

	if err := testStream.dispatchStreamError(
		[]byte(`{"error":{"message":"boom"}}`),
	); err == nil ||
		err.Error() != "boom" {
		t.Fatalf("dispatchStreamError() error = %v, want boom", err)
	}
	if got := (daemonRunStreamError{message: " trimmed "}).Error(); got != "trimmed" {
		t.Fatalf("daemonRunStreamError.Error() = %q, want trimmed", got)
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

func TestSortRunSummariesOrdersNewestThenRunID(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 18, 4, 0, 0, 0, time.UTC)
	items := []RunSummary{
		{RunID: "run-a", StartedAt: now.Add(-time.Minute)},
		{RunID: "run-c", StartedAt: now},
		{RunID: "run-b", StartedAt: now},
	}

	sortRunSummaries(items)
	if got := []string{items[0].RunID, items[1].RunID, items[2].RunID}; strings.Join(got, ",") != "run-c,run-b,run-a" {
		t.Fatalf("sorted run ids = %v, want [run-c run-b run-a]", got)
	}
}

func TestDoJSONHandlesSuccessDecodeAndStatusErrors(t *testing.T) {
	t.Parallel()

	reader := &defaultDaemonRunReader{
		baseURL: "http://daemon",
		httpClient: &http.Client{
			Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				switch request.URL.Path {
				case "/api/runs":
					return &http.Response{
						StatusCode: http.StatusOK,
						Body: io.NopCloser(
							strings.NewReader(
								`{"runs":[{"run_id":"run-1","status":"running","started_at":"2026-04-18T04:00:00Z"}]}`,
							),
						),
						Header: make(http.Header),
					}, nil
				case "/api/bad-json":
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader(`not-json`)),
						Header:     make(http.Header),
					}, nil
				default:
					return &http.Response{
						StatusCode: http.StatusServiceUnavailable,
						Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"offline"}}`)),
						Header:     make(http.Header),
					}, nil
				}
			}),
		},
	}

	var okPayload struct {
		Runs []daemonRunPayload `json:"runs"`
	}
	if err := reader.doJSON(context.Background(), "/api/runs", &okPayload); err != nil {
		t.Fatalf("doJSON(success) error = %v", err)
	}
	if len(okPayload.Runs) != 1 || okPayload.Runs[0].RunID != "run-1" {
		t.Fatalf("doJSON(success) payload = %#v, want one run", okPayload)
	}

	if err := reader.doJSON(
		context.Background(),
		"/api/bad-json",
		&okPayload,
	); err == nil ||
		!strings.Contains(err.Error(), "decode daemon response") {
		t.Fatalf("doJSON(bad-json) error = %v, want decode error", err)
	}

	if err := reader.doJSON(
		context.Background(),
		"/api/unavailable",
		&okPayload,
	); err == nil ||
		!errors.Is(err, ErrDaemonUnavailable) {
		t.Fatalf("doJSON(status) error = %v, want ErrDaemonUnavailable", err)
	}
}
