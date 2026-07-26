package daemon

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/network/participation"
	taskpkg "github.com/compozy/agh/internal/task"
	toolspkg "github.com/compozy/agh/internal/tools"
)

const (
	nativeProfileToolsDeletedKey = "deleted"
)

type taskExecutionProfileRefInput struct {
	TaskID string `json:"task_id"`
}

type taskExecutionProfileSetInput struct {
	TaskID  string                     `json:"task_id"`
	Profile *taskExecutionProfileInput `json:"profile"`
}

type taskExecutionProfileInput struct {
	TaskID               string                     `json:"task_id,omitempty"`
	Coordinator          taskpkg.CoordinatorProfile `json:"coordinator"`
	Worker               taskpkg.WorkerProfile      `json:"worker"`
	Review               taskpkg.ReviewProfile      `json:"review"`
	Participants         taskpkg.ParticipantPolicy  `json:"participants"`
	Sandbox              taskpkg.SandboxPolicy      `json:"sandbox"`
	Runtime              taskpkg.RuntimePolicy      `json:"runtime"`
	NetworkParticipation *participation.Request     `json:"network_participation,omitempty"`
}

func (n *daemonNativeTools) taskExecutionProfileGet(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	taskID, actor, err := decodeTaskExecutionProfileRef(req, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	profile, err := n.deps.Tasks.GetExecutionProfile(ctx, taskID, actor)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(map[string]any{"profile": profile}, fmt.Sprintf("profile %s", profile.TaskID))
}

func (n *daemonNativeTools) taskExecutionProfileSet(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input taskExecutionProfileSetInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	taskID, err := requiredNativeString(req.ToolID, coordinatorRuntimeTaskIDKey, input.TaskID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	if input.Profile == nil {
		return toolspkg.ToolResult{}, nativeRequiredInputError(req.ToolID, "profile")
	}
	actor, err := actorContextFromScope(scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	profile, err := input.Profile.profile(taskID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	stored, err := n.deps.Tasks.SetExecutionProfile(ctx, taskID, &profile, actor)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(map[string]any{"profile": stored}, fmt.Sprintf("updated profile %s", stored.TaskID))
}

func (n *daemonNativeTools) taskExecutionProfileDelete(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	taskID, actor, err := decodeTaskExecutionProfileRef(req, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	if err := n.deps.Tasks.DeleteExecutionProfile(ctx, taskID, actor); err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(
		map[string]any{coordinatorRuntimeTaskIDKey: taskID, nativeProfileToolsDeletedKey: true},
		fmt.Sprintf("deleted profile %s", taskID),
	)
}

func decodeTaskExecutionProfileRef(
	req toolspkg.CallRequest,
	scope toolspkg.Scope,
) (string, taskpkg.ActorContext, error) {
	var input taskExecutionProfileRefInput
	if err := decodeNativeInput(req, &input); err != nil {
		return "", taskpkg.ActorContext{}, err
	}
	taskID, err := requiredNativeString(req.ToolID, coordinatorRuntimeTaskIDKey, input.TaskID)
	if err != nil {
		return "", taskpkg.ActorContext{}, err
	}
	actor, err := actorContextFromScope(scope)
	if err != nil {
		return "", taskpkg.ActorContext{}, err
	}
	return taskID, actor, nil
}

func (i *taskExecutionProfileInput) profile(taskID string) (taskpkg.ExecutionProfile, error) {
	profileTaskID := strings.TrimSpace(i.TaskID)
	if profileTaskID != "" && profileTaskID != taskID {
		return taskpkg.ExecutionProfile{}, toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidInput,
			toolspkg.ToolIDTaskExecutionProfileSet,
			fmt.Sprintf("profile.task_id must match task_id %q", taskID),
			fmt.Errorf("%w: profile.task_id must match task_id %q", taskpkg.ErrValidation, taskID),
			toolspkg.ReasonSchemaInvalid,
		)
	}
	if profileTaskID == "" {
		profileTaskID = taskID
	}
	return taskpkg.ExecutionProfile{
		TaskID:               profileTaskID,
		Coordinator:          i.Coordinator,
		Worker:               i.Worker,
		Review:               i.Review,
		Participants:         i.Participants,
		Sandbox:              i.Sandbox,
		Runtime:              i.Runtime,
		NetworkParticipation: participation.CloneRequest(i.NetworkParticipation),
	}, nil
}
