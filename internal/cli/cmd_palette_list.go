package cli

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

func newCmdPaletteListCommand(deps commandDeps) *cobra.Command {
	var scope cmdPaletteScopeFlags
	var source string
	var available bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List commands and their current availability",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, workspace, err := cmdPaletteClientAndWorkspace(cmd, deps, scope.workspace)
			if err != nil {
				return err
			}
			catalog, err := client.ListCmdPaletteCommands(cmd.Context(), workspace, scope.clientID)
			if err != nil {
				return err
			}
			commands := filterCmdPaletteCommands(
				catalog.Commands,
				strings.TrimSpace(source),
				available,
				cmd.Flags().Changed(cmdPaletteAvailableFlag),
			)
			return writeCommandOutput(cmd, cmdPaletteListOutput(workspace, commands))
		},
	}
	scope.add(cmd, true)
	cmd.Flags().StringVar(&source, cmdPaletteSourceFlag, "", "Filter by source (core or ext.<name>)")
	cmd.Flags().BoolVar(&available, cmdPaletteAvailableFlag, true, "Filter by current availability")
	return cmd
}

func newCmdPaletteInspectCommand(deps commandDeps) *cobra.Command {
	var scope cmdPaletteScopeFlags
	cmd := &cobra.Command{
		Use:   "inspect <id>",
		Short: "Inspect one command descriptor",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			commandID, err := requiredCmdPaletteID(args[0], "command ID")
			if err != nil {
				return err
			}
			client, workspace, err := cmdPaletteClientAndWorkspace(cmd, deps, scope.workspace)
			if err != nil {
				return err
			}
			catalog, err := client.ListCmdPaletteCommands(cmd.Context(), workspace, scope.clientID)
			if err != nil {
				return err
			}
			command, ok := findCmdPaletteCommand(catalog.Commands, commandID)
			if !ok {
				return fmt.Errorf("command not found: %s", commandID)
			}
			return writeCommandOutput(cmd, cmdPaletteInspectOutput(command))
		},
	}
	scope.add(cmd, true)
	return cmd
}

func newCmdPaletteClientsCommand(deps commandDeps) *cobra.Command {
	var scope cmdPaletteScopeFlags
	cmd := &cobra.Command{
		Use:   "clients",
		Short: "List clients attached to the workspace",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, workspace, err := cmdPaletteClientAndWorkspace(cmd, deps, scope.workspace)
			if err != nil {
				return err
			}
			clients, err := client.ListCmdPaletteClients(cmd.Context(), workspace)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, cmdPaletteClientsOutput(clients))
		},
	}
	scope.add(cmd, false)
	return cmd
}

func filterCmdPaletteCommands(
	commands []contract.CmdPaletteCommand,
	source string,
	available bool,
	filterAvailability bool,
) []contract.CmdPaletteCommand {
	filtered := make([]contract.CmdPaletteCommand, 0, len(commands))
	for _, command := range commands {
		if source != "" && command.Source != source {
			continue
		}
		if filterAvailability && command.Available != available {
			continue
		}
		filtered = append(filtered, command)
	}
	sort.Slice(filtered, func(left, right int) bool { return filtered[left].ID < filtered[right].ID })
	return filtered
}

func findCmdPaletteCommand(commands []contract.CmdPaletteCommand, id string) (contract.CmdPaletteCommand, bool) {
	for _, command := range commands {
		if string(command.ID) == id {
			return command, true
		}
	}
	return contract.CmdPaletteCommand{}, false
}

func cmdPaletteListOutput(workspace string, commands []contract.CmdPaletteCommand) outputBundle {
	return listBundle(
		commands,
		commands,
		"COMMANDS (workspace: "+workspace+")",
		[]string{"ID", "TITLE", "SOURCE", "AVAILABLE", "BINDINGS"},
		"commands",
		[]string{"id", "title", "source", "available", "bindings"},
		func(command contract.CmdPaletteCommand) []string {
			return []string{
				string(command.ID), command.Title, command.Source,
				strconv.FormatBool(command.Available), strings.Join(command.Bindings, ","),
			}
		},
		func(command contract.CmdPaletteCommand) []string {
			return []string{
				string(command.ID), command.Title, command.Source,
				strconv.FormatBool(command.Available), strings.Join(command.Bindings, ","),
			}
		},
	)
}

func cmdPaletteInspectOutput(command contract.CmdPaletteCommand) outputBundle {
	return outputBundle{
		jsonValue: command,
		jsonl:     func(cmd *cobra.Command) error { return writeJSONLine(cmd, command) },
		human: func() (string, error) {
			return renderHumanSectionResult("Command", []keyValue{
				{Label: "ID", Value: string(command.ID)},
				{Label: "Title", Value: command.Title},
				{Label: "Source", Value: command.Source},
				{Label: "Available", Value: strconv.FormatBool(command.Available)},
				{Label: "Reason", Value: command.Reason},
			})
		},
		toon: func() (string, error) {
			return renderToonObject(
				"command",
				[]string{"id", "title", "source", "available", "reason"},
				[]string{string(command.ID), command.Title, command.Source, strconv.FormatBool(command.Available), command.Reason},
			), nil
		},
	}
}

func cmdPaletteClientsOutput(clients []contract.CmdPaletteClient) outputBundle {
	return listBundle(
		clients,
		clients,
		"ATTACHED CLIENTS",
		[]string{"CLIENT", "KIND", "WORKSPACE", "ATTACHED"},
		"clients",
		[]string{"client_id", "kind", "workspace", "attached_at"},
		func(client contract.CmdPaletteClient) []string {
			return []string{string(client.ClientID), client.Kind, string(client.Workspace), client.AttachedAt.Format(time.RFC3339)}
		},
		func(client contract.CmdPaletteClient) []string {
			return []string{string(client.ClientID), client.Kind, string(client.Workspace), client.AttachedAt.Format(time.RFC3339)}
		},
	)
}
