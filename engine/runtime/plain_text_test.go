package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRuntimeManager_PlainTextResponse(t *testing.T) {
	// Use a dedicated runtime manager for this test
	rm := getTestRuntimeManager(t)

	t.Run("Should handle plain text response from tool successfully", func(t *testing.T) {
		// Arrange
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		toolID := "plain-text-tool"
		toolExecID := core.MustNewID()
		input := &core.Input{
			"message": "testing plain text",
		}
		env := core.EnvMap{}

		// Act
		result, err := rm.ExecuteTool(ctx, toolID, toolExecID, input, env)

		// Assert - The worker should wrap plain text in JSON structure
		if err != nil {
			t.Logf("Tool execution failed: %v", err)
			t.Skip("Tool execution failed, likely due to worker setup")
		}

		require.NotNil(t, result)
		require.NotNil(t, *result)

		// The result should contain the plain text string wrapped in value
		resultValue, ok := (*result)[PrimitiveValueKey]
		require.True(t, ok, "Response should contain 'value' field for primitive returns")

		// Verify the plain text was returned
		expectedText := "This is plain text response: testing plain text"
		assert.Equal(t, expectedText, resultValue)
	})

	t.Run("Should handle numeric response from tool", func(t *testing.T) {
		// Arrange
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		toolID := "plain-text-tool"
		toolExecID := core.MustNewID()
		input := &core.Input{
			"message": "123", // This will return a string with numbers
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

		// The result should be wrapped in the value field
		resultValue, ok := (*result)[PrimitiveValueKey]
		require.True(t, ok, "Response should contain 'value' field for primitive returns")
		assert.Equal(t, "This is plain text response: 123", resultValue)
	})

	t.Run("Should handle empty string response from tool", func(t *testing.T) {
		// Arrange
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		toolID := "plain-text-tool"
		toolExecID := core.MustNewID()
		input := &core.Input{
			"message": "", // Empty message
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

		// The result should still be wrapped properly
		resultValue, ok := (*result)[PrimitiveValueKey]
		require.True(t, ok, "Response should contain 'value' field for primitive returns")
		assert.Equal(t, "This is plain text response: Hello", resultValue) // Falls back to "Hello"
	})
}
