package session

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	aghconfig "github.com/compozy/agh/internal/config"
	envpkg "github.com/compozy/agh/internal/sandbox"
	"github.com/compozy/agh/internal/store"
)

var benchmarkSessionTime = time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)

func BenchmarkDispatchSandboxSyncBeforeNoHooks(b *testing.B) {
	root := benchmarkSessionWorkspace(b, 128)
	manager := &Manager{
		now: func() time.Time {
			return benchmarkSessionTime
		},
	}
	session := &Session{
		ID:          "sess-bench",
		Name:        "bench",
		AgentName:   "coder",
		WorkspaceID: "ws-bench",
		Workspace:   root,
		Type:        SessionTypeUser,
		State:       StateActive,
		CreatedAt:   benchmarkSessionTime,
		UpdatedAt:   benchmarkSessionTime,
	}
	state := envpkg.SessionState{
		SandboxID:      "env-bench",
		Backend:        envpkg.BackendLocal,
		Profile:        "local",
		RuntimeRootDir: root,
	}
	meta := &store.SessionSandboxMeta{
		SandboxID:      "env-bench",
		Backend:        string(envpkg.BackendLocal),
		Profile:        "local",
		RuntimeRootDir: root,
	}

	ctx := context.Background()
	b.ReportAllocs()

	for b.Loop() {
		_, err := manager.dispatchSandboxSyncBefore(
			ctx,
			session,
			state,
			meta,
			envpkg.SyncDirectionToRuntime,
			envpkg.SyncReasonStart,
		)
		if err != nil {
			b.Fatalf("dispatchSandboxSyncBefore() error = %v", err)
		}
	}
}

func BenchmarkManagerListAllLarge(b *testing.B) {
	sessionsDir := b.TempDir()
	manager := &Manager{
		logger: slog.Default(),
		homePaths: aghconfig.HomePaths{
			SessionsDir: sessionsDir,
		},
	}

	for idx := range 256 {
		sessionDir := filepath.Join(sessionsDir, fmt.Sprintf("sess-%03d", idx))
		if err := os.MkdirAll(sessionDir, 0o755); err != nil {
			b.Fatalf("MkdirAll(%q) error = %v", sessionDir, err)
		}
		if err := store.WriteSessionMeta(store.SessionMetaFile(sessionDir), store.SessionMeta{
			ID:                   fmt.Sprintf("sess-%03d", idx),
			Name:                 fmt.Sprintf("Session %03d", idx),
			AgentName:            "coder",
			WorkspaceID:          "ws-bench",
			NetworkParticipation: testLocalParticipationPtr(),
			SessionType:          string(SessionTypeUser),
			State:                string(StateStopped),
			CreatedAt:            benchmarkSessionTime.Add(-time.Duration(idx) * time.Minute),
			UpdatedAt:            benchmarkSessionTime.Add(-time.Duration(idx) * time.Second),
		}); err != nil {
			b.Fatalf("WriteSessionMeta(%d) error = %v", idx, err)
		}
	}

	ctx := context.Background()
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

func BenchmarkSessionInfo(b *testing.B) {
	session := &Session{
		ID:                   "sess-bench",
		Name:                 "bench",
		AgentName:            "coder",
		WorkspaceID:          "ws-bench",
		Workspace:            "/tmp/workspace",
		NetworkParticipation: testLiveParticipation("ws-bench", "builders"),
		Type:                 SessionTypeUser,
		State:                StateActive,
		ACPSessionID:         "acp-bench",
		ACPCaps: acp.Caps{
			SupportsLoadSession: true,
			SupportedModes:      []string{"chat", "agentic"},
		},
		Sandbox: &store.SessionSandboxMeta{
			SandboxID:             "env-bench",
			Backend:               string(envpkg.BackendLocal),
			Profile:               "local",
			State:                 "prepared",
			RuntimeRootDir:        "/tmp/workspace",
			RuntimeAdditionalDirs: []string{"/tmp/shared"},
			ProviderState:         []byte(`{"runtime":"ok"}`),
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

func benchmarkSessionWorkspace(b *testing.B, files int) string {
	b.Helper()

	root := b.TempDir()
	for idx := range files {
		file := filepath.Join(root, fmt.Sprintf("file-%03d.txt", idx))
		if err := os.WriteFile(file, []byte("bench"), 0o644); err != nil {
			b.Fatalf("WriteFile(%q) error = %v", file, err)
		}
	}
	return root
}
