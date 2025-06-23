package memory

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/pkg/logger"
)

// Privacy error codes
const (
	ErrCodePrivacyRedaction  = "PRIVACY_REDACTION_ERROR"
	ErrCodePrivacyPolicy     = "PRIVACY_POLICY_ERROR"
	ErrCodePrivacyValidation = "PRIVACY_VALIDATION_ERROR"
)

// Default redaction string
const DefaultRedactionString = "[REDACTED]"

// PrivacyManager handles privacy controls and data protection
type PrivacyManager struct {
	policies             map[string]*PrivacyPolicyConfig
	compiledPatterns     map[string][]*regexp.Regexp
	mu                   sync.RWMutex
	consecutiveErrors    int
	maxConsecutiveErrors int
}

// NewPrivacyManager creates a new privacy manager
func NewPrivacyManager() *PrivacyManager {
	return &PrivacyManager{
		policies:             make(map[string]*PrivacyPolicyConfig),
		compiledPatterns:     make(map[string][]*regexp.Regexp),
		maxConsecutiveErrors: 10,
	}
}

// RegisterPolicy registers a privacy policy for a memory resource
func (pm *PrivacyManager) RegisterPolicy(resourceID string, policy *PrivacyPolicyConfig) error {
	if resourceID == "" {
		return core.NewError(
			fmt.Errorf("resource ID cannot be empty"),
			ErrCodePrivacyPolicy,
			nil,
		)
	}
	if policy == nil {
		// No policy means no privacy controls for this resource
		return nil
	}
	pm.mu.Lock()
	defer pm.mu.Unlock()
	// Compile regex patterns
	var compiledPatterns []*regexp.Regexp
	for _, pattern := range policy.RedactPatterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return core.NewError(
				fmt.Errorf("invalid regex pattern: %s: %w", pattern, err),
				ErrCodePrivacyPolicy,
				map[string]any{"pattern": pattern},
			)
		}
		compiledPatterns = append(compiledPatterns, re)
	}
	pm.policies[resourceID] = policy
	pm.compiledPatterns[resourceID] = compiledPatterns
	return nil
}

// applyRedactionPatterns applies redaction patterns to content
func (pm *PrivacyManager) applyRedactionPatterns(
	content string,
	patterns []*regexp.Regexp,
	redactionString string,
) (string, error) {
	redactedContent := content
	for _, pattern := range patterns {
		if pattern == nil {
			err := core.NewError(
				fmt.Errorf("nil pattern encountered"),
				ErrCodePrivacyRedaction,
				nil,
			)
			pm.mu.Lock()
			pm.consecutiveErrors++
			pm.mu.Unlock()
			return "", err
		}
		var redactionErr error
		func() {
			defer func() {
				if r := recover(); r != nil {
					redactionErr = fmt.Errorf("regex panic: %v", r)
				}
			}()
			redactedContent = pattern.ReplaceAllString(redactedContent, redactionString)
		}()
		if redactionErr != nil {
			pm.mu.Lock()
			pm.consecutiveErrors++
			pm.mu.Unlock()
			return "", core.NewError(redactionErr, ErrCodePrivacyRedaction, nil)
		}
	}
	return redactedContent, nil
}

// RedactMessage applies privacy redaction to a message
func (pm *PrivacyManager) RedactMessage(ctx context.Context, resourceID string, msg llm.Message) (llm.Message, error) {
	pm.mu.RLock()
	policy, exists := pm.policies[resourceID]
	patterns := pm.compiledPatterns[resourceID]
	pm.mu.RUnlock()
	if !exists || policy == nil {
		// No privacy policy for this resource
		return msg, nil
	}
	// Apply circuit breaker logic
	pm.mu.RLock()
	currentErrors := pm.consecutiveErrors
	pm.mu.RUnlock()
	if currentErrors >= pm.maxConsecutiveErrors {
		return msg, core.NewError(
			fmt.Errorf("privacy redaction circuit breaker open"),
			ErrCodePrivacyRedaction,
			map[string]any{"consecutive_errors": currentErrors},
		)
	}
	// Defer panic recovery and error handling
	var redactionErr error
	defer func() {
		if r := recover(); r != nil {
			redactionErr = core.NewError(
				fmt.Errorf("panic during redaction: %v", r),
				ErrCodePrivacyRedaction,
				map[string]any{"panic": r, "resource_id": resourceID},
			)
		}
		// Update error counter based on result
		pm.mu.Lock()
		if redactionErr != nil {
			pm.consecutiveErrors++
			// Log error for monitoring
			if ctx != nil {
				log := logger.FromContext(ctx)
				log.Error("privacy redaction failed",
					"error", redactionErr,
					"resource_id", resourceID,
					"consecutive_errors", pm.consecutiveErrors,
				)
			}
		} else {
			pm.consecutiveErrors = 0
		}
		pm.mu.Unlock()
	}()
	// Get redaction string
	redactionString := policy.DefaultRedactionString
	if redactionString == "" {
		redactionString = DefaultRedactionString
	}
	// Apply redaction patterns
	redactedContent, err := pm.applyRedactionPatterns(msg.Content, patterns, redactionString)
	if err != nil {
		redactionErr = err
		return msg, err
	}
	return llm.Message{
		Role:    msg.Role,
		Content: redactedContent,
	}, nil
}

// ShouldPersistMessage checks if a message should be persisted based on privacy policy
func (pm *PrivacyManager) ShouldPersistMessage(resourceID string, msg llm.Message) bool {
	pm.mu.RLock()
	policy, exists := pm.policies[resourceID]
	pm.mu.RUnlock()
	if !exists || policy == nil {
		// No privacy policy means persist everything
		return true
	}
	// Check non-persistable message types
	for _, nonPersistableType := range policy.NonPersistableMessageTypes {
		if strings.EqualFold(string(msg.Role), nonPersistableType) {
			return false
		}
	}
	return true
}

// LogPrivacyExclusion logs when sensitive data is excluded from persistence
func (pm *PrivacyManager) LogPrivacyExclusion(
	ctx context.Context,
	resourceID string,
	reason string,
	metadata map[string]any,
) {
	log := logger.FromContext(ctx)
	logData := map[string]any{
		"resource_id": resourceID,
		"reason":      reason,
	}
	// Merge metadata
	for k, v := range metadata {
		logData[k] = v
	}
	log.Info("Privacy exclusion applied", logData)
}

// GetCircuitBreakerStatus returns the current circuit breaker status
func (pm *PrivacyManager) GetCircuitBreakerStatus() (isOpen bool, consecutiveErrors int, maxErrors int) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.consecutiveErrors >= pm.maxConsecutiveErrors, pm.consecutiveErrors, pm.maxConsecutiveErrors
}

// ResetCircuitBreaker resets the circuit breaker error counter
func (pm *PrivacyManager) ResetCircuitBreaker() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.consecutiveErrors = 0
}

// ValidatePrivacyPolicy validates a privacy policy configuration
func ValidatePrivacyPolicy(policy *PrivacyPolicyConfig) error {
	if policy == nil {
		return nil
	}
	// Validate regex patterns
	for _, pattern := range policy.RedactPatterns {
		if _, err := regexp.Compile(pattern); err != nil {
			return core.NewError(
				fmt.Errorf("invalid regex pattern: %s: %w", pattern, err),
				ErrCodePrivacyValidation,
				map[string]any{"pattern": pattern},
			)
		}
	}
	return nil
}

// Common redaction patterns
var CommonRedactionPatterns = map[string]string{
	"ssn":         `\b\d{3}-\d{2}-\d{4}\b`,
	"credit_card": `\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b`,
	"email":       `\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`,
	"phone":       `\b(\+?1[-.\s]?)?\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}\b`,
	"ip_address":  `\b(?:\d{1,3}\.){3}\d{1,3}\b`,
}

// BuildRedactionPattern builds a redaction pattern from common patterns
func BuildRedactionPattern(patterns ...string) []string {
	var result []string
	for _, p := range patterns {
		if pattern, ok := CommonRedactionPatterns[p]; ok {
			result = append(result, pattern)
		} else {
			// Assume it's a custom pattern
			result = append(result, p)
		}
	}
	return result
}
