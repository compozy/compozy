package daemon

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	apicore "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/store/rundb"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

func TestLoadRunIntegrityMergesNewReasonsIntoStickyState(t *testing.T) {
	t.Parallel()

	runDB, err := rundb.Open(context.Background(), filepath.Join(t.TempDir(), "run-gap", "run.db"))
	if err != nil {
		t.Fatalf("rundb.Open() error = %v", err)
	}
	defer func() {
		_ = runDB.Close()
	}()

	if _, err := runDB.UpsertIntegrity(context.Background(), rundb.RunIntegrityUpdate{
		Incomplete:              true,
		Reasons:                 []string{runIntegrityReasonJournalSubmitDrops},
		JournalNonTerminalDrops: 2,
	}); err != nil {
		t.Fatalf("UpsertIntegrity(existing) error = %v", err)
	}

	manager := &RunManager{
		incompleteRunIDs: make(map[string]struct{}),
	}
	now := time.Date(2026, 4, 21, 5, 0, 0, 0, time.UTC)
	events := []eventspkg.Event{
		mustSessionUpdateEvent(t, "run-gap", 1, now, "hello from persisted transcript"),
		{
			SchemaVersion: eventspkg.SchemaVersion,
			RunID:         "run-gap",
			Seq:           2,
			Timestamp:     now.Add(time.Second),
			Kind:          eventspkg.EventKindRunCompleted,
			Payload:       []byte(`{"summary_message":"done"}`),
		},
	}

	state, err := manager.loadRunIntegrity(
		context.Background(),
		"run-gap",
		apicore.Run{Status: runStatusCompleted},
		runDB,
		events,
		nil,
		&events[len(events)-1],
	)
	if err != nil {
		t.Fatalf("loadRunIntegrity() error = %v", err)
	}
	if !state.Incomplete {
		t.Fatal("state.Incomplete = false, want true")
	}
	if got, want := state.Reasons, []string{
		runIntegrityReasonJournalSubmitDrops,
		runIntegrityReasonTranscriptGap,
	}; !slicesEqualStrings(
		got,
		want,
	) {
		t.Fatalf("state.Reasons = %#v, want %#v", got, want)
	}
	if state.JournalNonTerminalDrops != 2 {
		t.Fatalf("state.JournalNonTerminalDrops = %d, want 2", state.JournalNonTerminalDrops)
	}
	if got := manager.IncompleteRunCount(); got != 1 {
		t.Fatalf("IncompleteRunCount() = %d, want 1", got)
	}

	persisted, err := runDB.GetIntegrity(context.Background())
	if err != nil {
		t.Fatalf("GetIntegrity() error = %v", err)
	}
	if got, want := persisted.Reasons, []string{
		runIntegrityReasonJournalSubmitDrops,
		runIntegrityReasonTranscriptGap,
	}; !slicesEqualStrings(
		got,
		want,
	) {
		t.Fatalf("persisted reasons = %#v, want %#v", got, want)
	}
}

func TestAuditSnapshotIntegrityDetectsEventGapAndMissingTerminalEvent(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 21, 5, 30, 0, 0, time.UTC)
	update := auditSnapshotIntegrity(
		apicore.Run{Status: runStatusCompleted},
		[]eventspkg.Event{
			mustSessionUpdateEvent(t, "run-gap", 1, now, "first"),
			mustSessionUpdateEvent(t, "run-gap", 3, now.Add(2*time.Second), "third"),
		},
		nil,
		nil,
	)
	if !update.Incomplete {
		t.Fatal("update.Incomplete = false, want true")
	}
	if got, want := update.Reasons, []string{
		runIntegrityReasonEventGap,
		runIntegrityReasonTerminalEventMissing,
		runIntegrityReasonTranscriptGap,
	}; !slicesEqualStrings(
		got,
		want,
	) {
		t.Fatalf("update.Reasons = %#v, want %#v", got, want)
	}
}

func TestAssembleSnapshotTranscriptBoundsMessagesAndBytes(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 21, 6, 0, 0, 0, time.UTC)
	rows := make([]rundb.TranscriptMessageRow, 0, 250)
	for seq := 1; seq <= 250; seq++ {
		rows = append(rows, rundb.TranscriptMessageRow{
			Sequence:     uint64(seq),
			Stream:       "session",
			Role:         "assistant",
			Content:      "chunk-" + strings.Repeat("x", 8),
			MetadataJSON: `{"kind":"text"}`,
			Timestamp:    now.Add(time.Duration(seq) * time.Second),
		})
	}

	got := assembleSnapshotTranscript(rows)
	if len(got) != maxSnapshotTranscriptMessages {
		t.Fatalf("len(transcript) = %d, want %d", len(got), maxSnapshotTranscriptMessages)
	}
	if got[0].Sequence != 51 || got[len(got)-1].Sequence != 250 {
		t.Fatalf("bounded transcript sequences = %d..%d, want 51..250", got[0].Sequence, got[len(got)-1].Sequence)
	}

	oversized := assembleSnapshotTranscript([]rundb.TranscriptMessageRow{
		{
			Sequence:     1,
			Stream:       "session",
			Role:         "assistant",
			Content:      strings.Repeat("a", maxSnapshotTranscriptBytes/2),
			MetadataJSON: strings.Repeat("b", maxSnapshotTranscriptBytes/2),
			Timestamp:    now,
		},
		{
			Sequence:     2,
			Stream:       "session",
			Role:         "assistant",
			Content:      strings.Repeat("c", maxSnapshotTranscriptBytes/2),
			MetadataJSON: strings.Repeat("d", maxSnapshotTranscriptBytes/2),
			Timestamp:    now.Add(time.Second),
		},
	})
	if len(oversized) != 1 || oversized[0].Sequence != 2 {
		t.Fatalf("oversized transcript = %#v, want only latest entry", oversized)
	}
}

func TestRecordJournalDropTotalsTracksPerRunDeltas(t *testing.T) {
	t.Parallel()

	manager := &RunManager{
		journalDropsByRun: make(map[string]journalDropTotals),
	}

	manager.recordJournalDropTotals("run-1", 2, 1)
	manager.recordJournalDropTotals("run-1", 2, 1)
	manager.recordJournalDropTotals("run-1", 5, 4)
	manager.recordJournalDropTotals("run-2", 1, 0)

	terminal, nonTerminal := manager.JournalSubmitDropTotals()
	if terminal != 6 || nonTerminal != 4 {
		t.Fatalf("JournalSubmitDropTotals() = (%d, %d), want (6, 4)", terminal, nonTerminal)
	}
}

func mustSessionUpdateEvent(
	t *testing.T,
	runID string,
	seq uint64,
	timestamp time.Time,
	text string,
) eventspkg.Event {
	t.Helper()

	block, err := kinds.NewContentBlock(kinds.TextBlock{Text: text})
	if err != nil {
		t.Fatalf("NewContentBlock() error = %v", err)
	}
	payload, err := json.Marshal(kinds.SessionUpdatePayload{
		Index: 1,
		Update: kinds.SessionUpdate{
			Kind:   kinds.UpdateKindAgentMessageChunk,
			Status: kinds.StatusRunning,
			Blocks: []kinds.ContentBlock{block},
		},
	})
	if err != nil {
		t.Fatalf("marshal session update payload: %v", err)
	}
	return eventspkg.Event{
		SchemaVersion: eventspkg.SchemaVersion,
		RunID:         runID,
		Seq:           seq,
		Timestamp:     timestamp.UTC(),
		Kind:          eventspkg.EventKindSessionUpdate,
		Payload:       payload,
	}
}

func slicesEqualStrings(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for idx := range left {
		if left[idx] != right[idx] {
			return false
		}
	}
	return true
}
