package loop

import (
	"context"
	"encoding/json"
	"fmt"
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
	declaredSchema, err := resolvedDefinitionOutputSchema(resolved, node)
	if err != nil {
		return NodeAmendment{}, NewRequestReasonError(ReasonCodeAmendSchemaMissing, err, nil)
	}
	input.Schema, err = json.Marshal(declaredSchema)
	if err != nil {
		return NodeAmendment{}, fmt.Errorf("loop: marshal amendment output schema: %w", err)
	}
	input.RequestedAt = s.now().UTC()
	return store.AmendNodeOutput(ctx, input)
}
