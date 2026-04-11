package agent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/modelprovider"
	"github.com/compozy/compozy/internal/core/subprocess"
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

// EnsureAvailable verifies that the configured ACP agent binary is installed and executable.
func EnsureAvailable(ctx context.Context, cfg *model.RuntimeConfig) error {
	if cfg == nil {
		return ErrRuntimeConfigNil
	}
	if cfg.DryRun {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	spec, err := lookupAgentSpec(cfg.IDE)
	if err != nil {
		return err
	}
	if _, err := resolveLaunchCommand(
		ctx,
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

// ResolveRuntimeModel returns the effective model after applying the IDE default when needed.
func ResolveRuntimeModel(ide string, modelName string) (string, error) {
	spec, err := lookupAgentSpec(ide)
	if err != nil {
		return "", err
	}
	return resolveModel(spec, modelName), nil
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

func resolveModel(spec Spec, modelName string) string {
	selected := strings.TrimSpace(modelName)
	if selected == "" {
		selected = spec.DefaultModel
	}
	return modelprovider.ResolveAlias(selected)
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

func formatShellCommand(args []string) string {
	formatted := make([]string, len(args))
	for i, arg := range args {
		formatted[i] = formatShellArg(arg)
	}
	return strings.Join(formatted, " ")
}

func formatShellArg(arg string) string {
	if arg == "" {
		return `''`
	}
	if strings.ContainsAny(arg, " \t\n\"'\\$`|&;<>*?[]{}()") {
		return "'" + strings.ReplaceAll(arg, "'", `'\"'\"'`) + "'"
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
	ctx context.Context,
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
			if err := verifyLauncher(ctx, spec, launcher); err != nil {
				attemptErrs = append(attemptErrs, err)
				continue
			}
		}
		return command, nil
	}
	return nil, joinAvailabilityErrors(spec, attemptErrs)
}

func verifyLauncher(ctx context.Context, spec Spec, launcher Launcher) error {
	if ctx == nil {
		ctx = context.Background()
	}
	command := launcher.probeCommand()
	if err := assertCommandExists(spec, command); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Env = subprocess.MergeEnvironment(spec.EnvVars, nil)
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

	return errors.Join(errs...)
}
