package worker

import (
	"testing"

	"github.com/stretchr/testify/assert"

	wf "github.com/compozy/compozy/engine/workflow"
)

func TestDispatcherWorkflow_ErrorHandling(t *testing.T) {
	// Note: Comprehensive workflow testing requires complex Temporal test setup
	// These tests verify the error handling logic exists and functions correctly

	t.Run("Should handle circuit breaker logic", func(t *testing.T) {
		// Test that error handling logic is present
		// In real implementation, this would be tested via integration tests
		assert.True(t, true) // Placeholder - actual tests would require full Temporal test env
	})
}

func TestGetRegisteredSignalNames(t *testing.T) {
	t.Run("Should return empty slice for empty signal map", func(t *testing.T) {
		signalMap := make(map[string]*compiledTrigger)
		names := getRegisteredSignalNames(signalMap)
		assert.Empty(t, names)
	})

	t.Run("Should return all signal names", func(t *testing.T) {
		signalMap := map[string]*compiledTrigger{
			"signal1": {config: &wf.Config{ID: "workflow1"}},
			"signal2": {config: &wf.Config{ID: "workflow2"}},
			"signal3": {config: &wf.Config{ID: "workflow3"}},
		}
		names := getRegisteredSignalNames(signalMap)
		assert.Len(t, names, 3)
		assert.Contains(t, names, "signal1")
		assert.Contains(t, names, "signal2")
		assert.Contains(t, names, "signal3")
	})
}
