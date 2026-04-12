package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	core "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/model"
	coreRun "github.com/compozy/compozy/internal/core/run"
	"github.com/compozy/compozy/internal/core/workspace"
	"github.com/compozy/compozy/internal/setup"
	"github.com/spf13/cobra"
)

type workflowIdentity struct {
	pr         string
	name       string
	provider   string
	round      int
	nitpicks   bool
	reviewsDir string
	tasksDir   string
}

type runtimeConfig struct {
	dryRun           bool
	autoCommit       bool
	concurrent       int
	batchSize        int
	agentName        string
	ide              string
	model            string
	addDirs          []string
	tailLines        int
	reasoningEffort  string
	accessMode       string
	explicitRuntime  model.ExplicitRuntimeFlags
	includeCompleted bool
	includeResolved  bool
}

type execConfig struct {
	outputFormat       string
	verbose            bool
	tui                bool
	persist            bool
	extensionsEnabled  bool
	runID              string
	promptText         string
	promptFile         string
	readPromptStdin    bool
	resolvedPromptText string
}

type retryConfig struct {
	timeout                string
	maxRetries             int
	retryBackoffMultiplier float64
}

type commandStateCallbacks struct {
	isInteractive          func() bool
	collectForm            func(*cobra.Command, *commandState) error
	listBundledSkills      func() ([]setup.Skill, error)
	verifyBundledSkills    func(setup.VerifyConfig) (setup.VerifyResult, error)
	installBundledSkills   func(setup.InstallConfig) (*setup.Result, error)
	verifyExtensionSkills  func(setup.ExtensionVerifyConfig) (setup.ExtensionVerifyResult, error)
	installExtensionSkills func(setup.ExtensionInstallConfig) (*setup.ExtensionResult, error)
	confirmSkillRefresh    func(*cobra.Command, skillRefreshPrompt) (bool, error)
	fetchReviewsFn         func(context.Context, core.Config) (*core.FetchResult, error)
	runWorkflow            func(context.Context, core.Config) error
}

type commandState struct {
	workspaceRoot  string
	projectConfig  workspace.ProjectConfig
	kind           commandKind
	mode           core.Mode
	force          bool
	skipValidation bool

	workflowIdentity
	runtimeConfig
	execConfig
	retryConfig
	commandStateCallbacks
}

type commandStateDefaults struct {
	commandStateCallbacks
}

func defaultCommandStateDefaults() commandStateDefaults {
	return commandStateDefaults{
		commandStateCallbacks: commandStateCallbacks{
			isInteractive:          isInteractiveTerminal,
			collectForm:            collectFormParams,
			listBundledSkills:      setup.ListBundledSkills,
			verifyBundledSkills:    setup.VerifyBundledSkills,
			installBundledSkills:   setup.InstallBundledSkills,
			verifyExtensionSkills:  setup.VerifyExtensionSkillPacks,
			installExtensionSkills: setup.InstallExtensionSkillPacks,
			confirmSkillRefresh:    confirmSkillRefreshPrompt,
			fetchReviewsFn:         core.FetchReviews,
			runWorkflow:            core.Run,
		},
	}
}

func (defaults commandStateDefaults) withFallbacks() commandStateDefaults {
	builtin := defaultCommandStateDefaults()
	result := defaults
	if result.isInteractive == nil {
		result.isInteractive = builtin.isInteractive
	}
	if result.collectForm == nil {
		result.collectForm = builtin.collectForm
	}
	if result.listBundledSkills == nil {
		result.listBundledSkills = builtin.listBundledSkills
	}
	if result.verifyBundledSkills == nil {
		result.verifyBundledSkills = builtin.verifyBundledSkills
	}
	if result.installBundledSkills == nil {
		result.installBundledSkills = builtin.installBundledSkills
	}
	if result.verifyExtensionSkills == nil {
		result.verifyExtensionSkills = builtin.verifyExtensionSkills
	}
	if result.installExtensionSkills == nil {
		result.installExtensionSkills = builtin.installExtensionSkills
	}
	if result.confirmSkillRefresh == nil {
		result.confirmSkillRefresh = builtin.confirmSkillRefresh
	}
	if result.fetchReviewsFn == nil {
		result.fetchReviewsFn = builtin.fetchReviewsFn
	}
	if result.runWorkflow == nil {
		result.runWorkflow = builtin.runWorkflow
	}
	return result
}

func newCommandState(kind commandKind, mode core.Mode) *commandState {
	return newCommandStateWithDefaults(kind, mode, defaultCommandStateDefaults())
}

func newCommandStateWithDefaults(kind commandKind, mode core.Mode, defaults commandStateDefaults) *commandState {
	defaults = defaults.withFallbacks()

	return &commandState{
		kind:                  kind,
		mode:                  mode,
		commandStateCallbacks: defaults.commandStateCallbacks,
	}
}

type commonFlagOptions struct {
	includeConcurrent bool
}

func addCommonFlags(cmd *cobra.Command, state *commandState, opts commonFlagOptions) {
	cmd.Flags().BoolVar(&state.dryRun, "dry-run", false, "Only generate prompts; do not run IDE tool")
	cmd.Flags().BoolVar(
		&state.autoCommit,
		"auto-commit",
		false,
		"Include automatic commit instructions at task/batch completion",
	)
	if opts.includeConcurrent {
		cmd.Flags().IntVar(&state.concurrent, "concurrent", 1, "Number of batches to process in parallel")
	}
	cmd.Flags().StringVar(
		&state.ide,
		"ide",
		string(core.IDECodex),
		"ACP runtime to use. Built-in and enabled extension runtimes are validated against the active runtime catalog.",
	)
	cmd.Flags().StringVar(
		&state.model,
		"model",
		"",
		"Model to use. Leave empty to use the selected runtime default.",
	)
	cmd.Flags().StringSliceVar(
		&state.addDirs,
		"add-dir",
		nil,
		"Additional directory to allow for ACP runtimes that support extra writable roots "+
			"(currently claude and codex; repeatable or comma-separated)",
	)
	cmd.Flags().IntVar(
		&state.tailLines,
		"tail-lines",
		0,
		"Maximum number of log lines to retain in UI per job (0 = full history)",
	)
	cmd.Flags().StringVar(
		&state.reasoningEffort,
		"reasoning-effort",
		"medium",
		"Reasoning effort for runtimes that support bootstrap reasoning flags (low, medium, high, xhigh).",
	)
	cmd.Flags().StringVar(
		&state.accessMode,
		"access-mode",
		core.AccessModeFull,
		"Runtime access policy: default keeps native safeguards; "+
			"full requests the most permissive mode Compozy can configure",
	)
	cmd.Flags().StringVar(
		&state.timeout,
		"timeout",
		"10m",
		"Activity timeout duration (e.g., 5m, 30s). Job canceled if no output received within this period.",
	)
	cmd.Flags().IntVar(
		&state.maxRetries,
		"max-retries",
		0,
		"Retry execution-stage ACP failures or timeouts up to N times before marking them failed",
	)
	cmd.Flags().Float64Var(
		&state.retryBackoffMultiplier,
		"retry-backoff-multiplier",
		1.5,
		"Multiplier applied to the next activity timeout after each retry",
	)
}

func (s *commandState) maybeCollectInteractiveParams(cmd *cobra.Command) error {
	if cmd.Flags().NFlag() > 0 {
		return nil
	}

	// newCommandStateWithDefaults wires these callbacks, but tests and focused
	// helpers also construct commandState directly.
	isInteractive := s.isInteractive
	if isInteractive == nil {
		isInteractive = isInteractiveTerminal
	}
	if !isInteractive() {
		return fmt.Errorf(
			"%s requires an interactive terminal when called without flags; pass flags explicitly",
			cmd.CommandPath(),
		)
	}

	collectForm := s.collectForm
	if collectForm == nil {
		collectForm = collectFormParams
	}
	if err := collectForm(cmd, s); err != nil {
		return fmt.Errorf("interactive form failed: %w", err)
	}
	return nil
}

func (s *commandState) buildConfig() (core.Config, error) {
	timeoutDuration := time.Duration(0)
	if s.timeout != "" {
		parsed, err := time.ParseDuration(s.timeout)
		if err != nil {
			return core.Config{}, fmt.Errorf("parse timeout: %w", err)
		}
		if parsed <= 0 {
			return core.Config{}, fmt.Errorf("invalid timeout %q: must be > 0", s.timeout)
		}
		timeoutDuration = parsed
	}

	return core.Config{
		WorkspaceRoot: s.workspaceRoot,
		Name:          s.name,
		Round:         s.round,
		Provider:      s.provider,
		PR:            s.pr,
		Nitpicks:      s.nitpicks,
		ReviewsDir:    s.reviewsDir,
		TasksDir:      s.tasksDir,

		DryRun:           s.dryRun,
		AutoCommit:       s.autoCommit,
		Concurrent:       s.concurrent,
		BatchSize:        s.batchSize,
		AgentName:        s.agentName,
		IDE:              core.IDE(s.ide),
		Model:            s.model,
		AddDirs:          core.NormalizeAddDirs(s.addDirs),
		TailLines:        s.tailLines,
		ReasoningEffort:  s.reasoningEffort,
		AccessMode:       s.accessMode,
		ExplicitRuntime:  s.explicitRuntime,
		IncludeCompleted: s.includeCompleted,
		IncludeResolved:  s.includeResolved,

		Mode:                       s.mode,
		OutputFormat:               core.OutputFormat(s.outputFormat),
		Verbose:                    s.verbose,
		TUI:                        s.tui,
		Persist:                    s.persist,
		EnableExecutableExtensions: s.enableExecutableExtensions(),
		RunID:                      s.runID,
		PromptText:                 s.promptText,
		PromptFile:                 s.promptFile,
		ReadPromptStdin:            s.readPromptStdin,
		ResolvedPromptText:         s.resolvedPromptText,

		Timeout:                timeoutDuration,
		MaxRetries:             s.maxRetries,
		RetryBackoffMultiplier: s.retryBackoffMultiplier,
	}, nil
}

func (s *commandState) enableExecutableExtensions() bool {
	if s == nil {
		return false
	}

	switch s.kind {
	case commandKindStart, commandKindFixReviews:
		return true
	case commandKindExec:
		return s.extensionsEnabled
	default:
		return false
	}
}

func (s *commandState) normalizePresentationMode(cmd *cobra.Command) error {
	if s == nil || !s.isWorkflowExecutionCommand() {
		return nil
	}

	outputFormat := strings.TrimSpace(s.outputFormat)
	if outputFormat == "" {
		outputFormat = string(core.OutputFormatText)
		s.outputFormat = outputFormat
	}

	tuiExplicit := commandFlagChanged(cmd, "tui") || s.hasConfiguredWorkflowTUI()
	if isJSONOutputFormat(outputFormat) {
		if s.tui && tuiExplicit {
			return errors.New("tui mode is not supported with json or raw-json output")
		}
		s.tui = false
		return nil
	}

	isInteractive := s.isInteractive
	if isInteractive == nil {
		isInteractive = isInteractiveTerminal
	}

	if !isInteractive() {
		if s.tui && tuiExplicit {
			return fmt.Errorf(
				"%s requires an interactive terminal for tui mode; rerun with --tui=false",
				cmd.CommandPath(),
			)
		}
		s.tui = false
		return nil
	}

	if !tuiExplicit {
		s.tui = true
	}
	return nil
}

func (s *commandState) isWorkflowExecutionCommand() bool {
	if s == nil {
		return false
	}
	switch s.kind {
	case commandKindStart, commandKindFixReviews:
		return true
	default:
		return false
	}
}

func (s *commandState) hasConfiguredWorkflowTUI() bool {
	if s == nil {
		return false
	}
	switch s.kind {
	case commandKindStart:
		return s.projectConfig.Start.TUI != nil
	case commandKindFixReviews:
		return s.projectConfig.FixReviews.TUI != nil
	default:
		return false
	}
}

func (s *commandState) applyPersistedExecConfig(cmd *cobra.Command, cfg *core.Config) error {
	if cfg == nil || strings.TrimSpace(s.runID) == "" {
		return nil
	}

	record, err := coreRun.LoadPersistedExecRun(s.workspaceRoot, s.runID)
	if err != nil {
		return err
	}
	cfg.Persist = true
	cfg.RunID = record.RunID
	if err := s.assertPersistedExecCompatibility(cmd, *cfg, record); err != nil {
		return err
	}

	cfg.WorkspaceRoot = record.WorkspaceRoot
	cfg.IDE = core.IDE(record.IDE)
	cfg.Model = record.Model
	cfg.ReasoningEffort = record.ReasoningEffort
	cfg.AccessMode = record.AccessMode
	cfg.AddDirs = core.NormalizeAddDirs(record.AddDirs)
	return nil
}

func captureExplicitRuntimeFlags(cmd *cobra.Command) model.ExplicitRuntimeFlags {
	return model.ExplicitRuntimeFlags{
		IDE:             commandFlagChanged(cmd, "ide"),
		Model:           commandFlagChanged(cmd, "model"),
		ReasoningEffort: commandFlagChanged(cmd, "reasoning-effort"),
		AccessMode:      commandFlagChanged(cmd, "access-mode"),
	}
}

func commandFlagChanged(cmd *cobra.Command, name string) bool {
	if cmd == nil {
		return false
	}
	flags := cmd.Flags()
	if flags == nil || flags.Lookup(name) == nil {
		return false
	}
	return flags.Changed(name)
}

func (s *commandState) assertPersistedExecCompatibility(
	cmd *cobra.Command,
	cfg core.Config,
	record coreRun.PersistedExecRun,
) error {
	if cmd.Flags().Changed("ide") && string(cfg.IDE) != record.IDE {
		return fmt.Errorf("--run-id %q must continue with persisted --ide %q", record.RunID, record.IDE)
	}
	if cmd.Flags().Changed("model") && cfg.Model != record.Model {
		return fmt.Errorf("--run-id %q must continue with persisted --model %q", record.RunID, record.Model)
	}
	if cmd.Flags().Changed("reasoning-effort") && cfg.ReasoningEffort != record.ReasoningEffort {
		return fmt.Errorf(
			"--run-id %q must continue with persisted --reasoning-effort %q",
			record.RunID,
			record.ReasoningEffort,
		)
	}
	if cmd.Flags().Changed("access-mode") && cfg.AccessMode != record.AccessMode {
		return fmt.Errorf("--run-id %q must continue with persisted --access-mode %q", record.RunID, record.AccessMode)
	}
	if cmd.Flags().Changed("add-dir") &&
		!slices.Equal(core.NormalizeAddDirs(cfg.AddDirs), core.NormalizeAddDirs(record.AddDirs)) {
		return fmt.Errorf("--run-id %q must continue with persisted --add-dir values", record.RunID)
	}
	return nil
}

func (s *commandState) handleExecError(cmd *cobra.Command, err error) error {
	if err == nil {
		return nil
	}
	if isJSONOutputFormat(s.outputFormat) && !coreRun.IsExecErrorReported(err) {
		cmd.SilenceErrors = true
		if root := cmd.Root(); root != nil {
			root.SilenceErrors = true
		}
		if emitErr := coreRun.WriteExecJSONFailure(cmd.OutOrStdout(), strings.TrimSpace(s.runID), err); emitErr != nil {
			return errors.Join(err, emitErr)
		}
	}
	return err
}

func (s *commandState) resolveExecPromptSource(cmd *cobra.Command, args []string) error {
	promptFile := currentExecPromptFile(cmd, s.promptFile)
	s.promptText = ""
	s.promptFile = ""
	s.readPromptStdin = false
	s.resolvedPromptText = ""

	positionalPrompt := ""
	if len(args) == 1 && strings.TrimSpace(args[0]) != "" {
		positionalPrompt = args[0]
	}
	stdinPrompt, hasStdinPrompt, err := readPromptFromCommandInput(cmd.InOrStdin())
	if err != nil {
		return err
	}

	sourceCount := 0
	if positionalPrompt != "" {
		sourceCount++
	}
	if promptFile != "" {
		sourceCount++
	}
	if hasStdinPrompt {
		sourceCount++
	}

	if sourceCount > 1 {
		return fmt.Errorf(
			"%s accepts only one prompt source at a time: positional prompt, --prompt-file, or stdin",
			cmd.CommandPath(),
		)
	}

	switch {
	case positionalPrompt != "":
		s.promptText = positionalPrompt
		s.resolvedPromptText = positionalPrompt
		return nil
	case promptFile != "":
		content, err := os.ReadFile(promptFile)
		if err != nil {
			return fmt.Errorf("read prompt file %s: %w", promptFile, err)
		}
		if strings.TrimSpace(string(content)) == "" {
			return fmt.Errorf("prompt file %s is empty", promptFile)
		}
		s.promptFile = promptFile
		s.resolvedPromptText = string(content)
		return nil
	case hasStdinPrompt:
		s.readPromptStdin = true
		s.resolvedPromptText = stdinPrompt
		return nil
	default:
		return fmt.Errorf(
			"%s requires exactly one prompt source: positional prompt, --prompt-file, or non-empty stdin",
			cmd.CommandPath(),
		)
	}
}

func currentExecPromptFile(cmd *cobra.Command, raw string) string {
	if cmd == nil {
		return strings.TrimSpace(raw)
	}
	flag := cmd.Flags().Lookup("prompt-file")
	if flag == nil {
		return strings.TrimSpace(raw)
	}
	if !flag.Changed {
		return ""
	}
	return strings.TrimSpace(raw)
}

func readPromptFromCommandInput(reader io.Reader) (string, bool, error) {
	if reader == nil {
		return "", false, nil
	}

	if file, ok := reader.(*os.File); ok {
		info, err := file.Stat()
		if err != nil {
			return "", false, fmt.Errorf("inspect stdin: %w", err)
		}
		if info.Mode()&os.ModeCharDevice != 0 {
			return "", false, nil
		}
	}

	content, err := io.ReadAll(reader)
	if err != nil {
		return "", false, fmt.Errorf("read stdin prompt: %w", err)
	}
	if strings.TrimSpace(string(content)) == "" {
		return "", false, nil
	}
	return string(content), true, nil
}

func isJSONOutputFormat(value string) bool {
	switch strings.TrimSpace(value) {
	case string(core.OutputFormatJSON), string(core.OutputFormatRawJSON):
		return true
	default:
		return false
	}
}
