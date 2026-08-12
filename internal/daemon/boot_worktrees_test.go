package daemon

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/session"
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

func TestDaemonExecutionWorktreesResolveServiceAfterConsumerBoot(t *testing.T) {
	t.Parallel()

	var current executionWorktreeService
	resolver := daemonExecutionWorktrees{lookup: func() executionWorktreeService { return current }}
	if _, err := resolver.MaterializeForRun(
		context.Background(),
		"ws-late",
		worktree.RunWorktreeRequest{TaskSlug: "late", RunID: "run-late"},
	); !errors.Is(err, worktree.ErrPerRunMaterialization) {
		t.Fatalf("MaterializeForRun(before boot) error = %v, want %v", err, worktree.ErrPerRunMaterialization)
	}

	backing := &recordingTaskBridgeWorktrees{materialized: &worktree.Worktree{
		ID: "wt-late", WorkspaceID: "ws-late", RunID: "run-late",
	}}
	current = backing
	item, err := resolver.MaterializeForRun(
		context.Background(),
		"ws-late",
		worktree.RunWorktreeRequest{TaskSlug: "late", RunID: "run-late"},
	)
	if err != nil {
		t.Fatalf("MaterializeForRun(after boot) error = %v", err)
	}
	if item == nil || item.ID != "wt-late" || len(backing.materializeCalls) != 1 {
		t.Fatalf("MaterializeForRun(after boot) = %#v calls=%#v, want late-bound service", item, backing.materializeCalls)
	}
}

func TestDaemonSessionWorktreeResolver(t *testing.T) {
	t.Parallel()

	t.Run("Should resolve only ready attached worktrees", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		var gotWorkspace, gotRef string
		resolver := daemonSessionWorktreeResolver{lookup: func() sessionWorktreeLookup {
			return sessionWorktreeLookupFunc(func(_ context.Context, workspaceID, ref string) (*worktree.Worktree, error) {
				gotWorkspace, gotRef = workspaceID, ref
				return &worktree.Worktree{ID: "wt-ready", WorkspaceID: workspaceID, Path: root, State: worktree.StateReady}, nil
			})
		}}

		id, gotRoot, err := resolver.ResolveSessionWorktree(context.Background(), " ws-1 ", " wt-ref ")
		if err != nil {
			t.Fatalf("ResolveSessionWorktree(ready) error = %v", err)
		}
		if id != "wt-ready" || gotRoot != root || gotWorkspace != "ws-1" || gotRef != "wt-ref" {
			t.Fatalf(
				"ResolveSessionWorktree(ready) = (%q, %q), lookup (%q, %q)",
				id,
				gotRoot,
				gotWorkspace,
				gotRef,
			)
		}
	})

	for _, test := range []struct {
		name    string
		state   worktree.State
		getErr  error
		path    string
		wantErr error
	}{
		{name: "unknown", getErr: worktree.ErrNotFound, wantErr: worktree.ErrNotFound},
		{name: "pending", state: worktree.StatePending, wantErr: worktree.ErrNotReady},
		{name: "failed", state: worktree.StateFailed, wantErr: worktree.ErrNotReady},
		{name: "removing", state: worktree.StateRemoving, wantErr: worktree.ErrNotReady},
		{name: "missing", state: worktree.StateMissing, wantErr: worktree.ErrMissing},
		{name: "removed", state: worktree.StateRemoved, wantErr: worktree.ErrMissing},
		{name: "ready with vanished path", state: worktree.StateReady, path: filepath.Join(t.TempDir(), "gone"), wantErr: worktree.ErrMissing},
	} {
		t.Run("Should classify "+test.name+" deterministically", func(t *testing.T) {
			t.Parallel()

			resolver := daemonSessionWorktreeResolver{lookup: func() sessionWorktreeLookup {
				return sessionWorktreeLookupFunc(func(context.Context, string, string) (*worktree.Worktree, error) {
					if test.getErr != nil {
						return nil, test.getErr
					}
					return &worktree.Worktree{ID: "wt-target", Path: test.path, State: test.state}, nil
				})
			}}
			_, _, err := resolver.ResolveSessionWorktree(context.Background(), "ws-1", "target")
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("ResolveSessionWorktree(%s) error = %v, want %v", test.name, err, test.wantErr)
			}
		})
	}
}

func TestWorktreeTurnRefreshNotifier(t *testing.T) {
	t.Parallel()

	refresher := &recordingTurnWorktreeRefresher{}
	notify := worktreeTurnRefreshNotifier(
		turnWorktreeSessionReader{info: &session.Info{
			ID: "sess-bound", WorkspaceID: "ws-bound", WorktreeID: "wt-bound",
		}},
		refresher,
		nil,
	)
	notify("sess-bound")
	if refresher.workspaceID != "ws-bound" || refresher.worktreeID != "wt-bound" || !refresher.refresh {
		t.Fatalf(
			"Status() = workspace %q worktree %q refresh %v, want exact bound worktree refresh",
			refresher.workspaceID,
			refresher.worktreeID,
			refresher.refresh,
		)
	}
}

type turnWorktreeSessionReader struct {
	info *session.Info
}

func (r turnWorktreeSessionReader) Status(context.Context, string) (*session.Info, error) {
	return r.info, nil
}

type recordingTurnWorktreeRefresher struct {
	workspaceID string
	worktreeID  string
	refresh     bool
}

func (r *recordingTurnWorktreeRefresher) Status(
	_ context.Context,
	workspaceID string,
	worktreeID string,
	refresh bool,
) (*worktree.Status, error) {
	r.workspaceID = workspaceID
	r.worktreeID = worktreeID
	r.refresh = refresh
	return &worktree.Status{WorktreeID: worktreeID}, nil
}

type sessionWorktreeLookupFunc func(context.Context, string, string) (*worktree.Worktree, error)

func (f sessionWorktreeLookupFunc) Get(
	ctx context.Context,
	workspaceID string,
	ref string,
) (*worktree.Worktree, error) {
	return f(ctx, workspaceID, ref)
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
