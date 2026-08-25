package hooks

import "fmt"

var allHookEvents, hookEventSpecs = buildHookEventCatalog()

func buildHookEventCatalog() ([]HookEvent, map[HookEvent]hookEventDefinition) {
	definitions := make([]hookEventDefinition, 0, len(baseHookEventDefinitions)+32)
	definitions = append(definitions, baseHookEventDefinitions...)
	definitions = append(definitions, sessionAttentionHookEventDefinitions()...)
	definitions = append(definitions, networkHookEventDefinitions()...)
	definitions = append(definitions, windowManagerHookEventDefinitions()...)
	definitions = append(definitions, worktreeHookEventDefinitions()...)
	definitions = append(definitions, callHookEventDefinitions()...)

	events := make([]HookEvent, 0, len(definitions))
	specs := make(map[HookEvent]hookEventDefinition, len(definitions))
	for _, definition := range definitions {
		events = append(events, definition.event)
		specs[definition.event] = definition
	}
	return events, specs
}

// AllHookEvents returns the full taxonomy in deterministic order.
func AllHookEvents() []HookEvent {
	return append([]HookEvent(nil), allHookEvents...)
}

// String returns the literal hook event value.
func (e HookEvent) String() string {
	return string(e)
}

// Family reports the taxonomy family for the event.
func (e HookEvent) Family() HookEventFamily {
	spec, ok := hookEventSpecs[e]
	if !ok {
		return ""
	}
	return spec.family
}

// SyncEligible reports whether the event accepts sync hooks.
func (e HookEvent) SyncEligible() bool {
	spec, ok := hookEventSpecs[e]
	return ok && spec.syncEligible
}

// Validate ensures the event is part of the supported taxonomy.
func (e HookEvent) Validate() error {
	if _, ok := hookEventSpecs[e]; !ok {
		return fmt.Errorf("hooks: invalid hook event %q", e)
	}
	return nil
}
