package transcript

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/diagnostics"
	"github.com/compozy/agh/internal/events"
)

const (
	MarkerPromptCancel           = "transcript_marker.prompt_cancel"
	MarkerPromptTimeout          = "transcript_marker.prompt_timeout"
	MarkerPromptInterrupted      = "transcript_marker.prompt_interrupted"
	MarkerPromptSteered          = "transcript_marker.prompt_steered"
	MarkerPromptQueued           = "transcript_marker.prompt_queued"
	MarkerPromptAccepted         = "transcript_marker.prompt_accepted"
	MarkerPromptDropped          = "transcript_marker.prompt_dropped"
	MarkerSessionUnhealthy       = "transcript_marker.session_unhealthy"
	MarkerSessionRecovered       = "transcript_marker.session_recovered"
	MarkerProviderFailure        = "transcript_marker.provider_failure"
	MarkerMCPAuthRequired        = "transcript_marker.mcp_auth_required"
	MarkerFileMutationUnverified = "transcript_marker.file_mutation_unverified"

	maxMarkerSummaryBytes = 2048
)

var errTranscriptMarkerInvalid = errors.New("transcript: invalid marker")

// Marker is the canonical transcript marker payload persisted as a typed event.
type Marker struct {
	Kind       string          `json:"kind"`
	OccurredAt time.Time       `json:"occurred_at"`
	Summary    string          `json:"summary"`
	Evidence   map[string]any  `json:"evidence,omitempty"`
	Diagnostic json.RawMessage `json:"diagnostic,omitempty"`
}

// NewMarker builds a redacted marker payload from daemon-owned runtime evidence.
func NewMarker(kind string, summary string, occurredAt time.Time, evidence map[string]any) (Marker, error) {
	marker := Marker{
		Kind:       strings.TrimSpace(kind),
		OccurredAt: occurredAt.UTC(),
		Summary:    diagnostics.RedactAndBound(strings.TrimSpace(summary), maxMarkerSummaryBytes),
		Evidence:   diagnostics.RedactEvidence(evidence),
	}
	if marker.OccurredAt.IsZero() {
		marker.OccurredAt = time.Now().UTC()
	}
	if err := marker.Validate(); err != nil {
		return Marker{}, err
	}
	return marker, nil
}

// Validate ensures the marker is one of the closed runtime marker vocabulary.
func (m Marker) Validate() error {
	if !validMarkerKind(m.Kind) {
		return fmt.Errorf("%w kind %q", errTranscriptMarkerInvalid, m.Kind)
	}
	if strings.TrimSpace(m.Summary) == "" {
		return fmt.Errorf("%w summary is required", errTranscriptMarkerInvalid)
	}
	return nil
}

// Normalize returns a redacted, UTC-normalized marker copy.
func (m Marker) Normalize() Marker {
	normalized := Marker{
		Kind:       strings.TrimSpace(m.Kind),
		OccurredAt: m.OccurredAt.UTC(),
		Summary:    diagnostics.RedactAndBound(strings.TrimSpace(m.Summary), maxMarkerSummaryBytes),
		Evidence:   diagnostics.RedactEvidence(m.Evidence),
		Diagnostic: redactRawMessage(m.Diagnostic),
	}
	if normalized.OccurredAt.IsZero() {
		normalized.OccurredAt = time.Now().UTC()
	}
	return normalized
}

// AgentEvent converts the marker to the durable transcript marker event shape.
func (m Marker) AgentEvent(sessionID string, turnID string) (acp.AgentEvent, error) {
	normalized := m.Normalize()
	if err := normalized.Validate(); err != nil {
		return acp.AgentEvent{}, err
	}
	raw, err := json.Marshal(normalized)
	if err != nil {
		return acp.AgentEvent{}, fmt.Errorf("transcript: marshal marker: %w", err)
	}
	return acp.AgentEvent{
		Type:      events.TranscriptMarkerCreated,
		SessionID: strings.TrimSpace(sessionID),
		TurnID:    strings.TrimSpace(turnID),
		Timestamp: normalized.OccurredAt,
		Title:     normalized.Kind,
		Text:      normalized.Summary,
		Raw:       json.RawMessage(raw),
	}, nil
}

// ParseMarker decodes and validates a persisted marker payload.
func ParseMarker(raw json.RawMessage) (Marker, bool) {
	if len(raw) == 0 {
		return Marker{}, false
	}
	var marker Marker
	if err := json.Unmarshal(raw, &marker); err != nil {
		return Marker{}, false
	}
	marker = marker.Normalize()
	if err := marker.Validate(); err != nil {
		return Marker{}, false
	}
	return marker, true
}

func transcriptMarkerText(parsed event) string {
	if parsed.Marker == nil {
		return ""
	}
	marker := parsed.Marker.Normalize()
	return strings.TrimSpace(marker.Summary)
}

func parseMarkerFromPayload(payload map[string]any) *Marker {
	raw := firstNonEmptyRaw(
		rawMessageFromValue(payload["raw"]),
		rawMessageFromValue(payload["marker"]),
	)
	marker, ok := ParseMarker(raw)
	if !ok {
		return nil
	}
	return &marker
}

func validMarkerKind(kind string) bool {
	switch strings.TrimSpace(kind) {
	case MarkerPromptCancel,
		MarkerPromptTimeout,
		MarkerPromptInterrupted,
		MarkerPromptSteered,
		MarkerPromptQueued,
		MarkerPromptAccepted,
		MarkerPromptDropped,
		MarkerSessionUnhealthy,
		MarkerSessionRecovered,
		MarkerProviderFailure,
		MarkerMCPAuthRequired,
		MarkerFileMutationUnverified:
		return true
	default:
		return false
	}
}
