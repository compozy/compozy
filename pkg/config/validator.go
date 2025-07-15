package config

import (
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
)

// workflowIDPattern matches valid workflow ID formats:
// - UUIDs (with or without hyphens)
// - Alphanumeric with hyphens and underscores
// - Minimum 3 characters, maximum 100 characters
var workflowIDPattern = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9_-]*[a-zA-Z0-9])?$`)

// RegisterCustomValidators registers custom validation functions
func RegisterCustomValidators(v *validator.Validate) error {
	return v.RegisterValidation("workflow_id", validateWorkflowID)
}

// validateWorkflowID validates workflow ID format
func validateWorkflowID(fl validator.FieldLevel) bool {
	workflowID := fl.Field().String()

	// Empty string is not allowed
	if workflowID == "" {
		return false
	}

	// Check length constraints
	if len(workflowID) < 3 || len(workflowID) > 100 {
		return false
	}

	// Check for invalid characters or patterns
	if strings.Contains(workflowID, "--") || strings.Contains(workflowID, "__") {
		return false
	}

	// Must not start or end with hyphen or underscore
	if strings.HasPrefix(workflowID, "-") || strings.HasPrefix(workflowID, "_") ||
		strings.HasSuffix(workflowID, "-") || strings.HasSuffix(workflowID, "_") {
		return false
	}

	// Match the pattern
	return workflowIDPattern.MatchString(workflowID)
}
