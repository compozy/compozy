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
)

var benchmarkSessionTime = time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)

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
