package consolidation

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/agh/internal/memory"
	"github.com/compozy/agh/internal/session"
)

func TestNewSessionSpawnerContract(t *testing.T) {
	t.Parallel()

	t.Run("Should use prior lock timestamp during service run", func(t *testing.T) {
		t.Parallel()

		cfg := dreamConfig()
		prior := time.Date(2026, 4, 4, 8, 0, 0, 0, time.UTC)
		sessions := &fakeSessionManager{
			infos: []*session.Info{
				{
					ID:          "user-recent",
					WorkspaceID: "ws-recent",
					Type:        session.SessionTypeUser,
					UpdatedAt:   prior.Add(time.Hour),
				},
			},
		}
		globalMemoryDir := filepath.Join(t.TempDir(), "memory")
		lockPath := memory.ConsolidationLockPath(globalMemoryDir)
		if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
			t.Fatalf("os.MkdirAll(lock dir) error = %v", err)
		}
		if err := os.WriteFile(lockPath, nil, 0o644); err != nil {
			t.Fatalf("os.WriteFile(lock) error = %v", err)
		}
		if err := os.Chtimes(lockPath, prior, prior); err != nil {
			t.Fatalf("os.Chtimes(lock) error = %v", err)
		}

		service := memory.NewService(
			memory.WithLockPath(lockPath),
			memory.WithMinHours(0),
			memory.WithMinSessions(0),
			memory.WithLogger(discardLogger()),
		)
		spawner := newTestSessionSpawner(sessions, &fakeWorkspaceResolver{}, &cfg)
		if err := service.Run(context.Background(), spawner, ""); err != nil {
			t.Fatalf("service.Run() error = %v", err)
		}

		if got := sessions.createCount(); got != 1 {
			t.Fatalf("Create() calls = %d, want 1", got)
		}
		if got := sessions.createCall(0).Workspace; got != "ws-recent" {
			t.Fatalf("Create() workspace = %q, want ws-recent", got)
		}
	})
}
