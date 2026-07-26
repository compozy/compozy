package session

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/compozy/agh/internal/acp"
	sessionledger "github.com/compozy/agh/internal/sessions/ledger"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/sessiondb"
	"github.com/compozy/agh/internal/testutil"
	"github.com/compozy/agh/internal/transcript"
)

func TestClearConversationRestartsSameSessionWithFreshContext(t *testing.T) {
	t.Parallel()

	t.Run("Should restart the same session with fresh provider and transcript state", func(t *testing.T) {
		h := newHarness(t)
		session := createSession(t, h)

		firstEvents, err := h.manager.Prompt(testutil.Context(t), session.ID, "before clear")
		if err != nil {
			t.Fatalf("Prompt(before clear) error = %v", err)
		}
		collectEvents(t, firstEvents)

		originalACP := session.Info().ACPSessionID

		cleared, err := h.manager.ClearConversation(testutil.Context(t), session.ID)
		if err != nil {
			t.Fatalf("ClearConversation() error = %v", err)
		}
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), cleared.ID); err != nil {
				t.Fatalf("cleanup Stop() error = %v", err)
			}
		})

		if got, want := cleared.ID, session.ID; got != want {
			t.Fatalf("cleared.ID = %q, want %q", got, want)
		}
		if got := cleared.Info().State; got != StateActive {
			t.Fatalf("cleared state = %q, want %q", got, StateActive)
		}
		if got := cleared.Info().ACPSessionID; got == "" || got == originalACP {
			t.Fatalf("cleared ACP session id = %q, want fresh non-empty id distinct from %q", got, originalACP)
		}
		if got := len(h.driver.startCalls); got != 2 {
			t.Fatalf("len(startCalls) = %d, want 2", got)
		}
		if got := h.driver.startCalls[1].ResumeSessionID; got != "" {
			t.Fatalf("clear restart ResumeSessionID = %q, want empty for fresh provider context", got)
		}

		page, err := h.manager.TranscriptPage(testutil.Context(t), cleared.ID, transcript.PageQuery{})
		if err != nil {
			t.Fatalf("TranscriptPage(after clear) error = %v", err)
		}
		if got := len(page.Entries); got != 0 {
			t.Fatalf("TranscriptPage(after clear) len = %d, want 0", got)
		}

		stored := readStoredEvents(t, cleared)
		if got := len(stored); got != 0 {
			t.Fatalf("stored events after clear = %d, want 0", got)
		}

		secondEvents, err := h.manager.Prompt(testutil.Context(t), cleared.ID, "after clear")
		if err != nil {
			t.Fatalf("Prompt(after clear) error = %v", err)
		}
		collectEvents(t, secondEvents)

		stored = readStoredEvents(t, cleared)
		if got := len(stored); got == 0 {
			t.Fatal("stored events after second prompt = 0, want persisted prompt data")
		}
		for _, event := range stored {
			if strings.Contains(event.Content, "before clear") {
				t.Fatalf("stored event content still contains cleared prompt: %s", event.Content)
			}
		}
	})
}

func TestClearConversationDiscardsMaterializedLedger(t *testing.T) {
	t.Parallel()

	t.Run("Should remove stale materialized ledger before replacing the event store", func(t *testing.T) {
		h := newHarness(t)
		materializer, err := sessionledger.NewMaterializer(sessionledger.Config{
			RootDir: h.homePaths.SessionsDir,
		})
		if err != nil {
			t.Fatalf("NewMaterializer() error = %v", err)
		}
		h.manager = newManagerWithHarness(t, h, WithLedgerMaterializer(materializer))

		session := createSession(t, h)
		firstEvents, err := h.manager.Prompt(testutil.Context(t), session.ID, "before clear")
		if err != nil {
			t.Fatalf("Prompt(before clear) error = %v", err)
		}
		collectEvents(t, firstEvents)
		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("Stop(before clear) error = %v", err)
		}

		ledgerPath := filepath.Join(h.homePaths.SessionsDir, h.workspaceID, session.ID, "ledger.jsonl")
		ledgerBefore, err := os.ReadFile(ledgerPath)
		if err != nil {
			t.Fatalf("ReadFile(ledger before clear) error = %v", err)
		}
		if !strings.Contains(string(ledgerBefore), "before clear") {
			t.Fatalf("ledger before clear = %s, want original prompt content", ledgerBefore)
		}

		resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
		if err != nil {
			t.Fatalf("Resume() error = %v", err)
		}
		cleared, err := h.manager.ClearConversation(testutil.Context(t), resumed.ID)
		if err != nil {
			t.Fatalf("ClearConversation() error = %v", err)
		}

		if _, statErr := os.Stat(ledgerPath); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("Stat(discarded ledger) error = %v, want os.ErrNotExist", statErr)
		}
		events, err := h.manager.Events(testutil.Context(t), cleared.ID, store.EventQuery{})
		if err != nil {
			t.Fatalf("Events(after clear) error = %v", err)
		}
		if got := len(events); got != 0 {
			t.Fatalf("Events(after clear) len = %d, want 0", got)
		}
		page, err := h.manager.TranscriptPage(testutil.Context(t), cleared.ID, transcript.PageQuery{})
		if err != nil {
			t.Fatalf("TranscriptPage(after clear) error = %v", err)
		}
		if got := len(page.Entries); got != 0 {
			t.Fatalf("TranscriptPage(after clear) len = %d, want 0", got)
		}

		if err := h.manager.Stop(testutil.Context(t), cleared.ID); err != nil {
			t.Fatalf("Stop(after clear) error = %v", err)
		}
		ledgerAfter, err := os.ReadFile(ledgerPath)
		if err != nil {
			t.Fatalf("ReadFile(ledger after clear stop) error = %v", err)
		}
		if strings.Contains(string(ledgerAfter), "before clear") {
			t.Fatalf("ledger after clear stop still contains cleared prompt: %s", ledgerAfter)
		}
	})
}

func TestClearConversationResetsStoreOpenedWithStaleRows(t *testing.T) {
	t.Parallel()

	t.Run("Should clear stale rows present when the replacement store opens", func(t *testing.T) {
		h := newHarness(t)
		var openCount atomic.Int32
		h.manager = newManagerWithHarness(
			t,
			h,
			WithStore(func(ctx context.Context, sessionID string, path string) (EventRecorder, error) {
				recorder, err := sessiondb.OpenSessionDB(ctx, sessionID, path)
				if err != nil {
					return nil, err
				}
				if openCount.Add(1) == 2 {
					if err := recorder.Record(ctx, store.SessionEvent{
						TurnID:    "turn-stale-reset",
						Type:      acp.EventTypeUserMessage,
						AgentName: "coder",
						Content:   "{\"schema\":\"agh.session.event.v1\",\"type\":\"user_message\",\"text\":\"stale reset row\"}",
					}); err != nil {
						closeCtx := testutil.Context(t)
						return nil, errors.Join(err, recorder.Close(closeCtx))
					}
				}
				return recorder, nil
			}),
			WithQueryStore(func(ctx context.Context, sessionID string, path string) (EventReadCloser, error) {
				return sessiondb.OpenSessionDBReadOnly(ctx, sessionID, path)
			}),
		)

		session := createSession(t, h)
		firstEvents, err := h.manager.Prompt(testutil.Context(t), session.ID, "before clear")
		if err != nil {
			t.Fatalf("Prompt(before clear) error = %v", err)
		}
		collectEvents(t, firstEvents)

		cleared, err := h.manager.ClearConversation(testutil.Context(t), session.ID)
		if err != nil {
			t.Fatalf("ClearConversation() error = %v", err)
		}
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), cleared.ID); err != nil {
				t.Fatalf("cleanup Stop() error = %v", err)
			}
		})

		events, err := h.manager.Events(testutil.Context(t), cleared.ID, store.EventQuery{})
		if err != nil {
			t.Fatalf("Events(after clear) error = %v", err)
		}
		if got := len(events); got != 0 {
			t.Fatalf("Events(after clear) len = %d, want 0: %#v", got, events)
		}
	})
}

func TestClearConversationFailureRecovery(t *testing.T) {
	t.Parallel()

	t.Run("Should stop the replacement session and restore the old event store when epoch commit fails", func(
		t *testing.T,
	) {
		h := newHarness(t)
		epochStore := newFakeTranscriptEpochStore()
		epochStore.ensureErr = errors.New("epoch store unavailable")
		h.manager = newManagerWithHarness(t, h, WithTranscriptEpochStore(epochStore))
		session := createSession(t, h)
		eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "before clear")
		if err != nil {
			t.Fatalf("Prompt(before clear) error = %v", err)
		}
		collectEvents(t, eventsCh)

		_, err = h.manager.ClearConversation(testutil.Context(t), session.ID)
		if err == nil {
			t.Fatal("ClearConversation() error = nil, want epoch commit failure")
		}
		if !strings.Contains(err.Error(), "ensure transcript epoch") {
			t.Fatalf("ClearConversation() error = %v, want transcript epoch failure", err)
		}
		if got := h.driver.stopCalls; got < 2 {
			t.Fatalf("driver stop calls = %d, want original stop plus replacement rollback", got)
		}
		if _, ok := h.manager.Get(session.ID); ok {
			t.Fatal("manager.Get() ok = true, want replacement session removed after rollback")
		}
		if _, statErr := os.Stat(sessionDBClearCommitPath(session.DBPath())); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("Stat(clear commit marker) error = %v, want os.ErrNotExist", statErr)
		}

		stored := readStoredEvents(t, session)
		if got := len(stored); got == 0 {
			t.Fatal("stored events after failed clear = 0, want restored pre-clear transcript")
		}
		foundOriginalPrompt := false
		for _, event := range stored {
			if strings.Contains(event.Content, "before clear") {
				foundOriginalPrompt = true
				break
			}
		}
		if !foundOriginalPrompt {
			t.Fatalf("stored events after failed clear = %#v, want original prompt content", stored)
		}
	})

	t.Run("Should keep committed clear marker when epoch reconciliation fails before backup", func(t *testing.T) {
		h := newHarness(t)
		epochStore := newFakeTranscriptEpochStore()
		epochStore.ensureErr = errors.New("epoch store unavailable")
		h.manager = newManagerWithHarness(t, h, WithTranscriptEpochStore(epochStore))
		session := createSession(t, h)
		eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "before committed clear")
		if err != nil {
			t.Fatalf("Prompt(before committed clear) error = %v", err)
		}
		collectEvents(t, eventsCh)
		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}

		dbPath := session.DBPath()
		backups, err := backupSessionDBAfterRecovery(dbPath)
		if err != nil {
			t.Fatalf("backupSessionDBAfterRecovery() error = %v", err)
		}
		if got := len(backups); got == 0 {
			t.Fatal("backupSessionDBAfterRecovery() backups = 0, want at least session database backup")
		}
		recorder, err := sessiondb.OpenSessionDB(testutil.Context(t), session.ID, dbPath)
		if err != nil {
			t.Fatalf("OpenSessionDB(fresh committed DB) error = %v", err)
		}
		if err := recorder.Close(testutil.Context(t)); err != nil {
			t.Fatalf("Close(fresh committed DB) error = %v", err)
		}
		if err := commitSessionDBClear(dbPath, 7); err != nil {
			t.Fatalf("commitSessionDBClear() error = %v", err)
		}

		_, err = h.manager.backupSessionDBForClear(testutil.Context(t), dbPath, session.ID)
		if err == nil {
			t.Fatal("backupSessionDBForClear() error = nil, want epoch reconciliation failure")
		}
		if !strings.Contains(err.Error(), "reconcile transcript epoch") {
			t.Fatalf("backupSessionDBForClear() error = %v, want epoch reconciliation failure", err)
		}
		if _, statErr := os.Stat(sessionDBClearCommitPath(dbPath)); statErr != nil {
			t.Fatalf("Stat(clear commit marker) error = %v, want marker preserved", statErr)
		}
		if _, statErr := os.Stat(dbPath + ".clear-backup"); statErr != nil {
			t.Fatalf("Stat(clear backup) error = %v, want backup preserved", statErr)
		}
		if got := epochStore.ensureCallCount(); got != 1 {
			t.Fatalf("ensure call count = %d, want 1", got)
		}
		if got := epochStore.ensureMinimum(session.ID); got != 7 {
			t.Fatalf("ensure minimum = %d, want 7", got)
		}
	})
}

func TestClearConversationRejectsPromptInProgress(t *testing.T) {
	t.Parallel()

	t.Run("Should reject clearing while a prompt is in progress", func(t *testing.T) {
		h := newHarness(t)
		session := createSession(t, h)
		releasePrompt := make(chan struct{})
		h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			events := make(chan acp.AgentEvent)
			go func() {
				defer close(events)
				<-releasePrompt
				events <- acp.AgentEvent{
					Type:      acp.EventTypeDone,
					SessionID: session.Info().ACPSessionID,
					TurnID:    req.TurnID,
				}
			}()
			return events, nil
		}

		eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "hello")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		waitForCondition(t, "prompt setup", func() bool {
			return session.IsPrompting()
		})

		_, err = h.manager.ClearConversation(testutil.Context(t), session.ID)
		if !errors.Is(err, ErrPromptInProgress) {
			t.Fatalf("ClearConversation() error = %v, want %v", err, ErrPromptInProgress)
		}

		close(releasePrompt)
		collectEvents(t, eventsCh)
		if stopErr := h.manager.Stop(testutil.Context(t), session.ID); stopErr != nil {
			t.Fatalf("cleanup Stop() error = %v", stopErr)
		}
	})
}

type fakeTranscriptEpochStore struct {
	mu        sync.Mutex
	epochs    map[string]int64
	minimums  map[string]int64
	ensureErr error
}

func newFakeTranscriptEpochStore() *fakeTranscriptEpochStore {
	return &fakeTranscriptEpochStore{
		epochs:   make(map[string]int64),
		minimums: make(map[string]int64),
	}
}

func (s *fakeTranscriptEpochStore) SessionTranscriptEpoch(
	_ context.Context,
	sessionID string,
) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.epochs[strings.TrimSpace(sessionID)], nil
}

func (s *fakeTranscriptEpochStore) EnsureSessionTranscriptEpoch(
	_ context.Context,
	update store.SessionTranscriptEpochUpdate,
) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	target := strings.TrimSpace(update.SessionID)
	s.minimums[target] = update.Minimum
	if s.ensureErr != nil {
		return 0, s.ensureErr
	}
	if s.epochs[target] < update.Minimum {
		s.epochs[target] = update.Minimum
	}
	return s.epochs[target], nil
}

func (s *fakeTranscriptEpochStore) ensureCallCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.minimums)
}

func (s *fakeTranscriptEpochStore) ensureMinimum(sessionID string) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.minimums[strings.TrimSpace(sessionID)]
}

func TestBackupSessionDB(t *testing.T) {
	t.Parallel()

	t.Run("Should roll back partial rename failures", func(t *testing.T) {
		dir := t.TempDir()
		dbPath := filepath.Join(dir, "session.db")
		walPath := dbPath + "-wal"

		if err := os.WriteFile(dbPath, []byte("db"), 0o600); err != nil {
			t.Fatalf("WriteFile(session.db) error = %v", err)
		}
		if err := os.WriteFile(walPath, []byte("wal"), 0o600); err != nil {
			t.Fatalf("WriteFile(session.db-wal) error = %v", err)
		}

		blockedBackupDir := walPath + ".clear-backup"
		if err := os.Mkdir(blockedBackupDir, 0o755); err != nil {
			t.Fatalf("Mkdir(blocked backup) error = %v", err)
		}
		if err := os.WriteFile(filepath.Join(blockedBackupDir, "sentinel"), []byte("x"), 0o600); err != nil {
			t.Fatalf("WriteFile(blocked backup sentinel) error = %v", err)
		}

		_, err := backupSessionDB(dbPath)
		if err == nil {
			t.Fatal("backupSessionDB() error = nil, want rollback failure path")
		}
		if !strings.Contains(err.Error(), "remove stale clear backup") {
			t.Fatalf("backupSessionDB() error = %v, want stale backup failure", err)
		}

		if got, readErr := os.ReadFile(dbPath); readErr != nil || string(got) != "db" {
			t.Fatalf("ReadFile(restored session.db) = %q, %v, want db", got, readErr)
		}
		if got, readErr := os.ReadFile(walPath); readErr != nil || string(got) != "wal" {
			t.Fatalf("ReadFile(original session.db-wal) = %q, %v, want wal", got, readErr)
		}
		if _, statErr := os.Stat(dbPath + ".clear-backup"); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("Stat(session.db.clear-backup) error = %v, want os.ErrNotExist", statErr)
		}
		if _, statErr := os.Stat(blockedBackupDir); statErr != nil {
			t.Fatalf("Stat(blocked backup dir) error = %v", statErr)
		}
	})

	t.Run("Should restore interrupted clear backups before stored event queries", func(t *testing.T) {
		h := newHarness(t)
		session := createSession(t, h)

		eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "before interrupted clear")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		collectEvents(t, eventsCh)
		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}

		dbPath := session.DBPath()
		backups, err := backupSessionDB(dbPath)
		if err != nil {
			t.Fatalf("backupSessionDB() error = %v", err)
		}
		if got := len(backups); got == 0 {
			t.Fatal("backupSessionDB() backups = 0, want at least session database backup")
		}
		if _, statErr := os.Stat(dbPath); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("Stat(session database after backup) error = %v, want os.ErrNotExist", statErr)
		}
		if _, statErr := os.Stat(dbPath + ".clear-backup"); statErr != nil {
			t.Fatalf("Stat(session database backup) error = %v", statErr)
		}

		freshManager := newManagerWithHarness(t, h)
		events, err := freshManager.Events(testutil.Context(t), session.ID, store.EventQuery{})
		if err != nil {
			t.Fatalf("Events(after interrupted clear backup) error = %v", err)
		}
		if got := len(events); got == 0 {
			t.Fatal("Events(after interrupted clear backup) = 0, want restored transcript events")
		}
		if _, statErr := os.Stat(dbPath); statErr != nil {
			t.Fatalf("Stat(restored session database) error = %v", statErr)
		}
		if _, statErr := os.Stat(dbPath + ".clear-backup"); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("Stat(discarded interrupted backup) error = %v, want os.ErrNotExist", statErr)
		}
	})

	t.Run("Should discard committed clear backups without restoring old events", func(t *testing.T) {
		dir := t.TempDir()
		dbPath := filepath.Join(dir, "session.db")
		backupPath := dbPath + ".clear-backup"

		if err := os.WriteFile(dbPath, []byte("fresh"), 0o600); err != nil {
			t.Fatalf("WriteFile(fresh session.db) error = %v", err)
		}
		if err := os.WriteFile(backupPath, []byte("old"), 0o600); err != nil {
			t.Fatalf("WriteFile(session.db clear backup) error = %v", err)
		}
		if err := commitSessionDBClear(dbPath, 1); err != nil {
			t.Fatalf("commitSessionDBClear() error = %v", err)
		}

		if err := recoverSessionDBClear(dbPath); err != nil {
			t.Fatalf("recoverSessionDBClear(committed) error = %v", err)
		}
		if got, readErr := os.ReadFile(dbPath); readErr != nil || string(got) != "fresh" {
			t.Fatalf("ReadFile(session.db) = %q, %v, want fresh", got, readErr)
		}
		if _, statErr := os.Stat(backupPath); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("Stat(committed backup) error = %v, want os.ErrNotExist", statErr)
		}
		if _, statErr := os.Stat(sessionDBClearCommitPath(dbPath)); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("Stat(clear commit marker) error = %v, want os.ErrNotExist", statErr)
		}
	})

	t.Run("Should keep committed clear marker when committed backup discard fails", func(t *testing.T) {
		dir := t.TempDir()
		dbPath := filepath.Join(dir, "session.db")
		backupPath := dbPath + ".clear-backup"

		if err := os.WriteFile(dbPath, []byte("fresh"), 0o600); err != nil {
			t.Fatalf("WriteFile(fresh session.db) error = %v", err)
		}
		if err := os.Mkdir(backupPath, 0o755); err != nil {
			t.Fatalf("Mkdir(session.db clear backup blocker) error = %v", err)
		}
		if err := commitSessionDBClear(dbPath, 1); err != nil {
			t.Fatalf("commitSessionDBClear() error = %v", err)
		}

		err := (&Manager{}).finalizeCommittedSessionDBClear(
			dbPath,
			[]sessionDBBackup{{original: dbPath, backup: backupPath}},
		)
		if err == nil {
			t.Fatal("finalizeCommittedSessionDBClear() error = nil, want backup discard failure")
		}
		if !strings.Contains(err.Error(), "remove clear backup") {
			t.Fatalf("finalizeCommittedSessionDBClear() error = %v, want backup discard failure", err)
		}
		if _, statErr := os.Stat(sessionDBClearCommitPath(dbPath)); statErr != nil {
			t.Fatalf("Stat(clear commit marker) error = %v, want marker preserved", statErr)
		}
	})
}
