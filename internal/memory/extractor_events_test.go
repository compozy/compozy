package memory

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	memcontract "github.com/compozy/agh/internal/memory/contract"
	memoryextractor "github.com/compozy/agh/internal/memory/extractor"
	storepkg "github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
)

func TestStoreExtractorControllerFlow(t *testing.T) {
	t.Run("Should propose extracted candidates through the controller seam", func(t *testing.T) {
		t.Parallel()

		store := newOpenTestStore(t,
			filepath.Join(t.TempDir(), "memory"),
			WithCatalogDatabasePath(filepath.Join(t.TempDir(), storepkg.GlobalDatabaseName)),
		)
		if err := store.EnsureDirs(); err != nil {
			t.Fatalf("EnsureDirs() error = %v", err)
		}
		candidate := memcontract.Candidate{
			Scope:   memcontract.ScopeGlobal,
			Origin:  memcontract.OriginExtractor,
			Content: "Pedro prefers concise implementation updates.",
			Frontmatter: memcontract.Header{
				Name:  "Pedro update preference",
				Type:  memcontract.TypeUser,
				Scope: memcontract.ScopeGlobal,
			},
			Entity:    "pedro",
			Attribute: "preference",
		}

		decision, err := store.ProposeCandidate(testutil.Context(t), candidate)
		if err != nil {
			t.Fatalf("ProposeCandidate() error = %v", err)
		}
		if decision.Op != memcontract.OpAdd {
			t.Fatalf("decision op = %s, want add", decision.Op.String())
		}
		content, err := store.Read(memcontract.ScopeGlobal, decision.TargetFilename)
		if err != nil {
			t.Fatalf("Read(%q) error = %v", decision.TargetFilename, err)
		}
		if !strings.Contains(string(content), candidate.Content) {
			t.Fatalf("stored content = %q, want candidate content", content)
		}
	})
}

func TestStoreRecordExtractorEvent(t *testing.T) {
	t.Run("Should persist extractor telemetry into memory events", func(t *testing.T) {
		t.Parallel()

		store := newOpenTestStore(t,
			filepath.Join(t.TempDir(), "memory"),
			WithCatalogDatabasePath(filepath.Join(t.TempDir(), storepkg.GlobalDatabaseName)),
		)
		if err := store.EnsureDirs(); err != nil {
			t.Fatalf("EnsureDirs() error = %v", err)
		}
		recordedAt := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
		err := store.RecordExtractorEvent(testutil.Context(t), memoryextractor.Event{
			Op: memoryextractor.EventStarted,
			Turn: memcontract.TurnRecord{
				SessionID:       "sess-extractor",
				WorkspaceID:     "ws-extractor",
				AgentID:         "coder",
				ActorKind:       "agent_root",
				SinceMessageSeq: 4,
				UntilMessageSeq: 5,
				Trigger:         memcontract.TriggerPostMessage,
			},
			At: recordedAt,
		})
		if err != nil {
			t.Fatalf("RecordExtractorEvent() error = %v", err)
		}

		events, err := store.ListMemoryEventSummaries(
			testutil.Context(t),
			nil,
			storepkg.EventSummaryQuery{Type: memoryextractor.EventStarted},
		)
		if err != nil {
			t.Fatalf("ListMemoryEventSummaries() error = %v", err)
		}
		if len(events) != 1 {
			t.Fatalf("len(events) = %d, want 1; events=%#v", len(events), events)
		}
		event := events[0]
		if event.SessionID != "sess-extractor" || event.AgentName != "coder" {
			t.Fatalf("event identity = %#v, want extractor session and agent", event)
		}
		if !event.Timestamp.Equal(recordedAt) {
			t.Fatalf("event timestamp = %v, want %v", event.Timestamp, recordedAt)
		}
	})

	t.Run("Should reject unsupported extractor operations", func(t *testing.T) {
		t.Parallel()

		store := newOpenTestStore(t,
			filepath.Join(t.TempDir(), "memory"),
			WithCatalogDatabasePath(filepath.Join(t.TempDir(), storepkg.GlobalDatabaseName)),
		)
		if err := store.EnsureDirs(); err != nil {
			t.Fatalf("EnsureDirs() error = %v", err)
		}
		err := store.RecordExtractorEvent(testutil.Context(t), memoryextractor.Event{Op: "memory.extractor.unknown"})
		if err == nil {
			t.Fatal("RecordExtractorEvent(unsupported) error = nil, want non-nil")
		}
	})

	t.Run("Should redact and bound failed extractor metadata before persistence", func(t *testing.T) {
		t.Parallel()

		store := newOpenTestStore(t,
			filepath.Join(t.TempDir(), "memory"),
			WithCatalogDatabasePath(filepath.Join(t.TempDir(), storepkg.GlobalDatabaseName)),
		)
		if err := store.EnsureDirs(); err != nil {
			t.Fatalf("EnsureDirs() error = %v", err)
		}
		secret := "sk-test-secret"
		longSuffix := strings.Repeat("x", maxOperationSummaryBytes*2)
		err := store.RecordExtractorEvent(testutil.Context(t), memoryextractor.Event{
			Op: memoryextractor.EventFailed,
			Turn: memcontract.TurnRecord{
				SessionID:   "sess-extractor",
				WorkspaceID: "ws-extractor",
				AgentID:     "coder",
			},
			Metadata: map[string]string{
				"detail": "upstream authorization=Bearer " + secret,
			},
			Error: "request failed: token=" + secret + " " + longSuffix,
		})
		if err != nil {
			t.Fatalf("RecordExtractorEvent(failed) error = %v", err)
		}

		var rawMetadata string
		if err := store.catalog.db.QueryRowContext(
			testutil.Context(t),
			`SELECT metadata FROM memory_events WHERE op = ?`,
			memoryextractor.EventFailed,
		).Scan(&rawMetadata); err != nil {
			t.Fatalf("QueryRowContext(memory_events.metadata) error = %v", err)
		}
		if strings.Contains(rawMetadata, secret) {
			t.Fatalf("metadata = %q leaked secret %q", rawMetadata, secret)
		}
		metadata := map[string]string{}
		if err := json.Unmarshal([]byte(rawMetadata), &metadata); err != nil {
			t.Fatalf("json.Unmarshal(metadata) error = %v", err)
		}
		for _, key := range []string{"error", "detail"} {
			value := metadata[key]
			if strings.Contains(value, secret) {
				t.Fatalf("metadata[%q] = %q leaked secret", key, value)
			}
			if !strings.Contains(value, "[REDACTED]") {
				t.Fatalf("metadata[%q] = %q, want redacted placeholder", key, value)
			}
			if len(value) > maxOperationSummaryBytes {
				t.Fatalf("len(metadata[%q]) = %d, want <= %d", key, len(value), maxOperationSummaryBytes)
			}
		}
	})
}
