package model

import (
	"fmt"
	"strings"
)

// Built-in activation event names the runtime emits for triggers. Hook
// completions use the `hook.<hook_name>.completed` pattern and extension
// events use the open `ext.*` namespace; every other name must match one of
// these constants or no producer will ever fire the trigger.
const (
	TriggerEventSessionCreated     = "session.created"
	TriggerEventSessionStopped     = "session.stopped"
	TriggerEventMemoryConsolidated = "memory.consolidated"
	TriggerEventWebhook            = "webhook"

	triggerEventHookPrefix      = "hook."
	triggerEventHookSuffix      = ".completed"
	triggerEventExtensionPrefix = "ext."
)

// ValidateTriggerEvent rejects event names no activation producer emits, so a
// trigger cannot be created permanently dead. Built-in names are closed; hook
// completions require a hook name; extension events require a suffix after
// the `ext.` namespace prefix. Surrounding whitespace is rejected so stored
// triggers stay canonical, matching the extension ingress contract.
func ValidateTriggerEvent(event string, path string) error {
	field := nestedPath(path, "event")
	if event != strings.TrimSpace(event) {
		return fmt.Errorf("%s must not contain surrounding whitespace: %q", field, event)
	}
	switch event {
	case TriggerEventSessionCreated,
		TriggerEventSessionStopped,
		TriggerEventMemoryConsolidated,
		TriggerEventWebhook:
		return nil
	}
	if rest, ok := strings.CutPrefix(event, triggerEventHookPrefix); ok {
		hookName, isCompletion := strings.CutSuffix(rest, triggerEventHookSuffix)
		if isCompletion && strings.TrimSpace(hookName) != "" {
			return nil
		}
		return fmt.Errorf(
			"%s %q must name a hook: use hook.<hook_name>.completed",
			field,
			event,
		)
	}
	if rest, ok := strings.CutPrefix(event, triggerEventExtensionPrefix); ok {
		if strings.TrimSpace(rest) != "" {
			return nil
		}
		return fmt.Errorf(
			"%s %q must name an extension event after the %q prefix",
			field,
			event,
			triggerEventExtensionPrefix,
		)
	}
	return fmt.Errorf(
		"%s %q has no activation producer; supported events are %s, %s, %s, hook.<hook_name>.completed, %s, and ext.*",
		field,
		event,
		TriggerEventSessionCreated,
		TriggerEventSessionStopped,
		TriggerEventMemoryConsolidated,
		TriggerEventWebhook,
	)
}
