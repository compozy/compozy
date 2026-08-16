package session

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	sessionledger "github.com/compozy/compozy/internal/sessions/ledger"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/transcript"
)

func TestClearConversationRestartsSameSessionWithFreshContext(t *testing.T) {
	t.Parallel()

	t.Run("Should restart the same session with fresh provider and transcript state", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		attachmentPath := writeSessionAttachmentFixture(t, h, session.ID, "preserve-on-clear")

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
		payload, err := os.ReadFile(attachmentPath)
		if err != nil {
			t.Fatalf("ReadFile(attachment after clear) error = %v", err)
		}
		if got, want := string(payload), "preserve-on-clear"; got != want {
			t.Fatalf("attachment after clear = %q, want %q", got, want)
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

	t.Run("Should wait for active stored readers before replacing the event store", func(t *testing.T) {
		h := newHarness(t, withDefaultQueryStoreRuntime())
		shutdownQueryStoreRuntimeForTest(t, h.manager)
		session := createSession(t, h)
		ctx := testutil.Context(t)
		owner := testSessionDBOwner(session.ID, session.WorkspaceID)

		reader, err := h.manager.queryStoreRuntime.Open(ctx, owner, session.DBPath())
		if err != nil {
			t.Fatalf("Open(stored reader) error = %v", err)
		}
		closeReader := sync.OnceValue(func() error { return reader.Close(testutil.Context(t)) })
		t.Cleanup(func() {
			if closeErr := closeReader(); closeErr != nil {
				t.Errorf("Close(stored reader cleanup) error = %v", closeErr)
			}
		})

		type clearResult struct {
			session *Session
			err     error
		}
		clearDone := make(chan clearResult, 1)
		go func() {
			cleared, clearErr := h.manager.ClearConversation(ctx, session.ID)
			clearDone <- clearResult{session: cleared, err: clearErr}
		}()

		for {
			probe, openErr := h.manager.queryStoreRuntime.Open(ctx, owner, session.DBPath())
			if errors.Is(openErr, sessiondb.ErrReadOnlyPoolQuiescing) {
				break
			}
			if openErr != nil {
				t.Fatalf("Open(quiescence probe) error = %v", openErr)
			}
			if closeErr := probe.Close(ctx); closeErr != nil {
				t.Fatalf("Close(quiescence probe) error = %v", closeErr)
			}
			select {
			case result := <-clearDone:
				t.Fatalf("ClearConversation() returned before stored reader closed: %v", result.err)
			default:
				runtime.Gosched()
			}
		}

		select {
		case result := <-clearDone:
			t.Fatalf("ClearConversation() returned while stored reader was active: %v", result.err)
		default:
		}
		if _, err := os.Stat(session.DBPath()); err != nil {
			t.Fatalf("Stat(event store while clear quiesced) error = %v", err)
		}
		if err := closeReader(); err != nil {
			t.Fatalf("Close(stored reader) error = %v", err)
		}

		select {
		case result := <-clearDone:
			if result.err != nil {
				t.Fatalf("ClearConversation() error = %v", result.err)
			}
			if result.session == nil {
				t.Fatal("ClearConversation() session = nil")
			}
			t.Cleanup(func() {
				if stopErr := h.manager.Stop(testutil.Context(t), result.session.ID); stopErr != nil {
					t.Errorf("Stop(cleared session cleanup) error = %v", stopErr)
				}
			})
		case <-ctx.Done():
			t.Fatalf("ClearConversation() did not finish after stored reader closed: %v", ctx.Err())
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

	t.Run("Should preserve committed recovery state when ledger discard fails", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		materializer, err := sessionledger.NewMaterializer(sessionledger.Config{
			RootDir: h.homePaths.SessionsDir,
		})
		if err != nil {
			t.Fatalf("NewMaterializer() error = %v", err)
		}
		discardErr := errors.New("ledger storage unavailable")
		var failNextDiscard atomic.Bool
		ledgerMaterializer := &testLedgerMaterializer{
			delegate: materializer,
			discard: func(ctx context.Context, record store.SessionLedgerRecord) error {
				if failNextDiscard.CompareAndSwap(true, false) {
					return discardErr
				}
				return materializer.DiscardSessionLedger(ctx, record)
			},
		}
		h.manager = newManagerWithHarness(t, h, WithLedgerMaterializer(ledgerMaterializer))

		session := createSession(t, h)
		eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "before failed discard")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		collectEvents(t, eventsCh)
		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		ledgerPath := filepath.Join(h.homePaths.SessionsDir, h.workspaceID, session.ID, "ledger.jsonl")
		if _, err := os.ReadFile(ledgerPath); err != nil {
			t.Fatalf("ReadFile(ledger before clear) error = %v", err)
		}
		if _, err := h.manager.Resume(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("Resume() error = %v", err)
		}
		failNextDiscard.Store(true)

		cleared, err := h.manager.ClearConversation(testutil.Context(t), session.ID)
		if cleared == nil {
			t.Fatal("ClearConversation() session = nil, want committed replacement")
		}
		t.Cleanup(func() {
			if stopErr := h.manager.Stop(testutil.Context(t), cleared.ID); stopErr != nil {
				t.Errorf("Stop(replacement cleanup) error = %v", stopErr)
			}
		})
		if !errors.Is(err, discardErr) {
			t.Fatalf("ClearConversation() error = %v, want %v", err, discardErr)
		}
		if _, statErr := os.Stat(ledgerPath); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("Stat(ledger after failed discard) error = %v, want os.ErrNotExist", statErr)
		}

		dbPath := session.DBPath()
		manifest, present, err := readSessionDBClearManifest(dbPath)
		if err != nil {
			t.Fatalf("readSessionDBClearManifest() error = %v", err)
		}
		if !present {
			t.Fatal("readSessionDBClearManifest() present = false, want recovery manifest")
		}
		for _, path := range []string{
			dbPath + ".clear-backup",
			sessionDBClearManifestPath(dbPath),
			sessionDBClearCommitPath(dbPath),
		} {
			if _, statErr := os.Stat(path); statErr != nil {
				t.Fatalf("Stat(%q) error = %v, want committed recovery artifact", path, statErr)
			}
		}

		owner := testSessionDBOwner(session.ID, session.WorkspaceID)
		if err := h.manager.finalizeCommittedSessionDBClear(
			testutil.Context(t),
			owner,
			dbPath,
			manifest,
		); err != nil {
			t.Fatalf("finalizeCommittedSessionDBClear() error = %v", err)
		}
		for _, path := range []string{
			ledgerPath,
			dbPath + ".clear-backup",
			sessionDBClearManifestPath(dbPath),
			sessionDBClearCommitPath(dbPath),
		} {
			if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("Stat(%q) error = %v, want os.ErrNotExist", path, statErr)
			}
		}
	})

	t.Run("Should serialize a replacement stop after committed ledger discard", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		materializer, err := sessionledger.NewMaterializer(sessionledger.Config{
			RootDir: h.homePaths.SessionsDir,
		})
		if err != nil {
			t.Fatalf("NewMaterializer() error = %v", err)
		}
		discardEntered := make(chan struct{})
		releaseDiscard := make(chan struct{})
		var discardOnce sync.Once
		var clearDiscarding atomic.Bool
		prematureMaterializeErr := errors.New("replacement ledger materialized before clear discard completed")
		ledgerMaterializer := &testLedgerMaterializer{
			delegate: materializer,
			materialize: func(ctx context.Context, record store.SessionLedgerRecord) error {
				if clearDiscarding.Load() {
					return prematureMaterializeErr
				}
				return materializer.MaterializeSessionLedger(ctx, record)
			},
			discard: func(ctx context.Context, record store.SessionLedgerRecord) error {
				clearDiscarding.Store(true)
				discardOnce.Do(func() { close(discardEntered) })
				select {
				case <-releaseDiscard:
				case <-ctx.Done():
					return ctx.Err()
				}
				err := materializer.DiscardSessionLedger(ctx, record)
				clearDiscarding.Store(false)
				return err
			},
		}
		h.manager = newManagerWithHarness(t, h, WithLedgerMaterializer(ledgerMaterializer))

		session := createSession(t, h)
		eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "before serialized clear")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		collectEvents(t, eventsCh)

		type clearResult struct {
			session *Session
			err     error
		}
		clearDone := make(chan clearResult, 1)
		go func() {
			cleared, clearErr := h.manager.ClearConversation(testutil.Context(t), session.ID)
			clearDone <- clearResult{session: cleared, err: clearErr}
		}()
		select {
		case <-discardEntered:
		case <-testutil.Context(t).Done():
			t.Fatalf("ClearConversation() did not reach ledger discard: %v", testutil.Context(t).Err())
		}

		stopCtx, cancelStop := context.WithTimeout(testutil.Context(t), 100*time.Millisecond)
		stopErr := h.manager.Stop(stopCtx, session.ID)
		cancelStop()
		if !errors.Is(stopErr, context.DeadlineExceeded) {
			t.Fatalf("Stop(during clear) error = %v, want deadline while clear owns finalization", stopErr)
		}
		if errors.Is(stopErr, prematureMaterializeErr) {
			t.Fatalf("Stop(during clear) materialized the replacement ledger: %v", stopErr)
		}

		close(releaseDiscard)
		result := <-clearDone
		if result.err != nil {
			t.Fatalf("ClearConversation() error = %v", result.err)
		}
		if result.session == nil {
			t.Fatal("ClearConversation() session = nil")
		}
		if err := h.manager.Stop(testutil.Context(t), result.session.ID); err != nil {
			t.Fatalf("Stop(after clear) error = %v", err)
		}
		ledgerPath := filepath.Join(h.homePaths.SessionsDir, h.workspaceID, session.ID, "ledger.jsonl")
		ledger, err := os.ReadFile(ledgerPath)
		if err != nil {
			t.Fatalf("ReadFile(replacement ledger) error = %v", err)
		}
		if strings.Contains(string(ledger), "before serialized clear") {
			t.Fatalf("replacement ledger retained cleared content: %s", ledger)
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
			WithStore(func(ctx context.Context, owner store.SessionDBOwner, path string) (EventRecorder, error) {
				recorder, err := sessiondb.OpenSessionDB(ctx, owner, path)
				if err != nil {
					return nil, err
				}
				if openCount.Add(1) == 2 {
					if err := recorder.Record(ctx, store.SessionEvent{
						TurnID:    "turn-stale-reset",
						Type:      acp.EventTypeUserMessage,
						AgentName: "coder",
						Content:   "{\"schema\":\"compozy.session.event.v1\",\"type\":\"user_message\",\"text\":\"stale reset row\"}",
					}); err != nil {
						closeCtx := testutil.Context(t)
						return nil, errors.Join(err, recorder.Close(closeCtx))
					}
				}
				return recorder, nil
			}),
			WithQueryStore(func(ctx context.Context, owner store.SessionDBOwner, path string) (EventReadCloser, error) {
				return sessiondb.OpenSessionDBReadOnly(ctx, owner, path)
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
		materializer, err := sessionledger.NewMaterializer(sessionledger.Config{
			RootDir: h.homePaths.SessionsDir,
		})
		if err != nil {
			t.Fatalf("NewMaterializer() error = %v", err)
		}
		epochStore := newFakeTranscriptEpochStore()
		epochStore.ensureErr = errors.New("epoch store unavailable")
		h.manager = newManagerWithHarness(
			t,
			h,
			WithTranscriptEpochStore(epochStore),
			WithLedgerMaterializer(materializer),
		)
		session := createSession(t, h)
		eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "before clear")
		if err != nil {
			t.Fatalf("Prompt(before clear) error = %v", err)
		}
		collectEvents(t, eventsCh)
		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("Stop(before failed clear) error = %v", err)
		}
		ledgerPath := filepath.Join(h.homePaths.SessionsDir, h.workspaceID, session.ID, "ledger.jsonl")
		ledgerBefore, err := os.ReadFile(ledgerPath)
		if err != nil {
			t.Fatalf("ReadFile(ledger before failed clear) error = %v", err)
		}
		if !strings.Contains(string(ledgerBefore), "before clear") {
			t.Fatalf("ledger before failed clear does not contain original prompt: %s", ledgerBefore)
		}
		if _, err := h.manager.Resume(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("Resume(before failed clear) error = %v", err)
		}

		_, err = h.manager.ClearConversation(testutil.Context(t), session.ID)
		if err == nil {
			t.Fatal("ClearConversation() error = nil, want epoch commit failure")
		}
		if !strings.Contains(err.Error(), "ensure transcript epoch") {
			t.Fatalf("ClearConversation() error = %v, want transcript epoch failure", err)
		}
		clearErr := err
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
		ledgerAfter, readErr := os.ReadFile(ledgerPath)
		if readErr != nil {
			t.Fatalf("ReadFile(ledger after failed clear) error = %v; clear error = %v", readErr, clearErr)
		}
		if !strings.Contains(string(ledgerAfter), "before clear") ||
			!strings.Contains(string(ledgerAfter), EventTypeSessionStopped) {
			t.Fatalf("ledger after failed clear does not reflect restored stopped history: %s", ledgerAfter)
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
		owner := testSessionDBOwner(session.ID, session.WorkspaceID)
		manifest := backupOwnedSessionDBForTest(t, owner, dbPath)
		recorder, err := sessiondb.OpenSessionDB(
			testutil.Context(t),
			owner,
			dbPath,
		)
		if err != nil {
			t.Fatalf("OpenSessionDB(fresh committed DB) error = %v", err)
		}
		if err := recorder.Close(testutil.Context(t)); err != nil {
			t.Fatalf("Close(fresh committed DB) error = %v", err)
		}
		if err := commitSessionDBClear(dbPath, manifest.Generation, 7); err != nil {
			t.Fatalf("commitSessionDBClear() error = %v", err)
		}

		lease, err := sessiondb.AcquireFamilyLease(testutil.Context(t), dbPath)
		if err != nil {
			t.Fatalf("AcquireFamilyLease() error = %v", err)
		}
		defer lease.Release()
		_, err = h.manager.backupSessionDBForClear(
			testutil.Context(t),
			lease,
			owner,
			dbPath,
			session.ID,
		)
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

	t.Run("Should refuse a foreign database family before clear mutation", func(t *testing.T) {
		h := newHarness(t)
		target := createSession(t, h)
		foreign := createSession(t, h)
		if err := h.manager.Stop(testutil.Context(t), target.ID); err != nil {
			t.Fatalf("Stop(target) error = %v", err)
		}
		if err := h.manager.Stop(testutil.Context(t), foreign.ID); err != nil {
			t.Fatalf("Stop(foreign) error = %v", err)
		}

		substituteManagerSessionDBFamily(t, target.DBPath(), foreign.DBPath())
		before := readManagerSessionDBFamilyDigest(t, target.DBPath())
		_, err := h.manager.ClearConversation(testutil.Context(t), target.ID)
		if !errors.Is(err, sessiondb.ErrSessionDBOwnerMismatch) {
			t.Fatalf("ClearConversation(foreign family) error = %v, want owner mismatch", err)
		}
		assertManagerSessionDBFamilyUnchanged(t, target.DBPath(), before)
		if _, statErr := os.Stat(target.SessionDir()); statErr != nil {
			t.Fatalf("Stat(target session directory after refusal) error = %v", statErr)
		}
	})

	t.Run("Should reject coordinated metadata and database owner substitution against the catalog", func(t *testing.T) {
		catalog := newRecordingSessionCatalog()
		h := newHarness(t, WithSessionCatalog(catalog))
		target := createSession(t, h)
		if err := h.manager.Stop(testutil.Context(t), target.ID); err != nil {
			t.Fatalf("Stop(target) error = %v", err)
		}

		foreignWorkspaceID := "ws-coordinated-substitution"
		foreignPath := filepath.Join(t.TempDir(), store.SessionDatabaseName)
		foreignOwner := store.SessionDBOwner{SessionID: target.ID, WorkspaceID: foreignWorkspaceID}
		foreignDB, err := sessiondb.OpenSessionDB(testutil.Context(t), foreignOwner, foreignPath)
		if err != nil {
			t.Fatalf("OpenSessionDB(foreign coordinated owner) error = %v", err)
		}
		if err := foreignDB.Close(testutil.Context(t)); err != nil {
			t.Fatalf("Close(foreign coordinated owner) error = %v", err)
		}
		if err := os.Rename(target.DBPath(), target.DBPath()+".original"); err != nil {
			t.Fatalf("Rename(original database) error = %v", err)
		}
		if err := os.Rename(foreignPath, target.DBPath()); err != nil {
			t.Fatalf("Rename(foreign database) error = %v", err)
		}
		meta, err := store.ReadSessionMeta(store.SessionMetaFile(target.SessionDir()))
		if err != nil {
			t.Fatalf("ReadSessionMeta(target) error = %v", err)
		}
		meta.WorkspaceID = foreignWorkspaceID
		meta.Provider = ""
		meta.State = string(StateActive)
		metaPath := store.SessionMetaFile(target.SessionDir())
		if err := store.WriteSessionMeta(metaPath, meta); err != nil {
			t.Fatalf("WriteSessionMeta(coordinated substitution) error = %v", err)
		}
		metaBefore, err := os.ReadFile(metaPath)
		if err != nil {
			t.Fatalf("ReadFile(metadata before refusal) error = %v", err)
		}
		before := readManagerSessionDBFamilyDigest(t, target.DBPath())

		_, err = h.manager.ClearConversation(testutil.Context(t), target.ID)
		if !errors.Is(err, store.ErrSessionWorkspaceMismatch) {
			t.Fatalf("ClearConversation(coordinated substitution) error = %v, want workspace mismatch", err)
		}
		assertManagerSessionDBFamilyUnchanged(t, target.DBPath(), before)
		metaAfter, readErr := os.ReadFile(metaPath)
		if readErr != nil {
			t.Fatalf("ReadFile(metadata after refusal) error = %v", readErr)
		}
		if !bytes.Equal(metaAfter, metaBefore) {
			t.Fatalf("metadata changed before catalog owner refusal:\n got: %s\nwant: %s", metaAfter, metaBefore)
		}
	})
}

func TestClearConversationRejectsPromptInProgress(t *testing.T) {
	t.Parallel()

	t.Run("Should reject clearing while a prompt is in progress", func(t *testing.T) {
		h := newHarness(t)
		session := createSession(t, h)
		releasePrompt := make(chan struct{})
		promptStarted := make(chan struct{})
		h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			close(promptStarted)
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
		select {
		case <-promptStarted:
		case <-time.After(time.Second):
			t.Fatal("driver prompt did not start")
		}
		if !session.IsPrompting() {
			t.Fatal("session IsPrompting() = false after driver prompt started")
		}

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

type managerSessionDBFileDigest struct {
	exists bool
	digest [sha256.Size]byte
}

type managerSessionDBFamilyDigest struct {
	database managerSessionDBFileDigest
	wal      managerSessionDBFileDigest
	shm      managerSessionDBFileDigest
	journal  managerSessionDBFileDigest
}

type managerSessionDBClearStateDigest [10]managerSessionDBFileDigest

type testLedgerMaterializer struct {
	delegate    LedgerMaterializer
	materialize func(context.Context, store.SessionLedgerRecord) error
	discard     func(context.Context, store.SessionLedgerRecord) error
}

func (m *testLedgerMaterializer) MaterializeSessionLedger(
	ctx context.Context,
	record store.SessionLedgerRecord,
) error {
	if m.materialize != nil {
		return m.materialize(ctx, record)
	}
	return m.delegate.MaterializeSessionLedger(ctx, record)
}

func (m *testLedgerMaterializer) DiscardSessionLedger(
	ctx context.Context,
	record store.SessionLedgerRecord,
) error {
	if m.discard != nil {
		return m.discard(ctx, record)
	}
	return m.delegate.DiscardSessionLedger(ctx, record)
}

func substituteManagerSessionDBFamily(t *testing.T, targetPath string, foreignPath string) {
	t.Helper()
	for _, suffix := range sessionDBClearArtifactSuffixes {
		target := targetPath + suffix
		if _, err := os.Lstat(target); err == nil {
			if err := os.Rename(target, target+".before-owner-substitution"); err != nil {
				t.Fatalf("Rename(target family %q) error = %v", target, err)
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Lstat(target family %q) error = %v", target, err)
		}

		foreign := foreignPath + suffix
		if _, err := os.Lstat(foreign); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			t.Fatalf("Lstat(foreign family %q) error = %v", foreign, err)
		}
		if err := os.Rename(foreign, target); err != nil {
			t.Fatalf("Rename(foreign family %q to %q) error = %v", foreign, target, err)
		}
	}
}

func readManagerSessionDBFamilyDigest(t *testing.T, path string) managerSessionDBFamilyDigest {
	t.Helper()
	return managerSessionDBFamilyDigest{
		database: readManagerSessionDBFileDigest(t, path),
		wal:      readManagerSessionDBFileDigest(t, path+"-wal"),
		shm:      readManagerSessionDBFileDigest(t, path+"-shm"),
		journal:  readManagerSessionDBFileDigest(t, path+"-journal"),
	}
}

func readManagerSessionDBClearStateDigest(t *testing.T, dbPath string) managerSessionDBClearStateDigest {
	t.Helper()
	paths := [...]string{
		dbPath,
		dbPath + "-wal",
		dbPath + "-shm",
		dbPath + "-journal",
		dbPath + ".clear-backup",
		dbPath + "-wal.clear-backup",
		dbPath + "-shm.clear-backup",
		dbPath + "-journal.clear-backup",
		sessionDBClearManifestPath(dbPath),
		sessionDBClearCommitPath(dbPath),
	}
	var digest managerSessionDBClearStateDigest
	for index, path := range paths {
		digest[index] = readManagerSessionDBFileDigest(t, path)
	}
	return digest
}

func readManagerSessionDBFileDigest(t *testing.T, path string) managerSessionDBFileDigest {
	t.Helper()
	contents, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return managerSessionDBFileDigest{}
	}
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	return managerSessionDBFileDigest{exists: true, digest: sha256.Sum256(contents)}
}

func assertManagerSessionDBFamilyUnchanged(
	t *testing.T,
	path string,
	want managerSessionDBFamilyDigest,
) {
	t.Helper()
	if got := readManagerSessionDBFamilyDigest(t, path); got != want {
		t.Fatalf("session database family %q changed: got %#v want %#v", path, got, want)
	}
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

func backupOwnedSessionDBForTest(
	t *testing.T,
	owner store.SessionDBOwner,
	dbPath string,
) sessionDBClearManifest {
	t.Helper()
	ctx := testutil.Context(t)
	lease, err := sessiondb.AcquireFamilyLease(ctx, dbPath)
	if err != nil {
		t.Fatalf("AcquireFamilyLease(%q) error = %v", dbPath, err)
	}
	defer lease.Release()
	manifest, err := createSessionDBClearManifest(ctx, lease, owner, dbPath)
	if err != nil {
		t.Fatalf("createSessionDBClearManifest(%q) error = %v", dbPath, err)
	}
	if err := backupSessionDBManifestArtifacts(ctx, lease, owner, dbPath, manifest); err != nil {
		t.Fatalf("backupSessionDBManifestArtifacts(%q) error = %v", dbPath, err)
	}
	return manifest
}

func TestBackupSessionDB(t *testing.T) {
	t.Parallel()

	t.Run("Should refuse a blocked backup target before moving any owned artifact", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		dbPath := filepath.Join(dir, "session.db")
		walPath := dbPath + "-wal"
		owner := testSessionDBOwner("sess-clear-blocked-backup", "ws-clear-blocked-backup")
		db, err := sessiondb.OpenSessionDB(testutil.Context(t), owner, dbPath)
		if err != nil {
			t.Fatalf("OpenSessionDB() error = %v", err)
		}
		if err := db.Close(testutil.Context(t)); err != nil {
			t.Fatalf("Close() error = %v", err)
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

		before := readManagerSessionDBFamilyDigest(t, dbPath)
		ctx := testutil.Context(t)
		lease, err := sessiondb.AcquireFamilyLease(ctx, dbPath)
		if err != nil {
			t.Fatalf("AcquireFamilyLease() error = %v", err)
		}
		defer lease.Release()
		manifest, err := createSessionDBClearManifest(ctx, lease, owner, dbPath)
		if err != nil {
			t.Fatalf("createSessionDBClearManifest() error = %v", err)
		}
		err = backupSessionDBManifestArtifacts(ctx, lease, owner, dbPath, manifest)
		if err == nil {
			t.Fatal("backupSessionDBManifestArtifacts() error = nil, want blocked target refusal")
		}
		if !strings.Contains(err.Error(), "not a regular file") {
			t.Fatalf("backupSessionDBManifestArtifacts() error = %v, want regular-file refusal", err)
		}
		assertManagerSessionDBFamilyUnchanged(t, dbPath, before)
		if _, statErr := os.Stat(dbPath + ".clear-backup"); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("Stat(session.db.clear-backup) error = %v, want os.ErrNotExist", statErr)
		}
		if _, statErr := os.Stat(blockedBackupDir); statErr != nil {
			t.Fatalf("Stat(blocked backup dir) error = %v", statErr)
		}
	})

	t.Run("Should restore interrupted clear backups before stored event queries", func(t *testing.T) {
		t.Parallel()

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
		backupOwnedSessionDBForTest(t, testSessionDBOwner(session.ID, session.WorkspaceID), dbPath)
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

	t.Run("Should release the held family lease before rollback recovery", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		dbPath := session.DBPath()
		owner := testSessionDBOwner(session.ID, session.WorkspaceID)
		metaPath := store.SessionMetaFile(session.SessionDir())
		meta, err := store.ReadSessionMeta(metaPath)
		if err != nil {
			t.Fatalf("ReadSessionMeta() error = %v", err)
		}
		metaBefore, err := os.ReadFile(metaPath)
		if err != nil {
			t.Fatalf("ReadFile(metadata before rollback) error = %v", err)
		}
		familyBefore := readManagerSessionDBFamilyDigest(t, dbPath)
		manifest := backupOwnedSessionDBForTest(t, owner, dbPath)
		if err := store.WriteSessionMeta(metaPath, clearedConversationMeta(meta, h.manager.now())); err != nil {
			t.Fatalf("WriteSessionMeta(cleared metadata) error = %v", err)
		}

		lease, err := sessiondb.AcquireFamilyLease(testutil.Context(t), dbPath)
		if err != nil {
			t.Fatalf("AcquireFamilyLease() error = %v", err)
		}
		t.Cleanup(lease.Release)
		cleanup := sessionDBClearCleanup{
			manager:     h.manager,
			familyLease: lease,
			owner:       owner,
			dbPath:      dbPath,
			metaPath:    metaPath,
			manifest:    manifest,
			meta:        meta,
		}
		if err := cleanup.run(testutil.Context(t), sessionDBClearRollback); err != nil {
			t.Fatalf("sessionDBClearCleanup.run(rollback) error = %v", err)
		}

		assertManagerSessionDBFamilyUnchanged(t, dbPath, familyBefore)
		metaAfter, err := os.ReadFile(metaPath)
		if err != nil {
			t.Fatalf("ReadFile(metadata after rollback) error = %v", err)
		}
		if !bytes.Equal(metaAfter, metaBefore) {
			t.Fatalf("metadata changed across rollback:\n got: %s\nwant: %s", metaAfter, metaBefore)
		}
		for _, path := range []string{
			dbPath + ".clear-backup",
			sessionDBClearManifestPath(dbPath),
			sessionDBClearCommitPath(dbPath),
		} {
			if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("Stat(%q) error = %v, want os.ErrNotExist", path, statErr)
			}
		}
	})

	t.Run("Should discard committed clear backups without restoring old events", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		dbPath := filepath.Join(dir, "session.db")
		backupPath := dbPath + ".clear-backup"
		owner := testSessionDBOwner("sess-clear-committed", "ws-clear-committed")
		oldDB, err := sessiondb.OpenSessionDB(testutil.Context(t), owner, dbPath)
		if err != nil {
			t.Fatalf("OpenSessionDB(old) error = %v", err)
		}
		if err := oldDB.Close(testutil.Context(t)); err != nil {
			t.Fatalf("Close(old) error = %v", err)
		}
		materializer, err := sessionledger.NewMaterializer(sessionledger.Config{
			RootDir: filepath.Join(dir, "ledgers"),
		})
		if err != nil {
			t.Fatalf("NewMaterializer() error = %v", err)
		}
		ledgerRecord := store.SessionLedgerRecord{
			SessionID:    owner.SessionID,
			WorkspaceID:  owner.WorkspaceID,
			AgentName:    "coder",
			EventsDBPath: dbPath,
		}
		if err := materializer.MaterializeSessionLedger(testutil.Context(t), ledgerRecord); err != nil {
			t.Fatalf("MaterializeSessionLedger() error = %v", err)
		}
		ledgerPath, err := materializer.Path(ledgerRecord)
		if err != nil {
			t.Fatalf("Materializer.Path() error = %v", err)
		}
		manifest := backupOwnedSessionDBForTest(t, owner, dbPath)
		freshDB, err := sessiondb.OpenSessionDB(testutil.Context(t), owner, dbPath)
		if err != nil {
			t.Fatalf("OpenSessionDB(fresh) error = %v", err)
		}
		if err := freshDB.Close(testutil.Context(t)); err != nil {
			t.Fatalf("Close(fresh) error = %v", err)
		}
		freshDigest := readManagerSessionDBFileDigest(t, dbPath)
		if err := commitSessionDBClear(dbPath, manifest.Generation, 1); err != nil {
			t.Fatalf("commitSessionDBClear() error = %v", err)
		}
		var discardObserved atomic.Bool
		ledgerMaterializer := &testLedgerMaterializer{
			delegate: materializer,
			discard: func(ctx context.Context, record store.SessionLedgerRecord) error {
				for _, path := range []string{
					dbPath + ".clear-backup",
					sessionDBClearManifestPath(dbPath),
					sessionDBClearCommitPath(dbPath),
				} {
					if _, statErr := os.Stat(path); statErr != nil {
						return statErr
					}
				}
				discardObserved.Store(true)
				return materializer.DiscardSessionLedger(ctx, record)
			},
		}

		ctx := testutil.Context(t)
		lease, err := acquireVerifiedSessionDBFamilyLease(ctx, owner, dbPath, dbPath, backupPath)
		if err != nil {
			t.Fatalf("acquireVerifiedSessionDBFamilyLease() error = %v", err)
		}
		manager := &Manager{ledgerMaterializer: ledgerMaterializer}
		if err := manager.recoverSessionDBClear(ctx, lease, owner, dbPath); err != nil {
			lease.Release()
			t.Fatalf("recoverSessionDBClear(committed) error = %v", err)
		}
		lease.Release()
		if !discardObserved.Load() {
			t.Fatal("recoverSessionDBClear() did not discard the materialized ledger")
		}
		if got := readManagerSessionDBFileDigest(t, dbPath); got != freshDigest {
			t.Fatalf("fresh database digest = %#v, want %#v", got, freshDigest)
		}
		if _, statErr := os.Stat(ledgerPath); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("Stat(discarded ledger) error = %v, want os.ErrNotExist", statErr)
		}
		if _, statErr := os.Stat(backupPath); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("Stat(committed backup) error = %v, want os.ErrNotExist", statErr)
		}
		if _, statErr := os.Stat(sessionDBClearCommitPath(dbPath)); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("Stat(clear commit marker) error = %v, want os.ErrNotExist", statErr)
		}
	})

	t.Run("Should finalize a committed manifest after the commit marker is already absent", func(t *testing.T) {
		t.Parallel()

		dbPath := filepath.Join(t.TempDir(), "session.db")
		owner := testSessionDBOwner("sess-clear-manifest-commit", "ws-clear-manifest-commit")
		oldDB, err := sessiondb.OpenSessionDB(testutil.Context(t), owner, dbPath)
		if err != nil {
			t.Fatalf("OpenSessionDB(old) error = %v", err)
		}
		if err := oldDB.Close(testutil.Context(t)); err != nil {
			t.Fatalf("Close(old) error = %v", err)
		}
		manifest := backupOwnedSessionDBForTest(t, owner, dbPath)
		freshDB, err := sessiondb.OpenSessionDB(testutil.Context(t), owner, dbPath)
		if err != nil {
			t.Fatalf("OpenSessionDB(fresh) error = %v", err)
		}
		if err := freshDB.Close(testutil.Context(t)); err != nil {
			t.Fatalf("Close(fresh) error = %v", err)
		}
		freshDigest := readManagerSessionDBFileDigest(t, dbPath)
		if err := commitSessionDBClear(dbPath, manifest.Generation, 9); err != nil {
			t.Fatalf("commitSessionDBClear() error = %v", err)
		}
		if err := discardSessionDBClearCommit(dbPath); err != nil {
			t.Fatalf("discardSessionDBClearCommit() error = %v", err)
		}

		ctx := testutil.Context(t)
		lease, err := acquireVerifiedSessionDBFamilyLease(ctx, owner, dbPath, dbPath, dbPath+".clear-backup")
		if err != nil {
			t.Fatalf("acquireVerifiedSessionDBFamilyLease() error = %v", err)
		}
		manager := &Manager{}
		if err := manager.recoverSessionDBClear(ctx, lease, owner, dbPath); err != nil {
			lease.Release()
			t.Fatalf("recoverSessionDBClear() error = %v", err)
		}
		lease.Release()
		if got := readManagerSessionDBFileDigest(t, dbPath); got != freshDigest {
			t.Fatalf("fresh database digest = %#v, want %#v", got, freshDigest)
		}
		for _, path := range []string{
			dbPath + ".clear-backup",
			sessionDBClearManifestPath(dbPath),
			sessionDBClearCommitPath(dbPath),
		} {
			if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("Stat(%q) error = %v, want finalized removal", path, statErr)
			}
		}
	})

	t.Run("Should keep committed clear marker when committed backup discard fails", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		dbPath := filepath.Join(dir, "session.db")
		backupPath := dbPath + ".clear-backup"
		owner := testSessionDBOwner("sess-clear-backup-discard", "ws-clear-backup-discard")

		db, err := sessiondb.OpenSessionDB(testutil.Context(t), owner, dbPath)
		if err != nil {
			t.Fatalf("OpenSessionDB(old session.db) error = %v", err)
		}
		if err := db.Close(testutil.Context(t)); err != nil {
			t.Fatalf("Close(old session.db) error = %v", err)
		}
		manifest := backupOwnedSessionDBForTest(t, owner, dbPath)
		freshDB, err := sessiondb.OpenSessionDB(testutil.Context(t), owner, dbPath)
		if err != nil {
			t.Fatalf("OpenSessionDB(fresh session.db) error = %v", err)
		}
		if err := freshDB.Close(testutil.Context(t)); err != nil {
			t.Fatalf("Close(fresh session.db) error = %v", err)
		}
		if err := os.Rename(backupPath, backupPath+".before-substitution"); err != nil {
			t.Fatalf("Rename(clear backup before substitution) error = %v", err)
		}
		if err := os.Mkdir(backupPath, 0o755); err != nil {
			t.Fatalf("Mkdir(clear backup blocker) error = %v", err)
		}
		if err := commitSessionDBClear(dbPath, manifest.Generation, 1); err != nil {
			t.Fatalf("commitSessionDBClear() error = %v", err)
		}

		err = (&Manager{}).finalizeCommittedSessionDBClear(
			testutil.Context(t),
			owner,
			dbPath,
			manifest,
		)
		if err == nil {
			t.Fatal("finalizeCommittedSessionDBClear() error = nil, want backup discard failure")
		}
		if !strings.Contains(err.Error(), "regular") && !strings.Contains(err.Error(), "directory") {
			t.Fatalf("finalizeCommittedSessionDBClear() error = %v, want non-regular backup refusal", err)
		}
		if _, statErr := os.Stat(sessionDBClearCommitPath(dbPath)); statErr != nil {
			t.Fatalf("Stat(clear commit marker) error = %v, want marker preserved", statErr)
		}
		if _, statErr := os.Stat(sessionDBClearManifestPath(dbPath)); statErr != nil {
			t.Fatalf("Stat(clear manifest) error = %v, want manifest preserved", statErr)
		}
	})

	t.Run("Should refuse a replaced captured sidecar before committed cleanup mutates any artifact", func(
		t *testing.T,
	) {
		t.Parallel()

		dbPath := filepath.Join(t.TempDir(), "session.db")
		owner := testSessionDBOwner("sess-clear-sidecar-substitute", "ws-clear-sidecar-substitute")
		db, err := sessiondb.OpenSessionDB(testutil.Context(t), owner, dbPath)
		if err != nil {
			t.Fatalf("OpenSessionDB(old) error = %v", err)
		}
		if err := db.Close(testutil.Context(t)); err != nil {
			t.Fatalf("Close(old) error = %v", err)
		}
		if err := os.WriteFile(dbPath+"-wal", []byte("captured-wal"), 0o600); err != nil {
			t.Fatalf("WriteFile(captured WAL) error = %v", err)
		}
		manifest := backupOwnedSessionDBForTest(t, owner, dbPath)
		freshDB, err := sessiondb.OpenSessionDB(testutil.Context(t), owner, dbPath)
		if err != nil {
			t.Fatalf("OpenSessionDB(fresh) error = %v", err)
		}
		if err := freshDB.Close(testutil.Context(t)); err != nil {
			t.Fatalf("Close(fresh) error = %v", err)
		}

		walBackup := dbPath + "-wal.clear-backup"
		capturedWal := walBackup + ".captured"
		if err := os.Rename(walBackup, capturedWal); err != nil {
			t.Fatalf("Rename(captured WAL backup) error = %v", err)
		}
		if err := os.WriteFile(walBackup, []byte("substitute-wal"), 0o600); err != nil {
			t.Fatalf("WriteFile(substitute WAL backup) error = %v", err)
		}
		if err := commitSessionDBClear(dbPath, manifest.Generation, 3); err != nil {
			t.Fatalf("commitSessionDBClear() error = %v", err)
		}
		before := readManagerSessionDBClearStateDigest(t, dbPath)
		capturedBefore := readManagerSessionDBFileDigest(t, capturedWal)

		err = (&Manager{}).finalizeCommittedSessionDBClear(
			testutil.Context(t),
			owner,
			dbPath,
			manifest,
		)
		if !errors.Is(err, sessiondb.ErrSessionDBFamilyChanged) {
			t.Fatalf("finalizeCommittedSessionDBClear(replaced WAL) error = %v, want family changed", err)
		}
		if got := readManagerSessionDBClearStateDigest(t, dbPath); got != before {
			t.Fatalf("clear state changed after replaced WAL refusal: got %#v want %#v", got, before)
		}
		if got := readManagerSessionDBFileDigest(t, capturedWal); got != capturedBefore {
			t.Fatalf("captured WAL changed after refusal: got %#v want %#v", got, capturedBefore)
		}
	})

	t.Run("Should refuse an unlisted backup sidecar before committed cleanup", func(t *testing.T) {
		t.Parallel()

		dbPath := filepath.Join(t.TempDir(), "session.db")
		owner := testSessionDBOwner("sess-clear-unlisted-sidecar", "ws-clear-unlisted-sidecar")
		db, err := sessiondb.OpenSessionDB(testutil.Context(t), owner, dbPath)
		if err != nil {
			t.Fatalf("OpenSessionDB(old) error = %v", err)
		}
		if err := db.Close(testutil.Context(t)); err != nil {
			t.Fatalf("Close(old) error = %v", err)
		}
		manifest := backupOwnedSessionDBForTest(t, owner, dbPath)
		freshDB, err := sessiondb.OpenSessionDB(testutil.Context(t), owner, dbPath)
		if err != nil {
			t.Fatalf("OpenSessionDB(fresh) error = %v", err)
		}
		if err := freshDB.Close(testutil.Context(t)); err != nil {
			t.Fatalf("Close(fresh) error = %v", err)
		}
		if manifest.hasArtifact("-journal") {
			t.Fatal("manifest unexpectedly captured a rollback journal")
		}
		unexpectedBackup := dbPath + "-journal.clear-backup"
		if err := os.WriteFile(unexpectedBackup, []byte("unlisted-journal"), 0o600); err != nil {
			t.Fatalf("WriteFile(unlisted journal backup) error = %v", err)
		}
		if err := commitSessionDBClear(dbPath, manifest.Generation, 4); err != nil {
			t.Fatalf("commitSessionDBClear() error = %v", err)
		}
		before := readManagerSessionDBClearStateDigest(t, dbPath)

		err = (&Manager{}).finalizeCommittedSessionDBClear(
			testutil.Context(t),
			owner,
			dbPath,
			manifest,
		)
		if !errors.Is(err, sessiondb.ErrSessionDBFamilyChanged) {
			t.Fatalf("finalizeCommittedSessionDBClear(unlisted sidecar) error = %v, want family changed", err)
		}
		if got := readManagerSessionDBClearStateDigest(t, dbPath); got != before {
			t.Fatalf("clear state changed after unlisted sidecar refusal: got %#v want %#v", got, before)
		}
	})

	t.Run("Should reject a sidecar created after the manifest snapshot before backup mutation", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		dbPath := filepath.Join(t.TempDir(), "session.db")
		owner := testSessionDBOwner("sess-clear-late-sidecar", "ws-clear-late-sidecar")
		db, err := sessiondb.OpenSessionDB(ctx, owner, dbPath)
		if err != nil {
			t.Fatalf("OpenSessionDB(old) error = %v", err)
		}
		if err := db.Close(ctx); err != nil {
			t.Fatalf("Close(old) error = %v", err)
		}
		lease, err := sessiondb.AcquireFamilyLease(ctx, dbPath)
		if err != nil {
			t.Fatalf("AcquireFamilyLease() error = %v", err)
		}
		defer lease.Release()
		manifest, err := createSessionDBClearManifest(ctx, lease, owner, dbPath)
		if err != nil {
			t.Fatalf("createSessionDBClearManifest() error = %v", err)
		}
		if manifest.hasArtifact("-journal") {
			t.Fatal("manifest unexpectedly captured a rollback journal")
		}
		if err := os.WriteFile(dbPath+"-journal", []byte("late-journal"), 0o600); err != nil {
			t.Fatalf("WriteFile(late rollback journal) error = %v", err)
		}
		before := readManagerSessionDBClearStateDigest(t, dbPath)

		err = backupSessionDBManifestArtifacts(ctx, lease, owner, dbPath, manifest)
		if !errors.Is(err, sessiondb.ErrSessionDBFamilyChanged) {
			t.Fatalf("backupSessionDBManifestArtifacts(late sidecar) error = %v, want family changed", err)
		}
		if got := readManagerSessionDBClearStateDigest(t, dbPath); got != before {
			t.Fatalf("clear state changed after late-sidecar refusal: got %#v want %#v", got, before)
		}
	})

	t.Run("Should refuse a replaced sidecar before interrupted restore mutates any artifact", func(t *testing.T) {
		t.Parallel()

		dbPath := filepath.Join(t.TempDir(), "session.db")
		owner := testSessionDBOwner("sess-clear-restore-sidecar", "ws-clear-restore-sidecar")
		db, err := sessiondb.OpenSessionDB(testutil.Context(t), owner, dbPath)
		if err != nil {
			t.Fatalf("OpenSessionDB(old) error = %v", err)
		}
		if err := db.Close(testutil.Context(t)); err != nil {
			t.Fatalf("Close(old) error = %v", err)
		}
		if err := os.WriteFile(dbPath+"-shm", []byte("captured-shm"), 0o600); err != nil {
			t.Fatalf("WriteFile(captured SHM) error = %v", err)
		}
		manifest := backupOwnedSessionDBForTest(t, owner, dbPath)
		shmBackup := dbPath + "-shm.clear-backup"
		capturedShm := shmBackup + ".captured"
		if err := os.Rename(shmBackup, capturedShm); err != nil {
			t.Fatalf("Rename(captured SHM backup) error = %v", err)
		}
		if err := os.WriteFile(shmBackup, []byte("substitute-shm"), 0o600); err != nil {
			t.Fatalf("WriteFile(substitute SHM backup) error = %v", err)
		}
		before := readManagerSessionDBClearStateDigest(t, dbPath)
		capturedBefore := readManagerSessionDBFileDigest(t, capturedShm)

		ctx := testutil.Context(t)
		lease, err := acquireVerifiedSessionDBFamilyLease(
			ctx,
			owner,
			dbPath,
			dbPath+".clear-backup",
		)
		if err != nil {
			t.Fatalf("acquireVerifiedSessionDBFamilyLease() error = %v", err)
		}
		err = restoreSessionDBManifestArtifacts(ctx, lease, owner, dbPath, manifest)
		lease.Release()
		if !errors.Is(err, sessiondb.ErrSessionDBFamilyChanged) {
			t.Fatalf("restoreSessionDBManifestArtifacts(replaced SHM) error = %v, want family changed", err)
		}
		if got := readManagerSessionDBClearStateDigest(t, dbPath); got != before {
			t.Fatalf("clear state changed after replaced SHM refusal: got %#v want %#v", got, before)
		}
		if got := readManagerSessionDBFileDigest(t, capturedShm); got != capturedBefore {
			t.Fatalf("captured SHM changed after refusal: got %#v want %#v", got, capturedBefore)
		}
	})

	t.Run("Should preserve every clear artifact when commit generation diverges", func(t *testing.T) {
		t.Parallel()

		dbPath := filepath.Join(t.TempDir(), "session.db")
		owner := testSessionDBOwner("sess-clear-generation", "ws-clear-generation")
		db, err := sessiondb.OpenSessionDB(testutil.Context(t), owner, dbPath)
		if err != nil {
			t.Fatalf("OpenSessionDB(old) error = %v", err)
		}
		if err := db.Close(testutil.Context(t)); err != nil {
			t.Fatalf("Close(old) error = %v", err)
		}
		manifest := backupOwnedSessionDBForTest(t, owner, dbPath)
		freshDB, err := sessiondb.OpenSessionDB(testutil.Context(t), owner, dbPath)
		if err != nil {
			t.Fatalf("OpenSessionDB(fresh) error = %v", err)
		}
		if err := freshDB.Close(testutil.Context(t)); err != nil {
			t.Fatalf("Close(fresh) error = %v", err)
		}
		if err := writeSessionDBClearCommit(dbPath, sessionDBClearCommit{
			Generation:      manifest.Generation + "-different",
			TranscriptEpoch: 5,
		}); err != nil {
			t.Fatalf("writeSessionDBClearCommit() error = %v", err)
		}
		before := readManagerSessionDBClearStateDigest(t, dbPath)

		err = (&Manager{}).finalizeCommittedSessionDBClear(
			testutil.Context(t),
			owner,
			dbPath,
			manifest,
		)
		if !errors.Is(err, sessiondb.ErrSessionDBFamilyChanged) {
			t.Fatalf("finalizeCommittedSessionDBClear(generation mismatch) error = %v, want family changed", err)
		}
		if got := readManagerSessionDBClearStateDigest(t, dbPath); got != before {
			t.Fatalf("clear state changed after generation refusal: got %#v want %#v", got, before)
		}
	})

	t.Run("Should refuse a foreign interrupted backup before stored-event recovery", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		target := createSession(t, h)
		foreign := createSession(t, h)
		if err := h.manager.Stop(testutil.Context(t), target.ID); err != nil {
			t.Fatalf("Stop(target) error = %v", err)
		}
		if err := h.manager.Stop(testutil.Context(t), foreign.ID); err != nil {
			t.Fatalf("Stop(foreign) error = %v", err)
		}
		backupOwnedSessionDBForTest(
			t,
			testSessionDBOwner(target.ID, target.WorkspaceID),
			target.DBPath(),
		)
		for _, suffix := range sessionDBClearArtifactSuffixes {
			targetBackup := target.DBPath() + suffix + ".clear-backup"
			if _, err := os.Lstat(targetBackup); err == nil {
				if err := os.Rename(targetBackup, targetBackup+".before-owner-substitution"); err != nil {
					t.Fatalf("Rename(target backup %q) error = %v", targetBackup, err)
				}
			} else if !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("Lstat(target backup %q) error = %v", targetBackup, err)
			}
			foreignPath := foreign.DBPath() + suffix
			if _, err := os.Lstat(foreignPath); errors.Is(err, os.ErrNotExist) {
				continue
			} else if err != nil {
				t.Fatalf("Lstat(foreign family %q) error = %v", foreignPath, err)
			}
			if err := os.Rename(foreignPath, targetBackup); err != nil {
				t.Fatalf("Rename(foreign family %q to %q) error = %v", foreignPath, targetBackup, err)
			}
		}
		before := readManagerSessionDBClearBackupDigest(t, target.DBPath())

		freshManager := newManagerWithHarness(t, h)
		_, err := freshManager.Events(testutil.Context(t), target.ID, store.EventQuery{})
		if !errors.Is(err, sessiondb.ErrSessionDBOwnerMismatch) {
			t.Fatalf("Events(foreign interrupted backup) error = %v, want owner mismatch", err)
		}
		if _, statErr := os.Stat(target.DBPath()); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("Stat(target database after refusal) error = %v, want os.ErrNotExist", statErr)
		}
		if got := readManagerSessionDBClearBackupDigest(t, target.DBPath()); got != before {
			t.Fatalf("foreign clear backup changed: got %#v want %#v", got, before)
		}
	})
}

func readManagerSessionDBClearBackupDigest(t *testing.T, dbPath string) managerSessionDBFamilyDigest {
	t.Helper()
	return managerSessionDBFamilyDigest{
		database: readManagerSessionDBFileDigest(t, dbPath+".clear-backup"),
		wal:      readManagerSessionDBFileDigest(t, dbPath+"-wal.clear-backup"),
		shm:      readManagerSessionDBFileDigest(t, dbPath+"-shm.clear-backup"),
		journal:  readManagerSessionDBFileDigest(t, dbPath+"-journal.clear-backup"),
	}
}
