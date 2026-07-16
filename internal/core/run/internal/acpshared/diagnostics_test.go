package acpshared

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/model"
)

type diagnosticsFakeSession struct {
	id      string
	dropped uint64
	slow    uint64
}

func (s diagnosticsFakeSession) ID() string                          { return s.id }
func (s diagnosticsFakeSession) Identity() agent.SessionIdentity     { return agent.SessionIdentity{} }
func (s diagnosticsFakeSession) Updates() <-chan model.SessionUpdate { return nil }
func (s diagnosticsFakeSession) Done() <-chan struct{}               { return nil }
func (s diagnosticsFakeSession) Err() error                          { return nil }
func (s diagnosticsFakeSession) SlowPublishes() uint64               { return s.slow }
func (s diagnosticsFakeSession) DroppedUpdates() uint64              { return s.dropped }

type diagnosticsFakeJournal struct {
	dropsOnSubmit    uint64
	terminalDrops    uint64
	nonTerminalDrops uint64
}

func (j diagnosticsFakeJournal) DropsOnSubmit() uint64 { return j.dropsOnSubmit }

func (j diagnosticsFakeJournal) DroppedEventCounts() (uint64, uint64) {
	return j.terminalDrops, j.nonTerminalDrops
}

func TestLogProgressSignalDiagnostics(t *testing.T) {
	t.Parallel()

	t.Run("Should log non-zero drop and backpressure counters when drops occurred", func(t *testing.T) {
		t.Parallel()
		logger, buf := newBufferLogger()

		logProgressSignalDiagnostics(
			logger,
			"sess-drops",
			diagnosticsFakeSession{id: "sess-drops", dropped: 4, slow: 2},
			diagnosticsFakeJournal{dropsOnSubmit: 3, terminalDrops: 1, nonTerminalDrops: 2},
		)

		record := decodeSingleDiagnosticsRecord(t, buf)
		assertLogUint(t, record, "dropped_updates", 4)
		assertLogUint(t, record, "slow_publishes", 2)
		assertLogUint(t, record, "journal_drops_on_submit", 3)
		assertLogUint(t, record, "journal_terminal_drops", 1)
		assertLogUint(t, record, "journal_non_terminal_drops", 2)
		if got := record["session_id"]; got != "sess-drops" {
			t.Fatalf("unexpected session_id: %v", got)
		}
	})

	t.Run("Should log zero counters for a clean run", func(t *testing.T) {
		t.Parallel()
		logger, buf := newBufferLogger()

		logProgressSignalDiagnostics(
			logger,
			"sess-clean",
			diagnosticsFakeSession{id: "sess-clean"},
			diagnosticsFakeJournal{},
		)

		record := decodeSingleDiagnosticsRecord(t, buf)
		assertLogUint(t, record, "dropped_updates", 0)
		assertLogUint(t, record, "slow_publishes", 0)
		assertLogUint(t, record, "journal_drops_on_submit", 0)
	})

	t.Run("Should omit journal fields when submitter is not a journal", func(t *testing.T) {
		t.Parallel()
		logger, buf := newBufferLogger()

		logProgressSignalDiagnostics(logger, "sess-nojournal", diagnosticsFakeSession{slow: 1}, nil)

		record := decodeSingleDiagnosticsRecord(t, buf)
		assertLogUint(t, record, "slow_publishes", 1)
		if _, ok := record["journal_drops_on_submit"]; ok {
			t.Fatalf("expected no journal fields without a journal submitter, got %v", record)
		}
	})

	t.Run("Should no-op on nil session", func(t *testing.T) {
		t.Parallel()
		logger, buf := newBufferLogger()

		logProgressSignalDiagnostics(logger, "sess-nil", nil, nil)

		if got := strings.TrimSpace(buf.String()); got != "" {
			t.Fatalf("expected no log output for nil session, got %q", got)
		}
	})
}

func TestSessionExecutionCloseEmitsDiagnostics(t *testing.T) {
	t.Parallel()

	logger, buf := newBufferLogger()
	execution := &SessionExecution{
		Session: diagnosticsFakeSession{id: "sess-close", dropped: 5, slow: 1},
		Logger:  logger,
	}

	execution.Close()

	record := decodeSingleDiagnosticsRecord(t, buf)
	if got := record["session_id"]; got != "sess-close" {
		t.Fatalf("unexpected session_id: %v", got)
	}
	assertLogUint(t, record, "dropped_updates", 5)
	assertLogUint(t, record, "slow_publishes", 1)
}

func TestSessionExecutionCloseWithoutSessionSkipsDiagnostics(t *testing.T) {
	t.Parallel()

	logger, buf := newBufferLogger()
	execution := &SessionExecution{Logger: logger}

	execution.Close()

	if got := strings.TrimSpace(buf.String()); got != "" {
		t.Fatalf("expected no diagnostics without a session, got %q", got)
	}
}

func TestSessionExecutionCloseReadsJournalCounters(t *testing.T) {
	t.Parallel()

	_, runJournal, _, cleanup := openRuntimeEventCapture(t)
	t.Cleanup(cleanup)

	logger, buf := newBufferLogger()
	handler := NewSessionUpdateHandler(SessionUpdateHandlerConfig{RunJournal: runJournal})
	execution := &SessionExecution{
		Session: diagnosticsFakeSession{id: "sess-journal"},
		Handler: handler,
		Logger:  logger,
	}

	execution.Close()

	record := decodeSingleDiagnosticsRecord(t, buf)
	if _, ok := record["journal_drops_on_submit"]; !ok {
		t.Fatalf("expected journal counters when submitter is a journal, got %v", record)
	}
	assertLogUint(t, record, "journal_drops_on_submit", 0)
}

func newBufferLogger() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	return slog.New(slog.NewJSONHandler(&buf, nil)), &buf
}

func decodeSingleDiagnosticsRecord(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()

	raw := strings.TrimSpace(buf.String())
	if raw == "" {
		t.Fatal("expected a diagnostics log record, got none")
	}
	lines := strings.Split(raw, "\n")
	if len(lines) != 1 {
		t.Fatalf("expected exactly one diagnostics record, got %d: %q", len(lines), raw)
	}
	var record map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &record); err != nil {
		t.Fatalf("decode diagnostics record %q: %v", lines[0], err)
	}
	if msg := record["msg"]; msg != "acp progress-signal diagnostics" {
		t.Fatalf("unexpected diagnostics message: %v", msg)
	}
	return record
}

func assertLogUint(t *testing.T, record map[string]any, key string, want uint64) {
	t.Helper()

	raw, ok := record[key]
	if !ok {
		t.Fatalf("expected diagnostics key %q in record %v", key, record)
	}
	num, ok := raw.(float64)
	if !ok {
		t.Fatalf("expected numeric value for %q, got %T (%v)", key, raw, raw)
	}
	if uint64(num) != want {
		t.Fatalf("unexpected %q: got %d want %d", key, uint64(num), want)
	}
}
