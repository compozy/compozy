package core

import "fmt"

// Common error codes used throughout the memory package
const (
	// Memory operation errors
	ErrCodeMemoryAppend   = "MEMORY_APPEND_ERROR"
	ErrCodeMemoryRead     = "MEMORY_READ_ERROR"
	ErrCodeMemoryClear    = "MEMORY_CLEAR_ERROR"
	ErrCodeMemoryNotFound = "MEMORY_NOT_FOUND"
	ErrCodeMemoryExpired  = "MEMORY_EXPIRED"

	// Store operation errors
	ErrCodeStoreOperation = "STORE_OPERATION_ERROR"
	ErrCodeStoreTimeout   = "STORE_TIMEOUT"
	ErrCodeStoreLocked    = "STORE_LOCKED"

	// Token counting errors
	ErrCodeTokenCounting = "TOKEN_COUNTING_ERROR"
	ErrCodeTokenLimit    = "TOKEN_LIMIT_EXCEEDED" //nolint:gosec // This is not a credential, it's an error code

	// Configuration errors
	ErrCodeInvalidConfig = "INVALID_CONFIGURATION"
	ErrCodeMissingConfig = "MISSING_CONFIGURATION"

	// Flushing errors
	ErrCodeFlushFailed  = "FLUSH_FAILED"
	ErrCodeFlushPending = "FLUSH_ALREADY_PENDING"
	ErrCodeFlushTimeout = "FLUSH_TIMEOUT"

	// Lock errors
	ErrCodeLockAcquisition = "LOCK_ACQUISITION_FAILED"
	ErrCodeLockTimeout     = "LOCK_TIMEOUT"

	// Privacy errors (re-exported from privacy package)
	ErrCodePrivacyRedaction  = "PRIVACY_REDACTION_ERROR"
	ErrCodePrivacyPolicy     = "PRIVACY_POLICY_ERROR"
	ErrCodePrivacyValidation = "PRIVACY_VALIDATION_ERROR"
)

// Common errors
var (
	// ErrMemoryNotFound is returned when a memory instance is not found
	ErrMemoryNotFound = fmt.Errorf("memory instance not found")

	// ErrMemoryExpired is returned when a memory instance has expired
	ErrMemoryExpired = fmt.Errorf("memory instance has expired")

	// ErrTokenLimitExceeded is returned when the token limit is exceeded
	ErrTokenLimitExceeded = fmt.Errorf("token limit exceeded")

	// ErrMessageLimitExceeded is returned when the message limit is exceeded
	ErrMessageLimitExceeded = fmt.Errorf("message limit exceeded")

	// ErrFlushAlreadyPending is returned when a flush is already in progress
	ErrFlushAlreadyPending = fmt.Errorf("flush operation already pending")

	// ErrLockTimeout is returned when lock acquisition times out
	ErrLockTimeout = fmt.Errorf("lock acquisition timeout")

	// ErrInvalidConfiguration is returned when configuration is invalid
	ErrInvalidConfiguration = fmt.Errorf("invalid configuration")
)

// MemoryError represents a memory-specific error with additional context
type MemoryError struct {
	Code    string
	Message string
	Cause   error
	Context map[string]any
}

// Error implements the error interface
func (e *MemoryError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap returns the underlying error
func (e *MemoryError) Unwrap() error {
	return e.Cause
}

// NewMemoryError creates a new MemoryError
func NewMemoryError(code, message string, cause error) *MemoryError {
	return &MemoryError{
		Code:    code,
		Message: message,
		Cause:   cause,
		Context: make(map[string]any),
	}
}

// WithContext adds context to the error
func (e *MemoryError) WithContext(key string, value any) *MemoryError {
	e.Context[key] = value
	return e
}

// ConfigError represents a configuration-related error
type ConfigError struct {
	*MemoryError
}

// NewConfigError creates a new ConfigError
func NewConfigError(message string, cause error) *ConfigError {
	return &ConfigError{
		MemoryError: NewMemoryError(ErrCodeInvalidConfig, message, cause),
	}
}

// LockError represents a lock-related error
type LockError struct {
	*MemoryError
}

// NewLockError creates a new LockError
func NewLockError(message string, cause error) *LockError {
	return &LockError{
		MemoryError: NewMemoryError(ErrCodeLockAcquisition, message, cause),
	}
}
