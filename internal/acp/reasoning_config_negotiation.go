package acp

import (
	"context"
	"errors"
	"strings"
	"time"
)

// applySessionReasoningEffort waits for a late config_option_update when requested
// and rejects unadvertised intent before requesting provider acknowledgement.
func (d *Driver) applySessionReasoningEffort(
	ctx context.Context,
	process *AgentProcess,
	effort string,
	waitForCatalog bool,
) (bool, error) {
	if ctx == nil || process == nil || process.conn == nil {
		return false, nil
	}
	effortID := strings.TrimSpace(effort)
	if effortID == "" {
		return false, nil
	}

	option, err := d.awaitSessionReasoningOption(ctx, process, effortID, waitForCatalog)
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(option.CurrentValueID) == effortID {
		return false, nil
	}
	if err := d.applySessionConfigOption(ctx, process, SessionConfigOptionSelection{
		ID:      option.ID,
		ValueID: effortID,
	}); err != nil {
		if errors.Is(err, errConfigOptionUnconfirmed) {
			// The value is advertised but the provider never confirmed it: report the
			// cause without claiming the value is absent from the choices.
			return true, newNegotiationError(
				NegotiationCodeReasoningEffortUnsupported,
				"reasoning effort",
				effortID,
				option.ID,
				nil,
				err,
			)
		}
		return true, newNegotiationError(
			NegotiationCodeReasoningEffortUnsupported,
			"reasoning effort",
			effortID,
			option.ID,
			configOptionChoices(option),
			err,
		)
	}
	return true, nil
}

// awaitSessionReasoningOption reads the reasoning option from the current snapshot,
// optionally waiting up to configSettleBudget for a late config_option_update after
// session startup or a model switch.
func (d *Driver) awaitSessionReasoningOption(
	ctx context.Context,
	process *AgentProcess,
	effortID string,
	waitForCatalog bool,
) (SessionConfigOption, error) {
	options := process.CapsSnapshot().ConfigOptions
	option, ok := findReasoningConfigOption(options)
	if ok && configOptionAllowsValue(option, effortID) {
		return option, nil
	}
	if waitForCatalog && (!ok || !option.ReadOnly) {
		budget := defaultConfigSettleBudget
		if d != nil && d.configSettleBudget > 0 {
			budget = d.configSettleBudget
		}
		deadline := time.Now().Add(budget)
		for {
			remaining := time.Until(deadline)
			if remaining <= 0 {
				break
			}
			since, _ := process.configRevisionAndChange()
			options = process.CapsSnapshot().ConfigOptions
			if announced, found := findReasoningConfigOption(options); found &&
				configOptionAllowsValue(announced, effortID) {
				return announced, nil
			}
			if err := waitForConfigRevision(ctx, process, since, remaining); err != nil {
				if errors.Is(err, errConfigSettleTimeout) {
					break
				}
				return SessionConfigOption{}, err
			}
		}
	}

	options = process.CapsSnapshot().ConfigOptions
	option, ok = findReasoningConfigOption(options)
	if !ok {
		return SessionConfigOption{}, newNegotiationError(
			NegotiationCodeReasoningOptionMissing,
			"reasoning effort",
			effortID,
			"",
			nil,
			nil,
		)
	}
	if !configOptionAllowsValue(option, effortID) {
		return SessionConfigOption{}, newNegotiationError(
			NegotiationCodeReasoningEffortUnsupported,
			"reasoning effort",
			effortID,
			option.ID,
			configOptionChoices(option),
			nil,
		)
	}
	return option, nil
}
