package agent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/compozy/compozy/internal/core/model"
)

// Spec defines how to bootstrap an ACP-compatible agent process.
type Spec struct {
	ID                 string
	DisplayName        string
	DefaultModel       string
	Command            string
	FixedArgs          []string
	ProbeArgs          []string
	Fallbacks          []Launcher
	SupportsAddDirs    bool
	UsesBootstrapModel bool
	EnvVars            map[string]string
	DocsURL            string
	InstallHint        string
	FullAccessModeID   string
	BootstrapArgs      func(modelName, reasoningEffort string, addDirs []string, accessMode string) []string
}

// Launcher defines one ACP-compatible command shape for a runtime.
type Launcher struct {
	Command   string
	FixedArgs []string
	ProbeArgs []string
}

// DriverCatalogEntry exposes the stable command catalog for one supported ACP runtime.
type DriverCatalogEntry struct {
	IDE                string
	DisplayName        string
	CanonicalCommand   []string
	CanonicalProbe     []string
	FallbackLaunchers  []DriverCatalogLauncher
	SupportsAddDirs    bool
	UsesBootstrapModel bool
	DocsURL            string
	InstallHint        string
}

// DriverCatalogLauncher describes one fallback launcher for a supported ACP runtime.
type DriverCatalogLauncher struct {
	Command []string
	Probe   []string
}

var supportedRegistryIDEOrder = []string{
	model.IDEClaude,
	model.IDECodex,
	model.IDEDroid,
	model.IDECursor,
	model.IDEOpenCode,
	model.IDEPi,
	model.IDEGemini,
}

var (
	registryMu = sync.RWMutex{}
	registry   = map[string]Spec{
		model.IDEClaude: {
			ID:           model.IDEClaude,
			DisplayName:  "Claude",
			DefaultModel: model.DefaultClaudeModel,
			Command:      "claude-agent-acp",
			Fallbacks: []Launcher{
				{
					Command:   "npx",
					FixedArgs: []string{"--yes", "@agentclientprotocol/claude-agent-acp"},
				},
			},
			DocsURL:          "https://github.com/agentclientprotocol/claude-agent-acp",
			InstallHint:      "Install `@agentclientprotocol/claude-agent-acp` and expose `claude-agent-acp` on PATH.",
			FullAccessModeID: "bypassPermissions",
			BootstrapArgs: func(_ string, _ string, _ []string, _ string) []string {
				return nil
			},
		},
		model.IDECodex: {
			ID:           model.IDECodex,
			DisplayName:  "Codex",
			DefaultModel: model.DefaultCodexModel,
			Command:      "codex-acp",
			Fallbacks: []Launcher{
				{
					Command:   "npx",
					FixedArgs: []string{"--yes", "@zed-industries/codex-acp"},
				},
			},
			DocsURL:     "https://github.com/zed-industries/codex-acp",
			InstallHint: "Install the Codex ACP adapter from the GitHub releases or via `npx @zed-industries/codex-acp`, then expose `codex-acp` on PATH.",
			BootstrapArgs: func(_ string, _ string, _ []string, accessMode string) []string {
				if accessMode != model.AccessModeFull {
					return nil
				}
				return []string{
					"-c", `approval_policy="never"`,
					"-c", `sandbox_mode="danger-full-access"`,
					"-c", `web_search="live"`,
				}
			},
		},
		model.IDEDroid: {
			ID:           model.IDEDroid,
			DisplayName:  "Droid",
			DefaultModel: model.DefaultCodexModel,
			Command:      "droid",
			FixedArgs:    []string{"exec", "--output-format", "acp"},
			ProbeArgs:    []string{"exec", "--help"},
			Fallbacks: []Launcher{
				{
					Command:   "npx",
					FixedArgs: []string{"--yes", "droid", "exec", "--output-format", "acp"},
					ProbeArgs: []string{"--yes", "droid", "exec", "--help"},
				},
			},
			UsesBootstrapModel: true,
			EnvVars: map[string]string{
				"DROID_DISABLE_AUTO_UPDATE":         "true",
				"FACTORY_DROID_AUTO_UPDATE_ENABLED": "false",
			},
			DocsURL:     "https://factory.ai/product/cli",
			InstallHint: "Install or upgrade Droid so `droid exec --output-format acp` is available.",
			BootstrapArgs: func(modelName, reasoningEffort string, _ []string, accessMode string) []string {
				args := make([]string, 0, 5)
				if accessMode == model.AccessModeFull {
					args = append(args, "--skip-permissions-unsafe")
				}
				args = append(args, "--model", modelName, "--reasoning-effort", reasoningEffort)
				return args
			},
		},
		model.IDECursor: {
			ID:           model.IDECursor,
			DisplayName:  "Cursor",
			DefaultModel: model.DefaultCursorModel,
			Command:      "cursor-agent",
			FixedArgs:    []string{"acp"},
			ProbeArgs:    []string{"acp", "--help"},
			DocsURL:      "https://cursor.com/docs/cli/acp",
			InstallHint:  "Install the Cursor agent CLI package and expose `cursor-agent` on PATH so `cursor-agent acp` works.",
			BootstrapArgs: func(_ string, _ string, _ []string, _ string) []string {
				return nil
			},
		},
		model.IDEOpenCode: {
			ID:           model.IDEOpenCode,
			DisplayName:  "OpenCode",
			DefaultModel: model.DefaultOpenCodeModel,
			Command:      "opencode",
			FixedArgs:    []string{"acp"},
			ProbeArgs:    []string{"acp", "--help"},
			DocsURL:      "https://opencode.ai",
			InstallHint:  "Install or upgrade OpenCode so the `opencode acp` subcommand is available.",
			BootstrapArgs: func(_ string, _ string, _ []string, _ string) []string {
				return nil
			},
		},
		model.IDEPi: {
			ID:           model.IDEPi,
			DisplayName:  "Pi",
			DefaultModel: model.DefaultPiModel,
			Command:      "pi-acp",
			Fallbacks: []Launcher{
				{
					Command:   "npx",
					FixedArgs: []string{"--yes", "pi-acp"},
				},
			},
			DocsURL:     "https://github.com/svkozak/pi-acp",
			InstallHint: "Install `pi-acp` and expose the `pi-acp` binary on PATH.",
			BootstrapArgs: func(_ string, _ string, _ []string, _ string) []string {
				return nil
			},
		},
		model.IDEGemini: {
			ID:           model.IDEGemini,
			DisplayName:  "Gemini",
			DefaultModel: model.DefaultGeminiModel,
			Command:      "gemini",
			FixedArgs:    []string{"--acp"},
			ProbeArgs:    []string{"--acp", "--help"},
			Fallbacks: []Launcher{
				{
					Command:   "npx",
					FixedArgs: []string{"--yes", "@google/gemini-cli", "--acp"},
					ProbeArgs: []string{"--yes", "@google/gemini-cli", "--acp", "--help"},
				},
			},
			DocsURL:     "https://geminicli.com",
			InstallHint: "Install Gemini CLI with ACP support so `gemini --acp` succeeds.",
			BootstrapArgs: func(_ string, _ string, _ []string, _ string) []string {
				return nil
			},
		},
	}
)

// AvailabilityError reports an ACP runtime that is missing or incorrectly installed.
type AvailabilityError struct {
	IDE         string
	DisplayName string
	Command     []string
	DocsURL     string
	InstallHint string
	Output      string
	Cause       error
}

func (e *AvailabilityError) Error() string {
	if e == nil {
		return ""
	}

	command := formatShellCommand(e.Command)
	if command == "" {
		command = e.DisplayName
	}

	parts := []string{
		fmt.Sprintf("ACP transport required for %q", e.IDE),
		fmt.Sprintf("tried %s", command),
	}
	if e.Cause != nil {
		parts = append(parts, e.Cause.Error())
	}
	if trimmed := strings.TrimSpace(e.Output); trimmed != "" {
		parts = append(parts, "adapter output: "+trimmed)
	}
	if trimmed := strings.TrimSpace(e.InstallHint); trimmed != "" {
		parts = append(parts, trimmed)
	}
	if trimmed := strings.TrimSpace(e.DocsURL); trimmed != "" {
		parts = append(parts, "docs: "+trimmed)
	}
	return strings.Join(parts, ". ")
}

func (e *AvailabilityError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// ValidateRuntimeConfig verifies that the runtime config references a supported agent runtime.
func ValidateRuntimeConfig(cfg *model.RuntimeConfig) error {
	if cfg.Mode != model.ExecutionModePRReview && cfg.Mode != model.ExecutionModePRDTasks {
		return fmt.Errorf(
			"invalid --mode value %q: must be %q or %q",
			cfg.Mode,
			model.ModeCodeReview,
			model.ModePRDTasks,
		)
	}
	if _, err := lookupAgentSpec(cfg.IDE); err != nil {
		return fmt.Errorf("invalid --ide value %q: must be %s", cfg.IDE, quotedSupportedIDEs())
	}
	switch cfg.AccessMode {
	case "", model.AccessModeDefault, model.AccessModeFull:
	default:
		return fmt.Errorf(
			"invalid --access-mode value %q: must be %q or %q",
			cfg.AccessMode,
			model.AccessModeDefault,
			model.AccessModeFull,
		)
	}
	if cfg.Mode == model.ExecutionModePRDTasks && cfg.BatchSize != 1 {
		return fmt.Errorf("batch size must be 1 for prd-tasks mode (got %d)", cfg.BatchSize)
	}
	if cfg.MaxRetries < 0 {
		return fmt.Errorf("max-retries cannot be negative (got %d)", cfg.MaxRetries)
	}
	if cfg.RetryBackoffMultiplier <= 0 {
		return fmt.Errorf("retry-backoff-multiplier must be positive (got %.2f)", cfg.RetryBackoffMultiplier)
	}
	return nil
}

// EnsureAvailable verifies that the configured ACP agent binary is installed and executable.
func EnsureAvailable(cfg *model.RuntimeConfig) error {
	if cfg.DryRun {
		return nil
	}

	spec, err := lookupAgentSpec(cfg.IDE)
	if err != nil {
		return err
	}
	if _, err := resolveLaunchCommand(
		spec,
		spec.DefaultModel,
		cfg.ReasoningEffort,
		cfg.AddDirs,
		cfg.AccessMode,
		true,
	); err != nil {
		return err
	}
	return nil
}

// DisplayName returns the human-readable display name for an agent runtime.
func DisplayName(ide string) string {
	spec, err := lookupAgentSpec(ide)
	if err != nil {
		return ""
	}
	return spec.DisplayName
}

// DriverCatalog returns the stable ACP driver catalog in the supported IDE order.
func DriverCatalog() []DriverCatalogEntry {
	registryMu.RLock()
	defer registryMu.RUnlock()

	entries := make([]DriverCatalogEntry, 0, len(supportedRegistryIDEOrder))
	for _, ide := range supportedRegistryIDEOrder {
		spec, ok := registry[ide]
		if !ok {
			continue
		}
		entries = append(entries, driverCatalogEntryFromSpec(spec))
	}
	return entries
}

// DriverCatalogEntryForIDE returns the stable driver catalog entry for one supported IDE.
func DriverCatalogEntryForIDE(ide string) (DriverCatalogEntry, error) {
	spec, err := lookupAgentSpec(ide)
	if err != nil {
		return DriverCatalogEntry{}, err
	}
	return driverCatalogEntryFromSpec(spec), nil
}

// BuildShellCommandString renders a shell preview for the configured ACP agent bootstrap command.
func BuildShellCommandString(
	ide string,
	modelName string,
	addDirs []string,
	reasoningEffort string,
	accessMode string,
) string {
	spec, err := lookupAgentSpec(ide)
	if err != nil {
		return ""
	}

	resolvedModel := resolveModel(spec, modelName)
	resolvedDirs := addDirs
	if !spec.SupportsAddDirs {
		resolvedDirs = nil
	}
	launchModel := resolvedModel
	if !spec.UsesBootstrapModel {
		launchModel = spec.DefaultModel
	}
	args := spec.launchCommandForPreview(launchModel, reasoningEffort, resolvedDirs, accessMode)

	parts := make([]string, 0, len(spec.EnvVars)+1)
	parts = append(parts, sortedEnvAssignments(spec.EnvVars)...)
	parts = append(parts, formatShellCommand(args))
	return strings.Join(parts, " ")
}

func assertCommandExists(spec Spec, command []string) error {
	if len(command) == 0 {
		return &AvailabilityError{
			IDE:         spec.ID,
			DisplayName: spec.DisplayName,
			DocsURL:     spec.DocsURL,
			InstallHint: spec.InstallHint,
			Cause:       errors.New("missing ACP command configuration"),
		}
	}
	if _, err := exec.LookPath(command[0]); err != nil {
		return &AvailabilityError{
			IDE:         spec.ID,
			DisplayName: spec.DisplayName,
			Command:     command,
			DocsURL:     spec.DocsURL,
			InstallHint: spec.InstallHint,
			Cause:       fmt.Errorf("command %q was not found on PATH", command[0]),
		}
	}
	return nil
}

func lookupAgentSpec(ide string) (Spec, error) {
	registryMu.RLock()
	defer registryMu.RUnlock()

	spec, ok := registry[ide]
	if !ok {
		return Spec{}, fmt.Errorf("unknown agent runtime %q", ide)
	}
	return cloneAgentSpec(spec), nil
}

func resolveModel(spec Spec, modelName string) string {
	if strings.TrimSpace(modelName) != "" {
		return modelName
	}
	return spec.DefaultModel
}

func quotedSupportedIDEs() string {
	items := make([]string, 0, len(supportedRegistryIDEOrder))
	for _, ide := range supportedRegistryIDEOrder {
		items = append(items, fmt.Sprintf("%q", ide))
	}
	return strings.Join(items, ", ")
}

func sortedEnvAssignments(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	assignments := make([]string, 0, len(keys))
	for _, key := range keys {
		assignments = append(assignments, fmt.Sprintf("%s=%s", key, formatShellArg(env[key])))
	}
	return assignments
}

func cloneAgentSpec(spec Spec) Spec {
	spec.EnvVars = mapsClone(spec.EnvVars)
	spec.FixedArgs = slices.Clone(spec.FixedArgs)
	spec.ProbeArgs = slices.Clone(spec.ProbeArgs)
	spec.Fallbacks = cloneLaunchers(spec.Fallbacks)
	return spec
}

func driverCatalogEntryFromSpec(spec Spec) DriverCatalogEntry {
	primary := spec.primaryLauncher()
	entry := DriverCatalogEntry{
		IDE:                spec.ID,
		DisplayName:        spec.DisplayName,
		CanonicalCommand:   primary.catalogCommand(),
		CanonicalProbe:     primary.probeCommand(),
		SupportsAddDirs:    spec.SupportsAddDirs,
		UsesBootstrapModel: spec.UsesBootstrapModel,
		DocsURL:            spec.DocsURL,
		InstallHint:        spec.InstallHint,
	}
	if len(spec.Fallbacks) > 0 {
		entry.FallbackLaunchers = make([]DriverCatalogLauncher, 0, len(spec.Fallbacks))
		for _, launcher := range spec.Fallbacks {
			entry.FallbackLaunchers = append(entry.FallbackLaunchers, DriverCatalogLauncher{
				Command: launcher.catalogCommand(),
				Probe:   launcher.probeCommand(),
			})
		}
	}
	return entry
}

func mapsClone(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func formatShellCommand(args []string) string {
	formatted := make([]string, len(args))
	for i, arg := range args {
		formatted[i] = formatShellArg(arg)
	}
	return strings.Join(formatted, " ")
}

func formatShellArg(arg string) string {
	if arg == "" {
		return `""`
	}
	if strings.ContainsAny(arg, " \t\n\"'\\$`|&;<>*?[]{}()") {
		return strconv.Quote(arg)
	}
	return arg
}

func (s Spec) launchCommand(modelName, reasoningEffort string, addDirs []string, accessMode string) []string {
	return s.primaryLauncher().launchCommand(s, modelName, reasoningEffort, addDirs, accessMode)
}

func (s Spec) probeCommand() []string {
	return s.primaryLauncher().probeCommand()
}

func (s Spec) sessionModeForAccess(accessMode string) string {
	if accessMode == model.AccessModeFull {
		return s.FullAccessModeID
	}
	return ""
}

func (s Spec) launchCommandForPreview(modelName, reasoningEffort string, addDirs []string, accessMode string) []string {
	for _, launcher := range s.launchers() {
		command := launcher.launchCommand(s, modelName, reasoningEffort, addDirs, accessMode)
		if err := assertCommandExists(s, command); err == nil {
			return command
		}
	}
	return s.launchCommand(modelName, reasoningEffort, addDirs, accessMode)
}

func (s Spec) primaryLauncher() Launcher {
	return Launcher{
		Command:   s.Command,
		FixedArgs: slices.Clone(s.FixedArgs),
		ProbeArgs: slices.Clone(s.ProbeArgs),
	}
}

func (s Spec) launchers() []Launcher {
	launchers := []Launcher{s.primaryLauncher()}
	launchers = append(launchers, cloneLaunchers(s.Fallbacks)...)
	return launchers
}

func (l Launcher) launchCommand(
	spec Spec,
	modelName, reasoningEffort string,
	addDirs []string,
	accessMode string,
) []string {
	args := append([]string{l.Command}, slices.Clone(l.FixedArgs)...)
	if spec.BootstrapArgs != nil {
		args = append(args, spec.BootstrapArgs(modelName, reasoningEffort, addDirs, accessMode)...)
	}
	return args
}

func (l Launcher) catalogCommand() []string {
	return append([]string{l.Command}, slices.Clone(l.FixedArgs)...)
}

func (l Launcher) probeCommand() []string {
	args := slices.Clone(l.ProbeArgs)
	if len(args) == 0 {
		args = append(args, l.FixedArgs...)
		args = append(args, "--help")
	}
	return append([]string{l.Command}, args...)
}

func resolveLaunchCommand(
	spec Spec,
	modelName string,
	reasoningEffort string,
	addDirs []string,
	accessMode string,
	verify bool,
) ([]string, error) {
	var attemptErrs []error
	for _, launcher := range spec.launchers() {
		command := launcher.launchCommand(spec, modelName, reasoningEffort, addDirs, accessMode)
		if err := assertCommandExists(spec, command); err != nil {
			attemptErrs = append(attemptErrs, err)
			continue
		}
		if verify {
			if err := verifyLauncher(spec, launcher); err != nil {
				attemptErrs = append(attemptErrs, err)
				continue
			}
		}
		return command, nil
	}
	return nil, joinAvailabilityErrors(spec, attemptErrs)
}

func verifyLauncher(spec Spec, launcher Launcher) error {
	command := launcher.probeCommand()
	if err := assertCommandExists(spec, command); err != nil {
		return err
	}

	cmd := exec.CommandContext(context.Background(), command[0], command[1:]...)
	cmd.Env = mergeEnvironment(spec.EnvVars, nil)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		return &AvailabilityError{
			IDE:         spec.ID,
			DisplayName: spec.DisplayName,
			Command:     command,
			DocsURL:     spec.DocsURL,
			InstallHint: spec.InstallHint,
			Output:      output.String(),
			Cause:       err,
		}
	}
	return nil
}

func joinAvailabilityErrors(spec Spec, errs []error) error {
	if len(errs) == 0 {
		return &AvailabilityError{
			IDE:         spec.ID,
			DisplayName: spec.DisplayName,
			DocsURL:     spec.DocsURL,
			InstallHint: spec.InstallHint,
			Cause:       errors.New("no ACP launch candidates configured"),
		}
	}
	if len(errs) == 1 {
		return errs[0]
	}

	parts := make([]string, 0, len(errs))
	for _, err := range errs {
		parts = append(parts, err.Error())
	}
	return errors.New(strings.Join(parts, " | "))
}

func cloneLaunchers(src []Launcher) []Launcher {
	if len(src) == 0 {
		return nil
	}
	dst := make([]Launcher, len(src))
	for i, launcher := range src {
		dst[i] = Launcher{
			Command:   launcher.Command,
			FixedArgs: slices.Clone(launcher.FixedArgs),
			ProbeArgs: slices.Clone(launcher.ProbeArgs),
		}
	}
	return dst
}
