package activities

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/CompoZy/llm-router/engine/infra/cache"
	"github.com/CompoZy/llm-router/engine/llm"
	"github.com/CompoZy/llm-router/engine/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
)

// Mock implementations for dependencies of MemoryActivities
type mockActivityMemoryStore struct {
	mock.Mock
	memory.MemoryStore // Embed for unmocked methods if any, or satisfy interface
}

func (m *mockActivityMemoryStore) ReadMessages(ctx context.Context, key string) ([]llm.Message, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]llm.Message), args.Error(1)
}
func (m *mockActivityMemoryStore) ReplaceMessages(ctx context.Context, key string, messages []llm.Message) error {
	args := m.Called(ctx, key, messages)
	return args.Error(0)
}

// Implement other MemoryStore methods if they are called by the activity, otherwise they can panic or be no-op.
func (m *mockActivityMemoryStore) AppendMessage(ctx context.Context, key string, msg llm.Message) error { return nil }
func (m *mockActivityMemoryStore) AppendMessages(ctx context.Context, key string, msgs []llm.Message) error { return nil }
func (m *mockActivityMemoryStore) CountMessages(ctx context.Context, key string) (int, error) { return 0, nil }
func (m *mockActivityMemoryStore) TrimMessages(ctx context.Context, key string, keepCount int) error { return nil }
func (m *mockActivityMemoryStore) SetExpiration(ctx context.Context, key string, ttl time.Duration) error { return nil }
func (m *mockActivityMemoryStore) DeleteMessages(ctx context.Context, key string) error { return nil }
func (m *mockActivityMemoryStore) GetKeyTTL(ctx context.Context, key string) (time.Duration, error) { return 0, nil }


type mockActivityLockManager struct {
	mock.Mock
}
func (m *mockActivityLockManager) Acquire(ctx context.Context, resource string, ttl time.Duration) (cache.Lock, error) {
	args := m.Called(ctx, resource, ttl)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(cache.Lock), args.Error(1)
}

type mockActivityLock struct {
	mock.Mock
	cache.Lock // Embed for unmocked methods
}
func (m *mockActivityLock) Release(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}
func (m *mockActivityLock) Refresh(ctx context.Context) error { return nil } // Not directly called by activity, but part of interface
func (m *mockActivityLock) Resource() string { return "" }
func (m *mockActivityLock) IsHeld() bool { return true }


func TestFlushMemoryActivity(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestActivityEnvironment()

	// Prepare mock dependencies
	mockStore := new(mockActivityMemoryStore)
	mockLockMgr := new(mockActivityLockManager)
	mockLock := new(mockActivityLock)

	// Setup TokenCounter, MemoryResource, TokenMemoryManager, HybridFlushingStrategy for the activity
	tokenCounter, _ := memory.NewTiktokenCounter(memory.defaultEncoding)
	memResourceCfg := &memory.MemoryResource{
		ID:        "testResource",
		Type:      memory.TokenBasedMemory,
		MaxTokens: 50, // Example limit
		Persistence: memory.PersistenceConfig{TTL: "1h"},
		FlushingStrategy: &memory.FlushingStrategyConfig{
			Type: memory.HybridSummaryFlushing,
			SummarizeThreshold: 0.5, // Flush if 50% full (25 tokens)
			SummaryTokens: 10,
		},
	}
	tokenMgr, _ := memory.NewTokenMemoryManager(memResourceCfg, tokenCounter)
	summarizer := memory.NewRuleBasedSummarizer(tokenCounter, 1, 1) // Keep 1 first, 1 last
	flushStrategy, _ := memory.NewHybridFlushingStrategy(memResourceCfg.FlushingStrategy, summarizer, tokenMgr)

	activities := NewMemoryActivities(mockStore, memory.MemoryLockManager{ /* real one needs an infra.cache.LockManager */ }, tokenMgr, flushStrategy, memResourceCfg)
	env.RegisterActivity(activities.FlushMemory) // Register the actual activity instance

	input := FlushMemoryActivityInput{
		MemoryInstanceKey: "user123:sessionABC",
		ResourceID:        "testResource",
	}

	// --- Test Case 1: Successful flush ---
	t.Run("SuccessfulFlush", func(t *testing.T) {
		// Reset mocks for this sub-test if using testify's suite, or re-init here
		currentStore := new(mockActivityMemoryStore)
		currentLockMgr := new(mockActivityLockManager)
		currentLock := new(mockActivityLock)

		// Re-initialize activities with fresh mocks for this sub-test
		// This is tricky because NewMemoryActivities expects concrete types or specific interfaces.
		// For MemoryLockManager, it needs a cache.LockManager.
		// We pass our mockActivityLockManager which implements the Acquire method.
		// The concrete MemoryLockManager wrapper will call this mock.
		// This requires MemoryLockManager to take an interface that mockActivityLockManager satisfies,
		// or mockActivityLockManager to be a full cache.LockManager mock.
		// For simplicity, let's assume MemoryLockManager can be instantiated with our mock for testing.
		// This highlights a dependency challenge in testing activities.
		// A better way: MemoryActivities takes interfaces for all its dependencies.

		// Create a "real" MemoryLockManager but with a mocked internalLockManager
		// This is not ideal. The MemoryActivities struct should take a memory.MemoryLockManager *interface*
		// For now, let's proceed with the assumption that we can mock the underlying calls.
		// We'll mock the methods on the *wrapper* MemoryLockManager if it calls its internal one.
		// Or, we make MemoryLockManager itself an interface, and mock that.
		// Given MemoryLockManager is thin, we can mock its `Acquire` directly if it were an interface.
		// Since it's concrete, we mock the `cache.LockManager` it wraps.

		testLockManager, _ := memory.NewMemoryLockManager(currentLockMgr, "testprefix:")

		acts := NewMemoryActivities(currentStore, *testLockManager, tokenMgr, flushStrategy, memResourceCfg)
		currentEnv := ts.NewTestActivityEnvironment() // Fresh env for each subtest
		currentEnv.RegisterActivity(acts.FlushMemory)


		initialMessages := []llm.Message{
			{Content: "Old message 1, very long, many tokens here, definitely over twenty five"}, // 10 tokens
			{Content: "Old message 2 also many tokens, over threshold"}, // 8 tokens
			{Content: "Recent message 3"}, // 3 tokens
		} // Total 21 tokens. MaxTokens 50. Threshold 25. Not flushing.
		   // Let's make it flush:
		initialMessagesFlush := []llm.Message{
			{Content: "This is the first message to keep always"}, // 8 tokens
			{Content: "This is a long message that will be summarized as part of the flush process one"}, //15 tokens
			{Content: "This is another long message that will be summarized as part of the flush process two"}, //16 tokens
			{Content: "This is the last message to keep always"}, // 8 tokens
		} // 8+15+16+8 = 47 tokens. Threshold 0.5*50=25. Should flush.
		  // Summarizer keeps 1st and last. Summarizes middle 2 (15+16=31 tokens).
		  // Summary msg (target 10t) + first (8t) + last (8t) = ~26 tokens.

		currentLockMgr.On("Acquire", mock.Anything, "testprefix:"+input.MemoryInstanceKey, mock.AnythingOfType("time.Duration")).Return(currentLock, nil).Once()
		currentLock.On("Release", mock.Anything).Return(nil).Once()
		currentStore.On("ReadMessages", mock.Anything, input.MemoryInstanceKey).Return(initialMessagesFlush, nil).Once()
		// Expect ReplaceMessages to be called with the new set (summary + 2 kept messages)
		currentStore.On("ReplaceMessages", mock.Anything, input.MemoryInstanceKey, mock.MatchedBy(func(msgs []llm.Message) bool {
			return len(msgs) == 3 && msgs[0].Role == "system" && msgs[0].Content != "" // Summary + 2 kept
		})).Return(nil).Once()

		future, err := currentEnv.ExecuteActivity(acts.FlushMemory, input)
		require.NoError(t, err)

		var output FlushMemoryActivityOutput
		require.NoError(t, future.Get(&output))
		assert.Empty(t, output.Error)
		assert.True(t, output.SummaryGenerated)
		assert.Equal(t, 3, output.MessagesKept) // Summary + 2 original
		assert.LessOrEqual(t, output.TokensKept, memResourceCfg.MaxTokens) // Should be around 10 (summary) + 8 + 8 = 26
		assert.Greater(t, output.TokensKept, 18) // Ensure it's not empty

		currentLockMgr.AssertExpectations(t)
		currentLock.AssertExpectations(t)
		currentStore.AssertExpectations(t)
	})

	// --- Test Case 2: Lock acquisition fails ---
	t.Run("LockAcquisitionFails", func(t *testing.T) {
		currentStore := new(mockActivityMemoryStore)
		currentLockMgr := new(mockActivityLockManager)
		// currentLock := new(mockActivityLock) // Not used if acquire fails

		testLockManager, _ := memory.NewMemoryLockManager(currentLockMgr, "testprefix:")
		acts := NewMemoryActivities(currentStore, *testLockManager, tokenMgr, flushStrategy, memResourceCfg)
		currentEnv := ts.NewTestActivityEnvironment()
		currentEnv.RegisterActivity(acts.FlushMemory)

		currentLockMgr.On("Acquire", mock.Anything, "testprefix:"+input.MemoryInstanceKey, mock.AnythingOfType("time.Duration")).Return(nil, errors.New("lock busy")).Once()

		future, err := currentEnv.ExecuteActivity(acts.FlushMemory, input)
		require.NoError(t, err) // Activity execution itself doesn't err, error is in the result (or ApplicationError)

		var output FlushMemoryActivityOutput
		err = future.Get(&output) // This will error because activity returned ApplicationError
		require.Error(t, err)
		var appErr *temporal.ApplicationError
		require.True(t, errors.As(err, &appErr), "Error should be a temporal.ApplicationError")
		assert.True(t, appErr.Type() == "LOCK_ACQUISITION_FAILED" || appErr.Message() == "failed to acquire lock for flush")


		currentLockMgr.AssertExpectations(t)
		currentStore.AssertNotCalled(t, "ReadMessages") // Should not proceed if lock fails
	})

	// --- Test Case 3: ReadMessages fails ---
	t.Run("ReadMessagesFails", func(t *testing.T) {
		currentStore := new(mockActivityMemoryStore)
		currentLockMgr := new(mockActivityLockManager)
		currentLock := new(mockActivityLock)

		testLockManager, _ := memory.NewMemoryLockManager(currentLockMgr, "testprefix:")
		acts := NewMemoryActivities(currentStore, *testLockManager, tokenMgr, flushStrategy, memResourceCfg)
		currentEnv := ts.NewTestActivityEnvironment()
		currentEnv.RegisterActivity(acts.FlushMemory)

		currentLockMgr.On("Acquire", mock.Anything, mock.Anything, mock.Anything).Return(currentLock, nil).Once()
		currentLock.On("Release", mock.Anything).Return(nil).Once()
		currentStore.On("ReadMessages", mock.Anything, input.MemoryInstanceKey).Return(nil, errors.New("redis unavailable")).Once()

		future, err := currentEnv.ExecuteActivity(acts.FlushMemory, input)
		require.NoError(t, err)
		var output FlushMemoryActivityOutput
		err = future.Get(&output)
		require.Error(t, err)
		var appErr *temporal.ApplicationError
		require.True(t, errors.As(err, &appErr))
		assert.Equal(t, "READ_FAILED", appErr.Type())

		currentLockMgr.AssertExpectations(t)
		currentLock.AssertExpectations(t)
		currentStore.AssertExpectations(t)
	})

	// --- Test Case 4: No flush needed ---
	t.Run("NoFlushNeeded", func(t *testing.T) {
		currentStore := new(mockActivityMemoryStore)
		currentLockMgr := new(mockActivityLockManager)
		currentLock := new(mockActivityLock)

		testLockManager, _ := memory.NewMemoryLockManager(currentLockMgr, "testprefix:")
		acts := NewMemoryActivities(currentStore, *testLockManager, tokenMgr, flushStrategy, memResourceCfg)
		currentEnv := ts.NewTestActivityEnvironment()
		currentEnv.RegisterActivity(acts.FlushMemory)

		// Messages that are below the flush threshold (0.5 * 50 = 25 tokens)
		fewMessages := []llm.Message{ {Content: "Short one"}, {Content: "Another short"} } // e.g. 2+2 = 4 tokens

		currentLockMgr.On("Acquire", mock.Anything, mock.Anything, mock.Anything).Return(currentLock, nil).Once()
		currentLock.On("Release", mock.Anything).Return(nil).Once()
		currentStore.On("ReadMessages", mock.Anything, input.MemoryInstanceKey).Return(fewMessages, nil).Once()
		// If EnforceLimits does something (e.g. MaxMessages is hit), ReplaceMessages would be called.
		// Assuming MaxMessages is high for this test.
		// currentStore.On("ReplaceMessages", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()


		future, err := currentEnv.ExecuteActivity(acts.FlushMemory, input)
		require.NoError(t, err)
		var output FlushMemoryActivityOutput
		require.NoError(t, future.Get(&output))

		assert.Empty(t, output.Error)
		assert.False(t, output.SummaryGenerated)
		assert.Equal(t, len(fewMessages), output.MessagesKept)
		// TokensKept should be sum of tokens in fewMessages
		// This requires TokenManager to calculate it. The mock store doesn't give token counts.
		// The activity calculates it.
		// Example: 2+2 = 4 tokens
		assert.Equal(t, 4, output.TokensKept)


		currentLockMgr.AssertExpectations(t)
		currentLock.AssertExpectations(t)
		currentStore.AssertExpectations(t)
		// currentStore.AssertNotCalled(t, "ReplaceMessages") // This might be called if EnforceLimits does something.
	})


}
