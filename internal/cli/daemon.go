package cli

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"
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
	runCLIDaemonForeground         = daemon.Run
)

const (
	daemonHTTPPortEnv            = "COMPOZY_DAEMON_HTTP_PORT"
	daemonWebDevProxyEnv         = "COMPOZY_WEB_DEV_PROXY"
	daemonWebDevProxyFlag        = "--web-dev-proxy"
	daemonStartInternalChildFlag = "internal-child"
)

type daemonStartState struct {
	outputFormat      string
	foreground        bool
	internalChild     bool
	webDevProxyTarget string
}

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
	state := &daemonStartState{outputFormat: operatorOutputFormatText}
	cmd := &cobra.Command{
		Use:          "start",
		Short:        "Start the home-scoped daemon singleton in the background",
		SilenceUsage: true,
		Long: `Start the shared home-scoped daemon singleton.

By default this command detaches, waits for daemon readiness, and returns the
same status view exposed by "compozy daemon status". Use --foreground to keep
the daemon attached to the current terminal.`,
		Example: `  compozy daemon start
  compozy daemon start --format json
  compozy daemon start --foreground
  compozy daemon start --foreground --web-dev-proxy http://127.0.0.1:3000`,
		RunE: state.run,
	}
	cmd.Flags().BoolVar(&state.foreground, "foreground", false, "Run the daemon in the foreground")
	cmd.Flags().StringVar(
		&state.outputFormat,
		"format",
		operatorOutputFormatText,
		"Output format: text or json",
	)
	cmd.Flags().BoolVar(&state.internalChild, daemonStartInternalChildFlag, false, "Internal detached child mode")
	cmd.Flags().StringVar(
		&state.webDevProxyTarget,
		"web-dev-proxy",
		"",
		"Development frontend origin to proxy through the daemon, for example http://127.0.0.1:3000",
	)
	if err := cmd.Flags().MarkHidden(daemonStartInternalChildFlag); err != nil {
		panic(err)
	}
	return cmd
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
	state := &daemonStopState{
		outputFormat: operatorOutputFormatText,
		force:        true,
	}
	cmd := &cobra.Command{
		Use:          "stop",
		Short:        "Request graceful shutdown of the running daemon",
		SilenceUsage: true,
		RunE:         state.run,
	}
	cmd.Flags().BoolVar(&state.force, "force", state.force, "Cancel active runs before stopping the daemon")
	cmd.Flags().StringVar(
		&state.outputFormat,
		"format",
		operatorOutputFormatText,
		"Output format: text or json",
	)
	return cmd
}

func (s *daemonStartState) run(cmd *cobra.Command, _ []string) error {
	format, err := normalizeOperatorOutputFormat(s.outputFormat)
	if err != nil {
		return withExitCode(1, err)
	}

	mode := daemon.RunModeDetached
	if s.foreground {
		mode = daemon.RunModeForeground
	}

	httpPort, err := cliDaemonHTTPPortFromEnv()
	if err != nil {
		return withExitCode(1, err)
	}
	webDevProxyTarget, err := resolveDaemonWebDevProxyTarget(s.webDevProxyTarget)
	if err != nil {
		return withExitCode(1, err)
	}
	runOptions := daemon.RunOptions{
		Version:           version.String(),
		HTTPPort:          httpPort,
		Mode:              mode,
		WebDevProxyTarget: webDevProxyTarget,
	}

	if s.foreground {
		ctx, stop := signalCommandContext(cmd)
		defer stop()
		return runCLIDaemonForeground(ctx, runOptions)
	}

	if s.internalChild {
		return runCLIDaemonForeground(commandContextOrBackground(cmd), runOptions)
	}

	restoreEnv, err := overrideDaemonWebDevProxyEnv(runOptions.WebDevProxyTarget)
	if err != nil {
		return withExitCode(2, err)
	}
	defer func() {
		if restoreErr := restoreEnv(); restoreErr != nil {
			slog.Warn(
				"failed to restore daemon web dev proxy override",
				"env",
				daemonWebDevProxyEnv,
				"error",
				restoreErr,
			)
		}
	}()

	ctx, stop := signalCommandContext(cmd)
	defer stop()
	client, err := newCLIDaemonBootstrap().ensure(ctx)
	if err != nil {
		return withExitCode(2, err)
	}
	daemonStatus, err := client.DaemonStatus(ctx)
	if err != nil {
		return mapDaemonCommandError(err)
	}
	health, err := client.Health(ctx)
	if err != nil {
		return mapDaemonCommandError(err)
	}
	return writeDaemonStatusOutput(cmd, format, &daemonStatus, health, string(daemon.ReadyStateReady))
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

func cliDaemonRunOptionsFromEnv(mode daemon.RunMode) (daemon.RunOptions, error) {
	httpPort, err := cliDaemonHTTPPortFromEnv()
	if err != nil {
		return daemon.RunOptions{}, err
	}
	webDevProxyTarget, err := resolveDaemonWebDevProxyTarget("")
	if err != nil {
		return daemon.RunOptions{}, err
	}
	return daemon.RunOptions{
		Version:           version.String(),
		HTTPPort:          httpPort,
		Mode:              mode,
		WebDevProxyTarget: webDevProxyTarget,
	}, nil
}

func commandContextOrBackground(cmd *cobra.Command) context.Context {
	if cmd != nil && cmd.Context() != nil {
		return cmd.Context()
	}
	return context.Background()
}

func resolveDaemonWebDevProxyTarget(flagValue string) (string, error) {
	if strings.TrimSpace(flagValue) != "" {
		return normalizeDaemonWebDevProxyTarget(flagValue, daemonWebDevProxyFlag)
	}
	return cliDaemonWebDevProxyFromEnv()
}
func cliDaemonHTTPPortFromEnv() (int, error) {
	rawValue, ok := os.LookupEnv(daemonHTTPPortEnv)
	if !ok {
		return 0, nil
	}

	value := strings.TrimSpace(rawValue)
	if value == "" {
		return 0, nil
	}

	port, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s=%q: %w", daemonHTTPPortEnv, rawValue, err)
	}
	if port == 0 {
		return daemon.EphemeralHTTPPort, nil
	}
	return port, nil
}

func cliDaemonWebDevProxyFromEnv() (string, error) {
	rawValue, ok := os.LookupEnv(daemonWebDevProxyEnv)
	if !ok {
		return "", nil
	}
	return normalizeDaemonWebDevProxyTarget(rawValue, daemonWebDevProxyEnv)
}

func normalizeDaemonWebDevProxyTarget(rawValue string, source string) (string, error) {
	value := strings.TrimSpace(rawValue)
	if value == "" {
		return "", nil
	}
	sourceValue := fmt.Sprintf("%s=%q", source, rawValue)
	if !strings.Contains(value, "://") {
		return "", fmt.Errorf("%s must use http or https", sourceValue)
	}

	target, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("parse %s: %w", sourceValue, err)
	}
	if target.Scheme != "http" && target.Scheme != "https" {
		return "", fmt.Errorf("%s must use http or https", sourceValue)
	}
	if strings.TrimSpace(target.Host) == "" {
		return "", fmt.Errorf("%s must include a host", sourceValue)
	}
	return target.String(), nil
}

func overrideDaemonWebDevProxyEnv(value string) (func() error, error) {
	previousValue, hadPrevious := os.LookupEnv(daemonWebDevProxyEnv)
	if strings.TrimSpace(value) == "" {
		return func() error { return nil }, nil
	}
	if err := os.Setenv(daemonWebDevProxyEnv, value); err != nil {
		return nil, fmt.Errorf("set %s=%q: %w", daemonWebDevProxyEnv, value, err)
	}
	return func() error {
		if hadPrevious {
			if err := os.Setenv(daemonWebDevProxyEnv, previousValue); err != nil {
				return fmt.Errorf("restore %s=%q: %w", daemonWebDevProxyEnv, previousValue, err)
			}
			return nil
		}
		if err := os.Unsetenv(daemonWebDevProxyEnv); err != nil {
			return fmt.Errorf("unset %s: %w", daemonWebDevProxyEnv, err)
		}
		return nil
	}, nil
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
