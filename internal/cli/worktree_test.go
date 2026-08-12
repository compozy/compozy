package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
)

func TestWorktreeListCommand(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve the HTTP worktree fields and states in JSON output", func(t *testing.T) {
		t.Parallel()

		dirty, ahead, behind := true, 2, 1
		items := []WorktreeRecord{
			{
				ID: "wt-pending", WorkspaceID: "workspace-a", Name: "pending", Branch: "feature/pending",
				Path: "/repo/.compozy/worktrees/pending", State: "pending", Origin: "manual",
				Dirty: nil, Ahead: nil, Behind: nil, AgentActivity: "idle",
			},
			{
				ID: "wt-missing", WorkspaceID: "workspace-a", Name: "missing", Branch: "feature/missing",
				Path: "/repo/.compozy/worktrees/missing", State: "missing", Origin: "adopted",
				Dirty: &dirty, Ahead: &ahead, Behind: &behind, AgentActivity: "running",
			},
			{
				ID: "wt-failed", WorkspaceID: "workspace-a", Name: "failed", Branch: "feature/failed",
				Path: "/repo/.compozy/worktrees/failed", State: "failed", Origin: "manual",
				SetupError: "setup failed", AgentActivity: "idle",
			},
		}
		client := withWorkspaceResolution(&stubClient{listWorktreesFn: func(
			_ context.Context,
			workspaceID string,
			refresh bool,
		) (WorktreeListRecord, error) {
			if workspaceID != "workspace-a" || refresh {
				t.Fatalf("ListWorktrees() args = %q, %t", workspaceID, refresh)
			}
			return WorktreeListRecord{Worktrees: items}, nil
		}})
		stdout, _, err := executeRootCommand(
			t,
			newWorkspaceTestDeps(t, client),
			"worktree", "list", "--workspace", "workspace-a", "-o", "json",
		)
		if err != nil {
			t.Fatalf("worktree list error = %v", err)
		}
		var got []WorktreeRecord
		if err := json.Unmarshal([]byte(stdout), &got); err != nil {
			t.Fatalf("json.Unmarshal(worktree list) error = %v; output=%s", err, stdout)
		}
		if len(got) != len(items) {
			t.Fatalf("worktree list count = %d, want %d", len(got), len(items))
		}
		for index := range items {
			if got[index].ID != items[index].ID ||
				got[index].Name != items[index].Name ||
				got[index].Branch != items[index].Branch ||
				got[index].Path != items[index].Path ||
				got[index].State != items[index].State ||
				got[index].Origin != items[index].Origin ||
				got[index].AgentActivity != items[index].AgentActivity {
				t.Fatalf("worktree[%d] = %#v, want HTTP item %#v", index, got[index], items[index])
			}
		}
	})

	t.Run("Should return an empty JSON array with exit zero", func(t *testing.T) {
		t.Parallel()

		client := withWorkspaceResolution(&stubClient{listWorktreesFn: func(
			context.Context,
			string,
			bool,
		) (WorktreeListRecord, error) {
			return WorktreeListRecord{}, nil
		}})
		exitCode, stdout, stderr := executeRootCommandWithExit(
			t,
			newWorkspaceTestDeps(t, client),
			"worktree", "list", "--workspace", "workspace-a", "-o", "json",
		)
		if exitCode != 0 || strings.TrimSpace(stdout) != "[]" || stderr != "" {
			t.Fatalf("empty list exit/stdout/stderr = %d/%q/%q, want 0/[]/empty", exitCode, stdout, stderr)
		}
	})

	t.Run("Should return worktree_not_found without a root fallback message", func(t *testing.T) {
		t.Parallel()

		client := withWorkspaceResolution(&stubClient{listWorktreesFn: func(
			context.Context,
			string,
			bool,
		) (WorktreeListRecord, error) {
			return WorktreeListRecord{}, &daemonAPIError{
				statusCode: http.StatusNotFound,
				status:     "404 Not Found",
				payload: contract.ErrorPayload{
					Code: "worktree_not_found", Error: "worktree_not_found",
				},
			}
		}})
		exitCode, _, stderr := executeRootCommandWithExit(
			t,
			newWorkspaceTestDeps(t, client),
			"worktree", "list", "--workspace", "workspace-a", "-o", "json",
		)
		if exitCode == 0 {
			t.Fatal("worktree_not_found exit code = 0, want non-zero")
		}
		var payload contract.ErrorPayload
		if err := json.Unmarshal([]byte(stderr), &payload); err != nil {
			t.Fatalf("json.Unmarshal(worktree error) error = %v; stderr=%s", err, stderr)
		}
		if payload.Code != "worktree_not_found" || strings.Contains(stderr, "root fallback") {
			t.Fatalf("structured error = %#v; stderr=%q", payload, stderr)
		}
	})
}

func TestWorktreeRemovalRefusalOutput(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve the API refusal payload in CLI JSON", func(t *testing.T) {
		t.Parallel()

		const body = `{"code":"worktree_dirty_requires_force","risk":{"changed_files":2,"insertions":4,"deletions":1,"unpushed_commits":3,"exists_on_remote":false},"downgrade":false}`
		apiErr := readAPIErrorBody(http.StatusConflict, "409 Conflict", []byte(body))
		client := withWorkspaceResolution(&stubClient{removeWorktreeFn: func(
			context.Context,
			string,
			string,
			bool,
		) error {
			return apiErr
		}})
		exitCode, _, stderr := executeRootCommandWithExit(
			t,
			newWorkspaceTestDeps(t, client),
			"worktree", "remove", "wt-a", "--workspace", "workspace-a", "-o", "json",
		)
		if exitCode == 0 {
			t.Fatal("removal refusal exit code = 0, want non-zero")
		}
		if got := strings.TrimSpace(stderr); got != body {
			t.Fatalf("removal refusal = %s, want %s", got, body)
		}
	})
}
