package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
// next occurrence of any source raises Needs you again, and creators are not woken.
func (m *Manager) MarkAttentionSeen(ctx context.Context, sessionID string) (*Info, error) {
	if m == nil || m.attentionStore == nil {
		return nil, ErrAttentionSeenUnavailable
	}
	target := strings.TrimSpace(sessionID)
	before, err := m.Status(ctx, target)
	if err != nil {
		return nil, err
	}
	if ClassForBadge(BadgeForInfo(before)) != AttentionNeedsYou {
		return before, nil
	}
	if blocker := AttentionBlockerForInfo(before); blocker != AttentionBlockerNone {
		return nil, &AttentionRequiresActionError{SessionID: target, Blocker: blocker}
	}
	commit, err := m.attentionStore.DismissSessionAttention(ctx, target, m.now().UTC())
	if err != nil {
		return nil, fmt.Errorf("session: mark %q attention seen: %w", target, err)
	}
	m.applyAttentionProjection(target, commit.After)
	after, err := m.Status(ctx, target)
	if err != nil {
		return nil, err
	}
	if commit.Outcome != "unchanged" {
		m.publishAttentionEdge(ctx, before, after)
		m.sessionLogger(nil).InfoContext(ctx, "session.attention.seen", "session_id", target,
			"from", BadgeForInfo(before), "to", BadgeForInfo(after))
	}
	return after, nil
}
