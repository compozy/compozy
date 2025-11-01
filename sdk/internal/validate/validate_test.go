package validate

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateRequired(t *testing.T) {
	t.Run("Should return error when value is nil", func(t *testing.T) {
		err := Required(t.Context(), "name", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "name is required")
	})
	t.Run("Should return error when string is empty", func(t *testing.T) {
		err := Required(t.Context(), "title", "  ")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "title cannot be empty")
	})
	t.Run("Should return error when slice is empty", func(t *testing.T) {
		values := []string{}
		err := Required(t.Context(), "items", values)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "items cannot be empty")
	})
	t.Run("Should return error when pointer dereferences to empty value", func(t *testing.T) {
		value := "  "
		err := Required(t.Context(), "pointer", &value)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "pointer cannot be empty")
	})
	t.Run("Should succeed for valid value", func(t *testing.T) {
		err := Required(t.Context(), "description", "value")
		require.NoError(t, err)
	})
}

func TestValidateID(t *testing.T) {
	t.Run("Should return error when ID is empty", func(t *testing.T) {
		err := ID(t.Context(), "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "id is required")
	})
	t.Run("Should return error when ID contains invalid characters", func(t *testing.T) {
		err := ID(t.Context(), "invalid_id")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "letters, numbers, or hyphens")
	})
	t.Run("Should succeed for valid ID", func(t *testing.T) {
		err := ID(t.Context(), "abc-123")
		require.NoError(t, err)
	})
	t.Run("Should return error when context is nil", func(t *testing.T) {
		var missingCtx context.Context
		err := ID(missingCtx, "abc-123")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "context is required")
	})
}

func TestValidateNonEmpty(t *testing.T) {
	t.Run("Should return error when value is empty", func(t *testing.T) {
		err := NonEmpty(t.Context(), "name", "\t")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "name cannot be empty")
	})
	t.Run("Should succeed for non empty value", func(t *testing.T) {
		err := NonEmpty(t.Context(), "name", "value")
		require.NoError(t, err)
	})
}

func TestValidateURL(t *testing.T) {
	t.Run("Should return error when URL is empty", func(t *testing.T) {
		err := URL(t.Context(), "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "url is required")
	})
	t.Run("Should return error when scheme is missing", func(t *testing.T) {
		err := URL(t.Context(), "example.com/path")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must include a scheme")
	})
	t.Run("Should return error when host is missing", func(t *testing.T) {
		err := URL(t.Context(), "mailto:user@example.com")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must include a host")
	})
	t.Run("Should succeed for valid URL", func(t *testing.T) {
		err := URL(t.Context(), "https://example.com/path")
		require.NoError(t, err)
	})
}

func TestValidateDuration(t *testing.T) {
	t.Run("Should return error when duration is non positive", func(t *testing.T) {
		err := Duration(t.Context(), 0)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be positive")
	})
	t.Run("Should succeed for positive duration", func(t *testing.T) {
		err := Duration(t.Context(), time.Second)
		require.NoError(t, err)
	})
}

func TestValidateRange(t *testing.T) {
	t.Run("Should return error when bounds are invalid", func(t *testing.T) {
		err := Range(t.Context(), "score", 5, 10, 1)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "range is invalid")
	})
	t.Run("Should return error when value is outside bounds", func(t *testing.T) {
		err := Range(t.Context(), "score", 11, 1, 10)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be between 1 and 10")
	})
	t.Run("Should succeed when value is within range", func(t *testing.T) {
		err := Range(t.Context(), "score", 5, 1, 10)
		require.NoError(t, err)
	})
}
