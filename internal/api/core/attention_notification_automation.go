package core

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/observe/attention"
	"github.com/compozy/compozy/internal/redact"
	"github.com/compozy/compozy/internal/store"
)

const (
	automationJobFailureNotification     = "automation-job-failure"
	automationTriggerFailureNotification = "automation-trigger-failure"
)

func (h *BaseHandlers) automationFailureNotificationItems(
	ctx context.Context, reader attention.AutomationFailureReader, scope attention.Scope,
	labels map[string]string,
) ([]contract.AttentionNotificationPayload, error) {
	if h.Automation == nil {
		return nil, nil
	}
	// The bell reads all source profiles. The active profile owns the receipts.
	failures, err := reader.ListUnreadAutomationFailures(
		ctx, scope, store.ReadScope{AllProfiles: true}, slices.Sorted(maps.Keys(labels)),
	)
	if err != nil {
		return nil, fmt.Errorf("api: list automation failures for notifications: %w", err)
	}
	items := make([]contract.AttentionNotificationPayload, 0, len(failures))
	for _, run := range failures {
		kind, owner := automationJobFailureNotification, run.JobID
		if run.TriggerID != "" {
			kind, owner = automationTriggerFailureNotification, run.TriggerID
		}
		items = append(items, contract.AttentionNotificationPayload{
			ID: run.OccurrenceID, Kind: kind, SourceID: owner, RunID: run.RunID,
			WorkspaceID: run.WorkspaceID, Title: owner, Detail: "Run failed", OccurredAt: run.OccurredAt,
		})
	}
	return items, nil
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
