package task

import (
	"fmt"

	"strings"
)

// Validate reports whether the boot-recovery request contains one supported
// recovery action.
func (r RunBootRecovery) Validate(path string) error {
	if err := r.Action.Validate(nestedPath(path, "action")); err != nil {
		return err
	}
	if r.StopRequired && r.Action.Normalize() != RunBootRecoveryFail {
		return fmt.Errorf(
			"%w: %s is only valid for failed boot recovery",
			ErrValidation,
			nestedPath(path, "stop_required"),
		)
	}
	if err := ValidateReasonSize(r.Reason, nestedPath(path, "reason")); err != nil {
		return err
	}
	if err := ValidateReferenceSize(r.SessionState, nestedPath(path, "session_state")); err != nil {
		return err
	}
	if err := ValidateReferenceSize(r.Classification, nestedPath(path, "classification")); err != nil {
		return err
	}
	return ValidateDiagnosticSize(r.Detail, nestedPath(path, "detail"))
}

// Validate reports whether the audit event contains the canonical persisted shape.
func (e Event) Validate() error {
	if strings.TrimSpace(e.ID) == "" {
		return fmt.Errorf("%w: task_event.id is required", ErrValidation)
	}
	if err := ValidateReferenceSize(e.ID, "task_event.id"); err != nil {
		return err
	}
	if strings.TrimSpace(e.TaskID) == "" {
		return fmt.Errorf("%w: task_event.task_id is required", ErrValidation)
	}
	if err := ValidateReferenceSize(e.TaskID, "task_event.task_id"); err != nil {
		return err
	}
	if err := ValidateReferenceSize(e.RunID, "task_event.run_id"); err != nil {
		return err
	}
	if strings.TrimSpace(e.EventType) == "" {
		return fmt.Errorf("%w: task_event.event_type is required", ErrValidation)
	}
	if err := ValidateReferenceSize(e.EventType, "task_event.event_type"); err != nil {
		return err
	}
	if err := e.Actor.Validate("task_event.actor"); err != nil {
		return err
	}
	if err := e.Origin.Validate("task_event.origin"); err != nil {
		return err
	}
	if err := ValidatePayloadSize(e.Payload, "task_event.payload"); err != nil {
		return err
	}
	return nil
}

// Validate reports whether the persisted idempotency record contains the canonical shape.
func (r RunIdempotency) Validate() error {
	if strings.TrimSpace(r.IdempotencyKey) == "" {
		return fmt.Errorf("%w: task_run_idempotency.idempotency_key is required", ErrValidation)
	}
	if strings.TrimSpace(r.RunID) == "" {
		return fmt.Errorf("%w: task_run_idempotency.run_id is required", ErrValidation)
	}
	if err := r.Origin.Validate("task_run_idempotency.origin"); err != nil {
		return err
	}
	return nil
}

// Validate reports whether the durable task triage state contains the canonical shape.
func (t TriageState) Validate() error {
	if strings.TrimSpace(t.TaskID) == "" {
		return fmt.Errorf("%w: task_triage_state.task_id is required", ErrValidation)
	}
	if err := t.Actor.Validate("task_triage_state.actor"); err != nil {
		return err
	}
	if t.UpdatedAt.IsZero() {
		return fmt.Errorf("%w: task_triage_state.updated_at is required", ErrValidation)
	}
	return nil
}
