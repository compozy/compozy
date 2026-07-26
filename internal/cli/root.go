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

	"github.com/compozy/agh/internal/agentidentity"
	"github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"
	aghdaemon "github.com/compozy/agh/internal/daemon"
	diagnosticspkg "github.com/compozy/agh/internal/diagnostics"
	"github.com/compozy/agh/internal/version"
	"github.com/spf13/cobra"
)

const (
	rootAghKey       = "agh"
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

	defaultPollInterval = 100 * time.Millisecond
	defaultStartTimeout = 15 * time.Second
	defaultStopTimeout  = 15 * time.Second
)

type daemonRunner interface {
	Run(ctx context.Context) error
}

type runtimeContext struct {
	HomePaths aghconfig.HomePaths
	Config    aghconfig.Config
}

type installWizardRunner func(context.Context, installWizardInput) (installWizardSelection, error)

type mcpServeRunner func(context.Context, mcpServeOptions) error

type commandDeps struct {
	loadConfig                  func() (aghconfig.Config, error)
	loadSkillRegistrySources    skillRegistrySourceLoader
	resolveHome                 func() (aghconfig.HomePaths, error)
	resolveHomeForWorkspace     func(workspaceRoot string) (aghconfig.HomePaths, error)
	ensureHome                  func(aghconfig.HomePaths) error
	runInstallWizard            installWizardRunner
	runBridgeSetupWizard        bridgeSetupWizardRunner
	generateBridgeSetupSecret   bridgeSetupSecretGenerator
	newClient                   func(socketPath string) (DaemonClient, error)
	newDaemon                   func() (daemonRunner, error)
	runRelaunchHelper           func(context.Context, aghdaemon.RelaunchHelperConfig) error
	readDaemonInfo              func(path string) (aghdaemon.Info, error)
	signalProcess               func(pid int, sig syscall.Signal) error
	processAlive                func(pid int) bool
	processMatchesStartTime     func(pid int, startedAt time.Time) bool
	executable                  func() (string, error)
	getwd                       func() (string, error)
	getenv                      func(string) string
	lookPath                    func(string) (string, error)
	now                         func() time.Time
	pollInterval                time.Duration
	startTimeout                time.Duration
	stopTimeout                 time.Duration
	spawnDetached               func(context.Context, aghconfig.HomePaths) (daemonProcess, error)
	newUpdateManager            func(aghconfig.HomePaths) (updateManager, error)
	runProviderAuthCommand      providerAuthCommandRunner
	runProviderAuthLoginCommand providerAuthCommandRunner
	inputIsTerminal             func(io.Reader) bool
	runMCPServe                 mcpServeRunner
}

// NewRootCommand constructs the AGH v1 CLI command tree.
func NewRootCommand() *cobra.Command {
	return newRootCommand(commandDeps{})
}

func newRootCommand(deps commandDeps) *cobra.Command {
	deps = deps.withDefaults()

	cmd := &cobra.Command{
		Use:   rootAghKey,
		Short: "AGH — Artificial General Hivemind",
		Example: `  # Start the daemon and create a session in the current workspace
  agh daemon start
  agh session new --agent general

  # Print machine-readable output for automation
  agh session list -o json`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.PersistentFlags().
		StringP(outputFlagName, "o", string(OutputHuman), "Output format: human, json, jsonl, or toon")
	cmd.PersistentFlags().Bool(jsonFlagName, false, "Emit JSON output")

	cmd.AddCommand(newVersionCommand())
	cmd.AddCommand(newInstallCommand(deps))
	cmd.AddCommand(newConfigCommand(deps))
	cmd.AddCommand(newSupportCommand(deps))
	cmd.AddCommand(newUpdateCommand(deps))
	cmd.AddCommand(newUninstallCommand(deps))
	cmd.AddCommand(newStatusCommand(deps))
	cmd.AddCommand(newObserveCommand(deps))
	cmd.AddCommand(newDoctorCommand(deps))
	cmd.AddCommand(newDrainCommand(deps))
	cmd.AddCommand(newUndrainCommand(deps))
	cmd.AddCommand(newOnboardingCommand(deps))
	cmd.AddCommand(newDaemonCommand(deps))
	cmd.AddCommand(newNetworkCommand(deps))
	cmd.AddCommand(newMeCommand(deps))
	cmd.AddCommand(newSpawnCommand(deps))
	cmd.AddCommand(newChannelCommand(deps))
	cmd.AddCommand(newSessionCommand(deps))
	cmd.AddCommand(newProviderCommand(deps))
	cmd.AddCommand(newBridgeCommand(deps))
	cmd.AddCommand(newNotificationsCommand(deps))
	cmd.AddCommand(newMarketplaceCommand(deps))
	cmd.AddCommand(newBundleCommand(deps))
	cmd.AddCommand(newWorkspaceCommand(deps))
	cmd.AddCommand(newDesktopCommand(deps))
	cmd.AddCommand(newWindowCommand(deps))
	cmd.AddCommand(newLayoutCommand(deps))
	cmd.AddCommand(newLayoutProfileCommand(deps))
	cmd.AddCommand(newAgentCommand(deps))
	cmd.AddCommand(newRolesCommand(deps))
	cmd.AddCommand(newExtensionCommand(deps))
	cmd.AddCommand(newHooksCommand(deps))
	cmd.AddCommand(newAutomationCommand(deps))
	cmd.AddCommand(newLoopCommand(deps))
	cmd.AddCommand(newSchedulerCommand(deps))
	cmd.AddCommand(newTaskCommand(deps))
	cmd.AddCommand(newSkillCommand(deps))
	cmd.AddCommand(newResourceCommand(deps))
	cmd.AddCommand(newMemoryCommand(deps))
	cmd.AddCommand(newVaultCommand(deps))
	cmd.AddCommand(newToolCommand(deps))
	cmd.AddCommand(newToolsetsCommand(deps))
	cmd.AddCommand(newMCPCommand(deps))
	cmd.AddCommand(newLogsCommand(deps))
	cmd.AddCommand(newWhoamiCommand(deps))
	cmd.AddCommand(newOpenCommand(deps))
	cmd.AddCommand(newDocCommand())
	cmd.AddCommand(newInternalCommand())

	return cmd
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   rootVersionKey,
		Short: "Print the AGH version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return writeCommandOutput(cmd, outputBundle{
				jsonValue: version.Current(),
				human: func() (string, error) {
					return fmt.Sprintf("agh %s", version.Current().Version), nil
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
				_, _ = fmt.Fprintln(stderr)
			}
			return exitCode
		}
	}

	_, _ = fmt.Fprintf(stderr, "error: %v\n", err)
	return exitCode
}

func marshalStructuredExecutionError(args []string, err error) ([]byte, bool) {
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
		return []byte(renderToonObject("error", []string{"message"}, []string{payload.Error})), true
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
	var identityErr *agentidentity.Error
	return errors.As(err, &identityErr)
}

func requestedOutputFormat(args []string) OutputFormat {
	mode := OutputHuman
	for i := 0; i < len(args); i++ {
		switch arg := strings.TrimSpace(args[i]); {
		case arg == "--json":
			mode = OutputJSON
		case arg == "-o" || arg == "--output":
			if i+1 < len(args) {
				mode = OutputFormat(strings.ToLower(strings.TrimSpace(args[i+1])))
				i++
			}
		case strings.HasPrefix(arg, "--output="):
			mode = OutputFormat(strings.ToLower(strings.TrimSpace(strings.TrimPrefix(arg, "--output="))))
		case strings.HasPrefix(arg, "-o="):
			mode = OutputFormat(strings.ToLower(strings.TrimSpace(strings.TrimPrefix(arg, "-o="))))
		}
	}
	return mode
}
