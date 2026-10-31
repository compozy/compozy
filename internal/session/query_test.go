package session

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/workspaceaccess"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/procutil"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/store/sessiondb"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/transcript"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	worktreepkg "github.com/compozy/compozy/internal/worktree"
)

func TestManagerListForWorkspace(t *testing.T) {
	t.Parallel()
	t.Run("Should preserve workspace results without reading unrelated metadata", func(t *testing.T) {
		t.Parallel()
		manager := newWorkspaceSessionReadFixture(t, 32, 4)
		all, err := manager.ListAll(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		want := slices.DeleteFunc(all, func(info *Info) bool { return info.WorkspaceID != "ws-read-00" })
		reads := 0
		manager.readSessionMeta = func(path string) (store.SessionMeta, error) {
			meta, err := store.ReadSessionMeta(path)
			if err == nil && meta.WorkspaceID != "ws-read-00" {
				t.Fatalf("read unrelated workspace %q", meta.WorkspaceID)
			}
			reads++
			return meta, err
		}
		got, err := manager.ListForWorkspace(t.Context(), "ws-read-00")
		if err != nil {
			t.Fatal(err)
		}
		if reads != 0 || !reflect.DeepEqual(got, want) {
			t.Fatalf("ListForWorkspace() = %#v, reads=%d; want %#v, 0 cached reads", got, reads, want)
		}
	})
	t.Run("Should preserve active unpublished sessions and skip deleted or unreadable histories", func(t *testing.T) {
		t.Parallel()
		manager := newWorkspaceSessionReadFixture(t, 4, 2)
		active := &Session{
			ID: "unpublished", WorkspaceID: "ws-read-00", State: StateStarting,
			CreatedAt: benchmarkSessionTime.Add(time.Hour),
		}
		manager.sessions[active.ID] = active
		published := &Session{
			ID: "sess-read-002", WorkspaceID: "ws-read-00", State: StateActive,
			CreatedAt: benchmarkSessionTime,
		}
		manager.sessions[published.ID] = published
		if err := os.Remove(
			store.SessionMetaFile(filepath.Join(manager.homePaths.SessionsDir, "sess-read-000")),
		); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(
			store.SessionMetaFile(filepath.Join(manager.homePaths.SessionsDir, "sess-read-002")),
			[]byte("{"),
			0o600,
		); err != nil {
			t.Fatal(err)
		}
		infos, err := manager.ListForWorkspace(t.Context(), "ws-read-00")
		if err != nil || len(infos) != 2 || infos[0].ID != published.ID || infos[0].State != StateActive ||
			infos[1].ID != active.ID || infos[1].State != StateStarting {
			t.Fatalf("ListForWorkspace() = %#v, %v; want both active sessions", infos, err)
		}
	})
	t.Run("Should reject an absent context workspace or catalog", func(t *testing.T) {
		t.Parallel()
		manager := &Manager{}
		var nilCtx context.Context
		for _, query := range []struct {
			ctx       context.Context
			workspace string
			want      string
		}{
			{nilCtx, "ws", "list context is required"},
			{t.Context(), " ", "workspace id is required"},
			{t.Context(), "ws", "session catalog is required"},
		} {
			t.Run("Should report that "+query.want, func(t *testing.T) {
				if _, err := manager.ListForWorkspace(query.ctx, query.workspace); err == nil ||
					!strings.Contains(err.Error(), query.want) {
					t.Fatalf("ListForWorkspace() error = %v, want %q", err, query.want)
				}
			})
		}
	})
}

func newWorkspaceSessionReadFixture(t testing.TB, count, workspaceCount int) *Manager {
	t.Helper()
	ctx := t.Context()
	database, err := openSessionTestGlobalDB(ctx, filepath.Join(t.TempDir(), store.GlobalDatabaseName))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := database.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	manager := &Manager{
		sessions: make(map[string]*Session), logger: slog.Default(),
		homePaths:      compozyconfig.HomePaths{SessionsDir: t.TempDir()},
		sessionCatalog: database, transcriptEpochStore: database, attentionStore: database,
		now: func() time.Time { return benchmarkSessionTime },
	}
	for index := range workspaceCount {
		id := fmt.Sprintf("ws-read-%02d", index)
		if err := database.InsertWorkspace(
			ctx,
			workspacepkg.Workspace{ID: id, Name: id, RootDir: t.TempDir()},
		); err != nil {
			t.Fatal(err)
		}
	}
	for index := range count {
		target := &Session{
			ID: fmt.Sprintf("sess-read-%03d", index), AgentName: "coder", Provider: "claude",
			ProfileID: store.DefaultProfileID, WorkspaceID: fmt.Sprintf("ws-read-%02d", index%workspaceCount),
			State: StateStopped, RuntimeStatus: store.SessionRuntimeReady, stopReason: store.StopCompleted,
			CreatedAt: benchmarkSessionTime.Add(time.Duration(index) * time.Minute), UpdatedAt: benchmarkSessionTime,
		}
		if err := database.RegisterSession(ctx, sessionCatalogInfoFromRuntime(target.Info())); err != nil {
			t.Fatal(err)
		}
		meta := target.Meta()
		path := filepath.Join(manager.homePaths.SessionsDir, target.ID)
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := store.WriteSessionMeta(store.SessionMetaFile(path), &meta); err != nil {
			t.Fatal(err)
		}
	}
	return manager
}

func TestManagerListAllRequiresContext(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	var nilCtx context.Context
	if _, err := h.manager.ListAll(nilCtx); err == nil {
		t.Fatal("ListAll(nil) error = nil, want non-nil")
	}
}

func TestManagerQueriesUseWorkspaceRegistration(t *testing.T) {
	t.Parallel()
	t.Run("Should list and inspect stopped sessions without resolving runtime resources", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		sess := createSession(t, h)
		if err := h.manager.Stop(t.Context(), sess.ID); err != nil {
			outcome, awaitErr := h.manager.AwaitStopped(t.Context(), sess.ID)
			t.Fatalf("Stop() error = %v; outcome = %+v; AwaitStopped() error = %v", err, outcome, awaitErr)
		}
		h.resolver.mu.Lock()
		h.resolver.resolveErr = errors.New("runtime resources are unavailable")
		h.resolver.resolveCalls = nil
		h.resolver.mu.Unlock()
		infos, err := h.manager.ListAll(t.Context())
		if err != nil {
			t.Fatalf("ListAll() error = %v", err)
		}
		if len(infos) != 1 || infos[0].Workspace != h.workspace {
			t.Fatalf("ListAll() = %#v, want stopped session with registered workspace root", infos)
		}
		info, err := h.manager.Status(t.Context(), sess.ID)
		if err != nil {
			t.Fatalf("Status() error = %v", err)
		}
		if info.Workspace != h.workspace {
			t.Fatalf("Status().Workspace = %q, want %q", info.Workspace, h.workspace)
		}
		h.resolver.mu.Lock()
		defer h.resolver.mu.Unlock()
		if len(h.resolver.resolveCalls) != 0 {
			t.Fatalf("runtime resolutions = %v, want none", h.resolver.resolveCalls)
		}
	})
}

func TestManagerListAllReturnsActiveWhenSessionsDirMissing(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	t.Cleanup(func() {
		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil &&
			!errors.Is(err, ErrSessionNotFound) {
			t.Errorf("Stop(%q) error = %v", session.ID, err)
		}
	})

	if err := os.RemoveAll(h.homePaths.SessionsDir); err != nil {
		t.Fatalf("RemoveAll(sessions dir) error = %v", err)
	}

	infos, err := h.manager.ListAll(testutil.Context(t))
	if err != nil {
		t.Fatalf("ListAll() error = %v", err)
	}
	if len(infos) != 1 {
		t.Fatalf("ListAll() = %d sessions, want 1", len(infos))
	}
	if got := infos[0].ID; got != session.ID {
		t.Fatalf("ListAll()[0].ID = %q, want %q", got, session.ID)
	}
	if got := infos[0].State; got != StateActive {
		t.Fatalf("ListAll()[0].State = %q, want %q", got, StateActive)
	}
}

func TestManagerListAllMergesActiveAndStoppedSessions(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	active := createSession(t, h)
	t.Cleanup(func() {
		if err := h.manager.Stop(testutil.Context(t), active.ID); err != nil &&
			!errors.Is(err, ErrSessionNotFound) {
			t.Errorf("Stop(%q) error = %v", active.ID, err)
		}
	})

	stopped, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName: "coder",
		Name:      "stopped",
		Workspace: h.workspaceID,
	})
	if err != nil {
		t.Fatalf("Create(session stopped) error = %v", err)
	}
	if err := h.manager.Stop(testutil.Context(t), stopped.ID); err != nil {
		t.Fatalf("Stop(stopped) error = %v", err)
	}

	orphanDir := filepath.Join(h.homePaths.SessionsDir, "orphan")
	if err := os.MkdirAll(orphanDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(orphan) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(h.homePaths.SessionsDir, "notes.txt"), []byte("skip me"), 0o644); err != nil {
		t.Fatalf("WriteFile(notes) error = %v", err)
	}
	if err := os.WriteFile(active.MetaPath(), []byte("{"), 0o644); err != nil {
		t.Fatalf("WriteFile(corrupt active meta) error = %v", err)
	}

	badStoppedDir := filepath.Join(h.homePaths.SessionsDir, "bad-stopped")
	if err := os.MkdirAll(badStoppedDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(bad-stopped) error = %v", err)
	}
	if err := os.WriteFile(store.SessionMetaFile(badStoppedDir), []byte("{"), 0o644); err != nil {
		t.Fatalf("WriteFile(corrupt stopped meta) error = %v", err)
	}

	infos, err := h.manager.ListAll(testutil.Context(t))
	if err != nil {
		t.Fatalf("ListAll() error = %v", err)
	}
	if len(infos) != 2 {
		t.Fatalf("ListAll() = %d sessions, want 2", len(infos))
	}

	if got := infos[0].ID; got != active.ID {
		t.Fatalf("ListAll()[0].ID = %q, want %q", got, active.ID)
	}
	if got := infos[0].State; got != StateActive {
		t.Fatalf("ListAll()[0].State = %q, want %q", got, StateActive)
	}
	if got := infos[1].ID; got != stopped.ID {
		t.Fatalf("ListAll()[1].ID = %q, want %q", got, stopped.ID)
	}
	if got := infos[1].State; got != StateStopped {
		t.Fatalf("ListAll()[1].State = %q, want %q", got, StateStopped)
	}
}

func TestManagerListPageOverlaysActiveAndBindsCursor(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should hide verdict-only judges before counting and preserve actionable system sessions",
		func(t *testing.T) {
			t.Parallel()
			catalog := &multiPagedRecordingSessionCatalog{recordingSessionCatalog: newRecordingSessionCatalog()}
			h := newHarness(t, WithSessionCatalog(catalog))
			ctx := testutil.Context(t)
			for _, kind := range []Type{SessionTypeUser, SessionTypeSystem} {
				opts := CreateOpts{AgentName: "coder", Workspace: h.workspaceID, Type: kind}
				if kind == SessionTypeSystem {
					opts.RuntimeMode = RuntimeModeVerdictOnly
				}
				created, err := h.manager.Create(ctx, opts)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { reportSessionStop(t, h, created.ID) })
			}
			stored := store.SessionInfo{
				ID:            "persisted-judge",
				ProfileID:     store.DefaultProfileID,
				WorkspaceID:   h.workspaceID,
				SessionType:   "system",
				VerdictOnly:   true,
				State:         "stopped",
				StopEscalated: true,
				CreatedAt: time.Date(
					2026,
					10,
					16,
					12,
					0,
					0,
					0,
					time.UTC,
				),
				UpdatedAt: time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC),
			}
			catalog.durable = []store.SessionInfo{stored}
			query := ListQuery{ReadScope: store.ReadScope{AllProfiles: true}, WorkspaceID: h.workspaceID, Limit: 1}
			page, err := h.manager.ListPage(ctx, query)
			if err != nil || page.Total != 1 || len(page.Sessions) != 1 || page.HasMore ||
				page.Sessions[0].Type != SessionTypeUser {
				t.Fatalf("default catalog = %#v, %v", page, err)
			}
			summary, err := h.manager.AttentionSummary(ctx, query.ReadScope)
			if err != nil || summary.NeedsYou != 0 || summary.Finished != 0 {
				t.Fatalf("attention = %#v, %v", summary, err)
			}
			actionable, err := h.manager.Create(
				ctx,
				CreateOpts{AgentName: "coder", Workspace: h.workspaceID, Type: SessionTypeSystem},
			)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { reportSessionStop(t, h, actionable.ID) })
			query.Limit = MaxListLimit
			visible, err := h.manager.ListPage(ctx, query)
			if err != nil || visible.Total != 2 {
				t.Fatalf("actionable system catalog = %#v, %v", visible, err)
			}
			query.SessionType = SessionTypeSystem
			query.Limit = MaxListLimit
			page, err = h.manager.ListPage(ctx, query)
			if err != nil || page.Total != 3 || len(page.Sessions) != 3 {
				t.Fatalf("audit catalog = %#v, %v", page, err)
			}
		},
	)

	for _, sortKey := range []string{ListSortCreated, ListSortName} {
		t.Run("Should keep "+sortKey+" pages stable across active and durable activity changes", func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			ctx := t.Context()
			if err := h.manager.Shutdown(ctx); err != nil {
				t.Fatal(err)
			}
			catalog := openManagerInputQueueStore(t)
			registerManagerInputQueueWorkspace(t, catalog, h)
			h.manager = newManagerWithHarness(t, h, WithSessionCatalog(catalog))
			cleanupTestManager(t, h.manager)
			active, err := h.manager.Create(
				ctx,
				CreateOpts{AgentName: "coder", Name: "Alpha", Workspace: h.workspaceID},
			)
			if err != nil {
				t.Fatal(err)
			}
			base := active.Info().CreatedAt
			for _, fixture := range []struct {
				id, name string
				offset   time.Duration
			}{
				{"zzz-alpha", "ALPHA", 0}, {"aaa-alpha", "alpha", 0},
				{"unicode-z", "İ", time.Second}, {"unicode-a", "i̇", time.Second},
				{"unnamed", "", 2 * time.Second}, {"zulu", "Zulu", 3 * time.Second},
				{"accent-z", "ÉCLAIR", 4 * time.Second}, {"accent-a", "éclair", 4 * time.Second},
			} {
				info := sessionCatalogInfoFromRuntime(active.Info())
				info.ID, info.Name, info.State = fixture.id, fixture.name, string(StateStopped)
				info.Lineage = nil
				info.CreatedAt = base.Add(fixture.offset)
				info.UpdatedAt = base.Add(-time.Hour)
				if err := catalog.RegisterSession(ctx, info); err != nil {
					t.Fatal(err)
				}
			}
			want := []string{
				"zzz-alpha",
				active.ID,
				"aaa-alpha",
				"unicode-z",
				"unicode-a",
				"unnamed",
				"zulu",
				"accent-z",
				"accent-a",
			}
			if sortKey == ListSortCreated {
				want = []string{
					"accent-z",
					"accent-a",
					"zulu",
					"unnamed",
					"unicode-z",
					"unicode-a",
					"zzz-alpha",
					active.ID,
					"aaa-alpha",
				}
			}
			query := ListQuery{
				ReadScope:   store.ReadScope{AllProfiles: true},
				WorkspaceID: h.workspaceID,
				Sort:        sortKey,
				Limit:       1,
			}
			seen := []string{}
			for index := range want {
				page, err := h.manager.ListPage(ctx, query)
				if err != nil || len(page.Sessions) != 1 || page.Total != len(want) {
					t.Fatalf("page %d = %#v, error=%v", index, page, err)
				}
				seen = append(seen, page.Sessions[0].ID)
				if page.HasMore != (index < len(want)-1) {
					t.Fatalf("page %d has_more=%v", index, page.HasMore)
				}
				query.Cursor = page.NextCursor
				active.mu.Lock()
				active.UpdatedAt = base.Add(time.Duration(index+1) * time.Hour)
				active.PendingClarifyCount = index + 1
				active.mu.Unlock()
				if err := catalog.UpdateSessionState(
					ctx,
					store.SessionStateUpdate{
						ID:        "aaa-alpha",
						State:     string(StateStopped),
						UpdatedAt: base.Add(48 * time.Hour),
					},
				); err != nil {
					t.Fatal(err)
				}
			}
			if !slices.Equal(seen, want) {
				t.Fatalf("stable %s walk = %v, want %v", sortKey, seen, want)
			}
			query.Cursor = ""
			query.WorkspaceID, query.AllWorkspaces = "", true
			query.SearchFields, query.Search = "title_agent", "ALPHA"
			query.Limit = 100
			page, err := h.manager.ListPage(ctx, query)
			if err != nil || page.Total != 3 || len(page.Sessions) != 3 || page.Sessions[0].ID != "zzz-alpha" {
				t.Fatalf("all-workspace search page = %#v, error=%v", page, err)
			}
		})
	}

	t.Run("Should accept pre-upgrade public cursors and fence visible search mode", func(t *testing.T) {
		t.Parallel()
		catalog := &multiPagedRecordingSessionCatalog{recordingSessionCatalog: newRecordingSessionCatalog()}
		h := newHarness(t, WithSessionCatalog(catalog))
		query := ListQuery{
			ReadScope:     store.ReadScope{AllProfiles: true},
			AllWorkspaces: true,
			Cursor:        "eyJ2IjoyLCJrIjoic2Vzc2lvbnMiLCJmIjoiWmxPVEdkRkIzaXEzckNZbzhOQTBZeHd2SGZJc3R5Yzd0NWVWTW5sOVdQdyIsInAiOnsiYXR0ZW50aW9uX3JhbmsiOjAsInByaW1hcnlfYXQiOiIyMDI2LTA5LTI5VDEyOjAwOjAwWiIsInNlY29uZGFyeV9hdCI6IjIwMjYtMDktMjlUMTI6MDA6MDBaIiwiY3JlYXRlZF9hdCI6IjIwMjYtMDktMjlUMTI6MDA6MDBaIiwiaWQiOiJsZWdhY3ktc2Vzc2lvbiJ9fQ",
		}
		if _, err := h.manager.ListPage(testutil.Context(t), query); err != nil {
			t.Fatalf("legacy public cursor error=%v", err)
		}
		query.SearchFields = "title_agent"
		if _, err := h.manager.ListPage(testutil.Context(t), query); !errors.Is(err, ErrListCursorInvalid) {
			t.Fatalf("changed search mode error=%v, want invalid cursor", err)
		}
	})

	t.Run("Should archive only stopped sessions and keep archived sessions directly readable", func(t *testing.T) {
		t.Parallel()

		catalog := &pagedRecordingSessionCatalog{recordingSessionCatalog: newRecordingSessionCatalog()}
		h := newHarness(t, WithSessionCatalog(catalog))
		ctx := testutil.Context(t)
		const attachmentContents = "preserve-on-archive"
		stopped, err := h.manager.Create(ctx, CreateOpts{
			AgentName: "coder",
			Name:      "archivable",
			Workspace: h.workspaceID,
			Type:      SessionTypeUser,
		})
		if err != nil {
			t.Fatalf("Create(stopped) error = %v", err)
		}
		if err := h.manager.Stop(ctx, stopped.ID); err != nil {
			t.Fatalf("Stop(stopped) error = %v", err)
		}
		attachmentPath := writeSessionAttachmentFixture(t, h, stopped.ID, attachmentContents)
		catalog.durable = sessionCatalogInfoFromRuntime(stopped.Info())

		archived, err := h.manager.Archive(ctx, h.workspaceID, stopped.ID)
		if err != nil {
			t.Fatalf("Archive() error = %v", err)
		}
		if archived.ArchivedAt == nil || archived.State != StateStopped {
			t.Fatalf("Archive() = %#v, want archived stopped session", archived)
		}
		payload, err := os.ReadFile(attachmentPath)
		if err != nil {
			t.Fatalf("ReadFile(attachment after archive) error = %v", err)
		}
		if got := string(payload); got != attachmentContents {
			t.Fatalf("attachment after archive = %q, want %q", got, attachmentContents)
		}
		defaultPage, err := h.manager.ListPage(
			ctx,
			ListQuery{ReadScope: store.ReadScope{AllProfiles: true}, WorkspaceID: h.workspaceID},
		)
		if err != nil {
			t.Fatalf("ListPage(default) error = %v", err)
		}
		if defaultPage.Total != 0 || len(defaultPage.Sessions) != 0 {
			t.Fatalf("ListPage(default) = %#v, want archived session excluded", defaultPage)
		}
		archivedPage, err := h.manager.ListPage(ctx, ListQuery{ReadScope: store.ReadScope{AllProfiles: true},
			WorkspaceID: h.workspaceID,
			Archive:     ArchiveOnly,
		})
		if err != nil {
			t.Fatalf("ListPage(archived) error = %v", err)
		}
		if archivedPage.Total != 1 || len(archivedPage.Sessions) != 1 ||
			archivedPage.Sessions[0].ArchivedAt == nil {
			t.Fatalf("ListPage(archived) = %#v, want archived session", archivedPage)
		}
		status, err := h.manager.Status(ctx, stopped.ID)
		if err != nil || status.ArchivedAt == nil {
			t.Fatalf("Status(archived) = %#v, %v, want direct-readable archive marker", status, err)
		}
		if _, err := h.manager.Resume(ctx, stopped.ID); !errors.Is(err, ErrSessionArchived) {
			t.Fatalf("Resume(archived) error = %v, want ErrSessionArchived", err)
		}
		restored, err := h.manager.Unarchive(ctx, h.workspaceID, stopped.ID)
		if err != nil || restored.ArchivedAt != nil {
			t.Fatalf("Unarchive() = %#v, %v, want restored session", restored, err)
		}

		active := createSession(t, h)
		t.Cleanup(func() {
			if stopErr := h.manager.Stop(testutil.Context(t), active.ID); stopErr != nil &&
				!errors.Is(stopErr, ErrSessionNotFound) {
				t.Errorf("Stop(active cleanup) error = %v", stopErr)
			}
		})
		if _, err := h.manager.Archive(ctx, h.workspaceID, active.ID); !errors.Is(
			err,
			ErrSessionArchiveRequiresStopped,
		) {
			t.Fatalf("Archive(active) error = %v, want ErrSessionArchiveRequiresStopped", err)
		}
	})

	t.Run("Should stop and archive a live session in one archive-with-stop call", func(t *testing.T) {
		t.Parallel()

		catalog := &pagedRecordingSessionCatalog{recordingSessionCatalog: newRecordingSessionCatalog()}
		h := newHarness(t, WithSessionCatalog(catalog))
		ctx := testutil.Context(t)
		active := createSession(t, h)

		archived, err := h.manager.ArchiveWithStop(ctx, h.workspaceID, active.ID)
		if err != nil {
			t.Fatalf("ArchiveWithStop(active) error = %v", err)
		}
		if archived.ArchivedAt == nil || archived.State != StateStopped {
			t.Fatalf("ArchiveWithStop(active) = %#v, want stopped archived session", archived)
		}
		status, err := h.manager.Status(ctx, active.ID)
		if err != nil || status.State != StateStopped || status.ArchivedAt == nil {
			t.Fatalf("Status(after archive-with-stop) = %#v, %v, want stopped archived session", status, err)
		}
		again, err := h.manager.ArchiveWithStop(ctx, h.workspaceID, active.ID)
		if err != nil || again.ArchivedAt == nil {
			t.Fatalf("ArchiveWithStop(archived) = %#v, %v, want idempotent archive", again, err)
		}
	})

	t.Run("Should stop live family members the parent stop leaves running before archiving", func(t *testing.T) {
		t.Parallel()

		paged := &pagedRecordingSessionCatalog{recordingSessionCatalog: newRecordingSessionCatalog()}
		catalog := &familyArchiveCatalog{pagedRecordingSessionCatalog: paged, members: map[string][]string{}}
		h := newHarness(t, WithSessionCatalog(catalog))
		ctx := testutil.Context(t)
		parent := createSession(t, h)
		// A completed subagent keeps its session live until the reaper stops it;
		// the parent's stop cascade skips it.
		child := createSession(t, h)
		catalog.setMembers(parent.ID, child.ID)

		archived, err := h.manager.ArchiveWithStop(ctx, h.workspaceID, parent.ID)
		if err != nil {
			t.Fatalf("ArchiveWithStop(parent with live member) error = %v", err)
		}
		if archived.ArchivedAt == nil || archived.State != StateStopped {
			t.Fatalf("ArchiveWithStop(parent) = %#v, want stopped archived parent", archived)
		}
		status, err := h.manager.Status(ctx, child.ID)
		if err != nil || status.State != StateStopped {
			t.Fatalf("Status(family member) = %#v, %v, want stopped", status, err)
		}
	})

	t.Run("Should leave the session unarchived when the stop before archiving fails", func(t *testing.T) {
		t.Parallel()

		catalog := &pagedRecordingSessionCatalog{recordingSessionCatalog: newRecordingSessionCatalog()}
		h := newHarness(t, WithSessionCatalog(catalog))
		ctx := testutil.Context(t)
		// A durable live row without a runtime the manager can stop.
		catalog.sessions["sess-orphan-live"] = store.SessionInfo{
			ID: "sess-orphan-live", WorkspaceID: h.workspaceID, AgentName: "coder",
			State: string(StateActive), CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
		}

		_, err := h.manager.ArchiveWithStop(ctx, h.workspaceID, "sess-orphan-live")
		if !errors.Is(err, ErrSessionArchiveStopFailed) {
			t.Fatalf("ArchiveWithStop(unstoppable) error = %v, want ErrSessionArchiveStopFailed", err)
		}
		if row := catalog.sessions["sess-orphan-live"]; row.ArchivedAt != nil {
			t.Fatalf("catalog row after failed stop = %#v, want unarchived", row)
		}
	})

	t.Run("Should treat onboarding as an ordinary user agent name", func(t *testing.T) {
		t.Parallel()

		catalog := &pagedRecordingSessionCatalog{recordingSessionCatalog: newRecordingSessionCatalog()}
		h := newHarness(t, WithSessionCatalog(catalog))
		ctx := testutil.Context(t)
		resolved, err := h.resolver.Resolve(ctx, h.workspaceID)
		if err != nil {
			t.Fatalf("Resolve(workspace) error = %v", err)
		}
		resolved.Agents = append(resolved.Agents, compozyconfig.AgentDef{
			Name:     "onboarding",
			Provider: "claude",
			Prompt:   "Internal onboarding.",
		})
		h.resolver.upsert(&resolved)

		onboarding, err := h.manager.Create(ctx, CreateOpts{
			AgentName: "onboarding",
			Workspace: h.workspaceID,
			Type:      SessionTypeUser,
		})
		if err != nil {
			t.Fatalf("Create(onboarding) error = %v", err)
		}
		user, err := h.manager.Create(ctx, CreateOpts{
			AgentName: "coder",
			Workspace: h.workspaceID,
			Type:      SessionTypeUser,
		})
		if err != nil {
			t.Fatalf("Create(user) error = %v", err)
		}
		for _, created := range []*Session{onboarding, user} {
			t.Cleanup(func() {
				if stopErr := h.manager.Stop(testutil.Context(t), created.ID); stopErr != nil &&
					!errors.Is(stopErr, ErrSessionNotFound) {
					t.Errorf("Stop(%q) error = %v", created.ID, stopErr)
				}
			})
		}

		if got, want := onboarding.Info().Type, SessionTypeUser; got != want {
			t.Errorf("onboarding session type = %q, want %q", got, want)
		}
		if got, want := readMeta(t, onboarding.MetaPath()).SessionType, string(SessionTypeUser); got != want {
			t.Errorf("onboarding metadata session type = %q, want %q", got, want)
		}
		if got, want := user.Info().Type, SessionTypeUser; got != want {
			t.Errorf("user session type = %q, want %q", got, want)
		}

		userPage, err := h.manager.ListPage(ctx, ListQuery{ReadScope: store.ReadScope{AllProfiles: true},
			AllWorkspaces: true,
			State:         string(StateActive),
			SessionType:   SessionTypeUser,
			Sort:          ListSortRecent,
		})
		if err != nil {
			t.Fatalf("ListPage(user) error = %v", err)
		}
		if userPage.Total != 2 || len(userPage.Sessions) != 2 {
			t.Errorf("ListPage(user) = %#v, want onboarding and coder sessions", userPage)
		}

		allPage, err := h.manager.ListPage(ctx, ListQuery{ReadScope: store.ReadScope{AllProfiles: true},
			AllWorkspaces: true,
			State:         string(StateActive),
			Sort:          ListSortRecent,
		})
		if err != nil {
			t.Fatalf("ListPage(all) error = %v", err)
		}
		allIDs := make([]string, 0, len(allPage.Sessions))
		for _, info := range allPage.Sessions {
			allIDs = append(allIDs, info.ID)
		}
		if allPage.Total != 2 || !slices.Contains(allIDs, onboarding.ID) || !slices.Contains(allIDs, user.ID) {
			t.Errorf("ListPage(all) = %#v, want onboarding %q and user %q", allPage, onboarding.ID, user.ID)
		}
	})

	t.Run("Should page the active and durable union from the indexed projection", func(t *testing.T) {
		t.Parallel()

		catalog := &pagedRecordingSessionCatalog{recordingSessionCatalog: newRecordingSessionCatalog()}
		h := newHarness(t, WithSessionCatalog(catalog))
		active, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Name:      "catalog active",
			Workspace: h.workspaceID,
			Type:      SessionTypeUser,
		})
		if err != nil {
			t.Fatalf("Create(active) error = %v", err)
		}
		t.Cleanup(func() {
			if stopErr := h.manager.Stop(testutil.Context(t), active.ID); stopErr != nil &&
				!errors.Is(stopErr, ErrSessionNotFound) {
				t.Errorf("Stop(active cleanup) error = %v", stopErr)
			}
		})
		stopped, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Name:      "catalog stopped",
			Workspace: h.workspaceID,
		})
		if err != nil {
			t.Fatalf("Create(stopped) error = %v", err)
		}
		if err := h.manager.Stop(testutil.Context(t), stopped.ID); err != nil {
			t.Fatalf("Stop(stopped) error = %v", err)
		}
		internal, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Name:      "catalog internal",
			Workspace: h.workspaceID,
			Type:      SessionTypeSystem,
		})
		if err != nil {
			t.Fatalf("Create(internal) error = %v", err)
		}
		t.Cleanup(func() {
			if stopErr := h.manager.Stop(testutil.Context(t), internal.ID); stopErr != nil &&
				!errors.Is(stopErr, ErrSessionNotFound) {
				t.Errorf("Stop(internal cleanup) error = %v", stopErr)
			}
		})
		stoppedInfo := stopped.Info()
		stoppedInfo.ACPOptions = []acp.SessionConfigOptionSelection{{ID: "mode", ValueID: "plan"}}
		stoppedInfo.SelectedRuntime = &RuntimeSelection{
			Provider:        "cursor",
			Model:           "grok-4.6",
			ReasoningEffort: "xhigh",
			Speed:           speedpkg.SpeedFast,
			ACPOptions: []acp.SessionConfigOptionSelection{{
				ID: "thinking", BoolValue: new(true),
			}},
		}
		stoppedInfo.RuntimeSelectionRevision = 3
		catalog.durable = sessionCatalogInfoFromRuntime(stoppedInfo)
		metaReads := 0
		h.manager.readSessionMeta = func(path string) (store.SessionMeta, error) {
			metaReads++
			return store.ReadSessionMeta(path)
		}
		h.resolver.mu.Lock()
		resolverCallsBefore := len(h.resolver.resolveCalls)
		h.resolver.mu.Unlock()

		query := ListQuery{ReadScope: store.ReadScope{AllProfiles: true},
			WorkspaceID: h.workspaceID,
			SessionType: SessionTypeUser,
			AgentName:   "coder",
			Search:      "CATALOG",
			Sort:        ListSortRecent,
			Limit:       1,
		}
		first, err := h.manager.ListPage(testutil.Context(t), query)
		if err != nil {
			t.Fatalf("ListPage(first) error = %v", err)
		}
		if first.Total != 2 || len(first.Sessions) != 1 || !first.HasMore || first.NextCursor == "" {
			t.Fatalf("ListPage(first) = %#v, want one of two sessions with next cursor", first)
		}
		if !slices.Contains(catalog.lastQuery.ExcludeIDs, active.ID) {
			t.Fatalf("PageSessions().ExcludeIDs = %#v, want active id %q", catalog.lastQuery.ExcludeIDs, active.ID)
		}
		if catalog.lastQuery.SessionType != string(SessionTypeUser) {
			t.Fatalf("PageSessions().SessionType = %q, want %q", catalog.lastQuery.SessionType, SessionTypeUser)
		}

		query.Cursor = first.NextCursor
		second, err := h.manager.ListPage(testutil.Context(t), query)
		if err != nil {
			t.Fatalf("ListPage(second) error = %v", err)
		}
		if second.Total != 2 || len(second.Sessions) != 1 || second.HasMore {
			t.Fatalf("ListPage(second) = %#v, want final one-of-two page", second)
		}
		ids := []string{first.Sessions[0].ID, second.Sessions[0].ID}
		if !slices.Contains(ids, active.ID) || !slices.Contains(ids, stopped.ID) {
			t.Fatalf("paged ids = %#v, want active %q and durable %q", ids, active.ID, stopped.ID)
		}
		for _, page := range []ListPage{first, second} {
			for _, info := range page.Sessions {
				if info.ID != stopped.ID {
					continue
				}
				if info.Workspace != "" {
					t.Fatalf("projected stopped info = %#v, want no meta-only workspace path", info)
				}
				if len(info.ACPOptions) != 1 || info.ACPOptions[0].ValueID != "plan" {
					t.Fatalf("projected stopped ACP options = %#v, want mode=plan", info.ACPOptions)
				}
				if info.SelectedRuntime == nil || info.SelectedRuntime.Provider != "cursor" ||
					info.SelectedRuntime.Model != "grok-4.6" ||
					info.SelectedRuntime.ReasoningEffort != "xhigh" ||
					info.SelectedRuntime.Speed != speedpkg.SpeedFast || info.RuntimeSelectionRevision != 3 {
					t.Fatalf(
						"projected stopped selection = %#v revision %d, want Grok 4.6 xhigh fast at revision 3",
						info.SelectedRuntime,
						info.RuntimeSelectionRevision,
					)
				}
				if len(info.SelectedRuntime.ACPOptions) != 1 ||
					info.SelectedRuntime.ACPOptions[0].BoolValue == nil ||
					!*info.SelectedRuntime.ACPOptions[0].BoolValue {
					t.Fatalf("projected stopped selected ACP options = %#v, want thinking=true",
						info.SelectedRuntime.ACPOptions)
				}
			}
		}
		if metaReads != 0 {
			t.Fatalf("session metadata reads = %d, want zero for catalog pages", metaReads)
		}
		h.resolver.mu.Lock()
		resolverCallsAfter := len(h.resolver.resolveCalls)
		h.resolver.mu.Unlock()
		if resolverCallsAfter != resolverCallsBefore {
			t.Fatalf(
				"workspace resolver calls = %d after catalog pages, want unchanged %d",
				resolverCallsAfter,
				resolverCallsBefore,
			)
		}

		callsBeforeMismatch := catalog.calls
		query.Search = "different"
		if _, err := h.manager.ListPage(testutil.Context(t), query); !errors.Is(err, ErrListCursorInvalid) {
			t.Fatalf("ListPage(query mismatch) error = %v, want ErrListCursorInvalid", err)
		}
		query.Search = "catalog"
		query.WorkspaceID = "ws-foreign"
		if _, err := h.manager.ListPage(testutil.Context(t), query); !errors.Is(err, ErrListCursorInvalid) {
			t.Fatalf("ListPage(workspace mismatch) error = %v, want ErrListCursorInvalid", err)
		}
		query.WorkspaceID = h.workspaceID
		query.WorktreeID = "wt-foreign"
		if _, err := h.manager.ListPage(testutil.Context(t), query); !errors.Is(err, ErrListCursorInvalid) {
			t.Fatalf("ListPage(worktree mismatch) error = %v, want ErrListCursorInvalid", err)
		}
		query.WorktreeID = ""
		query.SessionType = SessionTypeSystem
		if _, err := h.manager.ListPage(testutil.Context(t), query); !errors.Is(err, ErrListCursorInvalid) {
			t.Fatalf("ListPage(type mismatch) error = %v, want ErrListCursorInvalid", err)
		}
		query.SessionType = SessionTypeUser
		query.Cursor = "not-an-opaque-cursor"
		if _, err := h.manager.ListPage(testutil.Context(t), query); !errors.Is(err, ErrListCursorInvalid) {
			t.Fatalf("ListPage(malformed cursor) error = %v, want ErrListCursorInvalid", err)
		}
		if catalog.calls != callsBeforeMismatch {
			t.Fatalf("PageSessions calls after mismatched cursors = %d, want %d", catalog.calls, callsBeforeMismatch)
		}
	})

	t.Run("Should restore active detail health and resumable catalog truth after attach expiry", func(t *testing.T) {
		t.Parallel()

		clock := newSessionHealthTestClock(time.Date(2026, 7, 11, 14, 0, 0, 0, time.UTC))
		catalog := &pagedRecordingSessionCatalog{recordingSessionCatalog: newRecordingSessionCatalog()}
		h := newHarness(t, WithNow(clock.Now), WithSessionCatalog(catalog))
		active := createSession(t, h)
		t.Cleanup(func() {
			if stopErr := h.manager.Stop(testutil.Context(t), active.ID); stopErr != nil &&
				!errors.Is(stopErr, ErrSessionNotFound) {
				t.Errorf("Stop(active cleanup) error = %v", stopErr)
			}
		})
		attach, err := h.manager.AttachSession(testutil.Context(t), store.SessionAttachRequest{
			SessionID:  active.ID,
			AttachedTo: "uds:operator",
			Now:        clock.Now(),
			TTL:        time.Minute,
		})
		if err != nil {
			t.Fatalf("AttachSession() error = %v", err)
		}

		locked, err := h.manager.ListPage(testutil.Context(t), ListQuery{ReadScope: store.ReadScope{AllProfiles: true},
			WorkspaceID: h.workspaceID,
			Resumable:   true,
		})
		if err != nil {
			t.Fatalf("ListPage(locked resumable) error = %v", err)
		}
		if locked.Total != 0 || len(locked.Sessions) != 0 {
			t.Fatalf("ListPage(locked resumable) = %#v, want empty", locked)
		}

		clock.Set(attach.AttachExpiresAt.Add(time.Second))
		status, err := h.manager.Status(testutil.Context(t), active.ID)
		if err != nil {
			t.Fatalf("Status(after expiry) error = %v", err)
		}
		if status.AttachedTo != "" || status.AttachExpiresAt != nil || !AttachableForInfo(status, clock.Now()) {
			t.Fatalf("Status(after expiry) = %#v, want normalized attachable session", status)
		}
		resumable, err := h.manager.ListPage(
			testutil.Context(t),
			ListQuery{ReadScope: store.ReadScope{AllProfiles: true},
				WorkspaceID: h.workspaceID,
				Resumable:   true,
			},
		)
		if err != nil {
			t.Fatalf("ListPage(after expiry) error = %v", err)
		}
		if resumable.Total != 1 || len(resumable.Sessions) != 1 || resumable.Sessions[0].ID != active.ID ||
			resumable.Sessions[0].AttachedTo != "" || resumable.Sessions[0].AttachExpiresAt != nil {
			t.Fatalf("ListPage(after expiry) = %#v, want active session unlocked", resumable)
		}
		healthByID, err := h.manager.SessionHealthForPage(testutil.Context(t), resumable.Sessions)
		if err != nil {
			t.Fatalf("SessionHealthForPage(after expiry) error = %v", err)
		}
		if !healthByID[active.ID].Attachable {
			t.Fatalf("SessionHealthForPage(after expiry) = %#v, want attachable", healthByID[active.ID])
		}
	})
}

func TestManagerAggregateSessionsByAgent(t *testing.T) {
	t.Parallel()

	t.Run("Should overlay live sessions onto exact durable workspace metrics", func(t *testing.T) {
		t.Parallel()

		baseAt := time.Date(2026, 7, 12, 14, 0, 0, 0, time.UTC)
		clock := newSessionHealthTestClock(baseAt)
		catalog := &pagedRecordingSessionCatalog{recordingSessionCatalog: newRecordingSessionCatalog()}
		h := newHarness(t, WithNow(clock.Now), WithSessionCatalog(catalog))
		active, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Name:      "live coder",
			Workspace: h.workspaceID,
		})
		if err != nil {
			t.Fatalf("Create(active) error = %v", err)
		}
		t.Cleanup(func() {
			if stopErr := h.manager.Stop(testutil.Context(t), active.ID); stopErr != nil &&
				!errors.Is(stopErr, ErrSessionNotFound) {
				t.Errorf("Stop(active cleanup) error = %v", stopErr)
			}
		})
		lastActivityAt := baseAt.Add(20 * time.Second)
		catalog.agentMetrics = []store.SessionAgentMetrics{{
			AgentName:      "coder",
			Total:          1,
			Failed:         1,
			RuntimeSeconds: 12,
			LastActivityAt: lastActivityAt,
		}}
		clock.Set(baseAt.Add(30 * time.Second))

		metrics, err := h.manager.AggregateSessionsByAgent(
			testutil.Context(t),
			store.ReadScope{AllProfiles: true},
			h.workspaceID,
		)
		if err != nil {
			t.Fatalf("AggregateSessionsByAgent() error = %v", err)
		}
		coder := metrics["coder"]
		if coder.Total != 2 || coder.Active != 1 || coder.Failed != 1 ||
			coder.RuntimeSeconds != 42 || !coder.LastActivityAt.Equal(lastActivityAt) {
			t.Fatalf(
				"AggregateSessionsByAgent() = %#v, want coder total=2 active=1 failed=1 runtime=42 last=%s",
				metrics,
				lastActivityAt,
			)
		}
		if catalog.lastAgentMetricsQuery.WorkspaceID != h.workspaceID ||
			!slices.Contains(catalog.lastAgentMetricsQuery.ExcludeIDs, active.ID) ||
			!slices.Contains(catalog.lastAgentMetricsQuery.ExcludeSpawnRoles, SpawnRoleAutoTitle) {
			t.Fatalf(
				"AggregateSessionsByAgent() query = %#v, want workspace and live/internal exclusions",
				catalog.lastAgentMetricsQuery,
			)
		}
		globalMetrics, err := h.manager.AggregateSessionsByAgent(t.Context(), store.ReadScope{AllProfiles: true}, "")
		if err != nil || globalMetrics["coder"] != coder || catalog.lastAgentMetricsQuery.WorkspaceID != "" {
			t.Fatalf("Global live overlay = %#v, error %v, want the same exact aggregate", globalMetrics, err)
		}
	})
}

func TestSessionMatchesListQuery(t *testing.T) {
	t.Parallel()

	base := &Info{
		ID:          "sess-visible",
		Name:        "Review launch",
		AgentName:   "coder",
		Provider:    "codex",
		WorkspaceID: "ws-alpha",
		WorktreeID:  "wt-alpha",
		Type:        SessionTypeUser,
		State:       StateActive,
	}
	now := time.Date(2026, 7, 10, 15, 0, 0, 0, time.UTC)

	t.Run("Should match visible Unicode casing for active catalog rows", func(t *testing.T) {
		t.Parallel()
		for _, match := range []struct{ title, agent, query string }{
			{title: "ΟΣ", agent: "coder", query: "ος"},
			{title: "Visible", agent: "İ", query: "i̇"},
			{title: "ος", agent: "coder", query: "ΟΣ"},
			{title: "Visible", agent: "i̇", query: "İ"},
		} {
			info := *base
			info.Name = match.title
			info.AgentName = match.agent
			query, err := normalizeListQuery(
				ListQuery{
					ReadScope:     store.ReadScope{AllProfiles: true},
					AllWorkspaces: true,
					SearchFields:  "title_agent",
					Search:        match.query,
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			if !sessionMatchesListQuery(&info, query, now) {
				t.Fatalf("visible title=%q agent=%q query=%q did not match", match.title, match.agent, match.query)
			}
		}
	})

	t.Run("Should apply exact workspace state agent and literal search filters", func(t *testing.T) {
		t.Parallel()

		if !sessionMatchesListQuery(base, ListQuery{ReadScope: store.ReadScope{AllProfiles: true},
			WorkspaceID: "ws-alpha",
			WorktreeID:  "wt-alpha",
			State:       "active",
			SessionType: SessionTypeUser,
			AgentName:   "coder",
			Search:      "review",
		}, now) {
			t.Fatal("sessionMatchesListQuery() = false, want exact filters to match")
		}
		for _, query := range []ListQuery{
			{WorkspaceID: "ws-foreign"},
			{WorktreeID: "wt-foreign"},
			{State: "stopped"},
			{SessionType: SessionTypeSystem},
			{AgentName: "reviewer"},
			{Search: "missing"},
		} {
			query.ReadScope = store.ReadScope{AllProfiles: true}
			if sessionMatchesListQuery(base, query, now) {
				t.Fatalf("sessionMatchesListQuery(%#v) = true, want false", query)
			}
		}
	})

	t.Run("Should hide daemon-owned internal sessions before the page cut", func(t *testing.T) {
		t.Parallel()

		for _, role := range []string{SpawnRoleAutoTitle} {
			internal := *base
			internal.Lineage = &store.SessionLineage{SpawnRole: role}
			if sessionMatchesListQuery(&internal, ListQuery{ReadScope: store.ReadScope{AllProfiles: true}}, now) {
				t.Fatalf("sessionMatchesListQuery(%s) = true, want false", role)
			}
		}
	})

	t.Run("Should apply parent and root lineage filters", func(t *testing.T) {
		t.Parallel()

		child := *base
		child.ID = "sess-child"
		child.Lineage = &store.SessionLineage{
			ParentSessionID: "sess-root",
			RootSessionID:   "sess-root",
			SpawnDepth:      1,
		}
		if !sessionMatchesListQuery(
			&child,
			ListQuery{ReadScope: store.ReadScope{AllProfiles: true}, ParentSessionID: "sess-root"},
			now,
		) {
			t.Fatal("sessionMatchesListQuery(parent) = false, want provenance child match")
		}
		if !sessionMatchesListQuery(
			&child,
			ListQuery{ReadScope: store.ReadScope{AllProfiles: true}, RootSessionID: "sess-root"},
			now,
		) {
			t.Fatal("sessionMatchesListQuery(root) = false, want descendant match")
		}
		if sessionMatchesListQuery(
			&child,
			ListQuery{ReadScope: store.ReadScope{AllProfiles: true}, ParentSessionID: "sess-other"},
			now,
		) {
			t.Fatal("sessionMatchesListQuery(foreign parent) = true, want false")
		}
		if !sessionMatchesListQuery(
			base,
			ListQuery{ReadScope: store.ReadScope{AllProfiles: true}, RootSessionID: base.ID},
			now,
		) {
			t.Fatal("sessionMatchesListQuery(own root) = false, want root session to match its own tree")
		}
		if sessionMatchesListQuery(
			base,
			ListQuery{ReadScope: store.ReadScope{AllProfiles: true}, ParentSessionID: "sess-root"},
			now,
		) {
			t.Fatal("sessionMatchesListQuery(rootless parent) = true, want false")
		}
	})

	t.Run("Should apply resumable eligibility before the page cut", func(t *testing.T) {
		t.Parallel()

		if !sessionMatchesListQuery(
			base,
			ListQuery{ReadScope: store.ReadScope{AllProfiles: true}, Resumable: true},
			now,
		) {
			t.Fatal("sessionMatchesListQuery(resumable) = false, want attachable active session")
		}
		locked := *base
		locked.AttachedTo = "uds:123"
		expiresAt := now.Add(time.Hour)
		locked.AttachExpiresAt = &expiresAt
		if sessionMatchesListQuery(
			&locked,
			ListQuery{ReadScope: store.ReadScope{AllProfiles: true}, Resumable: true},
			now,
		) {
			t.Fatal("sessionMatchesListQuery(locked resumable) = true, want false")
		}
	})
}

func TestNormalizeListQuery(t *testing.T) {
	t.Parallel()
	t.Run("Should bind cursors to canonical board columns and child visibility", func(t *testing.T) {
		t.Parallel()
		base := ListQuery{
			ReadScope:     store.ReadScope{AllProfiles: true},
			AllWorkspaces: true,
			Columns:       []string{"done", "in_review"},
		}
		query, err := normalizeListQuery(base)
		if err != nil {
			t.Fatal(err)
		}
		fingerprint, err := sessionListFingerprintForQuery(query)
		if err != nil {
			t.Fatal(err)
		}
		now := time.Now().UTC()
		cursor, err := encodeSessionListCursor(
			fingerprint,
			store.SessionCatalogPosition{ID: "cursor-row", PrimaryAt: now, SecondaryAt: now, CreatedAt: now},
		)
		if err != nil {
			t.Fatal(err)
		}
		for _, tc := range []struct {
			name    string
			columns []string
			hide    bool
			reject  bool
		}{
			{"same columns reordered", []string{"in_review", "done"}, false, false},
			{"different column", []string{"working"}, false, true},
			{"children excluded", base.Columns, true, true},
		} {
			changed := base
			changed.Columns, changed.HideChildren = tc.columns, tc.hide
			changed, err = normalizeListQuery(changed)
			if err != nil {
				t.Fatal(err)
			}
			key, err := sessionListFingerprintForQuery(changed)
			if err != nil {
				t.Fatal(err)
			}
			_, err = decodeSessionListCursor(cursor, key, changed.Sort)
			if tc.reject != errors.Is(err, ErrListCursorInvalid) {
				t.Fatalf("%s cursor: %v", tc.name, err)
			}
		}
		base.Columns = []string{"blocked"}
		if _, err := normalizeListQuery(base); !errors.Is(err, ErrListQueryInvalid) {
			t.Fatalf("invalid column: %v", err)
		}
	})

	t.Run("Should normalize defaults and case-insensitive search", func(t *testing.T) {
		t.Parallel()

		query, err := normalizeListQuery(
			ListQuery{ReadScope: store.ReadScope{AllProfiles: true}, AllWorkspaces: true, Search: "  Review  "},
		)
		if err != nil {
			t.Fatalf("normalizeListQuery() error = %v", err)
		}
		if query.Search != "review" || query.Sort != ListSortRecent || query.Limit != DefaultListLimit {
			t.Fatalf("normalizeListQuery() = %#v, want normalized search/sort/limit", query)
		}
	})

	for _, testCase := range []struct {
		name  string
		query ListQuery
		want  string
	}{
		{name: "state", query: ListQuery{ReadScope: store.ReadScope{AllProfiles: true}, AllWorkspaces: true, State: "unknown"}, want: "unsupported state"},
		{name: "type", query: ListQuery{ReadScope: store.ReadScope{AllProfiles: true}, AllWorkspaces: true, SessionType: "unknown"}, want: "unsupported type"},
		{name: "sort", query: ListQuery{ReadScope: store.ReadScope{AllProfiles: true}, AllWorkspaces: true, Sort: "oldest"}, want: "unsupported sort"},
		{name: "negative limit", query: ListQuery{ReadScope: store.ReadScope{AllProfiles: true}, AllWorkspaces: true, Limit: -1}, want: "limit must be between"},
		{name: "oversized limit", query: ListQuery{ReadScope: store.ReadScope{AllProfiles: true}, AllWorkspaces: true, Limit: MaxListLimit + 1}, want: "limit must be between"},
		{name: "missing scope", query: ListQuery{ReadScope: store.ReadScope{AllProfiles: true}}, want: "choose exactly one workspace or all workspaces"},
		{name: "conflicting scope", query: ListQuery{ReadScope: store.ReadScope{AllProfiles: true}, WorkspaceID: "ws-alpha", AllWorkspaces: true}, want: "choose exactly one workspace or all workspaces"},
	} {
		t.Run("Should reject invalid "+testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := normalizeListQuery(testCase.query)
			if !errors.Is(err, ErrListQueryInvalid) {
				t.Fatalf("normalizeListQuery(%#v) error = %v, want ErrListQueryInvalid", testCase.query, err)
			}
			if !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("normalizeListQuery(%#v) error = %v, want %q", testCase.query, err, testCase.want)
			}
		})
	}
}

func TestManagerAttentionCatalogUsesCanonicalBadgesAcrossPages(t *testing.T) {
	t.Parallel()

	t.Run("Should return exact summary counts beyond one catalog page", func(t *testing.T) {
		t.Parallel()

		catalog := &multiPagedRecordingSessionCatalog{
			recordingSessionCatalog: newRecordingSessionCatalog(),
			durable:                 attentionCatalogFixtures(105),
		}
		h := newHarness(t, WithSessionCatalog(catalog))

		summary, err := h.manager.AttentionSummary(
			testutil.Context(t),
			store.ReadScope{AllProfiles: true},
		)
		if err != nil {
			t.Fatalf("AttentionSummary() error = %v", err)
		}
		if got, want := summary.NeedsYou, 50; got != want {
			t.Fatalf("AttentionSummary().NeedsYou = %d, want %d", got, want)
		}
		if got, want := summary.Finished, 25; got != want {
			t.Fatalf("AttentionSummary().Finished = %d, want %d", got, want)
		}
		if got, want := len(summary.ByWorkspace), 3; got != want {
			t.Fatalf("AttentionSummary().ByWorkspace length = %d, want %d", got, want)
		}
		if catalog.calls != 0 || catalog.facetCalls != 1 {
			t.Fatalf(
				"row calls=%d metadata calls=%d, want zero rows and one metadata query",
				catalog.calls,
				catalog.facetCalls,
			)
		}
		if got := []string{
			summary.ByWorkspace[0].WorkspaceID,
			summary.ByWorkspace[1].WorkspaceID,
			summary.ByWorkspace[2].WorkspaceID,
		}; !slices.Equal(got, []string{"ws-a", "ws-b", "ws-c"}) {
			t.Fatalf("summary workspace order = %#v, want stable workspace order", got)
		}
	})

	t.Run("Should page exact badge matches with a stable attention cursor", func(t *testing.T) {
		t.Parallel()

		catalog := &multiPagedRecordingSessionCatalog{
			recordingSessionCatalog: newRecordingSessionCatalog(),
			durable:                 attentionCatalogFixtures(105),
		}
		h := newHarness(t, WithSessionCatalog(catalog))
		ctx := testutil.Context(t)
		query := ListQuery{
			ReadScope:     store.ReadScope{AllProfiles: true},
			AllWorkspaces: true,
			Badges:        []Badge{BadgeDone},
			Limit:         13,
		}
		first, err := h.manager.ListPage(ctx, query)
		if err != nil {
			t.Fatalf("ListPage(first) error = %v", err)
		}
		if first.Total != 25 || len(first.Sessions) != 13 || !first.HasMore || first.NextCursor == "" {
			t.Fatalf("ListPage(first) = %#v, want 13 of 25 with continuation", first)
		}
		query.Cursor = first.NextCursor
		second, err := h.manager.ListPage(ctx, query)
		if err != nil {
			t.Fatalf("ListPage(second) error = %v", err)
		}
		if second.Total != 25 || len(second.Sessions) != 12 || second.HasMore || second.NextCursor != "" {
			t.Fatalf("ListPage(second) = %#v, want remaining 12", second)
		}
		seen := make(map[string]struct{}, 25)
		for _, page := range [][]*Info{first.Sessions, second.Sessions} {
			for _, info := range page {
				if got := BadgeForInfo(info); got != BadgeDone {
					t.Fatalf("BadgeForInfo(%q) = %q, want %q", info.ID, got, BadgeDone)
				}
				if _, duplicate := seen[info.ID]; duplicate {
					t.Fatalf("session %q appeared on both cursor pages", info.ID)
				}
				seen[info.ID] = struct{}{}
			}
		}
	})

	t.Run("Should keep attention classes ordered across cursor pages", func(t *testing.T) {
		t.Parallel()

		base := time.Date(2026, 8, 15, 21, 0, 0, 0, time.UTC)
		changed := func(offset time.Duration) *time.Time {
			return new(base.Add(offset))
		}
		catalog := &multiPagedRecordingSessionCatalog{
			recordingSessionCatalog: newRecordingSessionCatalog(),
			durable: []store.SessionInfo{
				{
					ID: "sess-quiet-newest", AgentName: "coder", WorkspaceID: "ws-a",
					SessionType: string(SessionTypeUser), State: string(StateActive),
					Attention: &store.SessionAttention{AttentionChangedAt: changed(7 * time.Minute)},
					CreatedAt: base, UpdatedAt: base.Add(7 * time.Minute),
				},
				{
					ID:          "sess-prompting-unseen",
					AgentName:   "coder",
					WorkspaceID: "ws-a",
					SessionType: string(SessionTypeUser),
					State:       string(StateActive),
					Liveness: &store.SessionLivenessMeta{
						Activity: &store.SessionActivityMeta{TurnID: "turn-active"},
					},
					Attention: &store.SessionAttention{
						LastSettledRevision: 1,
						AttentionChangedAt:  changed(6 * time.Minute),
					},
					CreatedAt: base,
					UpdatedAt: base.Add(6 * time.Minute),
				},
				{
					ID:          "sess-stalled-unseen",
					AgentName:   "coder",
					WorkspaceID: "ws-a",
					SessionType: string(SessionTypeUser),
					State:       string(StateActive),
					Liveness: &store.SessionLivenessMeta{
						StallState:  store.SessionStallStateDetected,
						StallReason: "stalled",
					},
					Attention: &store.SessionAttention{
						LastSettledRevision: 1,
						AttentionChangedAt:  changed(5 * time.Minute),
					},
					CreatedAt: base,
					UpdatedAt: base.Add(5 * time.Minute),
				},
				{
					ID:          "sess-finished",
					AgentName:   "coder",
					WorkspaceID: "ws-a",
					SessionType: string(SessionTypeUser),
					State:       string(StateActive),
					Attention: &store.SessionAttention{
						LastSettledRevision: 1,
						AttentionChangedAt:  changed(4 * time.Minute),
					},
					CreatedAt: base,
					UpdatedAt: base.Add(4 * time.Minute),
				},
				{
					ID:          "sess-needs-input",
					AgentName:   "coder",
					WorkspaceID: "ws-a",
					SessionType: string(SessionTypeUser),
					State:       string(StateActive),
					Attention: &store.SessionAttention{
						PendingClarifyCount: 1,
						AttentionChangedAt:  changed(3 * time.Minute),
					},
					CreatedAt: base,
					UpdatedAt: base.Add(3 * time.Minute),
				},
				{
					ID:          "sess-needs-auth",
					AgentName:   "coder",
					WorkspaceID: "ws-a",
					SessionType: string(SessionTypeUser),
					State:       string(StateActive),
					Attention: &store.SessionAttention{
						PendingPermissionCount: 1,
						AttentionChangedAt:     changed(2 * time.Minute),
					},
					CreatedAt: base,
					UpdatedAt: base.Add(2 * time.Minute),
				},
				{
					ID: "sess-failed", AgentName: "coder", WorkspaceID: "ws-a",
					SessionType: string(SessionTypeUser), State: string(StateStopped),
					Failure:   &store.SessionFailure{Kind: store.FailurePrompt},
					Attention: &store.SessionAttention{AttentionChangedAt: changed(time.Minute)},
					CreatedAt: base, UpdatedAt: base.Add(time.Minute),
				},
			},
		}
		h := newHarness(t, WithSessionCatalog(catalog))
		query := ListQuery{
			ReadScope:     store.ReadScope{AllProfiles: true},
			AllWorkspaces: true,
			Sort:          ListSortAttention,
			Limit:         3,
		}
		ids := func(infos []*Info) []string {
			result := make([]string, 0, len(infos))
			for _, info := range infos {
				result = append(result, info.ID)
			}
			return result
		}

		first, err := h.manager.ListPage(testutil.Context(t), query)
		if err != nil {
			t.Fatalf("ListPage(first) error = %v", err)
		}
		if got, want := ids(
			first.Sessions,
		), []string{
			"sess-needs-input",
			"sess-needs-auth",
			"sess-failed",
		}; !slices.Equal(
			got,
			want,
		) {
			t.Fatalf("ListPage(first) ids = %#v, want %#v", got, want)
		}
		if !first.HasMore || first.NextCursor == "" {
			t.Fatalf("ListPage(first) = %#v, want continuation", first)
		}

		query.Cursor = first.NextCursor
		second, err := h.manager.ListPage(testutil.Context(t), query)
		if err != nil {
			t.Fatalf("ListPage(second) error = %v", err)
		}
		if got, want := ids(
			second.Sessions,
		), []string{
			"sess-finished",
			"sess-quiet-newest",
			"sess-prompting-unseen",
		}; !slices.Equal(
			got,
			want,
		) {
			t.Fatalf("ListPage(second) ids = %#v, want %#v", got, want)
		}
		if !second.HasMore || second.NextCursor == "" {
			t.Fatalf("ListPage(second) = %#v, want continuation", second)
		}

		query.Cursor = second.NextCursor
		third, err := h.manager.ListPage(testutil.Context(t), query)
		if err != nil {
			t.Fatalf("ListPage(third) error = %v", err)
		}
		if got, want := ids(third.Sessions), []string{"sess-stalled-unseen"}; !slices.Equal(got, want) {
			t.Fatalf("ListPage(third) ids = %#v, want %#v", got, want)
		}
		if third.HasMore || third.NextCursor != "" {
			t.Fatalf("ListPage(third) = %#v, want final page", third)
		}
	})
}

type multiPagedRecordingSessionCatalog struct {
	*recordingSessionCatalog
	durable    []store.SessionInfo
	calls      int
	facetCalls int
}

func (c *multiPagedRecordingSessionCatalog) SessionCatalogFacets(
	_ context.Context,
	query store.SessionCatalogPageQuery,
) (store.SessionCatalogFacetResult, error) {
	c.facetCalls++
	result := store.SessionCatalogFacetResult{}
	workspaces := map[string]store.SessionCatalogFacets{}
	for index := range c.durable {
		info := &c.durable[index]
		if slices.Contains(query.ExcludeIDs, info.ID) {
			continue
		}
		if query.Archive == store.SessionArchiveExclude && info.ArchivedAt != nil ||
			query.Archive == store.SessionArchiveOnly && info.ArchivedAt == nil {
			continue
		}
		if query.WorkspaceID != "" && info.WorkspaceID != query.WorkspaceID {
			continue
		}
		if (query.ExcludeVerdictOnly && info.SessionType == "system" && info.VerdictOnly) ||
			slices.Contains(query.ExcludeSessionTypes, info.SessionType) ||
			(query.SessionType != "" && info.SessionType != query.SessionType) {
			continue
		}
		badge := BadgeForInfo(sessionInfoFromCatalog(info))
		addSessionFacet(&result.Facets, badge)
		counts := workspaces[info.WorkspaceID]
		addSessionFacet(&counts, badge)
		workspaces[info.WorkspaceID] = counts
	}
	for id, counts := range workspaces {
		result.ByWorkspace = append(
			result.ByWorkspace,
			store.WorkspaceSessionCatalogFacets{WorkspaceID: id, Facets: counts},
		)
	}
	return result, nil
}

func (c *multiPagedRecordingSessionCatalog) PageSessions(
	_ context.Context,
	query store.SessionCatalogPageQuery,
) (store.SessionCatalogPage, error) {
	c.calls++
	excluded := make(map[string]struct{}, len(query.ExcludeIDs))
	for _, id := range query.ExcludeIDs {
		excluded[id] = struct{}{}
	}
	candidates := make([]store.SessionInfo, 0, len(c.durable))
	for index := range c.durable {
		info := &c.durable[index]
		if _, skip := excluded[info.ID]; skip {
			continue
		}
		archived := info.ArchivedAt != nil
		if query.Archive == store.SessionArchiveExclude && archived ||
			query.Archive == store.SessionArchiveOnly && !archived {
			continue
		}
		if (query.ExcludeVerdictOnly && info.SessionType == "system" && info.VerdictOnly) ||
			slices.Contains(query.ExcludeSessionTypes, info.SessionType) ||
			(query.SessionType != "" && info.SessionType != query.SessionType) {
			continue
		}
		badge := BadgeForInfo(sessionInfoFromCatalog(info))
		if query.AttentionOnly && ClassForBadge(badge) != AttentionNeedsYou {
			continue
		}
		if len(query.Badges) > 0 && !slices.Contains(query.Badges, string(badge)) {
			continue
		}
		candidates = append(candidates, *info)
	}
	slices.SortFunc(candidates, func(a, b store.SessionInfo) int {
		return compareSessionCatalogPosition(
			sessionCatalogPosition(&a, query.Sort),
			sessionCatalogPosition(&b, query.Sort),
		)
	})
	total := len(candidates)
	if query.After != nil {
		filtered := candidates[:0]
		for index := range candidates {
			info := &candidates[index]
			if compareSessionCatalogPosition(sessionCatalogPosition(info, query.Sort), *query.After) > 0 {
				filtered = append(filtered, *info)
			}
		}
		candidates = filtered
	}
	if len(candidates) > query.Limit {
		candidates = candidates[:query.Limit]
	}
	return store.SessionCatalogPage{Sessions: candidates, Total: total}, nil
}

func attentionCatalogFixtures(count int) []store.SessionInfo {
	base := time.Date(2026, 8, 15, 20, 0, 0, 0, time.UTC)
	infos := make([]store.SessionInfo, 0, count)
	for index := range count {
		workspaceID := []string{"ws-a", "ws-b", "ws-c"}[index%3]
		changedAt := base.Add(time.Duration(index/2) * time.Second)
		info := store.SessionInfo{
			ID:          fmt.Sprintf("sess-attention-%03d", index),
			Name:        fmt.Sprintf("Attention %03d", index),
			AgentName:   "coder",
			WorkspaceID: workspaceID,
			SessionType: string(SessionTypeUser),
			State:       string(StateActive),
			Attention:   &store.SessionAttention{AttentionChangedAt: &changedAt},
			CreatedAt:   base.Add(-time.Duration(index) * time.Minute),
			UpdatedAt:   base.Add(time.Duration(index) * time.Second),
		}
		switch {
		case index < 40:
			info.Attention.PendingPermissionCount = 1
		case index < 50:
			info.Attention.PendingClarifyCount = 1
		case index < 75:
			info.Attention.AttentionRevision = 1
			info.Attention.LastSettledRevision = 1
		}
		infos = append(infos, info)
	}
	return infos
}

type pagedRecordingSessionCatalog struct {
	*recordingSessionCatalog
	durable               store.SessionInfo
	agentMetrics          []store.SessionAgentMetrics
	lastQuery             store.SessionCatalogPageQuery
	lastAgentMetricsQuery store.SessionAgentMetricsQuery
	calls                 int
}

func (c *pagedRecordingSessionCatalog) PageSessions(
	_ context.Context,
	query store.SessionCatalogPageQuery,
) (store.SessionCatalogPage, error) {
	c.calls++
	c.lastQuery = query
	if c.durable.ID == "" {
		return store.SessionCatalogPage{}, nil
	}
	archived := c.durable.ArchivedAt != nil
	if (query.Archive == store.SessionArchiveExclude && archived) ||
		(query.Archive == store.SessionArchiveOnly && !archived) {
		return store.SessionCatalogPage{}, nil
	}
	page := store.SessionCatalogPage{Total: 1}
	position := sessionCatalogPosition(&c.durable, query.Sort)
	if query.After == nil || compareSessionCatalogPosition(position, *query.After) > 0 {
		page.Sessions = []store.SessionInfo{c.durable}
	}
	return page, nil
}

func (c *pagedRecordingSessionCatalog) SetSessionArchived(
	ctx context.Context,
	workspaceID string,
	sessionID string,
	archived bool,
) (store.SessionInfo, error) {
	updated, err := c.recordingSessionCatalog.SetSessionArchived(ctx, workspaceID, sessionID, archived)
	if err != nil {
		return store.SessionInfo{}, err
	}
	if c.durable.ID == sessionID {
		c.durable = updated
	}
	return updated, nil
}

func (c *pagedRecordingSessionCatalog) AggregateSessionsByAgent(
	_ context.Context,
	query store.SessionAgentMetricsQuery,
) ([]store.SessionAgentMetrics, error) {
	c.lastAgentMetricsQuery = query
	return append([]store.SessionAgentMetrics(nil), c.agentMetrics...), nil
}

func TestManagerStatusReturnsActiveAndStoredSessions(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)

	info, err := h.manager.Status(testutil.Context(t), session.ID)
	if err != nil {
		t.Fatalf("Status(active) error = %v", err)
	}
	if got := info.State; got != StateActive {
		t.Fatalf("Status(active).State = %q, want %q", got, StateActive)
	}

	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	info, err = h.manager.Status(testutil.Context(t), session.ID)
	if err != nil {
		t.Fatalf("Status(stopped) error = %v", err)
	}
	if got := info.State; got != StateStopped {
		t.Fatalf("Status(stopped).State = %q, want %q", got, StateStopped)
	}

	var nilCtx context.Context
	if _, err := h.manager.Status(nilCtx, session.ID); err == nil {
		t.Fatal("Status(nil, id) error = nil, want non-nil")
	}
	if _, err := h.manager.Status(testutil.Context(t), "   "); err == nil {
		t.Fatal("Status(blank id) error = nil, want non-nil")
	}
	if _, err := h.manager.Status(testutil.Context(t), "missing"); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("Status(missing) error = %v, want ErrSessionNotFound", err)
	}
}

func TestManagerStatusReturnsDurableAttentionForStoppedSession(t *testing.T) {
	t.Parallel()

	t.Run("Should retain pending permission attention after the runtime stops", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		h := newHarness(t)
		if err := h.manager.Shutdown(ctx); err != nil {
			t.Fatalf("Shutdown(initial manager) error = %v", err)
		}
		catalog := openManagerInputQueueStore(t)
		registerManagerInputQueueWorkspace(t, catalog, h)
		h.manager = newManagerWithHarness(t, h, WithSessionCatalog(catalog))
		cleanupTestManager(t, h.manager)

		target := createSession(t, h)
		if _, err := h.manager.createPendingInteraction(ctx, store.PendingInteractionCreate{
			InteractionID:     "interaction-status-restart",
			SessionID:         target.ID,
			Kind:              store.PendingInteractionKindPermission,
			ProviderRequestID: "request-status-restart",
			TurnID:            "turn-status-restart",
			Title:             "Allow the command?",
			CreatedAt:         time.Date(2026, 8, 16, 15, 0, 0, 0, time.UTC),
		}); err != nil {
			t.Fatalf("createPendingInteraction() error = %v", err)
		}
		if err := h.manager.Stop(ctx, target.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}

		info, err := h.manager.Status(ctx, target.ID)
		if err != nil {
			t.Fatalf("Status(stopped with attention) error = %v", err)
		}
		if got := info.PendingPermissionCount; got != 1 {
			t.Fatalf("Status(stopped with attention).PendingPermissionCount = %d, want 1", got)
		}
		if got := BadgeForInfo(info); got != BadgeWaitingForAuth {
			t.Fatalf("BadgeForInfo(Status(stopped with attention)) = %q, want %q", got, BadgeWaitingForAuth)
		}
	})
}

func TestManagerStatusRejectsTraversalSessionID(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	escapedID := createEscapedStoredSession(t, h)

	info, err := h.manager.Status(testutil.Context(t), "../"+escapedID)
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("Status(traversal) error = %v, want ErrSessionNotFound", err)
	}
	if info != nil {
		t.Fatalf("Status(traversal) info = %#v, want nil", info)
	}
}

func TestNormalizeStoredSessionIDRejectsWindowsDriveRelativePath(t *testing.T) {
	t.Parallel()

	if _, err := normalizeStoredSessionID("C:escape"); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("normalizeStoredSessionID(windows drive-relative) error = %v, want ErrSessionNotFound", err)
	}
}

func TestManagerStatusReadsDoNotRepublishSettledSessions(t *testing.T) {
	t.Parallel()

	t.Run("Should read a stopped session repeatedly without reprojecting or waking the catalog", func(t *testing.T) {
		t.Parallel()
		// Inactive reads republish only when the durable catalog projection changes.
		catalog := openManagerInputQueueStore(t)
		h := newHarness(t, WithSessionCatalog(catalog))
		registerManagerInputQueueWorkspace(t, catalog, h)
		session := createSession(t, h)
		events, err := h.manager.Prompt(testutil.Context(t), session.ID, "hello")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		collectEvents(t, events)
		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}

		catalogEvents, cancel, err := h.manager.SubscribeSessionCatalogEvents(
			testutil.Context(t),
			CatalogScope{ReadScope: store.ReadScope{AllProfiles: true}, AllWorkspaces: true},
		)
		if err != nil {
			t.Fatalf("SubscribeSessionCatalogEvents() error = %v", err)
		}
		defer cancel()

		for range 3 {
			info, err := h.manager.Status(testutil.Context(t), session.ID)
			if err != nil || info.State != StateStopped {
				t.Fatalf("Status(stopped) = %#v, %v", info, err)
			}
		}
		if _, err := h.manager.ListAll(testutil.Context(t)); err != nil {
			t.Fatalf("ListAll() error = %v", err)
		}
		select {
		case event := <-catalogEvents:
			t.Fatalf("settled session read republished catalog event %+v", event)
		case <-time.After(200 * time.Millisecond):
		}
	})
}

// exitedProcessLiveness restores the identity of a process that verifiably
// exited, so an interrupted start is classified from exit proof rather than
// left as an unverifiable process.
func exitedProcessLiveness(proc *AgentProcess) *store.SessionLivenessMeta {
	return &store.SessionLivenessMeta{SubprocessPID: proc.PID, SubprocessStartedAt: new(proc.StartedAt)}
}

func TestManagerReconcileIncompleteStartMetadata(t *testing.T) {
	t.Parallel()

	t.Run("Should retry catalog projection after metadata recovery already committed", func(t *testing.T) {
		t.Parallel()
		catalog := newRecordingSessionCatalog()
		h := newHarness(t, WithSessionCatalog(catalog))
		target := createSession(t, h)
		if err := h.manager.Stop(t.Context(), target.ID); err != nil {
			t.Fatal(err)
		}
		meta := readMeta(t, target.MetaPath())
		meta.State, meta.StopDetail, meta.StopReason = string(StateStarting), "", nil
		if err := store.WriteSessionMeta(target.MetaPath(), &meta); err != nil {
			t.Fatal(err)
		}
		projectionErr := errors.New("catalog unavailable")
		catalog.setUpdateErr(projectionErr)
		if err := h.manager.ReconcileInactiveSessions(t.Context()); !errors.Is(err, projectionErr) {
			t.Fatalf("failed projection: %v", err)
		}
		if err := h.manager.ReconcileInactiveSessions(t.Context()); !errors.Is(err, projectionErr) {
			t.Fatalf("session inventory hid incomplete recovery: %v", err)
		}
		// Without recorded process identity the interrupted start cannot prove exit,
		// so the committed recovery is a truthful stopping with startup attribution.
		if meta := readMeta(t, target.MetaPath()); meta.State != string(StateStopping) {
			t.Fatalf("metadata recovery did not commit: %s", meta.State)
		}
		catalog.setUpdateErr(nil)
		if err := h.manager.ReconcileInactiveSessions(t.Context()); err != nil {
			t.Fatal(err)
		}
		info, err := h.manager.Status(t.Context(), target.ID)
		if err != nil || info.StopReason != store.StopError {
			t.Fatalf("recovery retry: %#v, %v", info, err)
		}
		rows, err := catalog.ListSessions(t.Context(), store.SessionListQuery{ID: target.ID})
		if err != nil || len(rows) != 1 || rows[0].StopReason != store.StopError {
			t.Fatalf("retried catalog: %#v, %v", rows, err)
		}
	})
	t.Run("Should persist recovered state consistently in metadata and catalog", func(t *testing.T) {
		t.Parallel()
		catalog := openManagerInputQueueStore(t)
		h := newHarness(t, WithSessionCatalog(catalog))
		registerManagerInputQueueWorkspace(t, catalog, h)
		useExitedProcessForHealthRecovery(t, h)
		session := createSession(t, h)
		exited := h.driver.lastProcess().handle
		originalACP := session.Info().ACPSessionID

		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}

		meta, err := store.ReadSessionMeta(session.MetaPath())
		if err != nil {
			t.Fatalf("ReadSessionMeta() error = %v", err)
		}
		meta.State = string(StateStarting)
		meta.StopReason = nil
		meta.StopDetail = ""
		meta.ACPSessionID = stringPointer(originalACP)
		meta.Liveness = exitedProcessLiveness(exited)
		if err := store.WriteSessionMeta(session.MetaPath(), &meta); err != nil {
			t.Fatalf("WriteSessionMeta() error = %v", err)
		}

		if err := h.manager.ReconcileInactiveSessions(t.Context()); err != nil {
			t.Fatal(err)
		}
		info, err := h.manager.Status(testutil.Context(t), session.ID)
		if err != nil {
			t.Fatalf("Status(repaired) error = %v", err)
		}
		if got := info.State; got != StateStopped {
			t.Fatalf("Status(repaired).State = %q, want %q", got, StateStopped)
		}
		if got := info.StopReason; got != store.StopError {
			t.Fatalf("Status(repaired).StopReason = %q, want %q", got, store.StopError)
		}
		if got := info.StopDetail; got != resumeStopDetailStartIncomplete {
			t.Fatalf("Status(repaired).StopDetail = %q, want %q", got, resumeStopDetailStartIncomplete)
		}
		if got := info.ACPSessionID; got != "" {
			t.Fatalf("Status(repaired).ACPSessionID = %q, want empty", got)
		}

		repairedMeta, err := store.ReadSessionMeta(session.MetaPath())
		if err != nil {
			t.Fatalf("ReadSessionMeta(repaired) error = %v", err)
		}
		if got := repairedMeta.State; got != string(StateStopped) {
			t.Fatalf("repaired meta state = %q, want %q", got, StateStopped)
		}
		if repairedMeta.StopReason == nil || *repairedMeta.StopReason != store.StopError {
			t.Fatalf("repaired meta stop reason = %#v, want %q", repairedMeta.StopReason, store.StopError)
		}
		if got := repairedMeta.StopDetail; got != resumeStopDetailStartIncomplete {
			t.Fatalf("repaired meta stop detail = %q, want %q", got, resumeStopDetailStartIncomplete)
		}
		if repairedMeta.ACPSessionID != nil {
			t.Fatalf("repaired meta ACPSessionID = %#v, want nil", repairedMeta.ACPSessionID)
		}
		catalogRows, err := catalog.ListSessions(
			t.Context(),
			store.SessionListQuery{ID: session.ID, ReadScope: store.ReadScope{AllProfiles: true}},
		)
		if err != nil || len(catalogRows) != 1 {
			t.Fatalf("recovered catalog: %#v, %v", catalogRows, err)
		}
		row := catalogRows[0]
		if row.State != string(info.State) || row.StopReason != info.StopReason || row.StopDetail != info.StopDetail ||
			row.ACPSessionID != nil {
			t.Fatalf("catalog disagrees with recovered status: %#v", row)
		}
	})
}

func TestManagerReconcileInterruptedSessionAsStalledWhenLiveSubprocessIsStale(t *testing.T) {
	t.Parallel()

	t.Run("Should retain stalled live process identity and reject resume", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		session := createSession(t, h)
		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}

		meta, err := store.ReadSessionMeta(session.MetaPath())
		if err != nil {
			t.Fatalf("ReadSessionMeta() error = %v", err)
		}
		lastUpdate := time.Now().UTC().Add(-DefaultLivenessStallAfter - time.Minute)
		startedAt, err := procutil.StartedAt(os.Getpid())
		if err != nil {
			t.Fatal(err)
		}
		meta.State = string(StateActive)
		meta.StopReason = nil
		meta.StopDetail = ""
		meta.Liveness = &store.SessionLivenessMeta{
			SubprocessPID:       os.Getpid(),
			SubprocessStartedAt: &startedAt,
			LastUpdateAt:        &lastUpdate,
		}
		if err := store.WriteSessionMeta(session.MetaPath(), &meta); err != nil {
			t.Fatalf("WriteSessionMeta() error = %v", err)
		}

		if err := h.manager.ReconcileInactiveSessions(t.Context()); err != nil {
			t.Fatal(err)
		}
		info, err := h.manager.Status(testutil.Context(t), session.ID)
		if err != nil {
			t.Fatalf("Status(stalled) error = %v", err)
		}
		if got, want := info.State, StateStopping; got != want {
			t.Fatalf("Status(stalled).State = %q, want %q", got, want)
		}
		if got, want := info.StopReason, store.StopAgentCrashed; got != want {
			t.Fatalf("Status(stalled).StopReason = %q, want %q", got, want)
		}
		if got, want := info.StopDetail, resumeStopDetailAgentStalled; got != want {
			t.Fatalf("Status(stalled).StopDetail = %q, want %q", got, want)
		}
		if info.Liveness == nil {
			t.Fatal("Status(stalled).Liveness = nil, want liveness metadata")
		}
		if got, want := info.Liveness.StallState, store.SessionStallStateDetected; got != want {
			t.Fatalf("Status(stalled).Liveness.StallState = %q, want %q", got, want)
		}
		if got, want := info.Liveness.StallReason, store.SessionStallReasonActivityTimeout; got != want {
			t.Fatalf("Status(stalled).Liveness.StallReason = %q, want %q", got, want)
		}
		if _, err := h.manager.Resume(t.Context(), session.ID); !errors.Is(err, ErrStopVerificationFailed) {
			t.Fatalf("resume while old process is alive: %v", err)
		}
	})
}

func TestManagerStatusDoesNotRepairPendingStartMetadata(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	sessionID := "sess-pending"
	sessionDir := filepath.Join(h.homePaths.SessionsDir, sessionID)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(sessionDir) error = %v", err)
	}

	acpSessionID := "acp-pending"
	meta := store.SessionMeta{
		ID:            sessionID,
		Name:          "pending",
		AgentName:     "coder",
		WorkspaceID:   h.workspaceID,
		State:         string(StateStarting),
		RuntimeStatus: store.SessionRuntimeBinding,
		ACPSessionID:  stringPointer(acpSessionID),
		CreatedAt:     time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC),
		UpdatedAt:     time.Date(2026, 4, 20, 12, 0, 1, 0, time.UTC),
	}
	metaPath := store.SessionMetaFile(sessionDir)
	if err := store.WriteSessionMeta(metaPath, &meta); err != nil {
		t.Fatalf("WriteSessionMeta() error = %v", err)
	}

	h.manager.mu.Lock()
	h.manager.pending[sessionID] = sessionReservation{workspaceID: h.workspaceID}
	h.manager.mu.Unlock()

	info, err := h.manager.Status(testutil.Context(t), sessionID)
	if err != nil {
		t.Fatalf("Status(pending) error = %v", err)
	}
	if got := info.State; got != StateStarting {
		t.Fatalf("Status(pending).State = %q, want %q", got, StateStarting)
	}
	if got := info.ACPSessionID; got != acpSessionID {
		t.Fatalf("Status(pending).ACPSessionID = %q, want %q", got, acpSessionID)
	}
	if got := info.StopDetail; got != "" {
		t.Fatalf("Status(pending).StopDetail = %q, want empty", got)
	}

	storedMeta, err := store.ReadSessionMeta(metaPath)
	if err != nil {
		t.Fatalf("ReadSessionMeta() error = %v", err)
	}
	if got := storedMeta.State; got != string(StateStarting) {
		t.Fatalf("stored meta state = %q, want %q", got, StateStarting)
	}
	if storedMeta.StopReason != nil {
		t.Fatalf("stored meta stop reason = %#v, want nil", storedMeta.StopReason)
	}
	if got := stringValue(storedMeta.ACPSessionID); got != acpSessionID {
		t.Fatalf("stored meta ACPSessionID = %q, want %q", got, acpSessionID)
	}
}

func TestManagerEventsAndHistoryUseStoredEvents(t *testing.T) {
	t.Parallel()
	t.Run("Should sum settled own-turn work across idle gaps and recorder reopen", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		sess := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, sess.ID) })
		at := time.Date(2026, 10, 23, 10, 0, 0, 0, time.UTC)
		for _, turn := range []struct {
			id              string
			start, duration time.Duration
			terminal        string
		}{
			{"A", 0, 2 * time.Minute, acp.EventTypeDone},
			{"B", time.Hour, 3 * time.Minute, acp.EventTypeError},
			{"C", 2 * time.Hour, time.Minute, events.SessionTurnQuiesced},
			{"running", 3 * time.Hour, 0, ""},
		} {
			start := at.Add(turn.start)
			delivery := acp.AgentEvent{Type: acp.EventTypePromptDelivery, TurnID: turn.id, Timestamp: start}.
				WithDelivery(&acp.DeliveryManifest{TurnID: turn.id, SentAt: start})
			if err := h.manager.recordEvent(t.Context(), sess, delivery); err != nil {
				t.Fatal(err)
			}
			// A recovery delivery belongs to the same turn, not another interval.
			if turn.id == "A" {
				delivery.Timestamp = start.Add(time.Second)
				delivery = delivery.WithDelivery(&acp.DeliveryManifest{TurnID: turn.id, SentAt: delivery.Timestamp})
				if err := h.manager.recordEvent(t.Context(), sess, delivery); err != nil {
					t.Fatal(err)
				}
			}
			if turn.terminal != "" {
				if err := h.manager.recordEvent(t.Context(), sess, acp.AgentEvent{
					Type: turn.terminal, TurnID: turn.id, Timestamp: start.Add(turn.duration),
				}); err != nil {
					t.Fatal(err)
				}
			}
		}
		assertWork := func() {
			t.Helper()
			snapshot, err := h.manager.ContextEvents(t.Context(), sess.ID)
			if err != nil {
				t.Fatal(err)
			}
			work := snapshot.WorkTime
			if work.ActiveDurationMS != (6*time.Minute).Milliseconds() ||
				work.TurnCount != 3 || work.LastCompletedTurnID != "C" {
				t.Fatalf("work time = %#v, want six minutes across three settled turns", work)
			}
		}
		assertWork()
		if err := h.manager.Stop(t.Context(), sess.ID); err != nil {
			t.Fatal(err)
		}
		assertWork()
	})

	t.Run("Should preserve event metadata for active and stopped session reads", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		sess := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, sess.ID) })
		at := time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC)
		for _, event := range []acp.AgentEvent{
			{Type: acp.EventTypeAgentMessage, TurnID: "A", Timestamp: at, Text: "message"},
			{Type: acp.EventTypeUsage, TurnID: "A", Timestamp: at, Usage: &acp.TokenUsage{TotalTokens: new(int64(25))}},
		} {
			if err := h.manager.recordEvent(t.Context(), sess, event); err != nil {
				t.Fatal(err)
			}
		}
		assertMetadata := func() {
			t.Helper()
			query := store.EventQuery{TurnID: "A", AfterSequence: 1}
			full, err := h.manager.Events(t.Context(), sess.ID, query)
			if err != nil || len(full) == 0 {
				t.Fatalf("Events()=%#v/%v", full, err)
			}
			metadata, err := h.manager.EventMetadata(t.Context(), sess.ID, query)
			if err != nil || len(metadata) != len(full) {
				t.Fatalf("EventMetadata()=%#v/%v", metadata, err)
			}
			for index, event := range full {
				want := store.EventMetadata{ID: event.ID, SessionID: event.SessionID, Sequence: event.Sequence,
					TurnID: event.TurnID, Type: event.Type, AgentName: event.AgentName, Timestamp: event.Timestamp}
				if metadata[index] != want {
					t.Fatalf("metadata[%d]=%#v, want=%#v", index, metadata[index], want)
				}
			}
		}
		assertMetadata()
		if err := h.manager.Stop(t.Context(), sess.ID); err != nil {
			t.Fatal(err)
		}
		assertMetadata()
	})

	t.Run("Should read a settled context snapshot through one stopped recorder", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		sess := createSession(t, h)
		for _, event := range []acp.AgentEvent{
			{Type: acp.EventTypeUsage, TurnID: "usage", Usage: &acp.TokenUsage{ContextUsed: new(int64(42))}},
			{Type: acp.EventTypeDone, TurnID: "settled"},
		} {
			if err := h.manager.recordEvent(t.Context(), sess, event); err != nil {
				t.Fatal(err)
			}
		}
		if err := h.manager.Stop(t.Context(), sess.ID); err != nil {
			t.Fatal(err)
		}
		opens := 0
		open := h.manager.openQueryStore
		h.manager.openQueryStore = func(ctx context.Context, owner store.SessionDBOwner, path string) (EventReadCloser, error) {
			opens++
			return open(ctx, owner, path)
		}
		snapshot, err := h.manager.ContextEvents(t.Context(), sess.ID)
		if err != nil || opens != 1 || len(snapshot.Usage) != 1 || snapshot.Settled.TurnID != "settled" ||
			snapshot.Settled.Sequence <= snapshot.Usage[0].Sequence {
			t.Fatalf("ContextEvents()=%#v, opens=%d, error=%v", snapshot, opens, err)
		}
	})

	for _, freshBeforeNext := range []bool{false, true} {
		name := "Should leave after absent when the next compaction ends before an occupancy observation"
		if freshBeforeNext {
			name = "Should retain the first same-turn occupancy after the first terminal despite corrections"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			target := createSession(t, h)
			t.Cleanup(func() { reportSessionStop(t, h, target.ID) })
			at := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
			appendEvent := func(event acp.AgentEvent) int64 {
				t.Helper()
				event.TurnID, event.Timestamp = "A", at
				if err := h.manager.recordEvent(t.Context(), target, event); err != nil {
					t.Fatal(err)
				}
				rows, err := h.manager.Events(t.Context(), target.ID, store.EventQuery{Type: event.Type})
				if err != nil || len(rows) == 0 {
					t.Fatalf("recorded rows=%#v error=%v", rows, err)
				}
				return rows[len(rows)-1].Sequence
			}
			attribute := func(id string) {
				t.Helper()
				payload, err := json.Marshal(
					CompactionFiredPayload{CompactionID: id, Trigger: "agent", ContextUsed: new(int64(100))},
				)
				if err != nil {
					t.Fatal(err)
				}
				appendEvent(acp.AgentEvent{Type: events.SessionCompactionFired, Raw: payload})
			}
			attribute("c1")
			first := appendEvent(acp.AgentEvent{Type: acp.EventTypeCompaction, Compaction: &acp.CompactionObservation{
				CompactionID: "c1", Status: "completed", Terminal: true,
			}})
			appendEvent(acp.AgentEvent{Type: acp.EventTypeUsage, Usage: &acp.TokenUsage{TotalTokens: new(int64(999))}})
			pending, err := h.manager.Compactions(t.Context(), target.ID)
			if err != nil || len(pending) != 1 || pending[0].ContextAfter != nil {
				t.Fatalf("counter-only after=%#v error=%v", pending, err)
			}
			var firstAfter int64
			if freshBeforeNext {
				firstAfter = appendEvent(acp.AgentEvent{Type: acp.EventTypeUsage, Usage: &acp.TokenUsage{
					ContextUsed: new(int64(25)), ContextSize: new(int64(200)),
				}})
			}
			attribute("c2")
			second := appendEvent(acp.AgentEvent{Type: acp.EventTypeCompaction, Compaction: &acp.CompactionObservation{
				CompactionID: "c2", Status: "failed", Terminal: true,
			}})
			appendEvent(acp.AgentEvent{Type: acp.EventTypeDone, Usage: &acp.TokenUsage{TotalTokens: new(int64(1000))}})
			secondAfter := appendEvent(
				acp.AgentEvent{Type: acp.EventTypeUsage, Usage: &acp.TokenUsage{ContextUsed: new(int64(0))}},
			)
			appendEvent(acp.AgentEvent{Type: acp.EventTypeUsage, Usage: &acp.TokenUsage{
				ContextUsed: new(int64(40)), ContextSize: new(int64(300)),
			}})
			appendEvent(acp.AgentEvent{Type: acp.EventTypeCompaction, Compaction: &acp.CompactionObservation{
				CompactionID: "c1", Status: "failed", Terminal: true,
			}})
			appendEvent(acp.AgentEvent{Type: acp.EventTypeCompaction, Compaction: &acp.CompactionObservation{
				CompactionID: "c2", Status: "completed", Summary: "corrected summary",
			}})
			assertRead := func() {
				t.Helper()
				markers, err := h.manager.Compactions(t.Context(), target.ID)
				if err != nil || len(markers) != 2 {
					t.Fatalf("markers=%#v error=%v", markers, err)
				}
				byID := make(map[string]CompactionEnvelope)
				for _, marker := range markers {
					byID[marker.Payload.CompactionID] = marker
				}
				one, two := byID["c1"], byID["c2"]
				if one.Status != "failed" || two.Status != "completed" {
					t.Fatalf("latest corrective statuses=%#v", markers)
				}
				if freshBeforeNext {
					if one.ContextAfter == nil || one.ContextAfter.Used != 25 || one.ContextAfter.Size == nil ||
						*one.ContextAfter.Size != 200 || one.ContextAfter.Sequence != firstAfter {
						t.Fatalf("first same-turn after=%#v, boundary=%d", one.ContextAfter, first)
					}
				} else if one.ContextAfter != nil {
					t.Fatalf("earlier compaction borrowed later occupancy=%#v", one.ContextAfter)
				}
				if two.ContextAfter == nil || two.ContextAfter.Used != 0 || two.ContextAfter.Size != nil ||
					two.ContextAfter.Sequence != secondAfter {
					t.Fatalf("zero used-only after=%#v", two.ContextAfter)
				}
				cause, err := h.manager.CompactionClearBoundary(t.Context(), target.ID)
				if err != nil || cause == nil || cause.CompactionID != "c2" || cause.Sequence != second {
					t.Fatalf("first-terminal cause=%#v error=%v", cause, err)
				}
				boundary, err := h.manager.CompactionBoundary(t.Context(), target.ID)
				if err != nil || boundary == nil || *boundary != second {
					t.Fatalf("legacy boundary=%v error=%v", boundary, err)
				}
				snapshot, err := h.manager.ContextEvents(t.Context(), target.ID)
				if err != nil || !reflect.DeepEqual(snapshot.Compactions, markers) ||
					!reflect.DeepEqual(snapshot.ClearedBy, cause) {
					t.Fatalf(
						"snapshot compactions=%#v, boundary=%#v, error=%v",
						snapshot.Compactions,
						snapshot.ClearedBy,
						err,
					)
				}
			}
			assertRead()
			if err := h.manager.Stop(t.Context(), target.ID); err != nil {
				t.Fatal(err)
			}
			assertRead()
		})
	}
	t.Run(
		"Should read complete usage deliveries and attributed compaction snapshots on active and stopped sessions",
		func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			sess := createSession(t, h)
			ctx := testutil.Context(t)
			t.Cleanup(func() { reportSessionStop(t, h, sess.ID) })
			at := time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC)
			input := []acp.AgentEvent{
				acp.AgentEvent{
					Type:      acp.EventTypePromptDelivery,
					TurnID:    "A",
					Timestamp: at,
				}.WithDelivery(&acp.DeliveryManifest{
					TurnID:   "A",
					SentAt:   at.Add(-time.Second),
					Estimate: acp.TextEstimateMethod,
					Spans:    []acp.DeliveredSpan{{Key: "skills", Kind: "text", Bytes: 80, Tokens: new(int64(20))}},
				}),
				{
					Type:      acp.EventTypeUsage,
					TurnID:    "A",
					Timestamp: at,
					Usage: &acp.TokenUsage{
						TurnID:      "A",
						ContextUsed: new(int64(80)),
						ContextSize: new(int64(100)),
						Timestamp:   at,
					},
				},
				{
					Type:      acp.EventTypeDone,
					TurnID:    "A",
					Timestamp: at.Add(-time.Hour),
					Usage:     &acp.TokenUsage{TurnID: "A", InputTokens: new(int64(10)), Timestamp: at},
				},
			}
			for _, event := range input {
				if err := h.manager.recordEvent(ctx, sess, event); err != nil {
					t.Fatal(err)
				}
			}
			rows, err := h.manager.Events(ctx, sess.ID, store.EventQuery{})
			if err != nil {
				t.Fatal(err)
			}
			from, to := rows[0].Sequence, rows[len(rows)-1].Sequence
			raw, err := json.Marshal(CompactionFiredPayload{
				CompactionID: "c1", Trigger: "agent", ContextUsed: new(int64(85)), ContextSize: new(int64(100)),
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := h.manager.recordEvent(ctx, sess, acp.AgentEvent{
				Type: events.SessionCompactionFired, TurnID: "B", Raw: raw, Timestamp: at,
			}); err != nil {
				t.Fatal(err)
			}
			for _, event := range []acp.AgentEvent{
				{Type: events.SessionCompactionFired, TurnID: "B", Timestamp: at, Raw: []byte(`{"strategy":"summary_archive"}`)},
				{Type: acp.EventTypeCompaction, TurnID: "B", Timestamp: at, Compaction: &acp.CompactionObservation{CompactionID: "c1", Status: "in_progress"}},
				{Type: acp.EventTypeCompaction, TurnID: "B", Timestamp: at, Compaction: &acp.CompactionObservation{CompactionID: "c1", Status: "completed", Terminal: true}},
			} {
				if err := h.manager.recordEvent(ctx, sess, event); err != nil {
					t.Fatal(err)
				}
			}
			markers, err := h.manager.Compactions(ctx, sess.ID)
			if err != nil {
				t.Fatal(err)
			}
			if len(markers) != 1 || markers[0].Status != "completed" || markers[0].Payload.CompactionID != "c1" {
				t.Fatalf("unarchived marker=%#v", markers)
			}
			if _, err := sess.recorderHandle().(store.EventArchiver).ArchiveEvents(
				ctx,
				store.EventArchiveRequest{FromSequence: from, ToSequence: to},
			); err != nil {
				t.Fatal(err)
			}
			assertRead := func() {
				t.Helper()
				usage, err := h.manager.UsageEvents(ctx, sess.ID)
				if err != nil {
					t.Fatal(err)
				}
				if len(usage) != 2 || usage[0].Sequence >= usage[1].Sequence || usage[0].Usage.ContextUsed == nil ||
					usage[1].Usage.InputTokens == nil {
					t.Fatalf("usage ledger=%#v", usage)
				}
				deliveries, err := h.manager.Deliveries(ctx, sess.ID)
				if err != nil {
					t.Fatal(err)
				}
				if len(deliveries) != 1 || deliveries[0].Manifest.Estimate != acp.TextEstimateMethod ||
					deliveries[0].Manifest.Spans[0].Tokens == nil ||
					*deliveries[0].Manifest.Spans[0].Tokens != 20 {
					t.Fatalf("deliveries=%#v", deliveries)
				}
				settled, err := h.manager.LatestSettledTurn(ctx, sess.ID)
				if err != nil {
					t.Fatal(err)
				}
				if settled.TurnID != "A" || settled.Sequence != usage[1].Sequence {
					t.Fatalf("settled=%#v", settled)
				}
				markers, err := h.manager.Compactions(ctx, sess.ID)
				if err != nil {
					t.Fatal(err)
				}
				if len(markers) != 1 || markers[0].Status != "completed" || markers[0].Payload.Trigger != "agent" {
					t.Fatalf("archived marker=%#v", markers)
				}
				snapshot, err := h.manager.ContextEvents(ctx, sess.ID)
				if err != nil || !reflect.DeepEqual(snapshot.Usage, usage) ||
					!reflect.DeepEqual(snapshot.Deliveries, deliveries) ||
					!reflect.DeepEqual(snapshot.Compactions, markers) ||
					snapshot.Settled != settled {
					t.Fatalf("ContextEvents()=%#v, error=%v; want existing context projection", snapshot, err)
				}
			}
			assertRead()
			if err := h.manager.Stop(ctx, sess.ID); err != nil {
				t.Fatal(err)
			}
			assertRead()
			if _, err := h.manager.UsageEvents(ctx, "missing-session"); !errors.Is(err, ErrSessionNotFound) {
				t.Fatalf("unknown session error=%v", err)
			}
		},
	)

	h := newHarness(t)
	session := createSession(t, h)

	eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "hello")
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	runtimeEvents := collectEvents(t, eventsCh)
	if len(runtimeEvents) != 2 {
		t.Fatalf("Prompt() events = %d, want 2", len(runtimeEvents))
	}

	activeEvents, err := h.manager.Events(testutil.Context(t), session.ID, store.EventQuery{})
	if err != nil {
		t.Fatalf("Events(active) error = %v", err)
	}
	if len(activeEvents) != 3 {
		t.Fatalf("Events(active) = %d events, want 3", len(activeEvents))
	}
	activeHistory, err := h.manager.History(testutil.Context(t), session.ID, store.EventQuery{})
	if err != nil {
		t.Fatalf("History(active) error = %v", err)
	}
	if len(activeHistory) != 1 {
		t.Fatalf("History(active) = %d turns, want 1", len(activeHistory))
	}

	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	stopOnly, err := h.manager.Events(testutil.Context(t), session.ID, store.EventQuery{
		Type:  EventTypeSessionStopped,
		Limit: 1,
	})
	if err != nil {
		t.Fatalf("Events(stopOnly) error = %v", err)
	}
	if len(stopOnly) != 1 {
		t.Fatalf("Events(stopOnly) = %d events, want 1", len(stopOnly))
	}
	if got := stopOnly[0].Type; got != EventTypeSessionStopped {
		t.Fatalf("Events(stopOnly)[0].Type = %q, want %q", got, EventTypeSessionStopped)
	}
	if got := countEventType(stopOnly, EventTypeSessionStopped); got != 1 {
		t.Fatalf("Events(stopOnly) %q count = %d, want 1", EventTypeSessionStopped, got)
	}

	afterPrompt, err := h.manager.Events(testutil.Context(t), session.ID, store.EventQuery{
		AfterSequence: activeEvents[len(activeEvents)-1].Sequence,
	})
	if err != nil {
		t.Fatalf("Events(after prompt) error = %v", err)
	}
	// The fake process only exits on the forced stop, so the shared ladder records
	// its escalation before the terminal event and its marker.
	wantAfterPrompt := []string{
		events.SessionStopEscalated,
		EventTypeSessionStopped,
		events.TranscriptMarkerCreated,
	}
	if len(afterPrompt) != len(wantAfterPrompt) {
		types := make([]string, 0, len(afterPrompt))
		for _, event := range afterPrompt {
			types = append(types, event.Type)
		}
		t.Fatalf("Events(after prompt) = %d events %v, want %v", len(afterPrompt), types, wantAfterPrompt)
	}
	for index, want := range wantAfterPrompt {
		if got := afterPrompt[index].Type; got != want {
			t.Fatalf("Events(after prompt)[%d].Type = %q, want %q", index, got, want)
		}
	}

	stoppedHistory, err := h.manager.History(testutil.Context(t), session.ID, store.EventQuery{})
	if err != nil {
		t.Fatalf("History(stopped) error = %v", err)
	}
	if len(stoppedHistory) != 2 {
		t.Fatalf("History(stopped) = %d turns, want 2", len(stoppedHistory))
	}
	// The stop's whole footprint shares one history turn: the ladder escalation
	// record precedes the terminal event instead of opening a turn of its own.
	stopTurnTypes := make([]string, 0, len(stoppedHistory[1].Events))
	for _, event := range stoppedHistory[1].Events {
		stopTurnTypes = append(stopTurnTypes, event.Type)
	}
	if got := stopTurnTypes[0]; got != events.SessionStopEscalated {
		t.Fatalf("History(stopped)[1] events = %v, want ladder escalation first", stopTurnTypes)
	}
	if got := countEventType(stoppedHistory[1].Events, EventTypeSessionStopped); got != 1 {
		t.Fatalf("History(stopped)[1] events = %v, want one %q", stopTurnTypes, EventTypeSessionStopped)
	}
}

func TestCompactionMarkersAcrossConcurrentAppends(t *testing.T) {
	t.Parallel()

	t.Run("Should not lend a later compaction's usage to an earlier one appended between reads", func(t *testing.T) {
		t.Parallel()

		row := func(t *testing.T, sequence int64, event acp.AgentEvent) store.SessionEvent {
			t.Helper()
			content, err := transcript.MarshalAgentEvent(event)
			if err != nil {
				t.Fatalf("MarshalAgentEvent(%d) error = %v", sequence, err)
			}
			return store.SessionEvent{Sequence: sequence, Type: event.Type, TurnID: event.TurnID, Content: content}
		}
		fired := func(t *testing.T, sequence int64, id string) store.SessionEvent {
			t.Helper()
			raw, err := json.Marshal(CompactionFiredPayload{CompactionID: id, Trigger: "agent"})
			if err != nil {
				t.Fatal(err)
			}
			return row(t, sequence, acp.AgentEvent{Type: events.SessionCompactionFired, TurnID: id, Raw: raw})
		}
		terminal := func(t *testing.T, sequence int64, id string) store.SessionEvent {
			t.Helper()
			return row(t, sequence, acp.AgentEvent{Type: acp.EventTypeCompaction, TurnID: id,
				Compaction: &acp.CompactionObservation{CompactionID: id, Status: "completed", Terminal: true}})
		}
		// Compaction A completed with no later usage yet; B completes after the first reads.
		history := []store.SessionEvent{fired(t, 1, "a"), terminal(t, 2, "a")}
		later := []store.SessionEvent{
			fired(t, 3, "b"),
			terminal(t, 4, "b"),
			row(t, 5, acp.AgentEvent{Type: acp.EventTypeUsage, TurnID: "b",
				Usage: &acp.TokenUsage{ContextUsed: new(int64(7)), ContextSize: new(int64(100))}}),
		}
		reads := 0
		read := func(_ context.Context, kind string) ([]store.SessionEvent, error) {
			reads++
			// B lands after the probe and the fired read, before usage is read.
			if reads == 3 {
				history = append(history, later...)
			}
			result := make([]store.SessionEvent, 0)
			for _, event := range history {
				if event.Type == kind {
					result = append(result, event)
				}
			}
			return result, nil
		}

		markers, err := compactionMarkers(t.Context(), read)
		if err != nil {
			t.Fatalf("compactionMarkers() error = %v", err)
		}
		index := slices.IndexFunc(markers, func(marker CompactionEnvelope) bool {
			return marker.Payload.CompactionID == "a"
		})
		if index < 0 {
			t.Fatalf("markers = %#v, want compaction a", markers)
		}
		if after := markers[index].ContextAfter; after != nil {
			t.Fatalf("compaction a ContextAfter = %#v, want none: usage at 5 follows compaction b", after)
		}
		snapshot, err := contextEventsFromRows(history)
		if err != nil || len(snapshot.Compactions) != 2 || snapshot.Compactions[0].ContextAfter != nil ||
			snapshot.Compactions[1].ContextAfter == nil || snapshot.Compactions[1].ContextAfter.Sequence != 5 {
			t.Fatalf("union snapshot borrowed compaction observations: %#v, %v", snapshot.Compactions, err)
		}
	})
}

func TestManagerLatestSessionEventByType(t *testing.T) {
	t.Parallel()

	t.Run("Should return the newest typed event for active and stopped sessions", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		sess := createSession(t, h)
		ctx := testutil.Context(t)
		for revision := 1; revision <= 2; revision++ {
			if err := h.manager.AppendSessionEventIfAbsent(ctx, GoalEvent{
				EventID:         fmt.Sprintf("goal-snapshot-%d", revision),
				SessionID:       sess.ID,
				SyntheticTurnID: "goal-snapshot:" + sess.ID,
				AgentName:       "system",
				Type:            EventTypeGoalSnapshotChanged,
				Content:         json.RawMessage(fmt.Sprintf(`{"revision":%d}`, revision)),
				CreatedAt:       time.Now().UTC().Add(time.Duration(revision) * time.Second),
			}); err != nil {
				t.Fatalf("AppendSessionEventIfAbsent(%d) error = %v", revision, err)
			}
		}

		assertLatest := func(state string) {
			t.Helper()
			event, err := h.manager.LatestSessionEventByType(ctx, sess.ID, EventTypeGoalSnapshotChanged)
			if err != nil {
				t.Fatalf("LatestSessionEventByType(%s) error = %v", state, err)
			}
			if event == nil || event.ID != "goal-snapshot-2" {
				t.Fatalf("LatestSessionEventByType(%s) = %#v, want goal-snapshot-2", state, event)
			}
		}

		assertLatest("active")
		if err := h.manager.Stop(ctx, sess.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		assertLatest("stopped")
	})
}

func TestManagerEventsAndHistoryRetryClosedActiveRecorder(t *testing.T) {
	t.Parallel()

	t.Run("Should retry when finalization closes the active recorder during a query", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		originalRecorder := session.recorderHandle()
		if originalRecorder == nil {
			t.Fatal("recorderHandle() = nil, want active recorder")
		}
		if err := originalRecorder.Close(testutil.Context(t)); err != nil {
			t.Fatalf("Close(original recorder) error = %v", err)
		}

		wantEvent := store.SessionEvent{
			ID:        "ev-retry",
			SessionID: session.ID,
			Sequence:  1,
			TurnID:    "turn-retry",
			Type:      "agent_message",
			AgentName: "coder",
			Content:   "{}",
		}
		stub := &queryRecorderStub{
			events:       []store.SessionEvent{wantEvent},
			history:      []store.TurnHistory{{TurnID: wantEvent.TurnID, Events: []store.SessionEvent{wantEvent}}},
			usage:        []store.TokenUsage{{TurnID: wantEvent.TurnID}},
			queryErrs:    []error{store.ErrClosed},
			metadataErrs: []error{store.ErrClosed},
			historyErrs: []error{
				store.ErrClosed,
			},
			usageErrs: []error{store.ErrClosed},
		}
		session.setRecorder(stub)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
				t.Fatalf("cleanup Stop() error = %v", err)
			}
		})

		events, err := h.manager.Events(testutil.Context(t), session.ID, store.EventQuery{})
		if err != nil {
			t.Fatalf("Events() error = %v", err)
		}
		if len(events) != 1 || events[0].ID != wantEvent.ID {
			t.Fatalf("Events() = %#v, want retry event", events)
		}
		if got := len(stub.queryCalls); got != 2 {
			t.Fatalf("Query() calls = %d, want 2", got)
		}

		metadata, err := h.manager.EventMetadata(t.Context(), session.ID, store.EventQuery{})
		if err != nil || len(metadata) != 1 || metadata[0].ID != wantEvent.ID {
			t.Fatalf("EventMetadata()=%#v/%v", metadata, err)
		}
		if len(stub.metadataCalls) != 2 {
			t.Fatalf("QueryEventMetadata() calls=%d, want 2", len(stub.metadataCalls))
		}

		history, err := h.manager.History(testutil.Context(t), session.ID, store.EventQuery{})
		if err != nil {
			t.Fatalf("History() error = %v", err)
		}
		if len(history) != 1 || history[0].TurnID != wantEvent.TurnID {
			t.Fatalf("History() = %#v, want retry history", history)
		}
		if got := len(stub.historyCall); got != 2 {
			t.Fatalf("History() calls = %d, want 2", got)
		}

		usage, err := h.manager.TokenUsage(testutil.Context(t), session.ID)
		if err != nil {
			t.Fatalf("TokenUsage() error = %v", err)
		}
		if len(usage) != 1 || usage[0].TurnID != wantEvent.TurnID {
			t.Fatalf("TokenUsage() = %#v, want retry usage", usage)
		}
		if got := stub.usageCalls; got != 2 {
			t.Fatalf("ListTokenUsage() calls = %d, want 2", got)
		}
	})
}

func TestManagerEventsRejectTraversalSessionID(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	escapedID := createEscapedStoredSession(t, h)

	events, err := h.manager.Events(testutil.Context(t), "../"+escapedID, store.EventQuery{})
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("Events(traversal) error = %v, want ErrSessionNotFound", err)
	}
	if events != nil {
		t.Fatalf("Events(traversal) = %#v, want nil", events)
	}
}

func TestManagerOpenQueryRecorderValidationAndCleanup(t *testing.T) {
	t.Parallel()

	// The manager resolves migrated Global scope from the catalog while retaining
	// the creation witness's immutable events.db owner, including substitution refusal.
	for _, foreignOwner := range []bool{false, true} {
		name := "Should read migrated Global history through its original database owner"
		if foreignOwner {
			name = "Should refuse substituted metadata and database owners for Global history"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			catalog := openManagerInputQueueStore(t)
			h := newHarness(t, WithSessionCatalog(catalog))
			const sessionID = "sess-retained-global"
			const originalWorkspaceID = "ws-retired-home"
			const profileJSON = `{"version":3,"agent_name":"coder","provider":"claude","speed":"normal","workspace_id":"ws-retired-home","cwd":"/retained-home","sandbox_mode":"none","permissions":"approve-all"}`
			const optionsJSON = `{"session_id":"sess-retained-global","network_owner_key":"session:sess-retained-global","network_participation":{"version":"network-participation/v1","mode":"local","source":"built_in_local"},"session_type":"user"}`
			var profile store.SessionCreationProfile
			var options store.SessionCreationOptions
			if err := json.Unmarshal([]byte(profileJSON), &profile); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(optionsJSON), &options); err != nil {
				t.Fatal(err)
			}
			ref, err := catalog.PutSessionCreationProfile(t.Context(), profile)
			if err != nil {
				t.Fatal(err)
			}
			policy, err := profile.PolicySpecDigest()
			if err != nil {
				t.Fatal(err)
			}
			creation, err := profile.CreationDigest(options)
			if err != nil {
				t.Fatal(err)
			}
			now := time.Date(2026, 10, 4, 9, 35, 0, 0, time.UTC)
			_, err = catalog.RegisterSessionWithCreationIdentity(t.Context(), store.SessionInfo{
				ID: sessionID, ProfileID: store.DefaultProfileID, AgentName: "coder", Provider: "claude",
				State: string(StateStopped), RuntimeStatus: store.SessionRuntimeReady, CreatedAt: now, UpdatedAt: now,
			}, store.SessionCreationIdentity{CreationProfileRef: ref, PolicySpecDigest: policy, CreationDigest: creation})
			if err != nil {
				t.Fatal(err)
			}
			workspaceID := originalWorkspaceID
			if foreignOwner {
				workspaceID = "ws-substituted"
			}
			sessionDir := filepath.Join(h.homePaths.SessionsDir, sessionID)
			metaPath := store.SessionMetaFile(sessionDir)
			if err := os.MkdirAll(sessionDir, 0700); err != nil {
				t.Fatal(err)
			}
			if err := store.WriteSessionMeta(metaPath, &store.SessionMeta{
				ID: sessionID, AgentName: "coder", Provider: "claude", WorkspaceID: workspaceID,
				State: string(StateStopped), RuntimeStatus: store.SessionRuntimeReady,
				CreationProfile: &profile, CreationOptions: &options, CreationProfileRef: ref,
				PolicySpecDigest: policy, CreationDigest: creation, CreatedAt: now, UpdatedAt: now,
			}); err != nil {
				t.Fatal(err)
			}
			database, err := sessiondb.OpenSessionDB(
				t.Context(),
				testSessionDBOwner(sessionID, workspaceID),
				store.SessionDBFile(sessionDir),
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := database.Record(t.Context(), store.SessionEvent{
				TurnID: "turn-retained", Type: acp.EventTypeAgentMessage, AgentName: "coder",
				Content: `{"type":"agent_message","text":"Retained work"}`, Timestamp: now,
			}); err != nil {
				t.Fatal(err)
			}
			if err := database.Close(t.Context()); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(metaPath)
			if err != nil {
				t.Fatal(err)
			}
			owner, ownerErr := h.manager.SessionOwner(t.Context(), sessionID)
			if foreignOwner {
				if !errors.Is(ownerErr, store.ErrSessionWorkspaceMismatch) {
					t.Fatalf("SessionOwner(substituted) = %v, want catalog refusal", ownerErr)
				}
			} else {
				if ownerErr != nil || owner.WorkspaceID != "" {
					t.Fatalf("SessionOwner(global) = %+v, %v", owner, ownerErr)
				}
				info, err := h.manager.Status(t.Context(), sessionID)
				if err != nil || info.WorkspaceID != "" || info.ProfileID != store.DefaultProfileID {
					t.Fatalf("Status(global) = %+v, %v", info, err)
				}
				if err := h.manager.UpgradeSessionDatabase(t.Context(), sessionID); err != nil {
					t.Fatal(err)
				}
			}
			entries, readErr := h.manager.Events(t.Context(), sessionID, store.EventQuery{})
			if foreignOwner {
				if !errors.Is(readErr, store.ErrSessionWorkspaceMismatch) {
					t.Fatalf("Events(substituted) = %v, want catalog refusal", readErr)
				}
			} else if readErr != nil || len(entries) != 1 || !strings.Contains(entries[0].Content, "Retained work") {
				t.Fatalf("Events(global) = %+v, %v", entries, readErr)
			}
			after, err := os.ReadFile(metaPath)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("read changed retained metadata: %v", err)
			}
		})
	}

	// Invariant: startup upgrades only older owned databases through the writer,
	// allows an owned pre-bind session without history, and refuses missing bound
	// histories while query callers and incompatible databases remain read-only.
	// Owner: Manager recorder boundary; canonical suite: query_test.go.
	acpID := "acp-1"
	for _, tc := range []struct {
		name       string
		readErr    error
		wantWriter bool
	}{
		{"older schema", store.ErrSchemaBehind, true},
		{"ahead schema", store.ErrSchemaAhead, false},
		{"unreadable schema", errors.New("unreadable history"), false},
	} {
		t.Run("Should upgrade only "+tc.name, func(t *testing.T) {
			h := newHarness(t)
			sess := createSession(t, h)
			if err := h.manager.Stop(testutil.Context(t), sess.ID); err != nil {
				t.Fatal(err)
			}
			writerOpens := 0
			original := h.manager.openStore
			h.manager.openQueryStore = func(context.Context, store.SessionDBOwner, string) (EventReadCloser, error) { return nil, tc.readErr }
			h.manager.openStore = func(ctx context.Context, owner store.SessionDBOwner, path string) (EventRecorder, error) {
				writerOpens++
				return original(ctx, owner, path)
			}
			err := h.manager.UpgradeSessionDatabase(testutil.Context(t), sess.ID)
			if tc.wantWriter && (err != nil || writerOpens != 1) {
				t.Fatalf("upgrade=%v, writers=%d", err, writerOpens)
			}
			if !tc.wantWriter && (!errors.Is(err, tc.readErr) || writerOpens != 0) {
				t.Fatalf("refusal=%v, writers=%d", err, writerOpens)
			}
		})
	}

	t.Run("Should allow an accepted unbound session with no event database at boot", func(t *testing.T) {
		h := newHarness(t)
		accepted, err := h.manager.CreateAccepted(testutil.Context(t), CreateAcceptedOpts{
			Session: CreateOpts{AgentName: "coder", Workspace: h.workspaceID},
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := h.manager.UpgradeSessionDatabase(testutil.Context(t), accepted.ID); err != nil {
			t.Fatalf("UpgradeSessionDatabase(unbound) error = %v", err)
		}
	})

	for _, tc := range []struct {
		name       string
		status     store.SessionRuntimeStatus
		transition store.SessionRuntimeTransition
		acpID      *string
	}{
		{"bound session", store.SessionRuntimeReady, store.SessionRuntimeTransitionInitialBind, nil},
		{"previously bound unbound session", store.SessionRuntimeUnbound, store.SessionRuntimeTransitionInitialBind, nil},
		{"unbound session with ACP identity", store.SessionRuntimeUnbound, store.SessionRuntimeTransitionNone, &acpID},
	} {
		t.Run("Should refuse a missing event database for a "+tc.name, func(t *testing.T) {
			h := newHarness(t)
			dbPath := writeStoppedSessionArtifacts(t, h, "stored-no-db", false)
			metaPath := store.SessionMetaFile(filepath.Dir(dbPath))
			meta, err := store.ReadSessionMeta(metaPath)
			if err != nil {
				t.Fatal(err)
			}
			meta.RuntimeStatus = tc.status
			meta.RuntimeTransition = tc.transition
			meta.ACPSessionID = tc.acpID
			if err := store.WriteSessionMeta(metaPath, &meta); err != nil {
				t.Fatal(err)
			}
			if err := h.manager.UpgradeSessionDatabase(
				testutil.Context(t),
				meta.ID,
			); !errors.Is(
				err,
				ErrSessionNotFound,
			) {
				t.Fatalf("UpgradeSessionDatabase(%s without db) error = %v, want ErrSessionNotFound", tc.name, err)
			}
		})
	}

	t.Run("Should requires context and session id", func(t *testing.T) {
		h := newHarness(t)
		var nilCtx context.Context
		if _, _, err := h.manager.openQueryRecorder(nilCtx, "sess-1"); err == nil {
			t.Fatal("openQueryRecorder(nil, id) error = nil, want non-nil")
		}
		if _, _, err := h.manager.openQueryRecorder(testutil.Context(t), "   "); err == nil {
			t.Fatal("openQueryRecorder(ctx, blank) error = nil, want non-nil")
		}
	})

	t.Run("Should active session with nil recorder falls back to stored events", func(t *testing.T) {
		h := newHarness(t)
		session := createSession(t, h)
		t.Cleanup(func() {
			if session.recorderHandle() == nil {
				recorder, restoreErr := h.manager.openStore(
					testutil.Context(t),
					testSessionDBOwner(session.ID, session.WorkspaceID),
					session.DBPath(),
				)
				if restoreErr != nil {
					t.Errorf("restore recorder for Stop(%q) cleanup error = %v", session.ID, restoreErr)
					return
				}
				session.setRecorder(recorder)
			}
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil &&
				!errors.Is(err, ErrSessionNotFound) {
				t.Errorf("Stop(%q) error = %v", session.ID, err)
			}
		})

		eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "hello")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		if events := collectEvents(t, eventsCh); len(events) == 0 {
			t.Fatal("Prompt() returned no events, want stored transcript data")
		}
		recorder := session.recorderHandle()
		if recorder == nil {
			t.Fatal("recorderHandle() = nil, want active recorder")
		}
		if err := recorder.Close(testutil.Context(t)); err != nil {
			t.Fatalf("Close(recorder) error = %v", err)
		}
		session.setRecorder(nil)
		queried, err := h.manager.Events(testutil.Context(t), session.ID, store.EventQuery{})
		if err != nil {
			t.Fatalf("Events(active with nil recorder) error = %v", err)
		}
		if len(queried) == 0 {
			t.Fatal("Events(active with nil recorder) = 0, want persisted events")
		}
		history, err := h.manager.History(testutil.Context(t), session.ID, store.EventQuery{})
		if err != nil {
			t.Fatalf("History(active with nil recorder) error = %v", err)
		}
		if len(history) == 0 {
			t.Fatal("History(active with nil recorder) = 0, want persisted turns")
		}
		page, err := h.manager.TranscriptPage(testutil.Context(t), session.ID, transcript.PageQuery{})
		if err != nil {
			t.Fatalf("TranscriptPage(active with nil recorder) error = %v", err)
		}
		if len(page.Entries) == 0 {
			t.Fatal("TranscriptPage(active with nil recorder) = 0, want persisted entries")
		}
	})

	t.Run("Should wait for finalization before returning active recorder", func(t *testing.T) {
		h := newHarness(t)
		session := createSession(t, h)
		done := make(chan struct{})
		h.manager.mu.Lock()
		h.manager.finalizing[session.ID] = &sessionFinalization{done: done}
		h.manager.mu.Unlock()
		t.Cleanup(func() {
			h.manager.finishFinalization(session.ID, nil)
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil &&
				!errors.Is(err, ErrSessionNotFound) {
				t.Errorf("Stop(%q) error = %v", session.ID, err)
			}
		})

		ctx, cancel := context.WithCancel(testutil.Context(t))
		cancel()
		if _, _, err := h.manager.openQueryRecorder(ctx, session.ID); !errors.Is(err, context.Canceled) {
			t.Fatalf("openQueryRecorder(finalizing canceled ctx) error = %v, want context.Canceled", err)
		}
	})

	t.Run("Should wait for finalization before reading a closed recorder handle", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)

		eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "hello")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		runtimeEvents := collectEvents(t, eventsCh)
		if len(runtimeEvents) == 0 {
			t.Fatal("Prompt() returned no runtime events, want recorded prompt events")
		}

		activeEvents, err := h.manager.Events(testutil.Context(t), session.ID, store.EventQuery{})
		if err != nil {
			t.Fatalf("Events(active) error = %v", err)
		}
		if len(activeEvents) == 0 {
			t.Fatal("Events(active) returned no events, want persisted session events")
		}

		recorder := session.recorderHandle()
		if recorder == nil {
			t.Fatal("recorderHandle() = nil, want active recorder")
		}
		if err := recorder.Close(testutil.Context(t)); err != nil {
			t.Fatalf("Close(active recorder) error = %v", err)
		}

		done := make(chan struct{})
		h.manager.mu.Lock()
		h.manager.finalizing[session.ID] = &sessionFinalization{done: done}
		h.manager.mu.Unlock()

		type queryResult struct {
			events []store.SessionEvent
			err    error
		}
		started := make(chan struct{})
		resultCh := make(chan queryResult, 1)
		ctx, cancel := context.WithTimeout(testutil.Context(t), 5*time.Second)
		defer cancel()
		go func() {
			close(started)
			got, cleanup, err := h.manager.openQueryRecorder(ctx, session.ID)
			if err != nil {
				resultCh <- queryResult{err: err}
				return
			}
			events, err := got.Query(ctx, store.EventQuery{})
			if cleanupErr := cleanup(); cleanupErr != nil && err == nil {
				err = cleanupErr
			}
			resultCh <- queryResult{events: events, err: err}
		}()

		<-started
		select {
		case result := <-resultCh:
			t.Fatalf(
				"openQueryRecorder returned before finalization finished: events=%d err=%v",
				len(result.events),
				result.err,
			)
		case <-time.After(50 * time.Millisecond):
		}

		now := time.Now().UTC()
		if err := session.beginStopping(now); err != nil {
			t.Fatalf("beginStopping() error = %v", err)
		}
		if err := session.markStopped(now); err != nil {
			t.Fatalf("markStopped() error = %v", err)
		}
		if err := h.manager.persistSessionMetadataOnly(session); err != nil {
			t.Fatalf("writeMeta(stopped) error = %v", err)
		}
		h.manager.removeActive(session.ID)
		h.manager.finishFinalization(session.ID, nil)

		result := <-resultCh
		if result.err != nil {
			t.Fatalf("openQueryRecorder(finalized active) error = %v", result.err)
		}
		if err := compareQueriedSessionEvents(activeEvents, result.events); err != nil {
			t.Fatalf("openQueryRecorder(finalized active) events mismatch: %v", err)
		}
	})

	t.Run("Should missing session metadata", func(t *testing.T) {
		h := newHarness(t)
		if _, _, err := h.manager.openQueryRecorder(
			testutil.Context(t),
			"missing",
		); !errors.Is(
			err,
			ErrSessionNotFound,
		) {
			t.Fatalf("openQueryRecorder(missing) error = %v, want ErrSessionNotFound", err)
		}
	})

	t.Run("Should missing database file", func(t *testing.T) {
		h := newHarness(t)
		writeStoppedSessionArtifacts(t, h, "stored-no-db", false)

		if _, _, err := h.manager.openQueryRecorder(
			testutil.Context(t),
			"stored-no-db",
		); !errors.Is(
			err,
			ErrSessionNotFound,
		) {
			t.Fatalf("openQueryRecorder(no db) error = %v, want ErrSessionNotFound", err)
		}
	})

	t.Run("Should store open failure", func(t *testing.T) {
		openErr := errors.New("boom")
		h := newHarness(t, WithStore(func(context.Context, store.SessionDBOwner, string) (EventRecorder, error) {
			return nil, openErr
		}))
		writeStoppedSessionArtifacts(t, h, "stored-open-failure", true)

		if _, _, err := h.manager.openQueryRecorder(
			testutil.Context(t),
			"stored-open-failure",
		); !errors.Is(
			err,
			openErr,
		) {
			t.Fatalf("openQueryRecorder(open failure) error = %v, want wrapped %v", err, openErr)
		}
	})

	t.Run("Should hide query pool quiescence behind session absence", func(t *testing.T) {
		h := newHarness(t, WithQueryStore(func(context.Context, store.SessionDBOwner, string) (EventReadCloser, error) {
			return nil, sessiondb.ErrReadOnlyPoolQuiescing
		}))
		writeStoppedSessionArtifacts(t, h, "stored-quiescing", true)

		if _, _, err := h.manager.openQueryRecorder(
			testutil.Context(t),
			"stored-quiescing",
		); !errors.Is(err, ErrSessionNotFound) {
			t.Fatalf("openQueryRecorder(quiescing) error = %v, want ErrSessionNotFound", err)
		}
	})

	t.Run("Should cleanup closes reopened recorder", func(t *testing.T) {
		recorder := &queryRecorderStub{}
		h := newHarness(t, WithStore(func(context.Context, store.SessionDBOwner, string) (EventRecorder, error) {
			return recorder, nil
		}))
		writeStoppedSessionArtifacts(t, h, "stored-cleanup", true)

		got, cleanup, err := h.manager.openQueryRecorder(testutil.Context(t), "stored-cleanup")
		if err != nil {
			t.Fatalf("openQueryRecorder(cleanup) error = %v", err)
		}
		if got != recorder {
			t.Fatalf("openQueryRecorder(cleanup) recorder = %T, want queryRecorderStub", got)
		}
		if cleanup == nil {
			t.Fatal("openQueryRecorder(cleanup) cleanup = nil, want non-nil")
		}
		if err := cleanup(); err != nil {
			t.Fatalf("cleanup() error = %v", err)
		}
		if recorder.closeCalls != 1 {
			t.Fatalf("cleanup() close calls = %d, want 1", recorder.closeCalls)
		}
	})
}

func TestReadMetaAndQueryHelpers(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	if _, err := h.manager.readMetaWithContext(testutil.Context(t), "   "); err == nil {
		t.Fatal("readMeta(blank) error = nil, want non-nil")
	}
	if _, err := h.manager.readMetaWithContext(testutil.Context(t), "missing"); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("readMetaWithContext(missing) error = %v, want ErrSessionNotFound", err)
	}

	invalidDir := filepath.Join(h.homePaths.SessionsDir, "invalid")
	if err := os.MkdirAll(invalidDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(invalid) error = %v", err)
	}
	if err := os.WriteFile(store.SessionMetaFile(invalidDir), []byte("{"), 0o644); err != nil {
		t.Fatalf("WriteFile(invalid meta) error = %v", err)
	}
	if _, err := h.manager.readMetaWithContext(testutil.Context(t), "invalid"); err == nil {
		t.Fatal("readMetaWithContext(invalid) error = nil, want non-nil")
	}

	acpID := "  acp-123  "
	stopReason := store.StopTimeout
	createdAt := time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Minute)
	info := sessionInfoFromMeta(&store.SessionMeta{
		ID:              "sess-1",
		Name:            "stored",
		AgentName:       "coder",
		Provider:        "codex",
		Model:           "  gpt-4o  ",
		ReasoningEffort: "  high  ",
		WorkspaceID:     "ws-1",
		CreationOptions: &store.SessionCreationOptions{},
		State:           string(StateStopped),
		RuntimeStatus:   store.SessionRuntimeReady,
		StopReason:      &stopReason,
		StopDetail:      "deadline exceeded",
		ACPSessionID:    &acpID,
		CreatedAt:       createdAt,
		UpdatedAt:       updatedAt,
	})
	if got := info.ACPSessionID; got != "acp-123" {
		t.Fatalf("sessionInfoFromMeta().ACPSessionID = %q, want %q", got, "acp-123")
	}
	if got := info.Provider; got != "codex" {
		t.Fatalf("sessionInfoFromMeta().Provider = %q, want %q", got, "codex")
	}
	if got := info.Model; got != "gpt-4o" {
		t.Fatalf("sessionInfoFromMeta().Model = %q, want %q", got, "gpt-4o")
	}
	if got := info.ReasoningEffort; got != "high" {
		t.Fatalf("sessionInfoFromMeta().ReasoningEffort = %q, want %q", got, "high")
	}
	if got := info.Speed; got != speedpkg.SpeedNormal {
		t.Fatalf("sessionInfoFromMeta().Speed = %q, want %q for legacy metadata", got, speedpkg.SpeedNormal)
	}
	if got := info.State; got != StateStopped {
		t.Fatalf("sessionInfoFromMeta().State = %q, want %q", got, StateStopped)
	}
	if got := info.Type; got != SessionTypeUser {
		t.Fatalf("sessionInfoFromMeta().Type = %q, want %q", got, SessionTypeUser)
	}
	if got := info.StopReason; got != store.StopTimeout {
		t.Fatalf("sessionInfoFromMeta().StopReason = %q, want %q", got, store.StopTimeout)
	}
	if got := info.StopDetail; got != "deadline exceeded" {
		t.Fatalf("sessionInfoFromMeta().StopDetail = %q, want %q", got, "deadline exceeded")
	}

	t.Run("Should keep stop fields empty when omitted", func(t *testing.T) {
		infoWithoutStop := sessionInfoFromMeta(&store.SessionMeta{
			ID:            "sess-legacy",
			AgentName:     "coder",
			WorkspaceID:   "ws-1",
			State:         string(StateStopped),
			RuntimeStatus: store.SessionRuntimeReady,
			CreatedAt:     createdAt,
			UpdatedAt:     updatedAt,
		})
		if got := infoWithoutStop.StopReason; got != "" {
			t.Fatalf("sessionInfoFromMeta().StopReason = %q, want empty", got)
		}
		if got := infoWithoutStop.StopDetail; got != "" {
			t.Fatalf("sessionInfoFromMeta().StopDetail = %q, want empty", got)
		}
	})

	sameTime := time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
	sorted := sortSessionInfos([]*Info{
		{ID: "b", CreatedAt: sameTime},
		nil,
		{ID: "a", CreatedAt: sameTime},
		{ID: "c", CreatedAt: sameTime.Add(time.Minute)},
	})
	if got := []string{sorted[0].ID, sorted[1].ID, sorted[2].ID}; strings.Join(got, ",") != "a,b,c" {
		t.Fatalf("sortSessionInfos() ids = %v, want [a b c]", got)
	}

	if got := stringValue(nil); got != "" {
		t.Fatalf("stringValue(nil) = %q, want empty", got)
	}
	if got := stringValue(&acpID); got != "acp-123" {
		t.Fatalf("stringValue(channeld) = %q, want %q", got, "acp-123")
	}
}

func compareQueriedSessionEvents(want []store.SessionEvent, got []store.SessionEvent) error {
	if len(got) != len(want) {
		return fmt.Errorf("count = %d, want %d", len(got), len(want))
	}
	for index := range want {
		if got[index].ID != want[index].ID {
			return fmt.Errorf("event[%d].id = %q, want %q", index, got[index].ID, want[index].ID)
		}
		if got[index].Sequence != want[index].Sequence {
			return fmt.Errorf(
				"event[%d].sequence = %d, want %d",
				index,
				got[index].Sequence,
				want[index].Sequence,
			)
		}
		if got[index].SessionID != want[index].SessionID {
			return fmt.Errorf("event[%d].session_id = %q, want %q", index, got[index].SessionID, want[index].SessionID)
		}
		if got[index].TurnID != want[index].TurnID {
			return fmt.Errorf("event[%d].turn_id = %q, want %q", index, got[index].TurnID, want[index].TurnID)
		}
		if got[index].Type != want[index].Type {
			return fmt.Errorf("event[%d].type = %q, want %q", index, got[index].Type, want[index].Type)
		}
		if got[index].AgentName != want[index].AgentName {
			return fmt.Errorf(
				"event[%d].agent_name = %q, want %q",
				index,
				got[index].AgentName,
				want[index].AgentName,
			)
		}
		if got[index].Content != want[index].Content {
			return fmt.Errorf("event[%d].content = %q, want %q", index, got[index].Content, want[index].Content)
		}
		if !got[index].Timestamp.Equal(want[index].Timestamp) {
			return fmt.Errorf(
				"event[%d].timestamp = %s, want %s",
				index,
				got[index].Timestamp.UTC().Format(time.RFC3339Nano),
				want[index].Timestamp.UTC().Format(time.RFC3339Nano),
			)
		}
	}
	return nil
}

func TestNewAgentProcessDefaultsAndNotifierNoop(t *testing.T) {
	t.Parallel()

	args := []string{"--json"}
	caps := acp.Caps{ConfigOptions: []acp.SessionConfigOption{{
		ID: "model", Values: []acp.SessionConfigOptionValue{{Value: "gpt-5.6"}},
	}}}
	proc := NewAgentProcess(AgentProcessOptions{
		PID:       42,
		AgentName: "coder",
		Args:      args,
		Caps:      caps,
	})
	args[0] = "--changed"
	caps.ConfigOptions[0].Values[0].Value = "mutated input"

	select {
	case <-proc.Done():
	default:
		t.Fatal("Done() channel is not closed by default")
	}
	if err := proc.Wait(); err != nil {
		t.Fatalf("Wait() error = %v, want nil", err)
	}
	if got := proc.Stderr(); got != "" {
		t.Fatalf("Stderr() = %q, want empty", got)
	}
	if got := proc.Args[0]; got != "--json" {
		t.Fatalf("NewAgentProcess() copied args = %q, want %q", got, "--json")
	}
	firstCaps := proc.CapsSnapshot()
	firstCaps.ConfigOptions[0].Values[0].Value = "mutated output"
	if got := proc.CapsSnapshot().ConfigOptions[0].Values[0].Value; got != "gpt-5.6" {
		t.Fatalf("CapsSnapshot() value = %q, want isolated gpt-5.6", got)
	}
}

func writeStoppedSessionArtifacts(t *testing.T, h *harness, id string, withDB bool) string {
	t.Helper()

	sessionDir := filepath.Join(h.homePaths.SessionsDir, id)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", sessionDir, err)
	}

	now := time.Date(2026, 4, 3, 11, 0, 0, 0, time.UTC)
	if err := store.WriteSessionMeta(store.SessionMetaFile(sessionDir), &store.SessionMeta{
		ID:            id,
		Name:          "stored",
		AgentName:     "coder",
		WorkspaceID:   h.workspaceID,
		State:         string(StateStopped),
		RuntimeStatus: store.SessionRuntimeReady,
		CreatedAt:     now,
		UpdatedAt:     now,
	}); err != nil {
		t.Fatalf("WriteSessionMeta(%q) error = %v", id, err)
	}

	dbPath := store.SessionDBFile(sessionDir)
	if withDB {
		if err := os.WriteFile(dbPath, nil, 0o644); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", dbPath, err)
		}
	}

	return dbPath
}

func createEscapedStoredSession(t *testing.T, h *harness) string {
	t.Helper()

	escapedID := "escaped-session"
	escapedDir := filepath.Join(filepath.Dir(h.homePaths.SessionsDir), escapedID)
	if err := os.MkdirAll(escapedDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", escapedDir, err)
	}

	now := time.Now().UTC()
	if err := store.WriteSessionMeta(store.SessionMetaFile(escapedDir), &store.SessionMeta{
		ID:            escapedID,
		Name:          "escaped",
		AgentName:     "coder",
		WorkspaceID:   h.workspaceID,
		SessionType:   string(SessionTypeUser),
		State:         string(StateStopped),
		RuntimeStatus: store.SessionRuntimeReady,
		CreatedAt:     now.Add(-time.Minute),
		UpdatedAt:     now,
	}); err != nil {
		t.Fatalf("WriteSessionMeta(%q) error = %v", escapedDir, err)
	}

	recorder, err := h.manager.openStore(
		testutil.Context(t),
		testSessionDBOwner(escapedID, h.workspaceID),
		store.SessionDBFile(escapedDir),
	)
	if err != nil {
		t.Fatalf("openStore(%q) error = %v", escapedID, err)
	}
	t.Cleanup(func() {
		if err := recorder.Close(testutil.Context(t)); err != nil {
			t.Errorf("Close(recorder) error = %v", err)
		}
	})

	if err := recorder.Record(testutil.Context(t), store.SessionEvent{
		TurnID:    "turn-1",
		Type:      acp.EventTypeAgentMessage,
		AgentName: "coder",
		Content:   `{"type":"agent_message","text":"escaped"}`,
		Timestamp: now,
	}); err != nil {
		t.Fatalf("Record() error = %v", err)
	}

	return escapedID
}

type queryRecorderStub struct {
	events        []store.SessionEvent
	history       []store.TurnHistory
	usage         []store.TokenUsage
	queryErr      error
	queryErrs     []error
	metadataErrs  []error
	metadataCalls []store.EventQuery
	historyErr    error
	historyErrs   []error
	usageErrs     []error
	closeCalls    int
	queryCalls    []store.EventQuery
	historyCall   []store.EventQuery
	usageCalls    int
	appendErrs    []error
	appendCalls   int
	onAppend      func()
}

func (s *queryRecorderStub) ListTokenUsage(context.Context) ([]store.TokenUsage, error) {
	s.usageCalls++
	if len(s.usageErrs) > 0 {
		err := s.usageErrs[0]
		s.usageErrs = s.usageErrs[1:]
		if err != nil {
			return nil, err
		}
	}
	return append([]store.TokenUsage(nil), s.usage...), nil
}

func (s *queryRecorderStub) Record(context.Context, store.SessionEvent) error {
	return nil
}

func (s *queryRecorderStub) RecordTokenUsage(context.Context, store.TokenUsage) error {
	return nil
}

func (s *queryRecorderStub) AppendEventIfAbsent(
	_ context.Context,
	event store.SessionEvent,
) (store.SessionEvent, bool, error) {
	s.appendCalls++
	if s.onAppend != nil {
		s.onAppend()
	}
	if len(s.appendErrs) > 0 {
		err := s.appendErrs[0]
		s.appendErrs = s.appendErrs[1:]
		if err != nil {
			return store.SessionEvent{}, false, err
		}
	}
	return event, true, nil
}

func (s *queryRecorderStub) Query(_ context.Context, query store.EventQuery) ([]store.SessionEvent, error) {
	s.queryCalls = append(s.queryCalls, query)
	if len(s.queryErrs) > 0 {
		err := s.queryErrs[0]
		s.queryErrs = s.queryErrs[1:]
		if err != nil {
			return nil, err
		}
	}
	if s.queryErr != nil {
		return nil, s.queryErr
	}
	return append([]store.SessionEvent(nil), s.events...), nil
}

func (s *queryRecorderStub) QueryEventMetadata(
	_ context.Context,
	query store.EventQuery,
) ([]store.EventMetadata, error) {
	s.metadataCalls = append(s.metadataCalls, query)
	if len(s.metadataErrs) > 0 {
		err := s.metadataErrs[0]
		s.metadataErrs = s.metadataErrs[1:]
		if err != nil {
			return nil, err
		}
	}
	metadata := make([]store.EventMetadata, 0, len(s.events))
	for _, event := range s.events {
		metadata = append(metadata, store.EventMetadata{
			ID:        event.ID,
			SessionID: event.SessionID,
			Sequence:  event.Sequence,
			TurnID:    event.TurnID,
			Type:      event.Type,
			AgentName: event.AgentName,
			Timestamp: event.Timestamp,
		})
	}
	return metadata, nil
}

func (s *queryRecorderStub) MaxEventSequence(context.Context) (int64, error) {
	var maximum int64
	for _, event := range s.events {
		maximum = max(maximum, event.Sequence)
	}
	return maximum, nil
}

func (s *queryRecorderStub) History(_ context.Context, query store.EventQuery) ([]store.TurnHistory, error) {
	s.historyCall = append(s.historyCall, query)
	if len(s.historyErrs) > 0 {
		err := s.historyErrs[0]
		s.historyErrs = s.historyErrs[1:]
		if err != nil {
			return nil, err
		}
	}
	if s.historyErr != nil {
		return nil, s.historyErr
	}
	return append([]store.TurnHistory(nil), s.history...), nil
}

func (s *queryRecorderStub) Close(context.Context) error {
	s.closeCalls++
	return nil
}

func TestManagerUnreadableMetadataWarnings(t *testing.T) {
	t.Parallel()
	t.Run("Should bound repeated list warnings and report changed failures immediately", func(t *testing.T) {
		t.Parallel()
		var logs bytes.Buffer
		now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
		h := newHarness(
			t,
			WithLogger(slog.New(slog.NewJSONHandler(&logs, nil))),
			WithNow(func() time.Time { return now }),
		)
		for i := range 8 {
			dir := filepath.Join(h.homePaths.SessionsDir, fmt.Sprintf("bad-%d", i))
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(store.SessionMetaFile(dir), []byte("{"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		list := func() {
			t.Helper()
			infos, err := h.manager.ListAll(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if len(infos) != 0 {
				t.Fatal("unreadable sessions returned")
			}
		}
		for range 12 {
			list()
		}
		if count := strings.Count(logs.String(), "session: skip unreadable session metadata"); count != 1 {
			t.Fatalf("warnings = %d, want 1", count)
		}
		checked, failures, err := h.manager.SessionMetadataHealth(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if checked != 8 || failures.Count != 8 || len(failures.Samples) != 5 {
			t.Fatalf("health = %d, %#v", checked, failures)
		}
		if err := os.WriteFile(
			store.SessionMetaFile(filepath.Join(h.homePaths.SessionsDir, "bad-7")),
			[]byte("[]"),
			0600,
		); err != nil {
			t.Fatal(err)
		}
		list()
		if count := strings.Count(logs.String(), "session: skip unreadable session metadata"); count != 2 {
			t.Fatalf("new failure warnings = %d, want 2", count)
		}
		now = now.Add(5 * time.Minute)
		list()
		if count := strings.Count(logs.String(), "session: skip unreadable session metadata"); count != 3 {
			t.Fatalf("backoff warnings = %d, want 3", count)
		}
	})
}

func TestSessionWorkspaceFilter(t *testing.T) {
	t.Run("Should normalize selection and bind cursor identity to the full workspace set", func(t *testing.T) {
		t.Parallel()
		query := ListQuery{
			ReadScope:     store.ReadScope{AllProfiles: true},
			AllWorkspaces: true,
			WorkspaceIDs:  []string{" ws-a ", workspaceaccess.UserSelector, "ws-a"},
		}
		normalized, err := normalizeListQuery(query)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(normalized.WorkspaceIDs, []string{workspaceaccess.UserSelector, "ws-a"}) {
			t.Fatalf("selection = %v", normalized.WorkspaceIDs)
		}
		fingerprint, err := sessionListFingerprintForQuery(normalized)
		if err != nil {
			t.Fatal(err)
		}
		normalized.WorkspaceIDs = []string{"ws-b"}
		other, err := sessionListFingerprintForQuery(normalized)
		if err != nil {
			t.Fatal(err)
		}
		if fingerprint == other {
			t.Fatal("different selections share cursor identity")
		}
		for _, id := range []string{"", "ws-a"} {
			if !sessionMatchesWorkspaceIDs(id, []string{workspaceaccess.UserSelector, "ws-a"}) {
				t.Fatalf("excluded %q", id)
			}
		}
		if sessionMatchesWorkspaceIDs("ws-b", []string{workspaceaccess.UserSelector, "ws-a"}) {
			t.Fatal("included another project")
		}
	})
	for _, values := range [][]string{{""}, {"global"}} {
		t.Run("Should reject reserved or empty filter "+strings.Join(values, ","), func(t *testing.T) {
			t.Parallel()
			_, err := NormalizeWorkspaceIDs(values, true)
			if !errors.Is(err, ErrListQueryInvalid) {
				t.Fatalf("error = %v", err)
			}
		})
	}
	t.Run("Should reject a filter without aggregate breadth", func(t *testing.T) {
		t.Parallel()
		_, err := NormalizeWorkspaceIDs([]string{"ws-a"}, false)
		if !errors.Is(err, ErrListQueryInvalid) {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestSessionInfoFromCatalogKeepsEffectiveWorkspaceAccess(t *testing.T) {
	t.Parallel()
	t.Run("Should project the persisted grant for a session that is not loaded", func(t *testing.T) {
		t.Parallel()
		info := sessionInfoFromCatalog(&store.SessionInfo{ID: "sess-granted", EffectiveWorkspaceAccess: "unrestricted"})
		if info.EffectiveWorkspaceAccess != "unrestricted" {
			t.Fatalf(
				"EffectiveWorkspaceAccess = %q, want the persisted unrestricted grant (USW BR-13)",
				info.EffectiveWorkspaceAccess,
			)
		}
	})
}

// familyArchiveCatalog refuses an archive while a declared family member is
// live, naming that member the way the durable family archive does.
type familyArchiveCatalog struct {
	*pagedRecordingSessionCatalog
	members map[string][]string
}

func (c *familyArchiveCatalog) setMembers(parent string, members ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.members[parent] = members
}

func (c *familyArchiveCatalog) SetSessionArchived(
	ctx context.Context,
	workspaceID string,
	sessionID string,
	archived bool,
) (store.SessionInfo, error) {
	if archived {
		c.mu.Lock()
		for _, member := range c.members[sessionID] {
			if c.sessions[member].State != string(StateStopped) {
				c.mu.Unlock()
				return store.SessionInfo{}, &store.SessionArchiveLiveMemberError{SessionID: member}
			}
		}
		c.mu.Unlock()
	}
	return c.pagedRecordingSessionCatalog.SetSessionArchived(ctx, workspaceID, sessionID, archived)
}

func TestManagerListAllMetadataCache(t *testing.T) {
	t.Parallel()
	t.Run("Should skip metadata removed during reading without an unreadable warning", func(t *testing.T) {
		t.Parallel()
		manager := newWorkspaceSessionReadFixture(t, 1, 1)
		var logs bytes.Buffer
		manager.logger = slog.New(slog.NewJSONHandler(&logs, nil))
		manager.readSessionMeta = func(path string) (store.SessionMeta, error) {
			meta, err := store.ReadSessionMeta(path)
			if err != nil {
				return meta, err
			}
			return meta, os.Remove(path)
		}
		infos, err := manager.ListAll(t.Context())
		if err != nil || len(infos) != 0 {
			t.Fatalf("removed inventory = %#v, error = %v", infos, err)
		}
		if strings.Contains(logs.String(), "session: skip unreadable session metadata") {
			t.Fatalf("removed metadata reported as unreadable: %s", logs.String())
		}
	})
	t.Run("Should preserve fresh scoped metadata after replacement deletion and caller mutation", func(t *testing.T) {
		t.Parallel()
		manager := newWorkspaceSessionReadFixture(t, 4, 2)
		reads := 0
		manager.readSessionMeta = func(path string) (store.SessionMeta, error) {
			reads++
			return store.ReadSessionMeta(path)
		}
		first, err := manager.ListAll(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		first[0].Name = "caller mutation"
		second, err := manager.ListAll(t.Context())
		if err != nil || len(second) != 4 || reads != 4 || second[0].Name == first[0].Name {
			t.Fatalf("cached inventory: %#v reads=%d error=%v", second, reads, err)
		}
		path := store.SessionMetaFile(filepath.Join(manager.homePaths.SessionsDir, second[0].ID))
		meta, err := store.ReadSessionMeta(path)
		if err != nil {
			t.Fatal(err)
		}
		meta.Name = "replacement"
		if err := store.WriteSessionMeta(path, &meta); err != nil {
			t.Fatal(err)
		}
		third, err := manager.ListAll(t.Context())
		if err != nil || len(third) != 4 || reads != 5 || third[0].Name != "replacement" {
			t.Fatalf("replaced inventory: %#v reads=%d error=%v", third, reads, err)
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		fourth, err := manager.ListAll(t.Context())
		if err != nil || len(fourth) != 3 || fourth[0].ID == second[0].ID {
			t.Fatalf("deleted inventory: %#v %v", fourth, err)
		}
	})
	t.Run("Should refresh catalog epoch without rereading unchanged metadata", func(t *testing.T) {
		t.Parallel()
		manager := newWorkspaceSessionReadFixture(t, 1, 1)
		if _, err := manager.ListAll(t.Context()); err != nil {
			t.Fatal(err)
		}
		manager.readSessionMeta = func(string) (store.SessionMeta, error) {
			t.Fatal("unchanged metadata reread")
			return store.SessionMeta{}, nil
		}
		epoch, err := manager.transcriptEpochStore.EnsureSessionTranscriptEpoch(
			t.Context(), store.SessionTranscriptEpochUpdate{SessionID: "sess-read-000", Minimum: 7},
		)
		if err != nil {
			t.Fatal(err)
		}
		infos, err := manager.ListAll(t.Context())
		if err != nil || len(infos) != 1 || infos[0].TranscriptEpoch != epoch {
			t.Fatalf("fresh epoch: %#v %v", infos, err)
		}
	})
	t.Run("Should leave interrupted metadata unchanged until explicit reconciliation", func(t *testing.T) {
		t.Parallel()
		manager := newWorkspaceSessionReadFixture(t, 1, 1)
		path := store.SessionMetaFile(filepath.Join(manager.homePaths.SessionsDir, "sess-read-000"))
		meta, err := store.ReadSessionMeta(path)
		if err != nil {
			t.Fatal(err)
		}
		meta.State, meta.StopReason = string(StateStarting), nil
		if err := store.WriteSessionMeta(path, &meta); err != nil {
			t.Fatal(err)
		}
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for range 2 {
			infos, err := manager.ListAll(t.Context())
			if err != nil || len(infos) != 1 || infos[0].State != StateStarting {
				t.Fatalf("read: %#v %v", infos, err)
			}
			if _, err := manager.Status(t.Context(), meta.ID); err != nil {
				t.Fatal(err)
			}
		}
		after, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(before, after) {
			t.Fatalf("read changed metadata: %v", err)
		}
		rows, err := manager.sessionCatalog.ListSessions(t.Context(), store.SessionListQuery{
			ReadScope: store.ReadScope{AllProfiles: true},
		})
		if err != nil || len(rows) != 1 || rows[0].State != string(StateStopped) {
			t.Fatalf("read changed catalog: %#v %v", rows, err)
		}
		if err := manager.ReconcileInactiveSessions(t.Context()); err != nil {
			t.Fatal(err)
		}
		infos, err := manager.ListAll(t.Context())
		if err != nil || len(infos) != 1 || infos[0].State != StateStopping {
			t.Fatalf("reconciled: %#v %v", infos, err)
		}
	})
}

func TestManagerListLive(t *testing.T) {
	t.Parallel()
	t.Run("Should include retained live sessions and exclude stopped history without parsing it", func(t *testing.T) {
		t.Parallel()
		manager := newWorkspaceSessionReadFixture(t, 279, 1)
		id := "sess-read-278"
		path := store.SessionMetaFile(filepath.Join(manager.homePaths.SessionsDir, id))
		meta, err := store.ReadSessionMeta(path)
		if err != nil {
			t.Fatal(err)
		}
		meta.State = string(StateActive)
		if err := store.WriteSessionMeta(path, &meta); err != nil {
			t.Fatal(err)
		}
		if err := manager.sessionCatalog.UpdateSessionState(t.Context(), store.SessionStateUpdate{
			ID: id, State: string(StateActive),
		}); err != nil {
			t.Fatal(err)
		}
		reads := 0
		manager.readSessionMeta = func(path string) (store.SessionMeta, error) {
			reads++
			return store.ReadSessionMeta(path)
		}
		infos, err := manager.ListLive(t.Context())
		if err != nil || len(infos) != 1 || infos[0].ID != id || reads != 1 {
			t.Fatalf("live inventory=%#v reads=%d error=%v", infos, reads, err)
		}
	})
}

// Invariant: a bounded page resolves branches once and deduplicates shared PR lookups.
// Owner: session read-model projection; canonical query suite.
func TestSessionPullRequestProjection(t *testing.T) {
	t.Parallel()
	reader := &pullRequestQueryReader{}
	branches := &pullRequestQueryBranches{values: map[string]string{"one": "topic", "two": "topic", "default": "main"}}
	manager := &Manager{}
	manager.ConfigurePullRequests(reader, branches)
	infos := []*Info{
		{ID: "one", WorkspaceID: "ws"},
		{ID: "two", WorkspaceID: "ws"},
		{ID: "default", WorkspaceID: "ws"},
		{ID: "none", WorkspaceID: "ws"},
	}
	got, err := manager.SessionPullRequests(t.Context(), infos)
	if err != nil || branches.calls != 1 || reader.calls != 1 || len(reader.keys) != 1 || len(got) != 2 ||
		got["one"].Branch != "topic" ||
		got["two"].PullRequest.Number != 42 {
		t.Fatalf(
			"projection=%#v %v branchReads=%d PRReads=%d keys=%#v",
			got,
			err,
			branches.calls,
			reader.calls,
			reader.keys,
		)
	}
}

type pullRequestQueryBranches struct {
	ids    []string
	values map[string]string
	calls  int
}

func (b *pullRequestQueryBranches) SessionBranches(_ context.Context, ids []string) (map[string]string, error) {
	b.calls++
	b.ids = ids
	return b.values, nil
}

type pullRequestQueryReader struct {
	state        PullRequestState
	repositories map[string]string
	calls        int
	keys         []BranchKey
}

func (r *pullRequestQueryReader) PullRequests(_ context.Context, keys []BranchKey) map[BranchKey]PullRequestView {
	r.calls++
	r.keys = keys
	result := make(map[BranchKey]PullRequestView)
	for _, key := range keys {
		result[key] = PullRequestView{PullRequest: &PullRequest{Number: 42, State: r.state}}
	}
	return result
}
func (r *pullRequestQueryReader) DefaultBranches(_ context.Context, keys []BranchKey) map[BranchKey]bool {
	result := make(map[BranchKey]bool)
	for _, key := range keys {
		result[key] = key.Branch == "main"
	}
	return result
}

func (r *pullRequestQueryReader) RepositoryKeys(context.Context, store.ReadScope, []string) map[string]string {
	return r.repositories
}

func TestManagerBoardCatalog(t *testing.T) {
	t.Parallel()
	t.Run("Should discover old stopped PR branches outside the selected column and page", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		manager := newWorkspaceSessionReadFixture(t, 122, 2)
		catalog := manager.sessionCatalog.(*globaldb.GlobalDB)
		tracked := &catalogBranchReadCounter{GlobalDB: catalog}
		manager.sessionCatalog = tracked
		now := time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)
		manager.now = func() time.Time { return now }
		for _, value := range []struct{ workspace, id, branch string }{
			{"ws-read-00", "sess-read-000", "old-topic"},
			{"ws-read-00", "sess-read-002", "main"},
			{"ws-read-01", "sess-read-001", "foreign-topic"},
		} {
			if err := catalog.SaveSessionBranch(ctx, value.workspace, value.id, value.branch); err != nil {
				t.Fatal(err)
			}
		}
		const repo = "github.com/owner/repo"
		reader := &pullRequestQueryReader{repositories: map[string]string{"ws-read-00": repo}}
		manager.ConfigurePullRequests(reader, catalog)
		query := ListQuery{
			ReadScope:   store.ReadScope{ProfileID: store.DefaultProfileID},
			WorkspaceID: "ws-read-00",
			Columns:     []string{"in_review"},
			Limit:       DefaultListLimit,
		}
		page, err := manager.ListPage(ctx, query)
		if err != nil || page.Total != 0 {
			t.Fatalf("cold page = %#v, %v", page, err)
		}
		if !slices.Equal(reader.keys, []BranchKey{{WorkspaceID: "ws-read-00", Branch: "old-topic"}}) {
			t.Fatalf("refresh keys = %#v", reader.keys)
		}
		if err := catalog.Worktrees.SavePullRequest(
			ctx,
			worktreepkg.PullRequestEntry{
				Key:         worktreepkg.PullRequestKey{WorkspaceID: "ws-read-00", RepoKey: repo, Branch: "old-topic"},
				LookupState: "ok",
				Facts:       &worktreepkg.ForgeStatus{PRState: new("open"), PRNumber: new(42)},
			},
		); err != nil {
			t.Fatal(err)
		}
		page, err = manager.ListPage(ctx, query)
		if err != nil || page.Total != 1 || len(page.Sessions) != 1 || page.Sessions[0].ID != "sess-read-000" {
			t.Fatalf("refreshed page = %#v, %v", page, err)
		}
		reader.keys = nil
		if _, err := manager.CatalogFacets(ctx, query); err != nil {
			t.Fatal(err)
		}
		if len(reader.keys) != 0 || tracked.calls != 1 {
			t.Fatalf("repeated discovery: keys=%#v calls=%d", reader.keys, tracked.calls)
		}
		now = now.Add(time.Minute)
		if _, err := manager.CatalogFacets(ctx, query); err != nil {
			t.Fatal(err)
		}
		if tracked.calls != 2 ||
			!slices.Equal(reader.keys, []BranchKey{{WorkspaceID: "ws-read-00", Branch: "old-topic"}}) {
			t.Fatalf("expired discovery: keys=%#v calls=%d", reader.keys, tracked.calls)
		}
	})
	t.Run("Should overlay active PR columns and direct children before filtering and counting", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		h := newHarness(t)
		if err := h.manager.Shutdown(ctx); err != nil {
			t.Fatal(err)
		}
		catalog := openManagerInputQueueStore(t)
		registerManagerInputQueueWorkspace(t, catalog, h)
		h.manager = newManagerWithHarness(t, h, WithSessionCatalog(catalog))
		cleanupTestManager(t, h.manager)
		parent, err := h.manager.Create(ctx, CreateOpts{AgentName: "coder", Name: "Parent", Workspace: h.workspaceID})
		if err != nil {
			t.Fatal(err)
		}
		child, err := h.manager.Create(
			ctx,
			CreateOpts{
				AgentName: "coder",
				Name:      "Child",
				Workspace: h.workspaceID,
				Type:      SessionTypeSpawned,
				Lineage: &store.SessionLineage{
					ParentSessionID: parent.ID,
					RootSessionID:   parent.ID,
					SpawnDepth:      1,
					SpawnRole:       "worker",
					TTLExpiresAt:    new(time.Now().Add(time.Hour)),
				},
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		child.mu.Lock()
		child.supervisionState = &SupervisionState{WorkSignals: []WorkSignal{{Kind: WorkSignalActiveMonitor}}}
		child.mu.Unlock()
		durable := sessionCatalogInfoFromRuntime(child.Info())
		durable.ID, durable.Name, durable.State = "durable-child", "Durable child", string(StateStopped)
		if err := catalog.RegisterSession(ctx, durable); err != nil {
			t.Fatal(err)
		}
		for _, id := range []string{parent.ID, durable.ID} {
			if err := catalog.SaveSessionBranch(ctx, h.workspaceID, id, "topic"); err != nil {
				t.Fatal(err)
			}
		}
		const repo = "github.com/owner/repo"
		if err := catalog.Worktrees.SavePullRequest(ctx, worktreepkg.PullRequestEntry{
			Key: worktreepkg.PullRequestKey{
				WorkspaceID: h.workspaceID,
				RepoKey:     repo,
				Branch:      "topic",
			},
			LookupState: "ok",
			Facts:       &worktreepkg.ForgeStatus{PRState: new("open"), PRNumber: new(42)},
		}); err != nil {
			t.Fatal(err)
		}
		reader := &pullRequestQueryReader{state: PullRequestOpen, repositories: map[string]string{h.workspaceID: repo}}
		h.manager.ConfigurePullRequests(reader, catalog)
		query := ListQuery{
			ReadScope:    store.ReadScope{ProfileID: store.DefaultProfileID},
			WorkspaceID:  h.workspaceID,
			Columns:      []string{"in_review"},
			HideChildren: true,
		}
		page, err := h.manager.ListPage(ctx, query)
		if err != nil {
			t.Fatal(err)
		}
		if page.Total != 1 || len(page.Sessions) != 1 || page.Sessions[0].ID != parent.ID ||
			page.Sessions[0].BoardColumn != BoardInReview {
			t.Fatalf("root board = %#v", page)
		}
		if reader.calls != 1 {
			t.Fatalf("active projection calls = %d, want one batch", reader.calls)
		}
		counts := page.Sessions[0].Children
		// The monitoring child stays in Waiting despite its open pull request.
		if counts == nil || counts.Waiting != 1 || counts.Working != 0 || counts.InReview != 1 || counts.Done != 0 {
			t.Fatalf("children = %#v", counts)
		}
		facets, err := h.manager.CatalogFacets(ctx, query)
		if err != nil {
			t.Fatal(err)
		}
		if facets.Facets.Board.InReview != 1 || facets.Facets.All != 1 {
			t.Fatalf("root facets = %#v", facets)
		}
		query.HideChildren = false
		facets, err = h.manager.CatalogFacets(ctx, query)
		if err != nil {
			t.Fatal(err)
		}
		if facets.Facets.Board.InReview != 2 || facets.Facets.Board.Waiting != 1 ||
			facets.Facets.Board.Working != 0 || facets.Facets.All != 3 {
			t.Fatalf("all facets = %#v", facets)
		}
		query.Columns = []string{"waiting"}
		page, err = h.manager.ListPage(ctx, query)
		if err != nil {
			t.Fatal(err)
		}
		if page.Total != 1 || page.Sessions[0].ID != child.ID || page.Sessions[0].BoardColumn != BoardWaiting ||
			page.Sessions[0].ParentTitle != "Parent" {
			t.Fatalf("child board = %#v", page)
		}
		// Detail projects only direct children; the parent remains a title lookup.
		snapshots := &pullRequestQueryBranches{values: map[string]string{child.ID: "child-topic", parent.ID: "topic"}}
		h.manager.ConfigurePullRequests(reader, snapshots)
		detail := parent.Info()
		if err := h.manager.PopulateSessionRelations(ctx, detail); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(snapshots.ids, []string{child.ID}) {
			t.Fatalf("detail projection IDs=%v want only direct child", snapshots.ids)
		}
		if detail.Children == nil || *detail.Children != *counts {
			t.Fatalf("detail children = %#v", detail.Children)
		}
		childDetail := child.Info()
		if err := h.manager.PopulateSessionRelations(
			ctx,
			childDetail,
		); err != nil ||
			childDetail.ParentTitle != "Parent" {
			t.Fatalf("child detail parent=%q error=%v", childDetail.ParentTitle, err)
		}
		if len(snapshots.ids) != 0 {
			t.Fatalf("parent title lookup projected PRs: %v", snapshots.ids)
		}
		h.manager.ConfigurePullRequests(nil, nil)
		query.Columns = []string{"in_review"}
		page, err = h.manager.ListPage(ctx, query)
		if err != nil || page.Total != 0 {
			t.Fatalf("unconfigured reader = %#v, %v", page, err)
		}
	})
	t.Run("Should keep active forks top-level and out of the parent rollup", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		h := newHarness(t)
		if err := h.manager.Shutdown(ctx); err != nil {
			t.Fatal(err)
		}
		catalog := openManagerInputQueueStore(t)
		registerManagerInputQueueWorkspace(t, catalog, h)
		h.manager = newManagerWithHarness(t, h, WithSessionCatalog(catalog))
		cleanupTestManager(t, h.manager)
		parent, err := h.manager.Create(ctx, CreateOpts{AgentName: "coder", Name: "Parent", Workspace: h.workspaceID})
		if err != nil {
			t.Fatal(err)
		}
		child, err := h.manager.Create(ctx, CreateOpts{
			AgentName: "coder",
			Name:      "Child",
			Workspace: h.workspaceID,
			Lineage: &store.SessionLineage{
				ParentSessionID: parent.ID,
				RootSessionID:   parent.ID,
				SpawnDepth:      1,
				Kind:            store.LineageKindProvenance,
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		fork, err := h.manager.Create(ctx, CreateOpts{
			AgentName: "coder",
			Name:      "Fork",
			Workspace: h.workspaceID,
			Lineage: &store.SessionLineage{
				ParentSessionID: parent.ID,
				RootSessionID:   parent.ID,
				SpawnDepth:      1,
				Kind:            store.LineageKindFork,
				OriginAgentName: "coder",
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		query := ListQuery{
			ReadScope:    store.ReadScope{ProfileID: store.DefaultProfileID},
			WorkspaceID:  h.workspaceID,
			HideChildren: true,
		}
		page, err := h.manager.ListPage(ctx, query)
		if err != nil {
			t.Fatal(err)
		}
		byID := make(map[string]*Info, len(page.Sessions))
		for _, info := range page.Sessions {
			byID[info.ID] = info
		}
		if page.Total != 2 || len(byID) != 2 || byID[parent.ID] == nil || byID[fork.ID] == nil ||
			byID[child.ID] != nil {
			t.Fatalf(
				"hidden-children page = total %d ids %v, want parent and fork",
				page.Total,
				slices.Collect(maps.Keys(byID)),
			)
		}
		if counts := byID[parent.ID].Children; counts == nil ||
			counts.NeedsYou+counts.Working+counts.InReview+counts.Done != 1 {
			t.Fatalf("parent rollup = %#v, want only the provenance child", counts)
		}
		if byID[fork.ID].ParentTitle != "Parent" {
			t.Fatalf("fork origin title = %q, want the source title as provenance", byID[fork.ID].ParentTitle)
		}
		facets, err := h.manager.CatalogFacets(ctx, query)
		if err != nil {
			t.Fatal(err)
		}
		if facets.Facets.All != 2 {
			t.Fatalf("hidden-children facets = %#v", facets.Facets)
		}
		detail := parent.Info()
		if err := h.manager.PopulateSessionRelations(ctx, detail); err != nil {
			t.Fatal(err)
		}
		if counts := detail.Children; counts == nil || *counts != *byID[parent.ID].Children {
			t.Fatalf("detail rollup = %#v, want the page rollup", counts)
		}
	})
}

func (r *pullRequestQueryReader) DefaultBranchNames(context.Context, store.ReadScope, []string) map[string][]string {
	result := make(map[string][]string)
	for workspace := range r.repositories {
		result[workspace] = []string{"main", "master"}
	}
	return result
}

// The database remains real; only discovery calls are counted at its I/O boundary.
type catalogBranchReadCounter struct {
	*globaldb.GlobalDB
	calls int
}

func (r *catalogBranchReadCounter) SessionCatalogBranches(
	ctx context.Context,
	query store.SessionCatalogPageQuery,
) ([]store.SessionBranchKey, error) {
	r.calls++
	return r.GlobalDB.SessionCatalogBranches(ctx, query)
}
