package memory

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	memcontract "github.com/compozy/compozy/internal/memory/contract"
	memoryrecall "github.com/compozy/compozy/internal/memory/recall"
	"github.com/compozy/compozy/internal/session"
)

func TestNewRecallAugmenter(t *testing.T) {
	t.Parallel()

	t.Run("Should return original message when session or query is empty", func(t *testing.T) {
		t.Parallel()

		augmenter := NewRecallAugmenter(newOpenTestStore(t, filepath.Join(t.TempDir(), "global")))

		got, err := augmenter(context.Background(), nil, "hello")
		if err != nil {
			t.Fatalf("Augment(nil session) error = %v", err)
		}
		if got != "hello" {
			t.Fatalf("Augment(nil session) = %q, want original message", got)
		}

		got, err = augmenter(context.Background(), &session.Session{Type: session.SessionTypeUser}, "   ")
		if err != nil {
			t.Fatalf("Augment(blank query) error = %v", err)
		}
		if got != "   " {
			t.Fatalf("Augment(blank query) = %q, want original message", got)
		}
	})

	t.Run("Should prepend recall and preserve user message", func(t *testing.T) {
		t.Parallel()

		baseDir := t.TempDir()
		workspaceRoot := filepath.Join(baseDir, "workspace")
		store := newOpenTestStore(t,
			filepath.Join(baseDir, "global"),
			WithCatalogDatabasePath(filepath.Join(baseDir, "compozy.db")),
		).ForWorkspace(workspaceRoot)
		if err := store.EnsureDirs(); err != nil {
			t.Fatalf("Store.EnsureDirs() error = %v", err)
		}
		if err := store.Write(t.Context(), memcontract.ScopeWorkspace, "auth.md", mustMemoryContent(t, testMemoryMeta{
			Name:        "Auth",
			Description: "Auth migration notes",
			Type:        memcontract.TypeProject,
		}, "Remember auth sessions and migration details.\n")); err != nil {
			t.Fatalf("Store.Write() error = %v", err)
		}

		augmenter := NewRecallAugmenter(store)
		got, err := augmenter(
			context.Background(),
			&session.Session{Type: session.SessionTypeUser, Workspace: workspaceRoot},
			"auth migration sessions",
		)
		if err != nil {
			t.Fatalf("Augment() error = %v", err)
		}
		if !strings.Contains(got, "Relevant durable memory for this turn:") {
			t.Fatalf("Augment() = %q, want recall header", got)
		}
		if !strings.Contains(got, "Auth") {
			t.Fatalf("Augment() = %q, want memory metadata", got)
		}
		if !strings.Contains(got, "Memory: Remember auth sessions and migration details.") {
			t.Fatalf("Augment() = %q, want packaged memory body", got)
		}
		if !strings.Contains(got, "<turn-recall>\nRelevant durable memory for this turn:") {
			t.Fatalf("Augment() = %q, want tagged recall block", got)
		}
		if !strings.Contains(got, "</turn-recall>\n\n<user-message>\nauth migration sessions\n</user-message>") {
			t.Fatalf("Augment() = %q, want preserved user message", got)
		}
		if strings.Contains(got, "User message:") {
			t.Fatalf("Augment() = %q, want no legacy user message marker", got)
		}
	})
}

func TestBuildPackagedRecallBlock(t *testing.T) {
	t.Parallel()

	t.Run("Should cap final rendered recall block to the declared budget", func(t *testing.T) {
		t.Parallel()

		packaged := memcontract.Packaged{
			Blocks: []memcontract.Block{{
				Scope: memcontract.ScopeWorkspace,
				Entries: []memcontract.PackagedEntry{{
					Title:   "Large project note",
					Type:    memcontract.TypeProject,
					Body:    strings.Repeat("x", maxRecallCharacters*2),
					ModTime: time.Date(2026, 5, 27, 12, 0, 0, 0, time.UTC),
				}},
			}},
		}

		got := buildPackagedRecallBlock(packaged)
		if got == "" {
			t.Fatal("buildPackagedRecallBlock() = empty, want capped recall block")
		}
		if runes := utf8.RuneCountInString(got); runes > maxRecallCharacters {
			t.Fatalf(
				"buildPackagedRecallBlock() runes = %d, want <= %d",
				runes,
				maxRecallCharacters,
			)
		}
		if !strings.Contains(got, recallPromptSafetyFooter) {
			t.Fatalf("buildPackagedRecallBlock() = %q, want safety footer", got)
		}
	})
}

func TestStoreRecall(t *testing.T) {
	t.Parallel()

	t.Run("Should recall from chunk FTS with shadow precedence and live signals", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		baseDir := t.TempDir()
		globalDir := filepath.Join(baseDir, "compozy-home", memoryDirName)
		workspaceRoot := filepath.Join(baseDir, "workspace")
		catalogPath := filepath.Join(baseDir, "compozy.db")
		store := newOpenTestStore(t, globalDir, WithCatalogDatabasePath(catalogPath)).ForWorkspace(workspaceRoot)
		if err := store.EnsureDirs(); err != nil {
			t.Fatalf("Store.EnsureDirs() error = %v", err)
		}
		if err := store.Write(
			t.Context(),
			memcontract.ScopeGlobal,
			"project_auth.md",
			mustMemoryContent(t, testMemoryMeta{
				Name:        "Global Auth",
				Description: "Global auth migration",
				Type:        memcontract.TypeProject,
			}, "Global auth migration sessions are less specific.\n"),
		); err != nil {
			t.Fatalf("Store.Write(global) error = %v", err)
		}
		if err := store.Write(
			t.Context(),
			memcontract.ScopeWorkspace,
			"project_auth.md",
			mustMemoryContent(t, testMemoryMeta{
				Name:        "Workspace Auth",
				Description: "Workspace auth migration",
				Type:        memcontract.TypeProject,
			}, "Workspace auth migration sessions are more specific.\n"),
		); err != nil {
			t.Fatalf("Store.Write(workspace) error = %v", err)
		}
		workspaceID := storeWorkspaceID(ctx, t, store)
		agentStore := store.ForAgent(workspaceID, "coder", memcontract.AgentTierWorkspace)
		if err := agentStore.EnsureDirs(); err != nil {
			t.Fatalf("agentStore.EnsureDirs() error = %v", err)
		}
		if err := agentStore.Write(
			t.Context(),
			memcontract.ScopeAgent,
			"project_auth.md",
			mustMemoryContent(
				t,
				testMemoryMeta{
					Name:        "Agent Auth",
					Description: "Agent auth migration",
					Type:        memcontract.TypeProject,
				},
				"Agent auth migration sessions are most specific.\n",
			),
		); err != nil {
			t.Fatalf("agentStore.Write(agent) error = %v", err)
		}

		packaged, err := agentStore.Recall(
			ctx,
			memcontract.Query{QueryText: "auth migration sessions"},
			memcontract.RecallOptions{TopK: 5},
		)
		if err != nil {
			t.Fatalf("Store.Recall() error = %v", err)
		}
		entries := packagedRecallEntries(packaged)
		if len(entries) != 1 {
			t.Fatalf("recall entries = %d, want 1 after shadowing", len(entries))
		}
		if !strings.Contains(entries[0].Body, "most specific") {
			t.Fatalf("recall entry body = %q, want agent-specific memory", entries[0].Body)
		}
		closeRecallRecorders(t, agentStore)
		assertRecallSignal(t, store.catalog.db, entries[0].ID, workspaceID)
		assertMemoryEventOp(ctx, t, store.catalog.db, memoryEventRecallExecuted)
		assertMemoryEventOp(ctx, t, store.catalog.db, memoryEventWriteShadowed)
	})

	t.Run("Should record trivial recall skips without candidate lookup failure", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		baseDir := t.TempDir()
		store := newOpenTestStore(t,
			filepath.Join(baseDir, "global"),
			WithCatalogDatabasePath(filepath.Join(baseDir, "compozy.db")),
		)
		if err := store.EnsureDirs(); err != nil {
			t.Fatalf("Store.EnsureDirs() error = %v", err)
		}
		packaged, err := store.Recall(
			ctx,
			memcontract.Query{QueryText: "auth"},
			memcontract.RecallOptions{TopK: 5},
		)
		if err != nil {
			t.Fatalf("Store.Recall(trivial) error = %v", err)
		}
		if len(packaged.Blocks) != 0 {
			t.Fatalf("Store.Recall(trivial) blocks = %d, want 0", len(packaged.Blocks))
		}
		assertMemoryEventOp(ctx, t, store.catalog.db, memoryEventRecallSkipped)
	})

	t.Run("Should recall CJK substrings through trigram FTS", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		baseDir := t.TempDir()
		workspaceRoot := filepath.Join(baseDir, "workspace")
		store := newOpenTestStore(t,
			filepath.Join(baseDir, "global"),
			WithCatalogDatabasePath(filepath.Join(baseDir, "compozy.db")),
		).ForWorkspace(workspaceRoot)
		if err := store.EnsureDirs(); err != nil {
			t.Fatalf("Store.EnsureDirs() error = %v", err)
		}
		if err := store.Write(
			t.Context(),
			memcontract.ScopeWorkspace,
			"project_i18n.md",
			mustMemoryContent(t, testMemoryMeta{
				Name:        "I18N Recall",
				Description: "Japanese recall fixture",
				Type:        memcontract.TypeProject,
			}, "認証移行計画セッションを保持する。\n"),
		); err != nil {
			t.Fatalf("Store.Write(cjk) error = %v", err)
		}

		packaged, err := store.Recall(
			ctx,
			memcontract.Query{QueryText: "移行計画セッション"},
			memcontract.RecallOptions{TopK: 1},
		)
		if err != nil {
			t.Fatalf("Store.Recall(cjk) error = %v", err)
		}
		entries := packagedRecallEntries(packaged)
		if len(entries) != 1 {
			t.Fatalf("CJK recall entries = %d, want 1", len(entries))
		}
		if !strings.Contains(entries[0].Body, "認証移行計画") {
			t.Fatalf("CJK recall body = %q, want Japanese fixture body", entries[0].Body)
		}
	})

	t.Run("Should not persist signals after recorder admission closes", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		baseDir := t.TempDir()
		store := newOpenTestStore(t,
			filepath.Join(baseDir, "global"),
			WithCatalogDatabasePath(filepath.Join(baseDir, "compozy.db")),
		)
		if err := store.EnsureDirs(); err != nil {
			t.Fatalf("Store.EnsureDirs() error = %v", err)
		}
		if err := store.Write(
			t.Context(),
			memcontract.ScopeGlobal,
			"project_auth.md",
			mustMemoryContent(t, testMemoryMeta{
				Name:        "Global Auth",
				Description: "Global auth migration",
				Type:        memcontract.TypeProject,
			}, "Global auth migration sessions.\n"),
		); err != nil {
			t.Fatalf("Store.Write(global) error = %v", err)
		}
		closeRecallRecorders(t, store)

		packaged, err := store.Recall(
			ctx,
			memcontract.Query{QueryText: "auth migration sessions"},
			memcontract.RecallOptions{TopK: 1},
		)
		if err != nil {
			t.Fatalf("Store.Recall() error = %v", err)
		}
		if entries := packagedRecallEntries(packaged); len(entries) != 1 {
			t.Fatalf("recall entries = %d, want 1", len(entries))
		}

		var signalCount int
		if err := store.catalog.db.QueryRowContext(
			ctx,
			"SELECT COUNT(*) FROM memory_recall_signals",
		).Scan(&signalCount); err != nil {
			t.Fatalf("count recall signals: %v", err)
		}
		if signalCount != 0 {
			t.Fatalf("recall signal count = %d, want 0 after recorder admission closes", signalCount)
		}
	})
}

func TestStoreRecallFailureAndUtilityPaths(t *testing.T) {
	t.Parallel()

	t.Run("Should return empty package when catalog is disabled", func(t *testing.T) {
		t.Parallel()

		store := newOpenTestStore(t, filepath.Join(t.TempDir(), "global"))
		packaged, err := store.Recall(
			context.Background(),
			memcontract.Query{QueryText: "auth migration sessions"},
			memcontract.RecallOptions{TopK: 2},
		)
		if err != nil {
			t.Fatalf("Store.Recall(no catalog) error = %v", err)
		}
		if len(packaged.Blocks) != 0 {
			t.Fatalf("Store.Recall(no catalog) blocks = %d, want 0", len(packaged.Blocks))
		}
	})

	t.Run("Should reject invalid recall receivers and contexts", func(t *testing.T) {
		t.Parallel()

		var nilStore *Store
		if _, err := nilStore.Recall(
			context.Background(),
			memcontract.Query{QueryText: "auth migration sessions"},
			memcontract.RecallOptions{},
		); err == nil {
			t.Fatal("nil Store.Recall() error = nil, want failure")
		}
		if _, err := newOpenTestStore(t, filepath.Join(t.TempDir(), "global")).Recall(
			nilMemoryTestContext(),
			memcontract.Query{QueryText: "auth migration sessions"},
			memcontract.RecallOptions{},
		); err == nil {
			t.Fatal("Store.Recall(nil context) error = nil, want failure")
		}
		if got := sAgentName(nil); got != "" {
			t.Fatalf("sAgentName(nil) = %q, want empty", got)
		}
	})

	t.Run("Should record signal failure events without leaking secret material", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		baseDir := t.TempDir()
		store := newOpenTestStore(t,
			filepath.Join(baseDir, "global"),
			WithCatalogDatabasePath(filepath.Join(baseDir, "compozy.db")),
		)
		if err := store.EnsureDirs(); err != nil {
			t.Fatalf("Store.EnsureDirs() error = %v", err)
		}
		if _, err := store.HealthStats(ctx, nil); err != nil {
			t.Fatalf("Store.HealthStats() error = %v", err)
		}
		if err := store.RecordRecallSignalFailed(
			ctx,
			memcontract.Query{WorkspaceID: "ws_test", QueryText: "auth migration sessions"},
			sql.ErrConnDone,
		); err != nil {
			t.Fatalf("RecordRecallSignalFailed() error = %v", err)
		}
		if err := store.RecordRecallSignalFailed(
			ctx,
			memcontract.Query{WorkspaceID: "ws_test", QueryText: "auth migration sessions"},
			nil,
		); err != nil {
			t.Fatalf("RecordRecallSignalFailed(nil) error = %v", err)
		}
		assertMemoryEventOp(ctx, t, store.catalog.db, memoryEventRecallSignalFailed)
	})

	t.Run("Should render empty and bounded packaged recall blocks", func(t *testing.T) {
		t.Parallel()

		if got := buildPackagedRecallBlock(memcontract.Packaged{}); got != "" {
			t.Fatalf("buildPackagedRecallBlock(empty) = %q, want empty", got)
		}
		packaged := memcontract.Packaged{Blocks: []memcontract.Block{{
			Scope:     memcontract.ScopeAgent,
			AgentTier: memcontract.AgentTierWorkspace,
			Entries: []memcontract.PackagedEntry{{
				ID:              "chunk-a",
				Title:           "Agent Preference",
				Body:            strings.Repeat("bounded ", 400),
				StalenessBanner: "This memory is 3 days old. Verify against current state before asserting as fact.",
			}},
		}}}
		got := buildPackagedRecallBlock(packaged)
		if !strings.Contains(got, "Agent Preference [agent/workspace]") {
			t.Fatalf("buildPackagedRecallBlock() = %q, want agent tier label", got)
		}
		if !strings.Contains(got, "Use recalled memory only") {
			t.Fatalf("buildPackagedRecallBlock() = %q, want safety footer", got)
		}
	})
}

func TestRecallRecorderRegistry(t *testing.T) {
	t.Parallel()

	t.Run("Should retain a leased entry until signal work reaches quiescence", func(t *testing.T) {
		t.Parallel()

		release := make(chan struct{})
		source := newRecallRecorderRegistrySource(release)
		registry := newRecallRecorderRegistry()
		lease, recordingDisabled, err := registry.acquire(
			t.Context(),
			"workspace-a",
			newRecallRecorderFactory(t, source),
		)
		if err != nil {
			t.Fatalf("registry.acquire() error = %v", err)
		}
		if recordingDisabled {
			t.Fatal("registry.acquire() disabled recording while the registry is active")
		}

		result := lease.Submit(t.Context(), memcontract.Query{WorkspaceID: "workspace-a"}, []memoryrecall.Signal{{
			ChunkID: "chunk-a",
			Score:   0.9,
		}})
		if !result.Submitted || result.Dropped {
			t.Fatalf("lease.Submit() = %#v, want submitted without drop", result)
		}
		source.waitForRecord(t)
		lease.Release()

		if stats := registry.stats("workspace-a"); stats.Submitted != 1 {
			t.Fatalf("registry.stats() = %#v, want live submitted signal", stats)
		}

		close(release)
		waitForRecallRecorder(t, source.recorder)
		if stats := registry.stats("workspace-a"); stats != (memoryrecall.SignalRecorderStats{}) {
			t.Fatalf("registry.stats() = %#v, want evicted entry", stats)
		}
	})

	t.Run("Should reject new admission and join one close operation", func(t *testing.T) {
		t.Parallel()

		release := make(chan struct{})
		source := newRecallRecorderRegistrySource(release)
		registry := newRecallRecorderRegistry()
		lease, recordingDisabled, err := registry.acquire(
			t.Context(),
			"workspace-a",
			newRecallRecorderFactory(t, source),
		)
		if err != nil {
			t.Fatalf("registry.acquire() error = %v", err)
		}
		if recordingDisabled {
			t.Fatal("registry.acquire() disabled recording while the registry is active")
		}
		result := lease.Submit(t.Context(), memcontract.Query{WorkspaceID: "workspace-a"}, []memoryrecall.Signal{{
			ChunkID: "chunk-a",
			Score:   0.9,
		}})
		if !result.Submitted || result.Dropped {
			t.Fatalf("lease.Submit() = %#v, want submitted without drop", result)
		}
		source.waitForRecord(t)

		canceledCtx, cancel := context.WithCancel(t.Context())
		cancel()
		if err := registry.close(canceledCtx); !errors.Is(err, context.Canceled) {
			t.Fatalf("registry.close(canceled) error = %v, want context canceled", err)
		}
		waitForRecallRecorder(t, source.recorder)
		joinedClose := make(chan error, 1)
		go func() {
			joinedClose <- registry.close(t.Context())
		}()

		lateLease, lateDisabled, err := registry.acquire(
			t.Context(),
			"workspace-b",
			newRecallRecorderFactory(t, source),
		)
		if err != nil {
			t.Fatalf("registry.acquire(after close) error = %v", err)
		}
		if lateLease != nil || !lateDisabled {
			t.Fatalf("registry.acquire(after close) = (%#v, %t), want (nil, true)", lateLease, lateDisabled)
		}

		close(release)
		if err := <-joinedClose; err != nil {
			t.Fatalf("registry.close(join) error = %v", err)
		}
	})

	t.Run("Should cancel active signal I/O before publishing terminal registry state", func(t *testing.T) {
		t.Parallel()

		release := make(chan struct{})
		var releaseOnce sync.Once
		t.Cleanup(func() {
			releaseOnce.Do(func() {
				close(release)
			})
		})
		source := newRecallRecorderRegistrySource(release)
		source.blockUntilCanceled = true
		registry := newRecallRecorderRegistry()
		lease, recordingDisabled, err := registry.acquire(
			t.Context(),
			"workspace-a",
			newRecallRecorderFactory(t, source),
		)
		if err != nil {
			t.Fatalf("registry.acquire() error = %v", err)
		}
		if recordingDisabled {
			t.Fatal("registry.acquire() disabled recording while the registry is active")
		}

		result := lease.Submit(t.Context(), memcontract.Query{WorkspaceID: "workspace-a"}, []memoryrecall.Signal{{
			ChunkID: "chunk-a",
			Score:   0.9,
		}})
		if !result.Submitted || result.Dropped {
			t.Fatalf("lease.Submit() = %#v, want submitted without drop", result)
		}
		source.waitForRecord(t)

		shutdownCtx, shutdownCancel := context.WithCancel(t.Context())
		defer shutdownCancel()
		closeResult := make(chan error, 1)
		go func() {
			closeResult <- registry.close(shutdownCtx)
		}()
		select {
		case err := <-closeResult:
			t.Fatalf("registry.close() returned before its leased worker stopped: %v", err)
		default:
		}

		lease.Release()
		shutdownCancel()
		source.waitForCancellation(t)
		waitForRecallRecorder(t, source.recorder)

		registry.mu.Lock()
		_, retained := registry.recorders["workspace-a"]
		closeOp := registry.closeOp
		registry.mu.Unlock()
		if retained {
			t.Fatal("registry retained entry after SignalRecorder.Done() closed")
		}
		if closeOp == nil {
			t.Fatal("registry close operation = nil, want shared terminal operation")
		}
		select {
		case <-closeOp.done:
		default:
			t.Fatal("registry close operation remained open after SignalRecorder.Done() closed")
		}

		if err := <-closeResult; err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("registry.close() error = %v, want nil or context canceled", err)
		}
		releaseOnce.Do(func() {
			close(release)
		})
	})
}

func TestBuildRecallBlock(t *testing.T) {
	t.Parallel()

	t.Run("Should skip zero-score entries and cap results", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
		block := buildRecallBlock([]memcontract.SearchResult{
			{
				Name:    "Ignore",
				Scope:   memcontract.ScopeWorkspace,
				Score:   0,
				Snippet: "should not appear",
			},
			{
				Name:    "One",
				Scope:   memcontract.ScopeWorkspace,
				Score:   0.9,
				Snippet: "first result",
				ModTime: now,
			},
			{
				Name:    "Two",
				Scope:   memcontract.ScopeGlobal,
				Score:   0.8,
				Snippet: "second result",
				ModTime: now.Add(-48 * time.Hour),
			},
			{
				Name:    "Three",
				Scope:   memcontract.ScopeGlobal,
				Score:   0.7,
				Snippet: "third result",
				ModTime: now,
			},
			{
				Name:    "Four",
				Scope:   memcontract.ScopeGlobal,
				Score:   0.6,
				Snippet: "fourth result",
				ModTime: now,
			},
		}, now)

		if strings.Contains(block, "Ignore") {
			t.Fatalf("buildRecallBlock() = %q, want zero-score result omitted", block)
		}
		if !strings.Contains(block, "One") || !strings.Contains(block, "Two") || !strings.Contains(block, "Three") {
			t.Fatalf("buildRecallBlock() = %q, want first three positive-scored results", block)
		}
		if strings.Contains(block, "Four") {
			t.Fatalf("buildRecallBlock() = %q, want max result count enforced", block)
		}
		if !strings.Contains(block, "Freshness:") {
			t.Fatalf("buildRecallBlock() = %q, want freshness warning for stale memory", block)
		}
	})
}

func storeWorkspaceID(ctx context.Context, t *testing.T, store *Store) string {
	t.Helper()

	workspaceID, err := store.workspaceIDForRoot(ctx, store.workspaceRoot)
	if err != nil {
		t.Fatalf("store.workspaceIDForRoot() error = %v", err)
	}
	return workspaceID
}

func packagedRecallEntries(packaged memcontract.Packaged) []memcontract.PackagedEntry {
	entries := make([]memcontract.PackagedEntry, 0)
	for _, block := range packaged.Blocks {
		entries = append(entries, block.Entries...)
	}
	return entries
}

func closeRecallRecorders(t *testing.T, store *Store) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := store.CloseRecallSignalRecorders(ctx); err != nil {
		t.Fatalf("Store.CloseRecallSignalRecorders() error = %v", err)
	}
}

type recallRecorderRegistrySource struct {
	release            <-chan struct{}
	blockUntilCanceled bool
	started            chan struct{}
	canceled           chan error
	startedOnce        sync.Once
	recorder           *memoryrecall.SignalRecorder
}

func newRecallRecorderRegistrySource(release <-chan struct{}) *recallRecorderRegistrySource {
	return &recallRecorderRegistrySource{
		release:  release,
		started:  make(chan struct{}),
		canceled: make(chan error, 1),
	}
}

func newRecallRecorderFactory(
	t *testing.T,
	source *recallRecorderRegistrySource,
) recallRecorderFactory {
	t.Helper()

	return func(shouldStop func() bool, onStopped func()) (*memoryrecall.SignalRecorder, error) {
		recorder, err := memoryrecall.NewSignalRecorder(
			context.Background(),
			source,
			memoryrecall.SignalRecorderConfig{QueueCapacity: 1},
			slog.New(slog.DiscardHandler),
			memoryrecall.WithIdleStop(shouldStop),
			memoryrecall.WithStopped(onStopped),
		)
		if err != nil {
			return nil, err
		}
		source.recorder = recorder
		return recorder, nil
	}
}

func (s *recallRecorderRegistrySource) RecordRecall(ctx context.Context, _ []memoryrecall.Signal) error {
	s.startedOnce.Do(func() {
		close(s.started)
	})
	if s.blockUntilCanceled {
		select {
		case <-ctx.Done():
			s.canceled <- ctx.Err()
			return ctx.Err()
		case <-s.release:
			return nil
		}
	}
	if s.release != nil {
		select {
		case <-s.release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (s *recallRecorderRegistrySource) RecordRecallSignalFailed(
	context.Context,
	memcontract.Query,
	error,
) error {
	return nil
}

func (s *recallRecorderRegistrySource) RecordRecallSignalDropped(
	context.Context,
	memcontract.Query,
	[]memoryrecall.Signal,
	int,
) error {
	return nil
}

func (s *recallRecorderRegistrySource) waitForRecord(t *testing.T) {
	t.Helper()

	ctx, cancel := newRecallRecorderWaitContext(t)
	defer cancel()
	select {
	case <-s.started:
	case <-ctx.Done():
		t.Fatalf("wait for RecallRecorderRegistrySource.RecordRecall: %v", ctx.Err())
	}
}

func (s *recallRecorderRegistrySource) waitForCancellation(t *testing.T) {
	t.Helper()

	ctx, cancel := newRecallRecorderWaitContext(t)
	defer cancel()
	select {
	case err := <-s.canceled:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("RecordRecall() cancellation error = %v, want context canceled", err)
		}
	case <-ctx.Done():
		t.Fatalf("wait for RecallRecorderRegistrySource.RecordRecall cancellation: %v", ctx.Err())
	}
}

func waitForRecallRecorder(t *testing.T, recorder *memoryrecall.SignalRecorder) {
	t.Helper()
	if recorder == nil {
		t.Fatal("wait for recall recorder: recorder is nil")
	}
	ctx, cancel := newRecallRecorderWaitContext(t)
	defer cancel()
	select {
	case <-recorder.Done():
	case <-ctx.Done():
		t.Fatalf("wait for recall recorder: %v", ctx.Err())
	}
}

func newRecallRecorderWaitContext(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()

	return context.WithTimeout(t.Context(), 2*time.Second)
}

func assertRecallSignal(t *testing.T, db *sql.DB, chunkID string, workspaceID string) {
	t.Helper()

	var (
		recallCount        int
		recallWorkspaceID  sql.NullString
		recallScore        float64
		freshnessStartedAt int64
	)
	if err := db.QueryRowContext(
		context.Background(),
		`SELECT recall_count, workspace_id, recall_score, freshness_started_at
		 FROM memory_recall_signals WHERE chunk_id = ?`,
		chunkID,
	).Scan(&recallCount, &recallWorkspaceID, &recallScore, &freshnessStartedAt); err != nil {
		t.Fatalf("Query recall signal error = %v", err)
	}
	if recallCount != 1 {
		t.Fatalf("recall_count = %d, want 1", recallCount)
	}
	if recallWorkspaceID.String != workspaceID {
		t.Fatalf("signal workspace_id = %q, want %q", recallWorkspaceID.String, workspaceID)
	}
	if recallScore <= 0 {
		t.Fatalf("recall_score = %f, want positive", recallScore)
	}
	if freshnessStartedAt <= 0 {
		t.Fatalf("freshness_started_at = %d, want positive", freshnessStartedAt)
	}
}

func assertMemoryEventOp(ctx context.Context, t *testing.T, db *sql.DB, op string) {
	t.Helper()

	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM memory_events WHERE op = ?`, op).Scan(&count); err != nil {
		t.Fatalf("Query memory_events count error = %v", err)
	}
	if count == 0 {
		t.Fatalf("memory_events op %q count = 0, want > 0", op)
	}
}
