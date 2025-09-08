package project

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebhookSlugsValidator(t *testing.T) {
	t.Run("Should pass with unique slugs", func(t *testing.T) {
		v := NewWebhookSlugsValidator([]string{"a", "b", "c"})
		err := v.Validate()
		require.NoError(t, err)
	})

	t.Run("Should ignore empty slugs", func(t *testing.T) {
		v := NewWebhookSlugsValidator([]string{"", "x", ""})
		err := v.Validate()
		require.NoError(t, err)
	})

	t.Run("Should fail on duplicate slugs", func(t *testing.T) {
		v := NewWebhookSlugsValidator([]string{"dup", "x", "dup"})
		err := v.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate webhook slug")
	})
}
