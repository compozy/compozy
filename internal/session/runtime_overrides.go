package session

import (
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/diagnosticcontract"
	"github.com/compozy/agh/internal/diagnostics"
	"github.com/compozy/agh/internal/modelcatalog"
)

// ValidateReasoningEffort validates one reasoning effort override.
func ValidateReasoningEffort(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || modelcatalog.IsValidEffort(trimmed) {
		return nil
	}
	validChoices := modelcatalog.ReasoningEffortValues()
	cause := fmt.Errorf(
		"%w: reasoning_effort must be one of %s",
		ErrInvalidRuntimeOverride,
		strings.Join(validChoices, ", "),
	)
	item := diagnostics.NewItem(
		"provider.negotiation."+diagnosticcontract.CodeReasoningEffortUnsupported,
		diagnosticcontract.CodeReasoningEffortUnsupported,
		diagnosticcontract.CategoryProvider,
		"Reasoning effort is unsupported",
		cause.Error(),
		diagnosticcontract.SeverityError,
		diagnosticcontract.FreshnessLive,
		diagnostics.WithEvidence(map[string]any{
			"requested":     trimmed,
			"valid_choices": validChoices,
		}),
	)
	return diagnostics.NewStructuredError(item, cause)
}
