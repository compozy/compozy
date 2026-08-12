package daemon

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/internal/config"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/compozy/compozy/internal/worktree"
)

// Canonical suite: daemon translation from resolved workspaces into the worktree domain.
func TestDaemonWorktreeWorkspaceResolver(t *testing.T) {
	t.Parallel()

	homePaths, err := config.ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	workspaceRoot := t.TempDir()
	worktreesRoot := filepath.Join(t.TempDir(), "workspace-worktrees")
	resolvedConfig := config.DefaultWithHome(homePaths)
	resolvedConfig.Worktrees.Root = worktreesRoot
	resolvedConfig.Worktrees.SetupCommand = "bun install"
	resolved := workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{ID: "ws-resolved", Name: "Resolved", RootDir: workspaceRoot},
		Config:    resolvedConfig,
	}
	stub := &daemonWorktreeResolverStub{
		resolved: resolved,
		listed: []workspacepkg.Workspace{
			resolved.Workspace,
			{ID: "ws-other", Name: "Other", RootDir: t.TempDir()},
		},
	}
	resolver := daemonWorktreeWorkspaceResolver{resolver: stub, homePaths: homePaths}

	got, err := resolver.ResolveWorktreeWorkspace(context.Background(), resolved.ID)
	if err != nil {
		t.Fatalf("ResolveWorktreeWorkspace() error = %v", err)
	}
	if got.ID != resolved.ID || got.Root != workspaceRoot || got.WorktreesRoot != worktreesRoot ||
		got.Worktrees.SetupCommand != "bun install" {
		t.Fatalf("ResolveWorktreeWorkspace() = %#v, want resolved workspace overlay", got)
	}
	listed, err := resolver.ListWorktreeWorkspaces(context.Background())
	if err != nil {
		t.Fatalf("ListWorktreeWorkspaces() error = %v", err)
	}
	if len(listed) != 2 || listed[0].ID != "ws-resolved" || listed[1].ID != "ws-other" {
		t.Fatalf("ListWorktreeWorkspaces() = %#v, want both registered roots", listed)
	}

	empty := daemonWorktreeWorkspaceResolver{homePaths: homePaths}
	if _, err := empty.ResolveWorktreeWorkspace(context.Background(), "missing"); !errors.Is(
		err, worktree.ErrNotFound,
	) {
		t.Fatalf("ResolveWorktreeWorkspace(nil) error = %v, want ErrNotFound", err)
	}
}

type daemonWorktreeResolverStub struct {
	resolved workspacepkg.ResolvedWorkspace
	listed   []workspacepkg.Workspace
}

func (r *daemonWorktreeResolverStub) Resolve(
	context.Context,
	string,
) (workspacepkg.ResolvedWorkspace, error) {
	return r.resolved, nil
}

func (r *daemonWorktreeResolverStub) ResolveOrRegister(
	context.Context,
	string,
) (workspacepkg.ResolvedWorkspace, error) {
	return r.resolved, nil
}

func (r *daemonWorktreeResolverStub) List(context.Context) ([]workspacepkg.Workspace, error) {
	return append([]workspacepkg.Workspace(nil), r.listed...), nil
}
