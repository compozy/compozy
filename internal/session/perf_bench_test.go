package session

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb"
	"github.com/compozy/compozy/internal/transcript"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

var benchmarkSessionTime = time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)

func BenchmarkManagerCatalog(b *testing.B) {
	manager := newWorkspaceSessionReadFixture(b, 1024, 16)
	for index := range 64 {
		target := &Session{
			ID: fmt.Sprintf("sess-read-%03d", index), AgentName: "coder", Provider: "claude",
			ProfileID: store.DefaultProfileID, WorkspaceID: fmt.Sprintf("ws-read-%02d", index%16),
			State: StateActive, RuntimeStatus: store.SessionRuntimeReady,
			CreatedAt: benchmarkSessionTime.Add(time.Duration(index) * time.Minute), UpdatedAt: benchmarkSessionTime,
		}
		manager.sessions[target.ID] = target
	}
	query := ListQuery{
		ReadScope: store.ReadScope{AllProfiles: true}, AllWorkspaces: true,
		Archive: ArchiveExclude, Sort: ListSortNavigator, Limit: 50, SkipTotal: true,
	}
	b.Run("Page", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			page, err := manager.ListPage(b.Context(), query)
			if err != nil || len(page.Sessions) != 50 || !page.HasMore {
				b.Fatalf("page rows=%d has_more=%v error=%v", len(page.Sessions), page.HasMore, err)
			}
		}
	})
	b.Run("Facets", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			facets, err := manager.CatalogFacets(b.Context(), query)
			if err != nil || facets.Facets.All != 1024 {
				b.Fatalf("facets total=%d error=%v", facets.Facets.All, err)
			}
		}
	})
	b.Run("DetailRelations", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			info := manager.sessionInfoForRead(manager.sessions["sess-read-000"])
			if err := manager.PopulateSessionRelations(b.Context(), info); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkManagerPersistSessionHealth(b *testing.B) {
	ctx := b.Context()
	database, err := openSessionTestGlobalDB(ctx, filepath.Join(b.TempDir(), store.GlobalDatabaseName))
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if err := database.Close(context.Background()); err != nil {
			b.Error(err)
		}
	})
	if err := database.InsertWorkspace(ctx, workspacepkg.Workspace{
		ID: "ws-health-bench", Name: "health-bench", RootDir: b.TempDir(),
	}); err != nil {
		b.Fatal(err)
	}
	target := &Session{
		ID: "sess-health-bench", AgentName: "coder", WorkspaceID: "ws-health-bench", State: StateActive,
	}
	if err := database.RegisterSession(ctx, store.SessionInfo{
		ID: target.ID, ProfileID: store.DefaultProfileID, AgentName: target.AgentName,
		Provider: "claude", WorkspaceID: target.WorkspaceID, State: "active",
		RuntimeStatus: store.SessionRuntimeUnbound,
	}); err != nil {
		b.Fatal(err)
	}
	manager := &Manager{sessionHealthStore: database, now: func() time.Time { return benchmarkSessionTime }}
	at := benchmarkSessionTime
	input := sessionHealthInput{activePrompt: true, attachable: true, activityAt: at}
	if _, err := manager.persistSessionHealthForSession(ctx, target, at, input); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		at = at.Add(time.Millisecond)
		input.activityAt = at
		if _, err := manager.persistSessionHealthForSession(ctx, target, at, input); err != nil {
			b.Fatal(err)
		}
	}
	stored, err := database.GetSessionHealth(ctx, target.ID)
	if err != nil || !stored.LastActivityAt.Equal(at) || !stored.ActivePrompt || !stored.Attachable {
		b.Fatalf("persisted health = %#v, %v", stored, err)
	}
}

func BenchmarkManagerListAllLarge(b *testing.B) {
	sessionsDir := b.TempDir()
	manager := &Manager{
		logger: slog.Default(),
		homePaths: compozyconfig.HomePaths{
			SessionsDir: sessionsDir,
		},
	}

	for idx := range 256 {
		sessionDir := filepath.Join(sessionsDir, fmt.Sprintf("sess-%03d", idx))
		if err := os.MkdirAll(sessionDir, 0o755); err != nil {
			b.Fatalf("MkdirAll(%q) error = %v", sessionDir, err)
		}
		if err := store.WriteSessionMeta(store.SessionMetaFile(sessionDir), &store.SessionMeta{
			ID:            fmt.Sprintf("sess-%03d", idx),
			Name:          fmt.Sprintf("Session %03d", idx),
			AgentName:     "coder",
			WorkspaceID:   "ws-bench",
			SessionType:   string(SessionTypeUser),
			State:         string(StateStopped),
			RuntimeStatus: store.SessionRuntimeReady,
			CreatedAt:     benchmarkSessionTime.Add(-time.Duration(idx) * time.Minute),
			UpdatedAt:     benchmarkSessionTime.Add(-time.Duration(idx) * time.Second),
		}); err != nil {
			b.Fatalf("WriteSessionMeta(%d) error = %v", idx, err)
		}
	}

	ctx := b.Context()
	b.ReportAllocs()

	var infos []*Info
	for b.Loop() {
		var err error
		infos, err = manager.ListAll(ctx)
		if err != nil {
			b.Fatalf("ListAll() error = %v", err)
		}
	}

	if got, want := len(infos), 256; got != want {
		b.Fatalf("len(ListAll()) = %d, want %d", got, want)
	}
}

func BenchmarkManagerWorkspaceSessions(b *testing.B) {
	manager := newWorkspaceSessionReadFixture(b, 256, 16)
	for _, scoped := range []bool{false, true} {
		name := "all_then_filter"
		if scoped {
			name = "workspace_catalog"
		}
		b.Run(name, func(b *testing.B) {
			metadataReads := 0
			manager.readSessionMeta = func(path string) (store.SessionMeta, error) {
				metadataReads++
				return store.ReadSessionMeta(path)
			}
			b.ReportAllocs()
			for b.Loop() {
				var infos []*Info
				var err error
				if scoped {
					infos, err = manager.ListForWorkspace(b.Context(), "ws-read-00")
				} else {
					infos, err = manager.ListAll(b.Context())
					infos = slices.DeleteFunc(infos, func(info *Info) bool { return info.WorkspaceID != "ws-read-00" })
				}
				if err != nil || len(infos) != 16 {
					b.Fatalf("workspace sessions = %d, %v", len(infos), err)
				}
			}
			b.ReportMetric(float64(metadataReads)/float64(b.N), "metadata_reads/op")
		})
	}
}

func BenchmarkManagerContextEvents(b *testing.B) {
	manager := newWorkspaceSessionReadFixture(b, 1, 1)
	manager.lifecycleCtx = b.Context()
	owner := store.SessionDBOwner{SessionID: "sess-read-000", WorkspaceID: "ws-read-00"}
	path := store.SessionDBFile(filepath.Join(manager.homePaths.SessionsDir, owner.SessionID))
	database, err := sessiondb.OpenSessionDB(b.Context(), owner, path)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if err := database.Close(context.Background()); err != nil {
			b.Error(err)
		}
	})
	for index := range 400 {
		var event acp.AgentEvent
		switch index {
		case 100:
			event = acp.AgentEvent{
				Type: events.SessionCompactionFired,
				Raw:  []byte(`{"compaction_id":"c1","trigger":"agent"}`),
			}
		case 101:
			event = acp.AgentEvent{Type: acp.EventTypeCompaction, Compaction: &acp.CompactionObservation{
				CompactionID: "c1", Status: "completed", Terminal: true,
			}}
		default:
			event = acp.AgentEvent{Type: acp.EventTypeUsage, Usage: &acp.TokenUsage{
				ContextUsed: new(int64(index)), ContextSize: new(int64(200000)), TotalTokens: new(int64(100)),
			}}
			if index%10 == 9 {
				event.Type = acp.EventTypeDone
			}
		}
		event.TurnID, event.Timestamp = fmt.Sprintf("turn-%d", index/10), benchmarkSessionTime
		content, err := transcript.MarshalAgentEvent(event)
		if err != nil {
			b.Fatal(err)
		}
		if err := database.Record(b.Context(), store.SessionEvent{
			SessionID: owner.SessionID, TurnID: event.TurnID, Type: event.Type, AgentName: "coder",
			Content: content, Timestamp: event.Timestamp,
		}); err != nil {
			b.Fatal(err)
		}
	}
	if err := database.Close(b.Context()); err != nil {
		b.Fatal(err)
	}
	runtime := newDefaultQueryStoreRuntime(manager.logger)
	b.Cleanup(func() {
		if err := runtime.Shutdown(context.Background()); err != nil {
			b.Error(err)
		}
	})
	manager.openQueryStore = runtime.Open
	want, err := readBenchmarkContextSeparately(b.Context(), manager, owner.SessionID)
	if err != nil {
		b.Fatal(err)
	}
	for _, snapshot := range []bool{false, true} {
		name := "separate_reads"
		if snapshot {
			name = "union_snapshot"
		}
		b.Run(name, func(b *testing.B) {
			queries, rows := 0, 0
			manager.openQueryStore = func(ctx context.Context, owner store.SessionDBOwner, path string) (EventReadCloser, error) {
				reader, err := runtime.Open(ctx, owner, path)
				if err != nil {
					return nil, err
				}
				return &benchmarkContextReader{EventReadCloser: reader, queries: &queries, rows: &rows}, nil
			}
			var got ContextEvents
			b.ReportAllocs()
			for b.Loop() {
				if snapshot {
					got, err = manager.ContextEvents(b.Context(), owner.SessionID)
				} else {
					got, err = readBenchmarkContextSeparately(b.Context(), manager, owner.SessionID)
				}
				if err != nil {
					b.Fatal(err)
				}
			}
			b.ReportMetric(float64(queries)/float64(b.N), "queries/op")
			b.ReportMetric(float64(rows)/float64(b.N), "rows/op")
			if !reflect.DeepEqual(got, want) {
				b.Fatal("context snapshot changed the projected history")
			}
		})
	}
}

type benchmarkContextReader struct {
	EventReadCloser
	queries *int
	rows    *int
}

func (r *benchmarkContextReader) Query(ctx context.Context, query store.EventQuery) ([]store.SessionEvent, error) {
	*r.queries++
	rows, err := r.EventReadCloser.Query(ctx, query)
	*r.rows += len(rows)
	return rows, err
}

func readBenchmarkContextSeparately(ctx context.Context, manager *Manager, id string) (ContextEvents, error) {
	var result ContextEvents
	var err error
	result.Usage, err = manager.UsageEvents(ctx, id)
	if err != nil {
		return result, err
	}
	result.Deliveries, err = manager.Deliveries(ctx, id)
	if err != nil {
		return result, err
	}
	result.Compactions, err = manager.Compactions(ctx, id)
	if err != nil {
		return result, err
	}
	result.Settled, err = manager.LatestSettledTurn(ctx, id)
	if err != nil {
		return result, err
	}
	result.ClearedBy, err = manager.CompactionClearBoundary(ctx, id)
	return result, err
}

func BenchmarkManagerListAllActiveCatalog(b *testing.B) {
	ctx := b.Context()
	database, err := openSessionTestGlobalDB(ctx, filepath.Join(b.TempDir(), store.GlobalDatabaseName))
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if err := database.Close(context.Background()); err != nil {
			b.Error(err)
		}
	})
	if err := database.InsertWorkspace(ctx, workspacepkg.Workspace{
		ID: "ws-active-bench", Name: "active-bench", RootDir: b.TempDir(),
	}); err != nil {
		b.Fatal(err)
	}
	manager := &Manager{
		sessions: make(map[string]*Session), logger: slog.Default(),
		homePaths:      compozyconfig.HomePaths{SessionsDir: b.TempDir()},
		sessionCatalog: database, transcriptEpochStore: database,
		now: func() time.Time { return benchmarkSessionTime },
	}
	for index := range 64 {
		target := &Session{
			ID: fmt.Sprintf("sess-active-%03d", index), AgentName: "coder", Provider: "claude",
			ProfileID: store.DefaultProfileID, WorkspaceID: "ws-active-bench",
			State: StateActive, RuntimeStatus: store.SessionRuntimeReady,
			CreatedAt: benchmarkSessionTime, UpdatedAt: benchmarkSessionTime,
		}
		manager.sessions[target.ID] = target
		if err := database.RegisterSession(ctx, sessionCatalogInfoFromRuntime(target.Info())); err != nil {
			b.Fatal(err)
		}
		meta := target.Meta()
		path := filepath.Join(manager.homePaths.SessionsDir, target.ID)
		if err := os.MkdirAll(path, 0o755); err != nil {
			b.Fatal(err)
		}
		if err := store.WriteSessionMeta(store.SessionMetaFile(path), &meta); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportAllocs()
	for b.Loop() {
		infos, err := manager.ListAll(ctx)
		if err != nil || len(infos) != 64 {
			b.Fatalf("ListAll() = %d sessions, %v", len(infos), err)
		}
	}
}

func BenchmarkSessionInfo(b *testing.B) {
	session := &Session{
		ID:           "sess-bench",
		Name:         "bench",
		AgentName:    "coder",
		WorkspaceID:  "ws-bench",
		Workspace:    "/tmp/workspace",
		Type:         SessionTypeUser,
		State:        StateActive,
		ACPSessionID: "acp-bench",
		ACPCaps: acp.Caps{
			SupportsLoadSession: true,
			SupportedModes:      []string{"chat", "agentic"},
		},
		CreatedAt: benchmarkSessionTime,
		UpdatedAt: benchmarkSessionTime,
	}

	b.ReportAllocs()

	var info *Info
	for b.Loop() {
		info = session.Info()
	}

	if info == nil || info.ID == "" {
		b.Fatalf("Session.Info() = %#v, want populated snapshot", info)
	}
}

func BenchmarkManagerIdleInventory(b *testing.B) {
	for _, live := range []bool{false, true} {
		name := "ListAll"
		if live {
			name = "ReaperInventory"
		}
		b.Run(name, func(b *testing.B) {
			manager := newWorkspaceSessionReadFixture(b, 279, 1)
			counts := &benchmarkInventoryReads{
				SessionCatalog: manager.sessionCatalog, SessionAttentionStore: manager.attentionStore,
				SessionTranscriptEpochStore: manager.transcriptEpochStore,
			}
			manager.sessionCatalog, manager.attentionStore, manager.transcriptEpochStore = counts, counts, counts
			metadataReads := 0
			manager.readSessionMeta = func(path string) (store.SessionMeta, error) {
				metadataReads++
				return store.ReadSessionMeta(path)
			}
			read := manager.ListAll
			want := 279
			if live {
				read, want = manager.ListLive, 0
			}
			if _, err := read(b.Context()); err != nil {
				b.Fatal(err)
			}
			counts.reads, metadataReads = 0, 0
			b.ReportAllocs()
			for b.Loop() {
				infos, err := read(b.Context())
				if err != nil || len(infos) != want {
					b.Fatalf("inventory=%d want=%d error=%v", len(infos), want, err)
				}
			}
			b.ReportMetric(float64(metadataReads)/float64(b.N), "metadata_reads/op")
			b.ReportMetric(float64(counts.reads)/float64(b.N), "catalog_reads/op")
		})
	}
}

type benchmarkInventoryReads struct {
	store.SessionCatalog
	store.SessionAttentionStore
	store.SessionTranscriptEpochStore
	reads int
}

func (s *benchmarkInventoryReads) ListSessions(
	ctx context.Context, query store.SessionListQuery,
) ([]store.SessionInfo, error) {
	s.reads++
	return s.SessionCatalog.ListSessions(ctx, query)
}

func (s *benchmarkInventoryReads) GetSessionAttention(ctx context.Context, id string) (store.SessionAttention, error) {
	s.reads++
	return s.SessionAttentionStore.GetSessionAttention(ctx, id)
}

func (s *benchmarkInventoryReads) SessionTranscriptEpoch(ctx context.Context, id string) (int64, error) {
	s.reads++
	return s.SessionTranscriptEpochStore.SessionTranscriptEpoch(ctx, id)
}
