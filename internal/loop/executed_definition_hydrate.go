package loop

import (
	"fmt"

	"github.com/compozy/agh/internal/loop/dsl/refs"
)

func hydrateExecutedDefinitionSnapshot(
	envelope *executedDefinitionSnapshot,
) (*ResolvedDefinition, error) {
	resolved := &ResolvedDefinition{
		Definition:           envelope.Definition,
		DefinitionVersion:    envelope.DefinitionVersion,
		Templates:            map[string]*refs.Template{},
		Conditions:           map[string]*refs.Condition{},
		ToolSchemas:          cloneToolSchemaSnapshots(envelope.ToolSchemas),
		WatchEventsContracts: cloneWatchEventsContracts(envelope.WatchEventsContracts),
		Defaults:             envelope.Defaults,
		EffectiveConfig:      cloneEffectiveConfig(envelope.EffectiveConfig),
		compiled:             true,
	}
	definition := resolved.Definition
	definition.Normalize()
	resolved.Definition = definition
	if len(envelope.TemplateSources) == 0 && len(envelope.ConditionSources) == 0 {
		return resolved, nil
	}
	context := newLintContext(definition, &DefinitionLinter{})
	context.indexGraphTrusted()
	if err := compileContract(resolved, definition, context, context.namespace(false, false)); err != nil {
		return nil, fmt.Errorf("loop: hydrate executed contract: %w", err)
	}
	if err := compileStarts(resolved, definition, context); err != nil {
		return nil, fmt.Errorf("loop: hydrate executed starts: %w", err)
	}
	for _, node := range definition.Graph.Nodes {
		namespace := context.namespace(context.inFanoutScope(node.ID), context.hasTriggerStart())
		if err := compileNode(resolved, node, namespace, context); err != nil {
			return nil, fmt.Errorf("loop: hydrate executed node %s: %w", node.ID, err)
		}
	}
	if err := compileSubLoopBodies(resolved, definition, nil); err != nil {
		return nil, fmt.Errorf("loop: hydrate executed sub-loops: %w", err)
	}
	return resolved, nil
}

func resolvedTemplateSources(resolved *ResolvedDefinition) map[string]string {
	sources := map[string]string{}
	if resolved == nil {
		return sources
	}
	for key, template := range resolved.Templates {
		if template != nil {
			sources[key] = template.Raw
		}
	}
	return sources
}

func resolvedConditionSources(resolved *ResolvedDefinition) map[string]string {
	sources := map[string]string{}
	if resolved == nil {
		return sources
	}
	for key, condition := range resolved.Conditions {
		if condition != nil {
			sources[key] = condition.Raw
		}
	}
	return sources
}
