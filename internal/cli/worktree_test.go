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
				ID:            "wt-pending",
				ProfileName:   "default",
				WorkspaceID:   "workspace-a",
				Name:          "pending",
				Branch:        "feature/pending",
				Path:          "/repo/.compozy/worktrees/pending",
				State:         "pending",
				Origin:        "manual",
				Dirty:         nil,
				Ahead:         nil,
				Behind:        nil,
				AgentActivity: "idle",
			},
			{
				ID:            "wt-missing",
				ProfileName:   "default",
				WorkspaceID:   "workspace-a",
				Name:          "missing",
				Branch:        "feature/missing",
				Path:          "/repo/.compozy/worktrees/missing",
				State:         "missing",
				Origin:        "adopted",
				Dirty:         &dirty,
				Ahead:         &ahead,
				Behind:        &behind,
				AgentActivity: "running",
			},
			{
				ID:            "wt-failed",
				ProfileName:   "default",
				WorkspaceID:   "workspace-a",
				Name:          "failed",
				Branch:        "feature/failed",
				Path:          "/repo/.compozy/worktrees/failed",
				State:         "failed",
				Origin:        "manual",
				SetupError:    "setup failed",
				AgentActivity: "idle",
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
				got[index].ProfileName != items[index].ProfileName ||
				got[index].Name != items[index].Name ||
				got[index].Branch != items[index].Branch ||
				got[index].Path != items[index].Path ||
				got[index].State != items[index].State ||
				got[index].Origin != items[index].Origin ||
				got[index].AgentActivity != items[index].AgentActivity {
				t.Fatalf("worktree[%d] = %#v, want HTTP item %#v", index, got[index], items[index])
			}
		}

		human, _, err := executeRootCommand(
			t,
			newWorkspaceTestDeps(t, client),
			"worktree", "list", "--workspace", "workspace-a", "-o", "human",
		)
		if err != nil {
			t.Fatalf("worktree human list error = %v", err)
		}
		if !strings.Contains(human, "Profile") || !strings.Contains(human, "default") {
			t.Fatalf("worktree human list = %q, want profile column and value", human)
		}

		toon, _, err := executeRootCommand(
			t,
			newWorkspaceTestDeps(t, client),
			"worktree", "list", "--workspace", "workspace-a", "-o", "toon",
		)
		if err != nil {
			t.Fatalf("worktree TOON list error = %v", err)
		}
		if !strings.Contains(toon, "worktrees[3]{id,profile_name") || !strings.Contains(toon, "default") {
			t.Fatalf("worktree TOON list = %q, want profile_name column and value", toon)
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
		client := withWorkspaceResolution(&stubClient{inspectWorktreeFn: func(
			_ context.Context,
			workspaceID, ref string,
		) (WorktreeInspectRecord, error) {
			if workspaceID != "workspace-a" || ref != "feature-a" {
				t.Fatalf("InspectWorktree() args = %q, %q", workspaceID, ref)
			}
			return WorktreeInspectRecord{Worktree: WorktreeRecord{ID: "wt-a", Name: ref}}, nil
		}, removeWorktreeFn: func(
			_ context.Context,
			workspaceID string,
			ref string,
			force bool,
		) error {
			if workspaceID != "workspace-a" || ref != "wt-a" || force {
				t.Fatalf("RemoveWorktree() args = %q, %q, %t", workspaceID, ref, force)
			}
			return apiErr
		}})
		exitCode, _, stderr := executeRootCommandWithExit(
			t,
			newWorkspaceTestDeps(t, client),
			"worktree", "remove", "feature-a", "--workspace", "workspace-a", "-o", "json",
		)
		if exitCode == 0 {
			t.Fatal("removal refusal exit code = 0, want non-zero")
		}
		if got := strings.TrimSpace(stderr); got != body {
			t.Fatalf("removal refusal = %s, want %s", got, body)
		}
	})
}

// Invariant: every CLI mutation resolves a name once, forwards the canonical
// ID, and reports that same identity. Owning layer: worktree CLI. Canonical suite: this file.
func TestWorktreeMutationNameReferences(t *testing.T) {
	t.Parallel()

	inspect := func(t *testing.T, state string) func(context.Context, string, string) (WorktreeInspectRecord, error) {
		t.Helper()
		calls := 0
		t.Cleanup(func() {
			if calls != 1 {
				t.Errorf("InspectWorktree() calls = %d, want one", calls)
			}
		})
		return func(_ context.Context, workspaceID, ref string) (WorktreeInspectRecord, error) {
			calls++
			if workspaceID != "workspace-a" || ref != "feature-a" {
				t.Fatalf("InspectWorktree() args = %q, %q", workspaceID, ref)
			}
			return WorktreeInspectRecord{Worktree: WorktreeRecord{
				ID: "wt-a", WorkspaceID: workspaceID, Name: ref, State: state,
			}}, nil
		}
	}
	assertMutation := func(t *testing.T, stdout, action, state string) {
		t.Helper()
		var got struct {
			Action   string         `json:"action"`
			Worktree WorktreeRecord `json:"worktree"`
		}
		if err := json.Unmarshal([]byte(stdout), &got); err != nil {
			t.Fatalf("decode mutation output: %v; output=%s", err, stdout)
		}
		if got.Action != action || got.Worktree.ID != "wt-a" || got.Worktree.Name != "feature-a" ||
			got.Worktree.State != state {
			t.Fatalf("mutation output = %#v, want %s wt-a feature-a %s", got, action, state)
		}
	}

	t.Run("Should resolve a name when canceling creation", func(t *testing.T) {
		t.Parallel()
		client := withWorkspaceResolution(&stubClient{inspectWorktreeFn: inspect(t, "pending"), cancelWorktreeFn: func(
			_ context.Context,
			workspaceID string,
			ref string,
		) error {
			if workspaceID != "workspace-a" || ref != "wt-a" {
				t.Fatalf("CancelWorktreeCreate() args = %q, %q", workspaceID, ref)
			}
			return nil
		}})
		stdout, _, err := executeRootCommand(
			t, newWorkspaceTestDeps(t, client),
			"worktree", "cancel", "feature-a", "--workspace", "workspace-a", "-o", "json",
		)
		if err != nil {
			t.Fatalf("worktree cancel by name error = %v", err)
		}
		assertMutation(t, stdout, "canceled", "pending")
	})

	t.Run("Should resolve a name when removing a ready worktree", func(t *testing.T) {
		t.Parallel()
		client := withWorkspaceResolution(&stubClient{inspectWorktreeFn: inspect(t, "ready"), removeWorktreeFn: func(
			_ context.Context,
			workspaceID, ref string,
			force bool,
		) error {
			if workspaceID != "workspace-a" || ref != "wt-a" || !force {
				t.Fatalf("RemoveWorktree() args = %q, %q, %t", workspaceID, ref, force)
			}
			return nil
		}})
		stdout, _, err := executeRootCommand(
			t, newWorkspaceTestDeps(t, client),
			"worktree", "remove", "feature-a", "--force", "--workspace", "workspace-a", "-o", "json",
		)
		if err != nil {
			t.Fatalf("worktree remove by name error = %v", err)
		}
		assertMutation(t, stdout, "removed", "removed")
	})

	t.Run("Should resolve a name when dismissing a tombstone", func(t *testing.T) {
		t.Parallel()
		client := withWorkspaceResolution(&stubClient{inspectWorktreeFn: inspect(t, "removed"), dismissWorktreeFn: func(
			_ context.Context,
			workspaceID string,
			ref string,
		) error {
			if workspaceID != "workspace-a" || ref != "wt-a" {
				t.Fatalf("DismissWorktree() args = %q, %q", workspaceID, ref)
			}
			return nil
		}})
		stdout, _, err := executeRootCommand(
			t, newWorkspaceTestDeps(t, client),
			"worktree", "dismiss", "feature-a", "--workspace", "workspace-a", "-o", "json",
		)
		if err != nil {
			t.Fatalf("worktree dismiss by name error = %v", err)
		}
		assertMutation(t, stdout, "dismissed", "dismissed")
	})
}

func TestWorktreeExitCommands(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve the exit plan in JSON output", func(t *testing.T) {
		t.Parallel()
		client := withWorkspaceResolution(&stubClient{worktreeExitPlanFn: func(
			_ context.Context,
			workspaceID, ref string,
		) (WorktreeExitPlanRecord, error) {
			if workspaceID != "workspace-a" || ref != "feature-a" {
				t.Fatalf("GetWorktreeExitPlan() args = %q, %q", workspaceID, ref)
			}
			return WorktreeExitPlanRecord{
				WorktreeID: "wt-a", Primary: "commit_push", Base: "main",
				Actions: []contract.WorktreeExitActionPlan{{
					Action: "commit_push", Label: "Commit and push", Enabled: true, Publish: true,
				}},
				CommitScope: contract.WorktreeExitCommitScope{
					ChangedFiles: 2, UntrackedFiles: []string{"new.txt"}, UntrackedTotal: 1,
				},
			}, nil
		}})
		stdout, _, err := executeRootCommand(
			t, newWorkspaceTestDeps(t, client),
			"worktree", "exit", "feature-a", "--workspace", "workspace-a", "-o", "json",
		)
		if err != nil {
			t.Fatalf("worktree exit error = %v", err)
		}
		var got WorktreeExitPlanRecord
		if err := json.Unmarshal([]byte(stdout), &got); err != nil {
			t.Fatalf("decode exit output: %v; output=%s", err, stdout)
		}
		if got.WorktreeID != "wt-a" || got.Primary != "commit_push" || len(got.Actions) != 1 ||
			got.CommitScope.UntrackedTotal != 1 {
			t.Fatalf("worktree exit output = %#v", got)
		}
	})

	for _, testCase := range []struct {
		name string
		args []string
		want WorktreeExitActionRequest
	}{
		{
			name: "commit and push flags",
			args: []string{"commit", "feature-a", "--message", "Ready", "--push"},
			want: WorktreeExitActionRequest{Action: "commit_push", Message: "Ready"},
		},
		{
			name: "push action",
			args: []string{"push", "feature-a"},
			want: WorktreeExitActionRequest{Action: "push"},
		},
		{
			name: "pull request flags",
			args: []string{"pr", "feature-a", "--title", "Exit", "--body", "Ready", "--draft", "--base", "trunk"},
			want: WorktreeExitActionRequest{
				Action: "open_pr", Title: "Exit", Body: "Ready", Draft: true, Base: "trunk",
			},
		},
	} {
		t.Run("Should send "+testCase.name, func(t *testing.T) {
			t.Parallel()
			var got WorktreeExitActionRequest
			client := withWorkspaceResolution(&stubClient{runWorktreeExitFn: func(
				_ context.Context,
				workspaceID, ref string,
				request WorktreeExitActionRequest,
			) (WorktreeExitOperationRecord, error) {
				if workspaceID != "workspace-a" || ref != "feature-a" {
					t.Fatalf("RunWorktreeExitAction() args = %q, %q", workspaceID, ref)
				}
				got = request
				return WorktreeExitOperationRecord{OperationID: "op-exit"}, nil
			}})
			args := append([]string{"worktree"}, testCase.args...)
			args = append(args, "--workspace", "workspace-a", "-o", "json")
			stdout, _, err := executeRootCommand(t, newWorkspaceTestDeps(t, client), args...)
			var operation WorktreeExitOperationRecord
			if decodeErr := json.Unmarshal([]byte(stdout), &operation); decodeErr != nil {
				t.Fatalf("decode operation output: %v; output=%s", decodeErr, stdout)
			}
			if err != nil || got != testCase.want || operation.OperationID != "op-exit" {
				t.Fatalf("command error/request/output = %v/%#v/%s", err, got, stdout)
			}
		})
	}

	t.Run("Should cancel only the named exit operation", func(t *testing.T) {
		t.Parallel()
		var gotWorkspace, gotRef, gotOperation string
		client := withWorkspaceResolution(&stubClient{inspectWorktreeFn: func(
			_ context.Context,
			workspaceID, ref string,
		) (WorktreeInspectRecord, error) {
			if workspaceID != "workspace-a" || ref != "feature-a" {
				t.Fatalf("InspectWorktree() args = %q, %q", workspaceID, ref)
			}
			return WorktreeInspectRecord{Worktree: WorktreeRecord{ID: "wt-a", Name: ref}}, nil
		}, cancelWorktreeExitFn: func(
			_ context.Context,
			workspaceID, ref, opID string,
		) error {
			gotWorkspace, gotRef, gotOperation = workspaceID, ref, opID
			return nil
		}})
		stdout, _, err := executeRootCommand(
			t, newWorkspaceTestDeps(t, client),
			"worktree", "exit-cancel", "feature-a", "--op", " op-exit ",
			"--workspace", "workspace-a", "-o", "json",
		)
		var output struct {
			OperationID string `json:"op_id"`
			State       string `json:"state"`
		}
		if decodeErr := json.Unmarshal([]byte(stdout), &output); decodeErr != nil {
			t.Fatalf("decode cancel output: %v; output=%s", decodeErr, stdout)
		}
		if err != nil || gotWorkspace != "workspace-a" || gotRef != "wt-a" || gotOperation != "op-exit" ||
			output.OperationID != "op-exit" || output.State != "cancel_requested" {
			t.Fatalf("cancel error/args/output = %v/%q/%q/%q/%s", err, gotWorkspace, gotRef, gotOperation, stdout)
		}
	})

	t.Run("Should require an operation ID before canceling", func(t *testing.T) {
		t.Parallel()
		called := false
		client := withWorkspaceResolution(&stubClient{cancelWorktreeExitFn: func(
			context.Context,
			string, string, string,
		) error {
			called = true
			return nil
		}})
		_, _, err := executeRootCommand(
			t, newWorkspaceTestDeps(t, client),
			"worktree", "exit-cancel", "wt-a", "--workspace", "workspace-a",
		)
		if err == nil || !strings.Contains(err.Error(), "--op is required") || called {
			t.Fatalf("exit-cancel error/called = %v/%t, want missing --op", err, called)
		}
	})
}
