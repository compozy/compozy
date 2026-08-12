package daemon

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/config"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/compozy/compozy/internal/worktree"
)

type worktreeRegistry interface {
	worktree.EventSink
	WorktreeStore() worktree.Store
	HasActiveWorktreeSession(context.Context, string, string) (bool, error)
}

type daemonWorktreeSessionGuard struct {
	registry worktreeRegistry
}

func (g daemonWorktreeSessionGuard) HasActiveSession(
	ctx context.Context,
	workspaceID string,
	worktreeID string,
) (bool, error) {
	return g.registry.HasActiveWorktreeSession(ctx, workspaceID, worktreeID)
}

type daemonWorktreeWorkspaceResolver struct {
	resolver interface {
		workspacepkg.RuntimeResolver
		List(context.Context) ([]workspacepkg.Workspace, error)
	}
	homePaths config.HomePaths
}

func (r daemonWorktreeWorkspaceResolver) ResolveWorktreeWorkspace(
	ctx context.Context,
	workspaceID string,
) (worktree.Workspace, error) {
	if r.resolver == nil {
		return worktree.Workspace{}, worktree.ErrNotFound
	}
	resolved, err := r.resolver.Resolve(ctx, workspaceID)
	if err != nil {
		return worktree.Workspace{}, fmt.Errorf("daemon: resolve worktree workspace: %w", err)
	}
	return worktree.Workspace{
		ID: resolved.ID, Name: resolved.Name, Root: resolved.RootDir,
		Worktrees:     resolved.Config.Worktrees,
		WorktreesRoot: resolved.Config.Worktrees.ResolveRoot(r.homePaths),
	}, nil
}

func (r daemonWorktreeWorkspaceResolver) ListWorktreeWorkspaces(ctx context.Context) ([]worktree.Workspace, error) {
	if r.resolver == nil {
		return nil, worktree.ErrNotFound
	}
	registered, err := r.resolver.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("daemon: list worktree workspaces: %w", err)
	}
	result := make([]worktree.Workspace, 0, len(registered))
	for _, item := range registered {
		result = append(result, worktree.Workspace{ID: item.ID, Name: item.Name, Root: item.RootDir})
	}
	return result, nil
}

type daemonWorktreeHookDispatcher struct {
	hooks *hookspkg.Hooks
	now   func() time.Time
}

func (d daemonWorktreeHookDispatcher) DispatchWorktreeHook(
	ctx context.Context,
	request worktree.HookRequest,
) (worktree.HookVerdict, error) {
	if d.hooks == nil {
		return worktree.HookVerdict{}, nil
	}
	now := time.Now().UTC()
	if d.now != nil {
		now = d.now().UTC()
	}
	base := hookspkg.PayloadBase{Event: hookspkg.HookEvent(request.Event), Timestamp: now}
	var err error
	switch request.Event {
	case worktree.EventPreCreate:
		item, ok := request.Payload.(worktree.HookWorktree)
		if !ok {
			return worktree.HookVerdict{}, fmt.Errorf("daemon: invalid worktree pre-create payload")
		}
		_, err = d.hooks.DispatchWorktreePreCreate(ctx, hookspkg.WorktreePreCreatePayload{
			PayloadBase: base, WorktreeContext: worktreeHookContext(item),
		})
	case worktree.EventPreRemove:
		payload, ok := request.Payload.(worktree.HookPreRemovePayload)
		if !ok {
			return worktree.HookVerdict{}, fmt.Errorf("daemon: invalid worktree pre-remove payload")
		}
		_, err = d.hooks.DispatchWorktreePreRemove(ctx, hookspkg.WorktreePreRemovePayload{
			PayloadBase: base, Worktree: worktreeHookContext(payload.Worktree), Force: payload.Force,
			Risk: hookspkg.WorktreeRisk{
				ChangedFiles: payload.Risk.ChangedFiles, Insertions: payload.Risk.Insertions,
				Deletions: payload.Risk.Deletions, UnpushedCommits: payload.Risk.UnpushedCommits,
				ExistsOnRemote: payload.Risk.ExistsOnRemote,
			},
		})
	case worktree.EventCreated, worktree.EventAdopted, worktree.EventRemoved:
		item, ok := request.Payload.(worktree.HookWorktree)
		if !ok {
			return worktree.HookVerdict{}, fmt.Errorf("daemon: invalid worktree observation payload")
		}
		payload := hookspkg.WorktreeObservationPayload{
			PayloadBase: base, WorktreeContext: worktreeHookContext(item),
		}
		switch request.Event {
		case worktree.EventCreated:
			_, err = d.hooks.DispatchWorktreeCreated(ctx, payload)
		case worktree.EventAdopted:
			_, err = d.hooks.DispatchWorktreeAdopted(ctx, payload)
		case worktree.EventRemoved:
			_, err = d.hooks.DispatchWorktreeRemoved(ctx, payload)
		}
	default:
		return worktree.HookVerdict{}, nil
	}
	var denied *hookspkg.DeniedError
	if errors.As(err, &denied) {
		return worktree.HookVerdict{
			Denied: true, HookName: denied.HookName, Reason: denied.Reason,
		}, nil
	}
	return worktree.HookVerdict{}, err
}

func worktreeHookContext(item worktree.HookWorktree) hookspkg.WorktreeContext {
	return hookspkg.WorktreeContext{
		WorktreeID: item.WorktreeID, WorkspaceID: item.WorkspaceID,
		WorkspaceRoot: item.WorkspaceRoot, Name: item.Name, Branch: item.Branch,
		Path: item.Path, Origin: string(item.Origin),
	}
}

func (d *Daemon) bootWorktrees(ctx context.Context, state *bootState) error {
	if state == nil || state.registry == nil || state.workspaceResolver == nil {
		return nil
	}
	registry, ok := state.registry.(worktreeRegistry)
	if !ok {
		state.logger.Debug("daemon: worktree service skipped for test registry")
		return nil
	}
	runner, err := worktree.NewRealGitRunner(0)
	if err != nil {
		state.logger.Warn("daemon: Git unavailable for worktree operations", "error", err)
		runner = nil
	}
	root := state.cfg.Worktrees.ResolveRoot(d.homePaths)
	service := worktree.NewService(
		registry.WorktreeStore(),
		runner,
		worktree.WithWorkspaceResolver(daemonWorktreeWorkspaceResolver{
			resolver: state.workspaceResolver, homePaths: d.homePaths,
		}),
		worktree.WithConfig(state.cfg.Worktrees, root),
		worktree.WithHooks(daemonWorktreeHookDispatcher{hooks: state.hookDispatcher, now: d.now}),
		worktree.WithEvents(registry),
		worktree.WithSessionGuard(daemonWorktreeSessionGuard{registry: registry}),
		worktree.WithLogger(state.logger),
		worktree.WithClock(d.now),
	)
	if err := service.RecoverCreations(ctx); err != nil {
		return fmt.Errorf("daemon: recover worktree lifecycle: %w", err)
	}
	state.worktrees = service
	state.deps.Worktrees = service
	return nil
}
