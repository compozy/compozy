package cli

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

type sessionProcessesClient interface {
	ListSessionProcesses(context.Context, string) (contract.SessionProcessesResponse, error)
	TerminateSessionProcesses(
		context.Context,
		string,
		contract.TerminateSessionProcessesRequest,
	) (contract.TerminateSessionProcessesResponse, error)
	ListProcesses(context.Context, string, bool, bool) (contract.ProcessesResponse, error)
}

type processPolicyFlags struct{ terminate, keep bool }

func (f *processPolicyFlags) bind(cmd *cobra.Command) {
	cmd.Flags().
		BoolVar(&f.terminate, "terminate-processes", false, "Terminate session-owned background processes and terminals")
	cmd.Flags().BoolVar(&f.keep, "keep-processes", false, "Keep session-owned processes running (default)")
	cmd.MarkFlagsMutuallyExclusive("terminate-processes", "keep-processes")
}
func (f processPolicyFlags) policy() contract.SessionProcessPolicy {
	if f.terminate {
		return contract.SessionProcessesTerminate
	}
	if f.keep {
		return contract.SessionProcessesKeep
	}
	return ""
}

func processClient(deps commandDeps) (sessionProcessesClient, error) {
	client, err := clientFromDeps(deps)
	if err != nil {
		return nil, err
	}
	process, ok := client.(sessionProcessesClient)
	if !ok {
		return nil, errors.New("cli: session processes unavailable")
	}
	return process, nil
}
func newSessionProcessesCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "processes <id>",
		Short: "List session-owned processes and listening ports",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := processClient(deps)
			if err != nil {
				return err
			}
			result, err := client.ListSessionProcesses(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, processListBundle(result, result.Items))
		},
	}
	cmd.AddCommand(newSessionProcessesStopCommand(deps))
	return cmd
}

func newSessionProcessesStopCommand(deps commandDeps) *cobra.Command {
	var pid int
	var started string
	var all, includeTools bool
	stop := &cobra.Command{
		Use:   "stop <id>",
		Short: "Terminate verified session process trees",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			request := contract.TerminateSessionProcessesRequest{All: all, IncludeTools: includeTools}
			if includeTools && !all {
				return errors.New("cli: --include-tools requires --all")
			}
			if !all {
				stamp, err := time.Parse(time.RFC3339Nano, started)
				if err != nil || pid <= 1 {
					return errors.New("cli: provide --all or --pid > 1 with --started-at from the list")
				}
				request.Targets = []contract.SessionProcessTarget{{PID: pid, StartedAt: stamp}}
			} else if pid != 0 || started != "" {
				return errors.New("cli: --all cannot be combined with --pid or --started-at")
			}
			client, err := processClient(deps)
			if err != nil {
				return err
			}
			result, err := client.TerminateSessionProcesses(cmd.Context(), args[0], request)
			if err != nil {
				return err
			}
			output := listBundle(
				result,
				result.Results,
				"Process termination",
				[]string{"PID", "OUTCOME", "ERROR"},
				"results",
				[]string{"pid", cliOutcomeKey, "error"},
				func(r contract.SessionProcessResult) []string {
					return []string{strconv.Itoa(r.PID), string(r.Outcome), r.Error}
				},
				func(r contract.SessionProcessResult) []string {
					return []string{strconv.Itoa(r.PID), string(r.Outcome), r.Error}
				},
			)
			if err := writeCommandOutput(cmd, output); err != nil {
				return err
			}
			if !result.Supported {
				return errors.New("cli: process termination unsupported on this host")
			}
			for _, r := range result.Results {
				if r.Outcome != "terminated" && r.Outcome != "already_exited" {
					return fmt.Errorf("cli: process %d is %s", r.PID, r.Outcome)
				}
			}
			return nil
		},
	}
	stop.Flags().IntVar(&pid, "pid", 0, "PID returned by the process list")
	stop.Flags().StringVar(&started, "started-at", "", "Exact RFC3339 process start timestamp returned by the list")
	stop.Flags().BoolVar(&all, "all", false,
		"Terminate every process tree this session left running; tool calls still running in its turn stay")
	stop.Flags().BoolVar(&includeTools, "include-tools", false,
		"With --all, also terminate tool calls still running in the agent's turn")
	return stop
}
func newProcessCommand(deps commandDeps) *cobra.Command {
	root := &cobra.Command{Use: "process", Short: "Inspect processes left running by sessions"}
	var orphaned, allWorkspaces bool
	var workspace string
	list := &cobra.Command{
		Use:   agentListKey,
		Short: "List processes grouped by their session",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := processClient(deps)
			if err != nil {
				return err
			}
			base, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			workspaceID := ""
			if !allWorkspaces {
				workspaceID, err = resolveOptionalWorkspaceOverride(cmd.Context(), cmd, deps, base, workspace)
				if err != nil {
					return err
				}
			}
			result, err := client.ListProcesses(cmd.Context(), workspaceID, allWorkspaces, orphaned)
			if err != nil {
				return err
			}
			output := listBundle(
				result,
				result.Groups,
				"Session processes",
				[]string{strings.ToUpper("session"), "PROFILE", strings.ToUpper("workspace"), "PROCESSES"},
				"groups",
				[]string{automationSessionIDKey, "profile", "workspace_id", "processes"},
				processGroupRow,
				processGroupRow,
			)
			return writeCommandOutput(cmd, output)
		},
	}
	list.Flags().
		BoolVar(&orphaned, "orphaned", false, "Only processes belonging to stopped, archived or missing sessions")
	list.Flags().BoolVar(&allWorkspaces, "all-workspaces", false, "Include all workspaces and User scope")
	list.Flags().StringVar(&workspace, "workspace", "", "Workspace ID, name, path, or @user")
	list.MarkFlagsMutuallyExclusive("workspace", "all-workspaces")
	configureProfileReadCommand(list, deps)
	root.AddCommand(list)
	return root
}
func processGroupRow(g contract.SessionProcessGroupPayload) []string {
	return []string{g.SessionID, g.Profile, g.WorkspaceID, strconv.Itoa(len(g.Items))}
}
func processListBundle(result contract.SessionProcessesResponse, items []contract.SessionProcessPayload) outputBundle {
	row := func(p contract.SessionProcessPayload) []string {
		ports := make([]string, len(p.Ports))
		for i, v := range p.Ports {
			ports[i] = strconv.Itoa(v)
		}
		return []string{
			strconv.Itoa(p.PID),
			string(p.Kind),
			p.Command,
			strings.Join(ports, ","),
			p.StartedAt.Format(time.RFC3339Nano),
			p.TreeID,
		}
	}
	return listBundle(
		result,
		items,
		"Session processes",
		[]string{"PID", "KIND", "COMMAND", "PORTS", "STARTED AT", "TREE"},
		"items",
		[]string{"pid", cliKindKey, "command", "ports", "started_at", "tree_id"},
		row,
		row,
	)
}
