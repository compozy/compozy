package core

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/store/rundb"
	"github.com/compozy/compozy/pkg/compozy/events"
)

func TestProblemAndHelperFunctions(t *testing.T) {
	t.Run("Should format and unwrap problems safely", func(t *testing.T) {
		cause := errors.New("boom")
		problem := NewProblem(
			http.StatusConflict,
			"conflict",
			"workspace has active runs",
			map[string]any{"id": "ws-1"},
			cause,
		)

		if got := problem.Error(); got != "workspace has active runs" {
			t.Fatalf("problem.Error() = %q, want workspace has active runs", got)
		}
		if !errors.Is(problem, cause) {
			t.Fatalf("errors.Is(problem, cause) = false, want true")
		}
		if got := (*Problem)(nil).Error(); got != "" {
			t.Fatalf("(*Problem)(nil).Error() = %q, want empty string", got)
		}
		if (*Problem)(nil).Unwrap() != nil {
			t.Fatal("(*Problem)(nil).Unwrap() != nil, want nil")
		}
		if got := (&Problem{Status: http.StatusConflict}).Error(); got != http.StatusText(http.StatusConflict) {
			t.Fatalf("Problem.Error(status fallback) = %q, want %q", got, http.StatusText(http.StatusConflict))
		}
		if got := (&Problem{}).Error(); got != "transport error" {
			t.Fatalf("Problem.Error(default fallback) = %q, want transport error", got)
		}

		if got := messageForError(http.StatusConflict, problem); got != "workspace has active runs" {
			t.Fatalf("messageForError(conflict) = %q, want workspace has active runs", got)
		}
		if got := messageForError(
			http.StatusInternalServerError,
			cause,
		); got != http.StatusText(
			http.StatusInternalServerError,
		) {
			t.Fatalf("messageForError(500) = %q, want %q", got, http.StatusText(http.StatusInternalServerError))
		}
	})

	t.Run("Should map default status codes", func(t *testing.T) {
		if got := statusForError(nil); got != http.StatusOK {
			t.Fatalf("statusForError(nil) = %d, want 200", got)
		}
		if got := defaultCodeForStatus(http.StatusBadRequest); got != "invalid_request" {
			t.Fatalf("defaultCodeForStatus(400) = %q, want invalid_request", got)
		}
		if got := defaultCodeForStatus(http.StatusNotFound); got != "not_found" {
			t.Fatalf("defaultCodeForStatus(404) = %q, want not_found", got)
		}
		if got := defaultCodeForStatus(http.StatusConflict); got != "conflict" {
			t.Fatalf("defaultCodeForStatus(409) = %q, want conflict", got)
		}
		if got := defaultCodeForStatus(http.StatusUnprocessableEntity); got != "validation_error" {
			t.Fatalf("defaultCodeForStatus(422) = %q, want validation_error", got)
		}
		if got := defaultCodeForStatus(http.StatusServiceUnavailable); got != "service_unavailable" {
			t.Fatalf("defaultCodeForStatus(503) = %q, want service_unavailable", got)
		}
		if got := defaultCodeForStatus(http.StatusInternalServerError); got != "internal_error" {
			t.Fatalf("defaultCodeForStatus(500) = %q, want internal_error", got)
		}
	})

	t.Run("Should classify helper and schema errors", func(t *testing.T) {
		cause := errors.New("boom")
		invalidJSON := invalidJSONProblem("test", "decode request", cause)
		if status := statusForError(invalidJSON); status != http.StatusBadRequest {
			t.Fatalf("statusForError(invalidJSON) = %d, want 400", status)
		}
		if code := codeForError(statusForError(invalidJSON), invalidJSON); code != "invalid_request" {
			t.Fatalf("codeForError(invalidJSON) = %q, want invalid_request", code)
		}

		serviceUnavailable := serviceUnavailableProblem("daemon service")
		if status := statusForError(serviceUnavailable); status != http.StatusServiceUnavailable {
			t.Fatalf("statusForError(serviceUnavailable) = %d, want 503", status)
		}
		if code := codeForError(statusForError(serviceUnavailable), serviceUnavailable); code != "service_unavailable" {
			t.Fatalf("codeForError(serviceUnavailable) = %q, want service_unavailable", code)
		}
		if got := serviceUnavailableProblem("").Error(); got != "service unavailable" {
			t.Fatalf("serviceUnavailableProblem(\"\").Error() = %q, want service unavailable", got)
		}

		globalSchemaErr := globaldb.SchemaTooNewError{CurrentVersion: 5, KnownVersion: 4}
		if got := codeForError(statusForError(globalSchemaErr), globalSchemaErr); got != "schema_too_new" {
			t.Fatalf("codeForError(global schema too new) = %q, want schema_too_new", got)
		}
		if details := detailsForError(globalSchemaErr); details["database"] != "globaldb" {
			t.Fatalf("detailsForError(global schema too new) database = %v, want globaldb", details["database"])
		}

		runSchemaErr := rundb.SchemaTooNewError{CurrentVersion: 5, KnownVersion: 4}
		if got := codeForError(statusForError(runSchemaErr), runSchemaErr); got != "schema_too_new" {
			t.Fatalf("codeForError(run schema too new) = %q, want schema_too_new", got)
		}
		if details := detailsForError(runSchemaErr); details["database"] != "rundb" {
			t.Fatalf("detailsForError(run schema too new) database = %v, want rundb", details["database"])
		}
	})

	t.Run("Should map workflow conflicts and canceled requests", func(t *testing.T) {
		archiveConflict := globaldb.WorkflowActiveRunsError{Slug: "daemon", ActiveRuns: 1}
		if got := statusForError(archiveConflict); got != http.StatusConflict {
			t.Fatalf("statusForError(active run archive conflict) = %d, want 409", got)
		}
		if got := messageForError(http.StatusConflict, archiveConflict); !strings.Contains(got, "active run") {
			t.Fatalf("messageForError(active run archive conflict) = %q, want active run detail", got)
		}

		notArchivable := globaldb.WorkflowNotArchivableError{
			Slug:   "daemon",
			Reason: "task workflow not fully completed",
		}
		if got := statusForError(notArchivable); got != http.StatusConflict {
			t.Fatalf("statusForError(not archivable) = %d, want 409", got)
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if !requestCanceled(ctx) {
			t.Fatal("requestCanceled(canceled) = false, want true")
		}
		if requestCanceled(context.Background()) {
			t.Fatal("requestCanceled(background) = true, want false")
		}
	})
}

func TestHandlerInternalHelpers(t *testing.T) {
	handlers := NewHandlers(&HandlerConfig{})
	if got := handlers.transportName(); got != "api" {
		t.Fatalf("transportName() = %q, want api", got)
	}
	handlers.TransportName = "http"
	if got := handlers.transportName(); got != "http" {
		t.Fatalf("transportName(non-empty) = %q, want http", got)
	}
	if handlers.now().IsZero() {
		t.Fatal("now() = zero time, want non-zero")
	}
	handlers.Now = nil
	if handlers.now().IsZero() {
		t.Fatal("now() with nil Now = zero time, want non-zero")
	}

	done := make(chan struct{})
	handlers.SetStreamDone(done)
	if handlers.streamDoneChannel() != done {
		t.Fatal("streamDoneChannel() did not return the updated channel")
	}
	handlers.SetStreamDone(nil)
	if handlers.streamDoneChannel() == nil {
		t.Fatal("streamDoneChannel() = nil after SetStreamDone(nil), want non-nil channel")
	}
	cloned := handlers.Clone()
	if cloned == nil {
		t.Fatal("Clone() = nil, want non-nil")
	}
	if cloned == handlers {
		t.Fatal("Clone() returned the original handlers pointer")
	}
	if cloned.streamDoneChannel() == nil {
		t.Fatal("Clone().streamDoneChannel() = nil, want non-nil channel")
	}
	var nilHandlers *Handlers
	if nilHandlers.streamDoneChannel() != nil {
		t.Fatal("nil handlers streamDoneChannel() != nil")
	}

	handlers.SetHTTPPort(1234)
	if got := handlers.httpPort.Load(); got != 1234 {
		t.Fatalf("httpPort = %d, want 1234", got)
	}
	if got := cloned.httpPort.Load(); got != 1234 {
		t.Fatalf("cloned httpPort = %d, want 1234", got)
	}
	handlers.SetHTTPPort(0)
	if got := handlers.httpPort.Load(); got != 1234 {
		t.Fatalf("httpPort after invalid set = %d, want 1234", got)
	}
	nilHandlers.SetHTTPPort(4321)
}

func TestCursorHelpersAndTerminalEvents(t *testing.T) {
	t.Parallel()

	timestamp := time.Date(2026, 4, 17, 14, 0, 0, 0, time.UTC)
	event := events.Event{
		Seq:       2,
		Timestamp: timestamp,
		Kind:      events.EventKindRunCompleted,
	}

	cursor := CursorFromEvent(event)
	if cursor.Sequence != 2 || !cursor.Timestamp.Equal(timestamp) {
		t.Fatalf("CursorFromEvent() = %#v, want timestamp=%s sequence=2", cursor, timestamp)
	}

	if !EventAfterCursor(event, StreamCursor{Timestamp: timestamp, Sequence: 1}) {
		t.Fatal("EventAfterCursor(after older cursor) = false, want true")
	}
	if !EventAfterCursor(event, StreamCursor{}) {
		t.Fatal("EventAfterCursor(zero cursor) = false, want true")
	}
	if EventAfterCursor(event, StreamCursor{Timestamp: timestamp, Sequence: 2}) {
		t.Fatal("EventAfterCursor(equal cursor) = true, want false")
	}
	if EventAfterCursor(event, StreamCursor{Timestamp: timestamp.Add(time.Second), Sequence: 1}) {
		t.Fatal("EventAfterCursor(older event) = true, want false")
	}

	testCases := []struct {
		name string
		kind events.EventKind
		want bool
	}{
		{name: "Should mark run.completed as terminal", kind: events.EventKindRunCompleted, want: true},
		{name: "Should mark run.failed as terminal", kind: events.EventKindRunFailed, want: true},
		{name: "Should mark run.cancelled as terminal", kind: events.EventKindRunCancelled, want: true},
		{name: "Should mark shutdown.requested as terminal", kind: events.EventKindShutdownRequested, want: true},
		{name: "Should mark shutdown.draining as terminal", kind: events.EventKindShutdownDraining, want: true},
		{name: "Should mark shutdown.terminated as terminal", kind: events.EventKindShutdownTerminated, want: true},
		{name: "Should leave session.update non-terminal", kind: events.EventKindSessionUpdate, want: false},
		{name: "Should leave run.started non-terminal", kind: events.EventKindRunStarted, want: false},
	}
	for _, tt := range testCases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := isTerminalRunEvent(tt.kind); got != tt.want {
				t.Fatalf("isTerminalRunEvent(%s) = %v, want %v", tt.kind, got, tt.want)
			}
		})
	}
}
