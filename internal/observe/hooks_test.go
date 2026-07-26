package observe

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/sessiondb"
	"github.com/compozy/agh/internal/testutil"
)

func TestObserverAttachHooksAndQueryHookCatalog(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	if entries, err := h.observer.QueryHookCatalog(
		testutil.Context(t),
		hookspkg.CatalogFilter{},
	); err != nil ||
		entries != nil {
		t.Fatalf("QueryHookCatalog(before attach) = (%#v, %v), want (nil, nil)", entries, err)
	}

	source := &stubHookCatalogSource{
		entries: []hookspkg.CatalogEntry{{
			Order:  1,
			Name:   "catalog-hook",
			Event:  hookspkg.HookSessionPostCreate,
			Source: hookspkg.HookSourceConfig,
			Mode:   hookspkg.HookModeSync,
		}},
	}
	h.observer.AttachHooks(source)

	entries, err := h.observer.QueryHookCatalog(testutil.Context(t), hookspkg.CatalogFilter{
		WorkspaceID: h.workspaceID,
		AgentName:   "coder",
	})
	if err != nil {
		t.Fatalf("QueryHookCatalog() error = %v", err)
	}
	if got, want := len(entries), 1; got != want {
		t.Fatalf("len(entries) = %d, want %d", got, want)
	}
	if entries[0].Name != "catalog-hook" {
		t.Fatalf("entries[0].Name = %q, want catalog-hook", entries[0].Name)
	}
	if source.lastFilter.WorkspaceID != h.workspaceID || source.lastFilter.AgentName != "coder" {
		t.Fatalf("lastFilter = %#v", source.lastFilter)
	}
}

func TestObserverWriteHookRecordAndQueryHookRuns(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	sessionID := "sess-hook-audit"
	db := openObserverHookSessionDB(t, h.home, sessionID)
	closeObserverHookSessionDB(t, db)

	recordedAt := time.Date(2026, 4, 9, 19, 0, 0, 0, time.UTC)
	record := hookspkg.HookRunRecord{
		HookName:      "permission-audit",
		Event:         hookspkg.HookPermissionRequest,
		Source:        hookspkg.HookSourceConfig,
		Mode:          hookspkg.HookModeSync,
		Duration:      20 * time.Millisecond,
		Outcome:       hookspkg.HookRunOutcomeDenied,
		DispatchDepth: 2,
		PatchApplied:  []byte(`{"decision":"deny","reason":"policy"}`),
		Required:      true,
		RecordedAt:    recordedAt,
	}

	if err := h.observer.WriteHookRecord(testutil.Context(t), sessionID, record); err != nil {
		t.Fatalf("WriteHookRecord() error = %v", err)
	}

	records, err := h.observer.QueryHookRuns(testutil.Context(t), store.HookRunQuery{
		SessionID: sessionID,
		Event:     hookspkg.HookPermissionRequest.String(),
	})
	if err != nil {
		t.Fatalf("QueryHookRuns() error = %v", err)
	}
	if got, want := len(records), 1; got != want {
		t.Fatalf("len(records) = %d, want %d", got, want)
	}
	if records[0].HookName != "permission-audit" {
		t.Fatalf("records[0].HookName = %q, want permission-audit", records[0].HookName)
	}
	if string(records[0].PatchApplied) != `{"decision":"deny","reason":"policy"}` {
		t.Fatalf("records[0].PatchApplied = %s, want deny patch", records[0].PatchApplied)
	}
}

func TestObserverHookRunQueriesHandleMissingDBAndEvents(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	record := hookspkg.HookRunRecord{
		HookName:      "missing-db",
		Event:         hookspkg.HookSessionPostCreate,
		Source:        hookspkg.HookSourceConfig,
		Mode:          hookspkg.HookModeSync,
		Outcome:       hookspkg.HookRunOutcomeApplied,
		DispatchDepth: 1,
		RecordedAt:    time.Date(2026, 4, 9, 19, 5, 0, 0, time.UTC),
	}

	if err := h.observer.WriteHookRecord(testutil.Context(t), "missing-session", record); err != nil {
		t.Fatalf("WriteHookRecord(missing) error = %v", err)
	}
	records, err := h.observer.QueryHookRuns(testutil.Context(t), store.HookRunQuery{SessionID: "missing-session"})
	if err != nil {
		t.Fatalf("QueryHookRuns(missing) error = %v", err)
	}
	if records != nil {
		t.Fatalf("records = %#v, want nil for missing session DB", records)
	}

	events, err := h.observer.QueryHookEvents(testutil.Context(t), hookspkg.EventFilter{})
	if err != nil {
		t.Fatalf("QueryHookEvents() error = %v", err)
	}
	if got, want := len(events), len(hookspkg.AllEventDescriptors()); got != want {
		t.Fatalf("len(events) = %d, want %d", got, want)
	}
}

func TestObserverHookRunQueriesRejectUnsafeSessionID(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	_, err := h.observer.QueryHookRuns(testutil.Context(t), store.HookRunQuery{
		SessionID: "../escape",
	})
	if err == nil {
		t.Fatal("QueryHookRuns(unsafe session id) error = nil, want non-nil")
	}
	if got := err.Error(); got != `observe: invalid session id "../escape"` {
		t.Fatalf("QueryHookRuns(unsafe session id) error = %q, want invalid session id", got)
	}
}

func TestObserverHookOptionsUseCustomSourcesAndStores(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	source := &stubHookCatalogSource{
		entries: []hookspkg.CatalogEntry{{
			Order:  1,
			Name:   "option-hook",
			Event:  hookspkg.HookInputPreSubmit,
			Source: hookspkg.HookSourceSkill,
			Mode:   hookspkg.HookModeSync,
		}},
	}
	storeHandle := &stubHookRunStore{
		records: []hookspkg.HookRunRecord{{
			HookName:      "from-opener",
			Event:         hookspkg.HookPromptPostAssemble,
			Source:        hookspkg.HookSourceConfig,
			Mode:          hookspkg.HookModeSync,
			Outcome:       hookspkg.HookRunOutcomeApplied,
			DispatchDepth: 1,
			RecordedAt:    time.Date(2026, 4, 9, 19, 10, 0, 0, time.UTC),
		}},
	}
	sessionID := "sess-option-store"

	observer, err := New(testutil.Context(t),
		WithRegistry(h.registry),
		WithHomePaths(h.home),
		WithWorkspaceResolver(&fakeObserveWorkspaceResolver{}),
		WithHookCatalogSource(source),
		WithHookStoreOpener(func(_ context.Context, gotSessionID string, path string) (HookRunStore, error) {
			storeHandle.lastSessionID = gotSessionID
			storeHandle.lastPath = path
			return storeHandle, nil
		}),
		WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
	)
	if err != nil {
		t.Fatalf("New(custom hook options) error = %v", err)
	}

	path := observer.hookDBPath(sessionID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte("placeholder"), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}

	entries, err := observer.QueryHookCatalog(testutil.Context(t), hookspkg.CatalogFilter{AgentName: "coder"})
	if err != nil {
		t.Fatalf("QueryHookCatalog(custom) error = %v", err)
	}
	if got, want := len(entries), 1; got != want {
		t.Fatalf("len(entries) = %d, want %d", got, want)
	}

	records, err := observer.QueryHookRuns(testutil.Context(t), store.HookRunQuery{SessionID: sessionID})
	if err != nil {
		t.Fatalf("QueryHookRuns(custom opener) error = %v", err)
	}
	if got, want := len(records), 1; got != want {
		t.Fatalf("len(records) = %d, want %d", got, want)
	}
	if storeHandle.lastSessionID != sessionID || storeHandle.lastPath != path {
		t.Fatalf("custom opener saw session=%q path=%q", storeHandle.lastSessionID, storeHandle.lastPath)
	}

	written := hookspkg.HookRunRecord{
		HookName:      "written-via-opener",
		Event:         hookspkg.HookInputPreSubmit,
		Source:        hookspkg.HookSourceConfig,
		Mode:          hookspkg.HookModeSync,
		Outcome:       hookspkg.HookRunOutcomeApplied,
		DispatchDepth: 1,
		RecordedAt:    time.Date(2026, 4, 9, 19, 11, 0, 0, time.UTC),
	}
	if err := observer.WriteHookRecord(testutil.Context(t), sessionID, written); err != nil {
		t.Fatalf("WriteHookRecord(custom opener) error = %v", err)
	}
	if got, want := len(storeHandle.written), 1; got != want {
		t.Fatalf("len(written) = %d, want %d", got, want)
	}
	if storeHandle.written[0].HookName != "written-via-opener" {
		t.Fatalf("written[0].HookName = %q, want written-via-opener", storeHandle.written[0].HookName)
	}
	if !storeHandle.closed {
		t.Fatal("custom hook store Close() was not called")
	}
}

type stubHookCatalogSource struct {
	entries     []hookspkg.CatalogEntry
	lastFilter  hookspkg.CatalogFilter
	returnError error
}

func (s *stubHookCatalogSource) Catalog(filter hookspkg.CatalogFilter) ([]hookspkg.CatalogEntry, error) {
	s.lastFilter = filter
	if s.returnError != nil {
		return nil, s.returnError
	}
	return append([]hookspkg.CatalogEntry(nil), s.entries...), nil
}

type stubHookRunStore struct {
	records       []hookspkg.HookRunRecord
	written       []hookspkg.HookRunRecord
	lastSessionID string
	lastPath      string
	closed        bool
}

func (s *stubHookRunStore) RecordHookRun(_ context.Context, record hookspkg.HookRunRecord) error {
	s.written = append(s.written, record)
	return nil
}

func (s *stubHookRunStore) QueryHookRuns(_ context.Context, _ store.HookRunQuery) ([]hookspkg.HookRunRecord, error) {
	return append([]hookspkg.HookRunRecord(nil), s.records...), nil
}

func (s *stubHookRunStore) Close(context.Context) error {
	s.closed = true
	return nil
}

func openObserverHookSessionDB(t *testing.T, homePaths aghconfig.HomePaths, sessionID string) *sessiondb.SessionDB {
	t.Helper()

	db, err := sessiondb.OpenSessionDB(
		testutil.Context(t),
		sessionID,
		store.SessionDBFile(filepath.Join(homePaths.SessionsDir, sessionID)),
	)
	if err != nil {
		t.Fatalf("OpenSessionDB(%q) error = %v", sessionID, err)
	}
	return db
}

func closeObserverHookSessionDB(t *testing.T, db *sessiondb.SessionDB) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.Close(ctx); err != nil {
		t.Fatalf("SessionDB.Close() error = %v", err)
	}
}
