package participation

import (
	"fmt"
	"strings"
)

// OwnerKey returns the canonical budget-owner identity for one execution.
func OwnerKey(owner OwnerRef) string {
	return string(owner.Kind) + ":" + strings.TrimSpace(owner.ID)
}

// OwnerRefFromKey reconstructs a public owner reference at an internal owner-key boundary.
func OwnerRefFromKey(workspaceID, value string) (OwnerRef, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	value = strings.TrimSpace(value)
	if err := ValidateOwnerKey(value); err != nil {
		return OwnerRef{}, err
	}
	parts := strings.SplitN(value, ":", 2)
	owner := OwnerRef{
		WorkspaceID: workspaceID,
		Kind:        OwnerKind(parts[0]),
		ID:          parts[1],
	}
	if err := ValidateOwner(owner); err != nil {
		return OwnerRef{}, err
	}
	return owner, nil
}

// ValidateOwner verifies that an execution owner has a supported kind and stable identity.
func ValidateOwner(owner OwnerRef) error {
	owner.WorkspaceID = strings.TrimSpace(owner.WorkspaceID)
	owner.Kind = OwnerKind(strings.TrimSpace(string(owner.Kind)))
	owner.ID = strings.TrimSpace(owner.ID)
	if owner.WorkspaceID == "" {
		return fmt.Errorf("network participation: owner workspace_id is required")
	}
	return validateOwnerIdentity(owner)
}

func validateOwnerIdentity(owner OwnerRef) error {
	switch owner.Kind {
	case OwnerKindSession, OwnerKindTaskRun, OwnerKindLoopRun, OwnerKindAutomationRun:
	default:
		return fmt.Errorf("network participation: unsupported owner kind %q", owner.Kind)
	}
	if owner.ID == "" {
		return fmt.Errorf("network participation: %s owner id is required", owner.Kind)
	}
	return nil
}

// ValidateOwnerKey verifies a canonical kind-prefixed budget-owner identity.
func ValidateOwnerKey(value string) error {
	parts := strings.SplitN(strings.TrimSpace(value), ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("network participation: owner key must use kind:id form")
	}
	owner := OwnerRef{Kind: OwnerKind(parts[0]), ID: strings.TrimSpace(parts[1])}
	if err := validateOwnerIdentity(owner); err != nil {
		return err
	}
	if OwnerKey(owner) != strings.TrimSpace(value) {
		return fmt.Errorf("network participation: owner key is not canonical")
	}
	return nil
}
