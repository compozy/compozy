package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newTerminalOpenCommand(deps commandDeps) *cobra.Command {
	var workspace, cwd, shell, title string
	var cols, rows uint16
	var detach bool
	command := &cobra.Command{
		Use: "open", Short: "Open an interactive terminal", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, workspaceID, err := terminalClientAndWorkspace(cmd, deps, workspace)
			if err != nil {
				return err
			}
			terminal, err := client.CreateTerminal(cmd.Context(), workspaceID, TerminalCreateRequest{
				Cwd: cwd, Shell: shell, Cols: cols, Rows: rows, Title: title,
			})
			if err != nil {
				return err
			}
			if detach {
				return writeCommandOutput(cmd, terminalRecordBundle(terminal))
			}
			return client.AttachTerminal(cmd.Context(), workspaceID, terminal.ID, TerminalAttachOptions{
				Mode: "write", Flow: "ack", Cols: cols, Rows: rows,
			}, cmd.InOrStdin(), cmd.OutOrStdout())
		},
	}
	addTerminalWorkspaceFlag(command, &workspace)
	command.Flags().StringVar(&cwd, "cwd", "", "Working directory under the workspace")
	command.Flags().StringVar(&shell, "shell", "", "Shell executable")
	command.Flags().StringVar(&title, "title", "", "Pinned terminal title")
	command.Flags().Uint16Var(&cols, "cols", 80, "Initial columns")
	command.Flags().Uint16Var(&rows, "rows", 24, "Initial rows")
	command.Flags().BoolVar(&detach, "detach", false, "Open without attaching")
	configureProfileMutationCommand(command, deps)
	return command
}

func newTerminalListCommand(deps commandDeps) *cobra.Command {
	var workspace string
	command := &cobra.Command{
		Use: "list", Short: "List workspace terminals", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, workspaceID, err := terminalClientAndWorkspace(cmd, deps, workspace)
			if err != nil {
				return err
			}
			items, err := client.ListTerminals(cmd.Context(), workspaceID)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, terminalListBundle(items))
		},
	}
	addTerminalWorkspaceFlag(command, &workspace)
	configureProfileReadCommand(command, deps)
	return command
}

func newTerminalGetCommand(deps commandDeps) *cobra.Command {
	var workspace string
	command := &cobra.Command{
		Use: "get <id>", Short: "Show one terminal", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, workspaceID, err := terminalClientAndWorkspace(cmd, deps, workspace)
			if err != nil {
				return err
			}
			terminal, err := client.GetTerminal(cmd.Context(), workspaceID, args[0])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, terminalRecordBundle(terminal))
		},
	}
	addTerminalWorkspaceFlag(command, &workspace)
	configureSingleProfileReadCommand(command, deps)
	return command
}

func newTerminalAttachCommand(deps commandDeps) *cobra.Command {
	var workspace string
	var control bool
	var afterSeq uint64
	var cols, rows uint16
	command := &cobra.Command{
		Use: "attach <id>", Short: "Attach to a running terminal", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, workspaceID, err := terminalClientAndWorkspace(cmd, deps, workspace)
			if err != nil {
				return err
			}
			mode, flow := "read", "drop"
			if control {
				mode, flow = "write", "ack"
			}
			return client.AttachTerminal(cmd.Context(), workspaceID, args[0], TerminalAttachOptions{
				Mode: mode, Flow: flow, AfterSeq: afterSeq, Cols: cols, Rows: rows,
			}, cmd.InOrStdin(), cmd.OutOrStdout())
		},
	}
	addTerminalWorkspaceFlag(command, &workspace)
	command.Flags().BoolVar(&control, "control", false, "Attach with the write lease")
	command.Flags().Uint64Var(&afterSeq, "after-seq", 0, "Resume after this byte sequence")
	command.Flags().Uint16Var(&cols, "cols", 80, "Terminal columns")
	command.Flags().Uint16Var(&rows, "rows", 24, "Terminal rows")
	configureSingleProfileReadCommand(command, deps)
	return command
}

func newTerminalKillCommand(deps commandDeps) *cobra.Command {
	var workspace, signal string
	command := &cobra.Command{
		Use: "kill <id>", Short: "Terminate a terminal", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, workspaceID, err := terminalClientAndWorkspace(cmd, deps, workspace)
			if err != nil {
				return err
			}
			exit, err := client.DeleteTerminal(cmd.Context(), workspaceID, args[0], signal)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, terminalExitBundle(args[0], exit))
		},
	}
	addTerminalWorkspaceFlag(command, &workspace)
	command.Flags().StringVar(&signal, "signal", "HUP", "Signal: INT, TERM, KILL, or HUP")
	configureProfileMutationCommand(command, deps)
	return command
}

func terminalRecordBundle(record TerminalRecord) outputBundle {
	return outputBundle{
		jsonValue: map[string]any{"terminal": record},
		human: func() (string, error) {
			return fmt.Sprintf("%s\t%s\t%s\t%s", record.ID, record.ProfileName, record.State, record.Title), nil
		},
	}
}

func terminalListBundle(records []TerminalRecord) outputBundle {
	return outputBundle{
		jsonValue: map[string]any{"terminals": records},
		human: func() (string, error) {
			if len(records) == 0 {
				return "No terminals in this workspace.", nil
			}
			lines := []string{"ID\tPROFILE\tTITLE\tCONTROLLER\tSTATE"}
			for _, record := range records {
				controller := "available"
				if record.Controller != nil {
					controller = record.Controller.Kind + " " + record.Controller.ID
				}
				lines = append(lines, fmt.Sprintf("%s\t%s\t%s\t%s\t%s", record.ID, record.ProfileName, record.Title, controller, record.State))
			}
			return strings.Join(lines, "\n"), nil
		},
	}
}

func terminalExitBundle(id string, exit TerminalExitRecord) outputBundle {
	return outputBundle{
		jsonValue: map[string]any{"exit": exit},
		human: func() (string, error) {
			return fmt.Sprintf("%s terminated (%s)", id, exit.Cause), nil
		},
	}
}
