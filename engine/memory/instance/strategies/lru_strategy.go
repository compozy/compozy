package strategies

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/memory/core"
	lru "github.com/hashicorp/golang-lru/v2"
)

// LRUStrategy implements a Least Recently Used flushing strategy using hashicorp/golang-lru
type LRUStrategy struct {
	cache         *lru.Cache[int, time.Time] // Tracks message index -> last access time
	config        *core.FlushingStrategyConfig
	flushDecision *FlushDecisionEngine
	tokenCounter  core.TokenCounter
	options       *StrategyOptions
	mu            sync.RWMutex
}

// NewLRUStrategy creates a new LRU flushing strategy
func NewLRUStrategy(config *core.FlushingStrategyConfig, options *StrategyOptions) (*LRUStrategy, error) {
	resolvedOptions := resolveLRUOptions(options)
	if err := validateLRUOptions(resolvedOptions); err != nil {
		return nil, err
	}
	cacheSize := sanitizeLRUCacheSize(resolvedOptions.CacheSize)
	cache, err := lru.New[int, time.Time](cacheSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create LRU cache: %w", err)
	}
	thresholdPercent := resolveLRUThreshold(config)
	tokenCounter := buildLRUTokenCounter(resolvedOptions)
	return &LRUStrategy{
		cache:         cache,
		config:        config,
		flushDecision: NewFlushDecisionEngine(thresholdPercent),
		tokenCounter:  tokenCounter,
		options:       resolvedOptions,
	}, nil
}

func resolveLRUOptions(options *StrategyOptions) *StrategyOptions {
	if options != nil {
		return options
	}
	return GetDefaultStrategyOptions()
}

func validateLRUOptions(options *StrategyOptions) error {
	if options.LRUTargetCapacityPercent < 0 || options.LRUTargetCapacityPercent > 1 {
		return fmt.Errorf(
			"LRUTargetCapacityPercent must be between 0 and 1, got %f",
			options.LRUTargetCapacityPercent,
		)
	}
	return nil
}

func sanitizeLRUCacheSize(size int) int {
	const (
		defaultCacheSize = 1000
		maxCacheSize     = 10000
	)
	if size <= 0 {
		size = defaultCacheSize
	}
	if size > maxCacheSize {
		return maxCacheSize
	}
	return size
}

func resolveLRUThreshold(config *core.FlushingStrategyConfig) float64 {
	if config != nil && config.SummarizeThreshold > 0 {
		return config.SummarizeThreshold
	}
	return 0.8
}

func buildLRUTokenCounter(options *StrategyOptions) core.TokenCounter {
	baseCounter := NewGPTTokenCounterAdapter()
	estimationStrategy := options.TokenEstimationStrategy
	if estimationStrategy == "" {
		estimationStrategy = core.EnglishEstimation
	}
	tokenEstimator := core.NewTokenEstimator(estimationStrategy)
	return core.NewTokenCounterWithFallback(baseCounter, tokenEstimator)
}

// ShouldFlush determines if a flush should be triggered based on current state
func (s *LRUStrategy) ShouldFlush(tokenCount, messageCount int, config *core.Resource) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.flushDecision.ShouldFlush(tokenCount, messageCount, config)
}

// UpdateAccess marks a message as accessed (optional method for external use)
func (s *LRUStrategy) UpdateAccess(messageIndex int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache.Add(messageIndex, time.Now())
}

// PerformFlush executes the LRU flush operation
func (s *LRUStrategy) PerformFlush(
	ctx context.Context,
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
	// Calculate target message count
	targetCount := s.calculateTargetMessageCount(len(messages), config)
	if targetCount >= len(messages) {
		// No need to evict
		return &core.FlushMemoryActivityOutput{
			Success:          true,
			SummaryGenerated: false,
			MessageCount:     len(messages),
			TokenCount:       s.calculateTotalTokens(ctx, messages),
		}, nil
	}
	// Perform LRU eviction
	remainingMessages := s.performLRUEviction(messages, targetCount)
	remainingTokens := s.calculateTotalTokens(ctx, remainingMessages)
	return &core.FlushMemoryActivityOutput{
		Success:          true,
		SummaryGenerated: false,
		MessageCount:     len(remainingMessages),
		TokenCount:       remainingTokens,
	}, nil
}

// GetType returns the strategy type
func (s *LRUStrategy) GetType() core.FlushingStrategyType {
	return core.LRUFlushing
}

// calculateTargetMessageCount determines the target number of messages after flush
func (s *LRUStrategy) calculateTargetMessageCount(currentCount int, config *core.Resource) int {
	// Target: reduce to configured capacity to allow room for growth
	targetPercent := s.options.LRUTargetCapacityPercent
	if targetPercent <= 0 {
		targetPercent = 0.6 // Default to 60%
	}
	if config.MaxMessages > 0 {
		return int(float64(config.MaxMessages) * targetPercent)
	}
	// Default: keep configured percentage of messages
	return int(float64(currentCount) * targetPercent)
}

// calculateTotalTokens calculates total tokens in all messages
func (s *LRUStrategy) calculateTotalTokens(ctx context.Context, messages []llm.Message) int {
	total := 0
	for _, msg := range messages {
		count, err := s.tokenCounter.CountTokens(ctx, msg.Content)
		if err != nil {
			// TokenCounter should handle fallback internally
			count = 0
		}
		total += count
	}
	return total
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
	maxFlush = max(totalMsgs/2, minFlush)
	return minFlush, maxFlush
}

// indexedMessage holds a message with its access time and original index
type indexedMessage struct {
	index      int
	accessTime time.Time
	message    llm.Message
}

// performLRUEviction evicts messages based on LRU access patterns
func (s *LRUStrategy) performLRUEviction(messages []llm.Message, targetCount int) []llm.Message {
	// Build indexed messages with access times
	indexedMessages := s.buildIndexedMessages(messages)
	// Sort by access time (oldest first for eviction)
	sort.Slice(indexedMessages, func(i, j int) bool {
		return indexedMessages[i].accessTime.Before(indexedMessages[j].accessTime)
	})
	// Determine which messages to keep
	remainingIndices := s.selectMessagesToKeep(indexedMessages, len(messages)-targetCount)
	// Collect remaining messages in original order
	remainingMessages := s.collectRemainingMessages(messages, remainingIndices)
	// Update cache with remaining messages
	s.updateCacheAfterEviction(messages, remainingMessages, remainingIndices)
	return remainingMessages
}

// buildIndexedMessages creates indexed messages with their access times
func (s *LRUStrategy) buildIndexedMessages(messages []llm.Message) []indexedMessage {
	indexedMessages := make([]indexedMessage, len(messages))
	now := time.Now()
	for i, msg := range messages {
		accessTime, found := s.cache.Get(i)
		if !found {
			// Never accessed - use message order as fallback
			// Older messages get older times
			accessTime = now.Add(time.Duration(-len(messages)+i) * time.Millisecond)
			s.cache.Add(i, accessTime)
		}
		indexedMessages[i] = indexedMessage{
			index:      i,
			accessTime: accessTime,
			message:    msg,
		}
	}
	return indexedMessages
}

// selectMessagesToKeep determines which messages to keep based on LRU policy
func (s *LRUStrategy) selectMessagesToKeep(indexedMessages []indexedMessage, numToEvict int) map[int]bool {
	remainingIndices := make(map[int]bool)
	// Mark messages to keep (skip the first numToEvict)
	for i := numToEvict; i < len(indexedMessages); i++ {
		remainingIndices[indexedMessages[i].index] = true
	}
	return remainingIndices
}

// collectRemainingMessages collects messages that should be kept
func (s *LRUStrategy) collectRemainingMessages(messages []llm.Message, remainingIndices map[int]bool) []llm.Message {
	remainingMessages := make([]llm.Message, 0, len(remainingIndices))
	for i, msg := range messages {
		if remainingIndices[i] {
			remainingMessages = append(remainingMessages, msg)
		}
	}
	return remainingMessages
}

// updateCacheAfterEviction updates the cache to only contain remaining messages
func (s *LRUStrategy) updateCacheAfterEviction(
	messages []llm.Message,
	remainingMessages []llm.Message,
	remainingIndices map[int]bool,
) {
	// Build a map of original indices to access times for O(1) lookup
	accessTimeMap := make(map[int]time.Time)
	for origIdx := range remainingIndices {
		if accessTime, found := s.cache.Get(origIdx); found {
			accessTimeMap[origIdx] = accessTime
		}
	}
	// Clear the cache and repopulate with remaining messages
	s.cache.Purge()
	// Create a mapping from new index to original index for remaining messages
	newToOrigIndex := make([]int, len(remainingMessages))
	newIdx := 0
	for origIdx := range messages {
		if remainingIndices[origIdx] {
			newToOrigIndex[newIdx] = origIdx
			newIdx++
		}
	}
	// Rebuild cache with new indices
	for newIdx := range remainingMessages {
		origIdx := newToOrigIndex[newIdx]
		if accessTime, exists := accessTimeMap[origIdx]; exists {
			s.cache.Add(newIdx, accessTime)
		} else {
			s.cache.Add(newIdx, time.Now())
		}
	}
}
