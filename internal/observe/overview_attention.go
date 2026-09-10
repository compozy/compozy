package observe

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/notifications"

	taskpkg "github.com/compozy/compozy/internal/task"
)

func (o *Observer) overviewAttention(ctx context.Context, query OverviewQuery) (OverviewAttention, error) {
	items, err := o.TaskAttentionItems(ctx, query)
	if err != nil {
		return OverviewAttention{}, err
	}
	attention := OverviewAttention{ByKind: map[string]int{}}
	if query.AcknowledgementProfileID != "" {
		ids := make([]string, 0, len(items))
		for _, item := range items {
			ids = append(ids, item.NotificationID)
		}
		snapshot, unread, captureErr := o.CaptureAttentionSnapshot(ctx, OverviewAttentionScope(query), ids)
		if captureErr != nil {
			return OverviewAttention{}, captureErr
		}
		attention.Snapshot = snapshot
		visible := make(map[string]bool, len(unread))
		for _, id := range unread {
			visible[id] = true
		}
		items = slices.DeleteFunc(items, func(item OverviewAttentionItem) bool { return !visible[item.NotificationID] })
	}
	for _, item := range items {
		attention.ByKind[item.Kind]++
	}
	attention.Total = len(items)
	attention.Items = items[:min(len(items), overviewAttentionItemCap)]
	return attention, nil
}

// OverviewAttentionScope binds Home's snapshot to its selected read population.
func OverviewAttentionScope(query OverviewQuery) notifications.AttentionScope {
	return notifications.AttentionScope{
		ProfileID: query.AcknowledgementProfileID, ActorKind: string(query.Actor.Kind), ActorID: query.Actor.Ref,
		Population: notifications.AttentionIdentity(
			"home", string(query.TaskScope), query.WorkspaceID, query.ReadScope.ProfileID,
			strconv.FormatBool(query.ReadScope.AllProfiles),
		),
	}
}

// TaskAttentionItems reuses inbox visibility and triage before building a complete attention snapshot.
func (o *Observer) TaskAttentionItems(ctx context.Context, query OverviewQuery) ([]OverviewAttentionItem, error) {
	queries := []TaskInboxQuery{
		{Lane: TaskInboxLaneApprovals},
		{Lane: TaskInboxLaneFailedRuns},
	}
	items := make([]OverviewAttentionItem, 0)
	seen := make(map[string]struct{})
	for _, inboxQuery := range queries {
		inboxQuery.ReadScope, inboxQuery.Scope, inboxQuery.WorkspaceID = query.ReadScope, query.TaskScope, query.WorkspaceID
		inboxQuery.Limit = 200
		for {
			page, err := o.QueryTaskInbox(ctx, inboxQuery, query.Actor)
			if err != nil {
				return nil, fmt.Errorf("observe: query attention inbox: %w", err)
			}
			for _, group := range page.Groups {
				if group.Lane == TaskInboxLaneArchived {
					continue
				}
				for _, source := range group.Items {
					var item OverviewAttentionItem
					switch group.Lane {
					case TaskInboxLaneApprovals:
						item = approvalAttentionItem(source)
					case TaskInboxLaneFailedRuns:
						item = failureAttentionItem(source)
					default:
						continue
					}
					item.WorkspaceID = source.Task.WorkspaceID
					occurrence := item.RunID
					if item.Kind == OverviewAttentionKindApproval {
						occurrence, err = o.approvalOccurrence(ctx, item.TaskID)
						if err != nil {
							return nil, err
						}
					}
					item.NotificationID = notifications.AttentionIdentity(
						"task", source.Task.WorkspaceID, source.Task.ID, item.Kind, occurrence,
					)
					items = appendAttentionItem(items, seen, item)
				}
			}
			if !page.HasMore {
				break
			}
			if page.NextCursor == "" || page.NextCursor == inboxQuery.Cursor {
				return nil, errors.New("observe: attention inbox did not advance")
			}
			inboxQuery.Cursor = page.NextCursor
		}
	}
	needsInput, err := o.overviewNeedsInput(ctx, query)
	if err != nil {
		return nil, err
	}
	for _, item := range needsInput {
		items = appendAttentionItem(items, seen, item)
	}

	slices.SortFunc(items, func(a, b OverviewAttentionItem) int {
		if order := b.OccurredAt.Compare(a.OccurredAt); order != 0 {
			return order
		}
		return cmp.Compare(a.NotificationID, b.NotificationID)
	})
	return items, nil
}

func appendAttentionItem(
	items []OverviewAttentionItem,
	seen map[string]struct{},
	item OverviewAttentionItem,
) []OverviewAttentionItem {
	key := item.TaskID
	if key == "" {
		key = item.Kind + ":" + item.Title
	}
	if _, exists := seen[key]; exists {
		return items
	}
	seen[key] = struct{}{}
	return append(items, item)
}

func approvalAttentionItem(item taskpkg.InboxItem) OverviewAttentionItem {
	return OverviewAttentionItem{
		Kind:       OverviewAttentionKindApproval,
		Title:      item.Task.Title,
		TaskID:     item.Task.ID,
		OccurredAt: item.LatestActivityAt,
		Actions:    []string{OverviewActionApprove, OverviewActionReject, OverviewActionOpen},
	}
}

func failureAttentionItem(item taskpkg.InboxItem) OverviewAttentionItem {
	attention := OverviewAttentionItem{
		Kind:       OverviewAttentionKindFailure,
		Title:      item.Task.Title,
		TaskID:     item.Task.ID,
		OccurredAt: item.LatestActivityAt,
		Actions:    []string{OverviewActionOpen},
	}
	if item.Run != nil {
		attention.RunID = item.Run.ID
		attention.SessionID = item.Run.SessionID
		attention.Detail = strings.TrimSpace(item.Run.Error)
		if item.Run.ID != "" {
			attention.Actions = []string{OverviewActionRetry, OverviewActionOpen}
		}
	}
	return attention
}

// Approval receipts follow approval transitions, not unrelated task audit activity.
func (o *Observer) approvalOccurrence(ctx context.Context, taskID string) (string, error) {
	var latest taskpkg.Event
	for _, kind := range []string{"task.published", "task.updated"} {
		events, err := o.registry.ListTaskEvents(ctx, taskpkg.EventQuery{TaskID: taskID, EventType: kind})
		if err != nil {
			return "", fmt.Errorf("observe: read approval occurrence: %w", err)
		}
		for _, event := range events {
			if kind == "task.updated" {
				var payload struct {
					ChangedFields []string `json:"changed_fields"`
				}
				if err := json.Unmarshal(event.Payload, &payload); err != nil {
					return "", fmt.Errorf("observe: decode approval update: %w", err)
				}
				if !slices.Contains(payload.ChangedFields, "approval_policy") {
					continue
				}
			}
			if latest.ID == "" || event.Timestamp.After(latest.Timestamp) ||
				(event.Timestamp.Equal(latest.Timestamp) && event.ID > latest.ID) {
				latest = event
			}
		}
	}
	return latest.ID, nil
}
