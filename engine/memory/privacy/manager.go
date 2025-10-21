package privacy

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/engine/llm"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/pkg/logger"
)

// Default redaction string
const DefaultRedactionString = "[REDACTED]"

// normalizeMetaKey normalizes a metadata key by lowercasing and removing non-alphanumeric characters
func normalizeMetaKey(k string) string {
	lower := strings.ToLower(k)
	b := strings.Builder{}
	for i := 0; i < len(lower); i++ {
		c := lower[i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			b.WriteByte(c)
		}
	}
	return b.String()
}

// isSensitiveMetaKey checks if a normalized key contains sensitive information
func isSensitiveMetaKey(k string) bool {
	switch k {
	case "content", "raw", "message", "body", "payload",
		"password", "pass", "passwd", "pwd",
		"token", "accesstoken", "refreshtoken",
		"secret", "clientsecret",
		"apikey", "xapikey",
		"authorization", "bearer", "auth", "credential", "credentials",
		"cookie", "cookies", "setcookie", "session", "sessionid",
		"privatekey", "sshkey", "ssn":
		return true
	}
	if strings.Contains(k, "apikey") || strings.Contains(k, "token") ||
		strings.Contains(k, "authorization") || strings.Contains(k, "secret") ||
		strings.Contains(k, "privatekey") || strings.Contains(k, "sshkey") ||
		strings.Contains(k, "password") || strings.Contains(k, "credential") {
		return true
	}
	return false
}

// Manager handles privacy controls and data protection
type Manager struct {
	policies             map[string]*memcore.PrivacyPolicyConfig
	compiledPatterns     map[string][]*regexp.Regexp
	mu                   sync.RWMutex
	consecutiveErrors    int
	maxConsecutiveErrors int
}

// NewManager creates a new privacy manager
func NewManager() *Manager {
	return &Manager{
		policies:             make(map[string]*memcore.PrivacyPolicyConfig),
		compiledPatterns:     make(map[string][]*regexp.Regexp),
		maxConsecutiveErrors: 10,
	}
}

// RegisterPolicy registers a privacy policy for a memory resource
func (pm *Manager) RegisterPolicy(_ context.Context, resourceID string, policy *memcore.PrivacyPolicyConfig) error {
	if resourceID == "" {
		return memcore.NewMemoryError(
			memcore.ErrCodePrivacyPolicy,
			"resource ID cannot be empty",
			nil,
		)
	}
	if policy == nil {
		return nil
	}
	pm.mu.Lock()
	defer pm.mu.Unlock()
	var compiledPatterns []*regexp.Regexp
	for _, pattern := range policy.RedactPatterns {
		if err := validateRedactionPattern(pattern); err != nil {
			return memcore.NewMemoryError(
				memcore.ErrCodePrivacyValidation,
				fmt.Sprintf("unsafe regex pattern: %s", pattern),
				err,
			).WithContext("pattern", pattern)
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return memcore.NewMemoryError(
				memcore.ErrCodePrivacyPolicy,
				fmt.Sprintf("invalid regex pattern: %s", pattern),
				err,
			).WithContext("pattern", pattern)
		}
		compiledPatterns = append(compiledPatterns, re)
	}
	pm.policies[resourceID] = policy
	pm.compiledPatterns[resourceID] = compiledPatterns
	return nil
}

// GetPolicy retrieves the privacy policy for a resource
func (pm *Manager) GetPolicy(_ context.Context, resourceID string) (*memcore.PrivacyPolicyConfig, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	policy, exists := pm.policies[resourceID]
	return policy, exists
}

// ApplyPrivacyControls applies privacy controls to a message
func (pm *Manager) ApplyPrivacyControls(
	ctx context.Context,
	msg llm.Message,
	resourceID string,
	metadata memcore.PrivacyMetadata,
) (llm.Message, memcore.PrivacyMetadata, error) {
	if !pm.ShouldPersistMessage(string(msg.Role), pm.getNonPersistableTypes(resourceID)) {
		metadata.DoNotPersist = true
		return msg, metadata, nil
	}
	if !metadata.RedactionApplied {
		// NOTE: Apply redaction only once to avoid double-scrubbing sensitive content.
		redactedMsg, err := pm.redactMessage(ctx, resourceID, msg)
		if err != nil {
			return msg, metadata, err
		}
		metadata.RedactionApplied = true
		return redactedMsg, metadata, nil
	}
	return msg, metadata, nil
}

// RedactContent applies redaction patterns to content
func (pm *Manager) RedactContent(
	_ context.Context,
	content string,
	patterns []string,
	defaultRedaction string,
) (string, error) {
	if defaultRedaction == "" {
		defaultRedaction = DefaultRedactionString
	}
	var compiledPatterns []*regexp.Regexp
	for _, pattern := range patterns {
		if err := validateRedactionPattern(pattern); err != nil {
			return content, err
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return content, err
		}
		compiledPatterns = append(compiledPatterns, re)
	}
	return pm.applyRedactionPatterns(content, compiledPatterns, defaultRedaction)
}

// ShouldPersistMessage determines if a message should be persisted based on privacy rules
func (pm *Manager) ShouldPersistMessage(msgType string, nonPersistableTypes []string) bool {
	for _, nonPersistableType := range nonPersistableTypes {
		if strings.EqualFold(msgType, nonPersistableType) {
			return false
		}
	}
	return true
}

// getNonPersistableTypes gets the non-persistable message types for a resource
func (pm *Manager) getNonPersistableTypes(resourceID string) []string {
	pm.mu.RLock()
	policy, exists := pm.policies[resourceID]
	pm.mu.RUnlock()
	if !exists || policy == nil {
		return nil
	}
	return policy.NonPersistableMessageTypes
}

// redactMessage applies privacy redaction to a message
func (pm *Manager) redactMessage(ctx context.Context, resourceID string, msg llm.Message) (llm.Message, error) {
	policy, patterns := pm.getPolicyAndPatterns(resourceID)
	if policy == nil {
		return msg, nil
	}
	if err := pm.checkCircuitBreaker(); err != nil {
		return msg, err
	}
	var redactionErr error
	defer pm.handleRedactionResult(ctx, resourceID, &redactionErr)
	redactionString := pm.getRedactionString(policy)
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

// getPolicyAndPatterns retrieves policy and compiled patterns for a resource
func (pm *Manager) getPolicyAndPatterns(resourceID string) (*memcore.PrivacyPolicyConfig, []*regexp.Regexp) {
	pm.mu.RLock()
	policy, exists := pm.policies[resourceID]
	patterns := pm.compiledPatterns[resourceID]
	pm.mu.RUnlock()
	if !exists {
		return nil, nil
	}
	return policy, patterns
}

// checkCircuitBreaker checks if the circuit breaker is open
func (pm *Manager) checkCircuitBreaker() error {
	pm.mu.RLock()
	currentErrors := pm.consecutiveErrors
	pm.mu.RUnlock()
	if currentErrors >= pm.maxConsecutiveErrors {
		return memcore.NewMemoryError(
			memcore.ErrCodePrivacyRedaction,
			"privacy redaction circuit breaker open",
			nil,
		).WithContext("consecutive_errors", currentErrors)
	}
	return nil
}

// handleRedactionResult handles panic recovery and error tracking
func (pm *Manager) handleRedactionResult(ctx context.Context, resourceID string, redactionErr *error) {
	if r := recover(); r != nil {
		*redactionErr = memcore.NewMemoryError(
			memcore.ErrCodePrivacyRedaction,
			fmt.Sprintf("panic during redaction: %v", r),
			nil,
		).WithContext("panic", r).WithContext("resource_id", resourceID)
	}
	pm.updateErrorCounter(ctx, resourceID, *redactionErr)
}

// updateErrorCounter updates the consecutive error counter and logs errors
func (pm *Manager) updateErrorCounter(ctx context.Context, resourceID string, redactionErr error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if redactionErr != nil {
		pm.consecutiveErrors++
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
}

// getRedactionString gets the redaction string from policy with fallback
func (pm *Manager) getRedactionString(policy *memcore.PrivacyPolicyConfig) string {
	redactionString := policy.DefaultRedactionString
	if redactionString == "" {
		redactionString = DefaultRedactionString
	}
	return redactionString
}

// applyRedactionPatterns applies redaction patterns to content with timeout protection
func (pm *Manager) applyRedactionPatterns(
	content string,
	patterns []*regexp.Regexp,
	redactionString string,
) (string, error) {
	redactedContent := content
	for i, pattern := range patterns {
		if pattern == nil {
			return "", pm.handleNilPattern()
		}
		var err error
		redactedContent, err = pm.applyPatternWithTimeout(redactedContent, pattern, redactionString, i, content)
		if err != nil {
			return "", err
		}
	}
	return redactedContent, nil
}

// handleNilPattern handles nil pattern error
func (pm *Manager) handleNilPattern() error {
	err := memcore.NewMemoryError(
		memcore.ErrCodePrivacyRedaction,
		"nil pattern encountered",
		nil,
	)
	pm.incrementErrorCounter()
	return err
}

// applyPatternWithTimeout applies a single pattern with timeout protection
func (pm *Manager) applyPatternWithTimeout(
	content string,
	pattern *regexp.Regexp,
	redactionString string,
	patternIndex int,
	originalContent string,
) (string, error) {
	result := make(chan string, 1)
	errChan := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				errChan <- fmt.Errorf("regex panic: %v", r)
			}
		}()
		result <- pattern.ReplaceAllString(content, redactionString)
	}()
	return pm.waitForPatternResult(result, errChan, originalContent, patternIndex)
}

// waitForPatternResult waits for pattern result with timeout
func (pm *Manager) waitForPatternResult(
	result chan string,
	errChan chan error,
	originalContent string,
	patternIndex int,
) (string, error) {
	select {
	case redactedContent := <-result:
		return redactedContent, nil
	case err := <-errChan:
		pm.incrementErrorCounter()
		return "", memcore.NewMemoryError(memcore.ErrCodePrivacyRedaction, "regex error", err)
	case <-time.After(50 * time.Millisecond):
		pm.incrementErrorCounter()
		return originalContent, memcore.NewMemoryError(
			memcore.ErrCodePrivacyRedaction,
			"regex pattern timed out",
			nil,
		).WithContext("pattern_index", patternIndex)
	}
}

// incrementErrorCounter increments the consecutive error counter
func (pm *Manager) incrementErrorCounter() {
	pm.mu.Lock()
	pm.consecutiveErrors++
	pm.mu.Unlock()
}

// LogPrivacyExclusion logs when sensitive data is excluded from persistence
func (pm *Manager) LogPrivacyExclusion(
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
	for k, v := range metadata {
		normalized := normalizeMetaKey(k)
		if isSensitiveMetaKey(normalized) {
			logData["meta."+k] = DefaultRedactionString
			continue
		}
		if _, reserved := logData[k]; reserved {
			logData["meta."+k] = v
			continue
		}
		logData[k] = v
	}
	log.Info("Privacy exclusion applied", logData)
}

// GetCircuitBreakerStatus returns the current circuit breaker status
func (pm *Manager) GetCircuitBreakerStatus() (isOpen bool, consecutiveErrors int, maxErrors int) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.consecutiveErrors >= pm.maxConsecutiveErrors, pm.consecutiveErrors, pm.maxConsecutiveErrors
}

// ResetCircuitBreaker resets the circuit breaker error counter
func (pm *Manager) ResetCircuitBreaker() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.consecutiveErrors = 0
}

// validateRedactionPattern checks if a regex pattern is safe from ReDoS attacks
func validateRedactionPattern(pattern string) error {
	if err := checkExactPatterns(pattern); err != nil {
		return err
	}
	return checkRegexPatterns(pattern)
}

// checkExactPatterns checks for exact problematic patterns
func checkExactPatterns(pattern string) error {
	exactPatterns := []struct {
		pattern string
		reason  string
	}{
		{`(\w+)+`, "nested quantifiers with word characters"},
		{`(a+)+`, "nested quantifiers"},
		{`(a*)*`, "nested star quantifiers"},
		{`(a|a)*`, "alternation with star quantifier"},
		{`(a|b)*`, "alternation with star quantifier"},
		{`(.+)+`, "nested quantifiers with any character"},
		{`(\S+)+`, "nested quantifiers with non-whitespace"},
		{`([^,]+)+`, "nested quantifiers with negated character class"},
		{`(.*)*`, "nested star quantifiers with any character"},
		{`(\d+)+`, "nested quantifiers with digits"},
		{`(.{0,50000})*`, "memory exhaustion"},
		{`a{999999}`, "excessive repetition"},
		{`([a-zA-Z]+)*`, "nested star quantifiers"},
		{`(.*a){10,}`, "excessive repetition"},
		{`(a|b)*abb`, "alternation with star quantifier"},
		{`^(a+)+$`, "nested quantifiers"},
		{`(x+x+)+y`, "nested quantifiers"},
	}
	for _, prob := range exactPatterns {
		if strings.Contains(pattern, prob.pattern) {
			return fmt.Errorf("pattern contains %s which can cause catastrophic backtracking", prob.reason)
		}
	}
	return nil
}

// checkRegexPatterns checks for regex-based problematic patterns
func checkRegexPatterns(pattern string) error {
	problematicPatterns := []struct {
		detector *regexp.Regexp
		reason   string
	}{
		{regexp.MustCompile(`\([^)]*\+[^)]*\)\+`), "nested quantifiers"},
		{regexp.MustCompile(`\([^)]*\*[^)]*\)\*`), "nested star quantifiers"},
		{regexp.MustCompile(`\([^)]*\|[^)]*\)\*`), "alternation with star quantifier"},
		{regexp.MustCompile(`\([^)]*\{\d+,\}`), "excessive repetition"},
		{regexp.MustCompile(`\.\{\d{4,}\}`), "memory exhaustion"},
		{regexp.MustCompile(`[^\\][+*]\{9{5,}\}`), "excessive repetition"},
		{regexp.MustCompile(`\([^)]+\)\*`), "nested star quantifiers"},
		{regexp.MustCompile(`\([^)]+\)\+`), "nested quantifiers"},
		{regexp.MustCompile(`\.\*.*\)\{\d+,\}`), "excessive repetition"},
	}
	for _, prob := range problematicPatterns {
		if prob.detector.MatchString(pattern) {
			return fmt.Errorf("pattern contains %s which can cause catastrophic backtracking", prob.reason)
		}
	}
	return nil
}
