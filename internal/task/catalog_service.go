package task

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/workspaceaccess"
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
	if actor.ReadScope != (store.ReadScope{}) {
		query.ReadScope = actor.ReadScope
	}
	aggregateGranted := false
	if !isTaskOperator(actor) && query.AllWorkspaces {
		if m.workspaceAccess == nil {
			return CatalogPage{}, ErrPermissionDenied
		}
		decision, err := m.workspaceAccess.Authorize(ctx, workspaceaccess.Request{
			Actor:  workspaceaccess.ActorRef{Kind: workspaceaccess.ActorAgentSession, SessionID: actor.Scope.SessionID, WorkspaceID: actor.Scope.WorkspaceID},
			Target: workspaceaccess.TargetRef{Kind: workspaceaccess.TargetAggregate, Surface: "tasks", ProfileID: query.ReadScope.ProfileID, AllProfiles: query.ReadScope.AllProfiles}, ReadOnly: true, Seam: workspaceaccess.SeamCatalog,
		})
		if err != nil {
			return CatalogPage{}, err
		}
		if !decision.Allowed || decision.Source != workspaceaccess.SourceAgentGrant {
			return CatalogPage{}, ErrPermissionDenied
		}
		aggregateGranted = true
	}
	if !isTaskOperator(actor) && !aggregateGranted {
		workspaceID := strings.TrimSpace(actor.Scope.WorkspaceID)
		if target := strings.TrimSpace(
			query.WorkspaceID,
		); target != "" &&
			query.Scope.Normalize() != CatalogScopeGlobal {
			if err := m.authorizeTaskScope(ctx, actor, ScopeWorkspace, target); err != nil {
				return CatalogPage{}, err
			}
			workspaceID = target
		}
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
