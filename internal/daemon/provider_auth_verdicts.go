package daemon

import (
	"maps"
	"reflect"
	"slices"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

// changedProviderNames lists providers whose configured block was added, removed, or edited, so
// their last live sign-in verdict is dropped when the new configuration applies.
func changedProviderNames(previous, next *compozyconfig.Config) []string {
	names := make(map[string]struct{}, len(previous.Providers)+len(next.Providers))
	for name := range maps.Keys(previous.Providers) {
		names[name] = struct{}{}
	}
	for name := range maps.Keys(next.Providers) {
		names[name] = struct{}{}
	}
	changed := make([]string, 0)
	for _, name := range slices.Sorted(maps.Keys(names)) {
		before, hadBefore := previous.Providers[name]
		after, hasAfter := next.Providers[name]
		if hadBefore != hasAfter || !reflect.DeepEqual(before, after) {
			changed = append(changed, name)
		}
	}
	return changed
}
