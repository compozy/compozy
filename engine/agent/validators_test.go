package agent

import (
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	// "github.com/compozy/compozy/engine/autoload" // For later when registry is used
)

func TestMemoryValidator_Validate(t *testing.T) {
	t.Run("Should pass with no memory references (nil)", func(t *testing.T) {
		validator := NewMemoryValidator(nil /*, mockRegistry */)
		err := validator.Validate()
		assert.NoError(t, err)
	})

	t.Run("Should pass with no memory references (empty slice)", func(t *testing.T) {
		validator := NewMemoryValidator([]core.MemoryReference{} /*, mockRegistry */)
		err := validator.Validate()
		assert.NoError(t, err)
	})

	t.Run("Should validate a single memory reference", func(t *testing.T) {
		refs := []core.MemoryReference{
			{ID: "mem1", Key: "key-{{.workflow.id}}", Mode: "read-write"},
		}
		// Setup mockRegistry to return true for mockRegistry.MemoryExists("mem1") when Task 4 is done
		validator := NewMemoryValidator(refs /*, mockRegistry */)
		err := validator.Validate()
		assert.NoError(t, err)
	})

	t.Run("Should validate multiple memory references", func(t *testing.T) {
		refs := []core.MemoryReference{
			{ID: "mem1", Key: "key1", Mode: "read-write"},
			{ID: "mem2", Key: "key2", Mode: "read-only"},
		}
		validator := NewMemoryValidator(refs /*, mockRegistry */)
		err := validator.Validate()
		assert.NoError(t, err)
	})

	t.Run("Should error when ID is missing", func(t *testing.T) {
		refs := []core.MemoryReference{
			{ID: "", Key: "key1", Mode: "read-write"}, // Empty ID
		}
		validator := NewMemoryValidator(refs /*, mockRegistry */)
		err := validator.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "memory reference at index 0 has an empty ID")
	})

	t.Run("Should allow missing Key (fallback to default_key_template)", func(t *testing.T) {
		refs := []core.MemoryReference{
			{ID: "mem1", Key: "", Mode: "read-write"}, // Empty Key allowed
		}
		validator := NewMemoryValidator(refs /*, mockRegistry */)
		err := validator.Validate()
		assert.NoError(t, err)
	})

	t.Run("Should error when Mode is invalid", func(t *testing.T) {
		refs := []core.MemoryReference{
			{ID: "mem1", Key: "key1", Mode: "write-only"}, // Invalid mode
		}
		validator := NewMemoryValidator(refs /*, mockRegistry */)
		err := validator.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "memory reference for ID 'mem1' (index 0) has invalid mode 'write-only'")
	})

	t.Run("Should revalidate mode string strictly even if defaulted", func(t *testing.T) {
		// normalizeAndValidateMemoryConfig should set a default mode.
		// AgentMemoryValidator then re-validates this.
		// This test ensures AgentMemoryValidator itself checks the mode string strictly.
		refs := []core.MemoryReference{
			{ID: "mem1", Key: "key1", Mode: ""}, // Assume normalize already defaulted this.
			// If normalize didn't, this test might be slightly different.
			// Let's assume normalize *did* default it to "read-write".
			// So, to test AgentMemoryValidator specifically, we'd need to pass an invalid mode.
		}
		// If mode was defaulted to "read-write" by normalize, then this would pass.
		// Let's test the validator's own check for an invalid mode string.
		refs[0].Mode = "bad-mode"
		validator := NewMemoryValidator(refs)
		err := validator.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid mode 'bad-mode'")
	})
	// TODO: Enable registry existence validation when the registry integration is available
}

func TestActionsValidator_Validate(t *testing.T) {
	t.Run("Should return nil when actions slice is nil", func(t *testing.T) {
		v := NewActionsValidator(nil)
		assert.NoError(t, v.Validate())
	})

	t.Run("Should validate all actions and return error on invalid", func(t *testing.T) {
		valid := &ActionConfig{ID: "ok", Prompt: "p"}
		tmp := t.TempDir()
		require.NoError(t, valid.SetCWD(tmp))
		invalid := &ActionConfig{ID: "bad"} // missing CWD -> schema.CWDValidator fails
		v := NewActionsValidator([]*ActionConfig{valid, invalid})
		err := v.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "current working directory is required")
	})

	t.Run("Should pass when all actions are valid", func(t *testing.T) {
		a1 := &ActionConfig{ID: "a1", Prompt: "p1", InputSchema: &schema.Schema{"type": "object"}}
		tmp := t.TempDir()
		require.NoError(t, a1.SetCWD(tmp))
		a2 := &ActionConfig{ID: "a2", Prompt: "p2"}
		require.NoError(t, a2.SetCWD(tmp))
		v := NewActionsValidator([]*ActionConfig{a1, a2})
		assert.NoError(t, v.Validate())
	})
}
