package contract

import (
	"fmt"
	"strings"

	taskpkg "github.com/compozy/agh/internal/task"
)

// ValidateTaskListQuery validates transport-owned fields before workspace resolution or storage access.
func ValidateTaskListQuery(query TaskListQuery, path string) error {
	path = strings.TrimSpace(path)
	if err := validateTaskCatalogScope(query.Scope, path+".scope"); err != nil {
		return err
	}
	if err := validateTaskCatalogScopeBinding(query.Scope, query.Workspace, path); err != nil {
		return err
	}
	if err := validateOptionalTaskStatus(query.Status, path+".status"); err != nil {
		return err
	}
	if err := validateOptionalTaskPriority(query.Priority, path+".priority"); err != nil {
		return err
	}
	if query.ApprovalState.Normalize() != "" {
		if err := query.ApprovalState.Validate(path + ".approval_state"); err != nil {
			return err
		}
	}
	if err := validateOptionalTaskOwnerKind(query.OwnerKind, path+".owner_kind"); err != nil {
		return err
	}
	sortKey := query.Sort.Normalize()
	if sortKey != "" && sortKey != taskpkg.CatalogSortRecent && sortKey != taskpkg.CatalogSortPriority {
		return fmt.Errorf("%w: %s.sort has unsupported value %q", taskpkg.ErrValidation, path, sortKey)
	}
	return validateTaskCatalogLimit(query.Limit, path+".limit")
}

// ValidateTaskInboxQuery validates transport-owned fields before workspace resolution or storage access.
func ValidateTaskInboxQuery(query TaskInboxQuery, path string) error {
	path = strings.TrimSpace(path)
	if err := validateTaskCatalogScope(query.Scope, path+".scope"); err != nil {
		return err
	}
	if err := validateTaskCatalogScopeBinding(query.Scope, query.Workspace, path); err != nil {
		return err
	}
	if err := validateOptionalTaskOwnerKind(query.OwnerKind, path+".owner_kind"); err != nil {
		return err
	}
	if err := taskpkg.InboxLane(query.Lane).Validate(path + ".lane"); err != nil {
		return err
	}
	if err := validateOptionalTaskStatus(query.Status, path+".status"); err != nil {
		return err
	}
	if err := validateOptionalTaskPriority(query.Priority, path+".priority"); err != nil {
		return err
	}
	return validateTaskCatalogLimit(query.Limit, path+".limit")
}

func validateTaskCatalogScope(scope taskpkg.CatalogScope, path string) error {
	switch scope.Normalize() {
	case "", taskpkg.CatalogScopeAll, taskpkg.CatalogScopeGlobal, taskpkg.CatalogScopeWorkspace:
		return nil
	default:
		return fmt.Errorf("%w: %s has unsupported scope %q", taskpkg.ErrValidation, path, scope)
	}
}

func validateTaskCatalogScopeBinding(scope taskpkg.CatalogScope, workspace string, path string) error {
	normalizedScope := scope.Normalize()
	workspace = strings.TrimSpace(workspace)
	if normalizedScope == taskpkg.CatalogScopeGlobal && workspace != "" {
		return fmt.Errorf("%w: %s.workspace must be empty for global scope", taskpkg.ErrValidation, path)
	}
	if normalizedScope == taskpkg.CatalogScopeWorkspace && workspace == "" {
		return fmt.Errorf("%w: %s.workspace is required for workspace scope", taskpkg.ErrValidation, path)
	}
	return nil
}

func validateOptionalTaskStatus(status taskpkg.Status, path string) error {
	if status.Normalize() == "" {
		return nil
	}
	return status.Validate(path)
}

func validateOptionalTaskPriority(priority taskpkg.Priority, path string) error {
	if priority.Normalize() == "" {
		return nil
	}
	return priority.Validate(path)
}

func validateOptionalTaskOwnerKind(kind taskpkg.OwnerKind, path string) error {
	if kind.Normalize() == "" {
		return nil
	}
	return kind.Validate(path)
}

func validateTaskCatalogLimit(limit int, path string) error {
	if limit == 0 || (limit >= 1 && limit <= taskpkg.MaxCatalogLimit) {
		return nil
	}
	return fmt.Errorf(
		"%w: %s must be between 1 and %d: %d",
		taskpkg.ErrValidation,
		path,
		taskpkg.MaxCatalogLimit,
		limit,
	)
}
