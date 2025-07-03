package runtime

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsValidRuntimeType(t *testing.T) {
	t.Run("Should return true for supported runtime types", func(t *testing.T) {
		assert.True(t, IsValidRuntimeType(RuntimeTypeBun))
		assert.True(t, IsValidRuntimeType(RuntimeTypeNode))
	})

	t.Run("Should return false for unsupported runtime types", func(t *testing.T) {
		assert.False(t, IsValidRuntimeType("unsupported"))
		assert.False(t, IsValidRuntimeType("python"))
		assert.False(t, IsValidRuntimeType("go"))
		assert.False(t, IsValidRuntimeType(""))
	})

	t.Run("Should be case sensitive", func(t *testing.T) {
		assert.False(t, IsValidRuntimeType("BUN"))
		assert.False(t, IsValidRuntimeType("Node"))
		assert.False(t, IsValidRuntimeType("NODE"))
	})
}

func TestSupportedRuntimeTypes(t *testing.T) {
	t.Run("Should contain expected runtime types", func(t *testing.T) {
		assert.Contains(t, SupportedRuntimeTypes, RuntimeTypeBun)
		assert.Contains(t, SupportedRuntimeTypes, RuntimeTypeNode)
		assert.Len(t, SupportedRuntimeTypes, 2)
	})
}
