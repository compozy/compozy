package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newTerminalOpenCommand(deps commandDeps) *cobra.Command {
	var workspace, cwd, shell, title string
	var cols, rows uint16
	var detach bool
	command := &cobra.Command{
		Use: terminalOpenCommandKey, Short: "Open an interactive terminal", Args: cobra.NoArgs,
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
			if err := writeTerminalOpenAttachBanner(cmd.OutOrStdout(), terminal); err != nil {
				return err
			}
			if err := client.AttachTerminal(cmd.Context(), workspaceID, terminal.ID, TerminalAttachOptions{
				Mode: terminalStreamModeWrite, Flow: terminalStreamFlowAck, Cols: cols, Rows: rows,
			}, cmd.InOrStdin(), cmd.OutOrStdout()); err != nil {
				return err
			}
			return writeTerminalDetachNotice(cmd.Context(), cmd.OutOrStdout(), client, workspaceID, terminal.ID)
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
		Use: terminalListCommandKey, Short: "List workspace terminals", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, workspaceID, err := terminalClientAndWorkspace(cmd, deps, workspace)
			if err != nil {
				return err
			}
			items, err := client.ListTerminals(cmd.Context(), workspaceID)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, terminalListBundle(cmd, items, time.Now))
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
	var control, force bool
	var afterSeq uint64
	var cols, rows uint16
	command := &cobra.Command{
		Use: "attach <id>", Short: "Attach to a running terminal", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, workspaceID, err := terminalClientAndWorkspace(cmd, deps, workspace)
			if err != nil {
				return err
			}
			terminal, err := client.GetTerminal(cmd.Context(), workspaceID, args[0])
			if err != nil {
				return err
			}
			mode, flow := terminalStreamModeRead, terminalStreamFlowDrop
			takeover := false
			if control {
				mode, flow = terminalStreamModeWrite, terminalStreamFlowAck
				takeover = terminalControllerNeedsTakeover(terminal.Controller)
				if takeover &&
					terminal.Controller != nil &&
					terminal.Controller.Kind == terminalControllerHumanKind &&
					!force {
					confirmed, confirmErr := confirmTerminalTakeover(cmd, terminal.Controller.ID)
					if confirmErr != nil {
						return confirmErr
					}
					if !confirmed {
						return nil
					}
					force = true
				}
			}
			if err := writeTerminalAttachBanner(cmd.OutOrStdout(), terminal, control); err != nil {
				return err
			}
			if err := client.AttachTerminal(cmd.Context(), workspaceID, args[0], TerminalAttachOptions{
				Mode: mode, Flow: flow, AfterSeq: afterSeq, Cols: cols, Rows: rows,
				Takeover: takeover, Force: force,
			}, cmd.InOrStdin(), cmd.OutOrStdout()); err != nil {
				return err
			}
			return writeTerminalDetachNotice(cmd.Context(), cmd.OutOrStdout(), client, workspaceID, args[0])
		},
	}
	addTerminalWorkspaceFlag(command, &workspace)
	command.Flags().BoolVar(&control, "control", false, "Attach with the write lease")
	command.Flags().BoolVar(
		&force,
		terminalForceKey,
		false,
		"Displace another human controller without confirmation",
	)
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
	value := map[string]any{"terminal": record}
	return outputBundle{
		jsonValue: value,
		json:      terminalHTTPJSON(value),
		human: func() (string, error) {
			return fmt.Sprintf("%s\t%s\t%s\t%s", record.ID, record.ProfileName, record.State, record.Title), nil
		},
	}
}

func terminalListBundle(cmd *cobra.Command, records []TerminalRecord, now func() time.Time) outputBundle {
	value := map[string]any{"terminals": records}
	return outputBundle{
		jsonValue: value,
		json:      terminalHTTPJSON(value),
		human: func() (string, error) {
			if len(records) == 0 {
				workspace, profile := terminalListScopeLabels(cmd)
				return fmt.Sprintf(
					"No terminals in workspace %s (profile: %s). Open one: compozy terminal open",
					workspace,
					profile,
				), nil
			}
			selection, _ := commandProfileReadSelection(cmd)
			headers := []string{"ID"}
			if selection.AllProfiles {
				headers = append(headers, "PROFILE")
			}
			headers = append(headers, "TITLE", "CONTROLLER", "STATE", "CREATED")
			lines := []string{strings.Join(headers, "\t")}
			for _, record := range records {
				fields := []string{record.ID}
				if selection.AllProfiles {
					fields = append(fields, record.ProfileName)
				}
				fields = append(fields,
					record.Title,
					terminalListController(record.Controller),
					record.State,
					terminalListAge(now, record.CreatedAt),
				)
				lines = append(lines, strings.Join(fields, "\t"))
			}
			return strings.Join(lines, "\n"), nil
		},
	}
}

func terminalListScopeLabels(cmd *cobra.Command) (string, string) {
	workspace := terminalCurrentWorkspaceLabel
	if resolution, ok := commandWorkspaceResolution(cmd); ok {
		workspace = strings.TrimSpace(resolution.Detail.Workspace.Name)
		if workspace == "" {
			workspace = strings.TrimSpace(resolution.ID)
		}
	}
	profile := configDefaultKey
	if selection, ok := commandProfileReadSelection(cmd); ok && strings.TrimSpace(selection.Profile) != "" {
		profile = strings.TrimSpace(selection.Profile)
	}
	return workspace, profile
}

func terminalListController(controller *TerminalControllerRecord) string {
	if controller == nil {
		return terminalControllerAvailable
	}
	if controller.Kind == terminalControllerHumanKind && controller.ID == terminalCLIActorID {
		return "you"
	}
	return strings.TrimSpace(controller.ID)
}

func terminalListAge(now func() time.Time, createdAt time.Time) string {
	age := formatAge(now, createdAt)
	if age == "" {
		return "--"
	}
	return age + " ago"
}

func terminalHTTPJSON(value any) func(*cobra.Command) error {
	return func(cmd *cobra.Command) error {
		return writeJSONWithoutWorkspaceResolution(cmd, value)
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
