package acp

import (
	"fmt"
	"sort"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/diagnosticcontract"
	"github.com/compozy/agh/internal/diagnostics"
)

const (
	NegotiationCodeModelUnavailable           = diagnosticcontract.CodeModelUnavailable
	NegotiationCodeReasoningOptionMissing     = diagnosticcontract.CodeReasoningOptionMissing
	NegotiationCodeReasoningEffortUnsupported = diagnosticcontract.CodeReasoningEffortUnsupported
)

// NegotiationError reports one model or reasoning request rejected before the first prompt.
type NegotiationError struct {
	Code           string
	Stage          string
	Requested      string
	ConfigOptionID string
	ValidChoices   []string
	Cause          error
}

func (e *NegotiationError) Error() string {
	if e == nil {
		return ""
	}
	message := fmt.Sprintf("acp: %s %q is unavailable", e.Stage, strings.TrimSpace(e.Requested))
	if optionID := strings.TrimSpace(e.ConfigOptionID); optionID != "" {
		message += fmt.Sprintf(" in config option %q", optionID)
	}
	if len(e.ValidChoices) > 0 {
		message += fmt.Sprintf("; valid choices: %s", strings.Join(e.ValidChoices, ", "))
	}
	if e.Cause != nil {
		message += ": " + e.Cause.Error()
	}
	return message
}

func (e *NegotiationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func newNegotiationError(
	code string,
	stage string,
	requested string,
	optionID string,
	validChoices []string,
	cause error,
) error {
	negotiationErr := &NegotiationError{
		Code:           strings.TrimSpace(code),
		Stage:          strings.TrimSpace(stage),
		Requested:      strings.TrimSpace(requested),
		ConfigOptionID: strings.TrimSpace(optionID),
		ValidChoices:   normalizedChoices(validChoices),
		Cause:          cause,
	}
	evidence := map[string]any{
		"stage":     negotiationErr.Stage,
		"requested": negotiationErr.Requested,
	}
	if negotiationErr.ConfigOptionID != "" {
		evidence["config_option_id"] = negotiationErr.ConfigOptionID
	}
	if len(negotiationErr.ValidChoices) > 0 {
		evidence["valid_choices"] = append([]string(nil), negotiationErr.ValidChoices...)
	}
	return diagnostics.NewStructuredError(
		diagnostics.NewItem(
			"provider.negotiation."+negotiationErr.Code,
			negotiationErr.Code,
			diagnosticcontract.CategoryProvider,
			"Provider configuration is unavailable",
			negotiationErr.Error(),
			diagnosticcontract.SeverityError,
			diagnosticcontract.FreshnessLive,
			diagnostics.WithEvidence(evidence),
		),
		negotiationErr,
	)
}

func normalizedChoices(values []string) []string {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			set[trimmed] = struct{}{}
		}
	}
	choices := make([]string, 0, len(set))
	for value := range set {
		choices = append(choices, value)
	}
	sort.Strings(choices)
	return choices
}

func configOptionChoices(option SessionConfigOption) []string {
	choices := make([]string, 0, len(option.Values))
	for _, value := range option.Values {
		choices = append(choices, value.Value)
	}
	return normalizedChoices(choices)
}

func validateReasoningApplication(opts StartOpts) error {
	effort := strings.TrimSpace(opts.ReasoningEffort)
	if effort == "" {
		return nil
	}
	strategy := aghconfig.ReasoningApplyNone
	if opts.ProviderConfig != nil {
		strategy = opts.ProviderConfig.Models.EffectiveReasoningApply()
	}
	if strategy == aghconfig.ReasoningApplyACPOption {
		return nil
	}
	return newNegotiationError(
		NegotiationCodeReasoningOptionMissing,
		"reasoning effort",
		effort,
		"",
		nil,
		fmt.Errorf("provider reasoning apply strategy is %q", strategy),
	)
}
