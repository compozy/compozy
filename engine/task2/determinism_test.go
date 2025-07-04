package task2

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/task2/shared"
)

func TestSortedMapUtilities(t *testing.T) {
	t.Run("Should sort map keys alphabetically", func(t *testing.T) {
		m := map[string]int{
			"zebra":  1,
			"apple":  2,
			"banana": 3,
		}
		keys := shared.SortedMapKeys(m)
		expected := []string{"apple", "banana", "zebra"}
		require.Equal(t, expected, keys)
	})

	t.Run("Should handle maps with non-string keys using fmt.Sprint conversion", func(t *testing.T) {
		m := map[int]string{
			10: "ten",
			1:  "one",
			5:  "five",
		}
		keys := shared.SortedMapKeys(m)
		// fmt.Sprint converts to: "1", "10", "5" -> sorted as "1", "10", "5"
		expected := []int{1, 10, 5}
		require.Equal(t, expected, keys)
	})

	t.Run("Should provide deterministic iteration across multiple calls", func(t *testing.T) {
		m := map[string]int{
			"gamma": 3,
			"alpha": 1,
			"beta":  2,
			"delta": 4,
		}

		var results [][]string
		for i := 0; i < 3; i++ {
			var keys []string
			err := shared.IterateSortedMap(m, func(k string, _ int) error {
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

	t.Run("Should handle empty maps without errors", func(t *testing.T) {
		m := map[string]int{}
		keys := shared.SortedMapKeys(m)
		require.Empty(t, keys)

		var called bool
		err := shared.IterateSortedMap(m, func(_ string, _ int) error {
			called = true
			return nil
		})
		require.NoError(t, err)
		require.False(t, called)
	})

	t.Run("Should maintain deterministic order across multiple sequential calls", func(t *testing.T) {
		m := map[string]int{
			"concurrent": 1,
			"access":     2,
			"test":       3,
		}

		// Multiple sequential calls should produce identical results
		results := make([][]string, 5)
		for i := 0; i < 5; i++ {
			var keys []string
			err := shared.IterateSortedMap(m, func(k string, _ int) error {
				keys = append(keys, k)
				return nil
			})
			require.NoError(t, err)
			results[i] = keys
		}

		// All should be identical and deterministic
		expected := []string{"access", "concurrent", "test"}
		for _, result := range results {
			require.Equal(t, expected, result)
		}
	})
}

func TestDeterministicMapIteration(t *testing.T) {
	t.Run("Should maintain deterministic order for various map types", func(t *testing.T) {
		// Test with map[string]any (common in configs)
		configMap := map[string]any{
			"zebra": "value_z",
			"alpha": "value_a",
			"beta":  42,
			"gamma": true,
		}

		results := make([][]string, 5)
		for i := 0; i < 5; i++ {
			var keys []string
			keyList := shared.SortedMapKeys(configMap)
			keys = append(keys, keyList...)
			results[i] = keys
		}

		// All results should be identical
		expected := []string{"alpha", "beta", "gamma", "zebra"}
		for i, result := range results {
			require.Equal(t, expected, result, "Iteration %d should be deterministic", i)
		}
	})

	t.Run("Should ensure Temporal workflow replay determinism", func(t *testing.T) {
		// Simulate workflow state map
		workflowTasks := map[string]any{
			"task_zebra": map[string]any{"status": "completed"},
			"task_alpha": map[string]any{"status": "running"},
			"task_beta":  map[string]any{"status": "pending"},
		}

		// Multiple "replays" should produce identical order
		var replays [][]string
		for replay := 0; replay < 10; replay++ {
			var taskOrder []string
			keys := shared.SortedMapKeys(workflowTasks)
			taskOrder = append(taskOrder, keys...)
			replays = append(replays, taskOrder)
		}

		// All replays should have identical task processing order
		expectedOrder := []string{"task_alpha", "task_beta", "task_zebra"}
		for i, replay := range replays {
			require.Equal(t, expectedOrder, replay,
				"Replay %d should have deterministic task order for Temporal compatibility", i)
		}
	})
}
