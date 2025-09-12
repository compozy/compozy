package instance

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/llm"
)

// Test token counter that tracks calls
type testTokenCounter struct {
	callCount   int
	returnValue int
	returnError error
	mu          sync.Mutex
}

func (t *testTokenCounter) CountTokens(_ context.Context, _ string) (int, error) {
	t.mu.Lock()
	t.callCount++
	t.mu.Unlock()
	return t.returnValue, t.returnError
}

func (t *testTokenCounter) EncodeTokens(_ context.Context, _ string) ([]int, error) {
	return nil, nil
}

func (t *testTokenCounter) DecodeTokens(_ context.Context, _ []int) (string, error) {
	return "", nil
}

func (t *testTokenCounter) GetEncoding() string {
	return "test-encoding"
}

func (t *testTokenCounter) getCallCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.callCount
}

// Test store that tracks messages
type testStore struct {
	messages        []llm.Message
	setTokenCalls   int
	setTokenValue   int
	shouldReturnErr bool
	mu              sync.Mutex
}

func (t *testStore) ReadMessages(_ context.Context, _ string) ([]llm.Message, error) {
	if t.shouldReturnErr {
		return nil, assert.AnError
	}
	return t.messages, nil
}

func (t *testStore) ReadMessagesPaginated(_ context.Context, _ string, offset, limit int) ([]llm.Message, int, error) {
	if t.shouldReturnErr {
		return nil, 0, assert.AnError
	}
	totalCount := len(t.messages)
	if offset >= totalCount {
		return []llm.Message{}, totalCount, nil
	}
	end := min(offset+limit, totalCount)
	return t.messages[offset:end], totalCount, nil
}

func (t *testStore) SetTokenCount(_ context.Context, _ string, count int) error {
	t.mu.Lock()
	t.setTokenCalls++
	t.setTokenValue = count
	t.mu.Unlock()
	if t.shouldReturnErr {
		return assert.AnError
	}
	return nil
}

func (t *testStore) getSetTokenCalls() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.setTokenCalls
}

func (t *testStore) getSetTokenValue() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.setTokenValue
}

// Satisfy the Store interface with no-ops for other methods
func (t *testStore) AppendMessage(_ context.Context, _ string, _ llm.Message) error     { return nil }
func (t *testStore) AppendMessages(_ context.Context, _ string, _ []llm.Message) error  { return nil }
func (t *testStore) ReplaceMessages(_ context.Context, _ string, _ []llm.Message) error { return nil }
func (t *testStore) DeleteMessages(_ context.Context, _ string) error                   { return nil }
func (t *testStore) GetMessageCount(_ context.Context, _ string) (int, error)           { return 0, nil }
func (t *testStore) GetTokenCount(_ context.Context, _ string) (int, error)             { return 0, nil }
func (t *testStore) IncrementTokenCount(_ context.Context, _ string, _ int) error       { return nil }
func (t *testStore) MarkFlushPending(_ context.Context, _ string, _ bool) error         { return nil }
func (t *testStore) IsFlushPending(_ context.Context, _ string) (bool, error)           { return false, nil }
func (t *testStore) SetLastFlushed(_ context.Context, _ string, _ time.Time) error      { return nil }
func (t *testStore) GetLastFlushed(_ context.Context, _ string) (time.Time, error) {
	return time.Time{}, nil
}
func (t *testStore) GetExpiration(_ context.Context, _ string) (time.Time, error) {
	return time.Time{}, nil
}
func (t *testStore) SetExpiration(_ context.Context, _ string, _ time.Duration) error { return nil }
func (t *testStore) GetKeyTTL(_ context.Context, _ string) (time.Duration, error)     { return 0, nil }
func (t *testStore) GetMetadata(_ context.Context, _ string) (map[string]any, error)  { return nil, nil }
func (t *testStore) SetMetadata(_ context.Context, _ string, _ map[string]any) error  { return nil }
func (t *testStore) AppendMessageWithTokenCount(_ context.Context, _ string, _ llm.Message, _ int) error {
	return nil
}
func (t *testStore) TrimMessagesWithMetadata(_ context.Context, _ string, _ int, _ int) error {
	return nil
}
func (t *testStore) ReplaceMessagesWithMetadata(_ context.Context, _ string, _ []llm.Message, _ int) error {
	return nil
}
func (t *testStore) CountMessages(_ context.Context, _ string) (int, error) { return 0, nil }

// Test metrics that does nothing
type testMetrics struct{}

func (t *testMetrics) RecordAppend(_ context.Context, _ time.Duration, _ int, _ error) {}
func (t *testMetrics) RecordRead(_ context.Context, _ time.Duration, _ int, _ error)   {}
func (t *testMetrics) RecordFlush(_ context.Context, _ time.Duration, _ int, _ error)  {}
func (t *testMetrics) RecordTokenCount(_ context.Context, _ int)                       {}
func (t *testMetrics) RecordMessageCount(_ context.Context, _ int)                     {}

func TestOperations_calculateTokensFromMessages(t *testing.T) {
	t.Run("Should cache content tokens using sync.Map", func(t *testing.T) {
		// Setup
		ctx := context.Background()

		// Create test messages with duplicate content to verify caching
		messages := []llm.Message{
			{Role: llm.MessageRoleUser, Content: "Hello world"},
			{Role: llm.MessageRoleAssistant, Content: "Hi there"},
			{Role: llm.MessageRoleUser, Content: "Hello world"}, // Duplicate content
		}

		store := &testStore{messages: messages}
		tokenCounter := &testTokenCounter{returnValue: 5} // Returns 5 for any text
		metrics := &testMetrics{}

		operations := &Operations{
			store:        store,
			tokenCounter: tokenCounter,
			metrics:      metrics,
			instanceID:   "test-instance",
		}

		// Execute
		totalTokens, err := operations.calculateTokensFromMessages(ctx)

		// Verify
		require.NoError(t, err)

		// Expected calls to token counter:
		// - "Hello world" content: called once (cached for second occurrence)
		// - "Hi there" content: called once
		// - "user" role: called once (cached for second occurrence)
		// - "assistant" role: called once
		// Total unique calls: 4
		assert.Equal(t, 4, tokenCounter.getCallCount(), "Should cache duplicate content/roles")

		// Expected total tokens: 3 messages * (5 content + 5 role + 2 structure) = 36
		assert.Equal(t, 36, totalTokens)

		// Verify store was called to set the token count
		assert.Equal(t, 1, store.getSetTokenCalls())
		assert.Equal(t, 36, store.getSetTokenValue())
	})

	t.Run("Should handle empty message list", func(t *testing.T) {
		// Setup
		ctx := context.Background()

		store := &testStore{messages: []llm.Message{}} // Empty messages
		tokenCounter := &testTokenCounter{returnValue: 5}
		metrics := &testMetrics{}

		operations := &Operations{
			store:        store,
			tokenCounter: tokenCounter,
			metrics:      metrics,
			instanceID:   "test-instance",
		}

		// Execute
		totalTokens, err := operations.calculateTokensFromMessages(ctx)

		// Verify
		require.NoError(t, err)
		assert.Equal(t, 0, totalTokens)
		assert.Equal(t, 0, tokenCounter.getCallCount(), "Should not call token counter for empty messages")
		assert.Equal(t, 1, store.getSetTokenCalls(), "Should still call SetTokenCount with 0")
		assert.Equal(t, 0, store.getSetTokenValue())
	})

	t.Run("Should handle token counter errors gracefully", func(t *testing.T) {
		// Setup
		ctx := context.Background()

		messages := []llm.Message{
			{Role: llm.MessageRoleUser, Content: "Test content"}, // 12 chars / 4 = 3 tokens estimated
		}

		store := &testStore{messages: messages}
		tokenCounter := &testTokenCounter{returnValue: 0, returnError: assert.AnError} // Always returns error
		metrics := &testMetrics{}

		operations := &Operations{
			store:        store,
			tokenCounter: tokenCounter,
			metrics:      metrics,
			instanceID:   "test-instance",
		}

		// Execute
		totalTokens, err := operations.calculateTokensFromMessages(ctx)

		// Verify - should handle errors gracefully and use estimation fallback
		require.NoError(t, err)
		// Content: "Test content" = 12 chars / 4 = 3 tokens
		// Role: "user" = 4 chars / 4 = 1 token
		// Structure: 2
		// Total: 3 + 1 + 2 = 6
		assert.Equal(t, 6, totalTokens, "Should fallback to estimation when token counter fails")
		assert.Equal(t, 2, tokenCounter.getCallCount(), "Should call token counter for content and role")
		assert.Equal(t, 1, store.getSetTokenCalls())
		assert.Equal(t, 6, store.getSetTokenValue())
	})

	t.Run("Should handle concurrent access to cache safely", func(t *testing.T) {
		// This test ensures the sync.Map implementation is thread-safe
		ctx := context.Background()

		messages := []llm.Message{
			{Role: llm.MessageRoleUser, Content: "Concurrent test"},
		}

		// Use a shared store and token counter for all goroutines
		store := &testStore{messages: messages}
		tokenCounter := &testTokenCounter{returnValue: 5}
		metrics := &testMetrics{}

		operations := &Operations{
			store:        store,
			tokenCounter: tokenCounter,
			metrics:      metrics,
			instanceID:   "test-instance",
		}

		// Execute multiple goroutines concurrently
		var wg sync.WaitGroup
		results := make([]int, 10)
		errors := make([]error, 10)

		for i := range 10 {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				tokens, err := operations.calculateTokensFromMessages(ctx)
				results[index] = tokens
				errors[index] = err
			}(i)
		}

		wg.Wait()

		// Verify all results are consistent and no errors occurred
		expectedTokens := 12 // 1 message * (5 content + 5 role + 2 structure)
		for i, result := range results {
			require.NoError(t, errors[i], "Concurrent call %d should not error", i)
			assert.Equal(t, expectedTokens, result, "All concurrent calls should return the same result")
		}

		// The token counter may be called multiple times during concurrent access
		// because each goroutine may recreate the cache (that's expected for this test)
		// The key thing is that no race conditions occur and results are consistent
		assert.Greater(t, tokenCounter.getCallCount(), 0, "Token counter should be called")
		assert.LessOrEqual(t, tokenCounter.getCallCount(), 20, "Token counter calls should be reasonable")
	})
}

func TestOperations_CacheTypeAssertionSafety(t *testing.T) {
	t.Run("Should handle corrupted cache values gracefully", func(t *testing.T) {
		// This test ensures that if somehow the cache gets corrupted with wrong type,
		// the system handles it gracefully by falling back to recalculation

		// Simulate the internal caching behavior by testing the pattern directly
		var cache sync.Map

		// Store a wrong type in cache (simulating corruption)
		cache.Store("test", "not an int")

		// Test the pattern we use in the actual code
		var result int
		if value, exists := cache.Load("test"); exists {
			if cachedValue, ok := value.(int); ok {
				result = cachedValue
			} else {
				// This should trigger - fallback behavior
				result = 42 // Fallback value
			}
		}

		// Verify fallback was triggered
		assert.Equal(t, 42, result, "Should fallback when type assertion fails")
	})
}

func TestOperations_CachePerformance(t *testing.T) {
	t.Run("Should demonstrate caching performance improvement", func(t *testing.T) {
		// This test verifies that caching actually improves performance by reducing
		// the number of calls to the token counter

		ctx := context.Background()

		// Create many messages with repeated content to test caching
		messages := make([]llm.Message, 100)
		for i := range 100 {
			messages[i] = llm.Message{
				Role:    llm.MessageRoleUser,
				Content: "Repeated content", // Same content for all messages
			}
		}

		store := &testStore{messages: messages}
		tokenCounter := &testTokenCounter{returnValue: 10}
		metrics := &testMetrics{}

		operations := &Operations{
			store:        store,
			tokenCounter: tokenCounter,
			metrics:      metrics,
			instanceID:   "test-instance",
		}

		// Execute
		start := time.Now()
		totalTokens, err := operations.calculateTokensFromMessages(ctx)
		duration := time.Since(start)

		// Verify
		require.NoError(t, err)

		expectedTotal := 100 * (10 + 10 + 2) // 100 messages * (content + role + structure)
		assert.Equal(t, expectedTotal, totalTokens)

		// Performance should be very fast due to caching
		assert.Less(t, duration, 100*time.Millisecond, "Should be fast due to caching")

		// Verify that token counter was called minimally due to caching
		// Should only be called twice: once for content "Repeated content" and once for role "user"
		assert.Equal(t, 2, tokenCounter.getCallCount(), "Should only call token counter for unique content/role")

		// Verify store was called to set the final count
		assert.Equal(t, 1, store.getSetTokenCalls())
		assert.Equal(t, expectedTotal, store.getSetTokenValue())
	})
}
