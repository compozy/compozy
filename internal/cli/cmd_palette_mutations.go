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
	return cmd
}

func newCmdPaletteUnbindCommand(deps commandDeps) *cobra.Command {
	var scope cmdPaletteScopeFlags
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
	return cmd
}

func newCmdPaletteAliasCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use: "alias", Short: "Manage command aliases", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
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
			alias, err := requiredCmdPaletteID(args[1], "alias")
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
			if strings.EqualFold(candidate, chord) {
				return commandID
			}
		}
	}
	return ""
}

func cloneCmdPaletteAliases(source map[string]string) map[string]string {
	aliases := make(map[string]string, len(source))
	maps.Copy(aliases, source)
	return aliases
}
