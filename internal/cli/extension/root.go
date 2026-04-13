package extension

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	huh "charm.land/huh/v2"

	extensions "github.com/compozy/compozy/internal/core/extension"
	"github.com/compozy/compozy/internal/core/kernel"
	"github.com/compozy/compozy/internal/core/workspace"
	"github.com/spf13/cobra"
)

type discoverRequest struct {
	HomeDir         string
	WorkspaceRoot   string
	IncludeDisabled bool
	Store           *extensions.EnablementStore
}

type commandDeps struct {
	resolveHomeDir       func() (string, error)
	resolveWorkspaceRoot func(context.Context) (string, error)
	isInteractive        func() bool
	confirmInstall       func(*cobra.Command, installPrompt) (bool, error)
	resolveInstallSource func(context.Context, string, installSourceOptions) (resolvedInstallSource, error)
	loadManifest         func(context.Context, string) (*extensions.Manifest, error)
	loadInstallOrigin    func(string) (*extensions.InstallOrigin, error)
	writeInstallOrigin   func(string, extensions.InstallOrigin) error
	newEnablementStore   func(context.Context, string) (*extensions.EnablementStore, error)
	discover             func(context.Context, discoverRequest) (extensions.DiscoveryResult, error)
	copyDir              func(string, string) error
	removeAll            func(string) error
	pathExists           func(string) (bool, error)
}

type commandEnv struct {
	homeDir       string
	workspaceRoot string
	store         *extensions.EnablementStore
}

type installPrompt struct {
	Name         string
	Source       string
	InstallPath  string
	Capabilities []extensions.Capability
	SetupAssets  []string
}

func defaultCommandDeps() commandDeps {
	return commandDeps{
		resolveHomeDir: os.UserHomeDir,
		resolveWorkspaceRoot: func(ctx context.Context) (string, error) {
			root, err := workspace.Discover(ctx, "")
			if err != nil {
				return "", fmt.Errorf("resolve workspace root: %w", err)
			}
			return root, nil
		},
		isInteractive:        isInteractiveTerminal,
		confirmInstall:       confirmInstallPrompt,
		resolveInstallSource: resolveInstallSource,
		loadManifest:         extensions.LoadManifest,
		loadInstallOrigin:    extensions.LoadInstallOrigin,
		writeInstallOrigin:   extensions.WriteInstallOrigin,
		newEnablementStore:   extensions.NewEnablementStore,
		discover:             discoverExtensions,
		copyDir:              copyDirectoryTree,
		removeAll:            os.RemoveAll,
		pathExists:           pathExists,
	}
}

func discoverExtensions(ctx context.Context, req discoverRequest) (extensions.DiscoveryResult, error) {
	discovery := extensions.Discovery{
		WorkspaceRoot:   req.WorkspaceRoot,
		HomeDir:         req.HomeDir,
		IncludeDisabled: req.IncludeDisabled,
		Enablement:      req.Store,
	}
	return discovery.Discover(ctx)
}

func (d commandDeps) resolveEnv(ctx context.Context) (commandEnv, error) {
	homeDir, err := d.resolveHomeDir()
	if err != nil {
		return commandEnv{}, fmt.Errorf("resolve home directory: %w", err)
	}
	workspaceRoot, err := d.resolveWorkspaceRoot(ctx)
	if err != nil {
		return commandEnv{}, err
	}

	store, err := d.newEnablementStore(ctx, homeDir)
	if err != nil {
		return commandEnv{}, fmt.Errorf("create enablement store: %w", err)
	}

	return commandEnv{
		homeDir:       filepath.Clean(homeDir),
		workspaceRoot: workspaceRoot,
		store:         store,
	}, nil
}

func (d commandDeps) discoverAll(ctx context.Context, env commandEnv) (extensions.DiscoveryResult, error) {
	return d.discover(ctx, discoverRequest{
		HomeDir:         env.homeDir,
		WorkspaceRoot:   env.workspaceRoot,
		IncludeDisabled: true,
		Store:           env.store,
	})
}

func signalCommandContext(cmd *cobra.Command) (context.Context, context.CancelFunc) {
	baseCtx := context.Background()
	if cmd != nil && cmd.Context() != nil {
		baseCtx = cmd.Context()
	}
	return signal.NotifyContext(baseCtx, os.Interrupt, syscall.SIGTERM)
}

// NewExtCommand constructs the builtin `compozy ext` command group.
func NewExtCommand(_ *kernel.Dispatcher) *cobra.Command {
	return newExtCommandWithDeps(nil, defaultCommandDeps())
}

func newExtCommandWithDeps(_ *kernel.Dispatcher, deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "ext",
		Short:        "Manage discovered Compozy extensions",
		SilenceUsage: true,
		Long: `Inspect and manage bundled, user, and workspace Compozy extensions.

User and workspace extensions are discoverable but remain disabled on this
machine until the local operator explicitly enables them.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		newListCommand(deps),
		newInspectCommand(deps),
		newInstallCommand(deps),
		newUninstallCommand(deps),
		newEnableCommand(deps),
		newDisableCommand(deps),
		newDoctorCommand(deps),
	)

	return cmd
}

func normalizeExtensionName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", fmt.Errorf("extension name is required")
	}
	return name, nil
}

func matchingName(left string, right string) bool {
	return strings.EqualFold(strings.TrimSpace(left), strings.TrimSpace(right))
}

func findEffectiveExtension(
	result extensions.DiscoveryResult,
	name string,
) (extensions.DiscoveredExtension, bool) {
	for index := range result.Extensions {
		entry := result.Extensions[index]
		if matchingName(entry.Ref.Name, name) {
			return entry, true
		}
	}
	return extensions.DiscoveredExtension{}, false
}

func findToggleTarget(
	result extensions.DiscoveryResult,
	name string,
	enable bool,
) (extensions.DiscoveredExtension, bool) {
	candidates := matchingDiscoveredExtensions(result, name, func(entry extensions.DiscoveredExtension) bool {
		return entry.Enabled != enable
	})
	if len(candidates) == 0 {
		return extensions.DiscoveredExtension{}, false
	}

	slices.SortFunc(candidates, compareDiscoveredByPrecedence)
	return candidates[0], true
}

func hasAnyDiscoveredMatch(result extensions.DiscoveryResult, name string) bool {
	return len(matchingDiscoveredExtensions(result, name, nil)) > 0
}

func matchingDiscoveredExtensions(
	result extensions.DiscoveryResult,
	name string,
	include func(extensions.DiscoveredExtension) bool,
) []extensions.DiscoveredExtension {
	matches := make([]extensions.DiscoveredExtension, 0)
	for index := range result.Discovered {
		entry := result.Discovered[index]
		if !matchingName(entry.Ref.Name, name) {
			continue
		}
		if include != nil && !include(entry) {
			continue
		}
		matches = append(matches, entry)
	}
	return matches
}

func compareDiscoveredByPrecedence(left, right extensions.DiscoveredExtension) int {
	if diff := sourcePrecedence(right.Ref.Source) - sourcePrecedence(left.Ref.Source); diff != 0 {
		return diff
	}
	return strings.Compare(left.ManifestPath, right.ManifestPath)
}

func sourcePrecedence(source extensions.Source) int {
	switch source {
	case extensions.SourceBundled:
		return 0
	case extensions.SourceUser:
		return 1
	case extensions.SourceWorkspace:
		return 2
	default:
		return -1
	}
}

func overrideRecordsForName(result extensions.DiscoveryResult, name string) []extensions.OverrideRecord {
	records := make([]extensions.OverrideRecord, 0)
	for index := range result.Overrides {
		record := result.Overrides[index]
		if matchingName(record.Name, name) {
			records = append(records, record)
		}
	}
	return records
}

func sortedCapabilities(values []extensions.Capability) []extensions.Capability {
	capabilities := append([]extensions.Capability(nil), values...)
	slices.SortFunc(capabilities, func(left, right extensions.Capability) int {
		return strings.Compare(string(left), string(right))
	})
	return capabilities
}

func renderCapabilities(values []extensions.Capability) string {
	capabilities := sortedCapabilities(values)
	if len(capabilities) == 0 {
		return noneLabel
	}

	parts := make([]string, 0, len(capabilities))
	for _, capability := range capabilities {
		parts = append(parts, string(capability))
	}
	return strings.Join(parts, ", ")
}

func pathExists(path string) (bool, error) {
	_, err := os.Lstat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func runPromptField(field huh.Field) error {
	return huh.NewForm(huh.NewGroup(field)).Run()
}

func isInteractiveTerminal() bool {
	stdin, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	stdout, err := os.Stdout.Stat()
	if err != nil {
		return false
	}

	return stdin.Mode()&os.ModeCharDevice != 0 && stdout.Mode()&os.ModeCharDevice != 0
}
