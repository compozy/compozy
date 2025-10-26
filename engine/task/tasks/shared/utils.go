package shared

import (
	"fmt"
	"sort"
)

// SortedMapKeys returns a sorted slice of keys from the given map
// Uses fmt.Sprint for string representation to ensure consistent ordering
func SortedMapKeys[K comparable, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return fmt.Sprint(keys[i]) < fmt.Sprint(keys[j])
	})
	return keys
}

// IterateSortedMap iterates over a map in sorted key order
// This ensures deterministic iteration for Temporal workflow compatibility
func IterateSortedMap[K comparable, V any](
	m map[K]V,
	fn func(K, V) error,
) error {
	keys := SortedMapKeys(m)
	for _, k := range keys {
		if err := fn(k, m[k]); err != nil {
			return err
		}
	}
	return nil
}
