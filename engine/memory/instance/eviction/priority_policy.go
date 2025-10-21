package eviction

import (
	"context"
	"sort"
	"strings"

	"github.com/compozy/compozy/engine/llm"
	memcore "github.com/compozy/compozy/engine/memory/core"
)

// PriorityEvictionPolicy implements a priority-based eviction strategy
type PriorityEvictionPolicy struct {
	name              string
	importantKeywords []string
	tokenEstimator    memcore.TokenEstimator
}

// MessagePriority represents the priority level for eviction
type MessagePriority int

const (
	// PriorityCritical - system messages, never evict
	PriorityCritical MessagePriority = 0
	// PriorityHigh - assistant responses, evict last
	PriorityHigh MessagePriority = 1
	// PriorityMedium - user messages, standard priority
	PriorityMedium MessagePriority = 2
	// PriorityLow - tool messages, evict first
	PriorityLow MessagePriority = 3
	// PriorityLowest - unknown roles, evict first
	PriorityLowest MessagePriority = 4
)

// messageWithPriority holds a message with its priority information
type messageWithPriority struct {
	message       llm.Message
	priority      MessagePriority
	tokenEstimate int
	index         int
}

// getDefaultPriorityKeywords returns the default list of priority keywords
func getDefaultPriorityKeywords() []string {
	return []string{
		"error", "critical", "important", "warning",
		"failed", "exception", "issue", "problem",
		"urgent", "key", "essential", "required",
	}
}

// NewPriorityEvictionPolicy creates a new priority-based eviction policy
func NewPriorityEvictionPolicy(_ context.Context) *PriorityEvictionPolicy {
	return &PriorityEvictionPolicy{
		name:              "priority",
		importantKeywords: getDefaultPriorityKeywords(),
		tokenEstimator:    memcore.NewTokenEstimator(memcore.EnglishEstimation),
	}
}

// NewPriorityEvictionPolicyWithKeywords creates a new priority-based eviction policy with custom keywords
func NewPriorityEvictionPolicyWithKeywords(_ context.Context, keywords []string) *PriorityEvictionPolicy {
	if len(keywords) == 0 {
		keywords = getDefaultPriorityKeywords()
	}
	return &PriorityEvictionPolicy{
		name:              "priority",
		importantKeywords: keywords,
		tokenEstimator:    memcore.NewTokenEstimator(memcore.EnglishEstimation),
	}
}

// SelectMessagesToEvict selects messages for eviction based on priority
func (p *PriorityEvictionPolicy) SelectMessagesToEvict(
	ctx context.Context,
	messages []llm.Message,
	targetCount int,
) []llm.Message {
	if len(messages) <= targetCount || targetCount < 0 {
		return nil
	}
	priorityMessages := make([]messageWithPriority, len(messages))
	for i, msg := range messages {
		priority := p.extractPriority(msg)
		priorityMessages[i] = messageWithPriority{
			message:       msg,
			priority:      priority,
			tokenEstimate: p.estimateTokens(ctx, msg),
			index:         i,
		}
	}
	sort.SliceStable(priorityMessages, func(i, j int) bool {
		if priorityMessages[i].priority != priorityMessages[j].priority {
			return priorityMessages[i].priority > priorityMessages[j].priority
		}
		return priorityMessages[i].index < priorityMessages[j].index
	})
	// NOTE: Preserve critical system prompts regardless of eviction pressure.
	evictCount := len(messages) - targetCount
	evicted := make([]llm.Message, 0, evictCount)
	evictedCount := 0
	for i := 0; i < len(priorityMessages) && evictedCount < evictCount; i++ {
		if priorityMessages[i].priority != PriorityCritical {
			evicted = append(evicted, priorityMessages[i].message)
			evictedCount++
		}
	}
	return evicted
}

// extractPriority determines the priority of a message based on role and content
func (p *PriorityEvictionPolicy) extractPriority(msg llm.Message) MessagePriority {
	switch msg.Role {
	case llm.MessageRoleSystem:
		return PriorityCritical // Never evict system messages
	case llm.MessageRoleAssistant:
		if p.containsImportantKeywords(msg.Content) {
			return PriorityHigh
		}
		return PriorityMedium
	case llm.MessageRoleUser:
		if p.containsImportantKeywords(msg.Content) {
			return PriorityHigh
		}
		return PriorityMedium
	case llm.MessageRoleTool:
		return PriorityLow
	default:
		return PriorityLowest
	}
}

// containsImportantKeywords checks if content contains important keywords
func (p *PriorityEvictionPolicy) containsImportantKeywords(content string) bool {
	lowerContent := strings.ToLower(content)
	for _, keyword := range p.importantKeywords {
		if strings.Contains(lowerContent, keyword) {
			return true
		}
	}
	return false
}

// estimateTokens provides a token count estimate using the project's token estimator
func (p *PriorityEvictionPolicy) estimateTokens(ctx context.Context, msg llm.Message) int {
	contentTokens := p.tokenEstimator.EstimateTokens(ctx, msg.Content)
	roleOverhead := len(string(msg.Role)) + 2 // Role plus formatting overhead
	return contentTokens + roleOverhead
}

// WithTokenEstimator sets a custom token estimator
func (p *PriorityEvictionPolicy) WithTokenEstimator(estimator memcore.TokenEstimator) *PriorityEvictionPolicy {
	if estimator != nil {
		p.tokenEstimator = estimator
	}
	return p
}

// GetType returns the policy type
func (p *PriorityEvictionPolicy) GetType() string {
	return p.name
}
