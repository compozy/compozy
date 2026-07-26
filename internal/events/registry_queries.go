package events

import (
	"slices"
	"strings"
)

// All returns all canonical registry entries sorted by event name.
func All() []Metadata {
	entries := append([]Metadata(nil), registryEntries...)
	slices.SortFunc(entries, func(a Metadata, b Metadata) int {
		return strings.Compare(a.Name, b.Name)
	})
	return entries
}

// Lookup returns metadata for a canonical event name.
func Lookup(name string) (Metadata, bool) {
	meta, ok := registryByName[strings.TrimSpace(name)]
	return meta, ok
}

// ComponentFor returns the registered component for an event name.
func ComponentFor(name string) string {
	meta, ok := Lookup(name)
	if !ok {
		return ""
	}
	return meta.Component
}

// OutcomeFor returns the registered outcome for an event name, defaulting to info.
func OutcomeFor(name string) Outcome {
	meta, ok := Lookup(name)
	if !ok {
		return OutcomeInfo
	}
	return meta.Outcome
}

// AllowsGlobalScope reports whether a summary event may be emitted without a session.
func AllowsGlobalScope(name string) bool {
	meta, ok := Lookup(name)
	return ok && meta.GlobalScope
}

// NamesForComponent returns canonical event names registered for component.
func NamesForComponent(component string) []string {
	component = strings.TrimSpace(component)
	if component == "" {
		return nil
	}
	names := make([]string, 0)
	for _, meta := range registryEntries {
		if meta.Component == component {
			names = append(names, meta.Name)
		}
	}
	slices.Sort(names)
	return names
}

// NamesForOutcomes returns canonical event names matching any requested outcome.
func NamesForOutcomes(outcomes ...Outcome) []string {
	if len(outcomes) == 0 {
		return nil
	}
	allowed := make(map[Outcome]struct{}, len(outcomes))
	for _, outcome := range outcomes {
		allowed[outcome] = struct{}{}
	}
	names := make([]string, 0)
	for _, meta := range registryEntries {
		if _, ok := allowed[meta.Outcome]; ok {
			names = append(names, meta.Name)
		}
	}
	slices.Sort(names)
	return names
}
