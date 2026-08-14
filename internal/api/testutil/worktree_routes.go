package testutil

import (
	"sort"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// WorktreeRouteKeysFromGin returns the normalized worktree route keys registered on a Gin engine.
func WorktreeRouteKeysFromGin(routes gin.RoutesInfo) []string {
	keys := make([]string, 0)
	for _, route := range routes {
		if route.Path == "/api/worktrees/catalog-stream" ||
			strings.HasPrefix(route.Path, "/api/workspaces/:workspace_id/worktrees") ||
			route.Path == "/api/tasks/:id/execution-profile/worktree" {
			keys = append(keys, route.Method+" "+route.Path)
		}
	}
	sort.Strings(keys)
	return keys
}

// ExpectedWorktreeRouteKeys is the shared IT-033 parity contract for every transport.
func ExpectedWorktreeRouteKeys() []string {
	keys := []string{
		"DELETE /api/workspaces/:workspace_id/worktrees/:worktree_id",
		"GET /api/workspaces/:workspace_id/worktrees",
		"GET /api/workspaces/:workspace_id/worktrees/:worktree_id",
		"GET /api/workspaces/:workspace_id/worktrees/:worktree_id/exit",
		"GET /api/workspaces/:workspace_id/worktrees/:worktree_id/status",
		"GET /api/workspaces/:workspace_id/worktrees/:worktree_id/stream",
		"GET /api/worktrees/catalog-stream",
		"PATCH /api/tasks/:id/execution-profile/worktree",
		"POST /api/workspaces/:workspace_id/worktrees",
		"POST /api/workspaces/:workspace_id/worktrees/:worktree_id/cancel",
		"POST /api/workspaces/:workspace_id/worktrees/:worktree_id/dismiss",
		"POST /api/workspaces/:workspace_id/worktrees/:worktree_id/exit/actions",
		"POST /api/workspaces/:workspace_id/worktrees/:worktree_id/exit/cancel",
		"POST /api/workspaces/:workspace_id/worktrees/adopt",
	}
	sort.Strings(keys)
	return keys
}

// AssertWorktreeRouteParity fails when a transport drifts from the complete IT-033 route contract.
func AssertWorktreeRouteParity(t testing.TB, got []string) {
	t.Helper()
	want := ExpectedWorktreeRouteKeys()
	if len(got) != len(want) {
		t.Fatalf("len(worktree routes) = %d, want %d\nroutes=%v", len(got), len(want), got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("worktree route[%d] = %q, want %q", index, got[index], want[index])
		}
	}
}
