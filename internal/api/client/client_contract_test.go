package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	apicore "github.com/compozy/compozy/internal/api/core"
)

type idleSSEBody struct {
	data   []byte
	offset int
	done   chan struct{}
	once   sync.Once
}

func newIdleSSEBody(prefix string) *idleSSEBody {
	return &idleSSEBody{
		data: []byte(prefix),
		done: make(chan struct{}),
	}
}

func (b *idleSSEBody) Read(p []byte) (int, error) {
	if b.offset < len(b.data) {
		written := copy(p, b.data[b.offset:])
		b.offset += written
		return written, nil
	}
	<-b.done
	return 0, io.EOF
}

func (b *idleSSEBody) Close() error {
	b.once.Do(func() {
		close(b.done)
	})
	return nil
}

func TestClientUsesCanonicalTimeoutClassesByRoute(t *testing.T) {
	t.Parallel()

	client := &Client{
		target:  Target{SocketPath: "/tmp/compozy.sock"},
		baseURL: "http://daemon",
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch req.URL.Path {
				case "/api/daemon/health":
					assertApproxDeadline(req.Context(), t, 2*time.Second)
					return jsonResponse(http.StatusOK, `{"health":{"ready":true}}`), nil
				case "/api/tasks/demo/runs":
					assertApproxDeadline(req.Context(), t, 120*time.Second)
					return jsonResponse(http.StatusCreated, `{"run":{"run_id":"task-run-1","mode":"task"}}`), nil
				case "/api/runs/run-1/cancel":
					assertApproxDeadline(req.Context(), t, 30*time.Second)
					return jsonResponse(http.StatusAccepted, `{"accepted":true}`), nil
				default:
					t.Fatalf("unexpected request path: %s", req.URL.Path)
					return nil, nil
				}
			}),
		},
	}

	if _, err := client.Health(context.Background()); err != nil {
		t.Fatalf("Health() error = %v", err)
	}
	if _, err := client.StartTaskRun(context.Background(), "demo", apicore.TaskRunRequest{
		Workspace: "/tmp/workspace",
	}); err != nil {
		t.Fatalf("StartTaskRun() error = %v", err)
	}
	if err := client.CancelRun(context.Background(), "run-1"); err != nil {
		t.Fatalf("CancelRun() error = %v", err)
	}
}

func TestClientRemoteErrorsDecodeCanonicalEnvelopeAndRequestID(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		status   int
		body     string
		invoke   func(*Client) error
		wantCode string
	}{
		{
			name:   "conflict",
			status: http.StatusConflict,
			body:   `{"request_id":"req-conflict","code":"conflict","message":"workflow already running"}`,
			invoke: func(c *Client) error {
				_, err := c.StartTaskRun(
					context.Background(),
					"demo",
					apicore.TaskRunRequest{Workspace: "/tmp/workspace"},
				)
				return err
			},
			wantCode: "conflict",
		},
		{
			name:   "schema too new",
			status: http.StatusConflict,
			body:   `{"request_id":"req-schema","code":"schema_too_new","message":"reader is too old"}`,
			invoke: func(c *Client) error {
				_, err := c.GetRunSnapshot(context.Background(), "run-schema")
				return err
			},
			wantCode: "schema_too_new",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				target:  Target{SocketPath: "/tmp/compozy.sock"},
				baseURL: "http://daemon",
				httpClient: &http.Client{
					Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
						return jsonResponse(tt.status, tt.body), nil
					}),
				},
			}

			err := tt.invoke(client)
			if err == nil {
				t.Fatal("invoke() error = nil, want remote error")
			}

			var remoteErr *RemoteError
			if !errors.As(err, &remoteErr) {
				t.Fatalf("invoke() error = %T, want *RemoteError", err)
			}
			if remoteErr.Envelope.Code != tt.wantCode {
				t.Fatalf("remote code = %q, want %q", remoteErr.Envelope.Code, tt.wantCode)
			}
			if remoteErr.Envelope.RequestID == "" || !strings.Contains(remoteErr.Error(), "request_id=") {
				t.Fatalf("remote error = %v, want preserved request id", remoteErr)
			}
		})
	}
}

func TestGetRunSnapshotPreservesCanonicalFields(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 18, 4, 0, 0, 0, time.UTC)
	wantSnapshot := contract.RunSnapshot{
		Run: contract.Run{
			RunID:     "run-snapshot",
			Status:    "running",
			Mode:      "exec",
			StartedAt: now,
		},
		Jobs: []contract.RunJobState{{
			Summary: &contract.RunJobSummary{
				IDE:   "codex",
				Model: "gpt-5.5",
			},
		}},
		Transcript: []contract.RunTranscriptMessage{{
			Sequence:  1,
			Stream:    "stdout",
			Role:      "assistant",
			Content:   "hello",
			Timestamp: now,
		}},
		Usage: contract.RunSnapshot{}.Usage,
		Shutdown: &contract.RunShutdownState{
			Phase: "draining",
		},
		Incomplete:        true,
		IncompleteReasons: []string{"transcript_gap", "event_gap"},
		NextCursor: &contract.StreamCursor{
			Timestamp: now.Add(time.Second),
			Sequence:  9,
		},
	}
	wantSnapshot.Usage.TotalTokens = 18
	wantSnapshot.Usage.InputTokens = 7
	wantSnapshot.Usage.OutputTokens = 11

	body, err := json.Marshal(contract.RunSnapshotResponseFromSnapshot(wantSnapshot))
	if err != nil {
		t.Fatalf("marshal snapshot response: %v", err)
	}

	client := &Client{
		target:  Target{SocketPath: "/tmp/compozy.sock"},
		baseURL: "http://daemon",
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.Path != "/api/runs/run-snapshot/snapshot" {
					t.Fatalf("path = %s, want /api/runs/run-snapshot/snapshot", req.URL.Path)
				}
				return jsonResponse(http.StatusOK, string(body)), nil
			}),
		},
	}

	got, err := client.GetRunSnapshot(context.Background(), "run-snapshot")
	if err != nil {
		t.Fatalf("GetRunSnapshot() error = %v", err)
	}

	if got.Run.RunID != wantSnapshot.Run.RunID || got.Run.Status != wantSnapshot.Run.Status ||
		got.Run.Mode != wantSnapshot.Run.Mode {
		t.Fatalf("snapshot run = %#v, want %#v", got.Run, wantSnapshot.Run)
	}
	if len(got.Jobs) != 1 || got.Jobs[0].Summary == nil || got.Jobs[0].Summary.IDE != "codex" ||
		got.Jobs[0].Summary.Model != "gpt-5.5" {
		t.Fatalf("snapshot jobs = %#v, want codex/gpt-5.5 summary", got.Jobs)
	}
	if len(got.Transcript) != 1 || got.Transcript[0].Content != "hello" {
		t.Fatalf("snapshot transcript = %#v, want one hello message", got.Transcript)
	}
	if got.Usage.TotalTokens != 18 || got.Usage.InputTokens != 7 || got.Usage.OutputTokens != 11 {
		t.Fatalf("snapshot usage = %#v, want 7/11/18 tokens", got.Usage)
	}
	if got.Shutdown == nil || got.Shutdown.Phase != "draining" {
		t.Fatalf("snapshot shutdown = %#v, want draining", got.Shutdown)
	}
	if !got.Incomplete {
		t.Fatal("snapshot incomplete = false, want true")
	}
	if want := []string{"transcript_gap", "event_gap"}; !reflect.DeepEqual(got.IncompleteReasons, want) {
		t.Fatalf("snapshot incomplete reasons = %#v, want %#v", got.IncompleteReasons, want)
	}
	if got.NextCursor == nil || got.NextCursor.Sequence != 9 || !got.NextCursor.Timestamp.Equal(now.Add(time.Second)) {
		t.Fatalf("snapshot next cursor = %#v, want seq 9", got.NextCursor)
	}
}

func TestGetRunTranscriptPreservesStructuredMessages(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	want := contract.RunTranscript{
		RunID: "run-transcript",
		Messages: []contract.RunUIMessage{{
			ID:   "assistant-1",
			Role: contract.RunUIMessageRoleAssistant,
			Parts: []contract.RunUIMessagePart{{
				Type:  contract.RunUIMessagePartText,
				Text:  "hello",
				State: "done",
			}},
		}},
		Incomplete:        true,
		IncompleteReasons: []string{"transcript_gap"},
		NextCursor: &contract.StreamCursor{
			Timestamp: now,
			Sequence:  12,
		},
	}
	body, err := json.Marshal(contract.RunTranscriptResponseFromTranscript(want))
	if err != nil {
		t.Fatalf("marshal transcript response: %v", err)
	}

	client := &Client{
		target:  Target{SocketPath: "/tmp/compozy.sock"},
		baseURL: "http://daemon",
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.Path != "/api/runs/run-transcript/transcript" {
					t.Fatalf("path = %s, want /api/runs/run-transcript/transcript", req.URL.Path)
				}
				return jsonResponse(http.StatusOK, string(body)), nil
			}),
		},
	}

	got, err := client.GetRunTranscript(context.Background(), "run-transcript")
	if err != nil {
		t.Fatalf("GetRunTranscript() error = %v", err)
	}
	if got.RunID != want.RunID || len(got.Messages) != 1 || got.Messages[0].Parts[0].Text != "hello" {
		t.Fatalf("transcript = %#v, want structured hello message", got)
	}
	if !got.Incomplete || !reflect.DeepEqual(got.IncompleteReasons, []string{"transcript_gap"}) {
		t.Fatalf("transcript integrity = incomplete:%v reasons:%#v", got.Incomplete, got.IncompleteReasons)
	}
	if got.NextCursor == nil || got.NextCursor.Sequence != 12 || !got.NextCursor.Timestamp.Equal(now) {
		t.Fatalf("transcript cursor = %#v, want seq 12", got.NextCursor)
	}
}

func TestOpenRunStreamReconnectsFromLastAcknowledgedCursorAfterHeartbeatGap(t *testing.T) {
	previousGap := streamHeartbeatGapTolerance
	streamHeartbeatGapTolerance = 20 * time.Millisecond
	t.Cleanup(func() {
		streamHeartbeatGapTolerance = previousGap
	})

	initialCursor := apicore.StreamCursor{
		Timestamp: time.Date(2026, 4, 18, 5, 0, 0, 0, time.UTC),
		Sequence:  7,
	}
	heartbeatCursor := contract.StreamCursor{
		Timestamp: initialCursor.Timestamp.Add(time.Second),
		Sequence:  8,
	}

	var requests int
	client := &Client{
		target:  Target{SocketPath: "/tmp/compozy.sock"},
		baseURL: "http://daemon",
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requests++
				switch requests {
				case 1:
					if got := req.Header.Get(
						"Last-Event-ID",
					); got != contract.FormatCursor(
						initialCursor.Timestamp,
						initialCursor.Sequence,
					) {
						t.Fatalf("initial Last-Event-ID = %q, want seq 7 cursor", got)
					}
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body: newIdleSSEBody(strings.Join([]string{
							"event: heartbeat",
							`data: {"cursor":"2026-04-18T05:00:01Z|00000000000000000008","ts":"2026-04-18T05:00:01Z"}`,
							"",
							"",
						}, "\n")),
					}, nil
				case 2:
					if got := req.Header.Get(
						"Last-Event-ID",
					); got != contract.FormatCursor(
						heartbeatCursor.Timestamp,
						heartbeatCursor.Sequence,
					) {
						t.Fatalf("reconnect Last-Event-ID = %q, want seq 8 cursor", got)
					}
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body: newIdleSSEBody(strings.Join([]string{
							`data: {"schema_version":"1.0","run_id":"run-stream","seq":9,"ts":"2026-04-18T05:00:02Z","kind":"run.completed"}`,
							"",
							"",
						}, "\n")),
					}, nil
				default:
					t.Fatalf("unexpected request count %d", requests)
					return nil, nil
				}
			}),
		},
	}

	stream, err := client.OpenRunStream(context.Background(), "run-stream", initialCursor)
	if err != nil {
		t.Fatalf("OpenRunStream() error = %v", err)
	}
	defer func() {
		_ = stream.Close()
	}()

	first := awaitRunStreamItem(t, stream.Items())
	if first.Heartbeat == nil || first.Heartbeat.Cursor.Sequence != 8 {
		t.Fatalf("first item = %#v, want heartbeat seq 8", first)
	}

	second := awaitRunStreamItem(t, stream.Items())
	if second.Event == nil || second.Event.Seq != 9 || second.Event.Kind != "run.completed" {
		t.Fatalf("second item = %#v, want completed event seq 9", second)
	}

	if err := stream.Close(); err != nil {
		t.Fatalf("stream.Close() error = %v", err)
	}

	for err := range stream.Errors() {
		if err != nil {
			t.Fatalf("stream.Errors() unexpected error = %v", err)
		}
	}
	if requests != 2 {
		t.Fatalf("request count = %d, want 2", requests)
	}
}

func assertApproxDeadline(ctx context.Context, t *testing.T, want time.Duration) {
	t.Helper()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatalf("context deadline missing, want %s timeout", want)
	}
	got := time.Until(deadline)
	const tolerance = 750 * time.Millisecond
	if got < want-tolerance || got > want+tolerance {
		t.Fatalf("context deadline = %s from now, want about %s", got, want)
	}
}

func awaitRunStreamItem(t *testing.T, items <-chan RunStreamItem) RunStreamItem {
	t.Helper()

	select {
	case item := <-items:
		return item
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for run stream item")
		return RunStreamItem{}
	}
}
