package extensionpkg

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	apicontract "github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/network/participation"
	networkusage "github.com/compozy/agh/internal/network/usage"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
)

func (h *HostAPIHandler) taskRunDetailPayloadFromView(
	ctx context.Context,
	view *taskpkg.RunDetailView,
) (apicontract.TaskRunDetailPayload, error) {
	if view == nil {
		return apicontract.TaskRunDetailPayload{}, nil
	}

	var task *apicontract.TaskReferencePayload
	if view.Task != nil {
		payload := taskReferencePayloadFromReference(*view.Task)
		task = &payload
	}

	networkPayload, err := h.taskRunNetworkPayload(ctx, view.Run)
	if err != nil {
		return apicontract.TaskRunDetailPayload{}, err
	}

	return apicontract.TaskRunDetailPayload{
		Run:     taskRunPayloadFromRun(&view.Run),
		Task:    task,
		Session: taskRunSessionPayloadFromSession(view.Session),
		Summary: taskRunOperationalSummaryPayloadFromSummary(view.Summary),
		Network: networkPayload,
	}, nil
}

func (h *HostAPIHandler) taskRunNetworkPayload(
	ctx context.Context,
	run taskpkg.Run,
) (*apicontract.TaskRunNetworkPayload, error) {
	spec := run.NetworkSpecSnapshot()
	if spec.Mode != participation.ModeLive {
		return nil, nil
	}
	workspaceID := strings.TrimSpace(spec.WorkspaceID)
	channelID := strings.TrimSpace(spec.ChannelID)
	if workspaceID == "" || channelID == "" {
		return nil, errors.New("extension: live task run requires workspace and channel identity")
	}
	usageStore, err := h.requireHostAPINetworkUsageStore()
	if err != nil {
		return nil, err
	}
	report, err := usageStore.GetNetworkUsage(ctx, store.NetworkUsageQuery{
		WorkspaceID: workspaceID,
		RunID:       strings.TrimSpace(run.ID),
	})
	if err != nil {
		return nil, fmt.Errorf("extension: load task run network usage: %w", err)
	}
	usage, err := networkusage.ResponseFromReport(workspaceID, report)
	if err != nil {
		return nil, fmt.Errorf("extension: convert task run network usage: %w", err)
	}
	return &apicontract.TaskRunNetworkPayload{
		Conversation: apicontract.TaskRunConversationRefPayload{
			WorkspaceID: workspaceID,
			Channel:     channelID,
			Surface:     store.NetworkSurfaceThread,
			ThreadID:    apicontract.TaskRunConversationThreadID,
			StreamURL: "/api/task-runs/" + url.PathEscape(strings.TrimSpace(run.ID)) +
				"/conversation/stream",
		},
		Usage: usage,
	}, nil
}
