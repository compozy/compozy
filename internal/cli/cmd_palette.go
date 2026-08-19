package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

const (
	cmdPaletteKey           = "cmd-palette"
	cmdPaletteWorkspaceFlag = "workspace"
	cmdPaletteClientFlag    = "client"
	cmdPaletteSourceFlag    = "source"
	cmdPaletteAvailableFlag = "available"
	cmdPaletteArgFlag       = "arg"
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
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(
		newCmdPaletteListCommand(deps),
		newCmdPaletteInspectCommand(deps),
		newCmdPaletteInvokeCommand(deps),
		newCmdPaletteClientsCommand(deps),
	)
	return cmd
}

func cmdPaletteClientAndWorkspace(
	cmd *cobra.Command,
	deps commandDeps,
	workspaceRef string,
) (CmdPaletteClient, string, error) {
	client, err := clientFromDeps(deps)
	if err != nil {
		return nil, "", err
	}
	paletteClient, ok := client.(CmdPaletteClient)
	if !ok {
		return nil, "", errors.New("cli: command palette client is unavailable")
	}
	resolution, err := resolveCommandWorkspace(
		cmd.Context(),
		cmd,
		deps,
		paletteClient,
		workspaceResolutionRequest{FlagRef: workspaceRef},
	)
	if err != nil {
		return nil, "", err
	}
	return paletteClient, resolution.ID, nil
}

func requiredCmdPaletteID(value string, noun string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", withCommandExitCode(2, fmt.Errorf("cli: %s is required", noun))
	}
	return trimmed, nil
}
