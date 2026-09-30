package session

import (
	"context"
	"fmt"
	"maps"
	"strings"

	"github.com/compozy/compozy/internal/transcript"
)

const (
	transcriptMarkerEvidenceSourceKey    = "source"
	transcriptMarkerEvidenceEventTypeKey = "event_type"
)

func (m *Manager) emitTranscriptMarker(
	ctx context.Context,
	session *Session,
	turnID string,
	kind string,
	summary string,
	evidence map[string]any,
) {
	in := transcriptMarkerInput{Kind: kind, Summary: summary, Evidence: evidence}
	if err := m.recordTranscriptMarker(ctx, session, turnID, in); err != nil {
		m.sessionLogger(session).Warn("session: emit transcript marker failed", "kind", kind, "error", err)
	}
}

// transcriptMarkerInput makes attribution explicit: when Route is set, the marker's
// provider_command_fingerprint is the attempted route's fingerprint; otherwise the
// session's current routing snapshot (today's behavior).
type transcriptMarkerInput struct {
	Kind     string
	Summary  string
	Evidence map[string]any
	Route    *FallbackRoute
	Command  string // effective command of the attempt when Route has none
}

// markerCommandFingerprint attributes a provider failure to the attempted route when
// one is named, else to the session's bound route.
func (in transcriptMarkerInput) markerCommandFingerprint(session *Session) string {
	if in.Route != nil {
		if command := strings.TrimSpace(in.Route.Command); command != "" {
			return providerCommandFingerprint(command)
		}
		return providerCommandFingerprint(in.Command)
	}
	if session == nil {
		return ""
	}
	return providerCommandFingerprint(session.providerRoutingSnapshot().Command)
}

// recordTranscriptMarker correlates provider failures without persisting raw command credentials.
func (m *Manager) recordTranscriptMarker(
	ctx context.Context,
	session *Session,
	turnID string,
	in transcriptMarkerInput,
) error {
	kind, summary, evidence := in.Kind, in.Summary, in.Evidence
	if kind == transcript.MarkerProviderFailure && (session != nil || in.Route != nil) {
		evidence = maps.Clone(evidence)
		if evidence == nil {
			evidence = make(map[string]any)
		}
		evidence["provider_command_fingerprint"] = in.markerCommandFingerprint(session)
	}
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
	if err := m.recordTranscriptMarker(ctx, session, turnID, transcriptMarkerInput{
		Kind:    transcript.MarkerSessionRecovered,
		Summary: contextRebuiltMarkerSummary,
		Evidence: map[string]any{
			transcriptMarkerEvidenceSourceKey: "events.db",
			"message_count":                   spec.resumeReplayMessageCount,
			"fallback_reason":                 fallbackReason,
		},
	}); err != nil {
		return startupFailure(
			"session replay marker persistence failed",
			fmt.Errorf("session: persist context rebuilt marker for %q: %w", spec.sessionID, err),
		)
	}
	return nil
}
