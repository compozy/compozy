package daemon

import (
	"context"

	"fmt"

	"strings"
	"time"

	core "github.com/compozy/agh/internal/api/core"

	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func (n *daemonNativeTools) taskRead(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input taskReadInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	actor, err := actorContextFromScope(scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	view, err := n.deps.Tasks.GetTask(ctx, input.TaskID, actor)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	payload := core.TaskDetailPayloadFromView(view)
	return structuredResult(map[string]any{nativeToolsTaskKey: payload}, payload.Summary.Title)
}

func (n *daemonNativeTools) taskCreate(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input taskCreateInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	actor, err := actorContextFromScope(scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	created, err := n.deps.Tasks.CreateTask(ctx, input.spec(scope), actor)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(map[string]any{nativeToolsTaskKey: created}, created.Title)
}

func (n *daemonNativeTools) taskChildCreate(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input taskChildCreateInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	actor, err := actorContextFromScope(scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	created, err := n.deps.Tasks.CreateChildTask(ctx, input.ParentTaskID, input.spec(scope), actor)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(map[string]any{nativeToolsTaskKey: created}, created.Title)
}

func (n *daemonNativeTools) taskUpdate(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input taskUpdateInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	actor, err := actorContextFromScope(scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	updated, err := n.deps.Tasks.UpdateTask(ctx, input.TaskID, input.patch(), actor)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(map[string]any{nativeToolsTaskKey: updated}, updated.Title)
}

func (n *daemonNativeTools) taskCancel(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input taskCancelInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	actor, err := actorContextFromScope(scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	canceled, err := n.deps.Tasks.CancelTask(ctx, input.TaskID, input.cancel(), actor)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(map[string]any{nativeToolsTaskKey: canceled}, canceled.Title)
}

func (n *daemonNativeTools) taskBlock(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input taskBlockInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	actor, runID, claimToken, err := n.nativeTaskBlockActor(ctx, req.ToolID, scope, input.TaskID, input.RunID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	block, err := n.deps.Tasks.BlockTask(ctx, taskpkg.BlockRequest{
		TaskID:     strings.TrimSpace(input.TaskID),
		Kind:       taskpkg.BlockKind(strings.TrimSpace(input.Kind)),
		Reason:     strings.TrimSpace(input.Reason),
		Details:    cloneJSON(input.Details),
		ExpiresAt:  nativeTaskBlockExpiresAt(input.ExpiresAt),
		RunID:      runID,
		ClaimToken: claimToken,
	}, actor)
	if err != nil {
		return toolspkg.ToolResult{}, nativeTaskToolError(req.ToolID, err)
	}
	return structuredResult(
		map[string]any{daemonBlockKey: core.TaskBlockPayloadFromBlock(block)},
		fmt.Sprintf("blocked %s", block.TaskID),
	)
}

func (n *daemonNativeTools) taskUnblock(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input taskUnblockInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	actor, err := n.nativeTaskClearActor(ctx, req.ToolID, scope, input.TaskID, input.RunID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	block, err := n.deps.Tasks.ClearTaskBlock(
		ctx,
		strings.TrimSpace(input.TaskID),
		strings.TrimSpace(input.BlockID),
		strings.TrimSpace(input.Note),
		actor,
	)
	if err != nil {
		return toolspkg.ToolResult{}, nativeTaskToolError(req.ToolID, err)
	}
	return structuredResult(
		map[string]any{daemonBlockKey: core.TaskBlockPayloadFromBlock(block)},
		fmt.Sprintf("unblocked %s", block.TaskID),
	)
}

func (n *daemonNativeTools) taskBlocks(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input taskBlocksInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	actor, err := actorContextFromScope(scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	blocks, err := n.deps.Tasks.ListTaskBlocks(
		ctx,
		strings.TrimSpace(input.TaskID),
		input.IncludeCleared,
		actor,
	)
	if err != nil {
		return toolspkg.ToolResult{}, nativeTaskToolError(req.ToolID, err)
	}
	return structuredResult(
		map[string]any{"blocks": core.TaskBlockPayloadsFromBlocks(blocks)},
		fmt.Sprintf("listed %d task blocks", len(blocks)),
	)
}

func (n *daemonNativeTools) taskRecover(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input taskRecoverInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	if strings.TrimSpace(scope.SessionID) != "" && !scope.Operator {
		return toolspkg.ToolResult{}, nativeSessionTaskRecoverDenied(req.ToolID)
	}
	actor, err := nativeOperatorActorContext(scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	recovered, err := n.deps.Tasks.RecoverTask(
		ctx,
		strings.TrimSpace(input.TaskID),
		strings.TrimSpace(input.Note),
		actor,
	)
	if err != nil {
		return toolspkg.ToolResult{}, nativeTaskToolError(req.ToolID, err)
	}
	return structuredResult(
		map[string]any{nativeToolsTaskKey: core.TaskPayloadFromTask(recovered)},
		fmt.Sprintf("recovered %s", recovered.ID),
	)
}

func (n *daemonNativeTools) taskRunList(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input taskRunListInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	actor, err := actorContextFromScope(scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	runs, err := n.deps.Tasks.ListTaskRuns(ctx, input.TaskID, input.query(), actor)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(map[string]any{nativeToolsRunsKey: runs}, fmt.Sprintf("%d runs", len(runs)))
}

func (n *daemonNativeTools) taskPromoteFromThread(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input taskPromoteFromThreadInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	source, err := n.nativeThreadPromotionSource(ctx, scope, req.ToolID, input)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	actor, err := actorContextFromScope(scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	spec := taskpkg.CreateTask{
		Scope:       taskpkg.ScopeWorkspace,
		WorkspaceID: source.workspaceID,
		Title:       nativePromotedThreadTaskTitle(input, source),
		Description: firstNonEmpty(strings.TrimSpace(input.Description), source.digest),
		Priority:    taskpkg.Priority(strings.TrimSpace(input.Priority)).Normalize(),
		Metadata:    cloneJSON(input.Metadata),
	}
	taskRecord, err := n.deps.Tasks.CreateTask(ctx, spec, actor)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	now := time.Now().UTC()
	origin := store.NetworkTaskThreadOrigin{
		TaskID:           taskRecord.ID,
		WorkspaceID:      source.workspaceID,
		Channel:          source.channel,
		ThreadID:         source.threadID,
		OriginMessageID:  source.originMessageID,
		Digest:           source.digest,
		SourceMessageIDs: source.sourceMessageIDs,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := origin.Validate(); err != nil {
		return toolspkg.ToolResult{}, nativeNetworkInputError(
			req.ToolID,
			n.rollbackPromotedThreadTask(ctx, taskRecord.ID, actor, err),
		)
	}
	if err := n.deps.NetworkStore.PutNetworkTaskThreadOrigin(ctx, origin); err != nil {
		return toolspkg.ToolResult{}, nativeNetworkInputError(
			req.ToolID,
			n.rollbackPromotedThreadTask(ctx, taskRecord.ID, actor, err),
		)
	}
	return structuredResult(
		map[string]any{"task": taskRecord, "origin": core.NetworkTaskThreadOriginPayloadFromStore(origin)},
		taskRecord.Title,
	)
}
