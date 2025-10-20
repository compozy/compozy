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
	if workflowID == "" {
		return false
	}
	if len(workflowID) < 3 || len(workflowID) > 100 {
		return false
	}
	if strings.Contains(workflowID, "--") || strings.Contains(workflowID, "__") {
		return false
	}
	if strings.HasPrefix(workflowID, "-") || strings.HasPrefix(workflowID, "_") ||
		strings.HasSuffix(workflowID, "-") || strings.HasSuffix(workflowID, "_") {
		return false
	}
	return workflowIDPattern.MatchString(workflowID)
}
