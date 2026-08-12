//go:build integration && !windows

package daemon

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store/globaldb"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	worktreepkg "github.com/compozy/compozy/internal/worktree"
)

// Canonical integration suite: loop environment binding against real Git-backed roots.
func TestLoopActionEnvironmentRealGitIntegration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repository := filepath.Join(t.TempDir(), "repository")
	if err := os.MkdirAll(filepath.Join(repository, "packages", "api"), 0o700); err != nil {
		t.Fatalf("MkdirAll(repository) error = %v", err)
	}
	runner, err := worktreepkg.NewRealGitRunner(10 * time.Second)
	if err != nil {
		t.Fatalf("NewRealGitRunner() error = %v", err)
	}
	runLoopEnvironmentGit(t, runner, repository, "init", "-b", "main")
	runLoopEnvironmentGit(t, runner, repository, "config", "user.name", "Compozy Integration")
	runLoopEnvironmentGit(t, runner, repository, "config", "user.email", "integration@compozy.test")
	if err := os.WriteFile(filepath.Join(repository, "README.md"), []byte("loop environment\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(README.md) error = %v", err)
	}
	runLoopEnvironmentGit(t, runner, repository, "add", "README.md")
	runLoopEnvironmentGit(t, runner, repository, "commit", "-m", "initial")

	database, err := globaldb.OpenGlobalDB(ctx, filepath.Join(t.TempDir(), "compozy.db"))
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(context.Background()); err != nil {
			t.Errorf("GlobalDB.Close() error = %v", err)
		}
	})
	const workspaceID = "ws-loop-environment"
	now := time.Date(2026, 8, 12, 18, 0, 0, 0, time.UTC)
	if err := database.InsertWorkspace(ctx, workspacepkg.Workspace{
		ID: workspaceID, RootDir: repository, Name: "Loop Environment",
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("InsertWorkspace() error = %v", err)
	}
	settings := compozyconfig.DefaultWorktreesConfig()
	worktreesRoot := filepath.Join(t.TempDir(), "worktrees")
	worktrees := worktreepkg.NewService(
		database.WorktreeStore(),
		runner,
		worktreepkg.WithWorkspaceResolver(loopEnvironmentWorkspaceResolver{workspace: worktreepkg.Workspace{
			ID: workspaceID, Name: "Loop Environment", Root: repository,
		}}),
		worktreepkg.WithConfig(settings, worktreesRoot),
	)
	manual, err := worktrees.Create(ctx, workspaceID, worktreepkg.CreateOptions{Name: "Loop Ref"})
	if err != nil {
		t.Fatalf("Create(manual loop worktree) error = %v", err)
	}

	resolvedConfig := compozyconfig.DefaultWithHome(compozyconfig.HomePaths{HomeDir: t.TempDir()})
	resolvedConfig.Defaults.Provider = "mock"
	resolvedConfig.Defaults.Sandbox = "evidence-lab"
	resolvedConfig.Providers["mock"] = compozyconfig.ProviderConfig{Command: "mock-acp"}
	resolvedConfig.Sandboxes["evidence-lab"] = compozyconfig.SandboxProfile{Backend: "local"}
	resolved := workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{
			ID: workspaceID, RootDir: repository, SandboxRef: "evidence-lab",
		},
		WorkspaceID: workspaceID,
		Config:      resolvedConfig,
		Agents: []compozyconfig.AgentDef{{
			Name: "task-worker", Provider: "mock", Prompt: "Handle the loop node.",
		}},
	}
	sessions := &loopActionBinderSessionManager{sessionID: "sess-loop-environment"}
	binder := &loopActionSessionBinder{
		sessions:  sessions,
		worktrees: worktrees,
		policyGate: &loopSessionPolicyGate{workspaceResolver: loopActionBinderWorkspaceResolver{
			byID: map[string]workspacepkg.ResolvedWorkspace{workspaceID: resolved},
		}},
	}

	bind := func(environment dsl.EnvironmentSpec, itemIndex int) {
		t.Helper()
		if _, err := binder.BindActionSession(ctx, looppkg.ActionSessionBindRequest{
			WorkspaceID: workspaceID,
			LoopRunID:   "loop-real-environment",
			Generation:  2,
			NodeID:      "review",
			ItemIndex:   itemIndex,
			Agent:       "task-worker",
			Handle:      "review",
			Environment: environment,
		}); err != nil {
			t.Fatalf("BindActionSession(%#v, item=%d) error = %v", environment, itemIndex, err)
		}
	}

	bind(dsl.EnvironmentSpec{Mode: dsl.EnvironmentWorktree, WorktreeRef: manual.ID}, 0)
	refCall := sessions.allCreateCalls()[0]
	if refCall.Worktree != manual.ID || refCall.CWD != manual.Path {
		t.Fatalf("worktree ref binding = %#v, want %q at %q", refCall, manual.ID, manual.Path)
	}
	if _, err := os.Stat(filepath.Join(manual.Path, ".git")); err != nil {
		t.Fatalf("Stat(manual .git) error = %v", err)
	}

	for itemIndex := range 3 {
		bind(dsl.EnvironmentSpec{Mode: dsl.EnvironmentPerRun}, itemIndex)
	}
	perRunCalls := sessions.allCreateCalls()[1:4]
	seenIDs := make(map[string]struct{}, len(perRunCalls))
	seenPaths := make(map[string]struct{}, len(perRunCalls))
	for itemIndex, call := range perRunCalls {
		if call.Worktree == "" || call.CWD == "" {
			t.Fatalf("per-run binding[%d] = %#v, want worktree and cwd", itemIndex, call)
		}
		seenIDs[call.Worktree] = struct{}{}
		seenPaths[call.CWD] = struct{}{}
		item, err := worktrees.Get(ctx, workspaceID, call.Worktree)
		if err != nil {
			t.Fatalf("Get(per-run %d) error = %v", itemIndex, err)
		}
		wantRunID := "loop:loop-real-environment:2:review:" + strconv.Itoa(itemIndex)
		if item.Origin != worktreepkg.OriginPerRun || item.RunID != wantRunID || item.Path != call.CWD {
			t.Fatalf("per-run worktree[%d] = %#v, want run %q at %q", itemIndex, item, wantRunID, call.CWD)
		}
		if branch := runLoopEnvironmentGit(t, runner, repository, "branch", "--show-current"); branch != "main" {
			t.Fatalf("parent branch = %q, want main", branch)
		}
		if _, err := os.Stat(filepath.Join(call.CWD, ".git")); err != nil {
			t.Fatalf("Stat(per-run %d .git) error = %v", itemIndex, err)
		}
	}
	if len(seenIDs) != 3 || len(seenPaths) != 3 {
		t.Fatalf("per-run isolation ids=%#v paths=%#v, want three distinct roots", seenIDs, seenPaths)
	}

	bind(dsl.EnvironmentSpec{Mode: dsl.EnvironmentDirectory, Directory: "packages/api"}, 0)
	directoryCall := sessions.allCreateCalls()[4]
	resolvedDirectory, err := session.ResolveSessionCWD(repository, directoryCall.CWD)
	if err != nil {
		t.Fatalf("ResolveSessionCWD(directory) error = %v", err)
	}
	wantDirectory, err := filepath.EvalSymlinks(filepath.Join(repository, "packages", "api"))
	if err != nil {
		t.Fatalf("EvalSymlinks(directory) error = %v", err)
	}
	if resolvedDirectory != wantDirectory || directoryCall.Worktree != "" {
		t.Fatalf("directory binding = %#v resolved=%q, want contained %q without worktree", directoryCall, resolvedDirectory, wantDirectory)
	}
}

type loopEnvironmentWorkspaceResolver struct {
	workspace worktreepkg.Workspace
}

func (r loopEnvironmentWorkspaceResolver) ResolveWorktreeWorkspace(
	_ context.Context,
	workspaceID string,
) (worktreepkg.Workspace, error) {
	if strings.TrimSpace(workspaceID) != r.workspace.ID {
		return worktreepkg.Workspace{}, worktreepkg.ErrNotFound
	}
	return r.workspace, nil
}

func (r loopEnvironmentWorkspaceResolver) ListWorktreeWorkspaces(
	context.Context,
) ([]worktreepkg.Workspace, error) {
	return []worktreepkg.Workspace{r.workspace}, nil
}

func runLoopEnvironmentGit(
	t *testing.T,
	runner worktreepkg.GitRunner,
	dir string,
	args ...string,
) string {
	t.Helper()
	stdout, stderr, err := runner.Run(context.Background(), dir, args...)
	if err != nil {
		t.Fatalf("git %s error = %v; stderr=%q", strings.Join(args, " "), err, strings.TrimSpace(string(stderr)))
	}
	return strings.TrimSpace(string(stdout))
}
