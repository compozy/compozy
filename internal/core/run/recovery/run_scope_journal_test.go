package recovery

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/run/journal"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
)

func TestRefreshRunScopeJournalReplacesClosedAppendHandle(t *testing.T) {
	t.Parallel()

	t.Run("Should replace a closed append handle with a fresh journal", func(t *testing.T) {
		ctx := context.Background()
		scope := newRecoveryJournalTestScope(t, "refresh-run")
		original := scope.RunJournal()
		if err := original.Submit(
			ctx,
			recoveryJournalTestEvent("refresh-run", eventspkg.EventKindRunStarted),
		); err != nil {
			t.Fatalf("submit original event: %v", err)
		}

		if err := RefreshRunScopeJournal(ctx, scope); err != nil {
			t.Fatalf("RefreshRunScopeJournal() error = %v", err)
		}
		if scope.RunJournal() == nil {
			t.Fatal("expected refreshed journal")
		}
		if scope.RunJournal() == original {
			t.Fatal("expected refreshed journal handle to replace original")
		}
		if err := original.Submit(
			ctx,
			recoveryJournalTestEvent("refresh-run", eventspkg.EventKindRunFailed),
		); !errors.Is(
			err,
			journal.ErrClosed,
		) {
			t.Fatalf("submit to original journal error = %v, want ErrClosed", err)
		}
		if err := scope.RunJournal().
			Submit(ctx, recoveryJournalTestEvent("refresh-run", eventspkg.EventKindRunRecovered)); err != nil {
			t.Fatalf("submit refreshed event: %v", err)
		}
		if err := scope.Close(ctx); err != nil {
			t.Fatalf("close scope: %v", err)
		}

		payload := readRecoveryJournalEvents(t, scope.RunArtifacts().EventsPath)
		for _, kind := range []eventspkg.EventKind{eventspkg.EventKindRunStarted, eventspkg.EventKindRunRecovered} {
			if got := strings.Count(payload, string(kind)); got != 1 {
				t.Fatalf("event %s count = %d, want 1\n%s", kind, got, payload)
			}
		}
	})
}

func TestRunScopeEventSinkRefreshesBeforeSubmit(t *testing.T) {
	t.Parallel()

	t.Run("Should refresh the scope journal before submit", func(t *testing.T) {
		ctx := context.Background()
		scope := newRecoveryJournalTestScope(t, "sink-run")
		if err := scope.RunJournal().Close(ctx); err != nil {
			t.Fatalf("close original journal: %v", err)
		}
		sink := NewRunScopeEventSink(scope)
		if sink == nil {
			t.Fatal("expected event sink")
		}

		if err := sink.Submit(
			ctx,
			recoveryJournalTestEvent("sink-run", eventspkg.EventKindRunRecoveryStarted),
		); err != nil {
			t.Fatalf("Submit() error = %v", err)
		}
		if err := scope.Close(ctx); err != nil {
			t.Fatalf("close scope: %v", err)
		}

		payload := readRecoveryJournalEvents(t, scope.RunArtifacts().EventsPath)
		if got := strings.Count(payload, string(eventspkg.EventKindRunRecoveryStarted)); got != 1 {
			t.Fatalf("recovery started events = %d, want 1\n%s", got, payload)
		}
	})

	t.Run("Should serialize concurrent refresh-and-submit calls", func(t *testing.T) {
		ctx := context.Background()
		scope := newRecoveryJournalTestScope(t, "sink-run-concurrent")
		sink := NewRunScopeEventSink(scope)
		if sink == nil {
			t.Fatal("expected event sink")
		}

		const submits = 24
		start := make(chan struct{})
		errCh := make(chan error, submits)
		var wg sync.WaitGroup
		for i := 0; i < submits; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				errCh <- sink.Submit(
					ctx,
					recoveryJournalTestEvent("sink-run-concurrent", eventspkg.EventKindRunRecoveryStarted),
				)
			}()
		}
		close(start)
		wg.Wait()
		close(errCh)
		for err := range errCh {
			if err != nil {
				t.Fatalf("Submit() error = %v", err)
			}
		}
		if err := scope.Close(ctx); err != nil {
			t.Fatalf("close scope: %v", err)
		}

		payload := readRecoveryJournalEvents(t, scope.RunArtifacts().EventsPath)
		if got := strings.Count(payload, string(eventspkg.EventKindRunRecoveryStarted)); got != submits {
			t.Fatalf("recovery started events = %d, want %d\n%s", got, submits, payload)
		}
	})
}

func TestRefreshRunScopeJournalValidationErrors(t *testing.T) {
	t.Parallel()

	t.Run("Should reject a nil run scope", func(t *testing.T) {
		t.Parallel()

		err := RefreshRunScopeJournal(context.Background(), nil)
		if err == nil || !strings.Contains(err.Error(), "missing run scope") {
			t.Fatalf("RefreshRunScopeJournal(nil) error = %v, want missing run scope", err)
		}
	})

	t.Run("Should reject a scope without a journal setter", func(t *testing.T) {
		t.Parallel()

		err := RefreshRunScopeJournal(context.Background(), noSetterRunScope{})
		if err == nil || !strings.Contains(err.Error(), "unsupported run scope") {
			t.Fatalf("RefreshRunScopeJournal(noSetterRunScope) error = %v, want unsupported run scope", err)
		}
	})

	t.Run("Should reject a scope without an events path", func(t *testing.T) {
		t.Parallel()

		err := RefreshRunScopeJournal(context.Background(), &model.BaseRunScope{})
		if err == nil || !strings.Contains(err.Error(), "missing events path") {
			t.Fatalf("RefreshRunScopeJournal(empty scope) error = %v, want missing events path", err)
		}
	})

	t.Run("Should return a nil sink for a nil scope", func(t *testing.T) {
		t.Parallel()

		if sink := NewRunScopeEventSink(nil); sink != nil {
			t.Fatalf("NewRunScopeEventSink(nil) = %#v, want nil", sink)
		}
	})
}

type noSetterRunScope struct{}

func (noSetterRunScope) RunArtifacts() model.RunArtifacts {
	return model.RunArtifacts{EventsPath: "events.jsonl"}
}

func (noSetterRunScope) RunJournal() *journal.Journal {
	return nil
}

func (noSetterRunScope) RunEventBus() *eventspkg.Bus[eventspkg.Event] {
	return eventspkg.New[eventspkg.Event](1)
}

func (noSetterRunScope) RunManager() model.RuntimeManager {
	return nil
}

func (noSetterRunScope) Close(context.Context) error {
	return nil
}

func newRecoveryJournalTestScope(t *testing.T, runID string) *model.BaseRunScope {
	t.Helper()
	artifacts := model.NewRunArtifacts(t.TempDir(), runID)
	if err := os.MkdirAll(filepath.Dir(artifacts.EventsPath), 0o755); err != nil {
		t.Fatalf("mkdir artifacts dir: %v", err)
	}
	bus := eventspkg.New[eventspkg.Event](16)
	runJournal, err := journal.Open(artifacts.EventsPath, bus, 0)
	if err != nil {
		t.Fatalf("open journal: %v", err)
	}
	scope := &model.BaseRunScope{Artifacts: artifacts, Journal: runJournal, EventBus: bus}
	t.Cleanup(func() {
		if err := scope.Close(context.Background()); err != nil {
			t.Fatalf("close scope cleanup: %v", err)
		}
	})
	return scope
}

func recoveryJournalTestEvent(runID string, kind eventspkg.EventKind) eventspkg.Event {
	return eventspkg.Event{
		SchemaVersion: eventspkg.SchemaVersion,
		RunID:         runID,
		Timestamp:     time.Now().UTC(),
		Kind:          kind,
		Payload:       []byte(`{}`),
	}
}

func readRecoveryJournalEvents(t *testing.T, path string) string {
	t.Helper()
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read events %s: %v", path, err)
	}
	return string(payload)
}
