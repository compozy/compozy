package session

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/acp"
	eventspkg "github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

const sessionFallbackEventWriteTimeout = 10 * time.Second

// sessionFallbackEventPayload is the session.fallback.used ledger content. It carries
// the route's command fingerprint, never the command.
type sessionFallbackEventPayload struct {
	Agent                      string `json:"agent"`
	Phase                      string `json:"phase"` // "bind" | "create"
	Attempt                    int    `json:"attempt"`
	Provider                   string `json:"provider"`
	Model                      string `json:"model"`
	ProviderCommandFingerprint string `json:"provider_command_fingerprint,omitempty"`
}

func newSessionFallbackEventPayload(
	seq fallbackSequence,
	attempt int,
	route FallbackRoute,
) sessionFallbackEventPayload {
	return sessionFallbackEventPayload{
		Agent:                      strings.TrimSpace(seq.agent),
		Phase:                      seq.phase,
		Attempt:                    attempt,
		Provider:                   strings.TrimSpace(route.Provider),
		Model:                      strings.TrimSpace(route.Model),
		ProviderCommandFingerprint: providerCommandFingerprint(seq.commandFor(route)),
	}
}

// recordSessionFallbackEvent commits session.fallback.used to the daemon ledger before
// the attempt starts; a failure aborts the sequence.
func (m *Manager) recordSessionFallbackEvent(
	ctx context.Context,
	seq fallbackSequence,
	attempt int,
	route FallbackRoute,
) error {
	if seq.ledger == nil {
		return nil
	}
	payload := newSessionFallbackEventPayload(seq, attempt, route)
	content, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("session: marshal %s: %w", eventspkg.SessionFallbackUsed, err)
	}
	summary := store.EventSummary{
		Type:      eventspkg.SessionFallbackUsed,
		AgentName: payload.Agent,
		// Provider stays empty: the ledger projects it from the session row, and the
		// attempted route's provider lives in the payload.
		Outcome:   string(eventspkg.OutcomeFor(eventspkg.SessionFallbackUsed)),
		Summary:   fmt.Sprintf("%s fallback attempt %d (%s)", payload.Agent, attempt, seq.phase),
		Timestamp: m.now(),
	}
	if seq.session != nil {
		info := seq.session.Info()
		summary.ProfileID = strings.TrimSpace(info.ProfileID)
		summary.SessionID = info.ID
		summary.WorkspaceID = info.WorkspaceID
		if lineage := store.NormalizeSessionLineage(info.ID, info.Lineage); lineage != nil {
			summary.ParentSessionID = lineage.ParentSessionID
			summary.RootSessionID = lineage.RootSessionID
			summary.SpawnDepth = lineage.SpawnDepth
		}
	}
	if summary.ProfileID == "" {
		summary.ProfileID = store.DefaultProfileID
	}
	summary.SetContent(content)
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sessionFallbackEventWriteTimeout)
	defer cancel()
	if err := seq.ledger.WriteEventSummary(writeCtx, summary); err != nil {
		return fmt.Errorf("session: record %s: %w", eventspkg.SessionFallbackUsed, err)
	}
	return nil
}

// decorateRefusedAttempt returns the failure with Action=use_fallback and the next-route
// guidance when kind is rate_limited or not_authenticated and a route remains; every
// other failure is returned unchanged (the classifier's action stands).
func decorateRefusedAttempt(failure *store.SessionFailure, next *FallbackRoute) *store.SessionFailure {
	decorated := store.CloneSessionFailure(failure)
	if decorated == nil || next == nil {
		return decorated
	}
	diagnostic, ok := acp.ProviderFailureDiagnosticFromSummary(decorated.Summary)
	if !ok {
		return decorated
	}
	switch diagnostic.Kind {
	case acp.ProviderFailureRateLimited, acp.ProviderFailureUnauthenticated:
	default:
		return decorated
	}
	diagnostic.Action = acp.ProviderFailureActionUseFallback
	diagnostic.Guidance = fmt.Sprintf("CompozyOS is trying the next configured route (%s)", next.label())
	decorated.Summary = acp.ReplaceProviderFailureDiagnostic(decorated.Summary, diagnostic)
	return decorated
}

// recordRefusedAttemptMarker keeps one provider_failure marker for a refused attempt the
// chain moves past, attributed to the refused route's command fingerprint.
func (m *Manager) recordRefusedAttemptMarker(
	ctx context.Context,
	seq fallbackSequence,
	attempt int,
	route FallbackRoute,
	next *FallbackRoute,
	cause error,
) {
	failure := decorateRefusedAttempt(sessionFailureFromError(cause, store.FailureStartup), next)
	evidence := map[string]any{
		transcriptMarkerEvidenceSourceKey: "fallback_chain",
		"phase":                           seq.phase,
		"attempt":                         attempt,
		"provider":                        strings.TrimSpace(route.Provider),
		"model":                           strings.TrimSpace(route.Model),
	}
	if failure != nil {
		evidence["failure_kind"] = string(failure.Kind)
		if diagnostic, ok := acp.ProviderFailureDiagnosticFromSummary(failure.Summary); ok {
			evidence["provider_failure_kind"] = string(diagnostic.Kind)
			evidence["next_action"] = string(diagnostic.Action)
			evidence["guidance"] = diagnostic.Guidance
		}
	}
	summary := fmt.Sprintf("Provider refused route %d · trying the next configured route (%s)",
		attempt+1, next.label())
	command := seq.commandFor(route)
	seq.log().Info("session.fallback.attempt_refused",
		"phase", seq.phase, "attempt", attempt,
		"next_action", evidence["next_action"],
		"provider_command_fingerprint", providerCommandFingerprint(command))
	if seq.session == nil {
		return
	}
	turnID, err := m.newPromptTurnID()
	if err == nil {
		err = m.recordTranscriptMarker(ctx, seq.session, turnID, transcriptMarkerInput{
			Kind: transcript.MarkerProviderFailure, Summary: summary, Evidence: evidence,
			Route: &route, Command: command,
		})
	}
	if err != nil {
		seq.log().Warn("session.fallback.marker_failed", "phase", seq.phase, "attempt", attempt, "error", err)
	}
}
