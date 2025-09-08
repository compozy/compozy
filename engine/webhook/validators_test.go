package webhook

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebhook_ValidateTrigger_MinimalValid(t *testing.T) {
	cfg := &Config{
		Slug:   "events",
		Events: []EventConfig{{Name: "evt1", Filter: "true", Input: map[string]string{"a": "b"}}},
	}
	ApplyDefaults(cfg)
	err := ValidateTrigger(cfg)
	require.NoError(t, err)
	assert.Equal(t, "POST", cfg.Method)
}

func TestWebhook_ValidateTrigger_Errors(t *testing.T) {
	t.Run("missing slug", func(t *testing.T) {
		cfg := &Config{Events: []EventConfig{{Name: "e", Filter: "true", Input: map[string]string{"k": "v"}}}}
		err := ValidateTrigger(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "webhook slug is required")
	})
	t.Run("no events", func(t *testing.T) {
		cfg := &Config{Slug: "a"}
		err := ValidateTrigger(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "webhook events are required")
	})
	t.Run("empty input", func(t *testing.T) {
		cfg := &Config{Slug: "a", Events: []EventConfig{{Name: "e", Filter: "true", Input: map[string]string{}}}}
		err := ValidateTrigger(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "input is required and cannot be empty")
	})
	t.Run("duplicate event names", func(t *testing.T) {
		cfg := &Config{Slug: "a", Events: []EventConfig{
			{Name: "e", Filter: "true", Input: map[string]string{"k": "v"}},
			{Name: "e", Filter: "true", Input: map[string]string{"k": "v"}},
		}}
		err := ValidateTrigger(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate event name")
	})
	t.Run("hmac missing fields", func(t *testing.T) {
		cfg := &Config{
			Slug:   "a",
			Verify: &VerifySpec{Strategy: "hmac"},
			Events: []EventConfig{{Name: "e", Filter: "true", Input: map[string]string{"k": "v"}}},
		}
		err := ValidateTrigger(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "hmac verification requires secret and header")
		cfg.Verify.Secret = "env://SECRET"
		cfg.Verify.Header = "X-Signature"
		err = ValidateTrigger(cfg)
		require.NoError(t, err)
	})
}
