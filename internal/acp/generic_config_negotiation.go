package acp

import (
	"context"
	"fmt"
	"strings"
)

func (d *Driver) applySessionConfigSelections(
	ctx context.Context,
	process *AgentProcess,
	selections []SessionConfigOptionSelection,
) error {
	for _, selection := range selections {
		options := process.CapsSnapshot().ConfigOptions
		option, ok := findConfigOptionByID(options, selection.ID)
		if !ok {
			return fmt.Errorf("acp: config option %q is not uniquely advertised", selection.ID)
		}
		if err := validateSessionConfigOptionSelection(selection, option); err != nil {
			return err
		}
		if selection.matches(option) {
			continue
		}
		if err := d.applySessionConfigOption(ctx, process, selection); err != nil {
			return fmt.Errorf("acp: apply config option %q: %w", selection.ID, err)
		}
		updated, ok := findConfigOptionByID(process.CapsSnapshot().ConfigOptions, selection.ID)
		if !ok || !selection.matches(updated) {
			return fmt.Errorf("acp: provider did not confirm config option %q", selection.ID)
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
