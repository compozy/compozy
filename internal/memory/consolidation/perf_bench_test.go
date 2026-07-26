package consolidation

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/compozy/agh/internal/session"
)

func BenchmarkResolveWorkspacesRecentSessions(b *testing.B) {
	lockTime := time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC)

	sessions := &fakeSessionManager{infos: benchmarkSessionInfos(lockTime)}
	resolver := &fakeWorkspaceResolver{}
	ctx := context.Background()

	for b.Loop() {
		workspaces, err := resolveWorkspaces(ctx, sessions, resolver, lockTime, "")
		if err != nil {
			b.Fatalf("resolveWorkspaces() error = %v", err)
		}
		if len(workspaces) != 96 {
			b.Fatalf("len(workspaces) = %d, want 96", len(workspaces))
		}
	}
}

func benchmarkSessionInfos(lockTime time.Time) []*session.Info {
	infos := make([]*session.Info, 0, 512)
	for idx := range 512 {
		workspaceID := fmt.Sprintf("ws-%03d", idx%96)
		updatedAt := lockTime.Add(time.Duration(idx+1) * time.Minute)
		infos = append(infos, &session.Info{
			ID:          fmt.Sprintf("session-%03d", idx),
			WorkspaceID: workspaceID,
			Type:        session.SessionTypeUser,
			UpdatedAt:   updatedAt,
		})
	}
	return infos
}
