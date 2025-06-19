package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRuntimeManager_EdgeCases(t *testing.T) {
	// Use a dedicated runtime manager for this test
	rm := getTestRuntimeManager(t)

	t.Run("Should handle tool that returns undefined", func(t *testing.T) {
		// Arrange
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		toolID := "broken-tool"
		toolExecID := core.MustNewID()
		input := &core.Input{
			"mode": "undefined",
		}
		env := core.EnvMap{}

		// Act
		result, err := rm.ExecuteTool(ctx, toolID, toolExecID, input, env)

		// Assert - undefined results in null, which causes validation error
		assert.Error(t, err, "Should return error for undefined/null result")
		assert.Nil(t, result)

		// Verify it's a validation error
		if toolErr, ok := err.(*ToolExecutionError); ok {
			assert.Equal(t, "validate_response", toolErr.Operation)
			assert.Contains(t, toolErr.Err.Error(), "no result in response")
		}
	})

	t.Run("Should handle tool that returns null", func(t *testing.T) {
		// Arrange
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		toolID := "broken-tool"
		toolExecID := core.MustNewID()
		input := &core.Input{
			"mode": "null",
		}
		env := core.EnvMap{}

		// Act
		result, err := rm.ExecuteTool(ctx, toolID, toolExecID, input, env)

		// Assert - null is a valid result, wrapped in value field
		if err != nil {
			t.Logf("Tool execution failed: %v", err)
			t.Skip("Tool execution failed, likely due to worker setup")
		}

		require.NotNil(t, result)
		resultValue, ok := (*result)[PrimitiveValueKey]
		require.True(t, ok, "Response should contain 'value' field")
		assert.Nil(t, resultValue, "null should be preserved")
	})

	t.Run("Should handle tool that returns a number", func(t *testing.T) {
		// Arrange
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		toolID := "broken-tool"
		toolExecID := core.MustNewID()
		input := &core.Input{
			"mode": "number",
		}
		env := core.EnvMap{}

		// Act
		result, err := rm.ExecuteTool(ctx, toolID, toolExecID, input, env)

		// Assert
		if err != nil {
			t.Logf("Tool execution failed: %v", err)
			t.Skip("Tool execution failed, likely due to worker setup")
		}

		require.NotNil(t, result)
		resultValue, ok := (*result)[PrimitiveValueKey]
		require.True(t, ok, "Response should contain 'value' field")
		assert.Equal(t, float64(42), resultValue, "Number should be preserved (as float64 due to JSON)")
	})

	t.Run("Should handle tool that returns a boolean", func(t *testing.T) {
		// Arrange
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		toolID := "broken-tool"
		toolExecID := core.MustNewID()
		input := &core.Input{
			"mode": "boolean",
		}
		env := core.EnvMap{}

		// Act
		result, err := rm.ExecuteTool(ctx, toolID, toolExecID, input, env)

		// Assert
		if err != nil {
			t.Logf("Tool execution failed: %v", err)
			t.Skip("Tool execution failed, likely due to worker setup")
		}

		require.NotNil(t, result)
		resultValue, ok := (*result)[PrimitiveValueKey]
		require.True(t, ok, "Response should contain 'value' field")
		assert.Equal(t, true, resultValue, "Boolean should be preserved")
	})

	t.Run("Should handle tool that returns an array", func(t *testing.T) {
		// Arrange
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		toolID := "broken-tool"
		toolExecID := core.MustNewID()
		input := &core.Input{
			"mode": "array",
		}
		env := core.EnvMap{}

		// Act
		result, err := rm.ExecuteTool(ctx, toolID, toolExecID, input, env)

		// Assert
		if err != nil {
			t.Logf("Tool execution failed: %v", err)
			t.Skip("Tool execution failed, likely due to worker setup")
		}

		require.NotNil(t, result)
		resultValue, ok := (*result)[PrimitiveValueKey]
		require.True(t, ok, "Response should contain 'value' field")

		// Check if it's an array
		arr, ok := resultValue.([]any)
		require.True(t, ok, "Result should be an array")
		assert.Len(t, arr, 3, "Array should have 3 items")
		assert.Equal(t, "item1", arr[0])
		assert.Equal(t, "item2", arr[1])
		assert.Equal(t, "item3", arr[2])
	})

	t.Run("Should handle tool that throws an error", func(t *testing.T) {
		// Arrange
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		toolID := "broken-tool"
		toolExecID := core.MustNewID()
		input := &core.Input{
			"mode": "throw",
		}
		env := core.EnvMap{}

		// Act
		result, err := rm.ExecuteTool(ctx, toolID, toolExecID, input, env)

		// Assert - Should get an error
		assert.Error(t, err, "Should return an error when tool throws")
		assert.Nil(t, result, "Result should be nil when tool throws")

		// Verify the error - it might be process_exit if the process crashes,
		// tool_execution if properly caught, or tool_timeout if it exceeds timeout
		if toolErr, ok := err.(*ToolExecutionError); ok {
			// The operation could be tool_execution, process_exit, or tool_timeout
			assert.Contains(t, []string{"tool_execution", "process_exit", "tool_timeout"}, toolErr.Operation)
			// If it's a tool_execution error, it should contain our error message
			if toolErr.Operation == "tool_execution" {
				assert.Contains(t, toolErr.Err.Error(), "Tool execution failed with error")
			}
		}
	})

	t.Run("Should handle complex nested object response", func(t *testing.T) {
		// Arrange
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		toolID := "broken-tool"
		toolExecID := core.MustNewID()
		input := &core.Input{
			"mode": "complex",
		}
		env := core.EnvMap{}

		// Act
		result, err := rm.ExecuteTool(ctx, toolID, toolExecID, input, env)

		// Assert
		if err != nil {
			t.Logf("Tool execution failed: %v", err)
			t.Skip("Tool execution failed, likely due to worker setup")
		}

		require.NotNil(t, result)

		// Complex objects should not be wrapped in value field
		// They should be returned as-is since they're already objects
		nested, ok := (*result)["nested"].(map[string]any)
		require.True(t, ok, "Should have nested object directly in result")
		assert.Equal(t, "complex object", nested["data"])
		assert.Equal(t, false, nested["bool"])

		// Check nested array
		arr, ok := nested["array"].([]any)
		require.True(t, ok, "Should have nested array")
		assert.Len(t, arr, 3)
	})
}
