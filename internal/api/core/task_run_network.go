package core

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
)

func (h *BaseHandlers) taskRunNetworkPayload(
	ctx context.Context,
	run taskpkg.Run,
) (*contract.TaskRunNetworkPayload, error) {
	conversation, err := taskRunConversationRefPayload(run)
	if err != nil || conversation == nil {
		return nil, err
	}
	workspaceID := conversation.WorkspaceID
	if h.NetworkUsage == nil {
		return nil, errors.New("api: network usage store is unavailable")
	}
	report, err := h.NetworkUsage.GetNetworkUsage(ctx, store.NetworkUsageQuery{
		WorkspaceID: workspaceID,
		RunID:       strings.TrimSpace(run.ID),
	})
	if err != nil {
		return nil, fmt.Errorf("api: load task run network usage: %w", err)
	}
	usage, err := NetworkUsageResponseFromReport(workspaceID, report)
	if err != nil {
		return nil, fmt.Errorf("api: convert task run network usage: %w", err)
	}
	return &contract.TaskRunNetworkPayload{Conversation: *conversation, Usage: usage}, nil
}

func taskRunConversationRefPayload(run taskpkg.Run) (*contract.TaskRunConversationRefPayload, error) {
	spec := run.NetworkSpecSnapshot()
	if spec.Mode != participation.ModeLive {
		return nil, nil
	}
	workspaceID := strings.TrimSpace(spec.WorkspaceID)
	channel := strings.TrimSpace(spec.ChannelID)
	if workspaceID == "" || channel == "" {
		return nil, errors.New("api: live task run requires workspace and channel identity")
	}
	return &contract.TaskRunConversationRefPayload{
		WorkspaceID: workspaceID,
		Channel:     channel,
		Surface:     store.NetworkSurfaceThread,
		ThreadID:    contract.TaskRunConversationThreadID,
		StreamURL: "/api/task-runs/" + url.PathEscape(strings.TrimSpace(run.ID)) +
			"/conversation/stream",
	}, nil
}
