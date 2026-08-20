package cli

import (
	"maps"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/windowmanager"
	"github.com/spf13/cobra"
)

func newCmdPaletteBindCommand(deps commandDeps) *cobra.Command {
	var scope cmdPaletteScopeFlags
	var overwrite bool
	var globalBinding bool
	cmd := &cobra.Command{
		Use:   "bind <id> <chord>",
		Short: "Bind a command to a keyboard chord",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			commandID, err := requiredCmdPaletteID(args[0], "command ID")
			if err != nil {
				return err
			}
			chord, err := requiredCmdPaletteID(args[1], "shortcut chord")
			if err != nil {
				return err
			}
			client, workspace, err := cmdPaletteMutationClientAndWorkspace(cmd, deps, scope.workspace)
			if err != nil {
				return err
			}
			current, err := client.GetCmdPaletteBindings(cmd.Context(), workspace)
			if err != nil {
				return err
			}
			if globalBinding {
				owner := globalShortcutOwner(current.Config.GlobalShortcuts, chord, commandID)
				shortcuts := windowmanager.CloneGlobalShortcutMap(current.Config.GlobalShortcuts)
				shortcuts[commandID] = chord
				updated, updateErr := client.UpdateCmdPaletteBindings(
					cmd.Context(),
					workspace,
					contract.UpdateSettingsWindowManagerRequest{
						GlobalShortcuts: &shortcuts,
						Overwrite:       overwrite,
					},
				)
				if updateErr != nil {
					return updateErr
				}
				result := cmdPaletteBindingMutationResult{Status: "ok"}
				if bound, exists := updated.Config.GlobalShortcuts[commandID]; exists {
					result.Bound = []string{bound}
				}
				if overwrite {
					result.UnboundOwner = owner
				}
				return writeCommandOutput(cmd, cmdPaletteMutationOutput(result))
			}
			owner := shortcutOwner(current.EffectiveShortcuts, chord, commandID)
			shortcuts := windowmanager.CloneShortcutMap(current.Config.Shortcuts)
			shortcuts[commandID] = windowmanager.ShortcutBinding{chord}
			updated, err := client.UpdateCmdPaletteBindings(
				cmd.Context(),
				workspace,
				contract.UpdateSettingsWindowManagerRequest{Shortcuts: &shortcuts, Overwrite: overwrite},
			)
			if err != nil {
				return err
			}
			result := cmdPaletteBindingMutationResult{Status: "ok"}
			result.Bound = append([]string(nil), updated.EffectiveShortcuts[commandID]...)
			if overwrite {
				result.UnboundOwner = owner
			}
			return writeCommandOutput(cmd, cmdPaletteMutationOutput(result))
		},
	}
	scope.add(cmd, false)
	cmd.Flags().BoolVar(&overwrite, cmdPaletteOverwriteFlag, false, "Transfer a conflicting chord")
	cmd.Flags().BoolVar(&globalBinding, "global", false, "Bind as a desktop global hotkey")
	return cmd
}

func newCmdPaletteUnbindCommand(deps commandDeps) *cobra.Command {
	var scope cmdPaletteScopeFlags
	var globalBinding bool
	cmd := &cobra.Command{
		Use:   "unbind <id>",
		Short: "Remove every binding from a command",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			commandID, err := requiredCmdPaletteID(args[0], "command ID")
			if err != nil {
				return err
			}
			client, workspace, err := cmdPaletteMutationClientAndWorkspace(cmd, deps, scope.workspace)
			if err != nil {
				return err
			}
			current, err := client.GetCmdPaletteBindings(cmd.Context(), workspace)
			if err != nil {
				return err
			}
			if globalBinding {
				shortcuts := windowmanager.CloneGlobalShortcutMap(current.Config.GlobalShortcuts)
				delete(shortcuts, commandID)
				if _, updateErr := client.UpdateCmdPaletteBindings(
					cmd.Context(), workspace,
					contract.UpdateSettingsWindowManagerRequest{GlobalShortcuts: &shortcuts},
				); updateErr != nil {
					return updateErr
				}
				return writeCommandOutput(
					cmd,
					cmdPaletteMutationOutput(cmdPaletteStatusResult{Status: "ok"}),
				)
			}
			shortcuts := windowmanager.CloneShortcutMap(current.Config.Shortcuts)
			shortcuts[commandID] = windowmanager.ShortcutBinding{}
			if _, err := client.UpdateCmdPaletteBindings(
				cmd.Context(), workspace, contract.UpdateSettingsWindowManagerRequest{Shortcuts: &shortcuts},
			); err != nil {
				return err
			}
			return writeCommandOutput(cmd, cmdPaletteMutationOutput(cmdPaletteStatusResult{Status: "ok"}))
		},
	}
	scope.add(cmd, false)
	cmd.Flags().BoolVar(&globalBinding, "global", false, "Remove a desktop global hotkey")
	return cmd
}

func newCmdPaletteAliasCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use: "alias", Short: "Manage command aliases",
	}
	cmd.AddCommand(newCmdPaletteAliasSetCommand(deps), newCmdPaletteAliasClearCommand(deps))
	return cmd
}

func newCmdPaletteAliasSetCommand(deps commandDeps) *cobra.Command {
	var scope cmdPaletteScopeFlags
	var overwrite bool
	cmd := &cobra.Command{
		Use: "set <id> <alias>", Short: "Set a workspace command alias", Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			commandID, err := requiredCmdPaletteID(args[0], "command ID")
			if err != nil {
				return err
			}
			alias, err := requiredCmdPaletteAlias(args[1])
			if err != nil {
				return err
			}
			client, workspace, err := cmdPaletteMutationClientAndWorkspace(cmd, deps, scope.workspace)
			if err != nil {
				return err
			}
			current, err := client.GetCmdPaletteBindings(cmd.Context(), workspace)
			if err != nil {
				return err
			}
			aliases := cloneCmdPaletteAliases(current.Aliases)
			aliases[commandID] = alias
			if _, err := client.UpdateCmdPaletteBindings(
				cmd.Context(), workspace,
				contract.UpdateSettingsWindowManagerRequest{Aliases: &aliases, Overwrite: overwrite},
			); err != nil {
				return err
			}
			return writeCommandOutput(cmd, cmdPaletteMutationOutput(cmdPaletteAliasMutationResult{
				Status: "ok", Alias: alias,
			}))
		},
	}
	scope.add(cmd, false)
	cmd.Flags().BoolVar(&overwrite, cmdPaletteOverwriteFlag, false, "Transfer a conflicting alias")
	return cmd
}

func newCmdPaletteAliasClearCommand(deps commandDeps) *cobra.Command {
	var scope cmdPaletteScopeFlags
	cmd := &cobra.Command{
		Use: "clear <id>", Short: "Clear a workspace command alias", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			commandID, err := requiredCmdPaletteID(args[0], "command ID")
			if err != nil {
				return err
			}
			client, workspace, err := cmdPaletteMutationClientAndWorkspace(cmd, deps, scope.workspace)
			if err != nil {
				return err
			}
			current, err := client.GetCmdPaletteBindings(cmd.Context(), workspace)
			if err != nil {
				return err
			}
			aliases := cloneCmdPaletteAliases(current.Aliases)
			delete(aliases, commandID)
			if _, err := client.UpdateCmdPaletteBindings(
				cmd.Context(), workspace, contract.UpdateSettingsWindowManagerRequest{Aliases: &aliases},
			); err != nil {
				return err
			}
			return writeCommandOutput(cmd, cmdPaletteMutationOutput(cmdPaletteStatusResult{Status: "ok"}))
		},
	}
	scope.add(cmd, false)
	return cmd
}

func shortcutOwner(
	effective map[string]windowmanager.ShortcutBinding,
	chord string,
	exclude string,
) string {
	for commandID, binding := range effective {
		if commandID == exclude {
			continue
		}
		for _, candidate := range binding {
			if shortcutChordsEqual(candidate, chord) {
				return commandID
			}
		}
	}
	return ""
}

func globalShortcutOwner(
	shortcuts map[string]string,
	chord string,
	exclude string,
) string {
	for commandID, candidate := range shortcuts {
		if commandID != exclude && shortcutChordsEqual(candidate, chord) {
			return commandID
		}
	}
	return ""
}

func shortcutChordsEqual(left, right string) bool {
	leftCanonical, leftErr := windowmanager.CanonicalShortcutChord(left)
	rightCanonical, rightErr := windowmanager.CanonicalShortcutChord(right)
	if leftErr != nil || rightErr != nil {
		return strings.EqualFold(left, right)
	}
	return leftCanonical == rightCanonical
}

func cloneCmdPaletteAliases(source map[string]string) map[string]string {
	aliases := make(map[string]string, len(source))
	maps.Copy(aliases, source)
	return aliases
}
