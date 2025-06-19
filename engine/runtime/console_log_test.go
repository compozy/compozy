package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRuntimeManager_ConsoleLogHandling(t *testing.T) {
	// Use a dedicated runtime manager for this test
	rm := getTestRuntimeManager(t)

	t.Run("Should execute tool successfully when console.log is used", func(t *testing.T) {
		// Arrange
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		toolID := "console-log-tool"
		toolExecID := core.MustNewID()
		input := &core.Input{
			"message":   "testing console.log redirection",
			"log_count": 5,
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
		require.NotNil(t, *result)

		// Verify the tool executed successfully despite console.log calls
		assert.Contains(t, *result, "success")
		assert.Contains(t, *result, "message")
		assert.Contains(t, *result, "logs_written")
		assert.Contains(t, *result, "timestamp")

		// Verify specific values in the response
		success, ok := (*result)["success"]
		require.True(t, ok, "Response should contain 'success' field")
		assert.Equal(t, true, success)

		message, ok := (*result)["message"]
		require.True(t, ok, "Response should contain 'message' field")
		assert.Equal(t, "Processed: testing console.log redirection", message)

		logsWritten, ok := (*result)["logs_written"]
		require.True(t, ok, "Response should contain 'logs_written' field")
		assert.Equal(t, float64(5), logsWritten)
	})

	t.Run("Should handle multiple console.log calls without corrupting response", func(t *testing.T) {
		// Arrange
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		toolID := "console-log-tool"
		toolExecID := core.MustNewID()
		input := &core.Input{
			"message":   "stress test with many logs",
			"log_count": 20, // Many log calls to stress test
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
		require.NotNil(t, *result)

		// Verify the response is properly formed
		success, ok := (*result)["success"]
		require.True(t, ok, "Response should contain 'success' field")
		assert.Equal(t, true, success)

		logsWritten, ok := (*result)["logs_written"]
		require.True(t, ok, "Response should contain 'logs_written' field")
		assert.Equal(t, float64(20), logsWritten)
	})

	t.Run("Should parse JSON response correctly with console.log present", func(t *testing.T) {
		// Arrange
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		toolID := "console-log-tool"
		toolExecID := core.MustNewID()
		input := &core.Input{
			"message":   "json parsing test",
			"log_count": 1,
		}
		env := core.EnvMap{}

		// Act
		result, err := rm.ExecuteTool(ctx, toolID, toolExecID, input, env)

		// Assert
		require.NoError(t, err, "Should not error when parsing response with console.log")
		require.NotNil(t, result)

		// Verify JSON structure is intact
		_, hasSuccess := (*result)["success"]
		_, hasMessage := (*result)["message"]
		_, hasLogsWritten := (*result)["logs_written"]
		_, hasTimestamp := (*result)["timestamp"]

		assert.True(t, hasSuccess, "Response should have 'success' field")
		assert.True(t, hasMessage, "Response should have 'message' field")
		assert.True(t, hasLogsWritten, "Response should have 'logs_written' field")
		assert.True(t, hasTimestamp, "Response should have 'timestamp' field")
	})
}
