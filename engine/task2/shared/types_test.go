package shared

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/compozy/compozy/engine/core"
)

func TestDeferredOutputConfig_Structure(t *testing.T) {
	t.Run("Should create deferred config with reason", func(t *testing.T) {
		// Arrange & Act
		config := &DeferredOutputConfig{
			ShouldDefer: true,
			Reason:      "Collection tasks require deferred processing",
		}

		// Assert
		assert.True(t, config.ShouldDefer)
		assert.Equal(t, "Collection tasks require deferred processing", config.Reason)
	})

	t.Run("Should handle non-deferred config", func(t *testing.T) {
		// Arrange & Act
		config := &DeferredOutputConfig{
			ShouldDefer: false,
		}

		// Assert
		assert.False(t, config.ShouldDefer)
		assert.Empty(t, config.Reason)
	})
}

func TestResponseContext_Structure(t *testing.T) {
	t.Run("Should track parent task information", func(t *testing.T) {
		// Arrange
		parentID := core.MustNewID()
		deferredConfig := &DeferredOutputConfig{ShouldDefer: true}

		// Act
		context := &ResponseContext{
			ParentTaskID:   parentID.String(),
			ChildIndex:     2,
			IsParentTask:   false,
			DeferredConfig: deferredConfig,
		}

		// Assert
		assert.Equal(t, parentID.String(), context.ParentTaskID)
		assert.Equal(t, 2, context.ChildIndex)
		assert.False(t, context.IsParentTask)
		assert.Equal(t, deferredConfig, context.DeferredConfig)
	})

	t.Run("Should handle root task context", func(t *testing.T) {
		// Arrange & Act
		context := &ResponseContext{
			IsParentTask: true,
		}

		// Assert
		assert.True(t, context.IsParentTask)
		assert.Empty(t, context.ParentTaskID)
		assert.Equal(t, 0, context.ChildIndex)
	})
}

func TestValidationResult_Structure(t *testing.T) {
	t.Run("Should track validation success", func(t *testing.T) {
		// Arrange & Act
		result := &ValidationResult{
			Valid:  true,
			Errors: nil,
		}

		// Assert
		assert.True(t, result.Valid)
		assert.Nil(t, result.Errors)
	})

	t.Run("Should track validation errors", func(t *testing.T) {
		// Arrange
		errors := []string{"field is required", "invalid format"}

		// Act
		result := &ValidationResult{
			Valid:  false,
			Errors: errors,
		}

		// Assert
		assert.False(t, result.Valid)
		assert.Equal(t, errors, result.Errors)
		assert.Len(t, result.Errors, 2)
	})
}

func TestCollectionContext_Structure(t *testing.T) {
	t.Run("Should track collection item context", func(t *testing.T) {
		// Arrange & Act
		context := &CollectionContext{
			Item:     "test-item",
			Index:    1,
			ItemVar:  "item",
			IndexVar: "index",
		}

		// Assert
		assert.Equal(t, "test-item", context.Item)
		assert.Equal(t, 1, context.Index)
		assert.Equal(t, "item", context.ItemVar)
		assert.Equal(t, "index", context.IndexVar)
	})

	t.Run("Should handle default variable names", func(t *testing.T) {
		// Arrange & Act
		context := &CollectionContext{
			Item:  42,
			Index: 0,
		}

		// Assert
		assert.Equal(t, 42, context.Item)
		assert.Equal(t, 0, context.Index)
		assert.Empty(t, context.ItemVar)
		assert.Empty(t, context.IndexVar)
	})
}

func TestFilterResult_Structure(t *testing.T) {
	t.Run("Should track filtering results", func(t *testing.T) {
		// Arrange
		filteredItems := []any{"a", "b"}

		// Act
		result := &FilterResult{
			FilteredItems: filteredItems,
			SkippedCount:  3,
			TotalCount:    5,
		}

		// Assert
		assert.Equal(t, filteredItems, result.FilteredItems)
		assert.Equal(t, 3, result.SkippedCount)
		assert.Equal(t, 5, result.TotalCount)
		assert.Len(t, result.FilteredItems, 2)
	})

	t.Run("Should handle empty filter results", func(t *testing.T) {
		// Arrange & Act
		result := &FilterResult{
			FilteredItems: []any{},
			SkippedCount:  0,
			TotalCount:    0,
		}

		// Assert
		assert.Empty(t, result.FilteredItems)
		assert.Equal(t, 0, result.SkippedCount)
		assert.Equal(t, 0, result.TotalCount)
	})
}

func TestStatusAggregationRule_Structure(t *testing.T) {
	t.Run("Should define aggregation strategy", func(t *testing.T) {
		// Arrange & Act
		rule := &StatusAggregationRule{
			Strategy:    "fail_fast",
			AllowFailed: false,
		}

		// Assert
		assert.Equal(t, "fail_fast", rule.Strategy)
		assert.False(t, rule.AllowFailed)
	})

	t.Run("Should handle best effort strategy", func(t *testing.T) {
		// Arrange & Act
		rule := &StatusAggregationRule{
			Strategy:    "best_effort",
			AllowFailed: true,
		}

		// Assert
		assert.Equal(t, "best_effort", rule.Strategy)
		assert.True(t, rule.AllowFailed)
	})
}

func TestParentStatusUpdate_Structure(t *testing.T) {
	t.Run("Should track status update information", func(t *testing.T) {
		// Arrange
		parentID := core.MustNewID()
		timestamp := int64(1234567890)

		// Act
		update := &ParentStatusUpdate{
			ParentStateID: parentID,
			ChildStatus:   core.StatusSuccess,
			ChildTaskID:   "child-task-1",
			UpdatedAt:     timestamp,
		}

		// Assert
		assert.Equal(t, parentID, update.ParentStateID)
		assert.Equal(t, core.StatusSuccess, update.ChildStatus)
		assert.Equal(t, "child-task-1", update.ChildTaskID)
		assert.Equal(t, timestamp, update.UpdatedAt)
	})
}

func TestConfigStorageMetadata_Structure(t *testing.T) {
	t.Run("Should track metadata versioning", func(t *testing.T) {
		// Arrange
		createdAt := int64(1234567890)
		updatedAt := int64(1234567900)

		// Act
		metadata := &ConfigStorageMetadata{
			Version:   "1.0.0",
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		}

		// Assert
		assert.Equal(t, "1.0.0", metadata.Version)
		assert.Equal(t, createdAt, metadata.CreatedAt)
		assert.Equal(t, updatedAt, metadata.UpdatedAt)
	})
}

func TestHandlerError_ErrorInterface(t *testing.T) {
	t.Run("Should implement error interface with message only", func(t *testing.T) {
		// Arrange
		err := &HandlerError{
			HandlerType: "BasicResponseHandler",
			TaskType:    "basic",
			Message:     "processing failed",
		}

		// Act & Assert
		assert.Equal(t, "processing failed", err.Error())
		assert.Nil(t, err.Unwrap())
	})

	t.Run("Should implement error interface with cause", func(t *testing.T) {
		// Arrange
		cause := errors.New("database connection failed")
		err := &HandlerError{
			HandlerType: "BasicResponseHandler",
			TaskType:    "basic",
			Message:     "processing failed",
			Cause:       cause,
		}

		// Act & Assert
		assert.Equal(t, "processing failed: database connection failed", err.Error())
		assert.Equal(t, cause, err.Unwrap())
	})

	t.Run("Should provide structured error information", func(t *testing.T) {
		// Arrange
		err := &HandlerError{
			HandlerType: "CollectionResponseHandler",
			TaskType:    "collection",
			Message:     "item expansion failed",
		}

		// Act & Assert
		assert.Equal(t, "CollectionResponseHandler", err.HandlerType)
		assert.Equal(t, "collection", err.TaskType)
		assert.Equal(t, "item expansion failed", err.Message)
	})
}

func TestValidationError_ErrorInterface(t *testing.T) {
	t.Run("Should implement error interface with field and message", func(t *testing.T) {
		// Arrange
		err := &ValidationError{
			Field:   "task_config",
			Message: "cannot be nil",
		}

		// Act & Assert
		assert.Equal(t, "task_config: cannot be nil", err.Error())
	})

	t.Run("Should implement error interface with value", func(t *testing.T) {
		// Arrange
		err := &ValidationError{
			Field:   "task_type",
			Message: "invalid task type",
			Value:   "invalid_type",
		}

		// Act & Assert
		assert.Equal(t, "task_type: invalid task type (value: invalid_type)", err.Error())
	})

	t.Run("Should provide structured validation information", func(t *testing.T) {
		// Arrange
		err := &ValidationError{
			Field:   "timeout",
			Message: "must be positive",
			Value:   "-5s",
		}

		// Act & Assert
		assert.Equal(t, "timeout", err.Field)
		assert.Equal(t, "must be positive", err.Message)
		assert.Equal(t, "-5s", err.Value)
	})
}
