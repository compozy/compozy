package contract_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

func TestRouteInventoryMatchesCoreRouteGraph(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	engine := gin.New()
	core.RegisterRoutes(engine, core.NewHandlers(&core.HandlerConfig{TransportName: "test"}))

	actual := make(map[string]struct{}, len(engine.Routes()))
	for _, route := range engine.Routes() {
		actual[route.Method+" "+route.Path] = struct{}{}
	}

	expected := make(map[string]struct{}, len(contract.RouteInventory))
	for _, route := range contract.RouteInventory {
		expected[route.Method+" "+route.Path] = struct{}{}
	}

	for key := range expected {
		if _, ok := actual[key]; !ok {
			t.Fatalf("route inventory missing runtime route %q", key)
		}
	}
	for key := range actual {
		if _, ok := expected[key]; !ok {
			t.Fatalf("route inventory has no contract entry for runtime route %q", key)
		}
	}
}

func TestTimeoutClassesFollowCanonicalRoutePolicy(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		method     string
		path       string
		wantClass  contract.TimeoutClass
		wantUseTTL bool
		wantTTL    time.Duration
	}{
		{"probe health", http.MethodGet, "/api/daemon/health", contract.TimeoutProbe, true, 2 * time.Second},
		{"read snapshot", http.MethodGet, "/api/runs/run-1/snapshot", contract.TimeoutRead, true, 15 * time.Second},
		{"mutate cancel", http.MethodPost, "/api/runs/run-1/cancel", contract.TimeoutMutate, true, 30 * time.Second},
		{"long mutate exec", http.MethodPost, "/api/exec", contract.TimeoutLongMutate, true, 120 * time.Second},
		{"stream", http.MethodGet, "/api/runs/run-1/stream", contract.TimeoutStream, false, 0},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			class := contract.TimeoutClassForRoute(tt.method, tt.path)
			if class != tt.wantClass {
				t.Fatalf("TimeoutClassForRoute(%s, %s) = %q, want %q", tt.method, tt.path, class, tt.wantClass)
			}

			policy := contract.TimeoutPolicyForClass(class)
			if policy.UsesClientTimeout != tt.wantUseTTL {
				t.Fatalf(
					"TimeoutPolicyForClass(%q).UsesClientTimeout = %v, want %v",
					class,
					policy.UsesClientTimeout,
					tt.wantUseTTL,
				)
			}
			if policy.DefaultTimeout != tt.wantTTL {
				t.Fatalf(
					"TimeoutPolicyForClass(%q).DefaultTimeout = %v, want %v",
					class,
					policy.DefaultTimeout,
					tt.wantTTL,
				)
			}
		})
	}
}

func TestContractRoundTripsCanonicalResponses(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 20, 12, 30, 0, 123456789, time.UTC)

	t.Run("daemon status", func(t *testing.T) {
		t.Parallel()

		resp := contract.DaemonStatusResponse{
			Daemon: contract.DaemonStatus{
				PID:            1234,
				Version:        "v1.2.3",
				StartedAt:      now,
				SocketPath:     "/tmp/compozy.sock",
				HTTPPort:       4317,
				ActiveRunCount: 3,
				WorkspaceCount: 2,
			},
		}

		var decoded contract.DaemonStatusResponse
		roundTripJSON(t, resp, &decoded)

		if decoded.Daemon.PID != resp.Daemon.PID || decoded.Daemon.SocketPath != resp.Daemon.SocketPath {
			t.Fatalf("decoded daemon status = %#v, want %#v", decoded.Daemon, resp.Daemon)
		}
	})

	t.Run("daemon health", func(t *testing.T) {
		t.Parallel()

		resp := contract.DaemonHealthResponse{
			Health: contract.DaemonHealth{
				Ready:               false,
				Degraded:            true,
				UptimeSeconds:       42,
				ActiveRunCount:      2,
				ActiveRunsByMode:    []contract.DaemonModeCount{{Mode: "exec", Count: 1}, {Mode: "task", Count: 1}},
				WorkspaceCount:      3,
				IntegrityIssueCount: 1,
				Databases: contract.DaemonDatabaseDiagnostics{
					GlobalBytes: 12,
					RunDBBytes:  34,
				},
				Reconcile: contract.DaemonReconcileDiagnostics{
					ReconciledRuns:     5,
					CrashEventAppended: 4,
					CrashEventMissing:  1,
					LastRunID:          "run-9",
				},
				Details: []contract.HealthDetail{{
					Code:     "daemon_not_ready",
					Message:  "daemon still reconciling",
					Severity: "warning",
				}},
			},
		}

		var decoded contract.DaemonHealthResponse
		roundTripJSON(t, resp, &decoded)

		if decoded.Health.Ready != resp.Health.Ready ||
			decoded.Health.IntegrityIssueCount != 1 ||
			len(decoded.Health.ActiveRunsByMode) != 2 ||
			len(decoded.Health.Details) != 1 {
			t.Fatalf("decoded daemon health = %#v, want %#v", decoded.Health, resp.Health)
		}
	})

	t.Run("run response", func(t *testing.T) {
		t.Parallel()

		resp := contract.RunResponse{
			Run: contract.Run{
				RunID:            "run-1",
				WorkspaceID:      "ws-1",
				WorkflowSlug:     "daemon-improvs",
				Mode:             "task",
				Status:           "running",
				PresentationMode: "stream",
				StartedAt:        now,
				RequestID:        "req-1",
			},
		}

		var decoded contract.RunResponse
		roundTripJSON(t, resp, &decoded)

		if decoded.Run.RunID != resp.Run.RunID || decoded.Run.RequestID != resp.Run.RequestID {
			t.Fatalf("decoded run = %#v, want %#v", decoded.Run, resp.Run)
		}
	})

	t.Run("run snapshot", func(t *testing.T) {
		t.Parallel()

		snapshot := contract.RunSnapshot{
			Run: contract.Run{
				RunID:            "run-1",
				WorkspaceID:      "ws-1",
				Mode:             "task",
				Status:           "running",
				PresentationMode: "stream",
				StartedAt:        now,
			},
			Jobs: []contract.RunJobState{{
				Index:     1,
				JobID:     "job-1",
				Status:    "running",
				UpdatedAt: now,
				Summary: &contract.RunJobSummary{
					IDE:   "codex",
					Model: "gpt-5.4",
					Session: contract.SessionViewSnapshot{
						Revision: 1,
						Entries: []contract.SessionEntry{{
							ID:            "entry-1",
							Kind:          contract.SessionEntryKindAssistantMessage,
							Title:         "Assistant",
							ToolCallState: contract.ToolCallState("completed"),
							Blocks: []contract.ContentBlock{{
								Type: contract.ContentBlockType("text"),
								Data: json.RawMessage(`{"type":"text","text":"hello from snapshot"}`),
							}},
						}},
						Plan: contract.SessionPlanState{
							Entries: []contract.SessionPlanEntry{{
								Content:  "Lock the contract",
								Priority: "high",
								Status:   "done",
							}},
							DoneCount: 1,
						},
						Session: contract.SessionMetaState{
							CurrentModeID: "review",
							AvailableCommands: []contract.SessionAvailableCommand{{
								Name:         "run",
								Description:  "Run the task",
								ArgumentHint: "<task>",
							}},
							Status: contract.SessionStatus("running"),
						},
					},
				},
			}},
			Transcript: []contract.RunTranscriptMessage{{
				Sequence:  1,
				Stream:    "session",
				Role:      "assistant",
				Content:   "hello",
				Timestamp: now,
			}},
			Usage: kinds.Usage{
				InputTokens:  7,
				OutputTokens: 11,
				TotalTokens:  18,
			},
			Shutdown: &contract.RunShutdownState{
				Phase:       "draining",
				Source:      "signal",
				RequestedAt: now,
				DeadlineAt:  now.Add(30 * time.Second),
			},
			Incomplete:        true,
			IncompleteReasons: []string{"event_gap", "transcript_gap"},
			NextCursor:        &contract.StreamCursor{Timestamp: now, Sequence: 9},
		}

		var wire contract.RunSnapshotResponse
		roundTripJSON(t, contract.RunSnapshotResponseFromSnapshot(snapshot), &wire)

		decoded, err := wire.Decode()
		if err != nil {
			t.Fatalf("RunSnapshotResponse.Decode() error = %v", err)
		}
		if len(decoded.Jobs) != 1 || decoded.Usage.TotalTokens != 18 || decoded.Shutdown == nil {
			t.Fatalf("decoded snapshot = %#v", decoded)
		}
		if got, want := decoded.IncompleteReasons, []string{
			"event_gap",
			"transcript_gap",
		}; !reflect.DeepEqual(
			got,
			want,
		) {
			t.Fatalf("decoded incomplete reasons = %#v, want %#v", got, want)
		}
		if decoded.NextCursor == nil || decoded.NextCursor.Sequence != 9 {
			t.Fatalf("decoded snapshot cursor = %#v, want seq=9", decoded.NextCursor)
		}
		if got := decoded.Jobs[0].Summary.Session.Entries[0].Blocks[0].Type; got != contract.ContentBlockType("text") {
			t.Fatalf("decoded snapshot session block type = %q, want text", got)
		}
	})

	t.Run("run event page", func(t *testing.T) {
		t.Parallel()

		page := contract.RunEventPage{
			Events: []events.Event{{
				SchemaVersion: events.SchemaVersion,
				RunID:         "run-1",
				Seq:           5,
				Timestamp:     now,
				Kind:          events.EventKindRunStarted,
				Payload:       json.RawMessage(`{"status":"started"}`),
			}},
			NextCursor: &contract.StreamCursor{Timestamp: now, Sequence: 5},
			HasMore:    true,
		}

		var wire contract.RunEventPageResponse
		roundTripJSON(t, contract.RunEventPageResponseFromPage(page), &wire)

		decoded, err := wire.Decode()
		if err != nil {
			t.Fatalf("RunEventPageResponse.Decode() error = %v", err)
		}
		if len(decoded.Events) != 1 || !decoded.HasMore {
			t.Fatalf("decoded page = %#v", decoded)
		}
		if decoded.NextCursor == nil || decoded.NextCursor.Sequence != 5 {
			t.Fatalf("decoded page cursor = %#v, want seq=5", decoded.NextCursor)
		}
	})
}

func TestCursorFormattingParsingAndOrderingRemainStable(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 20, 13, 0, 0, 999, time.UTC)
	formatted := contract.FormatCursor(now, 42)
	parsed, err := contract.ParseCursor(formatted)
	if err != nil {
		t.Fatalf("ParseCursor(%q) error = %v", formatted, err)
	}
	if parsed.Timestamp != now.UTC() || parsed.Sequence != 42 {
		t.Fatalf("parsed cursor = %#v, want timestamp=%v sequence=42", parsed, now.UTC())
	}

	eventAtSameTime := events.Event{Timestamp: now, Seq: 43}
	if !contract.EventAfterCursor(eventAtSameTime, parsed) {
		t.Fatal("EventAfterCursor() = false, want true for higher sequence at same timestamp")
	}

	eventBefore := events.Event{Timestamp: now, Seq: 41}
	if contract.EventAfterCursor(eventBefore, parsed) {
		t.Fatal("EventAfterCursor() = true, want false for lower sequence at same timestamp")
	}
}

func TestTransportErrorEnvelopePreservesCanonicalCodesAndRequestIDs(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		status   int
		err      error
		wantCode string
		wantMsg  string
	}{
		{
			name:   "daemon not ready",
			status: http.StatusServiceUnavailable,
			err: contract.NewProblem(
				http.StatusServiceUnavailable,
				string(contract.CodeDaemonNotReady),
				"daemon is still booting",
				nil,
				nil,
			),
			wantCode: string(contract.CodeDaemonNotReady),
			wantMsg:  "daemon is still booting",
		},
		{
			name:     "conflict fallback",
			status:   http.StatusConflict,
			err:      errors.New("workflow already archived"),
			wantCode: string(contract.CodeConflict),
			wantMsg:  "workflow already archived",
		},
		{
			name:   "schema too new",
			status: http.StatusConflict,
			err: contract.NewProblem(
				http.StatusConflict,
				string(contract.CodeSchemaTooNew),
				"database schema is newer than this binary",
				nil,
				nil,
			),
			wantCode: string(contract.CodeSchemaTooNew),
			wantMsg:  "database schema is newer than this binary",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			envelope := contract.TransportErrorEnvelope("req-123", tt.status, tt.err, nil, true)
			if envelope.RequestID != "req-123" || envelope.Code != tt.wantCode || envelope.Message != tt.wantMsg {
				t.Fatalf(
					"envelope = %#v, want code=%q message=%q request_id=req-123",
					envelope,
					tt.wantCode,
					tt.wantMsg,
				)
			}
		})
	}
}

func TestProblemHelpersCoverStatusFallbacksAndMasking(t *testing.T) {
	t.Parallel()

	cause := errors.New("boom")
	problem := contract.NewProblem(
		http.StatusConflict,
		string(contract.CodeConflict),
		"workflow already archived",
		map[string]any{"slug": "daemon-improvs"},
		cause,
	)

	if got := problem.Error(); got != "workflow already archived" {
		t.Fatalf("problem.Error() = %q, want workflow already archived", got)
	}
	if !errors.Is(problem, cause) {
		t.Fatal("errors.Is(problem, cause) = false, want true")
	}
	if problem.Unwrap() != cause {
		t.Fatalf("problem.Unwrap() = %v, want %v", problem.Unwrap(), cause)
	}
	if got := (*contract.Problem)(nil).Error(); got != "" {
		t.Fatalf("(*Problem)(nil).Error() = %q, want empty string", got)
	}
	if (*contract.Problem)(nil).Unwrap() != nil {
		t.Fatal("(*Problem)(nil).Unwrap() != nil, want nil")
	}
	if got := (&contract.Problem{Status: http.StatusNotFound}).Error(); got != http.StatusText(http.StatusNotFound) {
		t.Fatalf("Problem.Error(status fallback) = %q, want %q", got, http.StatusText(http.StatusNotFound))
	}
	if got := (&contract.Problem{}).Error(); got != "transport error" {
		t.Fatalf("Problem.Error(default fallback) = %q, want transport error", got)
	}

	if got := contract.MessageForStatus(
		http.StatusInternalServerError,
		cause,
		true,
	); got != http.StatusText(
		http.StatusInternalServerError,
	) {
		t.Fatalf(
			"MessageForStatus(500, cause, true) = %q, want %q",
			got,
			http.StatusText(http.StatusInternalServerError),
		)
	}
	if got := contract.MessageForStatus(http.StatusConflict, problem, true); got != "workflow already archived" {
		t.Fatalf("MessageForStatus(conflict, explicit problem) = %q, want workflow already archived", got)
	}
	internalProblem := contract.NewProblem(
		http.StatusInternalServerError,
		string(contract.CodeInternalError),
		"raw sqlite failure",
		nil,
		cause,
	)
	if got := contract.MessageForStatus(
		http.StatusInternalServerError,
		internalProblem,
		true,
	); got != http.StatusText(
		http.StatusInternalServerError,
	) {
		t.Fatalf(
			"MessageForStatus(500, internal problem, true) = %q, want %q",
			got,
			http.StatusText(http.StatusInternalServerError),
		)
	}
	if got := contract.MessageForStatus(
		http.StatusServiceUnavailable,
		nil,
		true,
	); got != http.StatusText(
		http.StatusServiceUnavailable,
	) {
		t.Fatalf("MessageForStatus(503, nil, true) = %q, want %q", got, http.StatusText(http.StatusServiceUnavailable))
	}

	if got := contract.ErrorDetails(problem, nil); got["slug"] != "daemon-improvs" {
		t.Fatalf("ErrorDetails(problem) = %#v, want slug", got)
	}
	fallback := map[string]any{"request_id": "req-1"}
	if got := contract.ErrorDetails(problem, fallback); got["request_id"] != "req-1" {
		t.Fatalf("ErrorDetails(fallback) = %#v, want request_id", got)
	}
	if got := contract.ErrorDetails(nil, nil); got != nil {
		t.Fatalf("ErrorDetails(nil, nil) = %#v, want nil", got)
	}
}

func TestDefaultCodeAndTimeoutFallbacksRemainStable(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		status int
		want   string
	}{
		{http.StatusBadRequest, string(contract.CodeInvalidRequest)},
		{http.StatusNotFound, string(contract.CodeNotFound)},
		{http.StatusConflict, string(contract.CodeConflict)},
		{http.StatusUnprocessableEntity, string(contract.CodeValidationError)},
		{http.StatusServiceUnavailable, string(contract.CodeServiceUnavailable)},
		{http.StatusTeapot, string(contract.CodeInternalError)},
	}
	for _, tt := range testCases {
		if got := contract.DefaultCodeForStatus(tt.status); got != tt.want {
			t.Fatalf("DefaultCodeForStatus(%d) = %q, want %q", tt.status, got, tt.want)
		}
	}

	if got := contract.TimeoutClassForRoute(http.MethodHead, "/api/unknown"); got != contract.TimeoutRead {
		t.Fatalf("TimeoutClassForRoute(HEAD, unknown) = %q, want read", got)
	}
	if got := contract.TimeoutClassForRoute(http.MethodPost, "/api/unknown"); got != contract.TimeoutMutate {
		t.Fatalf("TimeoutClassForRoute(POST, unknown) = %q, want mutate", got)
	}
	if got := contract.DefaultTimeout(contract.TimeoutMutate); got != 30*time.Second {
		t.Fatalf("DefaultTimeout(mutate) = %v, want 30s", got)
	}
	if got := contract.TimeoutPolicyForClass(
		contract.TimeoutClass("unknown"),
	); got.Class != contract.TimeoutRead ||
		got.DefaultTimeout != 15*time.Second {
		t.Fatalf("TimeoutPolicyForClass(unknown) = %#v, want read fallback", got)
	}
}

func TestHeartbeatAndOverflowPayloadsUseCanonicalFieldNames(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 20, 13, 15, 0, 0, time.UTC)
	cursor := contract.StreamCursor{Timestamp: now, Sequence: 7}

	heartbeat := contract.HeartbeatMessage("run-1", cursor, now.Add(time.Second))
	overflow := contract.OverflowMessage("run-1", cursor, now.Add(2*time.Second), "slow consumer")

	assertMessagePayload(
		t,
		heartbeat,
		[]string{`"run_id":"run-1"`, `"cursor":"` + contract.FormatCursor(now, 7) + `"`, `"ts":"2026-04-20T13:15:01Z"`},
	)
	assertMessagePayload(
		t,
		overflow,
		[]string{
			`"run_id":"run-1"`,
			`"cursor":"` + contract.FormatCursor(now, 7) + `"`,
			`"reason":"slow consumer"`,
			`"ts":"2026-04-20T13:15:02Z"`,
		},
	)
}

func TestRouteLookupAndCursorHelpersCoverCanonicalEdgeCases(t *testing.T) {
	t.Parallel()

	route, ok := contract.FindRoute(http.MethodGet, "http://daemon/api/runs/run-1/events?limit=10")
	if !ok {
		t.Fatal("FindRoute(GET, absolute URL with query) = missing route, want run events route")
	}
	if route.Path != "/api/runs/:run_id/events" || route.TimeoutClass != contract.TimeoutRead {
		t.Fatalf("FindRoute(run events) = %#v, want canonical events route", route)
	}
	if _, ok := contract.FindRoute(http.MethodGet, "/api/runs//events"); ok {
		t.Fatal("FindRoute(GET, malformed path) matched unexpectedly")
	}

	eventTS := time.Date(2026, 4, 20, 13, 30, 0, 17, time.UTC)
	event := events.Event{Timestamp: eventTS, Seq: 8}
	cursor := contract.CursorFromEvent(event)
	if cursor.Timestamp != eventTS.UTC() || cursor.Sequence != 8 {
		t.Fatalf("CursorFromEvent() = %#v, want timestamp=%v sequence=8", cursor, eventTS.UTC())
	}
	if got := contract.FormatCursor(time.Time{}, 8); got != "" {
		t.Fatalf("FormatCursor(zero, 8) = %q, want empty string", got)
	}
	if got := contract.FormatCursor(eventTS, 0); got != "" {
		t.Fatalf("FormatCursor(ts, 0) = %q, want empty string", got)
	}
	if got := contract.FormatCursorPointer(nil); got != "" {
		t.Fatalf("FormatCursorPointer(nil) = %q, want empty string", got)
	}
	if got := contract.FormatCursorPointer(&cursor); got != contract.FormatCursor(eventTS, 8) {
		t.Fatalf("FormatCursorPointer(cursor) = %q, want formatted cursor", got)
	}

	for _, raw := range []string{"broken", "2026-04-20T13:30:00Z|0", "not-a-time|10"} {
		if _, err := contract.ParseCursor(raw); err == nil {
			t.Fatalf("ParseCursor(%q) error = nil, want invalid cursor", raw)
		}
	}

	if !contract.EventAfterCursor(event, contract.StreamCursor{}) {
		t.Fatal("EventAfterCursor(event, zero cursor) = false, want true")
	}
	if contract.EventAfterCursor(events.Event{Timestamp: eventTS.Add(-time.Second), Seq: 9}, cursor) {
		t.Fatal("EventAfterCursor(older timestamp) = true, want false")
	}
}

func TestCompatibilityNotesCoverRunReaderSurfaces(t *testing.T) {
	t.Parallel()

	var snapshotNote *contract.CompatibilityNote
	var pageNote *contract.CompatibilityNote
	for idx := range contract.RunCompatibilityNotes {
		note := &contract.RunCompatibilityNotes[idx]
		switch note.Surface {
		case "RunSnapshotResponse":
			snapshotNote = note
		case "RunEventPageResponse":
			pageNote = note
		}
	}

	if snapshotNote == nil || pageNote == nil {
		t.Fatalf("compatibility notes missing snapshot/page entries: %#v", contract.RunCompatibilityNotes)
	}
	for _, field := range []string{"jobs", "usage", "shutdown", "next_cursor"} {
		if !slices.Contains(snapshotNote.StableJSONFields, field) {
			t.Fatalf("snapshot compatibility note missing field %q: %#v", field, snapshotNote.StableJSONFields)
		}
	}
	for _, field := range []string{"events", "next_cursor", "has_more"} {
		if !slices.Contains(pageNote.StableJSONFields, field) {
			t.Fatalf("page compatibility note missing field %q: %#v", field, pageNote.StableJSONFields)
		}
	}
}

func roundTripJSON[T any](t *testing.T, value T, dst *T) {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if err := json.Unmarshal(data, dst); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
}

func assertMessagePayload(t *testing.T, msg contract.SSEMessage, want []string) {
	t.Helper()

	data, err := json.Marshal(msg.Data)
	if err != nil {
		t.Fatalf("json.Marshal(%q payload) error = %v", msg.Event, err)
	}
	text := string(data)
	for _, fragment := range want {
		if !strings.Contains(text, fragment) {
			t.Fatalf("%s payload = %s, want fragment %q", msg.Event, text, fragment)
		}
	}
}
