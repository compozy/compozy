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
	coreRun "github.com/compozy/compozy/internal/core/run"
	"github.com/compozy/compozy/internal/core/workspace"
	"github.com/compozy/compozy/internal/setup"
	"github.com/spf13/cobra"
)

type commandState struct {
	workspaceRoot          string
	projectConfig          workspace.ProjectConfig
	kind                   commandKind
	mode                   core.Mode
	pr                     string
	name                   string
	provider               string
	round                  int
	reviewsDir             string
	tasksDir               string
	dryRun                 bool
	autoCommit             bool
	concurrent             int
	batchSize              int
	ide                    string
	model                  string
	force                  bool
	skipValidation         bool
	addDirs                []string
	tailLines              int
	reasoningEffort        string
	accessMode             string
	outputFormat           string
	verbose                bool
	tui                    bool
	persist                bool
	runID                  string
	promptText             string
	promptFile             string
	readPromptStdin        bool
	resolvedPromptText     string
	includeCompleted       bool
	includeResolved        bool
	timeout                string
	maxRetries             int
	retryBackoffMultiplier float64
	isInteractive          func() bool
	collectForm            func(*cobra.Command, *commandState) error
	listBundledSkills      func() ([]setup.Skill, error)
	verifyBundledSkills    func(setup.VerifyConfig) (setup.VerifyResult, error)
	installBundledSkills   func(setup.InstallConfig) (*setup.Result, error)
	confirmSkillRefresh    func(*cobra.Command, skillRefreshPrompt) (bool, error)
	fetchReviewsFn         func(context.Context, core.Config) (*core.FetchResult, error)
	runWorkflow            func(context.Context, core.Config) error
}

type commandStateDefaults struct {
	isInteractive        func() bool
	collectForm          func(*cobra.Command, *commandState) error
	listBundledSkills    func() ([]setup.Skill, error)
	verifyBundledSkills  func(setup.VerifyConfig) (setup.VerifyResult, error)
	installBundledSkills func(setup.InstallConfig) (*setup.Result, error)
	confirmSkillRefresh  func(*cobra.Command, skillRefreshPrompt) (bool, error)
}

func defaultCommandStateDefaults() commandStateDefaults {
	return commandStateDefaults{
		isInteractive:        isInteractiveTerminal,
		collectForm:          collectFormParams,
		listBundledSkills:    setup.ListBundledSkills,
		verifyBundledSkills:  setup.VerifyBundledSkills,
		installBundledSkills: setup.InstallBundledSkills,
		confirmSkillRefresh:  confirmSkillRefreshPrompt,
	}
}

func (defaults commandStateDefaults) withFallbacks() commandStateDefaults {
	builtin := defaultCommandStateDefaults()
	if defaults.isInteractive == nil {
		defaults.isInteractive = builtin.isInteractive
	}
	if defaults.collectForm == nil {
		defaults.collectForm = builtin.collectForm
	}
	if defaults.listBundledSkills == nil {
		defaults.listBundledSkills = builtin.listBundledSkills
	}
	if defaults.verifyBundledSkills == nil {
		defaults.verifyBundledSkills = builtin.verifyBundledSkills
	}
	if defaults.installBundledSkills == nil {
		defaults.installBundledSkills = builtin.installBundledSkills
	}
	if defaults.confirmSkillRefresh == nil {
		defaults.confirmSkillRefresh = builtin.confirmSkillRefresh
	}
	return defaults
}

func newCommandState(kind commandKind, mode core.Mode) *commandState {
	return newCommandStateWithDefaults(kind, mode, defaultCommandStateDefaults())
}

func newCommandStateWithDefaults(kind commandKind, mode core.Mode, defaults commandStateDefaults) *commandState {
	defaults = defaults.withFallbacks()

	return &commandState{
		kind:                 kind,
		mode:                 mode,
		isInteractive:        defaults.isInteractive,
		collectForm:          defaults.collectForm,
		listBundledSkills:    defaults.listBundledSkills,
		verifyBundledSkills:  defaults.verifyBundledSkills,
		installBundledSkills: defaults.installBundledSkills,
		confirmSkillRefresh:  defaults.confirmSkillRefresh,
		fetchReviewsFn:       core.FetchReviews,
		runWorkflow:          core.Run,
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
		"ACP runtime to use: claude, codex, copilot, cursor-agent, droid, gemini, opencode, or pi "+
			"(requires the matching ACP adapter, ACP-capable CLI, or supported launcher such as npx)",
	)
	cmd.Flags().StringVar(
		&state.model,
		"model",
		"",
		"Model to use (per-IDE defaults: codex/droid=gpt-5.4, claude=opus, copilot=claude-sonnet-4.6, "+
			"cursor-agent=composer-1, opencode/pi=anthropic/claude-opus-4-6, gemini=gemini-2.5-pro)",
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
		"Reasoning effort for runtimes that support bootstrap reasoning flags, such as droid (low, medium, high, xhigh)",
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
		timeoutDuration = parsed
	}

	return core.Config{
		WorkspaceRoot:          s.workspaceRoot,
		Name:                   s.name,
		Round:                  s.round,
		Provider:               s.provider,
		PR:                     s.pr,
		ReviewsDir:             s.reviewsDir,
		TasksDir:               s.tasksDir,
		DryRun:                 s.dryRun,
		AutoCommit:             s.autoCommit,
		Concurrent:             s.concurrent,
		BatchSize:              s.batchSize,
		IDE:                    core.IDE(s.ide),
		Model:                  s.model,
		AddDirs:                core.NormalizeAddDirs(s.addDirs),
		TailLines:              s.tailLines,
		ReasoningEffort:        s.reasoningEffort,
		AccessMode:             s.accessMode,
		Mode:                   s.mode,
		OutputFormat:           core.OutputFormat(s.outputFormat),
		Verbose:                s.verbose,
		TUI:                    s.tui,
		Persist:                s.persist,
		RunID:                  s.runID,
		PromptText:             s.promptText,
		PromptFile:             s.promptFile,
		ReadPromptStdin:        s.readPromptStdin,
		ResolvedPromptText:     s.resolvedPromptText,
		IncludeCompleted:       s.includeCompleted,
		IncludeResolved:        s.includeResolved,
		Timeout:                timeoutDuration,
		MaxRetries:             s.maxRetries,
		RetryBackoffMultiplier: s.retryBackoffMultiplier,
	}, nil
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
	if isExecJSONOutputFormatFlag(s.outputFormat) && !coreRun.IsExecErrorReported(err) {
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
	s.promptText = ""
	s.readPromptStdin = false
	s.resolvedPromptText = ""

	positionalPrompt := ""
	if len(args) == 1 && strings.TrimSpace(args[0]) != "" {
		positionalPrompt = args[0]
	}
	promptFile := strings.TrimSpace(s.promptFile)

	sourceCount := 0
	if positionalPrompt != "" {
		sourceCount++
	}
	if promptFile != "" {
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
	default:
		stdinPrompt, hasStdinPrompt, err := readPromptFromCommandInput(cmd.InOrStdin())
		if err != nil {
			return err
		}
		if !hasStdinPrompt {
			return fmt.Errorf(
				"%s requires exactly one prompt source: positional prompt, --prompt-file, or non-empty stdin",
				cmd.CommandPath(),
			)
		}
		s.readPromptStdin = true
		s.resolvedPromptText = stdinPrompt
		return nil
	}
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

func isExecJSONOutputFormatFlag(value string) bool {
	switch strings.TrimSpace(value) {
	case string(core.OutputFormatJSON), string(core.OutputFormatRawJSON):
		return true
	default:
		return false
	}
}
