package loop

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// GenerationOutputPayloadKey identifies the exact cell that owns an externalized payload.
type GenerationOutputPayloadKey struct {
	WorkspaceID WorkspaceID
	RunID       RunID
	Generation  int
	NodeID      NodeID
	ItemIndex   int
	OutputRef   string
}

// GenerationOutputPayloadReader resolves externalized payloads through their owning cell.
type GenerationOutputPayloadReader interface {
	GetGenerationOutputPayload(
		ctx context.Context,
		key GenerationOutputPayloadKey,
	) (json.RawMessage, error)
}

type generationOutputRuntimeScope struct {
	workspaceID WorkspaceID
	runID       RunID
	generation  int
}

func generationOutputRuntimeView(
	ctx context.Context,
	reader GenerationOutputPayloadReader,
	scope generationOutputRuntimeScope,
	outputs []GenerationOutput,
) ([]GenerationOutput, error) {
	view := cloneGenerationOutputs(outputs)
	if overlays, ok := reader.(GenerationOutputOverlayReader); ok {
		var err error
		view, err = overlays.ApplyGenerationOutputOverlays(
			ctx, scope.workspaceID, scope.runID, scope.generation, view,
		)
		if err != nil {
			return nil, err
		}
	}
	if len(view) == 0 {
		return view, nil
	}
	resolved := make(map[GenerationOutputPayloadKey]json.RawMessage)
	for index := range view {
		ref := strings.TrimSpace(view[index].OutputRef)
		if !generationOutputHasKind(view[index], GenerationResultPayload, GenerationResultFailure) ||
			!OutputRefLooksContentAddressed(ref) {
			continue
		}
		if reader == nil {
			return nil, fmt.Errorf("%w: generation output payload reader is required", ErrValidation)
		}
		key := GenerationOutputPayloadKey{
			WorkspaceID: scope.workspaceID,
			RunID:       scope.runID,
			Generation:  scope.generation,
			NodeID:      NodeID(view[index].NodeID),
			ItemIndex:   view[index].ItemIndex,
			OutputRef:   ref,
		}
		payload, ok := resolved[key]
		if !ok {
			var err error
			payload, err = reader.GetGenerationOutputPayload(ctx, key)
			if err != nil {
				return nil, fmt.Errorf(
					"loop: resolve generation %d output %s/%d: %w",
					scope.generation,
					view[index].NodeID,
					view[index].ItemIndex,
					err,
				)
			}
			if !json.Valid(payload) || OutputRefForPayload(payload) != ref {
				return nil, fmt.Errorf(
					"loop: resolve generation %d output %s/%d ref %q: %w",
					scope.generation,
					view[index].NodeID,
					view[index].ItemIndex,
					ref,
					ErrOutputRefNotFound,
				)
			}
			resolved[key] = cloneRawMessage(payload)
		}
		view[index].runtimePayload = cloneRawMessage(payload)
	}
	return view, nil
}

func generationOutputRuntimePayload(output GenerationOutput) string {
	if len(output.runtimePayload) > 0 {
		return string(output.runtimePayload)
	}
	return output.OutputRef
}

func generationOutputRuntimeValue(output GenerationOutput) any {
	return generationResultValue(GenerationResultRef{
		Kind:       output.ResultKind,
		SchemaRef:  output.SchemaRef,
		PayloadRef: output.OutputRef,
	}, output.runtimePayload)
}

func setGenerationOutputRef(output *GenerationOutput, ref string) {
	if output == nil {
		return
	}
	result := generationResultForRef(ref)
	output.ResultKind = result.Kind
	output.SchemaRef = result.SchemaRef
	output.OutputRef = result.PayloadRef
	output.runtimePayload = nil
}
