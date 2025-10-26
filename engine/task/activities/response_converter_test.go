package activities

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/tasks/shared"
)

func TestResponseConverter_ConvertToMainTaskResponse(t *testing.T) {
	converter := NewResponseConverter()

	t.Run("Should extract existing MainTaskResponse from Response field", func(t *testing.T) {
		// Arrange
		state := &task.State{
			TaskID:     "test-task",
			TaskExecID: core.MustNewID(),
		}
		expectedResponse := &task.MainTaskResponse{
			State: state,
		}
		result := &shared.ResponseOutput{
			State:    state,
			Response: expectedResponse,
		}

		// Act
		response := converter.ConvertToMainTaskResponse(result)

		// Assert
		assert.Equal(t, expectedResponse, response)
		assert.Equal(t, state, response.State)
	})

	t.Run("Should create MainTaskResponse from state when Response field is nil", func(t *testing.T) {
		// Arrange
		state := &task.State{
			TaskID:     "test-task",
			TaskExecID: core.MustNewID(),
		}
		result := &shared.ResponseOutput{
			State:    state,
			Response: nil,
		}

		// Act
		response := converter.ConvertToMainTaskResponse(result)

		// Assert
		require.NotNil(t, response)
		assert.Equal(t, state, response.State)
	})

	t.Run("Should create MainTaskResponse from state when Response field is wrong type", func(t *testing.T) {
		// Arrange
		state := &task.State{
			TaskID:     "test-task",
			TaskExecID: core.MustNewID(),
		}
		result := &shared.ResponseOutput{
			State:    state,
			Response: "wrong type",
		}

		// Act
		response := converter.ConvertToMainTaskResponse(result)

		// Assert
		require.NotNil(t, response)
		assert.Equal(t, state, response.State)
	})
}

func TestResponseConverter_ConvertToCollectionResponse(t *testing.T) {
	converter := NewResponseConverter()
	ctx := t.Context()

	t.Run("Should convert with collection metadata from output", func(t *testing.T) {
		// Arrange
		taskExecID := core.MustNewID()
		output := core.Output{
			"collection_metadata": map[string]any{
				"item_count":    5,
				"skipped_count": 2,
			},
		}
		state := &task.State{
			TaskID:     "test-collection",
			TaskExecID: taskExecID,
			Output:     &output,
		}
		result := &shared.ResponseOutput{
			State: state,
		}

		// Act
		response := converter.ConvertToCollectionResponse(ctx, result, nil, nil, nil)

		// Assert
		require.NotNil(t, response)
		assert.Equal(t, state, response.State)
		assert.Equal(t, 5, response.ItemCount)
		assert.Equal(t, 2, response.SkippedCount)
	})

	t.Run("Should handle missing metadata gracefully", func(t *testing.T) {
		// Arrange
		state := &task.State{
			TaskID:     "test-collection",
			TaskExecID: core.MustNewID(),
		}
		result := &shared.ResponseOutput{
			State: state,
		}

		// Act
		response := converter.ConvertToCollectionResponse(ctx, result, nil, nil, nil)

		// Assert
		require.NotNil(t, response)
		assert.Equal(t, state, response.State)
		assert.Equal(t, 0, response.ItemCount)
		assert.Equal(t, 0, response.SkippedCount)
	})
}
