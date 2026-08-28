package cli

import (
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/spf13/cobra"
)

type sessionPromptRuntimeFlags struct {
	provider        string
	model           string
	reasoningEffort string
	speed           string
	acpOptions      []string
	acpToggles      []string
}

func (f *sessionPromptRuntimeFlags) selection(
	cmd *cobra.Command,
) (*contract.PromptRuntimeSelectionPayload, error) {
	if cmd == nil {
		return nil, nil
	}
	flags := cmd.Flags()
	if !flags.Changed(sessionProviderKey) && !flags.Changed("model") &&
		!flags.Changed("reasoning-effort") && !flags.Changed("speed") &&
		!flags.Changed(runtimeACPOptionFlag) && !flags.Changed(runtimeACPToggleFlag) {
		return nil, nil
	}
	acpOptions, err := parseACPFlags(f.acpOptions, f.acpToggles)
	if err != nil {
		return nil, err
	}
	selection := &contract.PromptRuntimeSelectionPayload{
		Provider:        strings.TrimSpace(f.provider),
		Model:           strings.TrimSpace(f.model),
		ReasoningEffort: contract.ReasoningEffort(strings.TrimSpace(f.reasoningEffort)),
		ACPOptions:      acpOptions,
	}
	if flags.Changed("speed") {
		parsedSpeed, err := speedpkg.Parse(f.speed)
		if err != nil {
			return nil, err
		}
		selection.Speed = parsedSpeed
	}
	return selection, nil
}

func bindSessionPromptRuntimeFlags(cmd *cobra.Command, flags *sessionPromptRuntimeFlags) {
	cmd.Flags().StringVar(&flags.provider, sessionProviderKey, "", "Runtime provider for this prompt")
	cmd.Flags().StringVar(&flags.model, "model", "", "Runtime model for this prompt")
	cmd.Flags().StringVar(
		&flags.reasoningEffort,
		"reasoning-effort",
		"",
		"Runtime reasoning effort for this prompt (none|minimal|low|medium|high|xhigh|max)",
	)
	cmd.Flags().StringVar(&flags.speed, "speed", "", "Runtime speed for this prompt (normal|fast)")
	bindACPOptionFlags(
		cmd,
		&flags.acpOptions,
		&flags.acpToggles,
		"ACP select override as id=value (repeatable)",
		"ACP boolean override as id=true|false (repeatable)",
	)
}
