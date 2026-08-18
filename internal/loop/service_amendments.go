package loop

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/internal/loop/dsl"
)

func (s *service) AmendNodeOutput(ctx context.Context, input AmendInput) (NodeAmendment, error) {
	store, ok := s.store.(AmendmentStore)
	if !ok {
		return NodeAmendment{}, fmt.Errorf("%w: amendment store is unavailable", ErrActionDependencyMissing)
	}
	if err := input.Actor.Validate(); err != nil {
		return NodeAmendment{}, fmt.Errorf("%w: amendment actor: %w", ErrValidation, err)
	}
	if err := s.rejectSelfOperation(ctx, input.WorkspaceID, input.RunID, input.Actor); err != nil {
		return NodeAmendment{}, err
	}
	run, err := s.store.GetLoopRun(ctx, input.WorkspaceID, input.RunID)
	if err != nil {
		return NodeAmendment{}, err
	}
	if input.Generation == 0 {
		input.Generation = run.Generation
	}
	resolved, err := s.pinnedResolvedDefinition(ctx, run)
	if err != nil {
		return NodeAmendment{}, err
	}
	node, found := graphNode(resolved.Definition.Graph, input.NodeID)
	if !found {
		return NodeAmendment{}, fmt.Errorf("%w: node %q not found", ErrValidation, input.NodeID)
	}
	input.Schema, err = declaredNodeOutputSchema(resolved, node)
	if err != nil {
		return NodeAmendment{}, err
	}
	input.RequestedAt = s.now().UTC()
	return store.AmendNodeOutput(ctx, input)
}

func declaredNodeOutputSchema(resolved *ResolvedDefinition, node dsl.Node) (json.RawMessage, error) {
	if len(node.Produces) > 0 {
		return marshalDeclaredSchema(node.Produces)
	}
	switch dsl.ActionKind(node.Kind) {
	case dsl.ActionRunAgent:
		var params dsl.RunAgentParams
		if err := node.Params.Decode(&params); err != nil {
			return nil, fmt.Errorf("loop: decode run-agent output schema: %w", err)
		}
		if len(params.OutputSchema) > 0 {
			return marshalDeclaredSchema(params.OutputSchema)
		}
	case dsl.ActionGoal:
		var params dsl.GoalParams
		if err := node.Params.Decode(&params); err != nil {
			return nil, fmt.Errorf("loop: decode Goal output schema: %w", err)
		}
		if params.OutputSchema != nil && len(*params.OutputSchema) > 0 {
			return marshalDeclaredSchema(*params.OutputSchema)
		}
	default:
		if resolved != nil {
			if snapshot, ok := resolved.ToolSchemas[node.Kind]; ok && len(snapshot.OutputSchema) > 0 {
				return cloneRawMessage(snapshot.OutputSchema), nil
			}
		}
	}
	return nil, NewRequestReasonError(ReasonCodeAmendSchemaMissing, ErrAmendSchemaMissing, nil)
}

func marshalDeclaredSchema(schema dsl.Schema) (json.RawMessage, error) {
	raw, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("loop: marshal declared output schema: %w", err)
	}
	return raw, nil
}
