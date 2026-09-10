package core

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/notifications"
	"github.com/compozy/compozy/internal/observe"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) bellNotificationItems(
	c *gin.Context, observer attentionNotificationObserver,
) ([]contract.AttentionNotificationPayload, error) {
	if h.Workspaces == nil || h.Loops == nil || h.Terminal == nil {
		return nil, errors.New("api: notification sources are unavailable")
	}
	ctx := c.Request.Context()
	workspaces, err := h.Workspaces.List(ctx)
	if err != nil {
		return nil, err
	}
	labels := make(map[string]string, len(workspaces))
	for _, workspace := range workspaces {
		labels[workspace.ID] = workspace.Name
	}
	items, err := h.sessionNotificationItems(ctx)
	if err != nil {
		return nil, err
	}
	actor, err := h.taskActorContext(c, taskActionOverview)
	if err != nil {
		return nil, err
	}
	query := observe.OverviewQuery{ReadScope: store.ReadScope{AllProfiles: true}, Actor: actor.Actor}
	workspaceIDs := []string{""}
	for _, workspace := range workspaces {
		workspaceIDs = append(workspaceIDs, workspace.ID)
	}
	for _, workspaceID := range workspaceIDs {
		query.WorkspaceID, query.TaskScope = workspaceID, taskpkg.CatalogScopeWorkspace
		if workspaceID == "" {
			query.TaskScope = taskpkg.CatalogScopeGlobal
		}
		tasks, err := observer.TaskAttentionItems(ctx, query)
		if err != nil {
			return nil, err
		}
		for _, item := range tasks {
			items = append(items, contract.AttentionNotificationPayload{
				ID: item.NotificationID, Kind: "task", SourceID: item.TaskID, WorkspaceID: workspaceID,
				Title: item.Title, Detail: item.Kind, OccurredAt: item.OccurredAt,
			})
		}
		if workspaceID == "" {
			continue
		}
		loops, err := h.loopNotificationItems(ctx, workspaceID)
		if err != nil {
			return nil, err
		}
		items = append(items, loops...)
		terminal, err := h.terminalNotificationItems(ctx, workspaceID)
		if err != nil {
			return nil, err
		}
		items = append(items, terminal...)
	}
	for index := range items {
		items[index].WorkspaceLabel = labels[items[index].WorkspaceID]
		if items[index].WorkspaceLabel == "" {
			items[index].WorkspaceLabel = items[index].WorkspaceID
			if items[index].WorkspaceID == "" {
				items[index].WorkspaceLabel = "Global"
			}
		}
	}
	return items, nil
}

func (h *BaseHandlers) sessionNotificationItems(ctx context.Context) ([]contract.AttentionNotificationPayload, error) {
	manager, ok := h.Sessions.(SessionPageManager)
	if !ok {
		return nil, errors.New("api: session notification source is unavailable")
	}
	query := session.ListQuery{
		ReadScope: store.ReadScope{AllProfiles: true}, AllWorkspaces: true, AttentionOnly: true,
		Sort: session.ListSortAttention, Limit: session.MaxListLimit,
	}
	items := make([]contract.AttentionNotificationPayload, 0)
	for {
		page, err := manager.ListPage(ctx, query)
		if err != nil {
			return nil, err
		}
		for _, info := range page.Sessions {
			badge := session.BadgeForInfo(info)
			class := session.ClassForBadge(badge)
			if class == session.AttentionNone {
				continue
			}
			at := info.UpdatedAt
			if info.AttentionChangedAt != nil {
				at = *info.AttentionChangedAt
			}
			title := strings.TrimSpace(info.Name)
			if title == "" {
				title = info.ID
			}
			reason := string(badge)
			for _, interaction := range info.PendingInteractions {
				if interaction.Status == "pending" && interaction.Title != "" {
					reason = interaction.Title
					break
				}
			}
			items = append(items, contract.AttentionNotificationPayload{
				ID: notifications.AttentionIdentity(
					"session", info.ProfileID, info.WorkspaceID, info.ID, string(badge),
					strconv.FormatInt(sessionNotificationRevision(info), 10),
				),
				Kind: "session", SourceID: info.ID, WorkspaceID: info.WorkspaceID, Title: title,
				Detail: reason, Badge: string(badge), AgentName: info.AgentName, OccurredAt: at,
				Finished: class == session.AttentionFinished,
			})
		}
		if !page.HasMore {
			return items, nil
		}
		if page.NextCursor == "" || page.NextCursor == query.Cursor {
			return nil, errors.New("api: session notification cursor did not advance")
		}
		query.Cursor = page.NextCursor
	}
}

func loopNotificationKey(workspace, run, node string, generation, item int) string {
	return notifications.AttentionIdentity(workspace, run, node, strconv.Itoa(generation), strconv.Itoa(item))
}

func (h *BaseHandlers) loopNotificationItems(
	ctx context.Context, workspace string,
) ([]contract.AttentionNotificationPayload, error) {
	items := make([]contract.AttentionNotificationPayload, 0)
	requests := make(map[string]bool)
	query := LoopRequestListQuery{State: "pending", Limit: 200}
	for {
		page, err := h.Loops.ListLoopRequests(ctx, workspace, query)
		if err != nil {
			return nil, err
		}
		for _, request := range page.Items {
			key := loopNotificationKey(
				workspace,
				request.LoopRunID,
				request.NodeID,
				request.Generation,
				request.ItemIndex,
			)
			requests[key] = true
			items = append(items, contract.AttentionNotificationPayload{
				ID: notifications.AttentionIdentity(
					"loop-request",
					key,
					request.OpenedAt.UTC().Format(time.RFC3339Nano),
				),
				Kind:        "loop-request",
				SourceID:    key,
				WorkspaceID: workspace,
				Title:       request.NodeID,
				RunID:       request.LoopRunID,
				NodeID:      request.NodeID,
				ItemIndex:   request.ItemIndex,
				Generation:  request.Generation,
				LoopName:    request.LoopName,
				RequestKind: request.Kind,
				OccurredAt:  request.OpenedAt,
			})
		}
		if page.NextCursor == "" {
			break
		}
		if page.NextCursor == query.Cursor {
			return nil, errors.New("api: loop request notification cursor did not advance")
		}
		query.Cursor = page.NextCursor
	}
	nodes, err := h.loopNodeNotificationItems(ctx, workspace, requests)
	if err != nil {
		return nil, err
	}
	items = append(items, nodes...)

	return items, nil
}

func (h *BaseHandlers) terminalNotificationItems(
	ctx context.Context, workspace string,
) ([]contract.AttentionNotificationPayload, error) {
	items := make([]contract.AttentionNotificationPayload, 0)

	requests, err := h.Terminal.InputRequests(ctx, workspace, store.ReadScope{AllProfiles: true}, "")
	if err != nil {
		return nil, err
	}
	for _, request := range requests {
		title := "Terminal input requested"
		if request.Redacted {
			title = "Private terminal input requested"
		}
		items = append(items, contract.AttentionNotificationPayload{
			ID: notifications.AttentionIdentity(
				"terminal-input",
				request.ProfileID,
				workspace,
				string(request.ID),
			),
			Kind:        "terminal-input",
			SourceID:    string(request.ID),
			WorkspaceID: workspace,
			TerminalID:  string(request.TerminalID),
			Title:       title,
			Detail:      request.Reason,
			AgentName:   request.Requester.ID,
			Redacted:    request.Redacted,
			OccurredAt:  request.RequestedAt,
		})
	}
	return items, nil
}

func (h *BaseHandlers) loopNodeNotificationItems(
	ctx context.Context, workspace string, requests map[string]bool,
) ([]contract.AttentionNotificationPayload, error) {
	items := make([]contract.AttentionNotificationPayload, 0)

	for _, state := range []string{"waiting", "attention"} {
		query := LoopNodeListQuery{State: state, Limit: 200}
		for {
			page, err := h.Loops.ListLoopNodes(ctx, workspace, query)
			if err != nil {
				return nil, err
			}
			for _, node := range page.Items {
				key := loopNotificationKey(workspace, node.LoopRunID, node.NodeID, node.Generation, node.ItemIndex)
				if requests[key] {
					continue
				}
				items = append(items, contract.AttentionNotificationPayload{
					ID: notifications.AttentionIdentity(
						"loop-node",
						key,
						state,
						node.StateAt.UTC().Format(time.RFC3339Nano),
					),
					Kind:        "loop-node",
					SourceID:    key,
					WorkspaceID: workspace,
					Title:       fmt.Sprintf("%s — %s", node.LoopName, node.NodeID),
					Detail:      state,
					RunID:       node.LoopRunID,
					NodeID:      node.NodeID,
					ItemIndex:   node.ItemIndex,
					Generation:  node.Generation,
					LoopName:    node.LoopName,
					OccurredAt:  node.StateAt,
				})
			}
			if page.NextCursor == "" {
				break
			}
			if page.NextCursor == query.Cursor {
				return nil, errors.New("api: loop node notification cursor did not advance")
			}
			query.Cursor = page.NextCursor
		}
	}
	return items, nil
}

// MarkSessionSeen advances the transport fence once without creating new attention.
// Normalize only that exact seen fence; subsequent source changes retain their own revision.
func sessionNotificationRevision(info *session.Info) int64 {
	revision := info.AttentionRevision
	if revision > 0 && info.LastSeenAt != nil && revision == info.LastSeenRevision &&
		info.LastSettledRevision < revision {
		return revision - 1
	}
	return revision
}
