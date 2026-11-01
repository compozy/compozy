package transcript

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/events"
	redactpkg "github.com/compozy/compozy/internal/redact"
)

const (
	MarkerPromptCancel           = "transcript_marker.prompt_cancel"
	MarkerPromptTimeout          = "transcript_marker.prompt_timeout"
	MarkerPromptInterrupted      = "transcript_marker.prompt_interrupted"
	MarkerPromptSuperseded       = "transcript_marker.prompt_superseded"
	MarkerPromptSteered          = "transcript_marker.prompt_steered"
	MarkerPromptQueued           = "transcript_marker.prompt_queued"
	MarkerPromptAccepted         = "transcript_marker.prompt_accepted"
	MarkerPromptDropped          = "transcript_marker.prompt_dropped"
	MarkerQueueCleared           = "transcript_marker.queue_cleared"
	MarkerSessionUnhealthy       = "transcript_marker.session_unhealthy"
	MarkerPostStop               = "transcript_marker.post_stop"
	MarkerSessionRecovered       = "transcript_marker.session_recovered"
	MarkerProviderFailure        = "transcript_marker.provider_failure"
	MarkerMCPAuthRequired        = "transcript_marker.mcp_auth_required"
	MarkerFileMutationUnverified = "transcript_marker.file_mutation_unverified"
	MarkerRuntimeChanged         = "transcript_marker.runtime_changed"

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
		Evidence:   redactMarkerEvidence(evidence),
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
		Evidence:   redactMarkerEvidence(m.Evidence),
		Diagnostic: redactRawMessage(m.Diagnostic),
	}
	if normalized.OccurredAt.IsZero() {
		normalized.OccurredAt = time.Now().UTC()
	}
	return normalized
}

// redactMarkerEvidence redacts free-text evidence like diagnostics evidence but
// gives identity handles (`id`, `*_id`) the exact-only structural redaction that
// tool call ids get: the entropy heuristic would collapse distinct ULID-shaped
// handles (`wake_01…`, `inq_01…`) into one `[REDACTED]` identity, which clients
// bind steer provenance by. Sensitive key names stay fully redacted.
func redactMarkerEvidence(evidence map[string]any) map[string]any {
	redacted := diagnostics.RedactEvidence(evidence)
	for key, value := range evidence {
		key = strings.TrimSpace(key)
		handle, ok := value.(string)
		if !ok || !isMarkerIdentityKey(key) || redactpkg.IsSensitiveKey(key) {
			continue
		}
		redacted[key] = redactStructuralString(handle)
	}
	return redacted
}

func isMarkerIdentityKey(key string) bool {
	return key == "id" || strings.HasSuffix(key, "_id")
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
		MarkerPromptSuperseded,
		MarkerPromptQueued,
		MarkerPromptAccepted,
		MarkerPromptDropped,
		MarkerQueueCleared,
		MarkerSessionUnhealthy,
		MarkerSessionRecovered,
		MarkerPostStop,
		MarkerProviderFailure,
		MarkerMCPAuthRequired,
		MarkerFileMutationUnverified,
		MarkerRuntimeChanged:
		return true
	default:
		return false
	}
}
