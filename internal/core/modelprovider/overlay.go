package modelprovider

import (
	"fmt"
	"slices"
	"strings"
	"sync"
)

// OverlayEntry captures one declarative model alias overlay.
type OverlayEntry struct {
	Name        string
	DisplayName string
	Target      string
	Metadata    map[string]string
}

// CatalogEntry is the public model alias catalog shape.
type CatalogEntry struct {
	Name        string
	DisplayName string
	Target      string
}

type snapshot struct {
	entries map[string]CatalogEntry
	order   []string
}

var (
	activeMu sync.RWMutex
	active   *snapshot
)

// ActivateOverlay installs one command-scoped model alias overlay.
func ActivateOverlay(entries []OverlayEntry) (func(), error) {
	next, err := buildSnapshot(entries)
	if err != nil {
		return nil, err
	}

	activeMu.Lock()
	previous := active
	active = next
	activeMu.Unlock()

	return func() {
		activeMu.Lock()
		active = previous
		activeMu.Unlock()
	}, nil
}

// ResolveAlias returns the canonical model string for one alias.
func ResolveAlias(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}

	activeMu.RLock()
	current := active
	activeMu.RUnlock()
	if current == nil {
		return trimmed
	}

	entry, ok := current.entries[strings.ToLower(trimmed)]
	if !ok || strings.TrimSpace(entry.Target) == "" {
		return trimmed
	}
	return entry.Target
}

// Catalog returns the active model alias catalog.
func Catalog() []CatalogEntry {
	activeMu.RLock()
	current := active
	activeMu.RUnlock()
	if current == nil {
		return nil
	}

	entries := make([]CatalogEntry, 0, len(current.order))
	for _, name := range current.order {
		entry, ok := current.entries[name]
		if ok {
			entries = append(entries, entry)
		}
	}
	return entries
}

func buildSnapshot(entries []OverlayEntry) (*snapshot, error) {
	if len(entries) == 0 {
		return nil, nil
	}

	result := &snapshot{
		entries: make(map[string]CatalogEntry, len(entries)),
		order:   make([]string, 0, len(entries)),
	}
	for _, entry := range entries {
		name := strings.ToLower(strings.TrimSpace(entry.Name))
		if name == "" {
			return nil, fmt.Errorf("declare model overlay: provider name is required")
		}
		target := strings.TrimSpace(entry.Target)
		if target == "" {
			return nil, fmt.Errorf("declare model overlay %q: target model is required", entry.Name)
		}
		if _, ok := result.entries[name]; !ok {
			result.order = append(result.order, name)
		}
		result.entries[name] = CatalogEntry{
			Name:        strings.TrimSpace(entry.Name),
			DisplayName: strings.TrimSpace(entry.DisplayName),
			Target:      target,
		}
	}
	slices.Sort(result.order)
	return result, nil
}
