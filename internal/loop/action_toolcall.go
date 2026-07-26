package loop

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/tools"
)

// ToolCallActionExecutor adapts a ToolID action to RuntimeRegistry.Call.
type ToolCallActionExecutor struct {
	runtime     tools.Registry
	toolID      tools.ToolID
	eventReader ActionEventRangeReader
	channel     ChannelResultHarvester
}

// Execute calls RuntimeRegistry.Call with rendered action params.
func (e *ToolCallActionExecutor) Execute(
	ctx context.Context,
	node dsl.Node,
	in ActionExecutionInput,
) (ActionRawResult, error) {
	if e == nil || e.runtime == nil {
		return ActionRawResult{}, reasonError(
			ReasonCodeActionDependencyMissing,
			ErrActionDependencyMissing,
			map[string]string{actionDependencyMetaKey: "runtime_registry"},
		)
	}
	callCtx, cancel, err := actionContextWithNodeTimeout(ctx, node.Timeout)
	if err != nil {
		return ActionRawResult{}, err
	}
	defer cancel()

	params, err := renderNodeParams(node, in.Namespace)
	if err != nil {
		return ActionRawResult{}, err
	}
	harvest, err := renderHarvestSpec(node, in.Namespace)
	if err != nil {
		return ActionRawResult{}, err
	}
	input, err := json.Marshal(params)
	if err != nil {
		return ActionRawResult{}, fmt.Errorf("marshal action params for %q: %w", node.ID, err)
	}
	scope := normalizeActionToolScope(in)
	result, err := e.runtime.Call(callCtx, scope, tools.CallRequest{
		ToolID:        e.toolID,
		ToolCallID:    actionToolCallID(node, in),
		SessionID:     scope.SessionID,
		WorkspaceID:   scope.WorkspaceID,
		AgentName:     scope.AgentName,
		ActorKind:     scope.ActorKind,
		CorrelationID: in.CorrelationID,
		Input:         input,
	})
	if err != nil {
		return ActionRawResult{}, fmt.Errorf("call action tool %q: %w", e.toolID, err)
	}
	raw := ActionRawResult{
		ToolResult:      result,
		Structured:      cloneRawMessage(result.Structured),
		WorkspaceID:     scope.WorkspaceID,
		SessionID:       firstNonEmpty(metadataString(result.Metadata, watchEventsFieldSessionID), scope.SessionID),
		RenderedParams:  cloneRawMessage(input),
		RenderedHarvest: harvest,
	}
	raw.EventStartSeq = firstNonZero(
		metadataInt64(result.Metadata, "event_start_seq"),
		metadataInt64(result.Metadata, "eventStartSeq"),
	)
	raw.EventEndSeq = firstNonZero(
		metadataInt64(result.Metadata, "event_end_seq"),
		metadataInt64(result.Metadata, "eventEndSeq"),
	)
	return raw, nil
}

// Harvest applies the node harvest policy to a tool call result.
func (e *ToolCallActionExecutor) Harvest(
	ctx context.Context,
	raw ActionRawResult,
	node dsl.Node,
) (ActionOutput, error) {
	kind := harvestKind(node)
	switch kind {
	case "", harvestKindSync:
		return outputFromRaw(raw)
	case harvestKindEventRange, harvestKindAsync:
		return e.harvestEventRange(ctx, raw, node)
	case harvestKindChannelResult:
		return e.harvestChannelResult(ctx, raw, node)
	default:
		return ActionOutput{}, fmt.Errorf("%w: unsupported harvest kind %q", ErrValidation, kind)
	}
}

func (e *ToolCallActionExecutor) harvestEventRange(
	ctx context.Context,
	raw ActionRawResult,
	node dsl.Node,
) (ActionOutput, error) {
	if e == nil || e.eventReader == nil {
		return ActionOutput{}, reasonError(
			ReasonCodeActionDependencyMissing,
			ErrActionDependencyMissing,
			map[string]string{actionDependencyMetaKey: "event_range_reader"},
		)
	}
	if err := validateEventRange(raw); err != nil {
		return ActionOutput{}, err
	}
	result, err := e.eventReader.ReadActionEventRange(ctx, ActionEventRangeRequest{
		SessionID:     raw.SessionID,
		EventStartSeq: raw.EventStartSeq,
		EventEndSeq:   raw.EventEndSeq,
		Raw:           raw,
		Node:          node,
	})
	if err != nil {
		return ActionOutput{}, fmt.Errorf("harvest action event range: %w", err)
	}
	if len(bytes.TrimSpace(result.Structured)) > 0 {
		out := raw
		out.Structured = cloneRawMessage(result.Structured)
		return outputFromRaw(out)
	}
	structured, err := json.Marshal(map[string]any{
		"events":                  result.Events,
		"event_start_seq":         raw.EventStartSeq,
		"event_end_seq":           raw.EventEndSeq,
		watchEventsFieldSessionID: raw.SessionID,
	})
	if err != nil {
		return ActionOutput{}, fmt.Errorf("marshal event range harvest: %w", err)
	}
	out := raw
	out.Structured = structured
	return outputFromRaw(out)
}

func (e *ToolCallActionExecutor) harvestChannelResult(
	ctx context.Context,
	raw ActionRawResult,
	node dsl.Node,
) (ActionOutput, error) {
	if e == nil || e.toolID != tools.ToolIDNetworkSend {
		return ActionOutput{}, fmt.Errorf(
			"%w: channel_result harvest requires %s action kind",
			ErrValidation,
			tools.ToolIDNetworkSend,
		)
	}
	if e == nil || e.channel == nil {
		return ActionOutput{}, reasonError(
			ReasonCodeActionDependencyMissing,
			ErrActionDependencyMissing,
			map[string]string{actionDependencyMetaKey: "channel_result_harvester"},
		)
	}
	harvest := raw.RenderedHarvest
	if harvest == nil {
		harvest = node.Harvest
	}
	if harvest == nil {
		return ActionOutput{}, fmt.Errorf("%w: channel_result harvest spec is required", ErrValidation)
	}
	result, err := e.channel.HarvestChannelResult(ctx, &ChannelResultHarvestRequest{
		Window:      harvest.Window,
		Responder:   harvest.Responder,
		ContentRule: harvest.ContentRule,
		Raw:         raw,
		Node:        node,
	})
	if err != nil {
		return ActionOutput{}, fmt.Errorf("harvest channel_result: %w", err)
	}
	if !result.Found {
		return ActionOutput{}, reasonError(
			ReasonCodeActionStalled,
			ErrActionStalled,
			map[string]string{actionKindMetaKey: harvestKindChannelResult},
		)
	}
	out := raw
	out.Structured = cloneRawMessage(result.Structured)
	out.Text = result.Text
	return outputFromRaw(out)
}
