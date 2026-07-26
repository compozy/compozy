package memory

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	eventspkg "github.com/compozy/agh/internal/events"
	storepkg "github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
	aghworkspace "github.com/compozy/agh/internal/workspace"
)

func TestMemoryEventOpsUseCanonicalRegistry(t *testing.T) {
	t.Parallel()

	t.Run("Should register every memory catalog event operation", func(t *testing.T) {
		t.Parallel()

		for _, op := range memoryEventOps {
			meta, ok := eventspkg.Lookup(op)
			if !ok {
				t.Fatalf("events.Lookup(%q) = false, want true", op)
			}
			if meta.Component != eventspkg.ComponentMemory {
				t.Fatalf("events.Lookup(%q).Component = %q, want %q", op, meta.Component, eventspkg.ComponentMemory)
			}
		}
	})
}

func TestStoreListMemoryEventSummaries(t *testing.T) {
	t.Run("Should aggregate global and workspace memory event databases once", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		baseDir := t.TempDir()
		workspaceRoot := filepath.Join(baseDir, "workspace")
		globalStore := newOpenTestStore(t,
			filepath.Join(baseDir, "global", "memory"),
			WithCatalogDatabasePath(filepath.Join(baseDir, "agh-home", storepkg.GlobalDatabaseName)),
		)
		workspaceCatalog, workspaceID := openWorkspaceObservabilityCatalog(ctx, t, workspaceRoot)

		globalAt := time.Date(2026, 5, 5, 10, 0, 0, 0, time.UTC)
		workspaceAt := globalAt.Add(time.Minute)
		excludedWorkspaceAt := workspaceAt.Add(time.Minute)
		excludedGlobalWorkspaceAt := excludedWorkspaceAt.Add(time.Minute)
		insertMemoryObservabilityEvent(
			ctx,
			t,
			globalStore.catalog,
			memoryEventWriteCommitted,
			"global",
			"",
			"daemon",
			"global write committed",
			globalAt,
		)
		insertMemoryObservabilityEvent(
			ctx,
			t,
			workspaceCatalog,
			memoryEventRecallExecuted,
			"workspace",
			workspaceID,
			"reviewer",
			"workspace recall executed",
			workspaceAt,
		)
		insertMemoryObservabilityEvent(
			ctx,
			t,
			workspaceCatalog,
			memoryEventRecallExecuted,
			"workspace",
			"ws-excluded",
			"intruder",
			"excluded workspace recall",
			excludedWorkspaceAt,
		)
		insertMemoryObservabilityEvent(
			ctx,
			t,
			globalStore.catalog,
			memoryEventRecallExecuted,
			"",
			"ws-excluded",
			"intruder",
			"excluded global workspace recall",
			excludedGlobalWorkspaceAt,
		)

		events, err := globalStore.ListMemoryEventSummaries(
			ctx,
			[]string{workspaceRoot, workspaceRoot},
			storepkg.EventSummaryQuery{},
		)
		if err != nil {
			t.Fatalf("ListMemoryEventSummaries() error = %v", err)
		}
		if got, want := len(events), 2; got != want {
			t.Fatalf("len(events) = %d, want %d; events=%#v", got, want, events)
		}
		if got, want := events[0].Summary, "global write committed"; got != want {
			t.Fatalf("events[0].Summary = %q, want %q", got, want)
		}
		if got, want := events[1].Summary, "workspace recall executed"; got != want {
			t.Fatalf("events[1].Summary = %q, want %q", got, want)
		}
		if got, want := events[1].WorkspaceID, workspaceID; got != want {
			t.Fatalf("events[1].WorkspaceID = %q, want %q", got, want)
		}
		if events[0].ID == events[1].ID {
			t.Fatalf("event IDs are not source-stable: %#v", events)
		}

		limited, err := globalStore.ListMemoryEventSummaries(
			ctx,
			[]string{workspaceRoot},
			storepkg.EventSummaryQuery{Limit: 1},
		)
		if err != nil {
			t.Fatalf("ListMemoryEventSummaries(limit) error = %v", err)
		}
		if len(limited) != 1 || limited[0].Type != memoryEventRecallExecuted {
			t.Fatalf("limited events = %#v, want latest workspace recall event", limited)
		}

		filtered, err := globalStore.ListMemoryEventSummaries(
			ctx,
			[]string{workspaceRoot},
			storepkg.EventSummaryQuery{Type: memoryEventRecallExecuted},
		)
		if err != nil {
			t.Fatalf("ListMemoryEventSummaries(type) error = %v", err)
		}
		if len(filtered) != 1 || filtered[0].AgentName != "reviewer" {
			t.Fatalf("filtered events = %#v, want reviewer recall event", filtered)
		}

		workspaceOnly, err := globalStore.ListMemoryEventSummaries(
			ctx,
			[]string{workspaceRoot},
			storepkg.EventSummaryQuery{WorkspaceID: workspaceID},
		)
		if err != nil {
			t.Fatalf("ListMemoryEventSummaries(workspace filter) error = %v", err)
		}
		if len(workspaceOnly) != 1 || workspaceOnly[0].Summary != "workspace recall executed" {
			t.Fatalf("workspace-filtered events = %#v, want only visible workspace event", workspaceOnly)
		}
	})

	t.Run("Should refuse a legacy workspace database before querying events", func(t *testing.T) {
		t.Parallel()

		workspaceRoot := filepath.Join(t.TempDir(), "workspace")
		dbPath := seedWorkspaceObservabilityDatabase(
			t,
			workspaceRoot,
			`CREATE TABLE memory_schema_migrations (version INTEGER PRIMARY KEY)`,
		)
		before := observabilityFileDigest(t, dbPath)
		store := NewStore(filepath.Join(t.TempDir(), "global-memory"))

		_, err := store.ListMemoryEventSummaries(
			testutil.Context(t),
			[]string{workspaceRoot},
			storepkg.EventSummaryQuery{},
		)
		if !errors.Is(err, storepkg.ErrLegacyDatabase) {
			t.Fatalf("ListMemoryEventSummaries(legacy) error = %v, want ErrLegacyDatabase", err)
		}
		if after := observabilityFileDigest(t, dbPath); after != before {
			t.Fatalf("legacy workspace database digest changed: got %x want %x", after, before)
		}
	})

	t.Run("Should refuse an ahead workspace database before querying events", func(t *testing.T) {
		t.Parallel()

		workspaceRoot := filepath.Join(t.TempDir(), "workspace")
		dbPath := seedWorkspaceObservabilityDatabase(
			t,
			workspaceRoot,
			`CREATE TABLE goose_db_version_memory (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				version_id INTEGER NOT NULL,
				is_applied INTEGER NOT NULL,
				tstamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)`,
			`INSERT INTO goose_db_version_memory (version_id, is_applied) VALUES (99, 1)`,
		)
		before := observabilityFileDigest(t, dbPath)
		store := NewStore(filepath.Join(t.TempDir(), "global-memory"))

		_, err := store.ListMemoryEventSummaries(
			testutil.Context(t),
			[]string{workspaceRoot},
			storepkg.EventSummaryQuery{},
		)
		if !errors.Is(err, storepkg.ErrSchemaAhead) {
			t.Fatalf("ListMemoryEventSummaries(ahead) error = %v, want ErrSchemaAhead", err)
		}
		if after := observabilityFileDigest(t, dbPath); after != before {
			t.Fatalf("ahead workspace database digest changed: got %x want %x", after, before)
		}
	})
}

func TestStoreHealthStats(t *testing.T) {
	t.Run("Should include workspace database events in health derivation", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		baseDir := t.TempDir()
		workspaceRoot := filepath.Join(baseDir, "workspace")
		globalStore := newOpenTestStore(t,
			filepath.Join(baseDir, "global", "memory"),
			WithCatalogDatabasePath(filepath.Join(baseDir, "agh-home", storepkg.GlobalDatabaseName)),
		)
		workspaceCatalog, workspaceID := openWorkspaceObservabilityCatalog(ctx, t, workspaceRoot)
		operatedAt := time.Date(2026, 5, 5, 10, 30, 0, 0, time.UTC)
		insertMemoryObservabilityEvent(
			ctx,
			t,
			workspaceCatalog,
			memoryEventRecallExecuted,
			"workspace",
			workspaceID,
			"daemon",
			"workspace recall health signal",
			operatedAt,
		)

		stats, err := globalStore.HealthStats(ctx, []string{workspaceRoot})
		if err != nil {
			t.Fatalf("HealthStats() error = %v", err)
		}
		if got, want := stats.OperationCount, 1; got != want {
			t.Fatalf("HealthStats().OperationCount = %d, want %d", got, want)
		}
		if stats.LastOperationAt == nil || !stats.LastOperationAt.Equal(operatedAt) {
			t.Fatalf("HealthStats().LastOperationAt = %v, want %s", stats.LastOperationAt, operatedAt)
		}
	})
}

func openWorkspaceObservabilityCatalog(
	ctx context.Context,
	t *testing.T,
	workspaceRoot string,
) (*catalog, string) {
	t.Helper()

	if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll(workspaceRoot) error = %v", err)
	}
	identity, err := aghworkspace.EnsureIdentity(ctx, workspaceRoot)
	if err != nil {
		t.Fatalf("EnsureIdentity() error = %v", err)
	}
	workspaceCatalog := newCatalog(
		filepath.Join(filepath.Dir(identity.Path), storepkg.GlobalDatabaseName),
		func() time.Time { return time.Now().UTC() },
	)
	if err := workspaceCatalog.open(ctx); err != nil {
		t.Fatalf("catalog.open() error = %v", err)
	}
	t.Cleanup(func() {
		if err := workspaceCatalog.close(context.Background()); err != nil {
			t.Errorf("catalog.close() error = %v", err)
		}
	})
	return workspaceCatalog, identity.WorkspaceID
}

func seedWorkspaceObservabilityDatabase(t *testing.T, workspaceRoot string, statements ...string) string {
	t.Helper()

	if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll(workspaceRoot) error = %v", err)
	}
	identity, err := aghworkspace.EnsureIdentity(testutil.Context(t), workspaceRoot)
	if err != nil {
		t.Fatalf("EnsureIdentity() error = %v", err)
	}
	dbPath := filepath.Join(filepath.Dir(identity.Path), storepkg.GlobalDatabaseName)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open(%q) error = %v", dbPath, err)
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(testutil.Context(t), statement); err != nil {
			if closeErr := db.Close(); closeErr != nil {
				t.Fatalf("seed workspace database error = %v; close error = %v", err, closeErr)
			}
			t.Fatalf("seed workspace database error = %v", err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close(seed workspace database) error = %v", err)
	}
	return dbPath
}

func observabilityFileDigest(t *testing.T, path string) [sha256.Size]byte {
	t.Helper()

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	return sha256.Sum256(contents)
}

func insertMemoryObservabilityEvent(
	ctx context.Context,
	t *testing.T,
	catalog *catalog,
	op string,
	scope string,
	workspaceID string,
	agentName string,
	summary string,
	timestamp time.Time,
) {
	t.Helper()

	db, err := catalog.ensureDB(ctx)
	if err != nil {
		t.Fatalf("catalog.ensureDB() error = %v", err)
	}
	metadata, err := json.Marshal(map[string]string{memoryEventMetadataSummaryKey: summary})
	if err != nil {
		t.Fatalf("json.Marshal(metadata) error = %v", err)
	}
	if _, err := db.ExecContext(
		ctx,
		`INSERT INTO memory_events (
			op, scope, agent_name, workspace_id, actor_kind, metadata, ts_ms
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		op,
		nullStringForEmpty(scope),
		agentName,
		nullStringForEmpty(workspaceID),
		"system",
		string(metadata),
		timeToUnixMillis(timestamp),
	); err != nil {
		t.Fatalf("insert memory event error = %v", err)
	}
}
