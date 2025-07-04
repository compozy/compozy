package uc

import (
	"fmt"
	"regexp"

	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/memory/service"
)

var (
	// Valid memory reference pattern: alphanumeric with underscores, 1-100 chars
	memRefPattern = regexp.MustCompile(`^[a-zA-Z0-9_]{1,100}$`)
	// Valid key pattern: no control characters, 1-255 chars
	keyPattern = regexp.MustCompile(`^[^\x00-\x1F\x7F]{1,255}$`)
)

// getDefaultLimits returns default validation limits from service layer
func getDefaultLimits() *service.ValidationLimits {
	config := service.DefaultConfig()
	return &config.ValidationLimits
}

// ValidateMemoryRef validates a memory reference
func ValidateMemoryRef(ref string) error {
	if ref == "" {
		return NewValidationError("memory_ref", ref, "memory reference cannot be empty")
	}
	if !memRefPattern.MatchString(ref) {
		return NewValidationError("memory_ref", ref, "must be alphanumeric with underscores, 1-100 characters")
	}
	return nil
}

// ValidateKey validates a memory key
func ValidateKey(key string) error {
	if key == "" {
		return NewValidationError("key", key, "key cannot be empty")
	}
	if !keyPattern.MatchString(key) {
		return NewValidationError("key", key, "must not contain control characters, 1-255 characters")
	}

	return nil
}

// ValidateRawMessages validates an array of raw message maps
func ValidateRawMessages(messages []map[string]any) error {
	if len(messages) == 0 {
		return NewValidationError("messages", nil, "messages cannot be empty")
	}
	limits := getDefaultLimits()
	// Check maximum number of messages
	if len(messages) > limits.MaxMessagesPerRequest {
		return NewValidationError("messages", len(messages),
			fmt.Sprintf("exceeded maximum number of messages (%d)", limits.MaxMessagesPerRequest))
	}
	totalContentSize := 0
	for i, msg := range messages {
		// Validate using existing ValidateMessage function
		if err := ValidateMessage(msg, i); err != nil {
			return err
		}
		// Track total content size
		if content, ok := msg["content"].(string); ok {
			totalContentSize += len(content)
		}
	}
	// Check total content size
	if totalContentSize > limits.MaxTotalContentSize {
		return NewValidationError("messages", totalContentSize,
			fmt.Sprintf("total content size exceeds maximum of %d bytes", limits.MaxTotalContentSize))
	}
	return nil
}

// ValidateMessage validates a single message
func ValidateMessage(msg map[string]any, index int) error {
	limits := getDefaultLimits()
	// Check content
	content, ok := msg["content"].(string)
	if !ok || content == "" {
		return NewValidationError(
			"content",
			nil,
			fmt.Sprintf("message[%d] content is required and must be a string", index),
		)
	}
	// Check content length using configurable limits
	if len(content) > limits.MaxMessageContentLength {
		return NewValidationError("content", len(content),
			fmt.Sprintf("message[%d] content too long (max %d bytes)", index, limits.MaxMessageContentLength))
	}
	// Check role if provided
	if role, exists := msg["role"]; exists {
		roleStr, ok := role.(string)
		if !ok {
			return NewValidationError("role", role, fmt.Sprintf("message[%d] role must be a string", index))
		}
		if err := ValidateMessageRole(roleStr); err != nil {
			return NewValidationError("role", roleStr, fmt.Sprintf("message[%d] %v", index, err))
		}
	}
	return nil
}

// ValidateMessageRole validates if the role is acceptable
func ValidateMessageRole(role string) error {
	switch role {
	case string(llm.MessageRoleUser), string(llm.MessageRoleAssistant),
		string(llm.MessageRoleSystem), string(llm.MessageRoleTool):
		return nil
	default:
		return NewValidationError("role", role, "must be one of: user, assistant, system, tool")
	}
}

// ValidateFlushInput validates flush operation input
func ValidateFlushInput(input *FlushMemoryInput) error {
	if input == nil {
		return ErrInvalidPayload
	}

	// Validate max keys
	if input.MaxKeys < 0 {
		return NewValidationError("max_keys", input.MaxKeys, "must be non-negative")
	}
	if input.MaxKeys > 10000 {
		return NewValidationError("max_keys", input.MaxKeys, "too large (max 10000)")
	}

	// Strategy validation is handled by the service layer
	// No need to duplicate it here

	return nil
}

// ValidateClearInput validates clear operation input
func ValidateClearInput(input *ClearMemoryInput) error {
	if input == nil {
		return ErrInvalidPayload
	}

	// Confirm flag must be true for safety
	if !input.Confirm {
		return NewValidationError("confirm", input.Confirm, "must be true to clear memory")
	}

	return nil
}
