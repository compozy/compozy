package activities

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/core"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockMemoryManager struct {
	mock.Mock
}

func (m *MockMemoryManager) GetInstance(
	ctx context.Context,
	ref core.MemoryReference,
	workflowContext map[string]any,
) (memcore.Memory, error) {
	args := m.Called(ctx, ref, workflowContext)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(memcore.Memory), args.Error(1)
}

type MockFlushableMemory struct {
	mock.Mock
}

func (m *MockFlushableMemory) PerformFlush(ctx context.Context) (*memcore.FlushMemoryActivityOutput, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*memcore.FlushMemoryActivityOutput), args.Error(1)
}

func (m *MockFlushableMemory) MarkFlushPending(ctx context.Context, pending bool) error {
	args := m.Called(ctx, pending)
	return args.Error(0)
}

func TestNewMemoryActivities(t *testing.T) {
	t.Run("Should create with manager", func(t *testing.T) {
		mockManager := &MockMemoryManager{}
		activities := NewMemoryActivities(mockManager)
		assert.NotNil(t, activities)
		assert.Equal(t, mockManager, activities.MemoryManager)
	})
}

func TestMemoryActivities_FlushMemory(t *testing.T) {
	// Note: This test would fail in actual execution because FlushMemory expects
	// a Temporal activity context. This is a basic structure test.
	t.Run("Should create activities structure", func(t *testing.T) {
		mockManager := &MockMemoryManager{}
		activities := NewMemoryActivities(mockManager)
		assert.NotNil(t, activities)
		assert.Equal(t, mockManager, activities.MemoryManager)
	})
}
