package loop

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/loop/dsl"
)

// InputEntityCatalog is the daemon-owned authority for exact entity lookups.
type InputEntityCatalog interface {
	HasInputEntity(context.Context, WorkspaceID, string, dsl.EntityKind, string) (bool, error)
}

func (s *service) validateResolvedInputEntities(
	ctx context.Context,
	workspaceID WorkspaceID,
	profileID string,
	loopName string,
	definition dsl.Definition,
	resolved ResolvedInputs,
) error {
	return ValidateInputEntities(
		ctx, workspaceID, profileID, loopName, definition, resolved, s.inputEntities, s.runtimeCatalog,
	)
}

// ValidateInputEntities checks resolved entity and runtime values against injected authorities.
func ValidateInputEntities(
	ctx context.Context,
	workspaceID WorkspaceID,
	profileID string,
	loopName string,
	definition dsl.Definition,
	resolved ResolvedInputs,
	entities InputEntityCatalog,
	runtimes WorkspaceRuntimeCatalog,
) error {
	for _, field := range sortedInputKeys(definition.Inputs) {
		value, present := resolved.Values[field]
		if !present {
			continue
		}
		input := definition.Inputs[field]
		origin := resolved.Origins[field]
		switch input.Type {
		case dsl.InputTypeRuntime:
			if err := validateRuntimeInput(
				ctx, workspaceID, field, input, value, origin, loopName, runtimes,
			); err != nil {
				return err
			}
		case dsl.InputTypeAgent:
			if err := validateEntityInput(
				ctx, workspaceID, profileID, dsl.EntityKindAgent, field, input, value, origin, loopName, entities,
			); err != nil {
				return err
			}
		case dsl.InputTypeRef:
			if input.Ref == nil {
				continue
			}
			if err := validateEntityInput(
				ctx, workspaceID, profileID, dsl.EntityKind(input.Ref.Kind), field, input, value, origin, loopName,
				entities,
			); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateEntityInput(
	ctx context.Context,
	workspaceID WorkspaceID,
	profileID string,
	kind dsl.EntityKind,
	field string,
	input dsl.Input,
	value any,
	origin InputOrigin,
	loopName string,
	entities InputEntityCatalog,
) error {
	if entities == nil {
		return nil
	}
	text, ok := value.(string)
	if !ok {
		return newInputValidationError(
			loopName, field, input, value, origin, InputValidationReasonInvalidKindPayload,
			fmt.Errorf("entity input must be a string"),
		)
	}
	exists, err := entities.HasInputEntity(ctx, workspaceID, profileID, kind, text)
	if err != nil {
		return fmt.Errorf("validate Loop input %q %s reference: %w", field, kind, err)
	}
	if exists {
		return nil
	}
	return newInputValidationError(
		loopName, field, input, value, origin, InputValidationReasonUnknownReference,
		fmt.Errorf("%s %q does not exist", kind, text),
	)
}

func validateRuntimeInput(
	ctx context.Context,
	workspaceID WorkspaceID,
	field string,
	input dsl.Input,
	value any,
	origin InputOrigin,
	loopName string,
	runtimes WorkspaceRuntimeCatalog,
) error {
	runtime, err := runtimeInputSpec(value)
	if err != nil {
		return newInputValidationError(
			loopName, field, input, value, origin, InputValidationReasonInvalidKindPayload, err,
		)
	}
	var catalog RuntimeCatalog
	if runtimes != nil {
		catalog, err = runtimes.ForWorkspace(ctx, workspaceID)
		if err != nil {
			return fmt.Errorf("resolve workspace runtime catalog: %w", err)
		}
		if catalog == nil {
			return fmt.Errorf("%w: workspace runtime catalog returned nil", ErrActionDependencyMissing)
		}
	}
	if err := validateRuntimeSpec(ctx, catalog, "inputs."+field, runtime); err != nil {
		return newInputValidationError(
			loopName, field, input, value, origin, InputValidationReasonInvalidRuntime, err,
		)
	}
	return nil
}
