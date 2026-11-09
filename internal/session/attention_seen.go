package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

// AttentionBlocker names a Needs you source the operator acts on instead of
// marking it as seen.
type AttentionBlocker string

const (
	AttentionBlockerNone              AttentionBlocker = ""
	AttentionBlockerPendingPermission AttentionBlocker = "pending_permission"
	AttentionBlockerPendingClarify    AttentionBlocker = "pending_clarify"
	AttentionBlockerProviderAuth      AttentionBlocker = "provider_auth"
	AttentionBlockerStopUnverified    AttentionBlocker = "stop_unverified"
	AttentionBlockerSupervision       AttentionBlocker = "supervision"
)

var (
	// ErrAttentionRequiresAction rejects marking as seen a session that still waits on the operator.
	ErrAttentionRequiresAction = errors.New("session: attention requires action")
	// ErrAttentionChanged rejects marking as seen after a newer occurrence replaced the one checked.
	ErrAttentionChanged = errors.New("session: attention changed before it was marked as seen")
	// ErrAttentionSeenUnavailable reports a manager without the canonical attention store.
	ErrAttentionSeenUnavailable = errors.New("session: attention store is unavailable")
)

// AttentionRequiresActionError names the source that keeps a session in Needs you.
type AttentionRequiresActionError struct {
	SessionID string
	Blocker   AttentionBlocker
}

func (e *AttentionRequiresActionError) Error() string {
	return fmt.Sprintf("session %s needs you to %s before it can be marked as seen", e.SessionID, e.Blocker.action())
}

func (e *AttentionRequiresActionError) Unwrap() error { return ErrAttentionRequiresAction }

func (blocker AttentionBlocker) action() string {
	switch blocker {
	case AttentionBlockerPendingPermission:
		return "answer its permission request"
	case AttentionBlockerPendingClarify:
		return "answer its question"
	case AttentionBlockerProviderAuth:
		return "sign in to its provider"
	case AttentionBlockerStopUnverified:
		return "stop its agent process"
	case AttentionBlockerSupervision:
		return "resolve the work signal it cannot check"
	default:
		return "act on it"
	}
}

// AttentionBlockerForInfo returns the source that keeps a Needs you session from
// being marked as seen, following CanonicalBadge precedence. A turn that ended
// early, a return from Later, and a stopped session's failure are dismissible.
func AttentionBlockerForInfo(info *Info) AttentionBlocker {
	if info == nil {
		return AttentionBlockerNone
	}
	if info.State != StateStopped {
		if info.StopVerificationFailed {
			return AttentionBlockerStopUnverified
		}
		if supervisionNeedsAttention(info.Supervision) {
			return AttentionBlockerSupervision
		}
	}
	if info.State == StateStopped && terminalFailureKind(info.Failure) {
		if info.PendingPermission || info.PendingPermissionCount > 0 {
			return AttentionBlockerPendingPermission
		}
		if info.PendingClarifyCount > 0 {
			return AttentionBlockerPendingClarify
		}
		return AttentionBlockerNone
	}

	switch {
	case info.PendingPermission || info.PendingPermissionCount > 0:
		return AttentionBlockerPendingPermission
	case infoFailureNeedsAuth(info.Failure):
		return AttentionBlockerProviderAuth
	case info.PendingClarifyCount > 0:
		return AttentionBlockerPendingClarify
	default:
		return AttentionBlockerNone
	}
}

// AttentionDismissible reports whether the session shows Needs you and the
// operator can mark it as seen.
func AttentionDismissible(info *Info) bool {
	return info != nil && ClassForBadge(BadgeForInfo(info)) == AttentionNeedsYou &&
		AttentionBlockerForInfo(info) == AttentionBlockerNone
}

// MarkAttentionSeen clears the current Needs you occurrence of one session for the
// operator. A session that does not need the operator is returned unchanged; one
// that waits on an answer, a sign-in, or a stop is refused with its blocker. The
// write is fenced on the checked occurrence, so a newer one is refused with
// ErrAttentionChanged instead of being cleared unseen. The next occurrence of any
// source raises Needs you again, and creators are not woken.
func (m *Manager) MarkAttentionSeen(ctx context.Context, sessionID string) (*Info, error) {
	if m == nil || m.attentionStore == nil {
		return nil, ErrAttentionSeenUnavailable
	}
	target := strings.TrimSpace(sessionID)
	before, err := m.Status(ctx, target)
	if err != nil {
		return nil, err
	}
	// The canonical projection is the occurrence the decision and the write both fence on.
	observed, err := m.attentionStore.GetSessionAttention(ctx, target)
	if err != nil {
		return nil, fmt.Errorf("session: read %q attention: %w", target, err)
	}
	applyAttentionToInfo(before, observed)
	if ClassForBadge(BadgeForInfo(before)) != AttentionNeedsYou {
		return before, nil
	}
	if blocker := AttentionBlockerForInfo(before); blocker != AttentionBlockerNone {
		return nil, &AttentionRequiresActionError{SessionID: target, Blocker: blocker}
	}
	unlock, err := m.lockAttentionCommit(ctx, target)
	if err != nil {
		return nil, err
	}
	defer unlock()
	commit, err := m.commitAttentionLocked(target, func() (store.SessionAttentionCommit, error) {
		return m.attentionStore.DismissSessionAttention(ctx, target, observed.Fence(), m.now().UTC())
	})
	if errors.Is(err, store.ErrSessionAttentionChanged) {
		return nil, fmt.Errorf("%w: %s", ErrAttentionChanged, target)
	}
	if err != nil {
		return nil, fmt.Errorf("session: mark %q attention seen: %w", target, err)
	}
	after, err := m.Status(ctx, target)
	if err != nil {
		return nil, err
	}
	if commit.Outcome != "unchanged" {
		// Dismissal retains the observer baseline but must never wake the creator.
		if _, err := m.emitAttentionCommit(ctx, target, commit, false); err != nil {
			return nil, err
		}
		m.sessionLogger(nil).InfoContext(ctx, "session.attention.seen", "session_id", target,
			"from", BadgeForInfo(before), "to", BadgeForInfo(after))
	}
	return after, nil
}
