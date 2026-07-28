package heartbeat

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"time"
)

func (s *ManagedWakeService) currentTime() time.Time {
	if s == nil || s.now == nil {
		return time.Now().UTC()
	}
	now := s.now()
	if now.IsZero() {
		return time.Now().UTC()
	}
	return now.UTC()
}

func normalizeWakeRequest(req WakeRequest) (WakeRequest, error) {
	normalized := WakeRequest{
		WorkspaceID:          strings.TrimSpace(req.WorkspaceID),
		AgentName:            strings.TrimSpace(req.AgentName),
		SessionID:            strings.TrimSpace(req.SessionID),
		Source:               WakeSource(strings.TrimSpace(string(req.Source))),
		DryRun:               req.DryRun,
		SyntheticCorrelation: req.SyntheticCorrelation.Normalize(),
	}
	switch {
	case normalized.WorkspaceID == "":
		return WakeRequest{}, errors.New("heartbeat: wake workspace id is required")
	case normalized.AgentName == "":
		return WakeRequest{}, errors.New("heartbeat: wake agent name is required")
	case normalized.SessionID == "":
		return WakeRequest{}, errors.New("heartbeat: wake session id is required")
	case !ValidWakeSource(normalized.Source):
		return WakeRequest{}, fmt.Errorf("heartbeat: invalid wake source %q", normalized.Source)
	default:
		return normalized, nil
	}
}

func cooldownDecisionForSource(source WakeSource) (WakeResult, WakeReason) {
	switch source {
	case WakeSourceManual:
		return WakeResultRateLimited, WakeReasonCooldownActive
	default:
		return WakeResultCoalesced, WakeReasonCoalesced
	}
}

func ineligibleWakeReason(req WakeRequest, health SessionHealth) WakeReason {
	normalized := health.Normalize()
	if normalized.WorkspaceID != req.WorkspaceID || normalized.AgentName != req.AgentName {
		return WakeReasonHeartbeatNoEligible
	}
	if normalized.EligibleForWake {
		return ""
	}
	switch SessionHealthIneligibilityReason(normalized.IneligibilityReason) {
	case SessionHealthReasonPromptActive:
		return WakeReasonSessionPromptActive
	case SessionHealthReasonNotAttachable:
		return WakeReasonSessionNotAttachable
	case SessionHealthReasonStale,
		SessionHealthReasonHung,
		SessionHealthReasonDead,
		SessionHealthReasonUnknown,
		SessionHealthReasonUnhealthy:
		return WakeReasonSessionUnhealthy
	default:
		return WakeReasonHeartbeatNoEligible
	}
}

func heartbeatWakePrompt(envelope *SnapshotEnvelope) string {
	if envelope == nil {
		return "Agent Heartbeat requested a reorientation for this existing session."
	}
	summary := strings.TrimSpace(envelope.Prompt.Summary)
	if summary == "" {
		summary = strings.TrimSpace(envelope.Summary)
	}
	guidance := strings.TrimSpace(envelope.Prompt.GuidanceMarkdown)
	if guidance == "" {
		guidance = strings.TrimSpace(envelope.GuidanceMarkdown)
	}

	var builder strings.Builder
	builder.WriteString("Agent Heartbeat requested a reorientation for this existing session.")
	if summary != "" {
		builder.WriteString("\n\nPolicy summary:\n")
		builder.WriteString(summary)
	}
	if guidance != "" {
		builder.WriteString("\n\nPolicy guidance:\n")
		builder.WriteString(guidance)
	}
	builder.WriteString(
		"\n\nThis prompt does not assign ownership or include task-run credentials. " +
			"Inspect /agent/context and current session state before acting. " +
			"If task work is needed, inspect current task state through the task APIs first. " +
			"If there is nothing useful to do, record that no action is needed and remain idle.",
	)
	return builder.String()
}

func defaultWakeID(prefix string) string {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		now := time.Now().UTC().UnixNano()
		if strings.TrimSpace(prefix) == "" {
			return fmt.Sprintf("%d", now)
		}
		return fmt.Sprintf("%s-%d", strings.TrimSpace(prefix), now)
	}
	if strings.TrimSpace(prefix) == "" {
		return hex.EncodeToString(random[:])
	}
	return fmt.Sprintf("%s-%s", strings.TrimSpace(prefix), hex.EncodeToString(random[:]))
}
