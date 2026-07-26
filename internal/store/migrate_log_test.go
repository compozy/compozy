package store

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/compozy/agh/internal/testutil"
)

type capturedMigrationLog struct {
	message string
	attrs   map[string]any
}

type migrationLogSink struct {
	mu      sync.Mutex
	records []capturedMigrationLog
}

type migrationLogHandler struct {
	sink  *migrationLogSink
	attrs []slog.Attr
}

func (h *migrationLogHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *migrationLogHandler) Handle(_ context.Context, record slog.Record) error {
	h.sink.mu.Lock()
	defer h.sink.mu.Unlock()
	attrs := make(map[string]any, len(h.attrs)+record.NumAttrs())
	for _, attr := range h.attrs {
		attrs[attr.Key] = attr.Value.Any()
	}
	record.Attrs(func(attr slog.Attr) bool {
		attrs[attr.Key] = attr.Value.Any()
		return true
	})
	h.sink.records = append(h.sink.records, capturedMigrationLog{message: record.Message, attrs: attrs})
	return nil
}

func (h *migrationLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &migrationLogHandler{
		sink:  h.sink,
		attrs: append(append([]slog.Attr(nil), h.attrs...), attrs...),
	}
}

func (h *migrationLogHandler) WithGroup(string) slog.Handler { return h }

func (h *migrationLogHandler) find(message string) (capturedMigrationLog, bool) {
	h.sink.mu.Lock()
	defer h.sink.mu.Unlock()
	for _, record := range h.sink.records {
		if record.message == message {
			return record, true
		}
	}
	return capturedMigrationLog{}, false
}

func TestApplyMigrationLogging(t *testing.T) {
	// not parallel: replaces the process-wide default slog logger.
	t.Run("Should emit applied attributes after success", func(t *testing.T) {
		handler := installMigrationLogCapture(t)
		db := openEngineTestDB(t, "log-applied.db")
		stream := fixtureMigrationStream("goose_db_version_global", "testdata/migrations/happy")
		if err := Apply(testutil.Context(t), db, stream); err != nil {
			t.Fatalf("Apply() error = %v", err)
		}
		record, ok := handler.find("store.migrations.applied")
		if !ok {
			t.Fatal("store.migrations.applied log not found")
		}
		assertMigrationLogAttr(t, record, "stream", stream.Name)
		assertMigrationLogAttr(t, record, "version", int64(2))
		assertMigrationLogAttr(t, record, "applied_count", int64(2))
		if _, ok := record.attrs["duration_ms"]; !ok {
			t.Fatalf("duration_ms missing from %#v", record.attrs)
		}
	})

	t.Run("Should emit legacy refusal attributes", func(t *testing.T) {
		handler := installMigrationLogCapture(t)
		db := openEngineTestDB(t, "log-legacy.db")
		if _, err := db.ExecContext(
			testutil.Context(t),
			`CREATE TABLE schema_migrations (version INTEGER)`,
		); err != nil {
			t.Fatalf("seed legacy table: %v", err)
		}
		stream := fixtureMigrationStream("goose_db_version_global", "testdata/migrations/happy")
		stream.LegacyTables = []string{"schema_migrations"}
		if err := Apply(testutil.Context(t), db, stream); err == nil {
			t.Fatal("Apply() error = nil, want legacy refusal")
		}
		record, ok := handler.find("store.migrations.refused")
		if !ok {
			t.Fatal("store.migrations.refused log not found")
		}
		assertMigrationLogAttr(t, record, "stream", stream.Name)
		assertMigrationLogAttr(t, record, "reason", "legacy_database")
		if _, ok := record.attrs["path"]; !ok {
			t.Fatalf("path missing from %#v", record.attrs)
		}
	})

	t.Run("Should emit sum mismatch refusal attributes", func(t *testing.T) {
		handler := installMigrationLogCapture(t)
		files := copyFixtureDirectory(t, "testdata/migrations/happy")
		files["00003_added.sql"] = &fstest.MapFile{Data: []byte("-- +goose Up\nSELECT 1;")}
		stream := MigrationStream{
			Name:         "sum-drift",
			FS:           files,
			Dir:          ".",
			VersionTable: "goose_db_version_sum_drift",
		}
		if err := Apply(testutil.Context(t), openEngineTestDB(t, "log-sum.db"), stream); err == nil {
			t.Fatal("Apply() error = nil, want sum mismatch")
		}
		record, ok := handler.find("store.migrations.refused")
		if !ok {
			t.Fatal("store.migrations.refused log not found")
		}
		assertMigrationLogAttr(t, record, "stream", stream.Name)
		assertMigrationLogAttr(t, record, "reason", "sum_mismatch")
	})
}

func installMigrationLogCapture(t *testing.T) *migrationLogHandler {
	t.Helper()
	handler := &migrationLogHandler{sink: &migrationLogSink{}}
	original := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() {
		slog.SetDefault(original)
	})
	return handler
}

func assertMigrationLogAttr(t *testing.T, record capturedMigrationLog, key string, want any) {
	t.Helper()
	got, ok := record.attrs[key]
	if !ok {
		t.Fatalf("attribute %s missing from %#v", key, record.attrs)
	}
	if got != want {
		t.Fatalf("attribute %s = %#v, want %#v", key, got, want)
	}
}
