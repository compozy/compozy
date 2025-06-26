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

// TokenAwareLRUStrategy implements a token-cost-aware LRU flushing strategy
type TokenAwareLRUStrategy struct {
	cache         *lru.Cache[int, MessageWithTokens]
	config        *core.FlushingStrategyConfig
	flushDecision *FlushDecisionEngine
	tokenCounter  core.TokenCounter
	options       *StrategyOptions
	maxTokens     int64
	mu            sync.RWMutex
}

// MessageWithTokens wraps a message with its token count and access time for cost calculation
type MessageWithTokens struct {
	Message    llm.Message
	TokenCount int
	Index      int
	LastAccess time.Time
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

	cache, err := lru.New[int, MessageWithTokens](cacheSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create LRU cache for token-aware LRU strategy: %w", err)
	}

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

	return &TokenAwareLRUStrategy{
		cache:         cache,
		config:        config,
		flushDecision: NewFlushDecisionEngine(thresholdPercent),
		tokenCounter:  tokenCounter,
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

	// Determine target based on memory type
	var remainingMessages []MessageWithTokens

	if config.Type == core.MessageCountBasedMemory && config.MaxMessages > 0 {
		// For message-based memory, target 60% of max messages
		targetMessages := int(float64(config.MaxMessages) * 0.6)
		_, remainingMessages = s.evictByMessageCount(messagesWithTokens, targetMessages)
	} else {
		// For token-based memory, use token-based eviction
		targetTokens := s.calculateTargetTokens(config)
		_, remainingMessages = s.evictByTokenCost(messagesWithTokens, targetTokens)
	}

	// Calculate final metrics
	remainingTokens := s.calculateTokensInMessages(remainingMessages)

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

// convertMessagesToTokenAware converts messages to token-aware format
func (s *TokenAwareLRUStrategy) convertMessagesToTokenAware(messages []llm.Message) []MessageWithTokens {
	result := make([]MessageWithTokens, len(messages))
	ctx := context.Background()
	now := time.Now()

	for i, msg := range messages {
		tokenCount, err := s.tokenCounter.CountTokens(ctx, msg.Content)
		if err != nil {
			// TokenCounter should handle fallback internally
			tokenCount = 0
		}

		// Initialize with current time, will be updated from cache if exists
		result[i] = MessageWithTokens{
			Message:    msg,
			TokenCount: tokenCount,
			Index:      i,
			LastAccess: now.Add(
				time.Duration(-len(messages)+i) * time.Nanosecond,
			), // Preserve order with higher precision
		}
	}
	return result
}

// populateCache adds messages to the LRU cache for access tracking
func (s *TokenAwareLRUStrategy) populateCache(messages []MessageWithTokens) {
	// Update cache with messages, preserving existing access patterns
	// Use token count as the cost for each message
	for i, msgWithTokens := range messages {
		// Check if message already exists in cache to preserve access time
		if existing, found := s.cache.Get(i); found {
			msgWithTokens.LastAccess = existing.LastAccess
		}
		s.cache.Add(i, msgWithTokens)
	}
}

// evictByTokenCost evicts messages based on combined token cost and recency score
func (s *TokenAwareLRUStrategy) evictByTokenCost(
	messages []MessageWithTokens,
	targetTokens int,
) (evicted []MessageWithTokens, remaining []MessageWithTokens) {
	currentTokens := s.calculateTokensInMessages(messages)

	if currentTokens <= targetTokens {
		return []MessageWithTokens{}, messages
	}

	// Calculate scores for each message
	scoredMessages := s.scoreMessages(messages)

	// Sort by score descending (evict high-score messages first)
	sort.Slice(scoredMessages, func(i, j int) bool {
		return scoredMessages[i].score > scoredMessages[j].score
	})

	evicted = make([]MessageWithTokens, 0)
	remaining = make([]MessageWithTokens, 0)
	remainingTokens := currentTokens

	// Evict messages starting with highest score until we reach target
	for _, scored := range scoredMessages {
		tokensAfterEviction := remainingTokens - scored.msg.TokenCount

		if remainingTokens > targetTokens && tokensAfterEviction >= 0 {
			evicted = append(evicted, scored.msg)
			remainingTokens = tokensAfterEviction
		} else {
			remaining = append(remaining, scored.msg)
		}
	}

	// Restore original order for remaining messages
	sort.Slice(remaining, func(i, j int) bool {
		return remaining[i].Index < remaining[j].Index
	})

	return evicted, remaining
}

// evictByMessageCount evicts messages to reach target message count using token-aware LRU scoring
func (s *TokenAwareLRUStrategy) evictByMessageCount(
	messages []MessageWithTokens,
	targetCount int,
) (evicted []MessageWithTokens, remaining []MessageWithTokens) {
	if len(messages) <= targetCount {
		return []MessageWithTokens{}, messages
	}

	// Use the same scoring approach as evictByTokenCost for consistency
	scoredMessages := s.scoreMessages(messages)

	// Sort by score descending (evict high-score messages first)
	sort.Slice(scoredMessages, func(i, j int) bool {
		return scoredMessages[i].score > scoredMessages[j].score
	})

	// Evict messages until we reach target count
	numToEvict := len(messages) - targetCount
	evicted = make([]MessageWithTokens, numToEvict)
	remaining = make([]MessageWithTokens, targetCount)

	for i := 0; i < numToEvict; i++ {
		evicted[i] = scoredMessages[i].msg
	}
	for i := 0; i < targetCount; i++ {
		remaining[i] = scoredMessages[numToEvict+i].msg
	}

	// Restore original order for remaining messages
	sort.Slice(remaining, func(i, j int) bool {
		return remaining[i].Index < remaining[j].Index
	})

	return evicted, remaining
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
		maxFlush = totalMsgs / 2
		if maxFlush < minFlush {
			maxFlush = minFlush
		}
	case float64(currentTokens) > float64(maxTokens)*0.9:
		// High token pressure (>90% capacity): be more aggressive
		maxFlush = totalMsgs / 2
		if maxFlush < minFlush {
			maxFlush = minFlush
		}
	default:
		// Normal case: flush up to 1/3 of messages
		maxFlush = totalMsgs / 3
		if maxFlush < minFlush {
			maxFlush = minFlush
		}
	}

	return minFlush, maxFlush
}

// scoredMessage holds a message with its eviction score
type scoredMessage struct {
	msg   MessageWithTokens
	score float64
}

// scoreMessages calculates eviction scores for all messages
func (s *TokenAwareLRUStrategy) scoreMessages(messages []MessageWithTokens) []scoredMessage {
	scoredMessages := make([]scoredMessage, len(messages))

	// Find min/max values for normalization
	minTokens, maxTokens, oldestTime, newestTime := s.findMinMaxValues(messages)

	// Calculate combined scores
	for i, msg := range messages {
		score := s.calculateEvictionScore(msg, minTokens, maxTokens, oldestTime, newestTime)
		scoredMessages[i] = scoredMessage{msg: msg, score: score}
	}

	return scoredMessages
}

// findMinMaxValues finds min/max values for token count and access time
func (s *TokenAwareLRUStrategy) findMinMaxValues(messages []MessageWithTokens) (int, int, time.Time, time.Time) {
	if len(messages) == 0 {
		return 0, 0, time.Time{}, time.Time{}
	}

	minTokens, maxTokens := messages[0].TokenCount, messages[0].TokenCount
	oldestTime, newestTime := messages[0].LastAccess, messages[0].LastAccess

	for _, msg := range messages[1:] {
		if msg.TokenCount < minTokens {
			minTokens = msg.TokenCount
		}
		if msg.TokenCount > maxTokens {
			maxTokens = msg.TokenCount
		}
		if msg.LastAccess.Before(oldestTime) {
			oldestTime = msg.LastAccess
		}
		if msg.LastAccess.After(newestTime) {
			newestTime = msg.LastAccess
		}
	}

	return minTokens, maxTokens, oldestTime, newestTime
}

// calculateEvictionScore calculates a combined score for eviction priority
func (s *TokenAwareLRUStrategy) calculateEvictionScore(
	msg MessageWithTokens,
	minTokens, maxTokens int,
	oldestTime, newestTime time.Time,
) float64 {
	// Normalize token count (0-1, higher tokens = higher score)
	tokenScore := 0.0
	if maxTokens > minTokens {
		tokenScore = float64(msg.TokenCount-minTokens) / float64(maxTokens-minTokens)
	}

	// Normalize age (0-1, older = higher score)
	ageScore := 0.0
	ageRange := newestTime.Sub(oldestTime)
	if ageRange > 0 {
		ageScore = 1.0 - (msg.LastAccess.Sub(oldestTime).Seconds() / ageRange.Seconds())
	}

	// Combined score with weights
	tokenWeight := 0.6
	ageWeight := 0.4
	return tokenWeight*tokenScore + ageWeight*ageScore
}
