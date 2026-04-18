package cli

import (
	"fmt"
	"time"

	apiclient "github.com/compozy/compozy/internal/api/client"
	apicore "github.com/compozy/compozy/internal/api/core"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/daemon"
	"github.com/compozy/compozy/internal/version"
	"github.com/spf13/cobra"
)

var (
	queryDaemonCommandStatus       = daemon.QueryStatus
	newDaemonCommandClientFromInfo = daemonClientFromInfo
)

type daemonStatusState struct {
	outputFormat string
}

type daemonStopState struct {
	outputFormat string
	force        bool
}

type daemonStatusOutput struct {
	State  string                `json:"state"`
	Health apicore.DaemonHealth  `json:"health"`
	Daemon *apicore.DaemonStatus `json:"daemon,omitempty"`
}

type daemonStopOutput struct {
	Accepted bool   `json:"accepted"`
	Force    bool   `json:"force"`
	State    string `json:"state"`
}

func newDaemonCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "daemon",
		Short:        "Manage the home-scoped daemon bootstrap lifecycle",
		SilenceUsage: true,
	}

	cmd.AddCommand(
		newDaemonStartCommand(),
		newDaemonStatusCommand(),
		newDaemonStopCommand(),
	)
	return cmd
}

func newDaemonStartCommand() *cobra.Command {
	return &cobra.Command{
		Use:          "start",
		Short:        "Start the home-scoped daemon singleton in the foreground",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, stop := signalCommandContext(cmd)
			defer stop()

			return daemon.Run(ctx, daemon.RunOptions{
				Version: version.String(),
			})
		},
	}
}

func newDaemonStatusCommand() *cobra.Command {
	state := &daemonStatusState{outputFormat: operatorOutputFormatText}
	cmd := &cobra.Command{
		Use:          "status",
		Short:        "Show the current daemon readiness state",
		SilenceUsage: true,
		RunE:         state.run,
	}
	cmd.Flags().StringVar(
		&state.outputFormat,
		"format",
		operatorOutputFormatText,
		"Output format: text or json",
	)
	return cmd
}

func newDaemonStopCommand() *cobra.Command {
	state := &daemonStopState{outputFormat: operatorOutputFormatText}
	cmd := &cobra.Command{
		Use:          "stop",
		Short:        "Request graceful shutdown of the running daemon",
		SilenceUsage: true,
		RunE:         state.run,
	}
	cmd.Flags().BoolVar(&state.force, "force", false, "Cancel active runs before stopping the daemon")
	cmd.Flags().StringVar(
		&state.outputFormat,
		"format",
		operatorOutputFormatText,
		"Output format: text or json",
	)
	return cmd
}

func (s *daemonStatusState) run(cmd *cobra.Command, _ []string) error {
	format, err := normalizeOperatorOutputFormat(s.outputFormat)
	if err != nil {
		return withExitCode(1, err)
	}

	ctx := cmd.Context()
	status, err := queryDaemonCommandStatus(ctx, compozyconfig.HomePaths{}, daemon.ProbeOptions{})
	if err != nil {
		return withExitCode(2, fmt.Errorf("query daemon status: %w", err))
	}
	if status.Info == nil || status.State == daemon.ReadyStateStopped {
		return writeDaemonStatusOutput(
			cmd,
			format,
			nil,
			apicore.DaemonHealth{Ready: false},
			string(daemon.ReadyStateStopped),
		)
	}

	client, err := newDaemonCommandClientFromInfo(*status.Info)
	if err != nil {
		return withExitCode(2, fmt.Errorf("build daemon status client: %w", err))
	}
	daemonStatus, err := client.DaemonStatus(ctx)
	if err != nil {
		return mapDaemonCommandError(err)
	}
	health, err := client.Health(ctx)
	if err != nil {
		return mapDaemonCommandError(err)
	}

	stateName := string(status.State)
	if health.Ready {
		stateName = string(daemon.ReadyStateReady)
	}
	return writeDaemonStatusOutput(cmd, format, &daemonStatus, health, stateName)
}

func (s *daemonStopState) run(cmd *cobra.Command, _ []string) error {
	format, err := normalizeOperatorOutputFormat(s.outputFormat)
	if err != nil {
		return withExitCode(1, err)
	}

	ctx := cmd.Context()
	status, err := queryDaemonCommandStatus(ctx, compozyconfig.HomePaths{}, daemon.ProbeOptions{})
	if err != nil {
		return withExitCode(2, fmt.Errorf("query daemon status before stop: %w", err))
	}
	if status.Info == nil || status.State == daemon.ReadyStateStopped {
		return writeDaemonStopOutput(cmd, format, false, s.force, string(daemon.ReadyStateStopped))
	}

	client, err := newDaemonCommandClientFromInfo(*status.Info)
	if err != nil {
		return withExitCode(2, fmt.Errorf("build daemon stop client: %w", err))
	}
	if err := client.StopDaemon(ctx, s.force); err != nil {
		return mapDaemonCommandError(err)
	}
	return writeDaemonStopOutput(cmd, format, true, s.force, string(status.State))
}

func daemonClientFromInfo(info daemon.Info) (daemonCommandClient, error) {
	target := apiclient.Target{
		SocketPath: info.SocketPath,
		HTTPPort:   info.HTTPPort,
	}
	return apiclient.New(target)
}

func writeDaemonStatusOutput(
	cmd *cobra.Command,
	format string,
	status *apicore.DaemonStatus,
	health apicore.DaemonHealth,
	state string,
) error {
	if format == operatorOutputFormatJSON {
		payload := daemonStatusOutput{
			State:  state,
			Health: health,
			Daemon: status,
		}
		if err := writeOperatorJSON(cmd.OutOrStdout(), payload); err != nil {
			return withExitCode(2, fmt.Errorf("write daemon status json: %w", err))
		}
		return nil
	}

	if status == nil {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), state)
		if err != nil {
			return withExitCode(2, fmt.Errorf("write daemon status: %w", err))
		}
		return nil
	}

	const daemonStatusTextFormat = "" +
		"state: %s\n" +
		"ready: %t\n" +
		"degraded: %t\n" +
		"pid: %d\n" +
		"version: %s\n" +
		"started_at: %s\n" +
		"socket: %s\n" +
		"http_port: %d\n" +
		"active_runs: %d\n" +
		"workspaces: %d\n"

	_, err := fmt.Fprintf(
		cmd.OutOrStdout(),
		daemonStatusTextFormat,
		state,
		health.Ready,
		health.Degraded,
		status.PID,
		status.Version,
		status.StartedAt.Format(time.RFC3339Nano),
		status.SocketPath,
		status.HTTPPort,
		status.ActiveRunCount,
		status.WorkspaceCount,
	)
	if err != nil {
		return withExitCode(2, fmt.Errorf("write daemon status: %w", err))
	}
	for _, detail := range health.Details {
		if _, detailErr := fmt.Fprintf(
			cmd.OutOrStdout(),
			"detail: %s (%s)\n",
			detail.Message,
			detail.Code,
		); detailErr != nil {
			return withExitCode(2, fmt.Errorf("write daemon status detail: %w", detailErr))
		}
	}
	return nil
}

func writeDaemonStopOutput(
	cmd *cobra.Command,
	format string,
	accepted bool,
	force bool,
	state string,
) error {
	if format == operatorOutputFormatJSON {
		if err := writeOperatorJSON(cmd.OutOrStdout(), daemonStopOutput{
			Accepted: accepted,
			Force:    force,
			State:    state,
		}); err != nil {
			return withExitCode(2, fmt.Errorf("write daemon stop json: %w", err))
		}
		return nil
	}

	message := "daemon already stopped"
	if accepted {
		message = "daemon stop requested"
	}
	if _, err := fmt.Fprintln(cmd.OutOrStdout(), message); err != nil {
		return withExitCode(2, fmt.Errorf("write daemon stop output: %w", err))
	}
	return nil
}
