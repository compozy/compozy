package agent

import (
	"testing"

	"github.com/compozy/compozy/engine/memory" // Assuming memory.MemoryReference is here
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	// "github.com/compozy/compozy/engine/autoload" // For later when registry is used
)

func TestAgentMemoryValidator_Validate(t *testing.T) {
	// var mockRegistry *autoload.Registry // Placeholder for when registry check is active

	t.Run("Valid: No memory references (nil)", func(t *testing.T) {
		validator := NewAgentMemoryValidator(nil /*, mockRegistry */)
		err := validator.Validate()
		assert.NoError(t, err)
	})

	t.Run("Valid: No memory references (empty slice)", func(t *testing.T) {
		validator := NewAgentMemoryValidator([]memory.MemoryReference{} /*, mockRegistry */)
		err := validator.Validate()
		assert.NoError(t, err)
	})

	t.Run("Valid: Single memory reference", func(t *testing.T) {
		refs := []memory.MemoryReference{
			{ID: "mem1", Key: "key-{{.workflow.id}}", Mode: "read-write"},
		}
		// Setup mockRegistry to return true for mockRegistry.MemoryExists("mem1") when Task 4 is done
		validator := NewAgentMemoryValidator(refs /*, mockRegistry */)
		err := validator.Validate()
		assert.NoError(t, err)
	})

	t.Run("Valid: Multiple memory references", func(t *testing.T) {
		refs := []memory.MemoryReference{
			{ID: "mem1", Key: "key1", Mode: "read-write"},
			{ID: "mem2", Key: "key2", Mode: "read-only"},
		}
		validator := NewAgentMemoryValidator(refs /*, mockRegistry */)
		err := validator.Validate()
		assert.NoError(t, err)
	})

	t.Run("Invalid: Missing ID in a reference", func(t *testing.T) {
		refs := []memory.MemoryReference{
			{ID: "", Key: "key1", Mode: "read-write"}, // Empty ID
		}
		validator := NewAgentMemoryValidator(refs /*, mockRegistry */)
		err := validator.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "memory reference at index 0 has an empty ID")
	})

	t.Run("Invalid: Missing Key in a reference", func(t *testing.T) {
		refs := []memory.MemoryReference{
			{ID: "mem1", Key: "", Mode: "read-write"}, // Empty Key
		}
		validator := NewAgentMemoryValidator(refs /*, mockRegistry */)
		err := validator.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "memory reference for ID 'mem1' (index 0) has an empty key template")
	})

	t.Run("Invalid: Invalid Mode in a reference", func(t *testing.T) {
		refs := []memory.MemoryReference{
			{ID: "mem1", Key: "key1", Mode: "write-only"}, // Invalid mode
		}
		validator := NewAgentMemoryValidator(refs /*, mockRegistry */)
		err := validator.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "memory reference for ID 'mem1' (index 0) has invalid mode 'write-only'")
	})

	t.Run("Invalid: Mode defaults to read-write if empty, but validator re-checks", func(t *testing.T) {
		// normalizeAndValidateMemoryConfig should set a default mode.
		// AgentMemoryValidator then re-validates this.
		// This test ensures AgentMemoryValidator itself checks the mode string strictly.
		refs := []memory.MemoryReference{
			{ID: "mem1", Key: "key1", Mode: ""}, // Assume normalize already defaulted this.
			                                     // If normalize didn't, this test might be slightly different.
												 // Let's assume normalize *did* default it to "read-write".
												 // So, to test AgentMemoryValidator specifically, we'd need to pass an invalid mode.
		}
		// If mode was defaulted to "read-write" by normalize, then this would pass.
		// Let's test the validator's own check for an invalid mode string.
		refs[0].Mode = "bad-mode"
		validator := NewAgentMemoryValidator(refs)
		err := validator.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid mode 'bad-mode'")
	})

	// Placeholder for when registry check is active in Task 4.0
	// t.Run("Invalid: Memory ID does not exist in registry", func(t *testing.T) {
	// 	refs := []memory.MemoryReference{
	// 		{ID: "non-existent-mem", Key: "key1", Mode: "read-write"},
	// 	}
	// 	// Setup mockRegistry to return false for mockRegistry.MemoryExists("non-existent-mem")
	// 	validator := NewAgentMemoryValidator(refs, mockRegistry)
	// 	err := validator.Validate()
	// 	require.Error(t, err)
	// 	assert.Contains(t, err.Error(), "memory resource with ID 'non-existent-mem' referenced by agent is not defined")
	// })
}
