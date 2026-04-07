package setup

import (
	"fmt"
	"slices"
	"strings"
)

type selectByNameConfig[T any] struct {
	subject      string
	emptyLabel   string
	invalidLabel string
	getName      func(T) string
	normalize    func(string) string
	less         func(T, T) int
}

func selectByName[T any](all []T, names []string, cfg selectByNameConfig[T]) ([]T, error) {
	if len(names) == 0 {
		return nil, fmt.Errorf("select %s: no %s requested", cfg.subject, cfg.emptyLabel)
	}

	index := make(map[string]T, len(all))
	for _, item := range all {
		index[cfg.getName(item)] = item
	}

	selected := make([]T, 0, len(names))
	seen := make(map[string]struct{}, len(names))
	var invalid []string
	for _, name := range names {
		canonical := cfg.normalize(name)
		item, ok := index[canonical]
		if !ok {
			invalid = append(invalid, name)
			continue
		}

		itemName := cfg.getName(item)
		if _, ok := seen[itemName]; ok {
			continue
		}
		seen[itemName] = struct{}{}
		selected = append(selected, item)
	}

	if len(invalid) > 0 {
		slices.Sort(invalid)
		return nil, fmt.Errorf("select %s: invalid %s: %s", cfg.subject, cfg.invalidLabel, strings.Join(invalid, ", "))
	}

	slices.SortFunc(selected, cfg.less)
	return selected, nil
}
