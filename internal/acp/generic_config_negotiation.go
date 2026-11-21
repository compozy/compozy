package acp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

// ErrSessionConfigOptionRejected reports that the provider's advertised session config options
// refused a selection, so a caller can offer reselection guidance instead of guessing.
var ErrSessionConfigOptionRejected = errors.New("acp: session config option rejected")

func (d *Driver) applyStartupConfigSelections(
	ctx context.Context,
	process *AgentProcess,
	selections []SessionConfigOptionSelection,
) error {
	// Only a derived approve-all mode may be skipped when the provider offers no
	// mode control; an authored mode the runtime does not advertise stays a failure.
	if _, ok := findConfigOptionByID(process.CapsSnapshot().ConfigOptions, sessionConfigModeKey); !ok &&
		process.permissions.mode == compozyconfig.PermissionModeApproveAll &&
		!compozyconfig.RequiresAdvertisedACPMode(process.permissionModeProvider()) {
		for _, selection := range selections {
			if selection.ID == sessionConfigModeKey &&
				compozyconfig.IsUnrestrictedACPMode(process.permissionModeProvider(), selection.ValueID) {
				process.unadvertisedStartupMode = selection.ValueID
				process.logger.DebugContext(ctx, "acp: skip unadvertised startup mode",
					"session_id", process.SessionID, "mode", selection.ValueID)
			}
		}
	}
	return d.applySessionConfigSelections(ctx, process, selections)
}

func (d *Driver) applySessionConfigSelections(
	ctx context.Context,
	process *AgentProcess,
	selections []SessionConfigOptionSelection,
) error {
	for _, selection := range selections {
		options := process.CapsSnapshot().ConfigOptions
		option, ok := findConfigOptionByID(options, selection.ID)
		if !ok {
			// Reapplying skipped startup intent must not break an unrelated runtime change.
			if process.permissions.mode == compozyconfig.PermissionModeApproveAll &&
				selection.ID == sessionConfigModeKey &&
				selection.ValueID != "" && selection.ValueID == process.unadvertisedStartupMode {
				continue
			}
			return fmt.Errorf("%w: config option %q is not uniquely advertised",
				ErrSessionConfigOptionRejected, selection.ID)
		}
		if err := validateSessionConfigOptionSelection(selection, option); err != nil {
			return fmt.Errorf("%w: %w", ErrSessionConfigOptionRejected, err)
		}
		if selection.matches(option) {
			continue
		}
		// applySessionConfigOption returns only after the provider confirmed the value,
		// including a late config_option_update.
		if err := d.applySessionConfigOption(ctx, process, selection); err != nil {
			if isConfigRejectionCause(err) {
				return fmt.Errorf("%w: apply config option %q: %w", ErrSessionConfigOptionRejected, selection.ID, err)
			}
			return fmt.Errorf("acp: apply config option %q: %w", selection.ID, err)
		}
	}
	return nil
}

func validateDedicatedConfigOptionConflicts(options []SessionConfigOption, config RuntimeConfig) error {
	reserved := make(map[string]string, 3)
	if strings.TrimSpace(config.Model) != "" {
		if option, ok := findModelConfigOption(options); ok {
			reserved[option.ID] = "model"
		}
	}
	if strings.TrimSpace(config.ReasoningEffort) != "" {
		if option, ok := findReasoningConfigOption(options); ok {
			reserved[option.ID] = "reasoning effort"
		}
	}
	if config.Speed != "" {
		match, _ := matchSpeedConfig(config.Speed, options)
		if match != nil {
			reserved[match.option.ID] = "speed"
		}
	}
	for _, selection := range config.ACPOptions {
		semantic, duplicate := reserved[selection.ID]
		if duplicate {
			return fmt.Errorf(
				"acp: config option %q duplicates the dedicated %s setting",
				selection.ID,
				semantic,
			)
		}
	}
	return nil
}
