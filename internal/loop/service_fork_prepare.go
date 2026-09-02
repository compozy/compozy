package loop

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
)

type forkPreparation struct {
	source   Run
	outputs  []GenerationOutput
	snapshot json.RawMessage
	resolved *ResolvedDefinition
	values   map[string]any
}

func (s *service) prepareForkRun(ctx context.Context, input ForkInput) (forkPreparation, error) {
	source, err := s.store.GetLoopRun(ctx, input.WorkspaceID, input.RunID)
	if err != nil {
		return forkPreparation{}, err
	}
	if err := s.rejectExecutingTimeTravelSelfOperation(ctx, source, input.Actor); err != nil {
		return forkPreparation{}, err
	}
	outputs, err := requireTimeTravelOutputs(
		ctx, s.store, source, int(input.Generation), ReasonCodeForkGenerationUnknown, ErrForkGenerationUnknown,
	)
	if err != nil {
		return forkPreparation{}, err
	}
	if err := validateForkGenerationOutputs(outputs, input.Generation); err != nil {
		return forkPreparation{}, err
	}
	snapshot, err := s.store.GetLoopDefinitionSnapshot(ctx, source.WorkspaceID, source.DefinitionDigest)
	if err != nil {
		return forkPreparation{}, err
	}
	resolved, err := LoadExecutedDefinitionSnapshot(snapshot.Definition, source.DefinitionDigest)
	if err != nil {
		return forkPreparation{}, err
	}
	values := make(map[string]any, len(source.Inputs)+len(input.Inputs))
	maps.Copy(values, source.Inputs)
	maps.Copy(values, input.Inputs)
	values, err = ResolveInputs(resolved.Definition, Inputs{Values: values})
	if err != nil {
		return forkPreparation{}, err
	}
	if err := s.validateForkInputEntities(ctx, input.WorkspaceID, source.ProfileID, resolved, values); err != nil {
		return forkPreparation{}, err
	}
	return forkPreparation{
		source: source, outputs: outputs, snapshot: snapshot.Definition,
		resolved: resolved, values: values,
	}, nil
}

func validateForkGenerationOutputs(outputs []GenerationOutput, generation int64) error {
	for _, output := range outputs {
		if generationOutputSettled(output.Status) {
			continue
		}
		return reasonError(
			ReasonCodeForkGenerationUnknown,
			ErrForkGenerationUnknown,
			map[string]string{
				metadataGenerationKey: fmt.Sprintf("%d", generation),
				metadataNodeIDKey:     output.NodeID,
				namespaceStatusKey:    output.Status,
			},
		)
	}
	return nil
}

func (s *service) validateForkInputEntities(
	ctx context.Context,
	workspaceID WorkspaceID,
	profileID string,
	resolved *ResolvedDefinition,
	values map[string]any,
) error {
	origins := make(map[string]InputOrigin, len(values))
	for field := range values {
		origins[field] = InputOriginRun
	}
	return s.validateResolvedInputEntities(
		ctx,
		workspaceID,
		profileID,
		resolved.Definition.Meta.Name,
		resolved.Definition,
		ResolvedInputs{Values: values, Origins: origins},
	)
}
