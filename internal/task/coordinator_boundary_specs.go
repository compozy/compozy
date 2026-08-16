package task

import (
	"fmt"
	"strings"
)

// Normalize returns a normalized loop stop request.
func (s CoordinatorStopSpec) Normalize() CoordinatorStopSpec {
	normalized := s
	normalized.LoopRunID = strings.TrimSpace(normalized.LoopRunID)
	normalized.ReasonCode = strings.TrimSpace(normalized.ReasonCode)
	return normalized
}

// Validate reports whether a loop stop request names a child loop run.
func (s CoordinatorStopSpec) Validate(path string) error {
	if strings.TrimSpace(s.LoopRunID) == "" {
		return fmt.Errorf("%w: %s is required", ErrValidation, nestedPath(path, "loop_run_id"))
	}
	return validateCoordinatorStopSpecReferences(s, path)
}

// Normalize trims a parent-close mutation without changing its authored policy.
func (s CoordinatorParentCloseSpec) Normalize() CoordinatorParentCloseSpec {
	return CoordinatorParentCloseSpec{
		ParentLoopRunID: strings.TrimSpace(s.ParentLoopRunID),
		ChildLoopRunID:  strings.TrimSpace(s.ChildLoopRunID),
		Policy:          strings.TrimSpace(s.Policy),
		ParentStatus:    strings.TrimSpace(s.ParentStatus),
	}
}

// Validate rejects parent-close mutations that cannot prove ownership or policy.
func (s CoordinatorParentCloseSpec) Validate(path string) error {
	if strings.TrimSpace(s.ParentLoopRunID) == "" || strings.TrimSpace(s.ChildLoopRunID) == "" ||
		strings.TrimSpace(s.ParentStatus) == "" {
		return fmt.Errorf("%w: %s parent, child, and status are required", ErrValidation, path)
	}
	switch strings.TrimSpace(s.Policy) {
	case "terminate", "cancel":
		return nil
	default:
		return fmt.Errorf("%w: %s.policy is invalid", ErrValidation, path)
	}
}

// Normalize trims a committed lane-cancellation delivery.
func (s CoordinatorLaneCancelSpec) Normalize() CoordinatorLaneCancelSpec {
	normalized := s
	normalized.LoopRunID = strings.TrimSpace(normalized.LoopRunID)
	normalized.NodeID = strings.TrimSpace(normalized.NodeID)
	normalized.ReasonCode = strings.TrimSpace(normalized.ReasonCode)
	normalized.SessionIDs = make([]string, 0, len(s.SessionIDs))
	for _, sessionID := range s.SessionIDs {
		if trimmed := strings.TrimSpace(sessionID); trimmed != "" {
			normalized.SessionIDs = append(normalized.SessionIDs, trimmed)
		}
	}
	return normalized
}

// Validate rejects lane deliveries without exact Loop cell identity.
func (s CoordinatorLaneCancelSpec) Validate(path string) error {
	if strings.TrimSpace(s.LoopRunID) == "" || strings.TrimSpace(s.NodeID) == "" ||
		s.ItemIndex < 0 || strings.TrimSpace(s.ReasonCode) == "" {
		return fmt.Errorf("%w: %s has incomplete lane cancellation identity", ErrValidation, path)
	}
	return nil
}

// Normalize returns a normalized post-commit wake request.
func (s CoordinatorWakeSpec) Normalize() CoordinatorWakeSpec {
	return CoordinatorWakeSpec{
		LoopRunID:      strings.TrimSpace(s.LoopRunID),
		IdempotencyKey: strings.TrimSpace(s.IdempotencyKey),
	}
}

// Validate reports whether a post-commit wake has a stable target.
func (s CoordinatorWakeSpec) Validate(path string) error {
	if strings.TrimSpace(s.LoopRunID) == "" {
		return fmt.Errorf("%w: %s is required", ErrValidation, nestedPath(path, "loop_run_id"))
	}
	if strings.TrimSpace(s.IdempotencyKey) == "" {
		return fmt.Errorf("%w: %s is required", ErrValidation, nestedPath(path, "idempotency_key"))
	}
	return validateCoordinatorWakeSpecReferences(s, path)
}

// Normalize returns normalized coordinator terminal data.
func (t CoordinatorTerminal) Normalize() CoordinatorTerminal {
	normalized := t
	normalized.Status = strings.TrimSpace(normalized.Status)
	normalized.Cause = strings.TrimSpace(normalized.Cause)
	normalized.ReasonCode = strings.TrimSpace(normalized.ReasonCode)
	normalized.GateID = strings.TrimSpace(normalized.GateID)
	normalized.Details = normalizeRawJSON(normalized.Details)
	return normalized
}

// Validate reports whether coordinator terminal data has the minimum persisted shape.
func (t CoordinatorTerminal) Validate(path string) error {
	status := strings.TrimSpace(t.Status)
	if status == "" {
		return fmt.Errorf("%w: %s is required", ErrValidation, nestedPath(path, "status"))
	}
	if err := validateCoordinatorTerminalReferences(t, path); err != nil {
		return err
	}
	if err := ValidatePayloadSize(t.Details, nestedPath(path, "details")); err != nil {
		return err
	}
	return nil
}
