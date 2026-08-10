package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"syscall"
	"time"

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	compozydaemon "github.com/compozy/compozy/internal/daemon"
	diagnosticspkg "github.com/compozy/compozy/internal/diagnostics"
	authproviders "github.com/compozy/compozy/internal/providers"
	"github.com/compozy/compozy/internal/version"
	"github.com/spf13/cobra"
)

const (
	rootCompozyKey   = "compozy"
	rootVersionKey   = "version"
	cliCodeKey       = "code"
	cliFieldKey      = "field"
	cliFieldValue    = "Field"
	cliGetKey        = "get"
	cliKeyKey        = "key"
	cliKeyValue      = "Key"
	cliRevisionValue = "Revision"
	cliSnapshotKey   = "snapshot"
)

const (
	outputFlagName = "output"
	jsonFlagName   = "json"
	yesFlagName    = "yes"
	outputFlagArg  = "--" + outputFlagName
	jsonFlagArg    = "--" + jsonFlagName

	defaultPollInterval = 100 * time.Millisecond
	defaultStartTimeout = 15 * time.Second
	defaultStopTimeout  = 15 * time.Second
)

type daemonRunner interface {
	Run(ctx context.Context) error
}

type runtimeContext struct {
	HomePaths compozyconfig.HomePaths
	Config    compozyconfig.Config
	Target    ClientTarget
}

type installWizardRunner func(context.Context, installWizardInput) (installWizardSelection, error)

type mcpServeRunner func(context.Context, mcpServeOptions) error

type commandDeps struct {
	commandContext              func() context.Context
	loadConfig                  func() (compozyconfig.Config, error)
	loadSkillRegistrySources    skillRegistrySourceLoader
	resolveHome                 func() (compozyconfig.HomePaths, error)
	resolveHomeForWorkspace     func(workspaceRoot string) (compozyconfig.HomePaths, error)
	ensureHome                  func(compozyconfig.HomePaths) error
	runInstallWizard            installWizardRunner
	runBridgeSetupWizard        bridgeSetupWizardRunner
	generateBridgeSetupSecret   bridgeSetupSecretGenerator
	newClient                   func(target ClientTarget) (DaemonClient, error)
	writeGatewayCredential      func(string, string, string) (string, error)
	readGatewayCredential       func(string, string) (string, error)
	removeGatewayCredential     func(string, string) error
	setActiveGatewayProfile     func(compozyconfig.HomePaths, string) error
	applyGatewayProfileState    func(compozyconfig.HomePaths, string, *compozyconfig.GatewayConnectionConfig, string) error
	newSSHExecutor              func() sshExecutor
	reportClientTarget          func(ClientTarget) error
	newDaemon                   func() (daemonRunner, error)
	runRelaunchHelper           func(context.Context, compozydaemon.RelaunchHelperConfig) error
	readDaemonInfo              func(path string) (compozydaemon.Info, error)
	signalProcess               func(pid int, sig syscall.Signal) error
	processAlive                func(pid int) bool
	processMatchesStartTime     func(pid int, startedAt time.Time) bool
	executable                  func() (string, error)
	getwd                       func() (string, error)
	getenv                      func(string) string
	lookPath                    func(string) (string, error)
	removeFile                  func(string) error
	openBrowser                 func(context.Context, string) error
	resolveAppInstallation      func(context.Context, compozyconfig.HomePaths) (appInstallation, error)
	callAppControl              appControlCaller
	now                         func() time.Time
	pollInterval                time.Duration
	startTimeout                time.Duration
	stopTimeout                 time.Duration
	spawnDetached               func(context.Context, compozyconfig.HomePaths) (daemonProcess, error)
	newUpdateManager            func(compozyconfig.HomePaths) (updateManager, error)
	runProviderAuthCommand      providerAuthCommandRunner
	runProviderAuthLoginCommand providerAuthCommandRunner
	resolveProviderAuthCommand  authproviders.ProviderAuthCommandResolver
	inputIsTerminal             func(io.Reader) bool
	runMCPServe                 mcpServeRunner
}

// NewRootCommand constructs the Compozy v1 CLI command tree.
func NewRootCommand() *cobra.Command {
	return newRootCommand(commandDeps{})
}

func newRootCommand(deps commandDeps) *cobra.Command {
	deps = deps.withDefaults()

	cmd := &cobra.Command{
		Use:   rootCompozyKey,
		Short: "Compozy — agent operating system",
		Example: `  # Start the daemon and create a session in the current workspace
  compozy daemon start
  compozy session new --agent general

  # Print machine-readable output for automation
  compozy session list -o json`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	if deps.commandContext == nil {
		deps.commandContext = cmd.Context
	}
	configureRootClientTargetReporting(cmd, &deps)
	configureRootFlags(cmd)
	registerRootCommands(cmd, deps)

	return cmd
}

func configureRootClientTargetReporting(cmd *cobra.Command, deps *commandDeps) {
	if deps.reportClientTarget != nil {
		return
	}
	var pendingTarget *ClientTarget
	deps.reportClientTarget = func(target ClientTarget) error {
		if !target.isRemoteGateway() || pendingTarget != nil {
			return nil
		}
		resolved := target
		pendingTarget = &resolved
		return nil
	}
	cmd.PersistentPostRunE = func(command *cobra.Command, _ []string) error {
		if pendingTarget == nil {
			return nil
		}
		if _, err := fmt.Fprintf(command.ErrOrStderr(), "target: %s\n", pendingTarget.displayName()); err != nil {
			return fmt.Errorf("cli: report remote target: %w", err)
		}
		pendingTarget = nil
		return nil
	}
}

func configureRootFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().
		StringP(outputFlagName, "o", string(OutputHuman), "Output format: human, json, jsonl, or toon")
	cmd.PersistentFlags().Bool(jsonFlagName, false, "Emit JSON output")
}

func registerRootCommands(cmd *cobra.Command, deps commandDeps) {
	cmd.AddCommand(
		newVersionCommand(), newInstallCommand(deps), newConfigCommand(deps), newSupportCommand(deps),
		newUpdateCommand(deps), newUninstallCommand(deps), newStatusCommand(deps), newObserveCommand(deps),
		newDoctorCommand(deps), newDrainCommand(deps), newUndrainCommand(deps), newOnboardingCommand(deps),
		newDaemonCommand(deps), newNetworkCommand(deps), newMeCommand(deps), newSpawnCommand(deps),
		newChannelCommand(deps), newSessionCommand(deps), newProviderCommand(deps), newBridgeCommand(deps),
		newGatewayCommand(deps), newPairCommand(deps), newDeviceCommand(deps), newConnectCommand(deps),
		newNotificationsCommand(deps), newMarketplaceCommand(deps), newWorkspaceCommand(deps), newDesktopCommand(deps),
		newWindowCommand(deps), newLayoutCommand(deps), newLayoutProfileCommand(deps), newAgentCommand(deps),
		newRolesCommand(deps), newExtensionCommand(deps), newHooksCommand(deps), newAutomationCommand(deps),
		newLoopCommand(deps), newSchedulerCommand(deps), newTaskCommand(deps), newSkillCommand(deps),
		newResourceCommand(deps), newMemoryCommand(deps), newVaultCommand(deps), newToolCommand(deps),
		newToolsetsCommand(deps), newMCPCommand(deps), newLogsCommand(deps), newWhoamiCommand(deps),
		newOpenCommand(deps), newDocCommand(), newInternalCommand(),
		newAppCommand(deps),
	)
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   rootVersionKey,
		Short: "Print the Compozy version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return writeCommandOutput(cmd, outputBundle{
				jsonValue: version.Current(),
				human: func() (string, error) {
					return fmt.Sprintf("compozy %s", version.Current().Version), nil
				},
				toon: func() (string, error) {
					info := version.Current()
					return renderToonObject(rootVersionKey, []string{rootVersionKey, "commit", "build_date"}, []string{
						info.Version,
						info.Commit,
						info.BuildDate,
					}), nil
				},
			})
		},
	}
}

func ExecuteContext(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	cmd := NewRootCommand()
	cmd.SetArgs(args)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	if err := cmd.ExecuteContext(ctx); err != nil {
		return writeExecutionError(stderr, args, err)
	}
	return 0
}

func writeExecutionError(stderr io.Writer, args []string, err error) int {
	exitCode := cliExitCodeForError(err)
	if payload, ok := marshalStructuredExecutionError(args, err); ok {
		if _, writeErr := stderr.Write(payload); writeErr == nil {
			if len(payload) == 0 || payload[len(payload)-1] != '\n' {
				if _, newlineErr := fmt.Fprintln(stderr); newlineErr != nil {
					return exitCode
				}
			}
			return exitCode
		}
	}

	if rendered, ok := renderHumanExecutionError(err); ok {
		if _, writeErr := fmt.Fprintln(stderr, rendered); writeErr != nil {
			return exitCode
		}
		return exitCode
	}
	if _, writeErr := fmt.Fprintf(stderr, "error: %v\n", err); writeErr != nil {
		return exitCode
	}
	return exitCode
}

func renderHumanExecutionError(err error) (string, bool) {
	item, ok := diagnosticspkg.ItemFromError(err)
	if !ok {
		return "", false
	}

	title := strings.TrimSpace(item.Title)
	if title == "" {
		title = diagnosticspkg.Redact(err.Error())
	}
	lines := []string{"error: " + title}
	if detail := strings.TrimSpace(item.Message); detail != "" && detail != title {
		lines = append(lines, "  "+detail)
	}
	if command := strings.TrimSpace(item.SuggestedCommand); command != "" {
		lines = append(lines, "try: "+command)
	}
	return strings.Join(lines, "\n"), true
}

func marshalStructuredExecutionError(args []string, err error) ([]byte, bool) {
	if appErr, ok := errors.AsType[*appCommandError](err); ok {
		return marshalAppCommandExecutionError(args, appErr)
	}
	if extensionErr, ok := errors.AsType[interface {
		error
		extensionOperationErrorPayload() contract.ExtensionOperationErrorPayload
	}](err); ok {
		return marshalExtensionOperationExecutionError(args, extensionErr.extensionOperationErrorPayload())
	}
	if windowManagerErr, ok := errors.AsType[interface {
		error
		windowManagerErrorPayload() contract.WindowManagerErrorPayload
	}](err); ok {
		return marshalWindowManagerExecutionError(args, windowManagerErr.windowManagerErrorPayload())
	}
	if goalErr, ok := errors.AsType[*goalCommandAPIError](err); ok {
		return marshalGoalCommandExecutionError(args, goalErr)
	}
	if apiErr, ok := errors.AsType[interface {
		error
		errorPayload() contract.ErrorPayload
	}](err); ok {
		return marshalDaemonAPIExecutionError(args, apiErr.errorPayload())
	}
	if !isStructuredAgentCommandError(err) {
		return marshalDiagnosticExecutionError(args, err)
	}

	switch requestedOutputFormat(args) {
	case OutputJSON:
		payload, marshalErr := agentidentity.MarshalErrorJSON(err)
		if marshalErr != nil {
			return nil, false
		}
		return payload, true
	case OutputJSONL:
		payload, marshalErr := agentidentity.MarshalErrorJSONL(err)
		if marshalErr != nil {
			return nil, false
		}
		return payload, true
	default:
		return nil, false
	}
}

func marshalExtensionOperationExecutionError(
	args []string,
	payload contract.ExtensionOperationErrorPayload,
) ([]byte, bool) {
	switch requestedOutputFormat(args) {
	case OutputJSON:
		encoded, err := json.Marshal(payload)
		return encoded, err == nil
	case OutputJSONL:
		encoded, err := json.Marshal(struct {
			Type  string                                  `json:"type"`
			Error contract.ExtensionOperationErrorPayload `json:"error"`
		}{Type: clientErrorKey, Error: payload})
		return append(encoded, '\n'), err == nil
	case OutputToon:
		return []byte(renderToonObject(
			"error",
			[]string{cliCodeKey, "current_digest", cliAgentsKey, "env_name", "declared_env", clientMessageKey},
			[]string{
				payload.Code,
				payload.CurrentDigest,
				strings.Join(payload.Agents, "|"),
				payload.EnvName,
				strings.Join(payload.DeclaredEnv, "|"),
				payload.Error,
			},
		)), true
	default:
		return nil, false
	}
}

func marshalWindowManagerExecutionError(
	args []string,
	payload contract.WindowManagerErrorPayload,
) ([]byte, bool) {
	switch requestedOutputFormat(args) {
	case OutputJSON:
		encoded, err := json.Marshal(payload)
		return encoded, err == nil
	case OutputJSONL:
		encoded, err := json.Marshal(payload)
		return append(encoded, '\n'), err == nil
	case OutputToon:
		currentRevision := ""
		if payload.CurrentRevision != nil {
			currentRevision = fmt.Sprintf("%d", *payload.CurrentRevision)
		}
		return []byte(renderToonObject(
			"error",
			[]string{cliCodeKey, "workspace_id", "current_revision", clientMessageKey},
			[]string{string(payload.Code), string(payload.WorkspaceID), currentRevision, payload.Error},
		)), true
	default:
		return nil, false
	}
}

func marshalDaemonAPIExecutionError(args []string, payload contract.ErrorPayload) ([]byte, bool) {
	switch requestedOutputFormat(args) {
	case OutputJSON:
		encoded, err := json.Marshal(payload)
		return encoded, err == nil
	case OutputJSONL:
		encoded, err := json.Marshal(struct {
			Type  string                `json:"type"`
			Error contract.ErrorPayload `json:"error"`
		}{Type: automationErrorKey, Error: payload})
		return append(encoded, '\n'), err == nil
	case OutputToon:
		return []byte(renderToonObject("error", []string{clientMessageKey}, []string{payload.Error})), true
	default:
		return nil, false
	}
}

func marshalDiagnosticExecutionError(args []string, err error) ([]byte, bool) {
	item, ok := diagnosticspkg.ItemFromError(err)
	if !ok {
		return nil, false
	}
	payload := contract.ErrorPayload{Error: diagnosticspkg.Redact(err.Error()), Diagnostic: &item}
	switch requestedOutputFormat(args) {
	case OutputJSON:
		encoded, marshalErr := json.Marshal(payload)
		if marshalErr != nil {
			return nil, false
		}
		return encoded, true
	case OutputJSONL:
		encoded, marshalErr := json.Marshal(struct {
			Type  string                `json:"type"`
			Error contract.ErrorPayload `json:"error"`
		}{
			Type:  clientErrorKey,
			Error: payload,
		})
		if marshalErr != nil {
			return nil, false
		}
		return append(encoded, '\n'), true
	default:
		return nil, false
	}
}

func isStructuredAgentCommandError(err error) bool {
	agentErr, ok := errors.AsType[*agentidentity.Error](err)
	return ok && agentErr != nil
}

func requestedOutputFormat(args []string) OutputFormat {
	mode := OutputHuman
	for i := 0; i < len(args); i++ {
		switch arg := strings.TrimSpace(args[i]); {
		case arg == jsonFlagArg:
			mode = OutputJSON
		case arg == "-o" || arg == outputFlagArg:
			if i+1 < len(args) {
				mode = OutputFormat(strings.ToLower(strings.TrimSpace(args[i+1])))
				i++
			}
		case strings.HasPrefix(arg, outputFlagArg+"="):
			mode = OutputFormat(strings.ToLower(strings.TrimSpace(strings.TrimPrefix(arg, outputFlagArg+"="))))
		case strings.HasPrefix(arg, "-o="):
			mode = OutputFormat(strings.ToLower(strings.TrimSpace(strings.TrimPrefix(arg, "-o="))))
		}
	}
	return mode
}
