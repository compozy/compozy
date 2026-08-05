package session

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/transcript"
)

const transcriptMarkerEvidenceSourceKey = "source"

func (m *Manager) emitTranscriptMarker(
	ctx context.Context,
	session *Session,
	turnID string,
	kind string,
	summary string,
	evidence map[string]any,
) {
	if err := m.recordTranscriptMarker(ctx, session, turnID, kind, summary, evidence); err != nil {
		m.sessionLogger(session).Warn("session: emit transcript marker failed", "kind", kind, "error", err)
	}
}

func (m *Manager) recordTranscriptMarker(
	ctx context.Context,
	session *Session,
	turnID string,
	kind string,
	summary string,
	evidence map[string]any,
) error {
	marker, err := transcript.NewMarker(kind, summary, m.now(), evidence)
	if err != nil {
		return fmt.Errorf("session: build transcript marker %q: %w", kind, err)
	}
	event, err := marker.AgentEvent("", turnID)
	if err != nil {
		return fmt.Errorf("session: convert transcript marker %q: %w", kind, err)
	}
	normalized := transcript.RedactAgentEvent(m.normalizeEvent(session, turnID, event))
	if err := m.recordEvent(ctx, session, normalized); err != nil {
		return fmt.Errorf("session: record transcript marker %q: %w", kind, err)
	}
	m.notifyAgentEvent(ctx, session, normalized)
	return nil
}

func (m *Manager) persistResumeReplayMarker(
	ctx context.Context,
	spec *sessionStartSpec,
	session *Session,
) error {
	if !spec.resumeReplay {
		return nil
	}
	fallbackReason := strings.TrimSpace(spec.resumeReplayReason)
	if fallbackReason == "" {
		fallbackReason = "session_load_unavailable"
	}
	turnID, err := m.newPromptTurnID()
	if err != nil {
		return startupFailure(
			"session replay marker turn id allocation failed",
			fmt.Errorf("session: generate context rebuilt marker turn id for %q: %w", spec.sessionID, err),
		)
	}
	if err := m.recordTranscriptMarker(
		ctx,
		session,
		turnID,
		transcript.MarkerSessionRecovered,
		contextRebuiltMarkerSummary,
		map[string]any{
			transcriptMarkerEvidenceSourceKey: "events.db",
			"message_count":                   spec.resumeReplayMessageCount,
			"fallback_reason":                 fallbackReason,
		},
	); err != nil {
		return startupFailure(
			"session replay marker persistence failed",
			fmt.Errorf("session: persist context rebuilt marker for %q: %w", spec.sessionID, err),
		)
	}
	return nil
}
