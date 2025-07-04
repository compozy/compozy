package service

import (
	"fmt"
	"unicode"

	"github.com/compozy/compozy/engine/llm"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/memory/instance/strategies"
)

// Default validation limits - used when no config is provided
const (
	// DefaultMaxMemoryRefLength is the default maximum allowed length for a memory reference
	DefaultMaxMemoryRefLength = 100
	// DefaultMaxKeyLength is the default maximum allowed length for a memory key
	DefaultMaxKeyLength = 255
	// DefaultMaxMessageContentLength is the default maximum allowed length for a single message content
	DefaultMaxMessageContentLength = 10 * 1024 // 10KB per message
	// DefaultMaxMessagesPerRequest is the default maximum number of messages allowed in a single request
	DefaultMaxMessagesPerRequest = 100
	// DefaultMaxTotalContentSize is the default maximum total size of all message content in a request
	DefaultMaxTotalContentSize = 100 * 1024 // 100KB total
)

// Legacy constants for backward compatibility
const (
	MaxMemoryRefLength      = DefaultMaxMemoryRefLength
	MaxKeyLength            = DefaultMaxKeyLength
	MaxMessageContentLength = DefaultMaxMessageContentLength
	MaxMessagesPerRequest   = DefaultMaxMessagesPerRequest
	MaxTotalContentSize     = DefaultMaxTotalContentSize
)

// ValidateMemoryRef validates a memory reference using default limits
func ValidateMemoryRef(ref string) error {
	return ValidateMemoryRefWithLimits(ref, nil)
}

// ValidateMemoryRefWithLimits validates a memory reference with configurable limits
func ValidateMemoryRefWithLimits(ref string, limits *ValidationLimits) error {
	maxLength := DefaultMaxMemoryRefLength
	if limits != nil && limits.MaxMemoryRefLength > 0 {
		maxLength = limits.MaxMemoryRefLength
	}
	if ref == "" {
		return memcore.NewMemoryError(
			memcore.ErrCodeInvalidConfig,
			"memory reference cannot be empty",
			nil,
		).WithContext("ref", ref)
	}
	if len(ref) > maxLength {
		return memcore.NewMemoryError(
			memcore.ErrCodeInvalidConfig,
			fmt.Sprintf("memory reference too long: maximum %d characters allowed", maxLength),
			nil,
		).WithContext("ref", ref).WithContext("length", len(ref)).WithContext("max_length", maxLength)
	}
	// Check each character - must be alphanumeric or underscore
	for i, r := range ref {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return memcore.NewMemoryError(
				memcore.ErrCodeInvalidConfig,
				fmt.Sprintf("invalid memory reference: character at position %d must be alphanumeric or underscore", i),
				nil,
			).WithContext("ref", ref).WithContext("position", i).WithContext("character", string(r))
		}
	}
	return nil
}

// ValidateKey validates a memory key using default limits
func ValidateKey(key string) error {
	return ValidateKeyWithLimits(key, nil)
}

// ValidateKeyWithLimits validates a memory key with configurable limits
func ValidateKeyWithLimits(key string, limits *ValidationLimits) error {
	maxLength := DefaultMaxKeyLength
	if limits != nil && limits.MaxKeyLength > 0 {
		maxLength = limits.MaxKeyLength
	}
	if key == "" {
		return memcore.NewMemoryError(
			memcore.ErrCodeInvalidConfig,
			"memory key cannot be empty",
			nil,
		).WithContext("key", key)
	}
	if len(key) > maxLength {
		return memcore.NewMemoryError(
			memcore.ErrCodeInvalidConfig,
			fmt.Sprintf("memory key too long: maximum %d characters allowed", maxLength),
			nil,
		).WithContext("key", key).WithContext("length", len(key)).WithContext("max_length", maxLength)
	}
	// Check for control characters
	for i, r := range key {
		if r < 32 || r == 127 { // Control characters
			return memcore.NewMemoryError(
				memcore.ErrCodeInvalidConfig,
				fmt.Sprintf("invalid memory key: control character at position %d not allowed", i),
				nil,
			).WithContext("key", key).WithContext("position", i).WithContext("character_code", int(r))
		}
	}
	return nil
}

// ValidateRawMessages validates an array of raw message maps using default limits
func ValidateRawMessages(messages []map[string]any) error {
	return ValidateRawMessagesWithLimits(messages, nil)
}

// ValidateRawMessagesWithLimits validates an array of raw message maps with configurable limits
func ValidateRawMessagesWithLimits(messages []map[string]any, limits *ValidationLimits) error {
	maxMessagesPerRequest := DefaultMaxMessagesPerRequest
	maxTotalContentSize := DefaultMaxTotalContentSize
	if limits != nil {
		if limits.MaxMessagesPerRequest > 0 {
			maxMessagesPerRequest = limits.MaxMessagesPerRequest
		}
		if limits.MaxTotalContentSize > 0 {
			maxTotalContentSize = limits.MaxTotalContentSize
		}
	}
	if len(messages) == 0 {
		return memcore.NewMemoryError(
			memcore.ErrCodeInvalidConfig,
			"messages cannot be empty",
			nil,
		)
	}

	// Check maximum number of messages
	if len(messages) > maxMessagesPerRequest {
		return memcore.NewMemoryError(
			memcore.ErrCodeInvalidConfig,
			fmt.Sprintf("exceeded maximum number of messages (%d)", maxMessagesPerRequest),
			memcore.ErrMessageLimitExceeded,
		).WithContext("message_count", len(messages)).WithContext("max_allowed", maxMessagesPerRequest)
	}

	totalContentSize := 0
	for i, msg := range messages {
		// Validate using configurable ValidateMessage function
		if err := ValidateMessageWithLimits(msg, i, limits); err != nil {
			return err
		}

		// Track total content size
		if content, ok := msg["content"].(string); ok {
			totalContentSize += len(content)
		}
	}

	// Check total content size
	if totalContentSize > maxTotalContentSize {
		return memcore.NewMemoryError(
			memcore.ErrCodeInvalidConfig,
			fmt.Sprintf("total content size exceeds maximum of %d bytes", maxTotalContentSize),
			memcore.ErrTokenLimitExceeded,
		).WithContext("total_size", totalContentSize).WithContext("max_allowed", maxTotalContentSize)
	}

	return nil
}

// ValidateMessage validates a single message using default limits
func ValidateMessage(msg map[string]any, index int) error {
	return ValidateMessageWithLimits(msg, index, nil)
}

// ValidateMessageWithLimits validates a single message with configurable limits
func ValidateMessageWithLimits(msg map[string]any, index int, limits *ValidationLimits) error {
	maxContentLength := DefaultMaxMessageContentLength
	if limits != nil && limits.MaxMessageContentLength > 0 {
		maxContentLength = limits.MaxMessageContentLength
	}
	// Check content
	content, ok := msg["content"].(string)
	if !ok || content == "" {
		return memcore.NewMemoryError(
			memcore.ErrCodeInvalidConfig,
			fmt.Sprintf("message[%d] content is required and must be a string", index),
			nil,
		).WithContext("message_index", index)
	}

	// Check content length (prevent DOS)
	if len(content) > maxContentLength {
		return memcore.NewMemoryError(
			memcore.ErrCodeInvalidConfig,
			fmt.Sprintf("message[%d] content too long (max %d bytes)", index, maxContentLength),
			nil,
		).WithContext("message_index", index).WithContext(
			"content_length", len(content),
		).WithContext("max_allowed", maxContentLength)
	}

	// Check role if provided
	if role, exists := msg["role"]; exists {
		roleStr, ok := role.(string)
		if !ok {
			return memcore.NewMemoryError(
				memcore.ErrCodeInvalidConfig,
				fmt.Sprintf("message[%d] role must be a string", index),
				nil,
			).WithContext("message_index", index).WithContext("role_type", fmt.Sprintf("%T", role))
		}
		if err := ValidateMessageRole(roleStr); err != nil {
			// Wrap the error with context
			if memErr, ok := err.(*memcore.MemoryError); ok {
				return memErr.WithContext("message_index", index)
			}
			return memcore.NewMemoryError(
				memcore.ErrCodeInvalidConfig,
				fmt.Sprintf("message[%d]: %v", index, err),
				err,
			).WithContext("message_index", index)
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
		return memcore.NewMemoryError(
			memcore.ErrCodeInvalidConfig,
			fmt.Sprintf("invalid message role '%s', must be one of: user, assistant, system, tool", role),
			nil,
		).WithContext("role", role)
	}
}

// ValidateBaseRequest validates common request fields using default limits
func ValidateBaseRequest(req *BaseRequest) error {
	return ValidateBaseRequestWithLimits(req, nil)
}

// ValidateBaseRequestWithLimits validates common request fields with configurable limits
func ValidateBaseRequestWithLimits(req *BaseRequest, limits *ValidationLimits) error {
	if err := ValidateMemoryRefWithLimits(req.MemoryRef, limits); err != nil {
		return err
	}
	return ValidateKeyWithLimits(req.Key, limits)
}

// ValidateFlushConfig validates flush operation configuration
func ValidateFlushConfig(config *FlushConfig, factory *strategies.StrategyFactory) error {
	if config == nil {
		return nil // Config is optional
	}

	// Validate max keys
	if config.MaxKeys < 0 {
		return memcore.NewMemoryError(
			memcore.ErrCodeInvalidConfig,
			"max_keys must be non-negative",
			nil,
		).WithContext("max_keys", config.MaxKeys)
	}
	if config.MaxKeys > 10000 {
		return memcore.NewMemoryError(
			memcore.ErrCodeInvalidConfig,
			"max_keys too large (max 10000)",
			nil,
		).WithContext("max_keys", config.MaxKeys)
	}

	// Validate threshold
	if config.Threshold < 0 || config.Threshold > 1 {
		return memcore.NewMemoryError(
			memcore.ErrCodeInvalidConfig,
			"threshold must be between 0 and 1",
			nil,
		).WithContext("threshold", config.Threshold)
	}

	// Validate strategy if provided
	if config.Strategy != "" {
		// Use the provided factory instance for validation
		if err := factory.ValidateStrategyType(config.Strategy); err != nil {
			validStrategies := factory.GetSupportedStrategies()
			return memcore.NewMemoryError(
				memcore.ErrCodeInvalidConfig,
				fmt.Sprintf("invalid strategy '%s', must be one of: %v", config.Strategy, validStrategies),
				nil,
			).WithContext("strategy", config.Strategy).WithContext("valid_strategies", validStrategies)
		}
	}

	return nil
}

// ValidateClearConfig validates clear operation configuration
func ValidateClearConfig(config *ClearConfig) error {
	if config == nil {
		return memcore.NewMemoryError(
			memcore.ErrCodeInvalidConfig,
			"clear operation requires clear_config to be provided",
			nil,
		)
	}

	// Confirm flag must be true for safety
	if !config.Confirm {
		return memcore.NewMemoryError(
			memcore.ErrCodeInvalidConfig,
			"confirm flag must be true to clear memory",
			nil,
		).WithContext("confirm", config.Confirm)
	}

	return nil
}
