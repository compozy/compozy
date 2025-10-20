package helpers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/compozy/compozy/cli/tui/models"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/spf13/cobra"
)

// CliError represents a CLI-specific error with enhanced context
type CliError struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Details   string         `json:"details,omitempty"`
	Context   map[string]any `json:"context,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

func (e *CliError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// NewCliError creates a new CLI error with context
func NewCliError(code, message string, details ...string) *CliError {
	err := &CliError{
		Code:      code,
		Message:   message,
		Timestamp: time.Now(),
		Context:   make(map[string]any),
	}
	if len(details) > 0 {
		err.Details = details[0]
	}
	return err
}

// WithContext adds context to the error
func (e *CliError) WithContext(key string, value any) *CliError {
	if e.Context == nil {
		e.Context = make(map[string]any)
	}
	e.Context[key] = value
	return e
}

// IsTimeoutError checks if an error is a timeout error
func IsTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	// Check using errors.Is for proper error types
	return errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, ErrTimeout) ||
		// Fallback to string matching for compatibility
		strings.Contains(strings.ToLower(err.Error()), "timeout") ||
		strings.Contains(strings.ToLower(err.Error()), "timed out")
}

// IsNetworkError checks if an error is a network-related error
func IsNetworkError(err error) bool {
	if err == nil {
		return false
	}
	// Check using errors.Is for proper error types
	if errors.Is(err, ErrNetwork) {
		return true
	}
	// Fallback to string matching for compatibility
	errStr := strings.ToLower(err.Error())
	networkKeywords := []string{
		"connection refused", "connection reset", "connection timeout",
		"no route to host", "network unreachable", "dns",
		"name resolution failed", "temporary failure",
	}
	for _, keyword := range networkKeywords {
		if strings.Contains(errStr, keyword) {
			return true
		}
	}
	return false
}

// IsAuthError checks if an error is authentication-related
func IsAuthError(err error) bool {
	if err == nil {
		return false
	}
	// Check using errors.Is for proper error types
	if errors.Is(err, ErrAuth) {
		return true
	}
	// Fallback to string matching for compatibility
	errStr := strings.ToLower(err.Error())
	authKeywords := []string{
		"unauthorized", "authentication", "invalid token",
		"permission denied", "forbidden", "access denied",
		"api key", "credential",
	}
	for _, keyword := range authKeywords {
		if strings.Contains(errStr, keyword) {
			return true
		}
	}
	return false
}

// FormatError formats errors based on output mode
func FormatError(err error, mode models.Mode) string {
	if err == nil {
		return ""
	}
	switch mode {
	case models.ModeJSON:
		return formatErrorJSON(err)
	case models.ModeTUI:
		return formatErrorTUI(err)
	default:
		return err.Error()
	}
}

// formatErrorJSON formats errors for JSON output according to API standards
func formatErrorJSON(err error) string {
	var errorResponse map[string]any
	if cliErr, ok := err.(*CliError); ok {
		errorResponse = map[string]any{
			"error":   cliErr.Message,
			"details": cliErr.Details,
		}
	} else {
		errorResponse = map[string]any{
			"error":   err.Error(),
			"details": "",
		}
	}
	jsonBytes, err := json.MarshalIndent(errorResponse, "", "  ")
	if err != nil {
		// Fallback to simple error message if JSON marshaling fails
		fallbackResponse := map[string]any{
			"error":   "JSON marshaling failed",
			"details": "",
		}
		if fallbackBytes, fallbackErr := json.Marshal(fallbackResponse); fallbackErr == nil {
			return string(fallbackBytes)
		}
		// Last resort - return minimal JSON with escaped message
		return `{"error": "JSON marshaling failed", "details": ""}`
	}
	return string(jsonBytes)
}

// formatErrorTUI formats errors for TUI output with colors and icons
func formatErrorTUI(err error) string {
	message, details := extractErrorInfo(err)
	icon := getErrorIcon(err)
	result := formatErrorMessage(icon, message)
	if details != "" {
		result += formatErrorDetails(details)
	}
	return result
}

// extractErrorInfo extracts message and details from error
func extractErrorInfo(err error) (message, details string) {
	if cliErr, ok := err.(*CliError); ok && cliErr != nil {
		return cliErr.Message, cliErr.Details
	}
	return err.Error(), ""
}

// getErrorIcon returns appropriate icon based on error type
func getErrorIcon(err error) string {
	switch {
	case IsNetworkError(err):
		return "🌐"
	case IsAuthError(err):
		return "🔐"
	case IsTimeoutError(err):
		return "⏰"
	default:
		return "❌"
	}
}

// formatErrorMessage formats the main error message with icon
func formatErrorMessage(icon, message string) string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF6B6B")).
		Bold(true)
	return fmt.Sprintf("%s %s", icon, style.Render(message))
}

// formatErrorDetails formats error details
func formatErrorDetails(details string) string {
	detailStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888")).
		Italic(true)
	return "\n" + detailStyle.Render(fmt.Sprintf("Details: %s", details))
}

// OutputError outputs an error to stderr in the appropriate format
func OutputError(err error, mode models.Mode) {
	if err == nil {
		return
	}
	formattedError := FormatError(err, mode)
	fmt.Fprintln(os.Stderr, formattedError)
}

// ValidateID validates that a string is a valid core.ID (UUID)
func ValidateID(id string) error {
	if id == "" {
		return NewCliError("INVALID_ID", "ID cannot be empty")
	}
	// Basic UUID validation pattern
	uuidPattern := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	if !uuidPattern.MatchString(id) {
		return NewCliError("INVALID_ID", "ID must be a valid UUID", fmt.Sprintf("provided: %s", id))
	}
	return nil
}

// ValidateIDOrEmpty validates that a string is either empty or a valid core.ID
func ValidateIDOrEmpty(id string) error {
	if id == "" {
		return nil
	}
	return ValidateID(id)
}

// ParseID parses a string into a core.ID with validation
func ParseID(id string) (core.ID, error) {
	if err := ValidateID(id); err != nil {
		return "", err
	}
	return core.ID(id), nil
}

// ValidateRequired validates that a required string value is not empty
func ValidateRequired(value, fieldName string) error {
	if strings.TrimSpace(value) == "" {
		return NewCliError("REQUIRED_FIELD", fmt.Sprintf("%s is required", fieldName))
	}
	return nil
}

// ValidateEnum validates that a value is in a set of allowed values
func ValidateEnum(value string, allowed []string, fieldName string) error {
	if value == "" {
		return nil // Allow empty values, use ValidateRequired separately if needed
	}
	if slices.Contains(allowed, value) {
		return nil
	}
	return NewCliError("INVALID_ENUM",
		fmt.Sprintf("%s must be one of: %s", fieldName, strings.Join(allowed, ", ")),
		fmt.Sprintf("provided: %s", value))
}

// Contains reports whether substr is within s using a case-insensitive comparison.
// An empty substr returns true.
func Contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// ContainsAny returns true if s contains any of the provided substrings.
// ContainsAny reports whether s contains any of the provided substrings.
// The comparison is case-insensitive; empty substrings are ignored.
func ContainsAny(s string, substrings ...string) bool {
	lower := strings.ToLower(s)
	for _, sub := range substrings {
		if sub == "" {
			continue
		}
		if strings.Contains(lower, strings.ToLower(sub)) {
			return true
		}
	}
	return false
}

// Truncate returns s truncated to at most maxLength characters.
// If s is longer than maxLength and maxLength > 3, the result ends with "..." and has length maxLength.
// If maxLength <= 3 the function truncates to maxLength characters without adding an ellipsis.
// If s is shorter than or equal to maxLength, it is returned unchanged.
func Truncate(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}
	if maxLength <= 3 {
		return s[:maxLength]
	}
	return s[:maxLength-3] + "..."
}

// GetFlagStringWithDefault gets a string flag with a default value
func GetFlagStringWithDefault(cmd *cobra.Command, flagName, defaultValue string) string {
	if value, err := cmd.Flags().GetString(flagName); err == nil && value != "" {
		return value
	}
	return defaultValue
}

// GetFlagBoolWithDefault gets a boolean flag with a default value
func GetFlagBoolWithDefault(cmd *cobra.Command, flagName string, defaultValue bool) bool {
	if value, err := cmd.Flags().GetBool(flagName); err == nil {
		return value
	}
	return defaultValue
}

// GetFlagIntWithDefault gets an integer flag with a default value
func GetFlagIntWithDefault(cmd *cobra.Command, flagName string, defaultValue int) int {
	if value, err := cmd.Flags().GetInt(flagName); err == nil {
		return value
	}
	return defaultValue
}

// LogOperation logs the start and completion of an operation
func LogOperation(ctx context.Context, operation string, fn func() error) error {
	log := logger.FromContext(ctx)
	start := time.Now()
	log.Info("starting operation", "operation", operation)
	err := fn()
	duration := time.Since(start)
	if err != nil {
		log.Error("operation failed", "operation", operation, "duration", duration, "error", err)
	} else {
		log.Info("operation completed", "operation", operation, "duration", duration)
	}
	return err
}

// WithTimeout wraps fn with a timeout context that cancels after the given duration.
// A timeout ≤ 0 disables the timeout and calls fn immediately.
func WithTimeout(ctx context.Context, timeout time.Duration, fn func(context.Context) error) error {
	if timeout <= 0 {
		return fn(ctx)
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- fn(timeoutCtx)
	}()
	select {
	case err := <-done:
		return err
	case <-timeoutCtx.Done():
		return NewCliError("OPERATION_TIMEOUT",
			fmt.Sprintf("operation timed out after %s", timeout))
	}
}

// Pluralize returns singular or plural form based on count
func Pluralize(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}

// FormatDuration formats a duration in a human-readable way
func FormatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.1fm", d.Minutes())
	}
	return fmt.Sprintf("%.1fh", d.Hours())
}

// SanitizeForJSON sanitizes a string for safe JSON output
func SanitizeForJSON(s string) string {
	// Remove control characters except tab, newline, and carriage return
	reg := regexp.MustCompile(`[\x00-\x08\x0B\x0C\x0E-\x1F\x7F]`)
	return reg.ReplaceAllString(s, "")
}

// GetWorkingDirectory returns the current working directory with error handling
func GetWorkingDirectory() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", NewCliError("DIRECTORY_ERROR", "Failed to get current working directory", err.Error())
	}
	return cwd, nil
}

// FileExists checks if a file exists and is not a directory
func FileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// DirExists checks if a directory exists
func DirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
