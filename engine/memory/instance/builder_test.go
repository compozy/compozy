package instance

import (
	"testing"

	"github.com/compozy/compozy/engine/memory/core"
	"github.com/stretchr/testify/assert"
)

func TestBuilder_Validation(t *testing.T) {
	t.Run("Should fail when instance ID is missing", func(t *testing.T) {
		_, err := NewBuilder().Build()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "instance ID cannot be empty")
	})

	t.Run("Should fail when resource config is missing", func(t *testing.T) {
		_, err := NewBuilder().
			WithInstanceID("test-instance").
			Build()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "resource config cannot be nil")
	})

	t.Run("Should fail when store is missing", func(t *testing.T) {
		resource := &core.Resource{}
		_, err := NewBuilder().
			WithInstanceID("test-instance").
			WithResourceConfig(resource).
			Build()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "memory store cannot be nil")
	})

	t.Run("Should fail when lock manager is missing", func(t *testing.T) {
		mockStore := &mockStore{}
		resource := &core.Resource{}
		_, err := NewBuilder().
			WithInstanceID("test-instance").
			WithResourceConfig(resource).
			WithStore(mockStore).
			Build()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "lock manager cannot be nil")
	})

	t.Run("Should fail when token counter is missing", func(t *testing.T) {
		mockStore := &mockStore{}
		mockLockManager := &mockLockManager{}
		resource := &core.Resource{}
		_, err := NewBuilder().
			WithInstanceID("test-instance").
			WithResourceConfig(resource).
			WithStore(mockStore).
			WithLockManager(mockLockManager).
			Build()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "token counter cannot be nil")
	})

	t.Run("Should fail when flushing strategy is missing", func(t *testing.T) {
		mockStore := &mockStore{}
		mockLockManager := &mockLockManager{}
		mockTokenCounter := &mockTokenCounter{}
		resource := &core.Resource{}
		_, err := NewBuilder().
			WithInstanceID("test-instance").
			WithResourceConfig(resource).
			WithStore(mockStore).
			WithLockManager(mockLockManager).
			WithTokenCounter(mockTokenCounter).
			Build()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "flushing strategy cannot be nil")
	})

	t.Run("Should fail when temporal client is missing", func(t *testing.T) {
		mockStore := &mockStore{}
		mockLockManager := &mockLockManager{}
		mockTokenCounter := &mockTokenCounter{}
		mockFlushStrategy := &mockFlushStrategy{}
		resource := &core.Resource{}
		_, err := NewBuilder().
			WithInstanceID("test-instance").
			WithResourceConfig(resource).
			WithStore(mockStore).
			WithLockManager(mockLockManager).
			WithTokenCounter(mockTokenCounter).
			WithFlushingStrategy(mockFlushStrategy).
			Build()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "temporal client cannot be nil")
	})
}
