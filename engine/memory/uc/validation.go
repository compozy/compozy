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
		return ErrInvalidMemoryRef
	}
	if !memRefPattern.MatchString(ref) {
		return fmt.Errorf("%w: must be alphanumeric with underscores, 1-100 characters", ErrInvalidMemoryRef)
	}
	return nil
}

// ValidateKey validates a memory key
func ValidateKey(key string) error {
	if key == "" {
		return ErrInvalidKey
	}
	if !keyPattern.MatchString(key) {
		return fmt.Errorf("%w: must not contain control characters, 1-255 characters", ErrInvalidKey)
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
		return fmt.Errorf("%w: message[%d] content is required and must be a string", ErrInvalidPayload, index)
	}
	// Check content length using configurable limits
	if len(content) > limits.MaxMessageContentLength {
		return fmt.Errorf(
			"%w: message[%d] content too long (max %d bytes)",
			ErrInvalidPayload,
			index,
			limits.MaxMessageContentLength,
		)
	}
	// Check role if provided
	if role, exists := msg["role"]; exists {
		roleStr, ok := role.(string)
		if !ok {
			return fmt.Errorf("%w: message[%d] role must be a string", ErrInvalidPayload, index)
		}
		if err := ValidateMessageRole(roleStr); err != nil {
			return fmt.Errorf("%w: message[%d] %v", ErrInvalidPayload, index, err)
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
		return fmt.Errorf("invalid message role '%s', must be one of: user, assistant, system, tool", role)
	}
}

// ValidateFlushInput validates flush operation input
func ValidateFlushInput(input *FlushMemoryInput) error {
	if input == nil {
		return ErrInvalidPayload
	}

	// Validate max keys
	if input.MaxKeys < 0 {
		return fmt.Errorf("%w: max_keys must be non-negative", ErrInvalidPayload)
	}
	if input.MaxKeys > 10000 {
		return fmt.Errorf("%w: max_keys too large (max 10000)", ErrInvalidPayload)
	}

	// Validate strategy if provided
	if input.Strategy != "" {
		validStrategies := []string{"summarize", "trim", "archive", "hybrid"}
		valid := false
		for _, s := range validStrategies {
			if input.Strategy == s {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("%w: invalid strategy '%s', must be one of: %v",
				ErrInvalidPayload, input.Strategy, validStrategies)
		}
	}

	return nil
}

// ValidateClearInput validates clear operation input
func ValidateClearInput(input *ClearMemoryInput) error {
	if input == nil {
		return ErrInvalidPayload
	}

	// Confirm flag must be true for safety
	if !input.Confirm {
		return fmt.Errorf("%w: confirm flag must be true to clear memory", ErrInvalidPayload)
	}

	return nil
}
