package run

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/compozy/looper/internal/looper/agent"
	"github.com/compozy/looper/internal/looper/model"
	"github.com/compozy/looper/internal/looper/prompt"
)

func notifyJobStart(useUI bool, uiCh chan uiMsg, index int, job *job, cfg *config) {
	if useUI {
		uiCh <- jobStartedMsg{Index: index}
		return
	}

	commandCfg := buildIDECommandConfig(cfg, job)
	shellCmd := agent.BuildShellCommandString(commandCfg)
	ideName := agent.DisplayName(cfg.ide)
	totalIssues := countTotalIssues(job)
	codeFileLabel := formatCodeFileLabel(job.codeFiles)
	fmt.Printf(
		"\n=== Running %s (non-interactive) for batch: %s (%d issues)\n$ %s\n",
		ideName,
		codeFileLabel,
		totalIssues,
		shellCmd,
	)
}

func countTotalIssues(job *job) int {
	total := 0
	for _, items := range job.groups {
		total += len(items)
	}
	return total
}

func formatCodeFileLabel(codeFiles []string) string {
	label := strings.Join(codeFiles, ", ")
	if len(codeFiles) > 1 {
		return fmt.Sprintf("%d files: %s", len(codeFiles), label)
	}
	return label
}

func createIDECommand(ctx context.Context, cfg *config, job *job) *exec.Cmd {
	return agent.Command(ctx, buildIDECommandConfig(cfg, job))
}

func buildIDECommandConfig(cfg *config, job *job) *model.RuntimeConfig {
	commandCfg := &model.RuntimeConfig{
		IDE:             cfg.ide,
		Model:           cfg.model,
		AddDirs:         cfg.addDirs,
		ReasoningEffort: cfg.reasoningEffort,
		SystemPrompt:    "",
	}
	if cfg.ide == model.IDEClaude {
		commandCfg.SystemPrompt = buildClaudeSystemPrompt(cfg.mode, job.safeName, cfg.signalPort, cfg.reasoningEffort)
	}
	return commandCfg
}

func buildClaudeSystemPrompt(
	mode model.ExecutionMode,
	jobID string,
	signalPort int,
	reasoningEffort string,
) string {
	sections := make([]string, 0, 2)
	if thinking := strings.TrimSpace(prompt.ClaudeReasoningPrompt(reasoningEffort)); thinking != "" {
		sections = append(sections, thinking)
	}
	sections = append(sections, prompt.BuildSystemPrompt(mode, jobID, signalPort))
	return strings.Join(sections, "\n\n")
}

func setupCommandIO(
	cmd *exec.Cmd,
	job *job,
	cwd string,
	useUI bool,
	uiCh chan uiMsg,
	index int,
	tailLines int,
	ideType string,
	aggregateUsage *TokenUsage,
	aggregateMu *sync.Mutex,
) (*os.File, *os.File, *activityMonitor, error) {
	configureCommandEnvironment(cmd, cwd, job.prompt, ideType)

	outFile, err := createLogFile(job.outLog)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create out log: %w", err)
	}
	errFile, err := createLogFile(job.errLog)
	if err != nil {
		outFile.Close()
		return nil, nil, nil, fmt.Errorf("create err log: %w", err)
	}

	monitor := newActivityMonitor()
	outTap, errTap := buildCommandTaps(
		outFile,
		errFile,
		tailLines,
		useUI,
		uiCh,
		index,
		ideType,
		aggregateUsage,
		aggregateMu,
		monitor,
	)
	cmd.Stdout = outTap
	cmd.Stderr = errTap
	return outFile, errFile, monitor, nil
}

func configureCommandEnvironment(cmd *exec.Cmd, cwd string, prompt []byte, ideType string) {
	cmd.Dir = cwd
	cmd.Stdin = bytes.NewReader(prompt)
	cmd.Env = append(os.Environ(),
		"FORCE_COLOR=1",
		"CLICOLOR_FORCE=1",
		"TERM=xterm-256color",
	)
	if ideType == model.IDEClaude {
		cmd.Env = append(cmd.Env, "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1")
	}
}
