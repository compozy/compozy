package activities

import (
	"context"
	"fmt"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/memory"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
)

// MockMemoryManager mocks the MemoryManager for testing
type MockMemoryManager struct {
	mock.Mock
}

func (m *MockMemoryManager) GetInstance(
	ctx context.Context,
	ref core.MemoryReference,
	workflowContext map[string]any,
) (memory.Memory, error) {
	args := m.Called(ctx, ref, workflowContext)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(memory.Memory), args.Error(1)
}

// MockMemoryInstance mocks a FlushableMemory instance for testing
type MockMemoryInstance struct {
	mock.Mock
}

func (m *MockMemoryInstance) Append(ctx context.Context, msg llm.Message) error {
	args := m.Called(ctx, msg)
	return args.Error(0)
}
func (m *MockMemoryInstance) Read(ctx context.Context) ([]llm.Message, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]llm.Message), args.Error(1)
}

func (m *MockMemoryInstance) Len(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}

func (m *MockMemoryInstance) GetTokenCount(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}

func (m *MockMemoryInstance) GetMemoryHealth(ctx context.Context) (*memory.Health, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*memory.Health), args.Error(1)
}

func (m *MockMemoryInstance) Clear(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockMemoryInstance) GetID() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockMemoryInstance) AppendWithPrivacy(
	ctx context.Context,
	msg llm.Message,
	metadata memory.PrivacyMetadata,
) error {
	args := m.Called(ctx, msg, metadata)
	return args.Error(0)
}

func (m *MockMemoryInstance) PerformFlush(ctx context.Context) (*memory.FlushMemoryActivityOutput, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*memory.FlushMemoryActivityOutput), args.Error(1)
}

func (m *MockMemoryInstance) MarkFlushPending(ctx context.Context, pending bool) error {
	args := m.Called(ctx, pending)
	return args.Error(0)
}

// Verify interface compliance
var _ memory.FlushableMemory = (*MockMemoryInstance)(nil)

func TestMemoryActivities_FlushMemory(t *testing.T) {
	t.Run("Should successfully flush memory using dynamic dependency injection", func(t *testing.T) {
		// Create test suite
		testSuite := &testsuite.WorkflowTestSuite{}
		env := testSuite.NewTestActivityEnvironment()
		// Create mocks
		mockManager := new(MockMemoryManager)
		mockInstance := new(MockMemoryInstance)
		log := logger.NewForTests()
		// Create activity instance with mocked manager
		activities := NewMemoryActivities(mockManager, log)
		// Register the activity
		env.RegisterActivity(activities.FlushMemory)
		// Test input
		input := memory.FlushMemoryActivityInput{
			MemoryInstanceKey: "test-instance-key",
			MemoryResourceID:  "test-resource-id",
		}
		// Expected output
		expectedOutput := &memory.FlushMemoryActivityOutput{
			Success:          true,
			MessageCount:     10,
			TokenCount:       500,
			SummaryGenerated: true,
		}
		// Setup expectations
		// MemoryManager.GetInstance is called with the correct reference and context
		mockManager.On("GetInstance", mock.Anything, mock.MatchedBy(func(ref core.MemoryReference) bool {
			return ref.ID == input.MemoryResourceID && ref.Key == input.MemoryInstanceKey
		}), mock.MatchedBy(func(ctx map[string]any) bool {
			// Verify the workflow context contains the required fields
			return ctx["memory_instance_key"] == input.MemoryInstanceKey &&
				ctx["memory_resource_id"] == input.MemoryResourceID
		})).Return(mockInstance, nil)
		// The returned instance's PerformFlush method is called
		mockInstance.On("PerformFlush", mock.Anything).Return(expectedOutput, nil)
		// Execute activity
		val, err := env.ExecuteActivity(activities.FlushMemory, input)
		require.NoError(t, err)
		// Verify result
		var output memory.FlushMemoryActivityOutput
		require.NoError(t, val.Get(&output))
		assert.Equal(t, expectedOutput.Success, output.Success)
		assert.Equal(t, expectedOutput.MessageCount, output.MessageCount)
		assert.Equal(t, expectedOutput.TokenCount, output.TokenCount)
		assert.Equal(t, expectedOutput.SummaryGenerated, output.SummaryGenerated)

		// Verify all expectations were met
		mockManager.AssertExpectations(t)
		mockInstance.AssertExpectations(t)
	})
	t.Run("Should handle memory resource not found error", func(t *testing.T) {
		// Create test suite
		testSuite := &testsuite.WorkflowTestSuite{}
		env := testSuite.NewTestActivityEnvironment()
		// Create mocks
		mockManager := new(MockMemoryManager)
		log := logger.NewForTests()
		// Create activity instance with mocked manager
		activities := NewMemoryActivities(mockManager, log)
		// Register the activity
		env.RegisterActivity(activities.FlushMemory)
		// Test input
		input := memory.FlushMemoryActivityInput{
			MemoryInstanceKey: "test-instance-key",
			MemoryResourceID:  "non-existent-resource",
		}
		// Setup expectations - resource not found (using typed error)
		configErr := memory.NewConfigError("non-existent-resource", "not found", fmt.Errorf("resource not found"))
		mockManager.On("GetInstance", mock.Anything, mock.Anything, mock.Anything).
			Return(nil, configErr)
		// Execute activity
		_, err := env.ExecuteActivity(activities.FlushMemory, input)
		require.Error(t, err)
		// Verify it's a non-retryable error
		assert.Contains(t, err.Error(), "MEMORY_CONFIG_ERROR")
		// Verify expectations
		mockManager.AssertExpectations(t)
	})
	t.Run("Should handle lock contention as retryable error", func(t *testing.T) {
		// Create test suite
		testSuite := &testsuite.WorkflowTestSuite{}
		env := testSuite.NewTestActivityEnvironment()
		// Create mocks
		mockManager := new(MockMemoryManager)
		mockInstance := new(MockMemoryInstance)
		log := logger.NewForTests()
		// Create activity instance with mocked manager
		activities := NewMemoryActivities(mockManager, log)
		// Register the activity
		env.RegisterActivity(activities.FlushMemory)
		// Test input
		input := memory.FlushMemoryActivityInput{
			MemoryInstanceKey: "test-instance-key",
			MemoryResourceID:  "test-resource-id",
		}
		// Setup expectations
		mockManager.On("GetInstance", mock.Anything, mock.Anything, mock.Anything).
			Return(mockInstance, nil)
		// PerformFlush fails due to lock contention (using typed error)
		lockErr := memory.NewLockError("flush", "test-instance-key", fmt.Errorf("lock contention"))
		mockInstance.On("PerformFlush", mock.Anything).
			Return(nil, lockErr)
		// Execute activity
		_, err := env.ExecuteActivity(activities.FlushMemory, input)
		require.Error(t, err)
		// Verify it's a retryable error related to lock contention
		assert.Contains(t, err.Error(), "LOCK_CONTENTION")
		// Verify expectations
		mockManager.AssertExpectations(t)
		mockInstance.AssertExpectations(t)
	})

	t.Run("Should return non-retryable error for missing memory resource", func(t *testing.T) {
		// Create test suite
		testSuite := &testsuite.WorkflowTestSuite{}
		env := testSuite.NewTestActivityEnvironment()

		// Create mocks
		mockManager := new(MockMemoryManager)
		log := logger.NewForTests()

		// Create activity instance
		activities := NewMemoryActivities(mockManager, log)

		// Register the activity
		env.RegisterActivity(activities.FlushMemory)

		// Test input
		input := memory.FlushMemoryActivityInput{
			MemoryInstanceKey: "test-instance-key",
			MemoryResourceID:  "missing-resource",
		}

		// Setup expectations - resource not found (using typed error)
		configErr := memory.NewConfigError("missing-resource", "not found", fmt.Errorf("resource not found"))
		mockManager.On("GetInstance", mock.Anything, mock.Anything, mock.Anything).
			Return(nil, configErr)

		// Execute activity - expect error
		_, err := env.ExecuteActivity(activities.FlushMemory, input)
		require.Error(t, err)

		// Verify it's a non-retryable error
		assert.Contains(t, err.Error(), "MEMORY_CONFIG_ERROR")

		// Verify expectations
		mockManager.AssertExpectations(t)
	})
}

func TestMemoryManager_CachedComponents(t *testing.T) {
	t.Run("Should cache and reuse expensive components", func(t *testing.T) {
		// This test would verify that TokenCounter and other expensive components
		// are cached and reused across multiple GetInstance calls for the same resource
		// Implementation depends on the actual MemoryManager structure
		t.Skip("TODO: Implement caching verification test")
	})
}
