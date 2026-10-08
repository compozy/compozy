package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/cmdpalette"
	"github.com/compozy/compozy/internal/cmdpalette/corecmds"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/spf13/cobra"
)

const (
	cmdPaletteKey               = "cmd-palette"
	cmdPaletteWorkspaceFlag     = "workspace"
	cmdPaletteClientFlag        = "client"
	cmdPaletteSourceFlag        = "source"
	cmdPaletteAvailableFlag     = "available"
	cmdPaletteAvailableFlagHelp = "Filter by current availability when provided; omitted lists all commands"
	cmdPaletteArgFlag           = "arg"
	cmdPaletteOverwriteFlag     = "overwrite"
	cmdPaletteInvalidArgs       = "invalid_arguments"
	cmdPaletteNoShell           = "no_attached_shell"
)

type cmdPaletteScopeFlags struct {
	workspace string
	clientID  string
}

func (flags *cmdPaletteScopeFlags) add(cmd *cobra.Command, includeClient bool) {
	cmd.Flags().StringVar(
		&flags.workspace,
		cmdPaletteWorkspaceFlag,
		"",
		"Override workspace (ID, name, or path)",
	)
	if includeClient {
		cmd.Flags().StringVar(&flags.clientID, cmdPaletteClientFlag, "", "Resolve against one attached client")
	}
}

func newCmdPaletteCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   cmdPaletteKey,
		Short: "List, inspect, and invoke command palette commands",
	}
	cmd.AddCommand(
		newCmdPaletteListCommand(deps),
		newCmdPaletteInspectCommand(deps),
		newCmdPaletteInvokeCommand(deps),
		newCmdPaletteClientsCommand(deps),
		newCmdPalettePersonalizationCommand(deps),
		newCmdPaletteBindCommand(deps),
		newCmdPaletteUnbindCommand(deps),
		newCmdPaletteAliasCommand(deps),
		newCmdPaletteBindingsCommand(deps),
		newCmdPalettePinCommand(deps, true),
		newCmdPalettePinCommand(deps, false),
	)
	return cmd
}

func cmdPaletteMutationClientAndWorkspace(
	cmd *cobra.Command,
	deps commandDeps,
	workspaceRef string,
) (CmdPaletteMutationClient, string, error) {
	client, err := clientFromDeps(deps)
	if err != nil {
		return nil, "", err
	}
	mutationClient, ok := client.(CmdPaletteMutationClient)
	if !ok {
		return nil, "", errors.New("cli: command palette mutation client is unavailable")
	}
	workspaceID, err := resolveCmdPaletteWorkspaceID(cmd, deps, mutationClient, workspaceRef)
	if err != nil {
		return nil, "", err
	}
	return mutationClient, workspaceID, nil
}

func cmdPaletteClientAndWorkspace(
	cmd *cobra.Command,
	deps commandDeps,
	workspaceRef string,
) (CmdPaletteClient, string, error) {
	paletteClient, err := cmdPaletteClientFromDeps(deps)
	if err != nil {
		return nil, "", err
	}
	workspaceID, err := resolveCmdPaletteWorkspaceID(cmd, deps, paletteClient, workspaceRef)
	if err != nil {
		return nil, "", err
	}
	return paletteClient, workspaceID, nil
}

func resolveCmdPaletteWorkspaceID(
	cmd *cobra.Command,
	deps commandDeps,
	lookup workspaceLookupClient,
	workspaceRef string,
) (string, error) {
	resolution, err := resolveCommandWorkspace(
		cmd.Context(),
		cmd,
		deps,
		lookup,
		workspaceResolutionRequest{FlagRef: workspaceRef},
	)
	if err != nil {
		return "", err
	}
	return resolution.ID, nil
}

func requiredCmdPaletteID(value string, noun string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", withCommandExitCode(2, fmt.Errorf("cli: %s is required", noun))
	}
	return trimmed, nil
}

// canonicalCmdPaletteCommandID resolves a retired palette command ID to its
// replacement with a stderr warning. Public aliases are removed in v0.5.0.
func canonicalCmdPaletteCommandID(cmd *cobra.Command, id string) string {
	replacement, retired := corecmds.RetiredCommandID(cmdpalette.CommandID(id))
	if !retired {
		return id
	}
	cmd.PrintErrf("warning: command %q is deprecated and will be removed in v0.5.0; use %q\n", id, replacement)
	return string(replacement)
}

func requiredCmdPaletteAlias(value string) (string, error) {
	if value == "" {
		return "", withCommandExitCode(2, fmt.Errorf("cli: alias is required"))
	}
	if err := compozyconfig.ValidateCmdPaletteAlias(value); err != nil {
		return "", withCommandExitCode(2, fmt.Errorf("cli: alias %w", err))
	}
	return value, nil
}
