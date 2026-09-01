package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
	goalpkg "github.com/compozy/compozy/internal/loop/goal"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/worktree"
)

type loopActionWorktrees interface {
	loopActionWorktreeReader
	MaterializeForRun(context.Context, string, worktree.RunWorktreeRequest) (*worktree.Worktree, error)
	RollbackRunMaterialization(context.Context, string, string, string) error
}

func (b *loopActionSessionBinder) resolveRunOwnedBindingProfile(
	ctx context.Context,
	req looppkg.ActionSessionBindRequest,
	active goalpkg.SessionBinding,
	activeFound bool,
) (store.SessionCreationProfile, session.CreateOpts, *worktree.Worktree, error) {
	profileRef := strings.TrimSpace(req.PinnedCreationProfileRef)
	if activeFound {
		profileRef = active.CreationProfileRef
	}
	profile, opts, materialized, err := b.resolveEffectiveCreationProfile(ctx, req, profileRef)
	if err != nil {
		return store.SessionCreationProfile{}, session.CreateOpts{}, nil, err
	}
	opts.ProvenanceParentSessionID = strings.TrimSpace(req.ProvenanceParentSessionID)
	policyDigest, err := profile.PolicySpecDigest()
	if err != nil {
		return store.SessionCreationProfile{}, session.CreateOpts{}, nil, err
	}
	if activeFound && (active.CreationProfileRef != profileRef || active.PolicySpecDigest != policyDigest) {
		return store.SessionCreationProfile{}, session.CreateOpts{}, nil, bindingMismatch(
			"active binding policy/profile drifted",
		)
	}
	return profile, opts, materialized, nil
}

func applyLoopDirectoryBeforePolicy(opts *session.CreateOpts, environment dsl.EnvironmentSpec) {
	if opts != nil && environment.Mode == dsl.EnvironmentDirectory {
		opts.CWD = strings.TrimSpace(environment.Directory)
	}
}

func (b *loopActionSessionBinder) applyLoopActionEnvironment(
	ctx context.Context,
	opts *session.CreateOpts,
	req looppkg.ActionSessionBindRequest,
) (*worktree.Worktree, error) {
	if opts == nil {
		return nil, errors.New("daemon: loop action session options are required")
	}
	environment, err := looppkg.ResolveActionEnvironment(req.EnvironmentValue(), dsl.EnvironmentSpec{})
	if err != nil {
		return nil, err
	}
	switch environment.Mode {
	case dsl.EnvironmentRoot, dsl.EnvironmentDirectory:
		return nil, nil
	case dsl.EnvironmentWorktree:
		item, getErr := resolveReadyLoopActionWorktree(
			ctx,
			b.worktrees,
			strings.TrimSpace(string(req.WorkspaceID)),
			environment.WorktreeRef,
		)
		if getErr != nil {
			return nil, getErr
		}
		opts.Worktree = item.ID
		opts.CWD = item.Path
		return nil, nil
	case dsl.EnvironmentPerRun:
		if b.worktrees == nil {
			return nil, fmt.Errorf("%w: loop worktree service is unavailable", worktree.ErrPerRunMaterialization)
		}
		workspaceID := strings.TrimSpace(string(req.WorkspaceID))
		item, materializeErr := b.worktrees.MaterializeForRun(ctx, workspaceID, worktree.RunWorktreeRequest{
			ProfileID: strings.TrimSpace(req.ProfileID),
			TaskSlug:  loopActionWorktreeSlug(req),
			RunID:     loopActionWorktreeRunID(req),
		})
		if materializeErr != nil {
			return nil, materializeErr
		}
		if item == nil {
			return nil, fmt.Errorf("%w: loop materializer returned nil worktree", worktree.ErrPerRunMaterialization)
		}
		opts.Worktree = item.ID
		opts.CWD = item.Path
		return item, nil
	default:
		return nil, fmt.Errorf("%w: unsupported loop environment mode %q", looppkg.ErrValidation, environment.Mode)
	}
}

func loopActionWorktreeSlug(req looppkg.ActionSessionBindRequest) string {
	return fmt.Sprintf("loop-%s-%s-%d", req.LoopRunID, req.NodeID, req.ItemIndex)
}

func loopActionWorktreeRunID(req looppkg.ActionSessionBindRequest) string {
	return fmt.Sprintf("loop:%s:%d:%s:%d", req.LoopRunID, req.Generation, req.NodeID, req.ItemIndex)
}

func (b *loopActionSessionBinder) rollbackLoopActionEnvironment(
	ctx context.Context,
	req looppkg.ActionSessionBindRequest,
	item *worktree.Worktree,
	cause error,
) error {
	if item == nil || b.worktrees == nil {
		return cause
	}
	rollbackErr := b.worktrees.RollbackRunMaterialization(
		context.WithoutCancel(ctx),
		item.WorkspaceID,
		item.ID,
		loopActionWorktreeRunID(req),
	)
	if rollbackErr != nil {
		rollbackErr = fmt.Errorf("daemon: roll back loop per-run worktree: %w", rollbackErr)
	}
	return errors.Join(cause, rollbackErr)
}

func (b *loopActionSessionBinder) stopAndRollbackLoopActionEnvironment(
	ctx context.Context,
	req looppkg.ActionSessionBindRequest,
	sessionID string,
	item *worktree.Worktree,
	cause error,
) error {
	if item == nil {
		return cause
	}
	var stopErr error
	if stopper, ok := b.sessions.(interface {
		StopWithCause(context.Context, string, session.StopCause, string) error
	}); ok && strings.TrimSpace(sessionID) != "" {
		stopErr = stopper.StopWithCause(
			context.WithoutCancel(ctx),
			strings.TrimSpace(sessionID),
			session.CauseFailed,
			"loop action session failed before worktree binding completed",
		)
	}
	return b.rollbackLoopActionEnvironment(ctx, req, item, errors.Join(cause, stopErr))
}
