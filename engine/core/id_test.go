package core_test

import (
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestID_String(t *testing.T) {
	t.Run("Should return string representation of ID", func(t *testing.T) {
		id := core.ID("test-id-123")
		result := id.String()
		assert.Equal(t, "test-id-123", result)
	})
}

func TestID_IsZero(t *testing.T) {
	t.Run("Should return true for zero-value ID", func(t *testing.T) {
		var zeroID core.ID
		assert.True(t, zeroID.IsZero())
	})
	t.Run("Should return true for empty string ID", func(t *testing.T) {
		emptyID := core.ID("")
		assert.True(t, emptyID.IsZero())
	})
	t.Run("Should return false for non-zero ID", func(t *testing.T) {
		id := core.MustNewID()
		assert.False(t, id.IsZero())
	})
	t.Run("Should return false for manually created non-empty ID", func(t *testing.T) {
		id := core.ID("some-id")
		assert.False(t, id.IsZero())
	})
}

func TestNewID(t *testing.T) {
	t.Run("Should generate a new unique ID", func(t *testing.T) {
		id1, err := core.NewID()
		require.NoError(t, err)
		assert.NotEmpty(t, id1)
		assert.False(t, id1.IsZero())
		id2, err := core.NewID()
		require.NoError(t, err)
		assert.NotEmpty(t, id2)
		assert.NotEqual(t, id1, id2, "IDs should be unique")
	})
	t.Run("Should generate valid KSUID format", func(t *testing.T) {
		id, err := core.NewID()
		require.NoError(t, err)
		// Validate using our parser instead of length to avoid brittle checks
		parsed, err := core.ParseID(id.String())
		require.NoError(t, err)
		assert.Equal(t, id, parsed)
	})
}

func TestMustNewID(t *testing.T) {
	t.Run("Should generate a new ID without error", func(t *testing.T) {
		assert.NotPanics(t, func() {
			id := core.MustNewID()
			assert.NotEmpty(t, id)
			assert.False(t, id.IsZero())
		})
	})
	t.Run("Should generate unique IDs", func(t *testing.T) {
		id1 := core.MustNewID()
		id2 := core.MustNewID()
		assert.NotEqual(t, id1, id2)
	})
}

func TestParseID(t *testing.T) {
	t.Run("Should parse valid KSUID", func(t *testing.T) {
		validID := core.MustNewID()
		parsed, err := core.ParseID(validID.String())
		require.NoError(t, err)
		assert.Equal(t, validID, parsed)
	})
	t.Run("Should return error for empty string", func(t *testing.T) {
		id, err := core.ParseID("")
		assert.ErrorContains(t, err, "empty ID")
		assert.True(t, id.IsZero())
	})
	t.Run("Should return error for invalid format", func(t *testing.T) {
		id, err := core.ParseID("not-a-valid-ksuid")
		assert.ErrorContains(t, err, "invalid ID format")
		assert.True(t, id.IsZero())
	})
	t.Run("Should return error for invalid characters", func(t *testing.T) {
		id, err := core.ParseID("!@#$%^&*()")
		assert.ErrorContains(t, err, "invalid ID format")
		assert.True(t, id.IsZero())
	})
}
