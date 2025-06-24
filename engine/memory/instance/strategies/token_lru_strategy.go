package strategies

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/memory/core"
	"github.com/dgraph-io/ristretto"
)

// TokenAwareLRUStrategy implements a token-cost-aware LRU flushing strategy
type TokenAwareLRUStrategy struct {
	cache         *ristretto.Cache[int, MessageWithTokens]
	config        *core.FlushingStrategyConfig
	flushDecision *FlushDecisionEngine
	tokenCounter  TokenCounter
	options       *StrategyOptions
	maxTokens     int64
	mu            sync.RWMutex
}

// MessageWithTokens wraps a message with its token count for cost calculation
type MessageWithTokens struct {
	Message    llm.Message
	TokenCount int
	Index      int
}

// NewTokenAwareLRUStrategy creates a new token-aware LRU strategy using ristretto
func NewTokenAwareLRUStrategy(
	config *core.FlushingStrategyConfig,
	options *StrategyOptions,
) (*TokenAwareLRUStrategy, error) {
	// Use default options if none provided
	if options == nil {
		options = GetDefaultStrategyOptions()
	}

	maxTokens := options.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4000 // Default token capacity
	}

	// Create ristretto cache with token-based cost awareness
	// The cost function will use the actual token count of each message
	cache, err := ristretto.NewCache(&ristretto.Config[int, MessageWithTokens]{
		NumCounters: int64(maxTokens), // One counter per token for fine-grained tracking
		MaxCost:     int64(maxTokens), // Total token budget
		BufferItems: 64,               // Standard buffer size
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create ristretto cache for token-aware LRU strategy: %w", err)
	}

	thresholdPercent := 0.8 // Default to 80%
	if config != nil && config.SummarizeThreshold > 0 {
		thresholdPercent = config.SummarizeThreshold
	}

	return &TokenAwareLRUStrategy{
		cache:         cache,
		config:        config,
		flushDecision: NewFlushDecisionEngine(thresholdPercent),
		tokenCounter:  NewGPTTokenCounter(),
		options:       options,
		maxTokens:     int64(maxTokens),
	}, nil
}

// ShouldFlush determines if a flush should be triggered based on token usage
func (s *TokenAwareLRUStrategy) ShouldFlush(tokenCount, messageCount int, config *core.Resource) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.flushDecision.ShouldFlush(tokenCount, messageCount, config)
}

// PerformFlush executes the token-aware LRU flush operation
func (s *TokenAwareLRUStrategy) PerformFlush(
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

	// Convert messages to token-aware format and populate cache
	messagesWithTokens := s.convertMessagesToTokenAware(messages)

	// Populate cache to trigger natural LRU eviction
	s.populateCache(messagesWithTokens)

	// Calculate current token usage
	currentTokens := s.calculateCurrentTokens(messages)

	// Determine target token count after flush
	targetTokens := s.calculateTargetTokens(config)

	if currentTokens <= targetTokens {
		// No flush needed
		return &core.FlushMemoryActivityOutput{
			Success:          true,
			SummaryGenerated: false,
			MessageCount:     len(messages),
			TokenCount:       currentTokens,
		}, nil
	}

	// Use cache to determine which messages to evict
	evictedMessages, remainingMessages := s.evictByTokenCost(messagesWithTokens, targetTokens)

	// Calculate final metrics
	tokensFlushed := s.calculateTokensInMessages(evictedMessages)

	return &core.FlushMemoryActivityOutput{
		Success:          true,
		SummaryGenerated: false,
		MessageCount:     len(remainingMessages),
		TokenCount:       tokensFlushed,
	}, nil
}

// GetType returns the strategy type
func (s *TokenAwareLRUStrategy) GetType() core.FlushingStrategyType {
	return core.TokenAwareLRUFlushing
}

// convertMessagesToTokenAware converts messages to token-aware format
func (s *TokenAwareLRUStrategy) convertMessagesToTokenAware(messages []llm.Message) []MessageWithTokens {
	result := make([]MessageWithTokens, len(messages))
	for i, msg := range messages {
		tokenCount := s.tokenCounter.CountTokens(msg)
		result[i] = MessageWithTokens{
			Message:    msg,
			TokenCount: tokenCount,
			Index:      i,
		}
	}
	return result
}

// populateCache adds messages to the LRU cache for access tracking
func (s *TokenAwareLRUStrategy) populateCache(messages []MessageWithTokens) {
	// Clear existing cache
	s.cache.Clear()

	// Add messages in order, simulating access pattern
	// Use token count as the cost for each message
	for i, msgWithTokens := range messages {
		s.cache.Set(i, msgWithTokens, int64(msgWithTokens.TokenCount))
	}
}

// evictByTokenCost evicts messages based on token cost and LRU order
func (s *TokenAwareLRUStrategy) evictByTokenCost(
	messages []MessageWithTokens,
	targetTokens int,
) (evicted []MessageWithTokens, remaining []MessageWithTokens) {
	currentTokens := s.calculateTokensInMessages(messages)

	if currentTokens <= targetTokens {
		return []MessageWithTokens{}, messages
	}

	// Create a map to track which messages are still in cache (not evicted)
	inCache := make(map[int]bool)
	for i := range messages {
		_, exists := s.cache.Get(i)
		inCache[i] = exists
	}

	// Separate evicted and remaining messages based on cache state
	evicted = make([]MessageWithTokens, 0)
	remaining = make([]MessageWithTokens, 0)

	for _, msgWithTokens := range messages {
		if inCache[msgWithTokens.Index] {
			remaining = append(remaining, msgWithTokens)
		} else {
			evicted = append(evicted, msgWithTokens)
		}
	}

	// If natural eviction isn't enough, force evict LRU messages
	remainingTokens := s.calculateTokensInMessages(remaining)
	if remainingTokens > targetTokens {
		additionalEvicted, finalRemaining := s.forceEvictByTokens(remaining, targetTokens)
		evicted = append(evicted, additionalEvicted...)
		remaining = finalRemaining
	}

	return evicted, remaining
}

// forceEvictByTokens forcibly evicts messages to reach target token count
func (s *TokenAwareLRUStrategy) forceEvictByTokens(
	messages []MessageWithTokens,
	targetTokens int,
) (evicted []MessageWithTokens, remaining []MessageWithTokens) {
	// Sort messages by access time (LRU order)
	sortedMessages := make([]MessageWithTokens, len(messages))
	copy(sortedMessages, messages)

	// Simple LRU sorting based on index (older messages first)
	sort.Slice(sortedMessages, func(i, j int) bool {
		return sortedMessages[i].Index < sortedMessages[j].Index
	})

	currentTokens := s.calculateTokensInMessages(sortedMessages)
	evicted = make([]MessageWithTokens, 0)
	remaining = make([]MessageWithTokens, 0, len(sortedMessages))

	// Evict oldest messages until we reach target token count
	for i, msg := range sortedMessages {
		if currentTokens <= targetTokens {
			// Add remaining messages
			remaining = append(remaining, sortedMessages[i:]...)
			break
		}

		evicted = append(evicted, msg)
		currentTokens -= msg.TokenCount
	}

	return evicted, remaining
}

// calculateCurrentTokens calculates total tokens in messages
func (s *TokenAwareLRUStrategy) calculateCurrentTokens(messages []llm.Message) int {
	total := 0
	for _, msg := range messages {
		total += s.tokenCounter.CountTokens(msg)
	}
	return total
}

// calculateTokensInMessages calculates total tokens in MessageWithTokens slice
func (s *TokenAwareLRUStrategy) calculateTokensInMessages(messages []MessageWithTokens) int {
	total := 0
	for _, msg := range messages {
		total += msg.TokenCount
	}
	return total
}

// calculateTargetTokens determines target token count after flush
func (s *TokenAwareLRUStrategy) calculateTargetTokens(config *core.Resource) int {
	maxTokens := int(s.maxTokens)
	if config.MaxTokens > 0 {
		maxTokens = config.MaxTokens
	}

	// Target configured percentage of max capacity to allow room for growth
	return int(float64(maxTokens) * s.options.TokenLRUTargetCapacityPercent)
}

// GetMinMaxToFlush returns the min/max number of messages to flush for this strategy
func (s *TokenAwareLRUStrategy) GetMinMaxToFlush(
	_ context.Context,
	totalMsgs int,
	currentTokens int,
	maxTokens int,
) (minFlush, maxFlush int) {
	// Token-aware strategy: base on token ratios rather than message counts
	if maxTokens > 0 && currentTokens > maxTokens {
		// Need to flush enough to get back under limit
		excessTokens := currentTokens - maxTokens
		avgTokensPerMsg := currentTokens / totalMsgs
		if avgTokensPerMsg > 0 {
			minFlush = excessTokens / avgTokensPerMsg
			maxFlush = totalMsgs / 2 // Never flush more than half
		}
	}

	if minFlush <= 0 {
		minFlush = 1
	}
	if maxFlush < minFlush {
		maxFlush = minFlush
	}

	return minFlush, maxFlush
}
