package recall

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	memcontract "github.com/compozy/agh/internal/memory/contract"
)

func TestRecallerRecall(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)

	t.Run("Should rank candidates deterministically and cap top K", func(t *testing.T) {
		t.Parallel()

		source := &fakeSource{candidates: []Candidate{
			recallCandidate("chunk-c", memcontract.ScopeWorkspace, "", "deploy", 0.2, 0.1, now),
			recallCandidate("chunk-a", memcontract.ScopeGlobal, "", "auth", 1.0, 0.1, now),
			recallCandidate("chunk-b", memcontract.ScopeWorkspace, "", "session", 0.5, 0.2, now),
		}}
		recaller := New(source, WithClock(func() time.Time { return now }))

		packaged, err := recaller.Recall(
			context.Background(),
			memcontract.Query{QueryText: "auth migration sessions"},
			memcontract.RecallOptions{TopK: 2},
		)
		if err != nil {
			t.Fatalf("Recall() error = %v", err)
		}

		got := packagedIDs(packaged)
		want := []string{"chunk-a", "chunk-b"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("packaged IDs = %#v, want %#v", got, want)
		}
		if len(source.signals) != 2 {
			t.Fatalf("recorded signals = %d, want 2", len(source.signals))
		}
	})

	t.Run("Should short circuit trivial queries with skip event", func(t *testing.T) {
		t.Parallel()

		source := &fakeSource{candidateErr: errors.New("candidate source should not be called")}
		recaller := New(source, WithClock(func() time.Time { return now }))

		packaged, err := recaller.Recall(
			context.Background(),
			memcontract.Query{QueryText: "auth"},
			memcontract.RecallOptions{TopK: 3},
		)
		if err != nil {
			t.Fatalf("Recall(trivial) error = %v", err)
		}
		if len(packaged.Blocks) != 0 {
			t.Fatalf("Recall(trivial) blocks = %d, want 0", len(packaged.Blocks))
		}
		if source.candidateCalls != 0 {
			t.Fatalf("candidate calls = %d, want 0", source.candidateCalls)
		}
		if source.skippedReason != "trivial_query" {
			t.Fatalf("skipped reason = %q, want trivial_query", source.skippedReason)
		}
	})

	t.Run("Should allow explicit trivial query recall", func(t *testing.T) {
		t.Parallel()

		source := &fakeSource{candidates: []Candidate{
			recallCandidate("chunk-launch", memcontract.ScopeWorkspace, "", "launch", 1.0, 0.1, now),
		}}
		recaller := New(source, WithClock(func() time.Time { return now }))

		packaged, err := recaller.Recall(
			context.Background(),
			memcontract.Query{QueryText: "launch"},
			memcontract.RecallOptions{TopK: 3, AllowTrivialQuery: true},
		)
		if err != nil {
			t.Fatalf("Recall(explicit trivial) error = %v", err)
		}
		if source.candidateCalls != 1 {
			t.Fatalf("candidate calls = %d, want 1", source.candidateCalls)
		}
		if source.skippedReason != "" {
			t.Fatalf("skipped reason = %q, want empty", source.skippedReason)
		}
		if got := packagedIDs(packaged); !reflect.DeepEqual(got, []string{"chunk-launch"}) {
			t.Fatalf("packaged IDs = %#v, want chunk-launch", got)
		}
		if len(packaged.Blocks) != 1 || len(packaged.Blocks[0].Entries) != 1 {
			t.Fatalf("packaged blocks = %#v, want one launch entry", packaged.Blocks)
		}
		if got := packaged.Blocks[0].Entries[0].ModTime; !got.Equal(now) {
			t.Fatalf("packaged entry mod time = %v, want %v", got, now)
		}
	})

	t.Run("Should recall two meaningful ASCII tokens", func(t *testing.T) {
		t.Parallel()

		source := &fakeSource{candidates: []Candidate{
			recallCandidate("chunk-auth", memcontract.ScopeWorkspace, "", "auth", 1.0, 0.1, now),
		}}
		recaller := New(source, WithClock(func() time.Time { return now }))

		packaged, err := recaller.Recall(
			context.Background(),
			memcontract.Query{QueryText: "auth sessions"},
			memcontract.RecallOptions{TopK: 3},
		)
		if err != nil {
			t.Fatalf("Recall(two tokens) error = %v", err)
		}
		if source.candidateCalls != 1 {
			t.Fatalf("candidate calls = %d, want 1", source.candidateCalls)
		}
		if got := packagedIDs(packaged); !reflect.DeepEqual(got, []string{"chunk-auth"}) {
			t.Fatalf("packaged IDs = %#v, want chunk-auth", got)
		}
		if source.skippedReason != "" {
			t.Fatalf("skipped reason = %q, want empty", source.skippedReason)
		}
	})

	t.Run("Should enforce scope precedence shadow by ID", func(t *testing.T) {
		t.Parallel()

		global := recallCandidate("chunk-global", memcontract.ScopeGlobal, "", "auth", 1.0, 0.1, now)
		workspace := recallCandidate("chunk-workspace", memcontract.ScopeWorkspace, "", "auth", 0.8, 0.1, now)
		agent := recallCandidate(
			"chunk-agent",
			memcontract.ScopeAgent,
			memcontract.AgentTierWorkspace,
			"auth",
			0.6,
			0.1,
			now,
		)
		agent.AgentName = "coder"
		source := &fakeSource{candidates: []Candidate{global, workspace, agent}}
		recaller := New(source, WithClock(func() time.Time { return now }))

		packaged, err := recaller.Recall(
			context.Background(),
			memcontract.Query{WorkspaceID: "ws_01", AgentName: "coder", QueryText: "auth migration sessions"},
			memcontract.RecallOptions{TopK: 5},
		)
		if err != nil {
			t.Fatalf("Recall(shadow) error = %v", err)
		}

		got := packagedIDs(packaged)
		want := []string{"chunk-agent"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("packaged IDs = %#v, want %#v", got, want)
		}
		if len(source.shadows) != 2 {
			t.Fatalf("shadow events = %d, want 2", len(source.shadows))
		}
		if source.shadows[len(source.shadows)-1].WinnerChunkID != "chunk-agent" {
			t.Fatalf("last shadow winner = %q, want chunk-agent", source.shadows[len(source.shadows)-1].WinnerChunkID)
		}
	})

	t.Run("Should filter already surfaced and system candidates", func(t *testing.T) {
		t.Parallel()

		system := recallCandidate("chunk-system", memcontract.ScopeGlobal, "", "system", 1.0, 1.0, now)
		system.Injection = false
		source := &fakeSource{candidates: []Candidate{
			system,
			recallCandidate("chunk-seen", memcontract.ScopeGlobal, "", "seen", 0.9, 0.1, now),
			recallCandidate("chunk-visible", memcontract.ScopeGlobal, "", "visible", 0.8, 0.1, now),
		}}
		recaller := New(source, WithClock(func() time.Time { return now }))

		packaged, err := recaller.Recall(
			context.Background(),
			memcontract.Query{QueryText: "auth migration sessions"},
			memcontract.RecallOptions{AlreadySurfaced: []string{"chunk-seen"}},
		)
		if err != nil {
			t.Fatalf("Recall(filters) error = %v", err)
		}
		got := packagedIDs(packaged)
		want := []string{"chunk-visible"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("packaged IDs = %#v, want %#v", got, want)
		}
	})

	t.Run("Should package stale entries with stable header across turns", func(t *testing.T) {
		t.Parallel()

		stale := recallCandidate("chunk-stale", memcontract.ScopeGlobal, "", "stale", 1.0, 0.3, now.Add(-72*time.Hour))
		source := &fakeSource{candidates: []Candidate{stale}}
		recaller := New(source, WithClock(func() time.Time { return now }))

		first, err := recaller.Recall(
			context.Background(),
			memcontract.Query{QueryText: "stale migration sessions"},
			memcontract.RecallOptions{TopK: 1},
		)
		if err != nil {
			t.Fatalf("Recall(first) error = %v", err)
		}
		second, err := recaller.Recall(
			context.Background(),
			memcontract.Query{QueryText: "stale migration sessions"},
			memcontract.RecallOptions{TopK: 1},
		)
		if err != nil {
			t.Fatalf("Recall(second) error = %v", err)
		}
		if first.Header.ContentHash == "" {
			t.Fatal("header content hash is empty, want stable cache hash")
		}
		if first.Header.ContentHash != second.Header.ContentHash {
			t.Fatalf("header hash changed from %q to %q", first.Header.ContentHash, second.Header.ContentHash)
		}
		entry := first.Blocks[0].Entries[0]
		if entry.AgeDays != 3 {
			t.Fatalf("entry age = %d, want 3", entry.AgeDays)
		}
		if entry.StalenessBanner == "" {
			t.Fatal("staleness banner is empty, want stale warning")
		}
	})

	t.Run("Should not bubble recall signal update failures", func(t *testing.T) {
		t.Parallel()

		source := &fakeSource{
			candidates:    []Candidate{recallCandidate("chunk-a", memcontract.ScopeGlobal, "", "auth", 1.0, 0.1, now)},
			recordErr:     errors.New("forced signal failure"),
			signalFailure: make([]error, 0),
		}
		recaller := New(source, WithClock(func() time.Time { return now }))

		packaged, err := recaller.Recall(
			context.Background(),
			memcontract.Query{QueryText: "auth migration sessions"},
			memcontract.RecallOptions{TopK: 1},
		)
		if err != nil {
			t.Fatalf("Recall(signal failure) error = %v", err)
		}
		if len(packaged.Blocks) != 1 {
			t.Fatalf("packaged blocks = %d, want 1", len(packaged.Blocks))
		}
		if len(source.signalFailure) != 1 {
			t.Fatalf("signal failure events = %d, want 1", len(source.signalFailure))
		}
	})
}

type fakeSource struct {
	candidates     []Candidate
	candidateErr   error
	recordErr      error
	candidateCalls int
	signals        []Signal
	droppedSignals []Signal
	shadows        []Shadow
	skippedReason  string
	executedCount  int
	signalFailure  []error
}

func (f *fakeSource) Candidates(
	context.Context,
	memcontract.Query,
	memcontract.RecallOptions,
) ([]Candidate, error) {
	f.candidateCalls++
	if f.candidateErr != nil {
		return nil, f.candidateErr
	}
	return append([]Candidate(nil), f.candidates...), nil
}

func (f *fakeSource) RecordRecall(_ context.Context, signals []Signal) error {
	f.signals = append(f.signals, signals...)
	return f.recordErr
}

func (f *fakeSource) RecordRecallExecuted(_ context.Context, _ memcontract.Query, resultCount int) error {
	f.executedCount = resultCount
	return nil
}

func (f *fakeSource) RecordRecallSkipped(_ context.Context, _ memcontract.Query, reason string) error {
	f.skippedReason = reason
	return nil
}

func (f *fakeSource) RecordRecallSignalFailed(_ context.Context, _ memcontract.Query, cause error) error {
	f.signalFailure = append(f.signalFailure, cause)
	return nil
}

func (f *fakeSource) RecordRecallSignalDropped(
	_ context.Context,
	_ memcontract.Query,
	signals []Signal,
	_ int,
) error {
	f.droppedSignals = append(f.droppedSignals, signals...)
	return nil
}

func (f *fakeSource) RecordShadow(_ context.Context, shadow Shadow) error {
	f.shadows = append(f.shadows, shadow)
	return nil
}

func recallCandidate(
	id string,
	scope memcontract.Scope,
	tier memcontract.AgentTier,
	slug string,
	unicodeScore float64,
	trigramScore float64,
	modTime time.Time,
) Candidate {
	return Candidate{
		ChunkID:      id,
		EntryID:      id + "-entry",
		WorkspaceID:  "ws_01",
		Scope:        scope,
		AgentTier:    tier,
		Type:         memcontract.TypeProject,
		Slug:         slug,
		Filename:     slug + ".md",
		Title:        "Memory " + slug,
		Body:         "Remember " + slug + " details.",
		ContentHash:  id + "-hash",
		ModTime:      modTime,
		Injection:    true,
		UnicodeScore: unicodeScore,
		TrigramScore: trigramScore,
	}
}

func packagedIDs(packaged memcontract.Packaged) []string {
	ids := make([]string, 0)
	for _, block := range packaged.Blocks {
		for _, entry := range block.Entries {
			ids = append(ids, entry.ID)
		}
	}
	return ids
}
