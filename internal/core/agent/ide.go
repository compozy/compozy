package agent

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/prompt"
)

type spec struct {
	id               string
	displayName      string
	defaultModel     string
	supportsAddDirs  bool
	formatsJSON      bool
	shellPreviewFunc func(model string, addDirs []string, reasoning string) string
	commandFunc      func(
		ctx context.Context,
		model string,
		addDirs []string,
		reasoning string,
		systemPrompt string,
	) *exec.Cmd
}

var specs = map[string]spec{
	model.IDECodex: {
		id:               model.IDECodex,
		displayName:      "Codex",
		defaultModel:     model.DefaultCodexModel,
		supportsAddDirs:  true,
		formatsJSON:      false,
		shellPreviewFunc: buildCodexCommand,
		commandFunc:      codexCommand,
	},
	model.IDEClaude: {
		id:               model.IDEClaude,
		displayName:      "Claude",
		defaultModel:     model.DefaultClaudeModel,
		supportsAddDirs:  true,
		formatsJSON:      true,
		shellPreviewFunc: buildClaudeCommand,
		commandFunc:      claudeCommand,
	},
	model.IDEDroid: {
		id:              model.IDEDroid,
		displayName:     "Droid",
		defaultModel:    model.DefaultCodexModel,
		supportsAddDirs: false,
		formatsJSON:     true,
		shellPreviewFunc: func(model string, _ []string, reasoning string) string {
			return buildDroidCommand(model, reasoning)
		},
		commandFunc: func(ctx context.Context, model string, _ []string, reasoning string, _ string) *exec.Cmd {
			return droidCommand(ctx, model, reasoning)
		},
	},
	model.IDECursor: {
		id:              model.IDECursor,
		displayName:     "Cursor",
		defaultModel:    model.DefaultCursorModel,
		supportsAddDirs: false,
		formatsJSON:     true,
		shellPreviewFunc: func(model string, _ []string, reasoning string) string {
			return buildCursorCommand(model, reasoning)
		},
		commandFunc: func(ctx context.Context, model string, _ []string, reasoning string, _ string) *exec.Cmd {
			return cursorCommand(ctx, model, reasoning)
		},
	},
	model.IDEOpenCode: {
		id:              model.IDEOpenCode,
		displayName:     "OpenCode",
		defaultModel:    model.DefaultOpenCodeModel,
		supportsAddDirs: false,
		formatsJSON:     true,
		shellPreviewFunc: func(modelName string, _ []string, reasoning string) string {
			return buildOpenCodeCommand(modelName, reasoning)
		},
		commandFunc: func(ctx context.Context, modelName string, _ []string, reasoning string, _ string) *exec.Cmd {
			return openCodeCommand(ctx, modelName, reasoning)
		},
	},
	model.IDEPi: {
		id:              model.IDEPi,
		displayName:     "Pi",
		defaultModel:    model.DefaultPiModel,
		supportsAddDirs: false,
		formatsJSON:     true,
		shellPreviewFunc: func(modelName string, _ []string, reasoning string) string {
			return buildPiCommand(modelName, reasoning)
		},
		commandFunc: func(ctx context.Context, modelName string, _ []string, reasoning string, _ string) *exec.Cmd {
			return piCommand(ctx, modelName, reasoning)
		},
	},
}

func ValidateRuntimeConfig(cfg *model.RuntimeConfig) error {
	if cfg.Mode != model.ExecutionModePRReview && cfg.Mode != model.ExecutionModePRDTasks {
		return fmt.Errorf(
			"invalid --mode value %q: must be %q or %q",
			cfg.Mode,
			model.ModeCodeReview,
			model.ModePRDTasks,
		)
	}
	if _, ok := specs[cfg.IDE]; !ok {
		return fmt.Errorf(
			"invalid --ide value %q: must be %q, %q, %q, %q, %q, or %q",
			cfg.IDE,
			model.IDEClaude,
			model.IDECodex,
			model.IDEDroid,
			model.IDECursor,
			model.IDEOpenCode,
			model.IDEPi,
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

func EnsureAvailable(cfg *model.RuntimeConfig) error {
	if cfg.DryRun {
		return nil
	}
	if err := assertIDEExists(cfg.IDE); err != nil {
		return err
	}
	if err := assertExecSupported(cfg.IDE); err != nil {
		return err
	}
	return nil
}

func DisplayName(ide string) string {
	if spec, ok := specs[ide]; ok {
		return spec.displayName
	}
	return ""
}

func BuildShellCommandString(ide string, modelName string, addDirs []string, reasoningEffort string) string {
	spec, ok := specs[ide]
	if !ok {
		return ""
	}
	dirs := addDirs
	if !spec.supportsAddDirs {
		dirs = nil
	}
	return spec.shellPreviewFunc(modelName, dirs, reasoningEffort)
}

func Command(ctx context.Context, cfg *model.RuntimeConfig) *exec.Cmd {
	spec, ok := specs[cfg.IDE]
	if !ok {
		return nil
	}
	modelToUse := cfg.Model
	if modelToUse == "" {
		modelToUse = spec.defaultModel
	}
	dirs := cfg.AddDirs
	if !spec.supportsAddDirs {
		dirs = nil
	}
	return spec.commandFunc(ctx, modelToUse, dirs, cfg.ReasoningEffort, cfg.SystemPrompt)
}

func buildCodexCommand(modelName string, addDirs []string, reasoningEffort string) string {
	modelToUse := model.DefaultCodexModel
	if modelName != "" && modelName != model.DefaultCodexModel {
		modelToUse = modelName
	}
	args := []string{
		model.IDECodex,
		"--dangerously-bypass-approvals-and-sandbox",
		"-m", modelToUse,
		"-c", fmt.Sprintf("model_reasoning_effort=%s", reasoningEffort),
	}
	args = appendAddDirs(args, addDirs)
	args = append(args, "exec", "--json", "-")
	return formatShellCommand(args)
}

func buildClaudeCommand(modelName string, addDirs []string, reasoningEffort string) string {
	thinkPrompt := prompt.ClaudeReasoningPrompt(reasoningEffort)
	modelToUse := model.DefaultClaudeModel
	if modelName != "" && modelName != model.DefaultClaudeModel {
		modelToUse = modelName
	}
	args := []string{
		model.IDEClaude,
		"--print",
		"--output-format", "stream-json",
		"--verbose",
		"--model", modelToUse,
	}
	args = appendAddDirs(args, addDirs)
	args = append(
		args,
		"--dangerously-skip-permissions",
		"--permission-mode", "bypassPermissions",
		"--append-system-prompt", thinkPrompt,
	)
	return "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1 " + formatShellCommand(args)
}

func buildDroidCommand(modelName string, reasoningEffort string) string {
	base := fmt.Sprintf(
		"droid exec --skip-permissions-unsafe --reasoning-effort %s --output-format stream-json",
		reasoningEffort,
	)
	if modelName != "" && modelName != model.DefaultCodexModel {
		return fmt.Sprintf("%s --model %s", base, modelName)
	}
	if modelName == model.DefaultCodexModel {
		return fmt.Sprintf("%s --model %s", base, model.DefaultCodexModel)
	}
	return base
}

func buildCursorCommand(modelName string, _ string) string {
	modelToUse := model.DefaultCursorModel
	if modelName != "" && modelName != model.DefaultCursorModel {
		modelToUse = modelName
	}
	return fmt.Sprintf("cursor-agent --print --output-format stream-json --model %s", modelToUse)
}

func appendAddDirs(args []string, values []string) []string {
	for _, value := range normalizeAddDirs(values) {
		args = append(args, "--add-dir", value)
	}
	return args
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

func codexCommand(
	ctx context.Context,
	modelName string,
	addDirs []string,
	reasoning string,
	_ string,
) *exec.Cmd {
	args := []string{"--dangerously-bypass-approvals-and-sandbox"}
	if modelName != "" {
		args = append(args, "-m", modelName)
	}
	args = append(args, "-c", fmt.Sprintf("model_reasoning_effort=%s", reasoning))
	args = appendAddDirs(args, addDirs)
	args = append(args, "exec", "--json", "-")
	return exec.CommandContext(ctx, model.IDECodex, args...)
}

func claudeCommand(
	ctx context.Context,
	modelName string,
	addDirs []string,
	reasoning string,
	systemPrompt string,
) *exec.Cmd {
	reasoningPrompt := prompt.ClaudeReasoningPrompt(reasoning)
	teamDirective := "<critical>YOU SHOULD use a team of agents to handle " +
		"properly the job and avoid do workaround to get it done</critical>"
	systemPrompt = composeSystemPrompt(reasoningPrompt, teamDirective, systemPrompt)
	args := []string{
		"--print",
		"--output-format", "stream-json",
		"--verbose",
		"--model", modelName,
	}
	args = appendAddDirs(args, addDirs)
	args = append(
		args,
		"--permission-mode", "bypassPermissions",
		"--dangerously-skip-permissions",
		"--append-system-prompt", systemPrompt,
	)
	return exec.CommandContext(ctx, model.IDEClaude, args...)
}

func composeSystemPrompt(parts ...string) string {
	trimmed := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		trimmed = append(trimmed, part)
	}
	return strings.Join(trimmed, "\n\n")
}

func droidCommand(ctx context.Context, modelName, reasoning string) *exec.Cmd {
	droidArgs := []string{
		"exec",
		"--skip-permissions-unsafe",
		"--reasoning-effort", reasoning,
		"--output-format", "stream-json",
	}
	if modelName != "" {
		droidArgs = append(droidArgs, "--model", modelName)
	}
	return exec.CommandContext(ctx, model.IDEDroid, droidArgs...)
}

func cursorCommand(ctx context.Context, modelName, _ string) *exec.Cmd {
	cursorArgs := []string{
		"--print",
		"--output-format", "stream-json",
	}
	if modelName != "" {
		cursorArgs = append(cursorArgs, "--model", modelName)
	} else {
		cursorArgs = append(cursorArgs, "--model", model.DefaultCursorModel)
	}
	return exec.CommandContext(ctx, model.IDECursor, cursorArgs...)
}

func buildOpenCodeCommand(modelName string, reasoningEffort string) string {
	modelToUse := model.DefaultOpenCodeModel
	if modelName != "" && modelName != model.DefaultOpenCodeModel {
		modelToUse = modelName
	}
	args := []string{
		model.IDEOpenCode, "run",
		"--format", "json",
		"--variant", reasoningEffort,
		"--thinking",
		"-",
	}
	if modelToUse != "" {
		args = append(args, "--model", modelToUse)
	}
	return formatShellCommand(args)
}

func openCodeCommand(ctx context.Context, modelName, reasoning string) *exec.Cmd {
	args := []string{
		"run",
		"--format", "json",
		"--variant", reasoning,
		"--thinking",
		"-",
	}
	if modelName != "" {
		args = append(args, "--model", modelName)
	}
	return exec.CommandContext(ctx, model.IDEOpenCode, args...)
}

func buildPiCommand(modelName string, reasoningEffort string) string {
	modelToUse := model.DefaultPiModel
	if modelName != "" && modelName != model.DefaultPiModel {
		modelToUse = modelName
	}
	args := []string{
		model.IDEPi,
		"--print",
		"--mode", "json",
		"--thinking", reasoningEffort,
		"--no-session",
		"--model", modelToUse,
	}
	return formatShellCommand(args)
}

func piCommand(ctx context.Context, modelName, reasoning string) *exec.Cmd {
	args := []string{
		"--print",
		"--mode", "json",
		"--thinking", reasoning,
		"--no-session",
	}
	if modelName != "" {
		args = append(args, "--model", modelName)
	}
	return exec.CommandContext(ctx, model.IDEPi, args...)
}

func normalizeAddDirs(dirs []string) []string {
	if len(dirs) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(dirs))
	normalized := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		trimmed := strings.TrimSpace(dir)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func assertIDEExists(ide string) error {
	if _, err := exec.LookPath(ide); err != nil {
		return fmt.Errorf("%s CLI not found on PATH", ide)
	}
	return nil
}

func assertExecSupported(ide string) error {
	cmd := exec.CommandContext(context.Background(), ide, "--help")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s CLI does not appear to be properly installed or configured", ide)
	}
	return nil
}
