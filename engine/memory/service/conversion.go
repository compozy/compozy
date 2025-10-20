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
	if limits != nil && limits.MaxMessageContentLength > 0 && len(content) > limits.MaxMessageContentLength {
		return llm.Message{}, memcore.NewMemoryError(
			memcore.ErrCodeInvalidConfig,
			fmt.Sprintf("message content too long (max %d bytes)", limits.MaxMessageContentLength),
			nil,
		).WithContext("content_length", len(content)).WithContext("max_allowed", limits.MaxMessageContentLength)
	}
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
	switch value := payload.(type) {
	case map[string]any:
		return convertSingleMap(value, limits)
	case []map[string]any:
		return convertMapSlice(value, limits)
	case []any:
		return convertAnySlice(value, limits)
	case string:
		return convertStringPayload(value, limits)
	default:
		return nil, memcore.NewMemoryError(
			memcore.ErrCodeInvalidConfig,
			"unsupported payload format",
			nil,
		).WithContext("payload_type", fmt.Sprintf("%T", payload))
	}
}

func convertSingleMap(msg map[string]any, limits *ValidationLimits) ([]llm.Message, error) {
	message, err := MapToMessageWithLimits(msg, limits)
	if err != nil {
		return nil, err
	}
	return []llm.Message{message}, nil
}

func convertMapSlice(messages []map[string]any, limits *ValidationLimits) ([]llm.Message, error) {
	result := make([]llm.Message, 0, len(messages))
	for i, msg := range messages {
		message, err := MapToMessageWithLimits(msg, limits)
		if err != nil {
			return nil, wrapMessageError(i, err)
		}
		result = append(result, message)
	}
	return result, nil
}

func convertAnySlice(messages []any, limits *ValidationLimits) ([]llm.Message, error) {
	result := make([]llm.Message, 0, len(messages))
	for i, item := range messages {
		msg, ok := item.(map[string]any)
		if !ok {
			return nil, memcore.NewMemoryError(
				memcore.ErrCodeInvalidConfig,
				fmt.Sprintf("invalid message format at index %d", i),
				nil,
			).WithContext("index", i)
		}
		message, err := MapToMessageWithLimits(msg, limits)
		if err != nil {
			return nil, wrapMessageError(i, err)
		}
		result = append(result, message)
	}
	return result, nil
}

func convertStringPayload(content string, limits *ValidationLimits) ([]llm.Message, error) {
	if err := validateStringContent(content, limits); err != nil {
		return nil, err
	}
	return []llm.Message{{Role: llm.MessageRoleUser, Content: content}}, nil
}

func wrapMessageError(index int, err error) error {
	return memcore.NewMemoryError(
		memcore.ErrCodeInvalidConfig,
		fmt.Sprintf("message[%d]", index),
		err,
	).WithContext("index", index)
}

func validateStringContent(content string, limits *ValidationLimits) error {
	if limits == nil || limits.MaxMessageContentLength <= 0 {
		return nil
	}
	if len(content) > limits.MaxMessageContentLength {
		return memcore.NewMemoryError(
			memcore.ErrCodeInvalidConfig,
			fmt.Sprintf("string content too long (max %d bytes)", limits.MaxMessageContentLength),
			nil,
		).WithContext("content_length", len(content)).WithContext("max_allowed", limits.MaxMessageContentLength)
	}
	return nil
}
