package task

import (
	"context"
	"errors"
	"fmt"
	"slices"
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
	actor.readOnly = true
	if err := requireReadAuthority(actor); err != nil {
		return CatalogPage{}, err
	}
	if actor.ReadScope != (store.ReadScope{}) {
		query.ReadScope = actor.ReadScope
	}
	aggregateGranted, err := m.authorizeTaskCatalogAggregate(ctx, query, actor)
	if err != nil {
		return CatalogPage{}, err
	}
	if !isTaskOperator(actor) && !aggregateGranted {
		if err := m.boundTaskCatalogQuery(ctx, &query, actor); err != nil {
			return CatalogPage{}, err
		}
	}
	reader, ok := m.store.(CatalogReader)
	if !ok {
		return CatalogPage{}, fmt.Errorf("task: catalog reader is not configured")
	}
	page, err := reader.ListTaskCatalog(ctx, query)
	if err != nil {
		return CatalogPage{}, err
	}
	if !canBypassTaskReadAuthorization(actor) {
		for i := range page.Tasks {
			if err := m.filterCatalogRunRelations(ctx, actor, &page.Tasks[i]); err != nil {
				return CatalogPage{}, err
			}
		}
	}
	return page, nil
}

var _ CatalogManager = (*Service)(nil)

func (m *Service) filterCatalogRunRelations(ctx context.Context, actor ActorContext, summary *Summary) error {
	record := taskRecordFromSummary(summary)
	runIDs := []string{summary.CurrentRunID}
	if summary.ActiveRun != nil {
		runIDs = append(runIDs, summary.ActiveRun.ID)
	}
	for _, runID := range slices.Compact(runIDs) {
		if strings.TrimSpace(runID) == "" {
			continue
		}
		run, err := m.store.GetTaskRun(ctx, runID)
		if err == nil {
			err = m.runReadAuthorizer.AuthorizeRunRead(ctx, actor, run, &record)
		}
		if err == nil {
			continue
		}
		if !errors.Is(err, ErrTaskRunNotFound) && !errors.Is(err, ErrPermissionDenied) {
			return err
		}
		if summary.CurrentRunID == runID {
			summary.CurrentRunID = ""
		}
		if summary.ActiveRun != nil && summary.ActiveRun.ID == runID {
			summary.ActiveRun = nil
		}
	}
	return nil
}

// authorizeTaskCatalogAggregate admits a non-operator All projects read through the
// workspace-access policy: the unrestricted grant or the session's permission mode.
func (m *Service) authorizeTaskCatalogAggregate(
	ctx context.Context,
	query CatalogQuery,
	actor ActorContext,
) (bool, error) {
	if isTaskOperator(actor) || !query.AllWorkspaces {
		return false, nil
	}
	if m.workspaceAccess == nil {
		return false, ErrPermissionDenied
	}
	decision, err := m.workspaceAccess.Authorize(ctx, workspaceaccess.Request{
		Actor: workspaceaccess.ActorRef{
			Kind:        workspaceaccess.ActorAgentSession,
			SessionID:   actor.Scope.SessionID,
			WorkspaceID: actor.Scope.WorkspaceID,
		},
		Target: workspaceaccess.TargetRef{
			Kind:        workspaceaccess.TargetAggregate,
			Surface:     "tasks",
			ProfileID:   query.ReadScope.ProfileID,
			AllProfiles: query.ReadScope.AllProfiles,
		},
		ReadOnly: true,
		Seam:     workspaceaccess.SeamCatalog,
	})
	if err != nil {
		return false, err
	}
	if !decision.Allowed {
		return false, ErrPermissionDenied
	}
	return true, nil
}

// boundTaskCatalogQuery narrows a non-operator read to the actor's authorized workspace or User rows.
func (m *Service) boundTaskCatalogQuery(ctx context.Context, query *CatalogQuery, actor ActorContext) error {
	workspaceID := strings.TrimSpace(actor.Scope.WorkspaceID)
	if target := strings.TrimSpace(query.WorkspaceID); target != "" && query.Scope.Normalize() != CatalogScopeGlobal {
		if err := m.authorizeTaskScope(ctx, actor, ScopeWorkspace, target); err != nil {
			return err
		}
		workspaceID = target
	}
	agentSession := actor.Actor.Kind.Normalize() == ActorKindAgentSession
	switch {
	case workspaceID == "" && (agentSession || actor.Scope.UserProfileID != ""):
		// A User-scope actor owns only its profile's User rows; an empty workspace never means
		// every project (USW BR-6, BR-7).
		query.Scope = CatalogScopeGlobal
		query.WorkspaceID = ""
		// Keep the authorized read lens (a granted session may read another profile); fall back
		// to the actor's own User profile only when no profile was selected.
		if profileID := strings.TrimSpace(actor.Scope.UserProfileID); profileID != "" &&
			query.ReadScope == (store.ReadScope{}) {
			query.ReadScope = store.ReadScope{ProfileID: profileID}
		}
	case agentSession || workspaceID != "":
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
	return nil
}
