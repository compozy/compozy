package loop

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/compozy/agh/internal/loop/dsl"
)

// RunLoopActionExecutor starts a child loop for await or detach semantics.
type RunLoopActionExecutor struct {
	starter ActionLoopStarter
}

// Execute starts the child through loop.Service.Start.
func (e *RunLoopActionExecutor) Execute(
	ctx context.Context,
	node dsl.Node,
	in ActionExecutionInput,
) (ActionRawResult, error) {
	if e == nil || e.starter == nil {
		return ActionRawResult{}, reasonError(
			ReasonCodeActionDependencyMissing,
			ErrActionDependencyMissing,
			map[string]string{actionDependencyMetaKey: "loop_starter"},
		)
	}
	runCtx, cancel, err := actionContextWithNodeTimeout(ctx, node.Timeout)
	if err != nil {
		return ActionRawResult{}, err
	}
	defer cancel()
	params, err := renderNodeParams(node, in.Namespace)
	if err != nil {
		return ActionRawResult{}, err
	}
	var spec dsl.RunLoopParams
	if err := dsl.NodeParams(params).Decode(&spec); err != nil {
		return ActionRawResult{}, fmt.Errorf("decode run-loop params: %w", err)
	}
	if spec.Mode == "" {
		spec.Mode = dsl.RunLoopAwait
	}
	child, err := e.starter.Start(runCtx, in.WorkspaceID, spec.Loop, Inputs{
		Values:          spec.Inputs,
		ParentLoopRunID: in.LoopRunID,
	}, in.Actor)
	if err != nil {
		return ActionRawResult{}, fmt.Errorf("start child loop %q: %w", spec.Loop, err)
	}
	if child == nil {
		return ActionRawResult{}, fmt.Errorf("%w: loop starter returned nil run", ErrActionDependencyMissing)
	}
	payload := map[string]any{metadataLoopRunIDKey: string(child.ID)}
	if spec.Mode == dsl.RunLoopAwait {
		payload["status"] = "awaiting_child"
	}
	structured, err := json.Marshal(payload)
	if err != nil {
		return ActionRawResult{}, fmt.Errorf("marshal run-loop output: %w", err)
	}
	raw := ActionRawResult{
		Structured:     structured,
		Value:          payload,
		ChildLoopRunID: child.ID,
	}
	if spec.Mode == dsl.RunLoopAwait {
		raw.Status = "awaiting_child"
	}
	// Detach intentionally emits no awaiting status; the coordinator should not wait for the child terminal wake.
	return raw, nil
}

// Harvest returns the run-loop child id and await status.
func (e *RunLoopActionExecutor) Harvest(_ context.Context, raw ActionRawResult, node dsl.Node) (ActionOutput, error) {
	if err := validateSyncHarvest(node); err != nil {
		return ActionOutput{}, err
	}
	return outputFromRaw(raw)
}
