package service

import (
	"fmt"

	"github.com/compozy/compozy/engine/llm"
	memcore "github.com/compozy/compozy/engine/memory/core"
)

// MapToMessageWithLimits converts a single message map to llm.Message with configurable limits
func MapToMessageWithLimits(msg map[string]any, limits *ValidationLimits) (llm.Message, error) {
	role, ok := msg["role"].(string)
	if !ok || role == "" {
		role = "user"
	}
	content, ok := msg["content"].(string)
	if !ok || content == "" {
		return llm.Message{}, memcore.NewMemoryError(
			memcore.ErrCodeInvalidConfig,
			"message content is required",
			nil,
		)
	}

	// Validate content length if limits are provided
	if limits != nil && limits.MaxMessageContentLength > 0 && len(content) > limits.MaxMessageContentLength {
		return llm.Message{}, memcore.NewMemoryError(
			memcore.ErrCodeInvalidConfig,
			fmt.Sprintf("message content too long (max %d bytes)", limits.MaxMessageContentLength),
			nil,
		).WithContext("content_length", len(content)).WithContext("max_allowed", limits.MaxMessageContentLength)
	}

	// Validate role
	if err := ValidateMessageRole(role); err != nil {
		return llm.Message{}, err
	}
	return llm.Message{Role: llm.MessageRole(role), Content: content}, nil
}

// PayloadToMessages converts various payload formats to []llm.Message using default limits
func PayloadToMessages(payload any) ([]llm.Message, error) {
	return PayloadToMessagesWithLimits(payload, nil)
}

// PayloadToMessagesWithLimits converts various payload formats to []llm.Message with configurable limits
func PayloadToMessagesWithLimits(payload any, limits *ValidationLimits) ([]llm.Message, error) {
	if payload == nil {
		return nil, memcore.NewMemoryError(
			memcore.ErrCodeInvalidConfig,
			"payload is nil",
			nil,
		)
	}
	// Handle single message
	if msg, ok := payload.(map[string]any); ok {
		message, err := MapToMessageWithLimits(msg, limits)
		if err != nil {
			return nil, err
		}
		return []llm.Message{message}, nil
	}
	// Handle array of messages ([]map[string]any)
	if messages, ok := payload.([]map[string]any); ok {
		result := make([]llm.Message, 0, len(messages))
		for i, msg := range messages {
			message, err := MapToMessageWithLimits(msg, limits)
			if err != nil {
				return nil, memcore.NewMemoryError(
					memcore.ErrCodeInvalidConfig,
					fmt.Sprintf("message[%d]", i),
					err,
				).WithContext("index", i)
			}
			result = append(result, message)
		}
		return result, nil
	}
	// Handle array of messages ([]any)
	if messages, ok := payload.([]any); ok {
		result := make([]llm.Message, 0, len(messages))
		for i, item := range messages {
			if msg, ok := item.(map[string]any); ok {
				message, err := MapToMessageWithLimits(msg, limits)
				if err != nil {
					return nil, memcore.NewMemoryError(
						memcore.ErrCodeInvalidConfig,
						fmt.Sprintf("message[%d]", i),
						err,
					).WithContext("index", i)
				}
				result = append(result, message)
			} else {
				return nil, memcore.NewMemoryError(
					memcore.ErrCodeInvalidConfig,
					fmt.Sprintf("invalid message format at index %d", i),
					nil,
				).WithContext("index", i)
			}
		}
		return result, nil
	}
	// Handle string payload as user message
	if content, ok := payload.(string); ok {
		// Validate string content length if limits are provided
		if limits != nil && limits.MaxMessageContentLength > 0 && len(content) > limits.MaxMessageContentLength {
			return nil, memcore.NewMemoryError(
				memcore.ErrCodeInvalidConfig,
				fmt.Sprintf("string content too long (max %d bytes)", limits.MaxMessageContentLength),
				nil,
			).WithContext("content_length", len(content)).WithContext("max_allowed", limits.MaxMessageContentLength)
		}
		return []llm.Message{{Role: llm.MessageRoleUser, Content: content}}, nil
	}
	return nil, memcore.NewMemoryError(
		memcore.ErrCodeInvalidConfig,
		"unsupported payload format",
		nil,
	).WithContext("payload_type", fmt.Sprintf("%T", payload))
}
