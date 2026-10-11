package session

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

var benchmarkSessionTime = time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)

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
