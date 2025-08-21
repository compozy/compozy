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
	// Use default options if none provided
	if options == nil {
		options = GetDefaultStrategyOptions()
	}

	maxTokens := options.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4000 // Default token capacity
	}

	// Create LRU cache for tracking message access patterns
	// We'll track tokens separately from the LRU cache
	cacheSize := options.CacheSize
	if cacheSize <= 0 {
		cacheSize = 1000 // Default cache size
	}
	// Enforce maximum cache size to prevent excessive memory usage
	maxCacheSize := options.MaxCacheSize
	if maxCacheSize <= 0 {
		maxCacheSize = 10000 // Default
	}
	if cacheSize > maxCacheSize {
		cacheSize = maxCacheSize
	}

	// Initialize strategy first to access methods
	strategy := &TokenAwareLRUStrategy{
		config:      config,
		options:     options,
		maxTokens:   int64(maxTokens),
		totalTokens: 0,
	}

	// Create LRU cache without eviction callback to avoid deadlock
	cache, err := lru.New[int, MessageWithTokens](cacheSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create LRU cache for token-aware LRU strategy: %w", err)
	}
	strategy.cache = cache

	thresholdPercent := 0.8 // Default to 80%
	if config != nil && config.SummarizeThreshold > 0 {
		thresholdPercent = config.SummarizeThreshold
	}

	// Create token counter with fallback estimation
	baseCounter := NewGPTTokenCounterAdapter()
	estimationStrategy := options.TokenEstimationStrategy
	if estimationStrategy == "" {
		estimationStrategy = core.EnglishEstimation
	}
	tokenEstimator := core.NewTokenEstimator(estimationStrategy)
	tokenCounter := core.NewTokenCounterWithFallback(baseCounter, tokenEstimator)

	strategy.flushDecision = NewFlushDecisionEngine(thresholdPercent)
	strategy.tokenCounter = tokenCounter

	return strategy, nil
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

	// Clear cache and rebuild with all messages
	s.cache.Purge()
	s.totalTokens = 0

	// Convert messages and add them to LRU cache
	for i, msg := range messages {
		tokenCount, err := s.tokenCounter.CountTokens(ctx, msg.Content)
		if err != nil {
			tokenCount = 0 // Fallback handled by counter
		}

		msgWithTokens := MessageWithTokens{
			Message:    msg,
			TokenCount: tokenCount,
			Index:      i,
		}

		// Check if we're replacing an existing message
		if existing, found := s.cache.Get(i); found {
			// Subtract the old token count before adding the new one
			s.totalTokens -= existing.TokenCount
		}

		// Add to cache - this will automatically evict LRU messages if cache is full
		s.cache.Add(i, msgWithTokens)
		s.totalTokens += tokenCount
	}

	// Now determine what needs to be evicted based on constraints
	var targetCapacity int
	var isTokenBased bool

	if config.Type == core.MessageCountBasedMemory && config.MaxMessages > 0 {
		// For message-based memory, target 60% of max messages
		targetCapacity = int(float64(config.MaxMessages) * 0.6)
		isTokenBased = false
	} else {
		// For token-based memory, calculate target tokens
		targetCapacity = s.calculateTargetTokens(config)
		isTokenBased = true
	}

	// Evict messages until we meet the target capacity
	remainingMessages := s.evictToMeetCapacity(targetCapacity, isTokenBased)

	// Calculate final metrics
	remainingTokens := s.totalTokens

	return &core.FlushMemoryActivityOutput{
		Success:          true,
		SummaryGenerated: false,
		MessageCount:     len(remainingMessages),
		TokenCount:       remainingTokens,
	}, nil
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
