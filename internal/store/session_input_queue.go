package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	// SessionInputQueueModeQueue stores operator input to run after the active turn ends.
	SessionInputQueueModeQueue = "queue"
	// SessionInputQueueModeSteer stores operator guidance staged while a turn is active.
	SessionInputQueueModeSteer = "steer"
)

const (
	SessionInputQueueStatusQueued      = "queued"
	SessionInputQueueStatusDispatching = "dispatching"
	SessionInputQueueStatusSent        = "sent"
	SessionInputQueueStatusFailed      = "failed"
	SessionInputQueueStatusCanceled    = "canceled"
)

var (
	// ErrSessionInputQueueFull reports that accepting an entry would exceed the configured capacity.
	ErrSessionInputQueueFull = errors.New("store: session input queue full")
	// ErrSessionInputQueueEntryNotFound reports that a queued input entry does not exist for a session.
	ErrSessionInputQueueEntryNotFound = errors.New("store: session input queue entry not found")
)

// SessionInputQueueEntry is one persisted busy-input item.
type SessionInputQueueEntry struct {
	ID                       string
	SessionID                string
	Status                   string
	Mode                     string
	Text                     string
	SessionGeneration        int64
	TaskRunID                string
	RunGeneration            *int64
	LoopRunID                string
	OwnerKind                string
	OwnerEpoch               *int64
	BindingEpoch             *int64
	PromptID                 string
	PromptKind               string
	OperationUsageBaseTokens *int64
	PromptAttempt            int
	Dispatchable             bool
	DispatchTokenHash        string
	AttemptCount             int
	EnqueuedAt               time.Time
	ActivatedAt              *time.Time
	DispatchStartedAt        *time.Time
	SentAt                   *time.Time
	FailedAt                 *time.Time
	FailureSummary           string
	CanceledAt               *time.Time
	FenceKind                string
	FenceDisposition         string
	FenceReasonCode          string
	FencedAt                 *time.Time
	TerminalEventStartSeq    *int64
	TerminalEventEndSeq      *int64
	TerminalKind             string
	TerminalStopReason       string
	TerminalDisposition      string
	TerminalReasonCode       string
	TerminalTokensReported   bool
	TerminalTokensUsed       *int64
	TerminalAt               *time.Time
	UpdatedAt                time.Time
}

// SessionInputQueueSummary captures the current generation and pending entries for recap/status reads.
type SessionInputQueueSummary struct {
	SessionID     string
	Generation    int64
	PendingActive int
	PendingQueued int
	PendingSteer  int
	PendingLeased int
}

// SessionInputQueueInsert captures the atomic insert request for busy input.
type SessionInputQueueInsert struct {
	ID                string
	SessionID         string
	Mode              string
	Text              string
	SessionGeneration int64
	TaskRunID         string
	RunGeneration     *int64
	QueueCap          int
	Now               time.Time
}

// Normalize returns a trimmed, UTC-normalized insert request.
func (r SessionInputQueueInsert) Normalize() SessionInputQueueInsert {
	normalized := r
	normalized.ID = strings.TrimSpace(normalized.ID)
	normalized.SessionID = strings.TrimSpace(normalized.SessionID)
	normalized.Mode = strings.TrimSpace(normalized.Mode)
	normalized.Text = strings.TrimSpace(normalized.Text)
	normalized.TaskRunID = strings.TrimSpace(normalized.TaskRunID)
	if normalized.Now.IsZero() {
		normalized.Now = time.Now().UTC()
	} else {
		normalized.Now = normalized.Now.UTC()
	}
	return normalized
}

// Validate ensures the insert request can be persisted.
func (r SessionInputQueueInsert) Validate() error {
	normalized := r.Normalize()
	switch {
	case normalized.ID == "":
		return errors.New("store: session input queue id is required")
	case normalized.SessionID == "":
		return errors.New("store: session input queue session id is required")
	case normalized.Text == "":
		return errors.New("store: session input queue text is required")
	case normalized.QueueCap <= 0:
		return fmt.Errorf("store: session input queue cap must be positive: %d", normalized.QueueCap)
	}
	switch normalized.Mode {
	case SessionInputQueueModeQueue, SessionInputQueueModeSteer:
		return nil
	default:
		return fmt.Errorf("store: invalid session input queue mode %q", normalized.Mode)
	}
}

// SessionInputQueueStore persists operator busy-input entries.
type SessionInputQueueStore interface {
	EnqueueSessionInput(
		ctx context.Context,
		req SessionInputQueueInsert,
	) (SessionInputQueueEntry, int, error)
	StageSessionSteer(
		ctx context.Context,
		req SessionInputQueueInsert,
	) (SessionInputQueueEntry, error)
	ConsumeSessionSteer(
		ctx context.Context,
		sessionID string,
		now time.Time,
	) (SessionInputQueueEntry, bool, error)
	ClaimNextSessionInput(
		ctx context.Context,
		sessionID string,
		now time.Time,
	) (SessionInputQueueEntry, bool, error)
	PeekNextSessionInput(
		ctx context.Context,
		sessionID string,
	) (SessionInputQueueEntry, bool, error)
	GetSessionInputQueueEntry(
		ctx context.Context,
		sessionID string,
		entryID string,
	) (SessionInputQueueEntry, error)
	MarkSessionInputSent(
		ctx context.Context,
		sessionID string,
		entryID string,
		now time.Time,
	) error
	ReleaseSessionInput(ctx context.Context, sessionID string, entryID string, now time.Time) error
	MarkSessionInputFailed(
		ctx context.Context,
		sessionID string,
		entryID string,
		summary string,
		now time.Time,
	) error
	CancelSessionInput(
		ctx context.Context,
		sessionID string,
		entryID string,
		now time.Time,
	) (SessionInputQueueEntry, error)
	CancelPendingSessionInputs(
		ctx context.Context,
		sessionID string,
		generation int64,
		now time.Time,
	) (int, error)
	AdvanceSessionInputGeneration(ctx context.Context, sessionID string, now time.Time) (int64, error)
	CurrentSessionInputGeneration(ctx context.Context, sessionID string) (int64, error)
	SessionInputQueueSummary(ctx context.Context, sessionID string) (SessionInputQueueSummary, error)
}
