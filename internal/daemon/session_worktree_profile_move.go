package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/worktree"
)

// profileMoveWorktreeService is the worktree surface a session move to another profile uses.
type profileMoveWorktreeService interface {
	Get(context.Context, string, string) (*worktree.Worktree, error)
	Status(context.Context, string, string, bool) (*worktree.Status, error)
	RemoveKeepingBranch(context.Context, string, string) (*worktree.RemovalRefusal, error)
	Create(context.Context, string, worktree.CreateOptions) (*worktree.Worktree, error)
}

var _ session.ProfileMoveWorktrees = daemonSessionWorktreeResolver{}

type profileMoveRecoverer interface {
	RecoverProfileMoves(context.Context) error
}

// recoverBootProfileMoves settles the moves to another profile a daemon stop left
// open. It runs after worktree recovery, so every interrupted removal or creation
// has already finished or reverted. A move that cannot settle stays journaled for
// the next attempt and never fails startup.
func recoverBootProfileMoves(ctx context.Context, state *bootState) error {
	recoverer, ok := state.sessions.(profileMoveRecoverer)
	if !ok {
		return nil
	}
	if err := recoverer.RecoverProfileMoves(ctx); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return errors.Join(err, ctxErr)
		}
		state.logger.Warn("daemon: an interrupted move to another profile is not settled yet", "error", err)
	}
	return nil
}

func (r daemonSessionWorktreeResolver) profileMoveService() (profileMoveWorktreeService, error) {
	service, ok := r.service().(profileMoveWorktreeService)
	if !ok || service == nil {
		return nil, errors.New("daemon: worktree service is unavailable")
	}
	return service, nil
}

// InspectProfileMoveWorktree reads the checkout with a fresh Git status, so the move
// sees uncommitted changes made after the last cached refresh. A worktree that was
// never recorded reports session.ErrProfileMoveWorktreeNotFound; one whose checkout
// no longer exists reports Gone.
func (r daemonSessionWorktreeResolver) InspectProfileMoveWorktree(
	ctx context.Context,
	workspaceID, worktreeID string,
) (session.ProfileMoveCheckout, error) {
	service, err := r.profileMoveService()
	if err != nil {
		return session.ProfileMoveCheckout{}, err
	}
	item, err := service.Get(ctx, workspaceID, worktreeID)
	if errors.Is(err, worktree.ErrNotFound) {
		return session.ProfileMoveCheckout{}, fmt.Errorf("%w: %s", session.ErrProfileMoveWorktreeNotFound, worktreeID)
	}
	if err != nil {
		return session.ProfileMoveCheckout{}, err
	}
	checkout := session.ProfileMoveCheckout{
		ID: item.ID, Branch: item.Branch, Ready: item.State == worktree.StateReady,
		Gone:         profileMoveCheckoutGone(item),
		RunNamespace: strings.TrimSpace(item.RunNamespace),
	}
	if !checkout.Ready {
		return checkout, nil
	}
	status, err := service.Status(ctx, workspaceID, item.ID, true)
	if err != nil {
		return session.ProfileMoveCheckout{}, fmt.Errorf("daemon: read worktree status: %w", err)
	}
	if status.ReadError != "" {
		return session.ProfileMoveCheckout{}, fmt.Errorf("daemon: read worktree status: %s", status.ReadError)
	}
	if status.Branch != nil {
		checkout.Branch = strings.TrimSpace(*status.Branch)
	}
	if status.HeadSHA != nil {
		checkout.HeadSHA = strings.TrimSpace(*status.HeadSHA)
	}
	checkout.Detached = status.Detached != nil && *status.Detached
	if status.DirtyFiles != nil {
		checkout.DirtyFiles = *status.DirtyFiles
	}
	return checkout, nil
}

// profileMoveCheckoutGone reports a row whose checkout no longer exists. A failed
// creation counts only once its rollback completed; until then Git may still hold
// the branch.
func profileMoveCheckoutGone(item *worktree.Worktree) bool {
	switch item.State {
	case worktree.StateRemoved, worktree.StateDismissed:
		return true
	case worktree.StateFailed:
		return item.CreateRollbackError == ""
	default:
		return false
	}
}

// ReleaseProfileMoveWorktree removes the checkout but never its branch: a clean checkout
// whose commits are not pushed is still removed, because its branch keeps them, while
// uncommitted files refuse the removal even if a removal hook writes them.
func (r daemonSessionWorktreeResolver) ReleaseProfileMoveWorktree(
	ctx context.Context,
	workspaceID, worktreeID string,
) error {
	service, err := r.profileMoveService()
	if err != nil {
		return err
	}
	_, err = service.RemoveKeepingBranch(ctx, workspaceID, worktreeID)
	if errors.Is(err, worktree.ErrSessionBound) {
		return &session.ProfileMoveError{
			Reason:  session.ProfileMoveRefusalWorktreeShared,
			Message: "another session still uses this worktree; archive it or bind it elsewhere first",
		}
	}
	if errors.Is(err, worktree.ErrDirtyRequiresForce) {
		return &session.ProfileMoveError{
			Reason:  session.ProfileMoveRefusalWorktreeDirty,
			Message: "the session's worktree has uncommitted changes; commit or discard them first",
		}
	}
	return err
}

// CreateProfileMoveWorktree checks the branch out in a new worktree with the move's
// chosen id, owned by profileID, recreating the branch at the recorded head when it
// no longer exists.
func (r daemonSessionWorktreeResolver) CreateProfileMoveWorktree(
	ctx context.Context,
	workspaceID, profileID, worktreeID string,
	checkout session.ProfileMoveCheckout,
) (string, string, error) {
	service, err := r.profileMoveService()
	if err != nil {
		return "", "", err
	}
	item, err := service.Create(ctx, workspaceID, worktree.CreateOptions{
		ID: worktreeID, ProfileID: profileID, ExistingBranch: checkout.Branch,
	})
	if errors.Is(err, worktree.ErrBaseRefNotFound) {
		item, err = service.Create(ctx, workspaceID, worktree.CreateOptions{
			ID: worktreeID, ProfileID: profileID, Branch: checkout.Branch, BaseRef: checkout.HeadSHA,
		})
	}
	if err != nil {
		return "", "", err
	}
	if item == nil || item.State != worktree.StateReady {
		return "", "", worktree.ErrNotReady
	}
	return item.ID, item.Path, nil
}
