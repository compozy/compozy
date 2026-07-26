package extractor_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/memory"
	memcontract "github.com/compozy/agh/internal/memory/contract"
	"github.com/compozy/agh/internal/memory/extractor"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
)

func TestRuntime(t *testing.T) {
	t.Parallel()

	t.Run("Should enqueue root persisted messages and skip subagent hooks", func(t *testing.T) {
		t.Parallel()

		fake := newFakeExtractor()
		runtime := newTestRuntime(t, t.TempDir(), fake, nil)
		rootPayload := hooks.SessionMessagePersistedPayload{
			PayloadBase: hooks.PayloadBase{
				Event:     hooks.HookSessionMessagePersisted,
				Timestamp: time.Date(2026, 5, 5, 10, 0, 0, 0, time.UTC),
			},
			SessionContext: hooks.SessionContext{
				SessionID:   "sess-root",
				WorkspaceID: "ws-1",
			},
			TurnContext:   hooks.TurnContext{TurnID: "turn-1"},
			MessageSeq:    7,
			Role:          "assistant",
			Text:          "Remember that Pedro prefers concise updates.",
			RootSessionID: "sess-root",
			ActorKind:     "agent_root",
			ActorID:       "sess-root",
		}
		if err := runtime.HandleSessionMessagePersisted(testutil.Context(t), rootPayload); err != nil {
			t.Fatalf("HandleSessionMessagePersisted(root) error = %v", err)
		}
		if err := runtime.Drain(testutil.Context(t)); err != nil {
			t.Fatalf("Drain() error = %v", err)
		}

		subagentPayload := rootPayload
		subagentPayload.SessionID = "sess-child"
		subagentPayload.RootSessionID = "sess-root"
		subagentPayload.ParentSessionID = "sess-root"
		subagentPayload.ActorKind = "agent_subagent"
		if err := runtime.HandleSessionMessagePersisted(testutil.Context(t), subagentPayload); err != nil {
			t.Fatalf("HandleSessionMessagePersisted(subagent) error = %v", err)
		}
		if err := runtime.Drain(testutil.Context(t)); err != nil {
			t.Fatalf("Drain() after subagent error = %v", err)
		}

		turns := fake.turns()
		if len(turns) != 1 {
			t.Fatalf("extracted turns = %d, want 1 root turn", len(turns))
		}
		got := turns[0]
		if got.SessionID != "sess-root" || got.UntilMessageSeq != 7 || got.Trigger != memcontract.TriggerPostMessage {
			t.Fatalf("turn = %#v, want root persisted message turn", got)
		}
		if len(got.Snapshot.Messages) != 1 || got.Snapshot.Messages[0].Content != rootPayload.Text {
			t.Fatalf("snapshot messages = %#v, want persisted assistant text", got.Snapshot.Messages)
		}
	})

	t.Run("Should coalesce queued ranges while one extraction is in flight", func(t *testing.T) {
		t.Parallel()

		fake := newFakeExtractor()
		fake.blockFirst()
		events := &recordingEventSink{}
		runtime := newTestRuntime(t, t.TempDir(), fake, events, extractor.WithCoalesceMax(16))

		if err := runtime.Enqueue(testutil.Context(t), testTurn("sess-merge", 1)); err != nil {
			t.Fatalf("Enqueue(1) error = %v", err)
		}
		fake.waitStarted(t)
		if err := runtime.Enqueue(testutil.Context(t), testTurn("sess-merge", 2)); err != nil {
			t.Fatalf("Enqueue(2) error = %v", err)
		}
		if err := runtime.Enqueue(testutil.Context(t), testTurn("sess-merge", 3)); err != nil {
			t.Fatalf("Enqueue(3) error = %v", err)
		}
		fake.release()
		if err := runtime.Drain(testutil.Context(t)); err != nil {
			t.Fatalf("Drain() error = %v", err)
		}

		turns := fake.turns()
		if len(turns) != 2 {
			t.Fatalf("extracted turns = %d, want initial + merged queued", len(turns))
		}
		if turns[1].SinceMessageSeq != 2 || turns[1].UntilMessageSeq != 3 {
			t.Fatalf("merged queued range = %d..%d, want 2..3", turns[1].SinceMessageSeq, turns[1].UntilMessageSeq)
		}
		if !events.containsOp(extractor.EventCoalesced) {
			t.Fatalf("events = %#v, want %s", events.ops(), extractor.EventCoalesced)
		}
	})

	t.Run("Should backpressure provider sessions with queue capacity without dropping turns", func(t *testing.T) {
		t.Parallel()

		fake := newFakeExtractor()
		fake.blockFirst()
		runtime := newTestRuntime(t, t.TempDir(), fake, nil, extractor.WithQueueCapacity(1))

		if err := runtime.Enqueue(testutil.Context(t), testTurn("sess-pressure-a", 1)); err != nil {
			t.Fatalf("Enqueue(first session) error = %v", err)
		}
		fake.waitStarted(t)
		if err := runtime.Enqueue(testutil.Context(t), testTurn("sess-pressure-b", 2)); err != nil {
			t.Fatalf("Enqueue(second session) error = %v", err)
		}
		waitRuntimeStats(t, runtime, func(stats extractor.RuntimeStats) bool {
			return stats.BackpressuredSessions == 1 && stats.InFlightSessions == 2
		})
		if turns := fake.turns(); len(turns) != 1 {
			t.Fatalf("turns before release = %#v, want only the active provider turn", turns)
		}

		fake.release()
		if err := runtime.Drain(testutil.Context(t)); err != nil {
			t.Fatalf("Drain() error = %v", err)
		}
		turns := fake.turns()
		if len(turns) != 2 {
			t.Fatalf("turns after drain = %#v, want both sessions preserved", turns)
		}
		if turns[0].SessionID != "sess-pressure-a" || turns[1].SessionID != "sess-pressure-b" {
			t.Fatalf("turn order = %#v, want queued provider work after the active session", turns)
		}
	})

	t.Run("Should throttle same-session bursts by coalescing queued turns until ready", func(t *testing.T) {
		t.Parallel()

		fake := newFakeExtractor()
		fake.blockFirst()
		runtime := newTestRuntime(
			t,
			t.TempDir(),
			fake,
			nil,
			extractor.WithThrottleTurns(3),
			extractor.WithThrottleFlushWait(time.Hour),
		)

		if err := runtime.Enqueue(testutil.Context(t), testTurn("sess-throttle", 1)); err != nil {
			t.Fatalf("Enqueue(1) error = %v", err)
		}
		fake.waitStarted(t)
		if err := runtime.Enqueue(testutil.Context(t), testTurn("sess-throttle", 2)); err != nil {
			t.Fatalf("Enqueue(2) error = %v", err)
		}
		fake.release()
		waitRuntimeStats(t, runtime, func(stats extractor.RuntimeStats) bool {
			return stats.InFlightSessions == 0 && stats.QueuedSessions == 1
		})
		if err := runtime.Enqueue(testutil.Context(t), testTurn("sess-throttle", 3)); err != nil {
			t.Fatalf("Enqueue(3) error = %v", err)
		}
		if err := runtime.Enqueue(testutil.Context(t), testTurn("sess-throttle", 4)); err != nil {
			t.Fatalf("Enqueue(4) error = %v", err)
		}
		if err := runtime.Drain(testutil.Context(t)); err != nil {
			t.Fatalf("Drain() error = %v", err)
		}

		turns := fake.turns()
		if len(turns) != 2 {
			t.Fatalf("turns = %#v, want initial + throttled merged turn", turns)
		}
		if turns[1].SinceMessageSeq != 2 || turns[1].UntilMessageSeq != 4 {
			t.Fatalf("throttled range = %d..%d, want 2..4", turns[1].SinceMessageSeq, turns[1].UntilMessageSeq)
		}
		if len(turns[1].Snapshot.Messages) != 3 {
			t.Fatalf("throttled messages = %#v, want all queued messages preserved", turns[1].Snapshot.Messages)
		}
	})

	t.Run("Should drain throttled turns before the threshold without silently dropping content", func(t *testing.T) {
		t.Parallel()

		fake := newFakeExtractor()
		fake.blockFirst()
		runtime := newTestRuntime(
			t,
			t.TempDir(),
			fake,
			nil,
			extractor.WithThrottleTurns(3),
			extractor.WithThrottleFlushWait(time.Hour),
		)
		if err := runtime.Enqueue(testutil.Context(t), testTurn("sess-throttle-drain", 1)); err != nil {
			t.Fatalf("Enqueue(1) error = %v", err)
		}
		fake.waitStarted(t)
		if err := runtime.Enqueue(testutil.Context(t), testTurn("sess-throttle-drain", 2)); err != nil {
			t.Fatalf("Enqueue(2) error = %v", err)
		}
		fake.release()
		waitRuntimeStats(t, runtime, func(stats extractor.RuntimeStats) bool {
			return stats.InFlightSessions == 0 && stats.QueuedSessions == 1
		})
		if err := runtime.Drain(testutil.Context(t)); err != nil {
			t.Fatalf("Drain() error = %v", err)
		}

		turns := fake.turns()
		if len(turns) != 2 {
			t.Fatalf("turns = %#v, want drain to flush the throttled queued turn", turns)
		}
		if turns[1].UntilMessageSeq != 2 || turns[1].Snapshot.Messages[0].Content != "message 2" {
			t.Fatalf("drained turn = %#v, want queued non-empty content preserved", turns[1])
		}
	})

	t.Run("Should drop the oldest queued range after the coalescing cap", func(t *testing.T) {
		t.Parallel()

		fake := newFakeExtractor()
		fake.blockFirst()
		events := &recordingEventSink{}
		runtime := newTestRuntime(t, t.TempDir(), fake, events, extractor.WithCoalesceMax(1))

		for seq := int64(1); seq <= 4; seq++ {
			if err := runtime.Enqueue(testutil.Context(t), testTurn("sess-drop", seq)); err != nil {
				t.Fatalf("Enqueue(%d) error = %v", seq, err)
			}
			if seq == 1 {
				fake.waitStarted(t)
			}
		}
		fake.release()
		if err := runtime.Drain(testutil.Context(t)); err != nil {
			t.Fatalf("Drain() error = %v", err)
		}

		turns := fake.turns()
		if len(turns) != 2 {
			t.Fatalf("extracted turns = %d, want first + retained newest", len(turns))
		}
		if turns[1].SinceMessageSeq != 4 || turns[1].UntilMessageSeq != 4 {
			t.Fatalf("retained range = %d..%d, want newest 4..4", turns[1].SinceMessageSeq, turns[1].UntilMessageSeq)
		}
		if dropped := events.eventsForOp(extractor.EventDropped); len(dropped) != 2 {
			t.Fatalf("dropped events = %#v, want one drop per overflowed queued turn", dropped)
		}
		if events.containsOp(extractor.EventCoalesced) {
			t.Fatalf("events = %#v, want no coalescing after the configured cap", events.ops())
		}
	})

	t.Run("Should reject new work while drain is in progress", func(t *testing.T) {
		t.Parallel()

		fake := newFakeExtractor()
		fake.blockFirst()
		runtime := newTestRuntime(t, t.TempDir(), fake, nil)
		if err := runtime.Enqueue(testutil.Context(t), testTurn("sess-drain", 1)); err != nil {
			t.Fatalf("Enqueue() error = %v", err)
		}
		fake.waitStarted(t)

		drainErr := make(chan error, 1)
		go func() {
			drainErr <- runtime.Drain(testutil.Context(t))
		}()

		drainingTurn := testTurn("sess-drain-rejected", 2)
		drainingTurn.Snapshot.Messages[0].Content = " \n\t "
		waitUntilDrainBlocksEnqueue(t, runtime, drainingTurn)

		fake.release()
		if err := <-drainErr; err != nil {
			t.Fatalf("Drain() error = %v", err)
		}
		turns := fake.turns()
		if len(turns) != 1 || turns[0].UntilMessageSeq != 1 {
			t.Fatalf("turns after drain = %#v, want only the original in-flight turn", turns)
		}
	})

	t.Run("Should close by joining workers, draining extractor, and rejecting new work", func(t *testing.T) {
		t.Parallel()

		fake := newFakeExtractor()
		runtime := newTestRuntime(t, t.TempDir(), fake, nil)
		if err := runtime.Enqueue(testutil.Context(t), testTurn("sess-close", 1)); err != nil {
			t.Fatalf("Enqueue() error = %v", err)
		}
		if err := runtime.Close(testutil.Context(t)); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
		if fake.drainCount() != 1 {
			t.Fatalf("extractor drain count = %d, want 1", fake.drainCount())
		}
		if err := runtime.Enqueue(testutil.Context(t), testTurn("sess-close", 2)); err == nil {
			t.Fatal("Enqueue() after Close error = nil, want non-nil")
		}
	})

	t.Run("Should no-op when a tool write occurred in the same turn", func(t *testing.T) {
		t.Parallel()

		fake := newFakeExtractor()
		runtime := newTestRuntime(t, t.TempDir(), fake, nil)
		runtime.RecordToolWrite("sess-tool", 7)
		if err := runtime.HandleSessionMessagePersisted(
			testutil.Context(t),
			testPersistedPayload("sess-tool", 7),
		); err != nil {
			t.Fatalf("HandleSessionMessagePersisted(tool write turn) error = %v", err)
		}
		if err := runtime.Drain(testutil.Context(t)); err != nil {
			t.Fatalf("Drain() error = %v", err)
		}
		if turns := fake.turns(); len(turns) != 0 {
			t.Fatalf("extracted turns = %#v, want no extraction after tool write", turns)
		}
	})

	t.Run("Should skip persisted messages without extractable content", func(t *testing.T) {
		t.Parallel()

		fake := newFakeExtractor()
		events := &recordingEventSink{}
		runtime := newTestRuntime(t, t.TempDir(), fake, events)
		payload := testPersistedPayload("sess-empty", 1)
		payload.Text = " \n\t "
		if err := runtime.HandleSessionMessagePersisted(testutil.Context(t), payload); err != nil {
			t.Fatalf("HandleSessionMessagePersisted(empty text) error = %v", err)
		}
		if err := runtime.Drain(testutil.Context(t)); err != nil {
			t.Fatalf("Drain() error = %v", err)
		}
		if turns := fake.turns(); len(turns) != 0 {
			t.Fatalf("extracted turns = %#v, want no extraction for empty persisted text", turns)
		}
		if events.containsOp(extractor.EventStarted) {
			t.Fatalf("events = %#v, want no extractor start for empty persisted text", events.ops())
		}
		stats := runtime.Stats()
		if stats.SkippedTurns != 1 {
			t.Fatalf("Stats().SkippedTurns = %d, want 1", stats.SkippedTurns)
		}
		dropped := events.eventsForOp(extractor.EventDropped)
		if len(dropped) != 1 {
			t.Fatalf("dropped events = %#v, want one skipped-turn event", dropped)
		}
		if dropped[0].Metadata["reason"] != "empty_snapshot" || dropped[0].Metadata["message_count"] != "1" {
			t.Fatalf("dropped metadata = %#v, want empty_snapshot skip without content", dropped[0].Metadata)
		}
	})

	t.Run("Should consume multiple pending tool write markers independently", func(t *testing.T) {
		t.Parallel()

		fake := newFakeExtractor()
		runtime := newTestRuntime(t, t.TempDir(), fake, nil)
		runtime.RecordToolWrite("sess-tool-multi", 7)
		runtime.RecordToolWrite("sess-tool-multi", 8)
		for _, seq := range []int64{7, 8, 9} {
			if err := runtime.HandleSessionMessagePersisted(
				testutil.Context(t),
				testPersistedPayload("sess-tool-multi", seq),
			); err != nil {
				t.Fatalf("HandleSessionMessagePersisted(seq %d) error = %v", seq, err)
			}
		}
		if err := runtime.Drain(testutil.Context(t)); err != nil {
			t.Fatalf("Drain() error = %v", err)
		}
		turns := fake.turns()
		if len(turns) != 1 || turns[0].UntilMessageSeq != 9 {
			t.Fatalf("extracted turns = %#v, want only unmarked turn 9", turns)
		}
	})

	t.Run("Should keep extracting later turns after an older tool write marker", func(t *testing.T) {
		t.Parallel()

		fake := newFakeExtractor()
		runtime := newTestRuntime(t, t.TempDir(), fake, nil)
		runtime.RecordToolWrite("sess-tool-prev", 6)
		if err := runtime.HandleSessionMessagePersisted(
			testutil.Context(t),
			testPersistedPayload("sess-tool-prev", 7),
		); err != nil {
			t.Fatalf("HandleSessionMessagePersisted(later turn) error = %v", err)
		}
		if err := runtime.Drain(testutil.Context(t)); err != nil {
			t.Fatalf("Drain() error = %v", err)
		}
		turns := fake.turns()
		if len(turns) != 1 || turns[0].UntilMessageSeq != 7 {
			t.Fatalf("extracted turns = %#v, want later turn 7", turns)
		}
	})

	t.Run("Should consume sequence-less tool write marker once", func(t *testing.T) {
		t.Parallel()

		fake := newFakeExtractor()
		runtime := newTestRuntime(t, t.TempDir(), fake, nil)
		runtime.RecordToolWrite("sess-tool-next", 0)
		if err := runtime.HandleSessionMessagePersisted(
			testutil.Context(t),
			testPersistedPayload("sess-tool-next", 1),
		); err != nil {
			t.Fatalf("HandleSessionMessagePersisted(first after marker) error = %v", err)
		}
		if err := runtime.HandleSessionMessagePersisted(
			testutil.Context(t),
			testPersistedPayload("sess-tool-next", 2),
		); err != nil {
			t.Fatalf("HandleSessionMessagePersisted(second after marker) error = %v", err)
		}
		if err := runtime.Drain(testutil.Context(t)); err != nil {
			t.Fatalf("Drain() error = %v", err)
		}
		turns := fake.turns()
		if len(turns) != 1 || turns[0].UntilMessageSeq != 2 {
			t.Fatalf("extracted turns = %#v, want only second turn extracted", turns)
		}
	})

	t.Run("Should write extracted candidates into the inbox and record completion", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		fake := newFakeExtractor()
		fake.setResult([]memcontract.Candidate{testCandidate("Pedro prefers brief updates.")})
		events := &recordingEventSink{}
		runtime := newTestRuntime(t, root, fake, events)
		if err := runtime.Enqueue(testutil.Context(t), testTurn("sess-inbox", 5)); err != nil {
			t.Fatalf("Enqueue() error = %v", err)
		}
		if err := runtime.Drain(testutil.Context(t)); err != nil {
			t.Fatalf("Drain() error = %v", err)
		}

		files, err := filepath.Glob(filepath.Join(root, "_inbox", "sess-inbox", "*.jsonl"))
		if err != nil {
			t.Fatalf("Glob() error = %v", err)
		}
		if len(files) != 1 {
			t.Fatalf("inbox files = %#v, want one JSONL file", files)
		}
		raw, err := os.ReadFile(files[0])
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		var stored memcontract.Candidate
		if err := json.Unmarshal(bytes.TrimSpace(raw), &stored); err != nil {
			t.Fatalf("json.Unmarshal(inbox candidate) error = %v", err)
		}
		if stored.Origin != memcontract.OriginExtractor {
			t.Fatalf("candidate origin = %q, want extractor", stored.Origin)
		}
		if stored.Metadata["session_id"] != "sess-inbox" || stored.Metadata["until_message_seq"] != "5" {
			t.Fatalf("candidate metadata = %#v, want session/sequence lineage", stored.Metadata)
		}
		if !events.containsOp(extractor.EventCompleted) {
			t.Fatalf("events = %#v, want %s", events.ops(), extractor.EventCompleted)
		}
	})

	t.Run("Should record extractor failures without producing inbox files", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		fake := newFakeExtractor()
		fake.setError(errors.New("extractor down"))
		events := &recordingEventSink{}
		runtime := newTestRuntime(t, root, fake, events)
		if err := runtime.Enqueue(testutil.Context(t), testTurn("sess-fail", 1)); err != nil {
			t.Fatalf("Enqueue() error = %v", err)
		}
		if err := runtime.Drain(testutil.Context(t)); err != nil {
			t.Fatalf("Drain() error = %v", err)
		}
		files, err := filepath.Glob(filepath.Join(root, "_inbox", "sess-fail", "*.jsonl"))
		if err != nil {
			t.Fatalf("Glob() error = %v", err)
		}
		if len(files) != 0 {
			t.Fatalf("inbox files = %#v, want none after extractor failure", files)
		}
		if !events.containsOp(extractor.EventFailed) {
			t.Fatalf("events = %#v, want %s", events.ops(), extractor.EventFailed)
		}
	})

	t.Run("Should reject invalid runtime inputs", func(t *testing.T) {
		t.Parallel()

		fake := newFakeExtractor()
		if _, err := extractor.NewRuntime(missingContext(), t.TempDir(), fake); err == nil {
			t.Fatal("NewRuntime(nil ctx) error = nil, want non-nil")
		}
		if _, err := extractor.NewRuntime(testutil.Context(t), "", fake); err == nil {
			t.Fatal("NewRuntime(empty root) error = nil, want non-nil")
		}
		if _, err := extractor.NewRuntime(testutil.Context(t), t.TempDir(), nil); err == nil {
			t.Fatal("NewRuntime(nil extractor) error = nil, want non-nil")
		}
		runtime := newTestRuntime(t, t.TempDir(), fake, nil, extractor.WithLogger(slog.Default()))
		if err := runtime.Enqueue(missingContext(), testTurn("sess-invalid", 1)); err == nil {
			t.Fatal("Enqueue(nil ctx) error = nil, want non-nil")
		}
		invalid := testTurn("sess-invalid", 1)
		invalid.UntilMessageSeq = 0
		if err := runtime.Enqueue(testutil.Context(t), invalid); err == nil {
			t.Fatal("Enqueue(invalid turn) error = nil, want non-nil")
		}
	})
}

func TestInbox(t *testing.T) {
	t.Parallel()

	t.Run("Should route extractor output through controller proposals", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		catalogPath := filepath.Join(t.TempDir(), "agh.db")
		memStore := memory.NewStore(root, memory.WithCatalogDatabasePath(catalogPath))
		openMemoryCatalog(t, memStore)
		if err := memStore.EnsureDirs(); err != nil {
			t.Fatalf("EnsureDirs() error = %v", err)
		}
		candidate := testCandidate("Pedro prefers brief Portuguese progress updates.")
		producer, err := extractor.NewProducer(root, testClock)
		if err != nil {
			t.Fatalf("NewProducer() error = %v", err)
		}
		path, count, err := producer.Write(
			testutil.Context(t),
			testTurn("sess-flow", 9),
			[]memcontract.Candidate{candidate},
		)
		if err != nil {
			t.Fatalf("Producer.Write() error = %v", err)
		}
		if count != 1 {
			t.Fatalf("Producer.Write() count = %d, want 1", count)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("stat inbox file error = %v", err)
		}

		consumer, err := extractor.NewInboxConsumer(root, memStore, extractor.WithConsumerClock(testClock))
		if err != nil {
			t.Fatalf("NewInboxConsumer() error = %v", err)
		}
		result, err := consumer.ConsumeOnce(testutil.Context(t))
		if err != nil {
			t.Fatalf("ConsumeOnce() error = %v", err)
		}
		if result.Proposed != 1 || len(result.Decisions) != 1 {
			t.Fatalf("ConsumeOnce() proposed = %d decisions = %d, want 1/1", result.Proposed, len(result.Decisions))
		}
		decision := result.Decisions[0]
		if decision.Op != memcontract.OpAdd {
			t.Fatalf("decision op = %s, want add", decision.Op.String())
		}
		content, err := memStore.Read(memcontract.ScopeGlobal, decision.TargetFilename)
		if err != nil {
			t.Fatalf("Store.Read(%q) error = %v", decision.TargetFilename, err)
		}
		if !strings.Contains(string(content), candidate.Content) {
			t.Fatalf("stored content = %q, want candidate body", content)
		}
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("inbox file stat error = %v, want not exist after consume", err)
		}
	})

	t.Run("Should move decode failures to system DLQ outside prompt packaging", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		inboxDir := filepath.Join(root, "_inbox", "sess-dlq")
		if err := os.MkdirAll(inboxDir, 0o755); err != nil {
			t.Fatalf("MkdirAll() error = %v", err)
		}
		badPath := filepath.Join(inboxDir, "bad.jsonl")
		if err := os.WriteFile(badPath, []byte("{not-json\n"), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		consumer, err := extractor.NewInboxConsumer(
			root,
			&recordingProposalSink{},
			extractor.WithConsumerClock(testClock),
		)
		if err != nil {
			t.Fatalf("NewInboxConsumer() error = %v", err)
		}

		result, err := consumer.ConsumeOnce(testutil.Context(t))
		if err == nil {
			t.Fatal("ConsumeOnce() error = nil, want decode failure")
		}
		if result.Failed != 1 || len(result.Failures) != 1 {
			t.Fatalf("ConsumeOnce() failed = %d failures = %d, want 1/1", result.Failed, len(result.Failures))
		}
		if _, err := os.Stat(result.Failures[0]); err != nil {
			t.Fatalf("stat dlq file error = %v", err)
		}
		if _, err := os.Stat(badPath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("bad inbox file stat error = %v, want not exist after DLQ move", err)
		}
		memStore := memory.NewStore(root)
		headers, err := memStore.Scan(memcontract.ScopeGlobal)
		if err != nil {
			t.Fatalf("Store.Scan() error = %v", err)
		}
		if len(headers) != 0 {
			t.Fatalf("Store.Scan() headers = %#v, want no prompt-visible _system files", headers)
		}
	})

	t.Run("Should move controller failures to DLQ and emit failure telemetry", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		producer, err := extractor.NewProducer(root, testClock)
		if err != nil {
			t.Fatalf("NewProducer() error = %v", err)
		}
		path, count, err := producer.Write(
			testutil.Context(t),
			testTurn("sess-controller-fail", 2),
			[]memcontract.Candidate{testCandidate("Pedro prefers detailed QA evidence.")},
		)
		if err != nil {
			t.Fatalf("Producer.Write() error = %v", err)
		}
		if count != 1 {
			t.Fatalf("Producer.Write() count = %d, want 1", count)
		}
		events := &recordingEventSink{}
		consumer, err := extractor.NewInboxConsumer(
			root,
			&recordingProposalSink{err: errors.New("controller rejected")},
			extractor.WithConsumerClock(testClock),
			extractor.WithConsumerEventSink(events),
		)
		if err != nil {
			t.Fatalf("NewInboxConsumer() error = %v", err)
		}

		result, err := consumer.ConsumeOnce(testutil.Context(t))
		if err == nil {
			t.Fatal("ConsumeOnce() error = nil, want controller failure")
		}
		if result.Failed != 1 || result.Proposed != 0 {
			t.Fatalf("ConsumeOnce() failed/proposed = %d/%d, want 1/0", result.Failed, result.Proposed)
		}
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("inbox file stat error = %v, want not exist after DLQ move", err)
		}
		if !events.containsOp(extractor.EventFailed) {
			t.Fatalf("events = %#v, want %s", events.ops(), extractor.EventFailed)
		}
	})

	t.Run("Should recover claimed processing files after restart", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		producer, err := extractor.NewProducer(root, testClock)
		if err != nil {
			t.Fatalf("NewProducer() error = %v", err)
		}
		path, _, err := producer.Write(
			testutil.Context(t),
			testTurn("sess-processing-recover", 4),
			[]memcontract.Candidate{testCandidate("Pedro prefers recovered inbox claims.")},
		)
		if err != nil {
			t.Fatalf("Producer.Write() error = %v", err)
		}
		processingPath := path + ".processing"
		if err := os.Rename(path, processingPath); err != nil {
			t.Fatalf("Rename(processing) error = %v", err)
		}
		sink := &recordingProposalSink{}
		consumer, err := extractor.NewInboxConsumer(root, sink, extractor.WithConsumerClock(testClock))
		if err != nil {
			t.Fatalf("NewInboxConsumer() error = %v", err)
		}

		result, err := consumer.ConsumeOnce(testutil.Context(t))
		if err != nil {
			t.Fatalf("ConsumeOnce() error = %v", err)
		}
		if result.Proposed != 1 || len(sink.candidates) != 1 {
			t.Fatalf("ConsumeOnce() proposed/candidates = %d/%d, want 1/1", result.Proposed, len(sink.candidates))
		}
		if _, err := os.Stat(processingPath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("processing file stat error = %v, want not exist after recovery", err)
		}
	})

	t.Run("Should requeue canceled controller proposals without DLQ", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		producer, err := extractor.NewProducer(root, testClock)
		if err != nil {
			t.Fatalf("NewProducer() error = %v", err)
		}
		path, _, err := producer.Write(
			testutil.Context(t),
			testTurn("sess-canceled-proposal", 5),
			[]memcontract.Candidate{testCandidate("Pedro prefers retryable inbox cancellation.")},
		)
		if err != nil {
			t.Fatalf("Producer.Write() error = %v", err)
		}
		consumer, err := extractor.NewInboxConsumer(
			root,
			&recordingProposalSink{err: context.Canceled},
			extractor.WithConsumerClock(testClock),
		)
		if err != nil {
			t.Fatalf("NewInboxConsumer() error = %v", err)
		}

		result, err := consumer.ConsumeOnce(testutil.Context(t))
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("ConsumeOnce() error = %v, want context.Canceled", err)
		}
		if result.Failed != 0 || result.Proposed != 0 {
			t.Fatalf("ConsumeOnce() failed/proposed = %d/%d, want 0/0", result.Failed, result.Proposed)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("inbox file stat error = %v, want original file requeued", err)
		}
		failures, err := filepath.Glob(filepath.Join(root, "_system", "extractor", "failures", "*.json"))
		if err != nil {
			t.Fatalf("Glob(failures) error = %v", err)
		}
		if len(failures) != 0 {
			t.Fatalf("DLQ failures = %#v, want none", failures)
		}
	})

	t.Run("Should replay multi-candidate DLQ files idempotently", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		catalogPath := filepath.Join(t.TempDir(), "agh.db")
		memStore := memory.NewStore(root, memory.WithCatalogDatabasePath(catalogPath))
		openMemoryCatalog(t, memStore)
		if err := memStore.EnsureDirs(); err != nil {
			t.Fatalf("EnsureDirs() error = %v", err)
		}
		first := testCandidate("First replay candidate.")
		first.Entity = "first"
		first.Attribute = "preference"
		first.Frontmatter.Name = "First Replay"
		second := testCandidate("Second replay candidate.")
		second.Entity = "second"
		second.Attribute = "preference"
		second.Frontmatter.Name = "Second Replay"
		producer, err := extractor.NewProducer(root, testClock)
		if err != nil {
			t.Fatalf("NewProducer() error = %v", err)
		}
		if _, _, err := producer.Write(
			testutil.Context(t),
			testTurn("sess-replay", 3),
			[]memcontract.Candidate{first, second},
		); err != nil {
			t.Fatalf("Producer.Write() error = %v", err)
		}
		sink := &failSecondCandidateOnceSink{delegate: memStore}
		consumer, err := extractor.NewInboxConsumer(root, sink, extractor.WithConsumerClock(testClock))
		if err != nil {
			t.Fatalf("NewInboxConsumer(first) error = %v", err)
		}
		firstResult, err := consumer.ConsumeOnce(testutil.Context(t))
		if err == nil {
			t.Fatal("ConsumeOnce(first) error = nil, want controlled second-candidate failure")
		}
		if firstResult.Proposed != 1 || firstResult.Failed != 1 || len(firstResult.Failures) != 1 {
			t.Fatalf("first ConsumeOnce result = %#v, want one proposed and one DLQ failure", firstResult)
		}

		rawFailure, err := os.ReadFile(firstResult.Failures[0])
		if err != nil {
			t.Fatalf("ReadFile(DLQ) error = %v", err)
		}
		var dlq map[string]string
		if err := json.Unmarshal(rawFailure, &dlq); err != nil {
			t.Fatalf("json.Unmarshal(DLQ) error = %v", err)
		}
		replayDir := filepath.Join(root, "_inbox", "sess-replay")
		if err := os.MkdirAll(replayDir, 0o755); err != nil {
			t.Fatalf("MkdirAll(replay dir) error = %v", err)
		}
		replayPath := filepath.Join(replayDir, "replay.jsonl")
		if err := os.WriteFile(replayPath, []byte(dlq["content"]), 0o644); err != nil {
			t.Fatalf("WriteFile(replay inbox) error = %v", err)
		}

		replayConsumer, err := extractor.NewInboxConsumer(root, sink, extractor.WithConsumerClock(testClock))
		if err != nil {
			t.Fatalf("NewInboxConsumer(replay) error = %v", err)
		}
		replayResult, err := replayConsumer.ConsumeOnce(testutil.Context(t))
		if err != nil {
			t.Fatalf("ConsumeOnce(replay) error = %v", err)
		}
		if replayResult.Proposed != 2 || replayResult.Failed != 0 || len(replayResult.Decisions) != 2 {
			t.Fatalf("replay result = %#v, want both candidates accepted idempotently", replayResult)
		}
		if _, err := memStore.Read(memcontract.ScopeGlobal, replayResult.Decisions[1].TargetFilename); err != nil {
			t.Fatalf("Store.Read(second replay target) error = %v", err)
		}
	})

	t.Run("Should handle empty inbox and invalid construction inputs", func(t *testing.T) {
		t.Parallel()

		if _, err := extractor.NewProducer("", testClock); err == nil {
			t.Fatal("NewProducer(empty root) error = nil, want non-nil")
		}
		if _, err := extractor.NewInboxConsumer("", &recordingProposalSink{}); err == nil {
			t.Fatal("NewInboxConsumer(empty root) error = nil, want non-nil")
		}
		if _, err := extractor.NewInboxConsumer(t.TempDir(), nil); err == nil {
			t.Fatal("NewInboxConsumer(nil sink) error = nil, want non-nil")
		}
		consumer, err := extractor.NewInboxConsumer(
			t.TempDir(),
			&recordingProposalSink{},
			extractor.WithConsumerClock(testClock),
			extractor.WithConsumerLogger(slog.Default()),
		)
		if err != nil {
			t.Fatalf("NewInboxConsumer() error = %v", err)
		}
		result, err := consumer.ConsumeOnce(testutil.Context(t))
		if err != nil {
			t.Fatalf("ConsumeOnce(empty) error = %v", err)
		}
		if result.Files != 0 || result.Proposed != 0 || result.Failed != 0 {
			t.Fatalf("ConsumeOnce(empty) = %#v, want zero result", result)
		}
	})
}

func openMemoryCatalog(t *testing.T, store *memory.Store) {
	t.Helper()
	if err := store.OpenCatalog(testutil.Context(t)); err != nil {
		t.Fatalf("Store.OpenCatalog() error = %v", err)
	}
	t.Cleanup(func() {
		if err := store.CloseCatalog(testutil.Context(t)); err != nil {
			t.Errorf("Store.CloseCatalog() error = %v", err)
		}
	})
}

func newTestRuntime(
	t *testing.T,
	root string,
	fake *fakeExtractor,
	events extractor.EventSink,
	opts ...extractor.Option,
) *extractor.Runtime {
	t.Helper()
	options := []extractor.Option{extractor.WithClock(testClock)}
	if events != nil {
		options = append(options, extractor.WithEventSink(events))
	}
	options = append(options, opts...)
	runtime, err := extractor.NewRuntime(testutil.Context(t), root, fake, options...)
	if err != nil {
		t.Fatalf("NewRuntime() error = %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := runtime.Close(ctx); err != nil && !strings.Contains(err.Error(), "runtime is closed") {
			t.Fatalf("runtime cleanup Close() error = %v", err)
		}
	})
	return runtime
}

func testTurn(sessionID string, seq int64) memcontract.TurnRecord {
	return memcontract.TurnRecord{
		SessionID:       sessionID,
		RootSessionID:   sessionID,
		AgentID:         sessionID,
		ActorKind:       "agent_root",
		WorkspaceID:     "ws-1",
		SinceMessageSeq: seq,
		UntilMessageSeq: seq,
		Snapshot: memcontract.TranscriptSnapshot{
			Messages: []memcontract.TranscriptMessage{{
				Sequence: seq,
				Role:     "assistant",
				Content:  "message " + strconv.FormatInt(seq, 10),
				At:       testClock(),
			}},
		},
		Trigger: memcontract.TriggerPostMessage,
	}
}

func testPersistedPayload(sessionID string, seq int64) hooks.SessionMessagePersistedPayload {
	return hooks.SessionMessagePersistedPayload{
		PayloadBase: hooks.PayloadBase{
			Event:     hooks.HookSessionMessagePersisted,
			Timestamp: testClock(),
		},
		SessionContext: hooks.SessionContext{
			SessionID:   sessionID,
			WorkspaceID: "ws-1",
		},
		MessageSeq:    seq,
		Role:          "assistant",
		Text:          "message " + strconv.FormatInt(seq, 10),
		RootSessionID: sessionID,
		ActorKind:     "agent_root",
		ActorID:       sessionID,
	}
}

func testCandidate(content string) memcontract.Candidate {
	return memcontract.Candidate{
		Scope:   memcontract.ScopeGlobal,
		Origin:  memcontract.OriginExtractor,
		Content: content,
		Frontmatter: memcontract.Header{
			Name:  "Pedro preference",
			Type:  memcontract.TypeUser,
			Scope: memcontract.ScopeGlobal,
		},
		Entity:    "pedro",
		Attribute: "preference",
	}
}

func testClock() time.Time {
	return time.Date(2026, 5, 5, 10, 0, 0, 0, time.UTC)
}

func missingContext() context.Context {
	return nil
}

func waitRuntimeStats(
	t *testing.T,
	runtime *extractor.Runtime,
	match func(extractor.RuntimeStats) bool,
) extractor.RuntimeStats {
	t.Helper()
	timeout := 5 * time.Second
	if testDeadline, ok := t.Deadline(); ok {
		if remaining := time.Until(testDeadline) / 2; remaining > 0 && remaining < timeout {
			timeout = remaining
		}
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		stats := runtime.Stats()
		if match(stats) {
			return stats
		}
		select {
		case <-deadline.C:
			t.Fatalf("timed out waiting for runtime stats, last stats = %#v", stats)
		case <-tick.C:
		}
	}
}

func waitUntilDrainBlocksEnqueue(
	t *testing.T,
	runtime *extractor.Runtime,
	turn memcontract.TurnRecord,
) {
	t.Helper()
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()

	for {
		err := runtime.Enqueue(testutil.Context(t), turn)
		switch {
		case err == nil:
		case err.Error() == "memory extractor: runtime is draining":
			return
		default:
			t.Fatalf("Enqueue() while waiting for drain barrier error = %v", err)
		}

		select {
		case <-timeout.C:
			t.Fatalf("timed out waiting for drain to reject new work")
		case <-tick.C:
		}
	}
}

func TestEvent(t *testing.T) {
	t.Parallel()

	t.Run("Should normalize identity fields from the turn", func(t *testing.T) {
		t.Parallel()

		event := extractor.Event{
			Op: " " + extractor.EventStarted + " ",
			Turn: memcontract.TurnRecord{
				SessionID:   "sess-event",
				WorkspaceID: "ws-event",
				AgentID:     "agent-event",
				ActorKind:   "agent_root",
			},
		}.Normalize(testClock)
		if event.Op != extractor.EventStarted {
			t.Fatalf("event op = %q, want %q", event.Op, extractor.EventStarted)
		}
		if event.SessionID != "sess-event" || event.WorkspaceID != "ws-event" || event.AgentID != "agent-event" {
			t.Fatalf("event identity = %#v, want turn-derived identity", event)
		}
		if event.Metadata == nil || event.At.IsZero() {
			t.Fatalf("event metadata/at = %#v/%v, want populated", event.Metadata, event.At)
		}
	})
}

type fakeExtractor struct {
	mu       sync.Mutex
	started  chan struct{}
	releaseC chan struct{}
	turnLog  []memcontract.TurnRecord
	drains   int
	result   []memcontract.Candidate
	err      error
}

func newFakeExtractor() *fakeExtractor {
	return &fakeExtractor{
		started: make(chan struct{}, 1),
		result:  []memcontract.Candidate{},
	}
}

func (f *fakeExtractor) blockFirst() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.releaseC = make(chan struct{})
}

func (f *fakeExtractor) setResult(result []memcontract.Candidate) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.result = append([]memcontract.Candidate(nil), result...)
}

func (f *fakeExtractor) setError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

func (f *fakeExtractor) Extract(ctx context.Context, turn memcontract.TurnRecord) ([]memcontract.Candidate, error) {
	f.mu.Lock()
	f.turnLog = append(f.turnLog, turn)
	release := f.releaseC
	result := append([]memcontract.Candidate(nil), f.result...)
	err := f.err
	if len(f.turnLog) == 1 {
		select {
		case f.started <- struct{}{}:
		default:
		}
	}
	f.mu.Unlock()

	if release != nil {
		select {
		case <-release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return result, err
}

func (f *fakeExtractor) Drain(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.drains++
	return nil
}

func (f *fakeExtractor) waitStarted(t *testing.T) {
	t.Helper()
	select {
	case <-f.started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for extractor start")
	}
}

func (f *fakeExtractor) release() {
	f.mu.Lock()
	release := f.releaseC
	f.releaseC = nil
	f.mu.Unlock()
	if release != nil {
		close(release)
	}
}

func (f *fakeExtractor) turns() []memcontract.TurnRecord {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]memcontract.TurnRecord(nil), f.turnLog...)
}

func (f *fakeExtractor) drainCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.drains
}

type recordingEventSink struct {
	mu     sync.Mutex
	events []extractor.Event
}

func (s *recordingEventSink) RecordExtractorEvent(_ context.Context, event extractor.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
	return nil
}

func (s *recordingEventSink) containsOp(op string) bool {
	return slices.Contains(s.ops(), op)
}

func (s *recordingEventSink) eventsForOp(op string) []extractor.Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	events := make([]extractor.Event, 0)
	for _, event := range s.events {
		if event.Op == op {
			events = append(events, event)
		}
	}
	return events
}

func (s *recordingEventSink) ops() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	ops := make([]string, 0, len(s.events))
	for _, event := range s.events {
		ops = append(ops, event.Op)
	}
	return ops
}

type recordingProposalSink struct {
	candidates []memcontract.Candidate
	err        error
}

func (s *recordingProposalSink) ProposeCandidate(
	_ context.Context,
	candidate memcontract.Candidate,
) (memcontract.Decision, error) {
	if s.err != nil {
		return memcontract.Decision{}, s.err
	}
	s.candidates = append(s.candidates, candidate)
	return memcontract.Decision{ID: store.NewID("dec"), Op: memcontract.OpNoop}, nil
}

type failSecondCandidateOnceSink struct {
	delegate *memory.Store
	failed   bool
}

func (s *failSecondCandidateOnceSink) ProposeCandidate(
	ctx context.Context,
	candidate memcontract.Candidate,
) (memcontract.Decision, error) {
	if !s.failed && strings.Contains(candidate.Content, "Second replay candidate") {
		s.failed = true
		return memcontract.Decision{}, errors.New("controlled second candidate failure")
	}
	return s.delegate.ProposeCandidate(ctx, candidate)
}

func TestCandidateJSONShape(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve candidate metadata when encoded as inbox JSONL", func(t *testing.T) {
		t.Parallel()

		candidate := testCandidate("Pedro prefers brief updates.")
		candidate.Metadata = map[string]string{"source": "test"}
		encoded, err := json.Marshal(candidate)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		var decoded memcontract.Candidate
		if err := json.Unmarshal(encoded, &decoded); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if decoded.Metadata["source"] != "test" {
			t.Fatalf("decoded metadata = %#v, want source=test", decoded.Metadata)
		}
	})
}
