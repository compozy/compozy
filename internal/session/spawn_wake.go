package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/redact"
	"github.com/compozy/compozy/internal/store"
)

const (
	// SpawnWakeReasonStopped wakes the creator when a child stops without failure.
	SpawnWakeReasonStopped SpawnWakeReason = "stopped"
	// SpawnWakeReasonFailed wakes the creator when a child reaches a failed badge.
	SpawnWakeReasonFailed SpawnWakeReason = "failed"
	// SpawnWakeReasonNeedsAttention wakes the creator when a child needs input or approval.
	SpawnWakeReasonNeedsAttention SpawnWakeReason = "needs_attention"

	spawnWakeTextMaxRunes = 240
)

var (
	// ErrSpawnWakeValidation reports structurally invalid child wake input.
	ErrSpawnWakeValidation = errors.New("session: spawn wake validation failed")
	// ErrSpawnWakeSessionNotLive reports that the creator cannot accept a wake prompt.
	ErrSpawnWakeSessionNotLive = errors.New("session: spawn wake creator is not live")
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
	WakeEventID    string          `json:"wake_event_id"`
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

func (reason SpawnWakeReason) valid() bool {
	switch reason {
	case SpawnWakeReasonStopped, SpawnWakeReasonFailed, SpawnWakeReasonNeedsAttention:
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
	case BadgeWaitingForAuth, BadgeWaitingForInput:
		return SpawnWakeReasonNeedsAttention, true
	default:
		return "", false
	}
}

func (m *Manager) dispatchSpawnWake(ctx context.Context, info *Info, badge Badge) {
	if m == nil || info == nil || m.spawnWakeNotifier == nil {
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
		WakeEventID: fmt.Sprintf(
			"session-wake:%s:%d:%d:%s",
			strings.TrimSpace(info.ID),
			info.TranscriptEpoch,
			info.AttentionRevision,
			reason,
		),
	}.Normalize()
	if err := event.Validate(); err != nil {
		m.logSpawnWakeSuppressed(ctx, creatorSessionID, event, "delivery_failed", err)
		return
	}
	if !lineage.NotifyCreator {
		m.logSpawnWakeSuppressed(ctx, creatorSessionID, event, "wake_creator_disabled", nil)
		return
	}
	if creatorSessionID == event.ChildSessionID {
		m.logSpawnWakeSuppressed(ctx, creatorSessionID, event, "self_wake", nil)
		return
	}
	if err := m.spawnWakeNotifier.WakeSpawnCreator(context.WithoutCancel(ctx), creatorSessionID, event); err != nil {
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
