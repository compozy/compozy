package activities

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/workflow"
)

// Duration wraps time.Duration to provide human-readable JSON marshaling.
// Instead of marshaling as nanoseconds (unfriendly), it marshals as duration strings like "30s", "1m", "500ms".
type Duration time.Duration

// MarshalText implements encoding.TextMarshaler for human-readable duration strings.
func (d Duration) MarshalText() ([]byte, error) {
	return []byte(time.Duration(d).String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler to parse duration strings like "30s".
func (d *Duration) UnmarshalText(text []byte) error {
	parsed, err := time.ParseDuration(string(text))
	if err != nil {
		return fmt.Errorf("invalid duration format: %w", err)
	}
	*d = Duration(parsed)
	return nil
}

// MarshalJSON implements json.Marshaler for JSON encoding.
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

// UnmarshalJSON implements json.Unmarshaler for JSON decoding.
func (d *Duration) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration format: %w", err)
	}
	*d = Duration(parsed)
	return nil
}

// ToDuration converts Duration to time.Duration.
func (d Duration) ToDuration() time.Duration {
	return time.Duration(d)
}

const TriggerLabel = "TriggerWorkflow"

type TriggerInput struct {
	WorkflowID     string      `json:"workflow_id"`
	WorkflowExecID core.ID     `json:"workflow_exec_id"`
	Input          *core.Input `json:"input"`
	InitialTaskID  string
	// ErrorHandlerTimeout bounds error-handling activities; zero uses worker defaults.
	ErrorHandlerTimeout time.Duration `json:"error_handler_timeout"`
	// ErrorHandlerMaxRetries limits retries for error-handling activities; zero uses worker defaults.
	ErrorHandlerMaxRetries int `json:"error_handler_max_retries"`
}

type Trigger struct {
	workflows    []*workflow.Config
	workflowRepo workflow.Repository
}

func NewTrigger(workflows []*workflow.Config, workflowRepo workflow.Repository) *Trigger {
	return &Trigger{
		workflows:    workflows,
		workflowRepo: workflowRepo,
	}
}

func (a *Trigger) Run(ctx context.Context, input *TriggerInput) (*workflow.State, error) {
	repo := a.workflowRepo
	wfState := workflow.NewState(
		input.WorkflowID,
		input.WorkflowExecID,
		input.Input,
	)
	if err := repo.UpsertState(ctx, wfState); err != nil {
		return nil, err
	}
	return wfState, nil
}
