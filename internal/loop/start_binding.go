package loop

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/loop/dsl/refs"
	"github.com/compozy/agh/internal/task"
)

// StartBindingRequest describes one concrete start from a declared loop surface.
type StartBindingRequest struct {
	WorkspaceID WorkspaceID
	LoopName    string
	Kind        dsl.StartKind
	ActorRef    string
	OriginRef   string
	Inputs      Inputs
}

// StartTargetValidation validates static and mapped inputs for one automation-carried start.
type StartTargetValidation struct {
	WorkspaceID  WorkspaceID
	LoopName     string
	Kind         dsl.StartKind
	Inputs       map[string]any
	InputMapping map[string]string
}

// StartTargetResolution resolves static inputs plus input_mapping against a trigger payload.
type StartTargetResolution struct {
	StartTargetValidation
	TriggerPayload map[string]any
}

const (
	startInputMetaKey = "input"
	startLoopMetaKey  = "loop"
)

// StartFromBinding derives the actor for the surface, enforces the definition allowlist,
// and then delegates execution to the single loop Service.Start point.
func StartFromBinding(
	ctx context.Context,
	service Service,
	resolver DefinitionResolver,
	req StartBindingRequest,
) (*Run, error) {
	actor, err := DeriveStartActor(req.Kind, string(req.WorkspaceID), req.ActorRef, req.OriginRef)
	if err != nil {
		return nil, err
	}
	return StartFromActor(ctx, service, resolver, req, actor)
}

// StartFromActor enforces the definition allowlist before delegating to Service.Start.
func StartFromActor(
	ctx context.Context,
	service Service,
	resolver DefinitionResolver,
	req StartBindingRequest,
	actor task.ActorContext,
) (*Run, error) {
	if service == nil {
		return nil, fmt.Errorf("%w: loop service is required", ErrValidation)
	}
	if _, err := ResolveStartBinding(ctx, resolver, req.WorkspaceID, req.LoopName, req.Kind); err != nil {
		return nil, err
	}
	return service.Start(ctx, req.WorkspaceID, req.LoopName, req.Inputs, actor)
}

// ResolveStartBinding loads the definition and returns the matching start binding.
func ResolveStartBinding(
	ctx context.Context,
	resolver DefinitionResolver,
	ws WorkspaceID,
	name string,
	kind dsl.StartKind,
) (*dsl.StartBinding, error) {
	_, binding, err := resolveStartBindingAndDefinition(ctx, resolver, ws, name, kind)
	return binding, err
}

// ValidateStartTarget checks the allowlist and the target input keys/templates without firing a loop.
func ValidateStartTarget(
	ctx context.Context,
	resolver DefinitionResolver,
	req StartTargetValidation,
) error {
	resolved, err := resolveStartTargetDefinition(ctx, resolver, req.WorkspaceID, req.LoopName, req.Kind)
	if err != nil {
		return err
	}
	return validateStartTargetAgainstDefinition(resolved.Definition, req)
}

// ResolveStartTargetInputs resolves static inputs plus input_mapping into typed loop inputs.
func ResolveStartTargetInputs(
	ctx context.Context,
	resolver DefinitionResolver,
	req StartTargetResolution,
) (map[string]any, error) {
	resolved, err := resolveStartTargetDefinition(ctx, resolver, req.WorkspaceID, req.LoopName, req.Kind)
	if err != nil {
		return nil, err
	}
	if err := validateStartTargetAgainstDefinition(resolved.Definition, req.StartTargetValidation); err != nil {
		return nil, err
	}
	values := cloneMap(req.Inputs)
	for input, rawTemplate := range req.InputMapping {
		field, err := triggerPayloadField(rawTemplate)
		if err != nil {
			return nil, wrapStartMappingError(input, err)
		}
		value, ok := req.TriggerPayload[field]
		if !ok {
			return nil, wrapStartMappingError(
				input,
				fmt.Errorf("%w: trigger payload field %q is missing", ErrValidation, field),
			)
		}
		values[input] = cloneAnyValue(value)
	}
	resolvedInputs, err := ResolveInputs(resolved.Definition, Inputs{Values: values})
	if err != nil {
		return nil, reasonError(ReasonCodeStartInputInvalid, err, map[string]string{startLoopMetaKey: req.LoopName})
	}
	return resolvedInputs, nil
}

// DeriveStartActor maps every start surface to its canonical task ActorContext derivation.
func DeriveStartActor(
	kind dsl.StartKind,
	workspaceID string,
	actorRef string,
	originRef string,
) (task.ActorContext, error) {
	normalizedKind, err := normalizeStartKind(kind)
	if err != nil {
		return task.ActorContext{}, err
	}
	switch normalizedKind {
	case dsl.StartManual:
		return task.DeriveHumanActorContextForWorkspace(actorRef, workspaceID, task.OriginKindWeb, originRef)
	case dsl.StartCLI:
		return task.DeriveHumanActorContextForWorkspace(actorRef, workspaceID, task.OriginKindCLI, originRef)
	case dsl.StartHTTP:
		return task.DeriveHumanActorContextForWorkspace(actorRef, workspaceID, task.OriginKindHTTP, originRef)
	case dsl.StartUDS:
		return task.DeriveHumanActorContextForWorkspace(actorRef, workspaceID, task.OriginKindUDS, originRef)
	case dsl.StartTrigger, dsl.StartSchedule, dsl.StartWebhook:
		return task.DeriveAutomationActorContext(actorRef, originRef)
	case dsl.StartNetwork:
		return task.DeriveNetworkPeerActorContext(actorRef, originRef)
	case dsl.StartExtension:
		return task.DeriveExtensionActorContext(actorRef, originRef)
	case dsl.StartNativeTool:
		return task.DeriveAgentSessionActorContextForOrigin(
			actorRef,
			workspaceID,
			task.OriginKindAgentSession,
			originRef,
		)
	default:
		return task.ActorContext{}, fmt.Errorf("%w: unsupported start kind %q", ErrValidation, kind)
	}
}

func resolveStartTargetDefinition(
	ctx context.Context,
	resolver DefinitionResolver,
	ws WorkspaceID,
	name string,
	kind dsl.StartKind,
) (*ResolvedDefinition, error) {
	resolved, _, err := resolveStartBindingAndDefinition(ctx, resolver, ws, name, kind)
	return resolved, err
}

func resolveStartBindingAndDefinition(
	ctx context.Context,
	resolver DefinitionResolver,
	ws WorkspaceID,
	name string,
	kind dsl.StartKind,
) (*ResolvedDefinition, *dsl.StartBinding, error) {
	if resolver == nil {
		return nil, nil, fmt.Errorf("%w: loop definition resolver is required", ErrValidation)
	}
	normalizedKind, err := normalizeStartKind(kind)
	if err != nil {
		return nil, nil, err
	}
	trimmedName, err := normalizeLoopName(name)
	if err != nil {
		return nil, nil, err
	}
	resolved, err := resolver.ResolveLoop(ctx, ws, trimmedName)
	if err != nil {
		return nil, nil, err
	}
	if resolved == nil {
		return nil, nil, fmt.Errorf("%w: loop definition %q resolved empty", ErrValidation, trimmedName)
	}
	for idx := range resolved.Definition.Start {
		binding := &resolved.Definition.Start[idx]
		if binding.Kind == normalizedKind {
			return resolved, binding, nil
		}
	}
	return nil, nil, reasonError(
		ReasonCodeStartKindNotAllowed,
		fmt.Errorf("%w: loop %q does not declare start kind %q", ErrValidation, trimmedName, normalizedKind),
		map[string]string{startLoopMetaKey: trimmedName, actionKindMetaKey: string(normalizedKind)},
	)
}

func validateStartTargetAgainstDefinition(def dsl.Definition, req StartTargetValidation) error {
	normalizedKind, err := normalizeStartKind(req.Kind)
	if err != nil {
		return err
	}
	if normalizedKind == dsl.StartSchedule && len(req.InputMapping) > 0 {
		return reasonError(
			ReasonCodeStartInputMappingInvalid,
			fmt.Errorf("%w: scheduled loop targets use static inputs only", ErrValidation),
			map[string]string{actionKindMetaKey: string(normalizedKind)},
		)
	}
	if err := validateStartInputKeys(def, req.Inputs, "inputs"); err != nil {
		return err
	}
	if err := validateStartMappingKeys(def, req.InputMapping); err != nil {
		return err
	}
	for name, input := range def.Inputs {
		if input.Default != nil {
			continue
		}
		_, hasStatic := req.Inputs[name]
		_, hasMapped := req.InputMapping[name]
		if input.Required && !hasStatic && !hasMapped {
			return reasonError(
				ReasonCodeStartInputInvalid,
				fmt.Errorf("%w: input %q is required", ErrValidation, name),
				map[string]string{startInputMetaKey: name},
			)
		}
	}
	return nil
}

func validateStartInputKeys(def dsl.Definition, inputs map[string]any, label string) error {
	for name, value := range inputs {
		declared, ok := def.Inputs[name]
		if !ok {
			return reasonError(
				ReasonCodeStartInputInvalid,
				fmt.Errorf("%w: %s key %q does not name a declared input", ErrValidation, label, name),
				map[string]string{startInputMetaKey: name},
			)
		}
		if err := validateInputType(name, declared.Type, value); err != nil {
			return reasonError(ReasonCodeStartInputInvalid, err, map[string]string{startInputMetaKey: name})
		}
	}
	return nil
}

func validateStartMappingKeys(def dsl.Definition, mapping map[string]string) error {
	namespace := refs.Namespace{Inputs: startInputNamespace(def), AllowTrigger: true}
	for input, raw := range mapping {
		if _, ok := def.Inputs[input]; !ok {
			return wrapStartMappingError(
				input,
				fmt.Errorf("%w: input_mapping key %q does not name a declared input", ErrValidation, input),
			)
		}
		if _, err := refs.CompileTemplate("start.input_mapping."+input, raw, namespace); err != nil {
			return wrapStartMappingError(input, err)
		}
		if _, err := triggerPayloadField(raw); err != nil {
			return wrapStartMappingError(input, err)
		}
	}
	return nil
}

func startInputNamespace(def dsl.Definition) map[string]refs.Schema {
	inputs := make(map[string]refs.Schema, len(def.Inputs))
	for name, input := range def.Inputs {
		inputs[name] = inputSchema(input)
	}
	return inputs
}

func triggerPayloadField(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "{{") || !strings.HasSuffix(trimmed, "}}") {
		return "", fmt.Errorf("%w: input_mapping must be {{ .trigger.payload.<field> }}", ErrValidation)
	}
	body := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "{{"), "}}"))
	body = strings.TrimPrefix(body, ".")
	parts := strings.Split(body, ".")
	if len(parts) != 3 || parts[0] != "trigger" || parts[1] != "payload" || strings.TrimSpace(parts[2]) == "" {
		return "", fmt.Errorf("%w: input_mapping must be {{ .trigger.payload.<field> }}", ErrValidation)
	}
	if strings.ContainsAny(parts[2], " \t\r\n") {
		return "", fmt.Errorf("%w: input_mapping field must be a single flat payload key", ErrValidation)
	}
	return parts[2], nil
}

func wrapStartMappingError(input string, err error) error {
	return reasonError(
		ReasonCodeStartInputMappingInvalid,
		err,
		map[string]string{startInputMetaKey: strings.TrimSpace(input)},
	)
}

func normalizeStartKind(kind dsl.StartKind) (dsl.StartKind, error) {
	normalized := dsl.StartKind(strings.TrimSpace(string(kind)))
	switch normalized {
	case dsl.StartManual,
		dsl.StartCLI,
		dsl.StartHTTP,
		dsl.StartUDS,
		dsl.StartTrigger,
		dsl.StartSchedule,
		dsl.StartWebhook,
		dsl.StartNetwork,
		dsl.StartExtension,
		dsl.StartNativeTool:
		return normalized, nil
	default:
		return "", fmt.Errorf("%w: start kind is invalid: %q", ErrValidation, kind)
	}
}
