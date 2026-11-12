package session

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/redact"
	"github.com/compozy/compozy/internal/store"
)

// SpawnWakeKindEndedEarly identifies a revision-fenced early-stop alarm in the durable queue.
const SpawnWakeKindEndedEarly = "session_ended_early"

const (
	// SpawnWakeReasonStopped wakes the creator when a child stops without failure.
	SpawnWakeReasonStopped SpawnWakeReason = "stopped"
	// SpawnWakeReasonFailed wakes the creator when a child reaches a failed badge.
	SpawnWakeReasonFailed SpawnWakeReason = "failed"
	// SpawnWakeReasonNeedsAttention wakes the creator when a child needs input or approval.
	SpawnWakeReasonNeedsAttention SpawnWakeReason = "needs_attention"
	// SpawnWakeReasonCompleted wakes the creator when a child finishes its turn normally.
	SpawnWakeReasonCompleted SpawnWakeReason = "completed"

	spawnWakeTextMaxRunes = 240
	// spawnWakeAnswerMaxRunes bounds the child's final answer carried by an early-stop wake.
	spawnWakeAnswerMaxRunes = 4000
	spawnWakeCacheMaxItems  = 4096
)

var (
	errSpawnWakeStale  = errors.New("session: stale early-stop wake")
	errSpawnWakeLookup = errors.New("session: early-stop wake lookup can be retried")
	// ErrSpawnWakeValidation reports structurally invalid child wake input.
	ErrSpawnWakeValidation = errors.New("session: spawn wake validation failed")
	// ErrSpawnWakeSessionNotLive reports that the creator cannot accept a wake prompt.
	ErrSpawnWakeSessionNotLive = errors.New("session: spawn wake creator is not live")
	// ErrSpawnWakeRetryable marks a failure before dispatch; retry must still consult durable admission.
	ErrSpawnWakeRetryable = errors.New("session: spawn wake can be retried")
)

// SpawnWakeReason identifies why a child woke its creator.
type SpawnWakeReason string

// SpawnWakeEvent is the bounded child-session wake payload handed to the daemon bridge.
type SpawnWakeEvent struct {
	ChildSessionID string          `json:"child_session_id"`
	ChildAgentName string          `json:"child_agent_name,omitempty"`
	Reason         SpawnWakeReason `json:"reason"`
	Badge          Badge           `json:"badge"`
	Detail         string          `json:"detail,omitempty"`
	// EndedEarly marks the wake for a child turn that stopped before delivering.
	EndedEarly bool `json:"ended_early,omitempty"`
	// FinalAnswer is the child's last answer, carried only when its turn ended early.
	FinalAnswer string `json:"final_answer,omitempty"`
	WakeEventID string `json:"wake_event_id"`
}

// SpawnWakeNotifier delivers a child-session wake to its creator.
type SpawnWakeNotifier interface {
	WakeSpawnCreator(ctx context.Context, creatorSessionID string, event SpawnWakeEvent) error
}

// Normalize returns canonical, redacted, bounded wake data.
func (event SpawnWakeEvent) Normalize() SpawnWakeEvent {
	return SpawnWakeEvent{
		ChildSessionID: strings.TrimSpace(event.ChildSessionID),
		ChildAgentName: strings.TrimSpace(event.ChildAgentName),
		Reason:         SpawnWakeReason(strings.ToLower(strings.TrimSpace(string(event.Reason)))),
		Badge:          Badge(strings.ToLower(strings.TrimSpace(string(event.Badge)))),
		Detail:         SanitizeSpawnWakeText(event.Detail),
		EndedEarly:     event.EndedEarly,
		FinalAnswer:    sanitizeSpawnWakeAnswer(event.FinalAnswer),
		WakeEventID:    strings.TrimSpace(event.WakeEventID),
	}
}

// Validate reports whether a normalized wake event has a complete delivery identity.
func (event SpawnWakeEvent) Validate() error {
	normalized := event.Normalize()
	if normalized.ChildSessionID == "" {
		return fmt.Errorf("%w: child_session_id is required", ErrSpawnWakeValidation)
	}
	if normalized.WakeEventID == "" {
		return fmt.Errorf("%w: wake_event_id is required", ErrSpawnWakeValidation)
	}
	if !normalized.Reason.valid() {
		return fmt.Errorf("%w: invalid reason %q", ErrSpawnWakeValidation, normalized.Reason)
	}
	if expected, ok := spawnWakeReasonForBadge(normalized.Badge); !ok || expected != normalized.Reason {
		return fmt.Errorf(
			"%w: badge %q does not match reason %q",
			ErrSpawnWakeValidation,
			normalized.Badge,
			normalized.Reason,
		)
	}
	return nil
}

// SanitizeSpawnWakeText removes secrets and bounds wake-facing text to 240 runes.
func SanitizeSpawnWakeText(value string) string {
	redacted := strings.TrimSpace(diagnostics.Redact(redact.ClaimTokens(value)))
	if redacted == "" {
		return ""
	}
	runes := []rune(redacted)
	if len(runes) <= spawnWakeTextMaxRunes {
		return redacted
	}
	return string(runes[:spawnWakeTextMaxRunes-3]) + "..."
}

func sanitizeSpawnWakeAnswer(value string) string {
	redacted := strings.TrimSpace(diagnostics.Redact(redact.ClaimTokens(value)))
	runes := []rune(redacted)
	if len(runes) <= spawnWakeAnswerMaxRunes {
		return redacted
	}
	return string(runes[:spawnWakeAnswerMaxRunes-3]) + "..."
}

func (reason SpawnWakeReason) valid() bool {
	switch reason {
	case SpawnWakeReasonStopped, SpawnWakeReasonFailed,
		SpawnWakeReasonNeedsAttention, SpawnWakeReasonCompleted:
		return true
	default:
		return false
	}
}

func spawnWakeReasonForBadge(badge Badge) (SpawnWakeReason, bool) {
	switch badge {
	case BadgeStopped:
		return SpawnWakeReasonStopped, true
	case BadgeFailed:
		return SpawnWakeReasonFailed, true
	case BadgeDone:
		return SpawnWakeReasonCompleted, true
	case BadgeWaitingForAuth, BadgeWaitingForInput, BadgeNeedsAttention:
		return SpawnWakeReasonNeedsAttention, true
	default:
		return "", false
	}
}

func (m *Manager) dispatchSpawnWake(ctx context.Context, info *Info, badge Badge) {
	m.dispatchSpawnWakeForTurn(ctx, info, badge, "")
}

// dispatchSpawnWakeForTurn wakes the creator for one child badge edge. completedTurnID
// names the turn that produced a done edge, so a reply watch on that exact turn can
// carry the answer instead of a duplicate completed wake.
func (m *Manager) dispatchSpawnWakeForTurn(ctx context.Context, info *Info, badge Badge, completedTurnID string) {
	if m == nil || info == nil || m.spawnWakeNotifier == nil {
		return
	}
	// A triage run reports through its check; waking the main session would spend a turn.
	if info.Type == SessionTypeTriage {
		return
	}
	lineage := store.NormalizeSessionLineage(info.ID, info.Lineage)
	creatorSessionID := strings.TrimSpace(lineage.ParentSessionID)
	if creatorSessionID == "" {
		return
	}
	reason, ok := spawnWakeReasonForBadge(badge)
	if !ok {
		return
	}
	event := SpawnWakeEvent{
		ChildSessionID: strings.TrimSpace(info.ID),
		ChildAgentName: strings.TrimSpace(info.AgentName),
		Reason:         reason,
		Badge:          badge,
		Detail:         spawnWakeDetail(info, badge),
		WakeEventID:    spawnWakeEventID(info, reason),
	}.Normalize()
	if err := event.Validate(); err != nil {
		m.logSpawnWakeSuppressed(ctx, creatorSessionID, event, "delivery_failed", err)
		return
	}
	if !m.reserveSpawnWakeEvent(event.WakeEventID) {
		return
	}
	// An early stop is a supervision alarm, not a lifecycle notification: it reaches
	// the parent even when the parent opted out of completion wakes.
	endedEarly := EndedEarlyAttention(info, badge)
	if endedEarly {
		if m.spawnWakeCovered(creatorSessionID, info, badge) {
			m.logSpawnWakeSuppressed(ctx, creatorSessionID, event, "session_monitor_covers", nil)
			return
		}
		event.EndedEarly = true
		event.FinalAnswer = sanitizeSpawnWakeAnswer(m.endedEarlyAnswer(ctx, info))
	}
	if !lineage.NotifyCreator && !endedEarly {
		m.logSpawnWakeSuppressed(ctx, creatorSessionID, event, "wake_creator_disabled", nil)
		return
	}
	if creatorSessionID == event.ChildSessionID {
		m.logSpawnWakeSuppressed(ctx, creatorSessionID, event, "self_wake", nil)
		return
	}
	if event.Reason == SpawnWakeReasonCompleted &&
		m.creatorAwaitsChildReply(ctx, creatorSessionID, event.ChildSessionID, completedTurnID) {
		m.logSpawnWakeSuppressed(ctx, creatorSessionID, event, "reply_watch_covers", nil)
		return
	}
	if err := m.spawnWakeNotifier.WakeSpawnCreator(context.WithoutCancel(ctx), creatorSessionID, event); err != nil {
		if errors.Is(err, ErrSpawnWakeRetryable) {
			m.releaseSpawnWakeEvent(event.WakeEventID)
		}
		if errors.Is(err, ErrSpawnWakeSessionNotLive) {
			m.logSpawnWakeSuppressed(ctx, creatorSessionID, event, "session_not_live", nil)
			return
		}
		m.logSpawnWakeSuppressed(ctx, creatorSessionID, event, "delivery_failed", err)
		return
	}
	m.logger.InfoContext(
		ctx,
		"session.spawn_wake.delivered",
		"creator_session_id", creatorSessionID,
		"child_session_id", event.ChildSessionID,
		"wake_event_id", event.WakeEventID,
		jsonReasonFieldReason, event.Reason,
		"badge", event.Badge,
	)
}

// ConfigureSpawnWakeCoverage installs the daemon's session-monitor coverage reader.
func (m *Manager) ConfigureSpawnWakeCoverage(covers func(string, *Info, Badge) bool) {
	m.spawnWakeMu.Lock()
	defer m.spawnWakeMu.Unlock()
	m.spawnWakeCoverage = covers
}

func (m *Manager) spawnWakeCovered(creatorID string, child *Info, badge Badge) bool {
	m.spawnWakeMu.Lock()
	covers := m.spawnWakeCoverage
	m.spawnWakeMu.Unlock()
	return covers != nil && covers(creatorID, child, badge)
}

func spawnWakeEventID(info *Info, reason SpawnWakeReason) string {
	if reason == SpawnWakeReasonNeedsAttention && strings.TrimSpace(info.EndedEarlyTurnID) != "" {
		return fmt.Sprintf("session-wake:%s:%d:ended-early:%s", strings.TrimSpace(info.ID),
			info.TranscriptEpoch, strings.TrimSpace(info.EndedEarlyTurnID))
	}
	return fmt.Sprintf("session-wake:%s:%d:%d:%s", strings.TrimSpace(info.ID),
		info.TranscriptEpoch, info.AttentionRevision, reason)
}

// validateSpawnWake drops an alarm whose child has advanced while the creator was busy.
func (m *Manager) validateSpawnWake(ctx context.Context, creatorID string, meta *acp.PromptSyntheticMeta) error {
	if meta == nil || meta.Kind != SpawnWakeKindEndedEarly {
		return nil
	}
	var info *Info
	if child, ok := m.Get(meta.ChildSessionID); ok {
		child.mu.RLock()
		info = child.infoLocked()
		currentTurnID := child.currentTurnID
		child.mu.RUnlock()
		if currentTurnID != "" && currentTurnID != info.EndedEarlyTurnID {
			return fmt.Errorf("%w: child started a newer turn", errSpawnWakeStale)
		}
	} else {
		if m.sessionCatalog == nil {
			return fmt.Errorf("%w: session catalog unavailable", errSpawnWakeLookup)
		}
		var err error
		info, err = m.durableSessionInfoForLifecycleTransition(ctx, meta.ChildSessionID)
		if errors.Is(err, ErrSessionNotFound) {
			return fmt.Errorf("%w: child no longer exists", errSpawnWakeStale)
		}
		if err != nil {
			return fmt.Errorf("%w: %w", errSpawnWakeLookup, err)
		}
	}
	if !EndedEarlyAttention(info, BadgeForInfo(info)) ||
		store.NormalizeSessionLineage(info.ID, info.Lineage).ParentSessionID != creatorID ||
		spawnWakeEventID(info, SpawnWakeReasonNeedsAttention) != meta.WakeEventID ||
		m.spawnWakeCovered(creatorID, info, BadgeNeedsAttention) {
		return fmt.Errorf("%w: child attention changed or a session monitor covers it", errSpawnWakeStale)
	}
	return nil
}

func (m *Manager) reserveSpawnWakeEvent(wakeEventID string) bool {
	key := strings.TrimSpace(wakeEventID)
	if key == "" {
		return false
	}
	m.spawnWakeMu.Lock()
	defer m.spawnWakeMu.Unlock()
	if _, exists := m.spawnWakeEventIDs[key]; exists {
		return false
	}
	for len(m.spawnWakeEventIDs) >= spawnWakeCacheMaxItems && len(m.spawnWakeEventOrder) > 0 {
		evicted := m.spawnWakeEventOrder[0]
		delete(m.spawnWakeEventIDs, evicted)
		m.spawnWakeEventOrder = m.spawnWakeEventOrder[1:]
	}
	m.spawnWakeEventIDs[key] = struct{}{}
	m.spawnWakeEventOrder = append(m.spawnWakeEventOrder, key)
	return true
}

func (m *Manager) forgetSpawnWakeEvents(sessionID string) {
	target := strings.TrimSpace(sessionID)
	if m == nil || target == "" {
		return
	}
	prefix := "session-wake:" + target + ":"
	m.spawnWakeMu.Lock()
	defer m.spawnWakeMu.Unlock()
	retained := m.spawnWakeEventOrder[:0]
	for _, key := range m.spawnWakeEventOrder {
		if strings.HasPrefix(key, prefix) {
			delete(m.spawnWakeEventIDs, key)
			continue
		}
		retained = append(retained, key)
	}
	m.spawnWakeEventOrder = retained
}

// releaseSpawnWakeEvent removes only the failed attempt's reservation, including
// its eviction entry so a later retry cannot be evicted by the stale reservation.
func (m *Manager) releaseSpawnWakeEvent(wakeEventID string) {
	key := strings.TrimSpace(wakeEventID)
	m.spawnWakeMu.Lock()
	defer m.spawnWakeMu.Unlock()
	delete(m.spawnWakeEventIDs, key)
	m.spawnWakeEventOrder = slices.DeleteFunc(m.spawnWakeEventOrder, func(value string) bool { return value == key })
}

func (m *Manager) logSpawnWakeSuppressed(
	ctx context.Context,
	creatorSessionID string,
	event SpawnWakeEvent,
	reason string,
	deliveryErr error,
) {
	attrs := []any{
		"suppression_reason", strings.TrimSpace(reason),
		"creator_session_id", strings.TrimSpace(creatorSessionID),
		"child_session_id", event.ChildSessionID,
		"wake_event_id", event.WakeEventID,
		jsonReasonFieldReason, event.Reason,
		"badge", event.Badge,
		"detail", event.Detail,
	}
	if deliveryErr != nil {
		attrs = append(attrs, "error", SanitizeSpawnWakeText(deliveryErr.Error()))
	}
	m.logger.WarnContext(ctx, "session.spawn_wake.suppressed", attrs...)
}

func spawnWakeDetail(info *Info, badge Badge) string {
	if info == nil {
		return ""
	}
	switch badge {
	case BadgeNeedsAttention:
		if EndedEarlyAttention(info, badge) {
			return endedEarlyWakeDetail
		}
		return "Process exit could not be verified; retry stop."
	case BadgeWaitingForInput:
		for _, interaction := range info.PendingInteractions {
			if title := strings.TrimSpace(interaction.Title); title != "" {
				return title
			}
		}
		return "Waiting for input."
	case BadgeWaitingForAuth:
		return "Waiting for permission."
	case BadgeFailed:
		if info.Failure != nil {
			if summary := strings.TrimSpace(info.Failure.Summary); summary != "" {
				return summary
			}
		}
		return info.StopDetail
	case BadgeStopped:
		return info.StopDetail
	default:
		return ""
	}
}
