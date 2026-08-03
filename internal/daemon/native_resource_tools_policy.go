package daemon

import (
	"strings"

	"github.com/compozy/compozy/internal/resources"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func nativeApplyResourceScope(
	id toolspkg.ToolID,
	filter *resources.ResourceFilter,
	scope toolspkg.Scope,
) error {
	if filter == nil || scope.Operator || strings.TrimSpace(scope.WorkspaceID) == "" {
		return nil
	}
	workspaceScope := resources.ResourceScope{
		Kind: resources.ResourceScopeKindWorkspace,
		ID:   strings.TrimSpace(scope.WorkspaceID),
	}
	if filter.Scope == nil {
		filter.Scope = &workspaceScope
		return nil
	}
	if filter.Scope.Kind.Normalize() != resources.ResourceScopeKindWorkspace ||
		strings.TrimSpace(filter.Scope.ID) != workspaceScope.ID {
		return nativeScopeMismatchError(id, "scope")
	}
	filter.Scope.ID = workspaceScope.ID
	return nil
}

func nativeResourceRecordAllowed(scope toolspkg.Scope, record resources.RawRecord) bool {
	if scope.Operator || strings.TrimSpace(scope.WorkspaceID) == "" {
		return true
	}
	recordScope := record.Scope.Normalize()
	return recordScope.Kind == resources.ResourceScopeKindWorkspace &&
		recordScope.ID == strings.TrimSpace(scope.WorkspaceID)
}
