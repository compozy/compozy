package task

import (
	"encoding/json"
	"fmt"

	"strings"
)

func validateJSONSize(payload json.RawMessage, maxBytes int, path string) error {
	if len(payload) == 0 {
		return nil
	}

	trimmed := bytesTrimSpace(payload)
	if !json.Valid(trimmed) {
		return fmt.Errorf("%w: %s must contain valid JSON", ErrValidation, path)
	}
	if len(trimmed) > maxBytes {
		return fmt.Errorf("%w: %s exceeds %d bytes", ErrPayloadTooLarge, path, maxBytes)
	}
	return nil
}

func validateBoundedCount(count int, maxCount int, label string) error {
	if count < 0 {
		return fmt.Errorf("%w: %s cannot be negative: %d", ErrValidation, label, count)
	}
	if count > maxCount {
		return fmt.Errorf("%w: %s exceeds %d: %d", ErrGraphLimitExceeded, label, maxCount, count)
	}
	return nil
}

func taskPatchHasMutableFields(p Patch) bool {
	return p.Title != nil ||
		p.Description != nil ||
		p.Priority != nil ||
		p.MaxAttempts != nil ||
		p.AutoEnqueueOnReady != nil ||
		p.ApprovalPolicy != nil ||
		p.Metadata != nil ||
		p.Owner != nil ||
		p.ClearOwner ||
		p.NetworkParticipation != nil
}

func validateTaskPatchTextAndSemantics(p Patch, path string) error {
	if p.Title != nil && strings.TrimSpace(*p.Title) == "" {
		return fmt.Errorf(
			"%w: %s is required when provided",
			ErrValidation,
			nestedPath(path, "title"),
		)
	}
	if p.Priority != nil {
		if err := p.Priority.Validate(nestedPath(path, "priority")); err != nil {
			return err
		}
	}
	if p.MaxAttempts != nil {
		if err := validateTaskMaxAttempts(*p.MaxAttempts, nestedPath(path, "max_attempts"), false); err != nil {
			return err
		}
	}
	if p.ApprovalPolicy != nil {
		if err := p.ApprovalPolicy.Validate(nestedPath(path, "approval_policy")); err != nil {
			return err
		}
	}
	return nil
}

func normalizePriorityOrDefault(priority Priority) Priority {
	normalized := priority.Normalize()
	if normalized == "" {
		return DefaultPriority
	}
	return normalized
}

func normalizeTaskMaxAttemptsOrDefault(maxAttempts int) int {
	if maxAttempts == 0 {
		return DefaultTaskMaxAttempts
	}
	return maxAttempts
}

func validateTaskMaxAttempts(maxAttempts int, path string, allowZeroDefault bool) error {
	if allowZeroDefault && maxAttempts == 0 {
		return nil
	}
	if maxAttempts <= 0 {
		return fmt.Errorf("%w: %s must be positive: %d", ErrValidation, path, maxAttempts)
	}
	if maxAttempts > MaxTaskMaxAttempts {
		return fmt.Errorf(
			"%w: %s must be <= %d: %d",
			ErrValidation,
			path,
			MaxTaskMaxAttempts,
			maxAttempts,
		)
	}
	return nil
}

func normalizeApprovalPolicyOrDefault(policy ApprovalPolicy) ApprovalPolicy {
	normalized := policy.Normalize()
	if normalized == "" {
		return DefaultApprovalPolicy
	}
	return normalized
}

func defaultApprovalStateForPolicy(policy ApprovalPolicy) ApprovalState {
	switch normalizeApprovalPolicyOrDefault(policy) {
	case ApprovalPolicyManual:
		return ApprovalStatePending
	default:
		return ApprovalStateNotRequired
	}
}

func normalizeApprovalStateOrDefault(policy ApprovalPolicy, state ApprovalState) ApprovalState {
	normalized := state.Normalize()
	if normalized == "" {
		return defaultApprovalStateForPolicy(policy)
	}
	return normalized
}

func nestedPath(path string, field string) string {
	trimmedPath := strings.TrimSpace(path)
	trimmedField := strings.TrimSpace(field)
	if trimmedPath == "" {
		return trimmedField
	}
	if trimmedField == "" {
		return trimmedPath
	}
	return trimmedPath + "." + trimmedField
}

func sameActorIdentity(left ActorIdentity, right ActorIdentity) bool {
	return left.Kind.Normalize() == right.Kind.Normalize() &&
		strings.TrimSpace(left.Ref) == strings.TrimSpace(right.Ref)
}

func sameOrigin(left Origin, right Origin) bool {
	return left.Kind.Normalize() == right.Kind.Normalize() &&
		strings.TrimSpace(left.Ref) == strings.TrimSpace(right.Ref)
}

func bytesTrimSpace(payload []byte) []byte {
	return []byte(strings.TrimSpace(string(payload)))
}
