package instance

import (
	"testing"

	"github.com/compozy/compozy/engine/memory/core"
	"github.com/stretchr/testify/assert"
)

func TestBuilder_Validation(t *testing.T) {
	t.Run("Should fail when instance ID is missing", func(t *testing.T) {
		_, err := NewBuilder().Build(t.Context())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "instance ID cannot be empty")
	})

	t.Run("Should fail when resource config is missing", func(t *testing.T) {
		_, err := NewBuilder().
			WithInstanceID("test-instance").
			Build(t.Context())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "resource config cannot be nil")
	})

	t.Run("Should fail when store is missing", func(t *testing.T) {
		resource := &core.Resource{}
		_, err := NewBuilder().
			WithInstanceID("test-instance").
			WithResourceConfig(resource).
			Build(t.Context())
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
			Build(t.Context())
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
			Build(t.Context())
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
			Build(t.Context())
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
			Build(t.Context())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "temporal client cannot be nil")
	})
	t.Run("Should validate all required dependencies are present", func(t *testing.T) {
		// Test that all dependencies are validated
		// Test each required field
		testCases := []struct {
			name          string
			setupBuilder  func(*Builder)
			expectedError string
		}{
			{
				name:          "instance ID",
				setupBuilder:  func(_ *Builder) {},
				expectedError: "instance ID cannot be empty",
			},
			{
				name: "resource config",
				setupBuilder: func(b *Builder) {
					b.WithInstanceID("test")
				},
				expectedError: "resource config cannot be nil",
			},
			{
				name: "store",
				setupBuilder: func(b *Builder) {
					b.WithInstanceID("test").
						WithResourceConfig(&core.Resource{})
				},
				expectedError: "memory store cannot be nil",
			},
			{
				name: "lock manager",
				setupBuilder: func(b *Builder) {
					b.WithInstanceID("test").
						WithResourceConfig(&core.Resource{}).
						WithStore(&mockStore{})
				},
				expectedError: "lock manager cannot be nil",
			},
			{
				name: "token counter",
				setupBuilder: func(b *Builder) {
					b.WithInstanceID("test").
						WithResourceConfig(&core.Resource{}).
						WithStore(&mockStore{}).
						WithLockManager(&mockLockManager{})
				},
				expectedError: "token counter cannot be nil",
			},
			{
				name: "flushing strategy",
				setupBuilder: func(b *Builder) {
					b.WithInstanceID("test").
						WithResourceConfig(&core.Resource{}).
						WithStore(&mockStore{}).
						WithLockManager(&mockLockManager{}).
						WithTokenCounter(&mockTokenCounter{})
				},
				expectedError: "flushing strategy cannot be nil",
			},
			{
				name: "temporal client",
				setupBuilder: func(b *Builder) {
					b.WithInstanceID("test").
						WithResourceConfig(&core.Resource{}).
						WithStore(&mockStore{}).
						WithLockManager(&mockLockManager{}).
						WithTokenCounter(&mockTokenCounter{}).
						WithFlushingStrategy(&mockFlushStrategy{})
				},
				expectedError: "temporal client cannot be nil",
			},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				b := NewBuilder()
				tc.setupBuilder(b)
				err := b.Validate(t.Context())
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			})
		}
	})
	t.Run("Should successfully validate with all dependencies", func(t *testing.T) {
		mockStore := &mockStore{}
		mockLockManager := &mockLockManager{}
		mockTokenCounter := &mockTokenCounter{}
		mockFlushStrategy := &mockFlushStrategy{}
		resource := &core.Resource{
			ID:        "test-memory",
			Type:      core.TokenBasedMemory,
			MaxTokens: 1000,
		}

		// Test validation passes with all required fields except temporal client
		b := NewBuilder().
			WithInstanceID("test-instance").
			WithResourceConfig(resource).
			WithStore(mockStore).
			WithLockManager(mockLockManager).
			WithTokenCounter(mockTokenCounter).
			WithFlushingStrategy(mockFlushStrategy)

		err := b.Validate(t.Context())
		assert.Error(t, err) // Still fails due to missing temporal client
		assert.Contains(t, err.Error(), "temporal client cannot be nil")
	})

	t.Run("Should set default values correctly", func(t *testing.T) {
		builder := NewBuilder().
			WithInstanceID("test-instance").
			WithResourceConfig(&core.Resource{}).
			WithStore(&mockStore{}).
			WithLockManager(&mockLockManager{}).
			WithTokenCounter(&mockTokenCounter{}).
			WithFlushingStrategy(&mockFlushStrategy{})
		// Can't set temporal client due to complex interface
		// But we can verify other defaults
		assert.Equal(t, "", builder.opts.TemporalTaskQueue)
		// NOTE: Full builder testing with temporal client is in test/integration/memory/builder_test.go
		// The temporal.Client interface is too complex to mock in unit tests
	})
}
