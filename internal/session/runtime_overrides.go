package session

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/diagnosticcontract"
	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/modelcatalog"
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
	item := diagnostics.NewItem(diagnostics.ItemSpec{
		ID:            "provider.negotiation." + diagnosticcontract.CodeReasoningEffortUnsupported,
		Code:          diagnosticcontract.CodeReasoningEffortUnsupported,
		Category:      diagnosticcontract.CategoryProvider,
		Title:         "Reasoning effort is unsupported",
		Message:       cause.Error(),
		Severity:      diagnosticcontract.SeverityError,
		DataFreshness: diagnosticcontract.FreshnessLive,
	},
		diagnostics.WithEvidence(map[string]any{
			"requested":     trimmed,
			"valid_choices": validChoices,
		}),
	)
	return diagnostics.NewStructuredError(item, cause)
}
