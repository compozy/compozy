package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/heartbeat"

	"github.com/compozy/agh/internal/store"
)

func (m *Manager) sessionHealthFromInfo(
	info *Info,
	existing heartbeat.SessionHealth,
	now time.Time,
	input sessionHealthInput,
) heartbeat.SessionHealth {
	if now.IsZero() {
		if m != nil && m.now != nil {
			now = m.now()
		} else {
			now = time.Now().UTC()
		}
	}
	now = now.UTC()
	health := heartbeat.SessionHealth{
		LastActivityAt: existing.LastActivityAt,
		LastPresenceAt: existing.LastPresenceAt,
		UpdatedAt:      now,
	}
	if info != nil {
		health.SessionID = strings.TrimSpace(info.ID)
		health.WorkspaceID = strings.TrimSpace(info.WorkspaceID)
		health.AgentName = strings.TrimSpace(info.AgentName)
	}
	if !input.activityAt.IsZero() {
		health.LastActivityAt = input.activityAt.UTC()
	} else if activityAt := infoLastActivityAt(info); !activityAt.IsZero() {
		health.LastActivityAt = activityAt
	}
	if input.touchPresence {
		health.LastPresenceAt = now
	}
	health.ActivePrompt = input.activePrompt
	health.Attachable = input.attachable
	health.LastError = strings.TrimSpace(input.lastError)
	health.State = sessionHealthStateForInfo(info, input)
	health.Health = sessionHealthStatusForInfo(info)
	applySessionHealthEligibility(&health)
	return health.Normalize()
}

func staleRecoveredSessionHealth(row heartbeat.SessionHealth, now time.Time) heartbeat.SessionHealth {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	health := row.Normalize()
	health.State = heartbeat.SessionHealthStateDetached
	health.Health = heartbeat.SessionHealthStale
	health.ActivePrompt = false
	health.Attachable = false
	health.EligibleForWake = false
	health.IneligibilityReason = string(heartbeat.SessionHealthReasonStale)
	health.UpdatedAt = now.UTC()
	return health
}

func sessionHealthStateForInfo(info *Info, input sessionHealthInput) heartbeat.SessionHealthState {
	if info == nil {
		return heartbeat.SessionHealthStateDetached
	}
	switch info.State {
	case StateStopped:
		return heartbeat.SessionHealthStateStopped
	case StateActive:
		if input.activePrompt {
			return heartbeat.SessionHealthStatePrompting
		}
		if !input.attachable {
			return heartbeat.SessionHealthStateDetached
		}
		return heartbeat.SessionHealthStateIdle
	default:
		return heartbeat.SessionHealthStateDetached
	}
}

func sessionHealthStatusForInfo(info *Info) heartbeat.SessionHealthStatus {
	if info == nil {
		return heartbeat.SessionHealthUnknown
	}
	if info.State == StateStopped {
		return heartbeat.SessionHealthDead
	}
	if info.Liveness != nil &&
		strings.TrimSpace(info.Liveness.StallState) == store.SessionStallStateDetected {
		return heartbeat.SessionHealthDegraded
	}
	if info.State != StateActive {
		return heartbeat.SessionHealthUnknown
	}
	return heartbeat.SessionHealthHealthy
}

func applySessionHealthEligibility(health *heartbeat.SessionHealth) {
	if health == nil {
		return
	}
	health.EligibleForWake = false
	health.IneligibilityReason = ""
	switch {
	case health.Health == heartbeat.SessionHealthStale:
		health.IneligibilityReason = string(heartbeat.SessionHealthReasonStale)
	case health.Health == heartbeat.SessionHealthDead:
		health.IneligibilityReason = string(heartbeat.SessionHealthReasonDead)
	case health.Health == heartbeat.SessionHealthUnknown:
		health.IneligibilityReason = string(heartbeat.SessionHealthReasonUnknown)
	case health.Health == heartbeat.SessionHealthDegraded:
		health.IneligibilityReason = string(heartbeat.SessionHealthReasonHung)
	case health.ActivePrompt || health.State == heartbeat.SessionHealthStatePrompting:
		health.IneligibilityReason = string(heartbeat.SessionHealthReasonPromptActive)
	case !health.Attachable || health.State == heartbeat.SessionHealthStateDetached:
		health.IneligibilityReason = string(heartbeat.SessionHealthReasonNotAttachable)
	case health.State != heartbeat.SessionHealthStateIdle:
		health.IneligibilityReason = string(heartbeat.SessionHealthReasonUnhealthy)
	default:
		health.EligibleForWake = true
	}
}

func (m *Manager) storeSessionHealth(
	ctx context.Context,
	health heartbeat.SessionHealth,
) (heartbeat.SessionHealth, error) {
	normalized := health.Normalize()
	if normalized.UpdatedAt.IsZero() {
		normalized.UpdatedAt = m.now()
	}
	if err := normalized.Validate(); err != nil {
		return heartbeat.SessionHealth{}, err
	}
	if m.sessionHealthStore == nil {
		return normalized, nil
	}
	previous, err := m.storedSessionHealth(ctx, normalized.SessionID)
	if err != nil {
		return heartbeat.SessionHealth{}, err
	}
	stored, err := m.sessionHealthStore.UpsertSessionHealth(ctx, normalized)
	if err != nil {
		return heartbeat.SessionHealth{}, fmt.Errorf("session: upsert health for %q: %w", normalized.SessionID, err)
	}
	if err := m.dispatchSessionHealthUpdateAfter(ctx, previous, stored); err != nil && m.logger != nil {
		m.logger.Warn(
			"session: session health hook failed",
			"session_id", stored.SessionID,
			"error", err,
		)
	}
	return stored, nil
}

func (m *Manager) storedSessionHealth(ctx context.Context, sessionID string) (heartbeat.SessionHealth, error) {
	if m == nil || m.sessionHealthStore == nil {
		return heartbeat.SessionHealth{}, nil
	}
	health, err := m.sessionHealthStore.GetSessionHealth(ctx, sessionID)
	if err != nil {
		if errors.Is(err, heartbeat.ErrSessionHealthNotFound) {
			return heartbeat.SessionHealth{}, nil
		}
		return heartbeat.SessionHealth{}, fmt.Errorf("session: get health for %q: %w", sessionID, err)
	}
	return health, nil
}
