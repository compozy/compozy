package task

import (
	"context"
	"fmt"
	"strings"
)

// ListTaskCatalog returns one batched public catalog page after enforcing read authority.
func (m *Service) ListTaskCatalog(
	ctx context.Context,
	query CatalogQuery,
	actor ActorContext,
) (CatalogPage, error) {
	if err := requireReadAuthority(actor); err != nil {
		return CatalogPage{}, err
	}
	if !isTaskOperator(actor) {
		workspaceID := strings.TrimSpace(actor.Scope.WorkspaceID)
		if actor.Actor.Kind.Normalize() == ActorKindAgentSession || workspaceID != "" {
			switch query.Scope.Normalize() {
			case CatalogScopeGlobal:
				query.WorkspaceID = ""
			case CatalogScopeWorkspace:
				query.WorkspaceID = workspaceID
			default:
				query.Scope = CatalogScopeAll
				query.WorkspaceID = workspaceID
			}
		}
	}
	reader, ok := m.store.(CatalogReader)
	if !ok {
		return CatalogPage{}, fmt.Errorf("task: catalog reader is not configured")
	}
	return reader.ListTaskCatalog(ctx, query)
}

var _ CatalogManager = (*Service)(nil)
