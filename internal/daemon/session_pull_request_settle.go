package daemon

import (
	"context"
	"log/slog"

	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/workspaceaccess"
	"github.com/compozy/compozy/internal/worktree"
)

type sessionBranchStore interface {
	session.BranchSnapshotReader
	SaveSessionBranch(context.Context, string, string, string) error
}

func registerSessionPullRequests(
	sessions SessionManager,
	store sessionBranchStore,
	reader sessionPullRequestAdapter,
	branches session.BranchReader,
	logger *slog.Logger,
) {
	projection, ok := sessions.(interface {
		ConfigurePullRequests(session.PullRequestReader, session.BranchSnapshotReader)
	})
	if ok {
		projection.ConfigurePullRequests(reader, store)
	}
	saveBranch := sessionBranchSaver(store, reader, branches)
	if saver, ok := sessions.(interface {
		ConfigureBranchSnapshotSaver(session.BranchSnapshotSaver)
	}); ok {
		saver.ConfigureBranchSnapshotSaver(saveBranch)
	}
	registrar, ok := sessions.(turnEndNotifierRegistrar)
	if !ok {
		return
	}
	refresh := func(ctx context.Context, identity session.PromptRunIdentity) {
		info, err := sessions.Status(ctx, identity.SessionID)
		if err != nil || info == nil || info.WorkspaceID == "" {
			return
		}
		if err := saveBranch(ctx, info); err != nil {
			logger.WarnContext(ctx, "session branch snapshot failed", "session_id", info.ID, "error", err)
			return
		}
		if publisher, ok := sessions.(interface{ PublishSessionCatalogChanged(context.Context, string) }); ok {
			publisher.PublishSessionCatalogChanged(ctx, info.ID)
		}
		snapshot, err := store.SessionBranches(ctx, []string{info.ID})
		if err != nil || snapshot[info.ID] == "" {
			return
		}
		workspace, err := reader.resolver.ResolveWorktreeWorkspace(ctx, info.WorkspaceID, "")
		if err != nil {
			return
		}
		if reader.service.IsDefaultBranch(ctx, workspace.Root, snapshot[info.ID]) {
			return
		}
		repo, err := reader.service.RepoKey(ctx, workspace.Root)
		if err != nil {
			return
		}
		reader.service.RequestRefresh(
			worktree.PullRequestKey{WorkspaceID: info.WorkspaceID, RepoKey: repo, Branch: snapshot[info.ID]},
		)
	}
	registrar.AddTurnEndNotifier(refresh)
	if bindings, ok := sessions.(interface {
		AddWorktreeBindingNotifier(session.WorktreeBindingNotifier)
	}); ok {
		bindings.AddWorktreeBindingNotifier(func(ctx context.Context, info *session.Info, _ string) {
			refresh(ctx, session.PromptRunIdentity{SessionID: info.ID})
		})
	}
}

// sessionBranchSaver records the current branch of a session in its workspace root;
// worktree sessions keep the branch bound to their worktree.
func sessionBranchSaver(
	store sessionBranchStore,
	reader sessionPullRequestAdapter,
	branches session.BranchReader,
) session.BranchSnapshotSaver {
	return func(ctx context.Context, info *session.Info) error {
		if info == nil || info.WorkspaceID == "" || info.WorkspaceID == workspaceaccess.UserSelector ||
			info.WorktreeID != "" {
			return nil
		}
		workspace, err := reader.resolver.ResolveWorktreeWorkspace(ctx, info.WorkspaceID, "")
		if err != nil {
			return err
		}
		branch, err := branches.CurrentBranch(ctx, workspace.Root)
		if err != nil {
			return err
		}
		return store.SaveSessionBranch(ctx, info.WorkspaceID, info.ID, branch)
	}
}
