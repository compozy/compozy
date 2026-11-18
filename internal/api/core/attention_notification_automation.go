package core

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	automationpkg "github.com/compozy/compozy/internal/automation"
	modelpkg "github.com/compozy/compozy/internal/automation/model"
	"github.com/compozy/compozy/internal/observe/attention"
	"github.com/compozy/compozy/internal/redact"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

const (
	automationJobFailureNotification     = "automation-job-failure"
	automationTriggerFailureNotification = "automation-trigger-failure"
)

func (h *BaseHandlers) automationFailureNotificationItems(
	ctx context.Context, labels map[string]string, profileID string,
) ([]contract.AttentionNotificationPayload, error) {
	if h.Automation == nil {
		return nil, nil
	}
	query := automationpkg.RunQuery{
		AllWorkspaces: true,
		WorkspaceIDs:  append(slices.Sorted(maps.Keys(labels)), workspaceaccess.UserSelector),
		ReadScope:     store.ReadScope{ProfileID: profileID},
		Status:        automationpkg.RunFailed,
		Limit:         automationpkg.MaxListLimit,
	}
	var items []contract.AttentionNotificationPayload
	for {
		runs, err := h.Automation.ListRuns(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("api: list automation failures for notifications: %w", err)
		}
		for _, run := range runs {
			item, ok := automationFailureNotificationItem(run)
			if ok {
				items = append(items, item)
			}
		}
		if len(runs) < query.Limit {
			return items, nil
		}
		cursor, err := modelpkg.EncodeRunCursor(query, runs[len(runs)-1])
		if err != nil {
			return nil, fmt.Errorf("api: encode automation failure cursor: %w", err)
		}
		query.Cursor = cursor
	}
}

func automationFailureNotificationItem(run automationpkg.Run) (contract.AttentionNotificationPayload, bool) {
	at := run.EndedAt
	if at == nil {
		at = run.StartedAt
	}
	if at == nil {
		return contract.AttentionNotificationPayload{}, false
	}
	kind, owner := automationJobFailureNotification, run.JobID
	if run.TriggerID != "" {
		kind, owner = automationTriggerFailureNotification, run.TriggerID
	}
	workspaceID := ""
	if run.WorkspaceID != nil {
		workspaceID = *run.WorkspaceID
	}
	return contract.AttentionNotificationPayload{
		ID:          attention.Identity(kind, workspaceID, run.ProfileID, run.ID),
		Kind:        kind,
		SourceID:    owner,
		RunID:       run.ID,
		WorkspaceID: workspaceID,
		Title:       owner,
		Detail:      "Run failed",
		OccurredAt:  *at,
	}, true
}

func (h *BaseHandlers) applyAutomationFailureTitles(
	ctx context.Context,
	items []contract.AttentionNotificationPayload,
) {
	type source struct{ kind, id string }
	titles := make(map[source]string)
	for i := range items {
		item := &items[i]
		key := source{kind: item.Kind, id: item.SourceID}
		title, seen := titles[key]
		if !seen {
			switch item.Kind {
			case automationJobFailureNotification:
				job, err := h.Automation.GetJob(ctx, item.SourceID)
				if err == nil {
					title = job.Name
				}
			case automationTriggerFailureNotification:
				trigger, err := h.Automation.GetTrigger(ctx, item.SourceID)
				if err == nil {
					title = trigger.Name
				}
			default:
				continue
			}
			title = strings.TrimSpace(redact.String(title))
			titles[key] = title
		}
		if title != "" {
			item.Title = title
		}
	}
}
