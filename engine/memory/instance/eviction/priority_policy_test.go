package eviction

import (
	"testing"

	"github.com/compozy/compozy/engine/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPriorityEvictionPolicy_NewPriorityEvictionPolicy(t *testing.T) {
	t.Run("Should create priority eviction policy", func(t *testing.T) {
		policy := NewPriorityEvictionPolicy()
		require.NotNil(t, policy)
		assert.Equal(t, "priority", policy.GetType())
	})
}

func TestPriorityEvictionPolicy_SelectMessagesToEvict(t *testing.T) {
	policy := NewPriorityEvictionPolicy()

	t.Run("Should return nil when no eviction needed", func(t *testing.T) {
		messages := []llm.Message{
			{Role: llm.MessageRoleUser, Content: "Message 1"},
			{Role: llm.MessageRoleAssistant, Content: "Message 2"},
		}
		evicted := policy.SelectMessagesToEvict(messages, 5)
		assert.Nil(t, evicted)
	})

	t.Run("Should never evict system messages", func(t *testing.T) {
		messages := []llm.Message{
			{Role: llm.MessageRoleSystem, Content: "System prompt"},
			{Role: llm.MessageRoleUser, Content: "User message"},
			{Role: llm.MessageRoleAssistant, Content: "Assistant response"},
			{Role: llm.MessageRoleTool, Content: "Tool output"},
		}
		// Keep only 1 message
		evicted := policy.SelectMessagesToEvict(messages, 1)
		require.Len(t, evicted, 3)
		// System message should not be in evicted list
		for _, msg := range evicted {
			assert.NotEqual(t, llm.MessageRoleSystem, msg.Role)
		}
		// Tool message should be evicted first
		assert.Equal(t, llm.MessageRoleTool, evicted[0].Role)
	})

	t.Run("Should evict based on role priority", func(t *testing.T) {
		messages := []llm.Message{
			{Role: llm.MessageRoleAssistant, Content: "Assistant 1"},
			{Role: llm.MessageRoleTool, Content: "Tool 1"},
			{Role: llm.MessageRoleUser, Content: "User 1"},
			{Role: "unknown", Content: "Unknown role"},
			{Role: llm.MessageRoleTool, Content: "Tool 2"},
		}
		// Keep only 2 messages
		evicted := policy.SelectMessagesToEvict(messages, 2)
		require.Len(t, evicted, 3)
		// Should evict in order: unknown role, tool messages
		assert.Equal(t, llm.MessageRole("unknown"), evicted[0].Role)
		assert.Equal(t, llm.MessageRoleTool, evicted[1].Role)
		assert.Equal(t, llm.MessageRoleTool, evicted[2].Role)
	})

	t.Run("Should prioritize messages with important keywords", func(t *testing.T) {
		messages := []llm.Message{
			{Role: llm.MessageRoleUser, Content: "Normal message"},
			{Role: llm.MessageRoleUser, Content: "This is critical information"},
			{Role: llm.MessageRoleUser, Content: "Another normal message"},
			{Role: llm.MessageRoleUser, Content: "Error occurred here"},
		}
		// Keep only 2 messages
		evicted := policy.SelectMessagesToEvict(messages, 2)
		require.Len(t, evicted, 2)
		// Should evict normal messages, keep important ones
		assert.Equal(t, "Normal message", evicted[0].Content)
		assert.Equal(t, "Another normal message", evicted[1].Content)
	})

	t.Run("Should maintain FIFO order within same priority", func(t *testing.T) {
		messages := []llm.Message{
			{Role: llm.MessageRoleUser, Content: "First user message"},
			{Role: llm.MessageRoleUser, Content: "Second user message"},
			{Role: llm.MessageRoleUser, Content: "Third user message"},
			{Role: llm.MessageRoleUser, Content: "Fourth user message"},
		}
		// Keep only 2 messages
		evicted := policy.SelectMessagesToEvict(messages, 2)
		require.Len(t, evicted, 2)
		// Should evict oldest messages first within same priority
		assert.Equal(t, "First user message", evicted[0].Content)
		assert.Equal(t, "Second user message", evicted[1].Content)
	})

	t.Run("Should handle mixed priorities correctly", func(t *testing.T) {
		messages := []llm.Message{
			{Role: llm.MessageRoleSystem, Content: "System instructions"},
			{Role: llm.MessageRoleAssistant, Content: "Important error message"},
			{Role: llm.MessageRoleTool, Content: "Tool output 1"},
			{Role: llm.MessageRoleUser, Content: "User query"},
			{Role: llm.MessageRoleAssistant, Content: "Normal response"},
			{Role: llm.MessageRoleTool, Content: "Tool output 2"},
		}
		// Keep only 3 messages
		evicted := policy.SelectMessagesToEvict(messages, 3)
		require.Len(t, evicted, 3)
		// Should evict tool messages first, then lower priority messages
		evictedRoles := make(map[string]int)
		for _, msg := range evicted {
			evictedRoles[string(msg.Role)]++
		}
		assert.Equal(t, 2, evictedRoles[string(llm.MessageRoleTool)])
		// System message should never be evicted
		assert.Equal(t, 0, evictedRoles[string(llm.MessageRoleSystem)])
	})

	t.Run("Should handle empty message list", func(t *testing.T) {
		evicted := policy.SelectMessagesToEvict([]llm.Message{}, 0)
		assert.Nil(t, evicted)
	})

	t.Run("Should handle negative target count", func(t *testing.T) {
		messages := []llm.Message{
			{Role: llm.MessageRoleUser, Content: "Message 1"},
		}
		evicted := policy.SelectMessagesToEvict(messages, -1)
		assert.Nil(t, evicted)
	})

	t.Run("Should handle all critical messages", func(t *testing.T) {
		messages := []llm.Message{
			{Role: llm.MessageRoleSystem, Content: "System 1"},
			{Role: llm.MessageRoleSystem, Content: "System 2"},
			{Role: llm.MessageRoleSystem, Content: "System 3"},
		}
		// Try to keep only 1 message
		evicted := policy.SelectMessagesToEvict(messages, 1)
		// Should not evict any system messages
		assert.Empty(t, evicted)
	})
}

func TestPriorityEvictionPolicy_extractPriority(t *testing.T) {
	policy := NewPriorityEvictionPolicy()

	testCases := []struct {
		name     string
		message  llm.Message
		expected MessagePriority
	}{
		{
			name:     "System message gets critical priority",
			message:  llm.Message{Role: llm.MessageRoleSystem, Content: "System prompt"},
			expected: PriorityCritical,
		},
		{
			name:     "Assistant message with error keyword gets high priority",
			message:  llm.Message{Role: llm.MessageRoleAssistant, Content: "An error occurred"},
			expected: PriorityHigh,
		},
		{
			name:     "Assistant message without keywords gets medium priority",
			message:  llm.Message{Role: llm.MessageRoleAssistant, Content: "Here is the answer"},
			expected: PriorityMedium,
		},
		{
			name:     "User message with critical keyword gets high priority",
			message:  llm.Message{Role: llm.MessageRoleUser, Content: "This is critical"},
			expected: PriorityHigh,
		},
		{
			name:     "User message without keywords gets medium priority",
			message:  llm.Message{Role: llm.MessageRoleUser, Content: "Hello there"},
			expected: PriorityMedium,
		},
		{
			name:     "Tool message gets low priority",
			message:  llm.Message{Role: llm.MessageRoleTool, Content: "Tool output"},
			expected: PriorityLow,
		},
		{
			name:     "Unknown role gets lowest priority",
			message:  llm.Message{Role: "custom", Content: "Custom message"},
			expected: PriorityLowest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			priority := policy.extractPriority(tc.message)
			assert.Equal(t, tc.expected, priority)
		})
	}
}

func TestPriorityEvictionPolicy_containsImportantKeywords(t *testing.T) {
	policy := NewPriorityEvictionPolicy()

	testCases := []struct {
		content  string
		expected bool
	}{
		{"This is an error message", true},
		{"Critical system failure", true},
		{"Important information", true},
		{"Warning: low memory", true},
		{"The operation failed", true},
		{"Exception thrown", true},
		{"Issue detected", true},
		{"Problem found", true},
		{"Urgent: review needed", true},
		{"Key requirement", true},
		{"Essential feature", true},
		{"Required field missing", true},
		{"Normal message", false},
		{"Hello world", false},
		{"Everything is fine", false},
		{"SUCCESS", false},
	}

	for _, tc := range testCases {
		t.Run(tc.content, func(t *testing.T) {
			result := policy.containsImportantKeywords(tc.content)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestPriorityEvictionPolicy_GetType(t *testing.T) {
	t.Run("Should return correct policy type", func(t *testing.T) {
		policy := NewPriorityEvictionPolicy()
		assert.Equal(t, "priority", policy.GetType())
	})
}

func TestPriorityEvictionPolicy_NewPriorityEvictionPolicyWithKeywords(t *testing.T) {
	t.Run("Should create policy with custom keywords", func(t *testing.T) {
		customKeywords := []string{"bug", "fix", "deadline"}
		policy := NewPriorityEvictionPolicyWithKeywords(customKeywords)
		require.NotNil(t, policy)
		assert.Equal(t, "priority", policy.GetType())
		assert.Equal(t, customKeywords, policy.importantKeywords)
	})

	t.Run("Should use default keywords when empty list provided", func(t *testing.T) {
		policy := NewPriorityEvictionPolicyWithKeywords([]string{})
		require.NotNil(t, policy)
		assert.Equal(t, getDefaultPriorityKeywords(), policy.importantKeywords)
	})

	t.Run("Should use default keywords when nil provided", func(t *testing.T) {
		policy := NewPriorityEvictionPolicyWithKeywords(nil)
		require.NotNil(t, policy)
		assert.Equal(t, getDefaultPriorityKeywords(), policy.importantKeywords)
	})
}

func TestPriorityEvictionPolicy_customKeywords(t *testing.T) {
	t.Run("Should use custom keywords for priority detection", func(t *testing.T) {
		customKeywords := []string{"security", "vulnerability", "breach"}
		policy := NewPriorityEvictionPolicyWithKeywords(customKeywords)

		testCases := []struct {
			name     string
			content  string
			expected bool
		}{
			{"Should detect security keyword", "Security issue found", true},
			{"Should detect vulnerability keyword", "Vulnerability assessment", true},
			{"Should detect breach keyword", "Data breach detected", true},
			{"Should ignore default keywords", "Critical error occurred", false},
			{"Should ignore default keywords", "Important warning", false},
			{"Should ignore non-matching content", "Normal message", false},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := policy.containsImportantKeywords(tc.content)
				assert.Equal(t, tc.expected, result, "Content: %s", tc.content)
			})
		}
	})

	t.Run("Should respect case insensitivity", func(t *testing.T) {
		customKeywords := []string{"bug", "fix", "urgent"}
		policy := NewPriorityEvictionPolicyWithKeywords(customKeywords)

		testCases := []struct {
			content  string
			expected bool
		}{
			{"Found a BUG in the system", true},
			{"Need to fix this issue", true},
			{"This is urgent!", true},
			{"This is URGENT!", true},
			{"bug report submitted", true},
			{"Normal message", false},
		}

		for _, tc := range testCases {
			t.Run(tc.content, func(t *testing.T) {
				result := policy.containsImportantKeywords(tc.content)
				assert.Equal(t, tc.expected, result)
			})
		}
	})
}

func TestPriorityEvictionPolicy_customKeywordsPriority(t *testing.T) {
	t.Run("Should prioritize messages with custom keywords during eviction", func(t *testing.T) {
		customKeywords := []string{"bug", "deadline"}
		policy := NewPriorityEvictionPolicyWithKeywords(customKeywords)

		messages := []llm.Message{
			{Role: llm.MessageRoleUser, Content: "Normal user message"},
			{Role: llm.MessageRoleUser, Content: "Found a bug in the code"},
			{Role: llm.MessageRoleUser, Content: "Another normal message"},
			{Role: llm.MessageRoleUser, Content: "Deadline approaching fast"},
		}

		// Keep only 2 messages
		evicted := policy.SelectMessagesToEvict(messages, 2)
		require.Len(t, evicted, 2)

		// Should evict normal messages, keep ones with custom keywords
		evictedContents := []string{evicted[0].Content, evicted[1].Content}
		assert.Contains(t, evictedContents, "Normal user message")
		assert.Contains(t, evictedContents, "Another normal message")

		// Important messages should not be evicted
		assert.NotContains(t, evictedContents, "Found a bug in the code")
		assert.NotContains(t, evictedContents, "Deadline approaching fast")
	})
}
