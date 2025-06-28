package uc

import (
	"fmt"

	"github.com/compozy/compozy/engine/llm"
)

// ConvertToLLMMessages converts a slice of generic message maps to a slice of llm.Message.
// It also validates the content of the messages.
func ConvertToLLMMessages(messages []map[string]any) ([]llm.Message, error) {
	result := make([]llm.Message, 0, len(messages))
	for i, msg := range messages {
		role, ok := msg["role"].(string)
		if !ok || role == "" {
			role = "user" // Default to user role
		}
		content, ok := msg["content"].(string)
		if !ok || content == "" {
			return nil, fmt.Errorf("message[%d] content is required", i)
		}
		// Validate role
		if err := ValidateMessageRole(role); err != nil {
			return nil, fmt.Errorf("message[%d]: %w", i, err)
		}
		result = append(result, llm.Message{
			Role:    llm.MessageRole(role),
			Content: content,
		})
	}
	return result, nil
}
