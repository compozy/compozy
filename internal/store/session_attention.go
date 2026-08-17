package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/diagnostics"
)

const (
	PendingInteractionKindPermission = "permission"
	PendingInteractionKindClarify    = "clarify"

	PendingInteractionStatusPending  = "pending"
	PendingInteractionStatusOrphaned = "orphaned"
	PendingInteractionStatusResolved = "resolved"
	PendingInteractionStatusTimedOut = "timed_out"
	PendingInteractionStatusCanceled = "canceled"

	PendingInteractionTitleMaxBytes      = 200
	PendingInteractionPayloadMaxBytes    = 4096
	PendingInteractionResolutionMaxBytes = 240
	PendingInteractionChoiceMaxBytes     = 120
)

const (
	PendingInteractionOutcomeApplied              = "applied"
	PendingInteractionOutcomeAnswered             = "answered"
	PendingInteractionOutcomeResolvedAfterRestart = "resolved-after-restart"
	PendingInteractionOutcomeAlreadyResolved      = "already-resolved"
	PendingInteractionOutcomeQueueFull            = "queue-full"
)

var (
	ErrPendingInteractionNotFound        = errors.New("store: pending interaction not found")
	ErrPendingInteractionAlreadyResolved = errors.New("store: pending interaction already resolved")
	ErrPendingInteractionConflict        = errors.New("store: pending interaction conflict")
	ErrPendingInteractionStatusInvalid   = errors.New("store: pending interaction status is invalid")
)

// SessionAttention is the catalog-owned attention projection for one session.
type SessionAttention struct {
	PendingPermissionCount int
	PendingClarifyCount    int
	AttentionRevision      int64
	LastSettledRevision    int64
	LastSeenRevision       int64
	LastSeenAt             *time.Time
	AttentionChangedAt     *time.Time
}

// CloneSessionAttention returns an isolated copy of an optional attention projection.
func CloneSessionAttention(attention *SessionAttention) *SessionAttention {
	if attention == nil {
		return nil
	}
	cloned := *attention
	if attention.LastSeenAt != nil {
		lastSeenAt := *attention.LastSeenAt
		cloned.LastSeenAt = &lastSeenAt
	}
	if attention.AttentionChangedAt != nil {
		attentionChangedAt := *attention.AttentionChangedAt
		cloned.AttentionChangedAt = &attentionChangedAt
	}
	return &cloned
}

// AttentionSnapshot returns an isolated zero-safe attention projection.
func (s *SessionInfo) AttentionSnapshot() SessionAttention {
	if s == nil || s.Attention == nil {
		return SessionAttention{}
	}
	return *CloneSessionAttention(s.Attention)
}

// Unseen reports whether the latest settled turn has not been observed.
func (a SessionAttention) Unseen() bool {
	return a.LastSettledRevision > a.LastSeenRevision
}

// PendingInteractionPayload is the allowlisted display-only interaction body.
type PendingInteractionPayload struct {
	Choices   []string `json:"choices,omitempty"`
	Decisions []string `json:"decisions,omitempty"`
}

// PendingInteraction is one restart-durable operator decision request.
type PendingInteraction struct {
	InteractionID     string
	SessionID         string
	Kind              string
	ProviderRequestID string
	TurnID            string
	Title             string
	Payload           PendingInteractionPayload
	Status            string
	CreatedAt         time.Time
	ResolvedAt        *time.Time
	Resolution        string
	ResolvedBy        string
}

// SanitizePendingInteraction applies the public redaction and size boundary to a canonical record.
func SanitizePendingInteraction(interaction PendingInteraction) PendingInteraction {
	content := PendingInteractionCreate{
		InteractionID:     interaction.InteractionID,
		SessionID:         interaction.SessionID,
		Kind:              interaction.Kind,
		ProviderRequestID: interaction.ProviderRequestID,
		TurnID:            interaction.TurnID,
		Title:             interaction.Title,
		Payload:           interaction.Payload,
		CreatedAt:         interaction.CreatedAt,
	}.Normalize()
	resolution := PendingInteractionTransition{
		InteractionID: interaction.InteractionID,
		Status:        interaction.Status,
		Resolution:    interaction.Resolution,
		ResolvedBy:    interaction.ResolvedBy,
		At:            interaction.CreatedAt,
	}.Normalize()
	interaction.InteractionID = content.InteractionID
	interaction.SessionID = content.SessionID
	interaction.Kind = content.Kind
	interaction.ProviderRequestID = content.ProviderRequestID
	interaction.TurnID = content.TurnID
	interaction.Title = content.Title
	interaction.Payload = content.Payload
	interaction.Status = strings.TrimSpace(interaction.Status)
	interaction.Resolution = resolution.Resolution
	interaction.ResolvedBy = resolution.ResolvedBy
	if interaction.ResolvedAt != nil {
		resolvedAt := interaction.ResolvedAt.UTC()
		interaction.ResolvedAt = &resolvedAt
	}
	return interaction
}

// PendingInteractionCreate records one new pending provider request.
type PendingInteractionCreate struct {
	InteractionID     string
	SessionID         string
	Kind              string
	ProviderRequestID string
	TurnID            string
	Title             string
	Payload           PendingInteractionPayload
	CreatedAt         time.Time
}

// Normalize redacts and bounds content before it can reach persistence.
func (r PendingInteractionCreate) Normalize() PendingInteractionCreate {
	r.InteractionID = strings.TrimSpace(r.InteractionID)
	r.SessionID = strings.TrimSpace(r.SessionID)
	r.Kind = strings.TrimSpace(r.Kind)
	r.ProviderRequestID = strings.TrimSpace(r.ProviderRequestID)
	r.TurnID = strings.TrimSpace(r.TurnID)
	r.Title = diagnostics.RedactAndBound(r.Title, PendingInteractionTitleMaxBytes)
	r.Payload = normalizePendingInteractionPayload(r.Payload)
	if !r.CreatedAt.IsZero() {
		r.CreatedAt = r.CreatedAt.UTC()
	}
	return r
}

// Validate rejects incomplete identities and payloads that cannot fit the public contract.
func (r PendingInteractionCreate) Validate() error {
	normalized := r.Normalize()
	switch {
	case normalized.InteractionID == "":
		return errors.New("store: interaction id is required")
	case normalized.SessionID == "":
		return errors.New("store: interaction session id is required")
	case normalized.ProviderRequestID == "":
		return errors.New("store: provider request id is required")
	case normalized.CreatedAt.IsZero():
		return errors.New("store: interaction created_at is required")
	}
	if !ValidPendingInteractionKind(normalized.Kind) {
		return fmt.Errorf("store: invalid pending interaction kind %q", normalized.Kind)
	}
	payload, err := json.Marshal(normalized.Payload)
	if err != nil {
		return fmt.Errorf("store: encode pending interaction payload: %w", err)
	}
	if len(payload) > PendingInteractionPayloadMaxBytes {
		return fmt.Errorf("store: pending interaction payload exceeds %d bytes", PendingInteractionPayloadMaxBytes)
	}
	return nil
}

// PendingInteractionTransition moves one pending record to another lifecycle status.
type PendingInteractionTransition struct {
	InteractionID string
	Status        string
	Resolution    string
	ResolvedBy    string
	At            time.Time
	OrphanInput   *SessionInputQueueInsert
}

// Normalize applies the same secret and size boundary used by every read surface.
func (r PendingInteractionTransition) Normalize() PendingInteractionTransition {
	r.InteractionID = strings.TrimSpace(r.InteractionID)
	r.Status = strings.TrimSpace(r.Status)
	r.Resolution = diagnostics.RedactAndBound(r.Resolution, PendingInteractionResolutionMaxBytes)
	r.ResolvedBy = diagnostics.RedactAndBound(strings.TrimSpace(r.ResolvedBy), PendingInteractionResolutionMaxBytes)
	if !r.At.IsZero() {
		r.At = r.At.UTC()
	}
	if r.OrphanInput != nil {
		normalizedInput := r.OrphanInput.Normalize()
		r.OrphanInput = &normalizedInput
	}
	return r
}

// Validate ensures terminal transitions are explicit and attributable.
func (r PendingInteractionTransition) Validate() error {
	normalized := r.Normalize()
	if normalized.InteractionID == "" {
		return errors.New("store: interaction id is required")
	}
	if normalized.At.IsZero() {
		return errors.New("store: interaction transition time is required")
	}
	switch normalized.Status {
	case PendingInteractionStatusOrphaned:
		if normalized.Resolution != "" || normalized.ResolvedBy != "" || normalized.OrphanInput != nil {
			return errors.New("store: orphan transition cannot resolve interaction")
		}
	case PendingInteractionStatusResolved:
		if normalized.Resolution == "" || normalized.ResolvedBy == "" {
			return errors.New("store: resolved interaction requires resolution and actor")
		}
	case PendingInteractionStatusTimedOut, PendingInteractionStatusCanceled:
		if normalized.OrphanInput != nil {
			return errors.New("store: terminal interaction cannot enqueue orphan input")
		}
	default:
		return fmt.Errorf("store: invalid interaction transition status %q", normalized.Status)
	}
	if normalized.OrphanInput != nil {
		if err := normalized.OrphanInput.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// SessionAttentionCommit reports one committed canonical attention mutation.
type SessionAttentionCommit struct {
	Before      SessionAttention
	After       SessionAttention
	Interaction *PendingInteraction
	Outcome     string
}

// SessionAttentionSummary is the exact operator-wide attention aggregate.
type SessionAttentionSummary struct {
	NeedsYou    int
	Finished    int
	ByWorkspace []WorkspaceAttentionSummary
}

// WorkspaceAttentionSummary is one exact workspace aggregate.
type WorkspaceAttentionSummary struct {
	WorkspaceID string
	NeedsYou    int
	Finished    int
}

// SessionAttentionStore owns canonical pending interactions and revision fences.
type SessionAttentionStore interface {
	GetSessionAttention(ctx context.Context, sessionID string) (SessionAttention, error)
	CreatePendingInteraction(ctx context.Context, req PendingInteractionCreate) (SessionAttentionCommit, error)
	TransitionPendingInteraction(
		ctx context.Context,
		req PendingInteractionTransition,
	) (SessionAttentionCommit, error)
	ListPendingInteractions(ctx context.Context, sessionID string, statuses []string) ([]PendingInteraction, error)
	MarkSessionSettled(ctx context.Context, sessionID string, seen bool, at time.Time) (SessionAttentionCommit, error)
	MarkSessionSeen(ctx context.Context, sessionID string, at time.Time) (SessionAttentionCommit, error)
}

// ValidPendingInteractionKind reports whether kind belongs to the durable vocabulary.
func ValidPendingInteractionKind(kind string) bool {
	switch strings.TrimSpace(kind) {
	case PendingInteractionKindPermission, PendingInteractionKindClarify:
		return true
	default:
		return false
	}
}

// ValidPendingInteractionStatus reports whether status belongs to the durable vocabulary.
func ValidPendingInteractionStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case PendingInteractionStatusPending,
		PendingInteractionStatusOrphaned,
		PendingInteractionStatusResolved,
		PendingInteractionStatusTimedOut,
		PendingInteractionStatusCanceled:
		return true
	default:
		return false
	}
}

func normalizePendingInteractionPayload(payload PendingInteractionPayload) PendingInteractionPayload {
	result := PendingInteractionPayload{
		Choices:   make([]string, 0, len(payload.Choices)),
		Decisions: make([]string, 0, len(payload.Decisions)),
	}
	for _, choice := range payload.Choices {
		if normalized := diagnostics.RedactAndBound(choice, PendingInteractionChoiceMaxBytes); normalized != "" {
			result.Choices = append(result.Choices, normalized)
		}
	}
	seen := make(map[string]struct{}, len(payload.Decisions))
	for _, decision := range payload.Decisions {
		normalized := strings.TrimSpace(decision)
		if !validPendingInteractionDecision(normalized) {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result.Decisions = append(result.Decisions, normalized)
	}
	return result
}

func validPendingInteractionDecision(value string) bool {
	switch value {
	case "allow-once", "allow-always", "reject-once", "reject-always":
		return true
	default:
		return false
	}
}
