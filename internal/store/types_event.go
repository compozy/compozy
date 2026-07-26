package store

import (
	"fmt"
	"strings"
	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
)

// SessionEvent is a persisted event row for a single AGH session.
type SessionEvent struct {
	ID        string
	SessionID string
	Sequence  int64
	TurnID    string
	Type      string
	AgentName string
	Content   string
	Archived  bool
	Timestamp time.Time
}

// SessionLedgerRecord carries the durable session evidence needed to materialize
// a forensic session ledger after the live event store has been closed.
type SessionLedgerRecord struct {
	SessionID    string
	WorkspaceID  string
	AgentName    string
	SessionType  string
	EventsDBPath string
	Lineage      *SessionLineage
	StartedAt    time.Time
	EndedAt      time.Time
}

// EventCorrelation carries the canonical cross-surface correlation keys for
// session and observability events.
type EventCorrelation struct {
	TaskID               string     `json:"task_id,omitempty"`
	RunID                string     `json:"run_id,omitempty"`
	WorkflowID           string     `json:"workflow_id,omitempty"`
	ClaimTokenHash       string     `json:"claim_token_hash,omitempty"`
	LeaseUntil           *time.Time `json:"lease_until,omitempty"`
	CoordinatorSessionID string     `json:"coordinator_session_id,omitempty"`
	SchedulerReason      string     `json:"scheduler_reason,omitempty"`
	HookEvent            string     `json:"hook_event,omitempty"`
	HookName             string     `json:"hook_name,omitempty"`
	ActorKind            string     `json:"actor_kind,omitempty"`
	ActorID              string     `json:"actor_id,omitempty"`
	ReleaseReason        string     `json:"release_reason,omitempty"`
}

// Normalize trims string fields and canonicalizes timestamps.
func (c EventCorrelation) Normalize() EventCorrelation {
	normalized := EventCorrelation{
		TaskID:               strings.TrimSpace(c.TaskID),
		RunID:                strings.TrimSpace(c.RunID),
		WorkflowID:           strings.TrimSpace(c.WorkflowID),
		ClaimTokenHash:       strings.TrimSpace(c.ClaimTokenHash),
		CoordinatorSessionID: strings.TrimSpace(c.CoordinatorSessionID),
		SchedulerReason:      strings.TrimSpace(c.SchedulerReason),
		HookEvent:            strings.TrimSpace(c.HookEvent),
		HookName:             strings.TrimSpace(c.HookName),
		ActorKind:            strings.TrimSpace(c.ActorKind),
		ActorID:              strings.TrimSpace(c.ActorID),
		ReleaseReason:        strings.TrimSpace(c.ReleaseReason),
	}
	normalized.LeaseUntil = cloneNormalizedTimestamp(c.LeaseUntil)
	return normalized
}

// IsZero reports whether the correlation payload carries any fields.
func (c EventCorrelation) IsZero() bool {
	normalized := c.Normalize()
	return normalized.TaskID == "" &&
		normalized.RunID == "" &&
		normalized.WorkflowID == "" &&
		normalized.ClaimTokenHash == "" &&
		normalized.LeaseUntil == nil &&
		normalized.CoordinatorSessionID == "" &&
		normalized.SchedulerReason == "" &&
		normalized.HookEvent == "" &&
		normalized.HookName == "" &&
		normalized.ActorKind == "" &&
		normalized.ActorID == "" &&
		normalized.ReleaseReason == ""
}

// Validate ensures the event has the required fields for persistence.
func (e SessionEvent) Validate() error {
	if err := requireField(e.TurnID, "event turn id"); err != nil {
		return err
	}
	if err := requireField(e.Type, "event type"); err != nil {
		return err
	}
	if err := requireField(e.AgentName, "event agent name"); err != nil {
		return err
	}
	return nil
}

// EventQuery filters per-session events while preserving follow-friendly ordering.
type EventQuery struct {
	Type           string
	AgentName      string
	TurnID         string
	Since          time.Time
	Limit          int
	AfterSequence  int64
	BeforeSequence int64
	Archive        EventArchiveFilter
}

// Validate ensures the query is internally consistent.
func (q EventQuery) Validate() error {
	if err := requirePositiveLimit(q.Limit, "event limit"); err != nil {
		return err
	}
	if q.AfterSequence < 0 {
		return fmt.Errorf("store: invalid event after sequence %d", q.AfterSequence)
	}
	if q.BeforeSequence < 0 {
		return fmt.Errorf("store: invalid event before sequence %d", q.BeforeSequence)
	}
	if q.AfterSequence > 0 && q.BeforeSequence > 0 {
		return fmt.Errorf(
			"store: event after sequence %d cannot be combined with before sequence %d",
			q.AfterSequence,
			q.BeforeSequence,
		)
	}
	if err := q.Archive.Validate(); err != nil {
		return err
	}
	return nil
}

// EventArchiveFilter selects rows by their non-destructive archive state.
type EventArchiveFilter string

const (
	// EventArchiveAll keeps archived and unarchived rows visible for history queries.
	EventArchiveAll EventArchiveFilter = ""
	// EventArchiveUnarchived selects rows eligible for transcript replay.
	EventArchiveUnarchived EventArchiveFilter = "unarchived"
	// EventArchiveArchived selects rows retained only for forensic history.
	EventArchiveArchived EventArchiveFilter = "archived"
)

// Validate rejects unsupported archive filters.
func (f EventArchiveFilter) Validate() error {
	switch f {
	case EventArchiveAll, EventArchiveUnarchived, EventArchiveArchived:
		return nil
	default:
		return fmt.Errorf("store: invalid event archive filter %q", f)
	}
}

// EventArchiveRequest identifies one closed event sequence range to archive.
type EventArchiveRequest struct {
	FromSequence int64
	ToSequence   int64
}

// Validate ensures an archive request names one positive ordered range.
func (r EventArchiveRequest) Validate() error {
	switch {
	case r.FromSequence <= 0:
		return fmt.Errorf("store: event archive from sequence must be positive: %d", r.FromSequence)
	case r.ToSequence < r.FromSequence:
		return fmt.Errorf(
			"store: event archive to sequence %d precedes from sequence %d",
			r.ToSequence,
			r.FromSequence,
		)
	default:
		return nil
	}
}

// EventArchiveResult reports the idempotent mutation applied to one range.
type EventArchiveResult struct {
	FromSequence  int64
	ToSequence    int64
	ArchivedCount int64
}

// TurnHistory groups ordered events by their turn identifier.
type TurnHistory struct {
	TurnID string
	Events []SessionEvent
}

// HookRunQuery filters persisted per-session hook run records.
type HookRunQuery struct {
	SessionID string
	Event     string
	Outcome   hookspkg.HookRunOutcome
	Since     time.Time
	Limit     int
}

// Validate ensures the query uses sane bounds.
func (q HookRunQuery) Validate() error {
	if q.Outcome != "" {
		if err := q.Outcome.Validate(); err != nil {
			return err
		}
	}
	return requirePositiveLimit(q.Limit, "hook run limit")
}

// TokenUsage captures per-turn usage data reported by an ACP provider.
type TokenUsage struct {
	TurnID           string
	InputTokens      *int64
	OutputTokens     *int64
	TotalTokens      *int64
	ThoughtTokens    *int64
	CacheReadTokens  *int64
	CacheWriteTokens *int64
	ContextUsed      *int64
	ContextSize      *int64
	CostAmount       *float64
	CostCurrency     *string
	Timestamp        time.Time
}

// Validate ensures the usage payload has the required fields.
func (u TokenUsage) Validate() error {
	return requireField(u.TurnID, "token usage turn id")
}

// StopReason classifies why a session ended.
