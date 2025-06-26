package privacy

import (
	"context"
	"time"

	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/memory/core"
)

// ManagerInterface defines the interface for privacy management operations
type ManagerInterface interface {
	// RegisterPolicy registers a privacy policy for a specific resource
	RegisterPolicy(resourceID string, policy *core.PrivacyPolicyConfig) error
	// GetPolicy retrieves the privacy policy for a resource
	GetPolicy(resourceID string) (*core.PrivacyPolicyConfig, bool)
	// ApplyPrivacyControls applies privacy controls to a message
	ApplyPrivacyControls(
		ctx context.Context,
		msg llm.Message,
		resourceID string,
		metadata core.PrivacyMetadata,
	) (llm.Message, core.PrivacyMetadata, error)
	// RedactContent applies redaction patterns to content
	RedactContent(content string, patterns []string, defaultRedaction string) (string, error)
	// ShouldPersistMessage determines if a message should be persisted based on privacy rules
	ShouldPersistMessage(msgType string, nonPersistableTypes []string) bool
}

// RedactionEngine handles content redaction operations
type RedactionEngine interface {
	// ValidatePattern validates a regex pattern for safety
	ValidatePattern(pattern string) error
	// ApplyPattern applies a single redaction pattern to content
	ApplyPattern(content, pattern, replacement string) (string, error)
	// ApplyPatterns applies multiple redaction patterns with timeout protection
	ApplyPatterns(content string, patterns []string, replacement string, timeout time.Duration) (string, error)
}
