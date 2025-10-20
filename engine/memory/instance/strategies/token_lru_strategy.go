package strategies

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/memory/core"
	lru "github.com/hashicorp/golang-lru/v2"
)

// TokenAwareLRUStrategy implements a true LRU (Least Recently Used) flushing strategy
// It maintains messages in an LRU cache and evicts the least recently used messages
// when capacity is exceeded, considering both message count and token limits
type TokenAwareLRUStrategy struct {
	cache         *lru.Cache[int, MessageWithTokens]
	config        *core.FlushingStrategyConfig
	flushDecision *FlushDecisionEngine
	tokenCounter  core.TokenCounter
	options       *StrategyOptions
	maxTokens     int64
	mu            sync.RWMutex
	// Track total tokens in cache to avoid recalculation
	totalTokens int
}

// MessageWithTokens wraps a message with its token count for LRU tracking
type MessageWithTokens struct {
	Message    llm.Message
	TokenCount int
	Index      int // Original index in the message array
}

// NewTokenAwareLRUStrategy creates a new token-aware LRU strategy using hashicorp/golang-lru
func NewTokenAwareLRUStrategy(
	config *core.FlushingStrategyConfig,
	options *StrategyOptions,
) (*TokenAwareLRUStrategy, error) {
	resolvedOptions := resolveLRUOptions(options)
	strategy := &TokenAwareLRUStrategy{
		config:    config,
		options:   resolvedOptions,
		maxTokens: resolveMaxTokens(resolvedOptions),
	}

	cacheSize := sanitizeTokenLRUCacheSize(resolvedOptions.CacheSize, resolvedOptions.MaxCacheSize)
	cache, err := lru.New[int, MessageWithTokens](cacheSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create LRU cache for token-aware LRU strategy: %w", err)
	}

	strategy.cache = cache
	strategy.flushDecision = NewFlushDecisionEngine(resolveLRUThreshold(config))
	strategy.tokenCounter = buildLRUTokenCounter(resolvedOptions)

	return strategy, nil
}

func resolveMaxTokens(options *StrategyOptions) int64 {
	if options.MaxTokens > 0 {
		return int64(options.MaxTokens)
	}
	return 4000
}

func sanitizeTokenLRUCacheSize(size, maxSize int) int {
	if size <= 0 {
		size = 1000
	}
	if maxSize <= 0 {
		maxSize = 10000
	}
	if size > maxSize {
		return maxSize
	}
	return size
}

// ShouldFlush determines if a flush should be triggered based on token usage
func (s *TokenAwareLRUStrategy) ShouldFlush(tokenCount, messageCount int, config *core.Resource) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.flushDecision.ShouldFlush(tokenCount, messageCount, config)
}

// PerformFlush executes the true LRU flush operation
func (s *TokenAwareLRUStrategy) PerformFlush(
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

	s.resetCacheState()
	s.rebuildCache(ctx, messages)

	targetCapacity, tokenBased := s.capacityConstraints(config)
	remainingMessages := s.evictToMeetCapacity(targetCapacity, tokenBased)

	return &core.FlushMemoryActivityOutput{
		Success:          true,
		SummaryGenerated: false,
		MessageCount:     len(remainingMessages),
		TokenCount:       s.totalTokens,
	}, nil
}

func (s *TokenAwareLRUStrategy) resetCacheState() {
	s.cache.Purge()
	s.totalTokens = 0
}

func (s *TokenAwareLRUStrategy) rebuildCache(ctx context.Context, messages []llm.Message) {
	for i, msg := range messages {
		tokenCount := s.countTokensWithFallback(ctx, msg.Content)

		if existing, found := s.cache.Get(i); found {
			s.totalTokens -= existing.TokenCount
		}

		s.cache.Add(i, MessageWithTokens{
			Message:    msg,
			TokenCount: tokenCount,
			Index:      i,
		})
		s.totalTokens += tokenCount
	}
}

func (s *TokenAwareLRUStrategy) capacityConstraints(config *core.Resource) (int, bool) {
	if config.Type == core.MessageCountBasedMemory && config.MaxMessages > 0 {
		return int(float64(config.MaxMessages) * 0.6), false
	}
	return s.calculateTargetTokens(config), true
}

func (s *TokenAwareLRUStrategy) countTokensWithFallback(ctx context.Context, content string) int {
	count, err := s.tokenCounter.CountTokens(ctx, content)
	if err != nil {
		return 0
	}
	return count
}

// GetType returns the strategy type
func (s *TokenAwareLRUStrategy) GetType() core.FlushingStrategyType {
	return core.TokenAwareLRUFlushing
}

// evictToMeetCapacity evicts LRU messages until the target capacity is met
func (s *TokenAwareLRUStrategy) evictToMeetCapacity(targetCapacity int, isTokenBased bool) []MessageWithTokens {
	// The LRU cache doesn't provide direct access to the LRU order,
	// so we need to collect all messages and sort by their original index
	allMessages := make([]MessageWithTokens, 0, s.cache.Len())
	keys := s.cache.Keys()

	for _, key := range keys {
		if msg, found := s.cache.Get(key); found {
			allMessages = append(allMessages, msg)
		}
	}

	// Sort by original index to maintain message order
	sort.Slice(allMessages, func(i, j int) bool {
		return allMessages[i].Index < allMessages[j].Index
	})

	// Now evict from the beginning (oldest messages) until we meet capacity
	if isTokenBased {
		// For token-based eviction, remove messages from the start until under limit
		for len(allMessages) > 0 && s.totalTokens > targetCapacity {
			// Remove the first (oldest) message
			oldMsg := allMessages[0]
			allMessages = allMessages[1:]
			s.cache.Remove(oldMsg.Index)
			s.totalTokens -= oldMsg.TokenCount
		}
	} else if len(allMessages) > targetCapacity {
		// For message count-based eviction
		// Remove oldest messages
		toRemove := len(allMessages) - targetCapacity
		for i := 0; i < toRemove && i < len(allMessages); i++ {
			s.cache.Remove(allMessages[i].Index)
			s.totalTokens -= allMessages[i].TokenCount
		}
		allMessages = allMessages[toRemove:]
	}

	return allMessages
}

// calculateTargetTokens determines target token count after flush
func (s *TokenAwareLRUStrategy) calculateTargetTokens(config *core.Resource) int {
	maxTokens := int(s.maxTokens)
	if config.MaxTokens > 0 {
		maxTokens = config.MaxTokens
	}

	// Target configured percentage of max capacity to allow room for growth
	targetPercent := s.options.TokenLRUTargetCapacityPercent
	if targetPercent == 0 {
		targetPercent = 0.5 // Default to 50%
	}
	return int(float64(maxTokens) * targetPercent)
}

// GetMinMaxToFlush returns the min/max number of messages to flush for this strategy
func (s *TokenAwareLRUStrategy) GetMinMaxToFlush(
	_ context.Context,
	totalMsgs int,
	currentTokens int,
	maxTokens int,
) (minFlush, maxFlush int) {
	// Always set minimum flush to 1
	minFlush = 1

	// Calculate max flush based on total messages
	// For token-aware strategy, we can be more aggressive
	switch {
	case totalMsgs <= 3:
		maxFlush = 1 // Very few messages, only flush 1
	case currentTokens > maxTokens:
		// When over token limit, flush up to half the messages
		maxFlush = max(totalMsgs/2, minFlush)
	case float64(currentTokens) > float64(maxTokens)*0.9:
		// High token pressure (>90% capacity): be more aggressive
		maxFlush = max(totalMsgs/2, minFlush)
	default:
		// Normal case: flush up to 1/3 of messages
		maxFlush = max(totalMsgs/3, minFlush)
	}

	return minFlush, maxFlush
}
