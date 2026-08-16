package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	WaitTimeoutDefault = 5 * time.Minute
	WaitTimeoutMax     = 30 * time.Minute
	WaitResumeGrace    = 10 * time.Second
	WaitEdgeBufferSize = 64
	WaitMaxPerSession  = 32
)

var (
	ErrWaitValidation   = errors.New("session: invalid wait request")
	ErrWaitStateInvalid = errors.New("session: invalid wait state")
	ErrWaitLimit        = errors.New("session: concurrent wait limit reached")
	ErrWaitExpired      = errors.New("session: wait registration expired")
	ErrWaitInUse        = errors.New("session: wait registration is already attached")
)

// WaitResult is the terminal result of one bounded badge wait.
type WaitResult string

const (
	WaitResultStateReached WaitResult = "state-reached"
	WaitResultTimeout      WaitResult = "timeout"
	WaitResultSessionGone  WaitResult = "session-gone"
	WaitResultCanceled     WaitResult = "canceled"
	WaitResultOverflow     WaitResult = "overflow"
)

// WaitRequest describes one always-bounded badge wait or resume attempt.
type WaitRequest struct {
	SessionID string
	Until     []Badge
	Timeout   time.Duration
	Epoch     int64
	ResumeID  string
}

// WaitOutcome reports the observed edge and its catalog-owned fence.
type WaitOutcome struct {
	Outcome  WaitResult
	State    Badge
	Waited   time.Duration
	Revision int64
	ResumeID string
}

var waitBadgeVocabulary = []Badge{
	BadgeWaitingForInput,
	BadgeWaitingForAuth,
	BadgeIdle,
	BadgeDone,
	BadgeRunning,
	BadgeStopped,
	BadgeFailed,
	BadgeHung,
	BadgeUnhealthy,
}

var defaultWaitBadges = []Badge{
	BadgeWaitingForInput,
	BadgeWaitingForAuth,
	BadgeIdle,
	BadgeStopped,
	BadgeFailed,
}

// WaitForBadge blocks until a matching transition, bounded timeout, cancellation,
// overflow, or target identity loss. Registration precedes the initial snapshot.
func (m *Manager) WaitForBadge(ctx context.Context, req WaitRequest) (outcome WaitOutcome, retErr error) {
	if m == nil {
		return WaitOutcome{}, errors.New("session: manager is required")
	}
	startedAt := m.now().UTC()
	normalized, until, err := normalizeWaitRequest(ctx, req)
	if err != nil {
		return WaitOutcome{}, err
	}
	defer func() {
		if outcome.Outcome != "" {
			m.logWaitCompleted(ctx, normalized, outcome)
		}
	}()
	if m.waitRegistry == nil {
		return WaitOutcome{}, errors.New("session: wait registry is unavailable")
	}

	registration, err := m.attachWaitRegistration(normalized, until)
	if err != nil {
		return WaitOutcome{}, err
	}
	if normalized.ResumeID == "" {
		if outcome, ready, snapshotErr := m.initializeWaitRegistration(ctx, registration, normalized); ready {
			outcome.Waited = m.now().UTC().Sub(startedAt)
			return outcome, snapshotErr
		}
	}

	timer := m.newWaitTimer(normalized.Timeout)
	defer timer.Stop()
	for {
		if outcome, ready := m.waitRegistry.next(registration); ready {
			outcome.Waited = m.now().UTC().Sub(startedAt)
			return outcome, nil
		}
		select {
		case <-ctx.Done():
			outcome := m.waitRegistry.cancel(registration)
			outcome.Waited = m.now().UTC().Sub(startedAt)
			return outcome, nil
		case <-timer.Channel():
			outcome := m.waitRegistry.detach(registration)
			outcome.Waited = normalized.Timeout
			return outcome, nil
		case <-registration.signal:
		}
	}
}

func (m *Manager) logWaitCompleted(ctx context.Context, req WaitRequest, outcome WaitOutcome) {
	if m == nil || m.logger == nil {
		return
	}
	logCtx := ctx
	if logCtx == nil {
		logCtx = context.Background()
	}
	predicate := make([]string, 0, len(req.Until))
	for _, badge := range req.Until {
		predicate = append(predicate, string(badge))
	}
	m.logger.InfoContext(
		logCtx,
		"session.wait.completed",
		"session_id", req.SessionID,
		"outcome", outcome.Outcome,
		"duration_ms", outcome.Waited.Milliseconds(),
		"predicate", predicate,
		"state", outcome.State,
		"revision", outcome.Revision,
		"resumable", outcome.ResumeID != "",
	)
}

func (m *Manager) attachWaitRegistration(
	req WaitRequest,
	until map[Badge]struct{},
) (*sessionWaitRegistration, error) {
	if req.ResumeID != "" {
		return m.waitRegistry.resume(req, until)
	}
	return m.waitRegistry.register(req.SessionID, req.Epoch, until)
}

func (m *Manager) initializeWaitRegistration(
	ctx context.Context,
	registration *sessionWaitRegistration,
	req WaitRequest,
) (WaitOutcome, bool, error) {
	info, err := m.Status(ctx, req.SessionID)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return m.waitRegistry.complete(registration, waitEdge{gone: true}), true, nil
		}
		m.waitRegistry.remove(registration)
		return WaitOutcome{}, true, err
	}
	if info == nil {
		m.waitRegistry.remove(registration)
		return WaitOutcome{}, true, errors.New("session: wait snapshot is nil")
	}
	pinnedEpoch := req.Epoch
	if pinnedEpoch == 0 {
		pinnedEpoch = info.TranscriptEpoch
	}
	if pinnedEpoch != info.TranscriptEpoch {
		return m.waitRegistry.complete(registration, waitEdge{
			gone: true, epoch: info.TranscriptEpoch, revision: info.AttentionRevision,
		}), true, nil
	}
	m.waitRegistry.pin(registration, pinnedEpoch, info.AttentionRevision)
	if outcome, ready := m.waitRegistry.next(registration); ready {
		return outcome, true, nil
	}
	badge := BadgeForInfo(info)
	if waitBadgeMatches(registration.until, badge) {
		return m.waitRegistry.complete(registration, waitEdge{
			badge: badge, epoch: pinnedEpoch, revision: info.AttentionRevision,
		}), true, nil
	}
	return WaitOutcome{}, false, nil
}

func normalizeWaitRequest(
	ctx context.Context,
	req WaitRequest,
) (WaitRequest, map[Badge]struct{}, error) {
	if ctx == nil {
		return WaitRequest{}, nil, fmt.Errorf("%w: context is required", ErrWaitValidation)
	}
	normalized := req
	normalized.SessionID = strings.TrimSpace(normalized.SessionID)
	normalized.ResumeID = strings.TrimSpace(normalized.ResumeID)
	if normalized.SessionID == "" {
		return WaitRequest{}, nil, fmt.Errorf("%w: session_id is required", ErrWaitValidation)
	}
	if normalized.Timeout <= 0 || normalized.Timeout > WaitTimeoutMax {
		return WaitRequest{}, nil, fmt.Errorf(
			"%w: timeout must be greater than zero and at most %s",
			ErrWaitValidation,
			WaitTimeoutMax,
		)
	}
	if normalized.Epoch < 0 {
		return WaitRequest{}, nil, fmt.Errorf("%w: epoch must not be negative", ErrWaitValidation)
	}
	badges, err := NormalizeWaitBadges(normalized.Until)
	if err != nil {
		return WaitRequest{}, nil, err
	}
	until := make(map[Badge]struct{}, len(badges))
	for _, badge := range badges {
		until[badge] = struct{}{}
	}
	normalized.Until = badges
	return normalized, until, nil
}

// NormalizeWaitBadges validates, deduplicates, and applies the settled-set
// default used by every wait surface.
func NormalizeWaitBadges(input []Badge) ([]Badge, error) {
	badges := input
	if len(badges) == 0 {
		badges = defaultWaitBadges
	}
	normalized := make([]Badge, 0, len(badges))
	seen := make(map[Badge]struct{}, len(badges))
	for _, raw := range badges {
		badge := Badge(strings.ToLower(strings.TrimSpace(string(raw))))
		if !validWaitBadge(badge) {
			return nil, fmt.Errorf(
				"%w: %w: unknown session state %q (valid: %s)",
				ErrWaitValidation,
				ErrWaitStateInvalid,
				raw,
				waitBadgeVocabularyText(),
			)
		}
		if _, exists := seen[badge]; exists {
			continue
		}
		seen[badge] = struct{}{}
		normalized = append(normalized, badge)
	}
	return normalized, nil
}

func validWaitBadge(candidate Badge) bool {
	for _, badge := range waitBadgeVocabulary {
		if candidate == badge {
			return true
		}
	}
	return false
}

func waitBadgeVocabularyText() string {
	values := make([]string, 0, len(waitBadgeVocabulary))
	for _, badge := range waitBadgeVocabulary {
		values = append(values, string(badge))
	}
	return strings.Join(values, ", ")
}

func waitBadgeMatches(until map[Badge]struct{}, badge Badge) bool {
	if _, ok := until[badge]; ok {
		return true
	}
	_, waitsForIdle := until[BadgeIdle]
	return badge == BadgeDone && waitsForIdle
}

func (m *Manager) publishWaitBadgeEdge(info *Info, badge Badge) {
	if m == nil || m.waitRegistry == nil || info == nil {
		return
	}
	m.waitRegistry.publish(strings.TrimSpace(info.ID), waitEdge{
		badge: badge, epoch: info.TranscriptEpoch, revision: info.AttentionRevision,
	})
}

func (m *Manager) publishWaitSessionGone(info *Info) {
	if m == nil || m.waitRegistry == nil || info == nil {
		return
	}
	m.waitRegistry.publish(strings.TrimSpace(info.ID), waitEdge{
		gone: true, epoch: info.TranscriptEpoch, revision: info.AttentionRevision,
	})
}
