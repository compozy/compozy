package loop

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/task"
)

func (r *CoordinatorRunner) configureActionExecutionInput(
	ctx context.Context,
	taskRun task.Run,
	actionCtx *coordinatorActionRunContext,
	input *ActionExecutionInput,
) error {
	if actionCtx.meta.ReviewedParamsRef != "" {
		payload, err := r.outputs.GetGenerationOutputPayload(ctx, GenerationOutputPayloadKey{
			WorkspaceID: actionCtx.loopRun.WorkspaceID, RunID: actionCtx.loopRun.ID,
			Generation: actionCtx.meta.Generation, NodeID: actionCtx.node.ID,
			ItemIndex: actionCtx.meta.ItemIndex, OutputRef: actionCtx.meta.ReviewedParamsRef,
		})
		if err != nil {
			return fmt.Errorf("loop: load reviewed action params: %w", err)
		}
		var params map[string]any
		if err := json.Unmarshal(payload, &params); err != nil {
			return fmt.Errorf("loop: decode reviewed action params: %w", err)
		}
		input.AdmittedParams = dsl.NodeParams(params)
	}
	retryFailure, err := r.actionRetryFailure(ctx, actionCtx.loopRun, actionCtx.meta)
	if err != nil {
		return err
	}
	input.RetryFailure = retryFailure
	input.UsageReporter = actionUsageReporterFromContext(ctx)
	if input.RuntimeSelection == nil {
		input.RuntimeSelection = &ActionRuntimeSelection{}
	}
	if r.runtimeCatalog != nil {
		input.RuntimeSelection.Catalog, err = r.runtimeCatalog.ForWorkspace(ctx, actionCtx.loopRun.WorkspaceID)
		if err != nil {
			return err
		}
	}
	if recorder, ok := r.outputs.(ActionAppliedRuntimeRecorder); ok {
		input.RuntimeSelection.Recorder = recorder
	}
	input.PersistedTaskTokensUsed = taskRun.TokensUsed
	return nil
}
