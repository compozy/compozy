package strategies

import (
	"context"
	"sort"
	"strings"
	"sync"

	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/memory/core"
)

// PriorityBasedStrategy implements a priority-based flushing strategy
type PriorityBasedStrategy struct {
	config        *core.FlushingStrategyConfig
	flushDecision *FlushDecisionEngine
	tokenCounter  TokenCounter
	options       *StrategyOptions
	mu            sync.RWMutex
}

// MessagePriority represents the priority level of a message
type MessagePriority int

const (
	// PriorityCritical - system prompts, user profiles (never evicted)
	PriorityCritical MessagePriority = 0
	// PriorityHigh - important context, recent interactions
	PriorityHigh MessagePriority = 1
	// PriorityMedium - normal user messages
	PriorityMedium MessagePriority = 2
	// PriorityLow - old historical messages, less important content
	PriorityLow MessagePriority = 3
)

// PriorityMessage wraps a message with its priority
type PriorityMessage struct {
	Message    llm.Message
	Priority   MessagePriority
	Index      int
	TokenCount int
}

// NewPriorityBasedStrategy creates a new priority-based flushing strategy
func NewPriorityBasedStrategy(config *core.FlushingStrategyConfig, options *StrategyOptions) *PriorityBasedStrategy {
	thresholdPercent := 0.8 // Default to 80%
	if config != nil && config.SummarizeThreshold > 0 {
		thresholdPercent = config.SummarizeThreshold
	}

	// Use default options if none provided
	if options == nil {
		options = GetDefaultStrategyOptions()
	}

	return &PriorityBasedStrategy{
		config:        config,
		flushDecision: NewFlushDecisionEngine(thresholdPercent),
		tokenCounter:  NewGPTTokenCounter(),
		options:       options,
	}
}

// ShouldFlush determines if a flush should be triggered based on current state
func (s *PriorityBasedStrategy) ShouldFlush(tokenCount, messageCount int, config *core.Resource) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.flushDecision.ShouldFlush(tokenCount, messageCount, config)
}

// PerformFlush executes the priority-based flush operation
func (s *PriorityBasedStrategy) PerformFlush(
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

	// Convert messages to priority messages
	priorityMessages := s.assignPriorities(messages)

	// Calculate how many messages/tokens to remove
	targetCount := s.calculateTargetCount(len(messages), config)
	targetTokens := s.calculateTargetTokens(priorityMessages, config)

	// Select messages for eviction based on priority
	evictedMessages := s.selectMessagesByPriority(priorityMessages, targetCount, targetTokens)

	// Calculate remaining metrics
	remainingCount := len(messages) - len(evictedMessages)
	tokensFlushed := s.calculateEvictedTokens(evictedMessages)

	return &core.FlushMemoryActivityOutput{
		Success:          true,
		SummaryGenerated: false,
		MessageCount:     remainingCount,
		TokenCount:       tokensFlushed,
	}, nil
}

// GetType returns the strategy type
func (s *PriorityBasedStrategy) GetType() core.FlushingStrategyType {
	return core.PriorityBasedFlushing
}

// assignPriorities assigns priority levels to messages based on content analysis
func (s *PriorityBasedStrategy) assignPriorities(messages []llm.Message) []PriorityMessage {
	result := make([]PriorityMessage, len(messages))

	for i, msg := range messages {
		priority := s.determinePriority(msg, i, len(messages))
		tokenCount := s.tokenCounter.CountTokens(msg)

		result[i] = PriorityMessage{
			Message:    msg,
			Priority:   priority,
			Index:      i,
			TokenCount: tokenCount,
		}
	}

	return result
}

// determinePriority analyzes a message to determine its priority level
func (s *PriorityBasedStrategy) determinePriority(msg llm.Message, index, totalMessages int) MessagePriority {
	content := strings.ToLower(msg.Content)

	if s.isCriticalMessage(string(msg.Role), content) {
		return PriorityCritical
	}

	if s.isRecentMessage(index, totalMessages) || s.hasImportantKeywords(content) {
		return PriorityHigh
	}

	if s.isMediumPriorityMessage(string(msg.Role), msg.Content) {
		return PriorityMedium
	}

	if s.isLowPriorityMessage(msg.Content, content) {
		return PriorityLow
	}

	return PriorityMedium
}

// isCriticalMessage checks if message is critical priority
func (s *PriorityBasedStrategy) isCriticalMessage(role, content string) bool {
	return role == "system" ||
		strings.Contains(content, "system") ||
		strings.Contains(content, "instruction") ||
		strings.Contains(content, "profile") ||
		strings.Contains(content, "rule") ||
		strings.Contains(content, "guideline")
}

// isRecentMessage checks if message is in recent portion of conversation
func (s *PriorityBasedStrategy) isRecentMessage(index, totalMessages int) bool {
	recentThreshold := int(float64(totalMessages) * s.options.PriorityRecentThreshold)
	return index >= recentThreshold
}

// hasImportantKeywords checks for high priority keywords
func (s *PriorityBasedStrategy) hasImportantKeywords(content string) bool {
	return strings.Contains(content, "important") ||
		strings.Contains(content, "critical") ||
		strings.Contains(content, "error") ||
		strings.Contains(content, "problem") ||
		strings.Contains(content, "issue") ||
		strings.Contains(content, "urgent")
}

// isMediumPriorityMessage checks for medium priority conditions
func (s *PriorityBasedStrategy) isMediumPriorityMessage(role, content string) bool {
	return role == "assistant" || len(content) > 100
}

// isLowPriorityMessage checks for low priority conditions
func (s *PriorityBasedStrategy) isLowPriorityMessage(content, contentLower string) bool {
	return len(content) < 50 ||
		strings.Contains(contentLower, "hello") ||
		strings.Contains(contentLower, "hi") ||
		strings.Contains(contentLower, "thanks") ||
		strings.Contains(contentLower, "ok") ||
		strings.Contains(contentLower, "yes") ||
		strings.Contains(contentLower, "no")
}

// selectMessagesByPriority selects messages for eviction based on priority levels
func (s *PriorityBasedStrategy) selectMessagesByPriority(
	messages []PriorityMessage,
	targetMessageCount int,
	targetTokenCount int,
) []PriorityMessage {
	if targetMessageCount >= len(messages) {
		return []PriorityMessage{}
	}

	// Group messages by priority
	priorityGroups := make(map[MessagePriority][]PriorityMessage)
	for _, msg := range messages {
		priorityGroups[msg.Priority] = append(priorityGroups[msg.Priority], msg)
	}

	evicted := make([]PriorityMessage, 0)
	currentTokens := s.calculateTotalTokens(messages)
	remainingMessages := len(messages)

	// Evict in priority order: Low -> Medium -> High (never evict Critical)
	priorities := []MessagePriority{PriorityLow, PriorityMedium, PriorityHigh}

	for _, priority := range priorities {
		if remainingMessages <= targetMessageCount && currentTokens <= targetTokenCount {
			break
		}

		group, exists := priorityGroups[priority]
		if !exists || len(group) == 0 {
			continue
		}

		// Sort by index (oldest first within priority level)
		s.sortMessagesByAge(group)

		// Evict messages from this priority level
		for _, msg := range group {
			if remainingMessages <= targetMessageCount && currentTokens <= targetTokenCount {
				break
			}

			evicted = append(evicted, msg)
			currentTokens -= msg.TokenCount
			remainingMessages--
		}
	}

	return evicted
}

// sortMessagesByAge sorts messages by index (oldest first)
func (s *PriorityBasedStrategy) sortMessagesByAge(messages []PriorityMessage) {
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].Index < messages[j].Index
	})
}

// calculateTargetCount determines target message count after flush
func (s *PriorityBasedStrategy) calculateTargetCount(currentCount int, config *core.Resource) int {
	// Target: reduce to configured capacity to allow room for growth while preserving important messages
	targetPercent := s.options.PriorityTargetCapacityPercent

	if config.MaxMessages > 0 {
		targetCount := int(float64(config.MaxMessages) * targetPercent)
		if currentCount > targetCount {
			return targetCount
		}
	}

	// Default: keep configured conservative percentage of messages
	return int(float64(currentCount) * s.options.PriorityConservativePercent)
}

// calculateTargetTokens determines target token count after flush
func (s *PriorityBasedStrategy) calculateTargetTokens(_ []PriorityMessage, config *core.Resource) int {
	if config.MaxTokens <= 0 {
		return int(^uint(0) >> 1) // Max int if no token limit
	}

	// Target: reduce to configured percentage of token capacity
	return int(float64(config.MaxTokens) * s.options.PriorityTargetCapacityPercent)
}

// calculateTotalTokens calculates total tokens in priority messages
func (s *PriorityBasedStrategy) calculateTotalTokens(messages []PriorityMessage) int {
	total := 0
	for _, msg := range messages {
		total += msg.TokenCount
	}
	return total
}

// calculateEvictedTokens calculates total tokens in evicted messages
func (s *PriorityBasedStrategy) calculateEvictedTokens(evicted []PriorityMessage) int {
	total := 0
	for _, msg := range evicted {
		total += msg.TokenCount
	}
	return total
}

// GetMinMaxToFlush returns the min/max number of messages to flush for this strategy
func (s *PriorityBasedStrategy) GetMinMaxToFlush(
	_ context.Context,
	totalMsgs int,
	_ int, // currentTokens - unused for priority-based
	_ int, // maxTokens - unused for priority-based
) (minFlush, maxFlush int) {
	// Priority-based strategy: preserve critical messages, be conservative
	minFlush = 1
	maxFlush = int(
		float64(totalMsgs) * s.options.PriorityMaxFlushRatio,
	) // Never flush more than configured ratio to preserve context
	if maxFlush < minFlush {
		maxFlush = minFlush
	}
	return minFlush, maxFlush
}
