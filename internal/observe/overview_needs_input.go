package observe

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/notifications"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func (o *Observer) overviewNeedsInput(ctx context.Context, query OverviewQuery) ([]OverviewAttentionItem, error) {
	ownerKind, ownerRef := taskpkg.OwnerKindForActor(query.Actor.Kind), strings.TrimSpace(query.Actor.Ref)
	if ownerKind == "" || ownerRef == "" {
		return nil, nil
	}
	// Home also exposes stored escalations; acknowledgement must not reconcile their lifecycle.
	summaries, err := o.registry.ListTasks(ctx, taskpkg.Query{
		ReadScope: query.ReadScope, Scope: taskpkg.Scope(query.TaskScope), WorkspaceID: query.WorkspaceID,
		Status: taskpkg.TaskStatusNeedsAttention, OwnerKind: ownerKind, OwnerRef: ownerRef,
	})
	if err != nil {
		return nil, fmt.Errorf("observe: read escalations: %w", err)
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
	for _, summary := range summaries {
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
		if summary.NeedsAttention != nil {
			item.Detail = strings.TrimSpace(summary.NeedsAttention.Reason)
		}
		if summary.ActiveRun != nil {
			item.RunID, item.SessionID = summary.ActiveRun.ID, summary.ActiveRun.SessionID
		}
		item.NotificationID = notifications.AttentionIdentity(
			"task", summary.WorkspaceID, summary.ID, item.Kind, at.UTC().Format(time.RFC3339Nano), item.RunID,
		)
		items = append(items, item)
	}
	return items, nil
}
