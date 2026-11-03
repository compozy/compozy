package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

// ProfileMoveRefusal names the rule that refused a move to another profile.
type ProfileMoveRefusal string

const (
	ProfileMoveRefusalSameProfile      ProfileMoveRefusal = "same_profile"
	ProfileMoveRefusalOwnSession       ProfileMoveRefusal = "own_session"
	ProfileMoveRefusalNotMovable       ProfileMoveRefusal = "not_movable"
	ProfileMoveRefusalRunning          ProfileMoveRefusal = "running"
	ProfileMoveRefusalArchived         ProfileMoveRefusal = "archived"
	ProfileMoveRefusalHasChildren      ProfileMoveRefusal = "has_children"
	ProfileMoveRefusalWorktreeDirty    ProfileMoveRefusal = "worktree_dirty"
	ProfileMoveRefusalWorktreeDetached ProfileMoveRefusal = "worktree_detached"
	ProfileMoveRefusalWorktreeShared   ProfileMoveRefusal = "worktree_shared"
	ProfileMoveRefusalWorktreeRunOwned ProfileMoveRefusal = "worktree_run_owned"
	ProfileMoveRefusalWorktreeNotReady ProfileMoveRefusal = "worktree_not_ready"
	// ProfileMoveRefusalWorktreeChanged reports a worktree binding that changed
	// between planning and the archive; nothing else changed and a retry re-plans.
	ProfileMoveRefusalWorktreeChanged ProfileMoveRefusal = "worktree_changed"
	// ProfileMoveRefusalRecoveryIncomplete reports a failed move whose rollback did
	// not finish; the message names what is left and how to finish it by hand.
	ProfileMoveRefusalRecoveryIncomplete ProfileMoveRefusal = "recovery_incomplete"
)

// ErrProfileMoveRefused matches every ProfileMoveError.
var ErrProfileMoveRefused = errors.New("session: move to profile refused")

// ProfileMoveError is a move-to-profile refusal with its rule and operator-facing reason.
type ProfileMoveError struct {
	Reason  ProfileMoveRefusal
	Message string
	cause   error
}

func (e *ProfileMoveError) Error() string {
	return e.Message
}

func (e *ProfileMoveError) Unwrap() []error {
	if e.cause == nil {
		return []error{ErrProfileMoveRefused}
	}
	return []error{ErrProfileMoveRefused, e.cause}
}

func profileMoveRefused(reason ProfileMoveRefusal, format string, args ...any) error {
	return &ProfileMoveError{Reason: reason, Message: fmt.Sprintf(format, args...)}
}

// ProfileMoveCheckout is the source session's worktree as a move sees it.
type ProfileMoveCheckout struct {
	ID           string
	Branch       string
	HeadSHA      string
	Ready        bool
	Detached     bool
	DirtyFiles   int
	RunNamespace string
}

// ProfileMoveWorktrees releases a stopped session's worktree and recreates its
// branch in another profile. The daemon's worktree resolver implements it.
type ProfileMoveWorktrees interface {
	InspectProfileMoveWorktree(ctx context.Context, workspaceID, worktreeID string) (ProfileMoveCheckout, error)
	// ReleaseProfileMoveWorktree removes the checkout and keeps its branch and commits.
	ReleaseProfileMoveWorktree(ctx context.Context, workspaceID, worktreeID string) error
	// CreateProfileMoveWorktree checks the branch out again in a new worktree owned by
	// profileID, recreating the branch at the recorded head if it no longer exists.
	CreateProfileMoveWorktree(
		ctx context.Context, workspaceID, profileID string, checkout ProfileMoveCheckout,
	) (id string, root string, err error)
}

// MoveToProfileOpts moves one stopped user session into another profile.
type MoveToProfileOpts struct {
	// WorkspaceID is the source's workspace; "" is the User scope.
	WorkspaceID string
	// ProfileID is the caller's profile, which must own the source.
	ProfileID       string
	SessionID       string
	TargetProfileID string
}

// MoveToProfileResult is a completed move: the copy in the target profile and the
// archived original.
type MoveToProfileResult struct {
	Copy     *Info
	Original *Info
	// WorktreeID is the copy's new worktree; empty when the source had none.
	WorktreeID string
	Branch     string
}

// profileMovePlan is what a validated move acts on.
type profileMovePlan struct {
	opts     MoveToProfileOpts
	meta     store.SessionMeta
	checkout *ProfileMoveCheckout
	worktree ProfileMoveWorktrees
}

// MoveToProfile copies a stopped user session into another profile and archives
// the original. The session's profile never changes: the copy is a fork owned by
// the target profile, carrying the conversation as a fork does. A source worktree
// is released and its branch checked out again in a new worktree owned by the
// target profile. Any failure after the archive rolls back in reverse order, so
// the original session and its branch are never lost.
func (m *Manager) MoveToProfile(ctx context.Context, opts MoveToProfileOpts) (MoveToProfileResult, error) {
	if m == nil {
		return MoveToProfileResult{}, errors.New("session: manager is required")
	}
	if ctx == nil {
		return MoveToProfileResult{}, errors.New("session: move to profile context is required")
	}
	opts = normalizeMoveToProfileOpts(opts)
	if opts.SessionID == "" || opts.TargetProfileID == "" {
		return MoveToProfileResult{}, fmt.Errorf("%w: session id and target profile are required", ErrValidation)
	}
	if err := m.checkNewWorkAdmission(ctx); err != nil {
		return MoveToProfileResult{}, err
	}
	unlock := m.lockPromptAdmission(opts.WorkspaceID, "\x00profile-move", opts.SessionID)
	defer unlock()
	plan, err := m.planProfileMove(ctx, opts)
	if err != nil {
		return MoveToProfileResult{}, err
	}
	return m.applyProfileMove(ctx, &plan)
}

func normalizeMoveToProfileOpts(opts MoveToProfileOpts) MoveToProfileOpts {
	opts.WorkspaceID = strings.TrimSpace(opts.WorkspaceID)
	opts.ProfileID = normalizeCreateProfileID(opts.ProfileID)
	opts.SessionID = strings.TrimSpace(opts.SessionID)
	opts.TargetProfileID = strings.TrimSpace(opts.TargetProfileID)
	return opts
}

// planProfileMove applies every refusal before anything changes.
func (m *Manager) planProfileMove(ctx context.Context, opts MoveToProfileOpts) (profileMovePlan, error) {
	meta, err := m.readSessionMetaReadOnly(ctx, opts.SessionID)
	if err != nil {
		return profileMovePlan{}, err
	}
	if strings.TrimSpace(meta.WorkspaceID) != opts.WorkspaceID ||
		normalizeCreateProfileID(meta.ProfileID) != opts.ProfileID {
		return profileMovePlan{}, fmt.Errorf("%w: %s", ErrSessionNotFound, opts.SessionID)
	}
	if err := m.refuseProfileMoveSource(ctx, opts, &meta); err != nil {
		return profileMovePlan{}, err
	}
	plan := profileMovePlan{opts: opts, meta: meta}
	worktreeID := meta.WorktreeIDValue()
	if worktreeID == "" {
		return plan, nil
	}
	port, ok := m.worktreeResolver.(ProfileMoveWorktrees)
	if !ok || port == nil {
		return profileMovePlan{}, errors.New("session: worktree release is unavailable")
	}
	checkout, err := port.InspectProfileMoveWorktree(ctx, meta.WorkspaceID, worktreeID)
	if err != nil {
		return profileMovePlan{}, err
	}
	if err := refuseProfileMoveCheckout(&checkout); err != nil {
		return profileMovePlan{}, err
	}
	if err := m.refuseSharedProfileMoveWorktree(ctx, &meta); err != nil {
		return profileMovePlan{}, err
	}
	plan.checkout, plan.worktree = &checkout, port
	return plan, nil
}

func (m *Manager) refuseProfileMoveSource(ctx context.Context, opts MoveToProfileOpts, meta *store.SessionMeta) error {
	if opts.TargetProfileID == opts.ProfileID {
		return profileMoveRefused(ProfileMoveRefusalSameProfile, "session %s already belongs to this profile", meta.ID)
	}
	if actingSessionID(ctx) == meta.ID {
		return profileMoveRefused(ProfileMoveRefusalOwnSession, "a session cannot move itself to another profile")
	}
	lineage := store.NormalizeSessionLineage(meta.ID, meta.Lineage)
	if normalizeSessionType(Type(meta.SessionType)) != SessionTypeUser ||
		(lineage != nil && (lineage.Kind == store.LineageKindSpawn || lineage.SpawnRole != "")) {
		return profileMoveRefused(
			ProfileMoveRefusalNotMovable, "only user sessions can move; session %s follows its parent", meta.ID,
		)
	}
	if store.HierarchyParentSessionID(lineage) != "" {
		return profileMoveRefused(
			ProfileMoveRefusalNotMovable, "session %s is nested under another session; make it top-level first",
			meta.ID,
		)
	}
	if _, live := m.Get(meta.ID); live || meta.State != string(StateStopped) || m.isPending(meta.ID) ||
		m.sessionStartRun(meta.ID) != nil {
		return profileMoveRefused(ProfileMoveRefusalRunning, "session %s is running; stop it first", meta.ID)
	}
	if err := m.requireSessionUnarchived(ctx, meta.WorkspaceID, meta.ID); err != nil {
		if errors.Is(err, ErrSessionArchived) {
			return profileMoveRefused(ProfileMoveRefusalArchived, "session %s is archived; unarchive it first", meta.ID)
		}
		return err
	}
	tree, err := m.sessionTreeStore()
	if err != nil {
		return err
	}
	children, err := tree.ListSessionDescendants(ctx, meta.WorkspaceID, meta.ID)
	if err != nil {
		return fmt.Errorf("session: list children of %q: %w", meta.ID, err)
	}
	if len(children) > 0 {
		return profileMoveRefused(
			ProfileMoveRefusalHasChildren, "session %s has %d child session(s); move it without children", meta.ID,
			len(children),
		)
	}
	return nil
}

func refuseProfileMoveCheckout(checkout *ProfileMoveCheckout) error {
	switch {
	case !checkout.Ready:
		return profileMoveRefused(ProfileMoveRefusalWorktreeNotReady, "the session's worktree is not ready")
	case checkout.RunNamespace != "":
		return profileMoveRefused(
			ProfileMoveRefusalWorktreeRunOwned, "the session's worktree belongs to a task run and stays with it",
		)
	case checkout.Detached || checkout.Branch == "" || checkout.HeadSHA == "":
		return profileMoveRefused(
			ProfileMoveRefusalWorktreeDetached, "the session's worktree is not on a branch; check out a branch first",
		)
	case checkout.DirtyFiles > 0:
		return profileMoveRefused(
			ProfileMoveRefusalWorktreeDirty,
			"the session's worktree has %d uncommitted file(s) on %s; commit or discard them first",
			checkout.DirtyFiles, checkout.Branch,
		)
	}
	return nil
}

// refuseSharedProfileMoveWorktree keeps a checkout another listed session still uses.
func (m *Manager) refuseSharedProfileMoveWorktree(ctx context.Context, meta *store.SessionMeta) error {
	if m.sessionCatalog == nil {
		return errors.New("session: session catalog is unavailable")
	}
	rows, err := m.sessionCatalog.ListSessions(ctx, store.SessionListQuery{
		ReadScope:   store.ReadScope{AllProfiles: true},
		WorkspaceID: meta.WorkspaceID,
		WorktreeID:  meta.WorktreeIDValue(),
		Archive:     store.SessionArchiveExclude,
	})
	if err != nil {
		return fmt.Errorf("session: list sessions sharing worktree %q: %w", meta.WorktreeIDValue(), err)
	}
	for i := range rows {
		if rows[i].ID != meta.ID {
			return profileMoveRefused(
				ProfileMoveRefusalWorktreeShared,
				"session %s also uses this worktree; archive it or bind it elsewhere first", rows[i].ID,
			)
		}
	}
	return nil
}
