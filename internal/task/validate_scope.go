package task

import (
	"encoding/json"
	"fmt"

	"strings"
)

// Validate reports whether the session reference returned by the bridge is usable.
func (r SessionRef) Validate() error {
	if strings.TrimSpace(r.SessionID) == "" {
		return fmt.Errorf("%w: session_ref.session_id is required", ErrValidation)
	}
	return nil
}

// ValidateScopeBinding enforces the canonical scope/workspace invariant shared by task-domain records.
func ValidateScopeBinding(
	scope Scope,
	workspaceBinding string,
	path string,
	workspaceField string,
) error {
	scopePath := nestedPath(path, "scope")
	if err := scope.Validate(scopePath); err != nil {
		return err
	}

	workspacePath := nestedPath(path, workspaceField)
	switch scope.Normalize() {
	case ScopeGlobal:
		if strings.TrimSpace(workspaceBinding) != "" {
			return fmt.Errorf(
				"%w: %s must be empty when %s is %q",
				ErrInvalidScopeBinding,
				workspacePath,
				scopePath,
				ScopeGlobal,
			)
		}
	case ScopeWorkspace:
		if strings.TrimSpace(workspaceBinding) == "" {
			return fmt.Errorf(
				"%w: %s is required when %s is %q",
				ErrInvalidScopeBinding,
				workspacePath,
				scopePath,
				ScopeWorkspace,
			)
		}
	}

	return nil
}

// ValidateImmutableTaskFields reports whether an update attempted to change immutable task fields.
func ValidateImmutableTaskFields(current Task, next Task) error {
	if !sameActorIdentity(current.CreatedBy, next.CreatedBy) {
		return fmt.Errorf("%w: %s", ErrImmutableField, TaskFieldCreatedBy)
	}
	if !sameOrigin(current.Origin, next.Origin) {
		return fmt.Errorf("%w: %s", ErrImmutableField, TaskFieldOrigin)
	}
	if current.Scope.Normalize() != next.Scope.Normalize() {
		return fmt.Errorf("%w: %s", ErrImmutableField, TaskFieldScope)
	}
	if strings.TrimSpace(current.WorkspaceID) != strings.TrimSpace(next.WorkspaceID) {
		return fmt.Errorf("%w: %s", ErrImmutableField, TaskFieldWorkspaceID)
	}
	if strings.TrimSpace(current.ParentTaskID) != strings.TrimSpace(next.ParentTaskID) {
		return fmt.Errorf("%w: %s", ErrImmutableField, TaskFieldParentTaskID)
	}
	return nil
}

// ValidateMetadataSize reports whether metadata JSON respects the shared 16 KiB guardrail.
func ValidateMetadataSize(payload json.RawMessage, path string) error {
	return validateJSONSize(payload, MaxMetadataBytes, path)
}

// ValidatePayloadSize reports whether a persisted JSON payload respects the shared 64 KiB guardrail.
func ValidatePayloadSize(payload json.RawMessage, path string) error {
	return validateJSONSize(payload, MaxPayloadBytes, path)
}

// ValidateResultSize reports whether a persisted run result respects the shared 64 KiB guardrail.
func ValidateResultSize(payload json.RawMessage, path string) error {
	return validateJSONSize(payload, MaxResultBytes, path)
}

// ValidateHierarchyDepth reports whether the supplied task depth stays within the bounded hierarchy limit.
func ValidateHierarchyDepth(depth int) error {
	return validateBoundedCount(depth, MaxHierarchyDepth, "hierarchy depth")
}

// ValidateDependencyCount reports whether the supplied dependency count stays within the bounded edge limit.
func ValidateDependencyCount(count int) error {
	return validateBoundedCount(count, MaxDependencyCount, "dependency count")
}

// ValidateDirectChildCount reports whether the supplied direct-child count stays within the bounded fan-out limit.
func ValidateDirectChildCount(count int) error {
	return validateBoundedCount(count, MaxDirectChildren, "direct child count")
}

// ValidateApprovalSemantics reports whether one approval policy and state pair is internally consistent.
func ValidateApprovalSemantics(policy ApprovalPolicy, state ApprovalState, path string) error {
	normalizedPolicy := normalizeApprovalPolicyOrDefault(policy)
	normalizedState := normalizeApprovalStateOrDefault(normalizedPolicy, state)

	if err := normalizedPolicy.Validate(nestedPath(path, "approval_policy")); err != nil {
		return err
	}
	if err := normalizedState.Validate(nestedPath(path, "approval_state")); err != nil {
		return err
	}

	switch normalizedPolicy {
	case ApprovalPolicyNone:
		if normalizedState != ApprovalStateNotRequired {
			return fmt.Errorf(
				"%w: %s must be %q when %s is %q",
				ErrValidation,
				nestedPath(path, "approval_state"),
				ApprovalStateNotRequired,
				nestedPath(path, "approval_policy"),
				ApprovalPolicyNone,
			)
		}
	case ApprovalPolicyManual:
		switch normalizedState {
		case ApprovalStatePending, ApprovalStateApproved, ApprovalStateRejected:
			return nil
		default:
			return fmt.Errorf(
				"%w: %s must be one of %q, %q, or %q when %s is %q",
				ErrValidation,
				nestedPath(path, "approval_state"),
				ApprovalStatePending,
				ApprovalStateApproved,
				ApprovalStateRejected,
				nestedPath(path, "approval_policy"),
				ApprovalPolicyManual,
			)
		}
	}

	return nil
}
