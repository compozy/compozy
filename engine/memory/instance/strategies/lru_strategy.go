package strategies

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/memory/core"
	"github.com/dgraph-io/ristretto"
)

// LRUStrategy implements a Least Recently Used flushing strategy using ristretto
type LRUStrategy struct {
	cache         *ristretto.Cache[int, messageInfo]
	config        *core.FlushingStrategyConfig
	flushDecision *FlushDecisionEngine
	tokenCounter  TokenCounter
	options       *StrategyOptions
	mu            sync.RWMutex
	messageAccess map[int]time.Time // Track message access times
}

// messageInfo holds information about a message for LRU tracking
type messageInfo struct {
	index      int
	lastAccess time.Time
}

// messageWithTime pairs a message index with its last access time for LRU sorting
type messageWithTime struct {
	index      int
	accessTime time.Time
}

// NewLRUStrategy creates a new LRU flushing strategy using ristretto
func NewLRUStrategy(config *core.FlushingStrategyConfig, options *StrategyOptions) (*LRUStrategy, error) {
	// Use default options if none provided
	if options == nil {
		options = GetDefaultStrategyOptions()
	}

	cacheSize := options.CacheSize
	if cacheSize <= 0 {
		cacheSize = 1000 // Default cache size for message tracking
	}

	// Create ristretto cache optimized for LRU access pattern tracking
	cache, err := ristretto.NewCache(&ristretto.Config[int, messageInfo]{
		NumCounters: int64(cacheSize * 10), // 10x the cache size for better admission control
		MaxCost:     int64(cacheSize),      // Each message info costs 1 unit
		BufferItems: 64,                    // Standard buffer size
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create ristretto cache for LRU strategy: %w", err)
	}

	thresholdPercent := 0.8 // Default to 80%
	if config != nil && config.SummarizeThreshold > 0 {
		thresholdPercent = config.SummarizeThreshold
	}

	return &LRUStrategy{
		cache:         cache,
		config:        config,
		flushDecision: NewFlushDecisionEngine(thresholdPercent),
		tokenCounter:  NewGPTTokenCounter(),
		options:       options,
		messageAccess: make(map[int]time.Time),
	}, nil
}

// ShouldFlush determines if a flush should be triggered based on current state
func (s *LRUStrategy) ShouldFlush(tokenCount, messageCount int, config *core.Resource) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.flushDecision.ShouldFlush(tokenCount, messageCount, config)
}

// PerformFlush executes the LRU flush operation
func (s *LRUStrategy) PerformFlush(
	_ context.Context,
	messages []llm.Message,
	config *core.Resource,
) (*core.FlushMemoryActivityOutput, error) {
	if len(messages) == 0 {
		return &core.FlushMemoryActivityOutput{
			Success:          true,
			SummaryGenerated: false,
			MessageCount:     0,
			TokenCount:       0,
		}, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Update access patterns for all messages
	now := time.Now()
	for i := range messages {
		s.updateMessageAccess(i, now)
	}

	// Calculate how many messages to remove based on LRU
	messagesToRemove := s.calculateLRUMessagesToRemove(len(messages), config)
	if messagesToRemove == 0 {
		return &core.FlushMemoryActivityOutput{
			Success:          true,
			SummaryGenerated: false,
			MessageCount:     len(messages),
			TokenCount:       s.calculateTotalTokens(messages),
		}, nil
	}

	// Select messages to evict based on LRU order
	evictedIndices := s.selectLRUMessages(messages, messagesToRemove)

	// Calculate tokens in evicted messages
	tokensFlushed := s.calculateEvictedTokens(messages, evictedIndices)
	remainingMessages := len(messages) - len(evictedIndices)

	// Clean up access tracking for evicted messages
	s.cleanupEvictedMessages(evictedIndices)

	return &core.FlushMemoryActivityOutput{
		Success:          true,
		SummaryGenerated: false,
		MessageCount:     remainingMessages,
		TokenCount:       tokensFlushed,
	}, nil
}

// GetType returns the strategy type
func (s *LRUStrategy) GetType() core.FlushingStrategyType {
	return core.LRUFlushing
}

// updateMessageAccess records access time for a message and updates the LRU cache
func (s *LRUStrategy) updateMessageAccess(index int, accessTime time.Time) {
	s.messageAccess[index] = accessTime

	// Update ristretto cache with message info
	info := messageInfo{
		index:      index,
		lastAccess: accessTime,
	}
	s.cache.Set(index, info, 1) // Cost of 1 per message
}

// calculateLRUMessagesToRemove determines how many messages to remove based on LRU strategy
func (s *LRUStrategy) calculateLRUMessagesToRemove(currentCount int, config *core.Resource) int {
	// Target: reduce to configured capacity to allow room for growth
	targetPercent := s.options.LRUTargetCapacityPercent

	if config.MaxMessages > 0 {
		targetCount := int(float64(config.MaxMessages) * targetPercent)
		if currentCount > targetCount {
			return currentCount - targetCount
		}
	}

	// Default: remove configured minimum percentage of messages based on LRU
	return int(float64(currentCount) * s.options.LRUMinFlushPercent)
}

// selectLRUMessages selects which messages to evict based on efficient LRU tracking
func (s *LRUStrategy) selectLRUMessages(messages []llm.Message, count int) []int {
	if count <= 0 || count >= len(messages) {
		return []int{}
	}

	// Use a more efficient approach: find the count oldest messages using partial selection
	// This avoids sorting the entire array and is O(N + k log k) where k = count
	oldestIndices := s.findOldestMessages(messages, count)
	return oldestIndices
}

// findOldestMessages efficiently finds the count oldest messages using partial selection
// Time complexity: O(N + k log k) where N = total messages, k = count to evict
// This is much more efficient than O(N log N) when k << N (typical for memory flushing)
func (s *LRUStrategy) findOldestMessages(messages []llm.Message, count int) []int {
	if count <= 0 {
		return []int{}
	}
	if count >= len(messages) {
		// Return all indices if we need to evict everything
		result := make([]int, len(messages))
		for i := range messages {
			result[i] = i
		}
		return result
	}

	// For small count, use a simple approach with a slice that we keep sorted
	candidates := make([]messageWithTime, 0, count+1)

	for i := range messages {
		lastAccess, exists := s.messageAccess[i]
		if !exists {
			// If no access time recorded, use index as fallback (older = smaller index)
			lastAccess = time.Unix(int64(i), 0)
		}

		msg := messageWithTime{
			index:      i,
			accessTime: lastAccess,
		}

		if len(candidates) < count {
			// Haven't filled our candidate list yet, just add and keep sorted
			candidates = append(candidates, msg)
			s.insertSorted(candidates, len(candidates)-1)
		} else if msg.accessTime.Before(candidates[count-1].accessTime) {
			// This message is older than our newest candidate, replace it
			candidates[count-1] = msg
			s.insertSorted(candidates, count-1)
		}
	}

	// Extract the indices of the oldest messages
	result := make([]int, len(candidates))
	for i, msg := range candidates {
		result[i] = msg.index
	}

	return result
}

// insertSorted maintains sorted order by moving element at pos to its correct position
// Sorts by accessTime (oldest first)
func (s *LRUStrategy) insertSorted(candidates []messageWithTime, pos int) {
	if pos <= 0 {
		return
	}

	// Move the element at pos to its correct position (insertion sort style)
	elem := candidates[pos]
	i := pos - 1

	// Shift elements that are newer (have later access times) to the right
	for i >= 0 && candidates[i].accessTime.After(elem.accessTime) {
		candidates[i+1] = candidates[i]
		i--
	}

	candidates[i+1] = elem
}

// calculateEvictedTokens calculates total tokens in evicted messages
func (s *LRUStrategy) calculateEvictedTokens(messages []llm.Message, evictedIndices []int) int {
	tokensFlushed := 0
	for _, index := range evictedIndices {
		if index >= 0 && index < len(messages) {
			tokensFlushed += s.tokenCounter.CountTokens(messages[index])
		}
	}
	return tokensFlushed
}

// calculateTotalTokens calculates total tokens in all messages
func (s *LRUStrategy) calculateTotalTokens(messages []llm.Message) int {
	total := 0
	for _, msg := range messages {
		total += s.tokenCounter.CountTokens(msg)
	}
	return total
}

// cleanupEvictedMessages removes access tracking for evicted messages
func (s *LRUStrategy) cleanupEvictedMessages(evictedIndices []int) {
	for _, index := range evictedIndices {
		delete(s.messageAccess, index)
		s.cache.Del(index)
	}
}

// GetMinMaxToFlush returns the min/max number of messages to flush for this strategy
func (s *LRUStrategy) GetMinMaxToFlush(
	_ context.Context,
	totalMsgs int,
	_ int, // currentTokens - unused for LRU
	_ int, // maxTokens - unused for LRU
) (minFlush, maxFlush int) {
	// LRU strategy: minimum 1 message, maximum 50% of total messages
	minFlush = 1
	maxFlush = totalMsgs / 2
	if maxFlush < minFlush {
		maxFlush = minFlush
	}
	return minFlush, maxFlush
}
