package shared

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSortedMapKeys(t *testing.T) {
	t.Run("Should sort string keys alphabetically", func(t *testing.T) {
		m := map[string]int{
			"zebra":  1,
			"apple":  2,
			"banana": 3,
		}
		keys := SortedMapKeys(m)
		expected := []string{"apple", "banana", "zebra"}
		require.Equal(t, expected, keys)
	})

	t.Run("Should sort integer keys using string representation", func(t *testing.T) {
		m := map[int]string{
			10: "ten",
			1:  "one",
			5:  "five",
		}
		keys := SortedMapKeys(m)
		// fmt.Sprint converts to: "1", "10", "5" -> sorted as "1", "10", "5"
		expected := []int{1, 10, 5}
		require.Equal(t, expected, keys)
	})

	t.Run("Should handle empty map", func(t *testing.T) {
		m := map[string]int{}
		keys := SortedMapKeys(m)
		require.Empty(t, keys)
	})

	t.Run("Should handle single key map", func(t *testing.T) {
		m := map[string]int{"single": 1}
		keys := SortedMapKeys(m)
		expected := []string{"single"}
		require.Equal(t, expected, keys)
	})

	t.Run("Should provide deterministic ordering across calls", func(t *testing.T) {
		m := map[string]int{
			"gamma": 3,
			"alpha": 1,
			"beta":  2,
			"delta": 4,
		}

		// Call multiple times to ensure consistent ordering
		keys1 := SortedMapKeys(m)
		keys2 := SortedMapKeys(m)
		keys3 := SortedMapKeys(m)

		require.Equal(t, keys1, keys2)
		require.Equal(t, keys2, keys3)
		expected := []string{"alpha", "beta", "delta", "gamma"}
		require.Equal(t, expected, keys1)
	})
}

func TestIterateSortedMap(t *testing.T) {
	t.Run("Should iterate in sorted key order", func(t *testing.T) {
		m := map[string]int{
			"zebra":  1,
			"apple":  2,
			"banana": 3,
		}

		var keys []string
		var values []int

		err := IterateSortedMap(m, func(k string, v int) error {
			keys = append(keys, k)
			values = append(values, v)
			return nil
		})

		require.NoError(t, err)
		require.Equal(t, []string{"apple", "banana", "zebra"}, keys)
		require.Equal(t, []int{2, 3, 1}, values)
	})

	t.Run("Should handle empty map", func(t *testing.T) {
		m := map[string]int{}
		var called bool

		err := IterateSortedMap(m, func(_ string, _ int) error {
			called = true
			return nil
		})

		require.NoError(t, err)
		require.False(t, called)
	})

	t.Run("Should propagate error from iterator function", func(t *testing.T) {
		m := map[string]int{"key": 1}
		expectedErr := fmt.Errorf("test error")

		err := IterateSortedMap(m, func(_ string, _ int) error {
			return expectedErr
		})

		require.Equal(t, expectedErr, err)
	})

	t.Run("Should stop iteration on first error", func(t *testing.T) {
		m := map[string]int{
			"a": 1,
			"b": 2,
			"c": 3,
		}

		var visitedKeys []string
		expectedErr := fmt.Errorf("stop here")

		err := IterateSortedMap(m, func(k string, _ int) error {
			visitedKeys = append(visitedKeys, k)
			if k == "b" {
				return expectedErr
			}
			return nil
		})

		require.Equal(t, expectedErr, err)
		require.Equal(t, []string{"a", "b"}, visitedKeys)
	})

	t.Run("Should provide deterministic iteration order", func(t *testing.T) {
		m := map[string]int{
			"gamma": 3,
			"alpha": 1,
			"beta":  2,
			"delta": 4,
		}

		// Test multiple iterations to ensure consistency
		var results [][]string
		for i := 0; i < 3; i++ {
			var keys []string
			err := IterateSortedMap(m, func(k string, _ int) error {
				keys = append(keys, k)
				return nil
			})
			require.NoError(t, err)
			results = append(results, keys)
		}

		// All iterations should produce the same order
		expected := []string{"alpha", "beta", "delta", "gamma"}
		for _, result := range results {
			require.Equal(t, expected, result)
		}
	})
}
