package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	commandpkg "github.com/compozy/compozy/internal/command"
)

const (
	SessionPromptOperationPrompt = "prompt"
	SessionPromptOperationSteer  = "steer"

	SessionPromptAdmissionReserved          = "reserved"
	SessionPromptAdmissionDispatchCommitted = "dispatch_committed"
	SessionPromptAdmissionCompleted         = "completed"
	SessionPromptAdmissionIndeterminate     = "indeterminate"
)

const (
	SessionPromptResultStatusAccepted     = "accepted"
	SessionPromptResultStatusQueued       = "queued"
	SessionPromptResultStatusSteering     = "steering"
	SessionPromptResultStatusInterrupting = "interrupting"
	SessionPromptResultStatusCanceled     = "canceled"
)

var (
	ErrSessionPromptAdmissionInProgress   = errors.New("store: session prompt admission is in progress")
	ErrSessionPromptIdempotencyConflict   = errors.New("store: session prompt idempotency conflict")
	ErrSessionPromptMessageConflict       = errors.New("store: session prompt message identity conflict")
	ErrSessionPromptDispatchIndeterminate = errors.New(
		"store: session prompt dispatch is indeterminate",
	)
)

// SessionPromptAdmissionResult is the durable non-streaming result replayed for one admitted command.
type SessionPromptAdmissionResult struct {
	Status                string          `json:"status"`
	Mode                  string          `json:"mode,omitempty"`
	QueueEntryID          string          `json:"queue_entry_id,omitempty"`
	QueuePosition         int             `json:"queue_position,omitempty"`
	QueueGeneration       int64           `json:"queue_generation,omitempty"`
	Delivery              string          `json:"delivery,omitempty"`
	PreviousTurnID        string          `json:"previous_turn_id,omitempty"`
	NewTurnID             string          `json:"new_turn_id,omitempty"`
	CanceledQueuedEntries int             `json:"canceled_queued_entries,omitempty"`
	Goal                  json.RawMessage `json:"goal,omitempty"`
}

// SessionPromptAdmission is the durable command receipt for one external prompt or steer request.
type SessionPromptAdmission struct {
	ID                  string
	WorkspaceID         string
	SessionID           string
	MessageID           string
	IdempotencyKey      string
	Operation           string
	FingerprintVersion  string
	RequestFingerprint  string
	State               string
	Mode                string
	AuthoredText        string
	Runtime             SessionInputRuntime
	SkillInvocations    []commandpkg.Invocation
	Attachments         []SessionInputAttachment
	TurnID              string
	EventID             string
	Result              *SessionPromptAdmissionResult
	IndeterminateReason string
	CreatedAt           time.Time
	DispatchCommittedAt *time.Time
	CompletedAt         *time.Time
	UpdatedAt           time.Time
}

// SessionPromptAdmissionRequest carries immutable command identity and request material.
type SessionPromptAdmissionRequest struct {
	ID                 string
	WorkspaceID        string
	SessionID          string
	MessageID          string
	IdempotencyKey     string
	Operation          string
	FingerprintVersion string
	RequestFingerprint string
	Mode               string
	AuthoredText       string
	Runtime            SessionInputRuntime
	SkillInvocations   []commandpkg.Invocation
	Attachments        []SessionInputAttachment
	TurnID             string
	EventID            string
	Now                time.Time
}

// Normalize trims identity fields and normalizes the request clock.
func (r SessionPromptAdmissionRequest) Normalize() SessionPromptAdmissionRequest {
	normalized := r
	normalized.ID = strings.TrimSpace(normalized.ID)
	normalized.WorkspaceID = strings.TrimSpace(normalized.WorkspaceID)
	normalized.SessionID = strings.TrimSpace(normalized.SessionID)
	normalized.MessageID = strings.TrimSpace(normalized.MessageID)
	normalized.IdempotencyKey = strings.TrimSpace(normalized.IdempotencyKey)
	normalized.Operation = strings.TrimSpace(normalized.Operation)
	normalized.FingerprintVersion = strings.TrimSpace(normalized.FingerprintVersion)
	normalized.RequestFingerprint = strings.TrimSpace(normalized.RequestFingerprint)
	normalized.Mode = strings.TrimSpace(normalized.Mode)
	normalized.AuthoredText = strings.TrimSpace(normalized.AuthoredText)
	normalized.Runtime = normalized.Runtime.Normalize()
	normalized.SkillInvocations = append([]commandpkg.Invocation(nil), normalized.SkillInvocations...)
	normalized.Attachments = cloneSessionInputAttachments(normalized.Attachments)
	normalized.TurnID = strings.TrimSpace(normalized.TurnID)
	normalized.EventID = strings.TrimSpace(normalized.EventID)
	if normalized.Now.IsZero() {
		normalized.Now = time.Now().UTC()
	} else {
		normalized.Now = normalized.Now.UTC()
	}
	return normalized
}

// Validate ensures an admission has complete, stable identity before persistence.
func (r SessionPromptAdmissionRequest) Validate() error {
	normalized := r.Normalize()
	switch {
	case normalized.ID == "":
		return errors.New("store: session prompt admission id is required")
	case normalized.WorkspaceID == "":
		return errors.New("store: session prompt admission workspace id is required")
	case normalized.SessionID == "":
		return errors.New("store: session prompt admission session id is required")
	case normalized.MessageID == "":
		return errors.New("store: session prompt admission message id is required")
	case normalized.IdempotencyKey == "":
		return errors.New("store: session prompt admission idempotency key is required")
	case normalized.FingerprintVersion == "":
		return errors.New("store: session prompt admission fingerprint version is required")
	case normalized.RequestFingerprint == "":
		return errors.New("store: session prompt admission request fingerprint is required")
	case normalized.Operation == SessionPromptOperationSteer && len(normalized.Attachments) > 0:
		return ErrSessionInputSteerTextOnly
	case normalized.AuthoredText == "" &&
		(normalized.Operation == SessionPromptOperationSteer || len(normalized.Attachments) == 0):
		return errors.New("store: session prompt admission authored text is required")
	case normalized.TurnID == "":
		return errors.New("store: session prompt admission turn id is required")
	case normalized.EventID == "":
		return errors.New("store: session prompt admission event id is required")
	}
	switch normalized.Operation {
	case SessionPromptOperationPrompt, SessionPromptOperationSteer:
		return nil
	default:
		return fmt.Errorf("store: invalid session prompt operation %q", normalized.Operation)
	}
}

// SessionPromptAdmissionStore persists command receipts and their queue-linked admission decisions.
type SessionPromptAdmissionStore interface {
	ReplaySessionPromptAdmission(
		ctx context.Context,
		request SessionPromptAdmissionRequest,
	) (SessionPromptAdmission, bool, error)
	ClaimSessionPromptAdmission(
		ctx context.Context,
		request SessionPromptAdmissionRequest,
	) (SessionPromptAdmission, bool, error)
	CommitSessionPromptDispatch(
		ctx context.Context,
		workspaceID string,
		sessionID string,
		idempotencyKey string,
		now time.Time,
	) error
	CompleteSessionPromptAdmission(
		ctx context.Context,
		workspaceID string,
		sessionID string,
		idempotencyKey string,
		result SessionPromptAdmissionResult,
		now time.Time,
	) (SessionPromptAdmission, error)
	MarkSessionPromptAdmissionIndeterminate(
		ctx context.Context,
		workspaceID string,
		sessionID string,
		idempotencyKey string,
		reason string,
		now time.Time,
	) error
	EnqueueAdmittedSessionInput(
		ctx context.Context,
		request SessionPromptAdmissionRequest,
		queueInput SessionInputQueueInsert,
	) (SessionPromptAdmission, SessionInputQueueEntry, int, bool, error)
	StageAdmittedSessionSteer(
		ctx context.Context,
		request SessionPromptAdmissionRequest,
		queueInput SessionInputQueueInsert,
	) (SessionPromptAdmission, SessionInputQueueEntry, bool, error)
}
