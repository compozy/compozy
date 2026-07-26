package task

import (
	"encoding/json"

	"time"

	diagnosticcontract "github.com/compozy/agh/internal/diagnosticcontract"
	diagnosticitems "github.com/compozy/agh/internal/diagnostics"
)

func forceRunDiagnosticError(
	code string,
	title string,
	message string,
	severity string,
	suggestedCommand string,
	evidence map[string]any,
	cause error,
) error {
	item := diagnosticitems.NewItem(
		"task.force."+code,
		code,
		diagnosticcontract.CategoryTask,
		title,
		message,
		severity,
		diagnosticcontract.FreshnessLive,
		diagnosticitems.WithDocURL(forceOpsDocURL),
		diagnosticitems.WithSuggestedCommand(suggestedCommand),
		diagnosticitems.WithEvidence(evidence),
	)
	return diagnosticitems.NewStructuredError(item, cause)
}

func optionalPayloadTime(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	normalized := value.UTC()
	return &normalized
}

type operatorForcedFailPayload struct {
	Manual               bool            `json:"manual"`
	ActorKind            ActorKind       `json:"actor_kind"`
	ActorID              string          `json:"actor_id"`
	PreviousStatus       RunStatus       `json:"previous_status"`
	Status               RunStatus       `json:"status"`
	TaskStatus           Status          `json:"task_status"`
	Reason               string          `json:"reason"`
	SessionID            string          `json:"session_id,omitempty"`
	QueueGeneration      int64           `json:"queue_generation,omitempty"`
	CanceledQueuedInputs int             `json:"canceled_queued_inputs,omitempty"`
	Metadata             json.RawMessage `json:"metadata,omitempty"`
}

type operatorRetryPayload struct {
	Manual      bool      `json:"manual"`
	ActorKind   ActorKind `json:"actor_kind"`
	ActorID     string    `json:"actor_id"`
	SourceRunID string    `json:"source_run_id"`
	NewRunID    string    `json:"new_run_id"`
	TaskStatus  Status    `json:"task_status"`
}

type recoveredFromAttentionPayload struct {
	Manual         bool      `json:"manual"`
	ActorKind      ActorKind `json:"actor_kind"`
	ActorID        string    `json:"actor_id"`
	SourceRunID    string    `json:"source_run_id"`
	NewRunID       string    `json:"new_run_id"`
	PreviousStatus RunStatus `json:"previous_status"`
	Status         RunStatus `json:"status"`
	TaskStatus     Status    `json:"task_status"`
	Reason         string    `json:"reason,omitempty"`
}
