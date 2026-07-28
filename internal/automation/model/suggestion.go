package model

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrSuggestionNotFound   = errors.New("automation: suggestion not found")
	ErrSuggestionResolved   = errors.New("automation: suggestion already resolved")
	ErrSuggestionPendingCap = errors.New("automation: suggestion pending cap reached")
)

// DefaultSuggestionPendingCap bounds unresolved suggestions per workspace.
const DefaultSuggestionPendingCap = 5

// SuggestionSource identifies the emitter that proposed a Job.
type SuggestionSource string

const (
	// SuggestionSourceCatalog identifies the first-party starter catalog.
	SuggestionSourceCatalog SuggestionSource = "catalog"
	// SuggestionSourceUsage reserves usage-derived suggestions for a future emitter.
	SuggestionSourceUsage SuggestionSource = "usage"
	// SuggestionSourceIntegration reserves integration-derived suggestions for a future emitter.
	SuggestionSourceIntegration SuggestionSource = "integration"
)

// Validate ensures the source belongs to the closed public enum.
func (s SuggestionSource) Validate(path string) error {
	switch SuggestionSource(strings.TrimSpace(string(s))) {
	case SuggestionSourceCatalog, SuggestionSourceUsage, SuggestionSourceIntegration:
		return nil
	default:
		return fmt.Errorf("%s must be one of %q, %q, or %q: %q", path,
			SuggestionSourceCatalog, SuggestionSourceUsage, SuggestionSourceIntegration, s)
	}
}

// SuggestionStatus identifies the consent resolution state.
type SuggestionStatus string

const (
	// SuggestionStatusPending has not received operator or agent consent.
	SuggestionStatusPending SuggestionStatus = "pending"
	// SuggestionStatusAccepted records durable consent to create the Job.
	SuggestionStatusAccepted SuggestionStatus = "accepted"
	// SuggestionStatusDismissed records durable refusal.
	SuggestionStatusDismissed SuggestionStatus = "dismissed"
)

// Validate ensures the status belongs to the closed public enum.
func (s SuggestionStatus) Validate(path string) error {
	switch SuggestionStatus(strings.TrimSpace(string(s))) {
	case SuggestionStatusPending, SuggestionStatusAccepted, SuggestionStatusDismissed:
		return nil
	default:
		return fmt.Errorf("%s must be one of %q, %q, or %q: %q", path,
			SuggestionStatusPending, SuggestionStatusAccepted, SuggestionStatusDismissed, s)
	}
}

// Suggestion is a workspace-scoped proposal for a prefilled Job.
type Suggestion struct {
	ID          string           `json:"id"`
	WorkspaceID string           `json:"workspace_id"`
	Source      SuggestionSource `json:"source"`
	DedupKey    string           `json:"dedup_key"`
	Status      SuggestionStatus `json:"status"`
	Payload     Job              `json:"payload"`
	CreatedAt   time.Time        `json:"created_at"`
	ResolvedAt  *time.Time       `json:"resolved_at,omitempty"`
}

// Validate ensures persisted suggestion metadata and resolution state are coherent.
func (s Suggestion) Validate(path string) error {
	if strings.TrimSpace(s.ID) == "" {
		return errors.New(nestedPath(path, "id") + " is required")
	}
	if strings.TrimSpace(s.WorkspaceID) == "" {
		return errors.New(nestedPath(path, "workspace_id") + " is required")
	}
	if err := s.Source.Validate(nestedPath(path, "source")); err != nil {
		return err
	}
	if strings.TrimSpace(s.DedupKey) == "" {
		return errors.New(nestedPath(path, "dedup_key") + " is required")
	}
	if err := s.Status.Validate(nestedPath(path, "status")); err != nil {
		return err
	}
	if strings.TrimSpace(s.Payload.ID) == "" {
		return errors.New(nestedPath(path, "payload.id") + " is required")
	}
	if s.Payload.Scope != AutomationScopeWorkspace {
		return fmt.Errorf(
			"%s must be %q: %q",
			nestedPath(path, "payload.scope"),
			AutomationScopeWorkspace,
			s.Payload.Scope,
		)
	}
	if strings.TrimSpace(s.Payload.WorkspaceID) != strings.TrimSpace(s.WorkspaceID) {
		return fmt.Errorf(
			"%s must match %s",
			nestedPath(path, "payload.workspace_id"),
			nestedPath(path, "workspace_id"),
		)
	}
	if s.Payload.Source != JobSourceDynamic {
		return fmt.Errorf(
			"%s must be %q: %q",
			nestedPath(path, "payload.source"),
			JobSourceDynamic,
			s.Payload.Source,
		)
	}
	if err := s.Payload.Validate(nestedPath(path, "payload")); err != nil {
		return err
	}
	if s.CreatedAt.IsZero() {
		return errors.New(nestedPath(path, "created_at") + " is required")
	}
	if s.Status == SuggestionStatusPending && s.ResolvedAt != nil {
		return errors.New(nestedPath(path, "resolved_at") + " must be empty while status is pending")
	}
	if s.Status != SuggestionStatusPending && (s.ResolvedAt == nil || s.ResolvedAt.IsZero()) {
		return errors.New(nestedPath(path, "resolved_at") + " is required after resolution")
	}
	return nil
}
