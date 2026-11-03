package cli

import (
	"errors"

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
	cmd.Flags().StringVar(
		&f.background, "appearance-background", "", "Icon or emoji background: plate (default) or none",
	)
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
		"appearance-background",
	} {
		if cmd.Flags().Changed(name) {
			return true
		}
	}
	return false
}

// errAppearanceBackgroundNeedsSymbol keeps a plate choice from silently turning an icon or emoji
// into a generated face: the flag only applies where a symbol is drawn.
var errAppearanceBackgroundNeedsSymbol = errors.New(
	"--appearance-background applies to an icon or emoji; pass --appearance-icon or --appearance-emoji with it",
)

func applyAgentIdentityFlags(cmd *cobra.Command, p *contract.CreateAgentPayload, f agentDefinitionFlags) error {
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
	for _, name := range []string{
		agentAppearanceAvatarKey,
		cliSeedKey,
		loopGenerationKey,
		cliColorKey,
		agentAppearanceIconKey,
		agentAppearanceEmojiKey,
		agentAppearanceBackgroundKey,
	} {
		changed = changed || cmd.Flags().Changed("appearance-"+name)
	}
	if !changed {
		return nil
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
		cliColorKey: f.color, agentAppearanceIconKey: f.icon, agentAppearanceEmojiKey: f.emoji,
		agentAppearanceBackgroundKey: f.background,
	} {
		if cmd.Flags().Changed("appearance-" + key) {
			a[key] = value
		}
	}
	if cmd.Flags().Changed("appearance-icon") {
		a[agentAppearanceAvatarKey] = agentAppearanceIconKey
		delete(a, agentAppearanceEmojiKey)
		delete(a, cliSeedKey)
		delete(a, loopGenerationKey)
	}
	if cmd.Flags().Changed("appearance-emoji") {
		a[agentAppearanceAvatarKey] = agentAppearanceEmojiKey
		delete(a, agentAppearanceIconKey)
		delete(a, cliSeedKey)
		delete(a, loopGenerationKey)
	}
	switch a[agentAppearanceAvatarKey] {
	case "generated":
		// A duplicate starts from no appearance, so a lone plate choice would replace its face.
		if cmd.Flags().Changed("appearance-" + agentAppearanceBackgroundKey) {
			return errAppearanceBackgroundNeedsSymbol
		}
		// A generated face has no plate to drop.
		delete(a, agentAppearanceIconKey)
		delete(a, agentAppearanceEmojiKey)
		delete(a, agentAppearanceBackgroundKey)
		if a[cliSeedKey] == nil || a[cliSeedKey] == "" {
			a[cliSeedKey] = p.Name
		}
		if a[loopGenerationKey] == nil {
			a[loopGenerationKey] = 1
		}
	case agentAppearanceIconKey:
		delete(a, agentAppearanceEmojiKey)
		delete(a, cliSeedKey)
		delete(a, loopGenerationKey)
	case agentAppearanceEmojiKey:
		delete(a, agentAppearanceIconKey)
		delete(a, cliSeedKey)
		delete(a, loopGenerationKey)
	}
	p.Appearance = a
	return nil
}

const agentAppearanceAvatarKey = "avatar"

const agentAppearanceBackgroundKey = "background"

const agentAppearanceEmojiKey = "emoji"

const agentAppearanceIconKey = "icon"
