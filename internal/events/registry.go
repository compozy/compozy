// Package events owns canonical runtime event names and metadata shared by
// producers, logs, notifications, and contract tests.
package events

import (
	"fmt"
	"strings"
)

// Outcome classifies an event for log filtering and notification policy.
type Outcome string

const (
	OutcomeInfo    Outcome = "info"
	OutcomeSuccess Outcome = "success"
	OutcomeFailure Outcome = "failure"
	OutcomeWarning Outcome = "warning"
)

// Metadata is the canonical registry entry for one event name.
type Metadata struct {
	Name                 string
	Family               string
	Component            string
	Outcome              Outcome
	EmitsToLogs          bool
	NotificationEligible bool
	GlobalScope          bool
}

var registryByName = mustBuildRegistry(registryEntries)

// ValidatePublicName rejects deleted or unsupported public event families.
func ValidatePublicName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return nil
	}
	if strings.HasPrefix(trimmed, "task_run.") {
		return fmt.Errorf("events: %q is not a public event family; use task.run_* events", trimmed)
	}
	return nil
}

// ValidOutcome reports whether value is one of the canonical event outcomes.
func ValidOutcome(value string) bool {
	switch Outcome(strings.TrimSpace(value)) {
	case "", OutcomeInfo, OutcomeSuccess, OutcomeFailure, OutcomeWarning:
		return true
	default:
		return false
	}
}

// ValidComponent reports whether component is present in the registry.
func ValidComponent(component string) bool {
	component = strings.TrimSpace(component)
	if component == "" {
		return true
	}
	for _, meta := range registryEntries {
		if meta.Component == component {
			return true
		}
	}
	return false
}

func mustBuildRegistry(entries []Metadata) map[string]Metadata {
	registry := make(map[string]Metadata, len(entries))
	for _, entry := range entries {
		name := strings.TrimSpace(entry.Name)
		if name == "" {
			panic("events: registry entry missing name")
		}
		if _, exists := registry[name]; exists {
			panic("events: duplicate registry entry " + name)
		}
		if strings.TrimSpace(entry.Family) == "" {
			panic("events: registry entry missing family for " + name)
		}
		if strings.TrimSpace(entry.Component) == "" {
			panic("events: registry entry missing component for " + name)
		}
		if !ValidOutcome(string(entry.Outcome)) || entry.Outcome == "" {
			panic("events: registry entry has invalid outcome for " + name)
		}
		entry.Name = name
		entry.Family = strings.TrimSpace(entry.Family)
		entry.Component = strings.TrimSpace(entry.Component)
		registry[name] = entry
	}
	return registry
}
