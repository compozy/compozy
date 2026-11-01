package cli

import (
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/config"
	"github.com/spf13/cobra"
)

func addAgentIdentityFlags(cmd *cobra.Command, f *agentDefinitionFlags) {
	cmd.Flags().StringVar(&f.displayName, "display-name", "", "Agent display name (up to 80 characters)")
	cmd.Flags().StringVar(&f.description, "description", "", "Agent purpose (up to 280 characters)")
	cmd.Flags().StringVar(&f.kind, "kind", "", "Agent kind: on_demand (default) or stays_on; fixed at creation")
	cmd.Flags().StringVar(&f.avatar, "appearance-avatar", "generated", "Avatar: generated, icon or emoji")
	cmd.Flags().StringVar(&f.seed, "appearance-seed", "", "Generated avatar seed (default: agent name)")
	cmd.Flags().IntVar(&f.generation, "appearance-generation", 1, "Avatar generation")
	cmd.Flags().StringVar(&f.color, "appearance-color", "#7c5cff", "Avatar color (#rrggbb)")
	cmd.Flags().StringVar(&f.icon, "appearance-icon", "", "Lucide icon name")
	cmd.Flags().StringVar(&f.emoji, "appearance-emoji", "", "One emoji")
}

func agentIdentityFlagsChanged(cmd *cobra.Command) bool {
	for _, name := range []string{
		"display-name",
		"description",
		"kind",
		"appearance-avatar",
		"appearance-seed",
		"appearance-generation",
		"appearance-color",
		"appearance-icon",
		"appearance-emoji",
	} {
		if cmd.Flags().Changed(name) {
			return true
		}
	}
	return false
}

func applyAgentIdentityFlags(cmd *cobra.Command, p *contract.CreateAgentPayload, f agentDefinitionFlags) {
	if cmd.Flags().Changed("display-name") {
		p.DisplayName = new(f.displayName)
	}
	if cmd.Flags().Changed("description") {
		p.Description = new(f.description)
	}
	if cmd.Flags().Changed("kind") {
		p.Kind = f.kind
	}
	changed := false
	for _, name := range []string{agentAppearanceAvatarKey, cliSeedKey, loopGenerationKey, cliColorKey, "icon", "emoji"} {
		changed = changed || cmd.Flags().Changed("appearance-"+name)
	}
	if !changed {
		return
	}
	a := config.CloneAgentAppearance(p.Appearance)
	if a == nil {
		a = map[string]any{
			"version":                1,
			agentAppearanceAvatarKey: "generated",
			cliSeedKey:               p.Name,
			loopGenerationKey:        1,
			cliColorKey:              "#7c5cff",
		}
	}
	for key, value := range map[string]any{
		agentAppearanceAvatarKey: f.avatar, cliSeedKey: f.seed, loopGenerationKey: f.generation,
		cliColorKey: f.color, "icon": f.icon, "emoji": f.emoji,
	} {
		if cmd.Flags().Changed("appearance-" + key) {
			a[key] = value
		}
	}
	if cmd.Flags().Changed("appearance-icon") {
		a[agentAppearanceAvatarKey] = "icon"
		delete(a, "emoji")
		delete(a, cliSeedKey)
		delete(a, loopGenerationKey)
	}
	if cmd.Flags().Changed("appearance-emoji") {
		a[agentAppearanceAvatarKey] = "emoji"
		delete(a, "icon")
		delete(a, cliSeedKey)
		delete(a, loopGenerationKey)
	}
	p.Appearance = a
}

const agentAppearanceAvatarKey = "avatar"
