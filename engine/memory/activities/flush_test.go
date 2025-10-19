package activities

import (
	"context"
	"errors"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/temporal"
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

type stubMemory struct{}

func (stubMemory) Append(context.Context, llm.Message) error       { return nil }
func (stubMemory) AppendMany(context.Context, []llm.Message) error { return nil }
func (stubMemory) Read(context.Context) ([]llm.Message, error)     { return nil, nil }
func (stubMemory) ReadPaginated(context.Context, int, int) ([]llm.Message, int, error) {
	return nil, 0, nil
}
func (stubMemory) Len(context.Context) (int, error)           { return 0, nil }
func (stubMemory) GetTokenCount(context.Context) (int, error) { return 0, nil }
func (stubMemory) GetMemoryHealth(context.Context) (*memcore.Health, error) {
	return &memcore.Health{}, nil
}
func (stubMemory) Clear(context.Context) error { return nil }
func (stubMemory) GetID() string               { return "stub" }
func (stubMemory) AppendWithPrivacy(context.Context, llm.Message, memcore.PrivacyMetadata) error {
	return nil
}

func TestMemoryActivities_getMemoryInstance(t *testing.T) {
	t.Run("Should wrap config error as non-retryable memory config problem", func(t *testing.T) {
		ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
		manager := &MockMemoryManager{}
		activities := &MemoryActivities{MemoryManager: manager}
		configErr := memcore.NewConfigError("bad config", errors.New("boom"))
		manager.
			On("GetInstance", mock.Anything, mock.Anything, mock.Anything).
			Return(nil, configErr)
		input := memcore.FlushMemoryActivityInput{MemoryResourceID: "mem", MemoryInstanceKey: "key", ProjectID: "proj"}
		instance, err := activities.getMemoryInstance(ctx, input)
		require.Nil(t, instance)
		var appErr *temporal.ApplicationError
		require.ErrorAs(t, err, &appErr)
		assert.True(t, appErr.NonRetryable())
		assert.Equal(t, ErrCodeMemoryConfigError, appErr.Type())
		manager.AssertExpectations(t)
	})

	t.Run("Should wrap generic manager error as retryable instance creation failure", func(t *testing.T) {
		ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
		manager := &MockMemoryManager{}
		activities := &MemoryActivities{MemoryManager: manager}
		manager.
			On("GetInstance", mock.Anything, mock.Anything, mock.Anything).
			Return(nil, errors.New("boom"))
		input := memcore.FlushMemoryActivityInput{MemoryResourceID: "mem", MemoryInstanceKey: "key", ProjectID: "proj"}
		instance, err := activities.getMemoryInstance(ctx, input)
		require.Nil(t, instance)
		var appErr *temporal.ApplicationError
		require.ErrorAs(t, err, &appErr)
		assert.False(t, appErr.NonRetryable())
		assert.Equal(t, ErrCodeInstanceCreationFailed, appErr.Type())
		manager.AssertExpectations(t)
	})
}

func TestMemoryActivities_validateFlushable(t *testing.T) {
	activities := &MemoryActivities{}
	_, err := activities.validateFlushable(stubMemory{})
	var appErr *temporal.ApplicationError
	require.ErrorAs(t, err, &appErr)
	assert.True(t, appErr.NonRetryable())
	assert.Equal(t, ErrCodeNotFlushable, appErr.Type())
}

func TestMemoryActivities_handleFlushError(t *testing.T) {
	t.Run("Should return retryable error on lock contention", func(t *testing.T) {
		activities := &MemoryActivities{}
		lockErr := memcore.NewLockError("busy", errors.New("lock"))
		err := activities.handleFlushError(lockErr)
		var appErr *temporal.ApplicationError
		require.ErrorAs(t, err, &appErr)
		assert.False(t, appErr.NonRetryable())
		assert.Equal(t, ErrCodeLockContention, appErr.Type())
	})

	t.Run("Should return non-retryable error on generic flush failure", func(t *testing.T) {
		activities := &MemoryActivities{}
		err := activities.handleFlushError(errors.New("boom"))
		var appErr *temporal.ApplicationError
		require.ErrorAs(t, err, &appErr)
		assert.True(t, appErr.NonRetryable())
		assert.Equal(t, ErrCodeFlushFailed, appErr.Type())
	})
}
