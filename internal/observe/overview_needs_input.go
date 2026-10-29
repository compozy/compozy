package observe

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/observe/attention"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func (o *Observer) overviewNeedsInput(ctx context.Context, query OverviewQuery) ([]OverviewAttentionItem, error) {
	ownerKind, ownerRef := taskpkg.OwnerKindForActor(query.Actor.Kind), strings.TrimSpace(query.Actor.Ref)
	if ownerKind == "" || ownerRef == "" {
		return nil, nil
	}
	// Home also exposes stored escalations; acknowledgement must not reconcile their lifecycle.
	summaries := make([]taskpkg.Summary, 0)
	for _, scope := range needsInputTaskScopes(query) {
		rows, err := o.registry.ListTasks(ctx, taskpkg.Query{
			ReadScope: query.ReadScope, Scope: scope.scope, WorkspaceID: scope.workspaceID,
			Status: taskpkg.TaskStatusNeedsAttention, OwnerKind: ownerKind, OwnerRef: ownerRef,
		})
		if err != nil {
			return nil, fmt.Errorf("observe: read escalations: %w", err)
		}
		summaries = append(summaries, rows...)
	}
	states, err := o.registry.ListTaskTriageStates(ctx, query.Actor)
	if err != nil {
		return nil, fmt.Errorf("observe: read escalation triage: %w", err)
	}
	triage := make(map[string]taskpkg.TriageState, len(states))
	for _, state := range states {
		triage[state.TaskID] = state
	}
	items := make([]OverviewAttentionItem, 0, len(summaries))
	for i := range summaries {
		summary := &summaries[i]
		at := summary.LastActivityAt
		if at.IsZero() {
			at = summary.UpdatedAt
		}
		state := triage[summary.ID]
		if state.Archived || (state.Dismissed && !at.After(state.LastSeenActivityAt)) {
			continue
		}
		item := OverviewAttentionItem{
			Kind: OverviewAttentionKindNeedsInput, Title: summary.Title, TaskID: summary.ID,
			WorkspaceID: summary.WorkspaceID, OccurredAt: at, Actions: []string{OverviewActionOpen},
		}
		occurrenceAt := summary.CreatedAt
		if summary.NeedsAttention != nil {
			item.Detail = strings.TrimSpace(summary.NeedsAttention.Reason)
			if !summary.NeedsAttention.At.IsZero() {
				occurrenceAt = summary.NeedsAttention.At
			}
		}
		if summary.ActiveRun != nil {
			item.RunID, item.SessionID = summary.ActiveRun.ID, summary.ActiveRun.SessionID
		}
		item.NotificationID = attention.Identity(
			"task", summary.WorkspaceID, summary.ID, item.Kind, occurrenceAt.UTC().Format(time.RFC3339Nano), item.RunID,
		)
		items = append(items, item)
	}
	return items, nil
}

type needsInputTaskScope struct {
	scope       taskpkg.Scope
	workspaceID string
}

// needsInputTaskScopes maps a catalog population onto stored task scopes: the all
// population is every task, or global plus the selected workspace, as the inbox reads it.
func needsInputTaskScopes(query OverviewQuery) []needsInputTaskScope {
	if len(query.WorkspaceIDs) > 0 {
		scopes := make([]needsInputTaskScope, 0, len(query.WorkspaceIDs))
		for _, id := range query.WorkspaceIDs {
			if id == "@user" {
				scopes = append(scopes, needsInputTaskScope{scope: taskpkg.ScopeGlobal})
			} else {
				scopes = append(scopes, needsInputTaskScope{scope: taskpkg.ScopeWorkspace, workspaceID: id})
			}
		}
		return scopes
	}
	if query.TaskScope.Normalize() != taskpkg.CatalogScopeAll {
		return []needsInputTaskScope{{scope: taskpkg.Scope(query.TaskScope), workspaceID: query.WorkspaceID}}
	}
	if query.WorkspaceID == "" {
		return []needsInputTaskScope{{}}
	}
	return []needsInputTaskScope{
		{scope: taskpkg.ScopeGlobal},
		{scope: taskpkg.ScopeWorkspace, workspaceID: query.WorkspaceID},
	}
}
