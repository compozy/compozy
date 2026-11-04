package acp

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	acpsdk "github.com/coder/acp-go-sdk"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/diagnosticcontract"
	"github.com/compozy/compozy/internal/diagnostics"
)

const (
	NegotiationCodeModelUnavailable           = diagnosticcontract.CodeModelUnavailable
	NegotiationCodeReasoningOptionMissing     = diagnosticcontract.CodeReasoningOptionMissing
	NegotiationCodeReasoningEffortUnsupported = diagnosticcontract.CodeReasoningEffortUnsupported
	NegotiationCodeSpeedRejected              = diagnosticcontract.CodeSpeedRejected
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
		diagnostics.NewItem(diagnostics.ItemSpec{
			ID:            "provider.negotiation." + negotiationErr.Code,
			Code:          negotiationErr.Code,
			Category:      diagnosticcontract.CategoryProvider,
			Title:         "Provider configuration is unavailable",
			Message:       negotiationErr.Error(),
			Severity:      diagnosticcontract.SeverityError,
			DataFreshness: diagnosticcontract.FreshnessLive,
		},
			diagnostics.WithEvidence(evidence),
		),
		negotiationErr,
	)
}

// IsConfigurationRejection reports whether err is a provider rejection of a requested model,
// reasoning effort, or speed: an unadvertised value, a missing or read-only option, a provider
// invalid-params refusal, or a response that did not apply the value. Deadlines, cancellation,
// and transport failures wrapped in a NegotiationError are not configuration rejections.
func IsConfigurationRejection(err error) bool {
	negotiationErr, ok := errors.AsType[*NegotiationError](err)
	if !ok || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return false
	}
	cause := negotiationErr.Cause
	if cause == nil || errors.Is(cause, errConfigOptionUnconfirmed) ||
		errors.Is(cause, errModelConfigOptionRequired) || errors.Is(cause, errModelConfigOptionReadOnly) {
		return true
	}
	requestErr, ok := errors.AsType[*acpsdk.RequestError](cause)
	return ok && requestErr.Code == acpsdk.NewInvalidParams(nil).Code
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
	slices.Sort(choices)
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
	strategy := compozyconfig.ReasoningApplyNone
	if opts.ProviderConfig != nil {
		strategy = opts.ProviderConfig.Models.EffectiveReasoningApply()
	}
	if strategy == compozyconfig.ReasoningApplyACPOption {
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
