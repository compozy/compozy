package daemon

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/internal/api/core"

	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/session"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
	toolspkg "github.com/compozy/compozy/internal/tools"
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
		t.Fatalf(
			"MaterializeForRun(after boot) = %#v calls=%#v, want late-bound service",
			item,
			backing.materializeCalls,
		)
	}
}

func TestDaemonForgeRuntimeErrorTranslation(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve unavailable extension errors as forge unavailable", func(t *testing.T) {
		t.Parallel()

		err := mapForgeRuntimeError(fmt.Errorf("provider absent: %w", toolspkg.ErrToolUnavailable))
		if !errors.Is(err, worktree.ErrForgeUnavailable) || errors.Is(err, worktree.ErrForge) {
			t.Fatalf("mapForgeRuntimeError(unavailable) = %v, want only ErrForgeUnavailable", err)
		}
	})

	t.Run("Should classify other extension failures as forge errors", func(t *testing.T) {
		t.Parallel()

		err := mapForgeRuntimeError(errors.New("provider protocol failed"))
		if !errors.Is(err, worktree.ErrForge) || errors.Is(err, worktree.ErrForgeUnavailable) {
			t.Fatalf("mapForgeRuntimeError(protocol) = %v, want only ErrForge", err)
		}
	})

	t.Run("Should classify transient provider causes as forge errors", func(t *testing.T) {
		t.Parallel()
		for _, cause := range []string{"rate_limited", "credential_expired"} {
			t.Run("Should classify "+cause, func(t *testing.T) {
				t.Parallel()
				if got := forgeFailureKind(cause); !errors.Is(got, worktree.ErrForge) {
					t.Fatalf("forgeFailureKind(%q) = %v, want ErrForge", cause, got)
				}
			})
		}
	})
}

func TestDaemonSessionWorktreeResolver(t *testing.T) {
	t.Parallel()

	t.Run("Should resolve only ready attached worktrees", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		var gotWorkspace, gotRef string
		resolver := daemonSessionWorktreeResolver{lookup: func() sessionWorktreeLookup {
			return sessionWorktreeLookupFunc(
				func(_ context.Context, workspaceID, ref string) (*worktree.Worktree, error) {
					gotWorkspace, gotRef = workspaceID, ref
					return &worktree.Worktree{
						ID:          "wt-ready",
						WorkspaceID: workspaceID,
						Path:        root,
						State:       worktree.StateReady,
					}, nil
				},
			)
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
	notify(t.Context(), session.PromptRunIdentity{SessionID: "sess-bound"})
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

// Terminal root authority belongs to the daemon's active session/worktree binding.
func TestDaemonTerminalExecutionRoot(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"bound", "unbound", "foreign workspace", "foreign profile", "foreign run", "stale generation", "missing worktree", "mismatched worktree", "foreign worktree profile", "inactive run", "missing session", "pending worktree", "backend unavailable"} {
		t.Run("Should resolve or reject "+name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			info := &session.Info{
				ID:                "sess-a",
				WorkspaceID:       "ws-a",
				ProfileID:         "profile-a",
				WorktreeID:        "wt-a",
				RuntimeGeneration: 2,
			}
			active := session.PromptRunIdentity{
				SessionID:   info.ID,
				WorkspaceID: info.WorkspaceID,
				ProfileID:   info.ProfileID,
				RunID:       "run-a",
				Generation:  2,
			}
			actor := terminalpkg.Actor{
				Kind:       terminalpkg.ActorKindAgent,
				ID:         "agent-a",
				SessionID:  info.ID,
				ProfileID:  info.ProfileID,
				RunID:      active.RunID,
				Generation: 2,
			}
			switch name {
			case "unbound":
				info.WorktreeID = ""
			case "foreign workspace":
				active.WorkspaceID = "ws-b"
			case "foreign profile":
				actor.ProfileID = "profile-b"
			case "foreign run":
				actor.RunID = "run-b"
			case "stale generation":
				actor.Generation = 1
			}
			sessions := &fakeSessionManager{
				infos: []*session.Info{info},
				activePromptRunHook: func(context.Context, string) (session.PromptRunIdentity, error) {
					if name == "inactive run" {
						return session.PromptRunIdentity{}, session.ErrPromptNotActive
					}
					return active, nil
				},
			}
			if name == "missing session" {
				sessions.infos = nil
			}
			resolver := daemonSessionWorktreeResolver{lookup: func() sessionWorktreeLookup {
				return sessionWorktreeLookupFunc(func(_ context.Context, ws, ref string) (*worktree.Worktree, error) {
					if ws != "ws-a" || ref != "wt-a" {
						t.Fatalf("lookup = %q, %q", ws, ref)
					}
					if name == "missing worktree" {
						return nil, worktree.ErrMissing
					}
					id := ref
					if name == "mismatched worktree" {
						id = "wt-b"
					}
					profileID, state := "profile-a", worktree.StateReady
					if name == "foreign worktree profile" {
						profileID = "profile-b"
					}
					if name == "pending worktree" {
						state = worktree.StatePending
					}
					if name == "backend unavailable" {
						return nil, errors.New("private backend detail")
					}
					return &worktree.Worktree{
						ID:          id,
						WorkspaceID: ws,
						ProfileID:   profileID,
						Path:        root,
						State:       state,
					}, nil
				})
			}}
			got, err := resolveTerminalExecutionRoot(t.Context(), sessions, resolver, "ws-a", actor)
			switch name {
			case "bound":
				if err != nil || got != root {
					t.Fatalf("bound root = %q, %v", got, err)
				}
			case "unbound":
				if err != nil || got != "" {
					t.Fatalf("unbound root = %q, %v", got, err)
				}
			default:
				wantErr, wantCode := terminalpkg.ErrNotFound, terminalpkg.ErrorCodeNotFound
				switch name {
				case "stale generation", "inactive run":
					wantErr, wantCode = terminalpkg.ErrGenerationFenced, terminalpkg.ErrorCodeGenerationFenced
				case "missing worktree":
					wantErr, wantCode = worktree.ErrMissing, terminalpkg.ErrorCodeInvalidCwd
				case "pending worktree":
					wantErr, wantCode = worktree.ErrNotReady, terminalpkg.ErrorCodeInvalidCwd
				case "backend unavailable":
					wantErr, wantCode = terminalpkg.ErrServiceUnavailable, ""
				}
				if !errors.Is(err, wantErr) || got != "" {
					t.Fatalf("invalid binding root=%q error=%v, want %v", got, err, wantErr)
				}
				wantStatus := http.StatusNotFound
				switch wantCode {
				case terminalpkg.ErrorCodeGenerationFenced:
					wantStatus = http.StatusConflict
				case terminalpkg.ErrorCodeInvalidCwd:
					wantStatus = http.StatusUnprocessableEntity
				case "":
					wantStatus = http.StatusServiceUnavailable
				}
				status, _, _ := core.TerminalErrorStatus(err)
				if status != wantStatus {
					t.Fatalf("HTTP status = %d, want %d", status, wantStatus)
				}
				nativeErr := terminalToolError(toolspkg.ToolIDTerminalExec, err)
				toolErr, ok := errors.AsType[*toolspkg.ToolError](nativeErr)
				wantNativeCode := toolspkg.ErrorCode(wantCode)
				if wantCode == "" {
					wantNativeCode = toolspkg.ErrorCodeUnavailable
				}
				if !ok || toolErr.Code != wantNativeCode {
					t.Fatalf("native error = %v, want %s", nativeErr, wantNativeCode)
				}
				if wantCode != "" {
					typed, ok := errors.AsType[*terminalpkg.Error](err)
					if !ok || typed.Code != wantCode {
						t.Fatalf("error=%v, want code %s", err, wantCode)
					}
				}
			}
		})
	}
}
