package run

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/model"
)

func notifyJobStart(
	useUI bool,
	uiCh chan uiMsg,
	index int,
	job *job,
	ide string,
	model string,
	addDirs []string,
	reasoningEffort string,
) {
	if useUI {
		uiCh <- jobStartedMsg{Index: index}
		return
	}

	shellCmd := agent.BuildShellCommandString(ide, model, addDirs, reasoningEffort)
	ideName := agent.DisplayName(ide)
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

func createIDECommand(ctx context.Context, cfg *config) *exec.Cmd {
	return agent.Command(ctx, &model.RuntimeConfig{
		IDE:             cfg.ide,
		Model:           cfg.model,
		AddDirs:         cfg.addDirs,
		ReasoningEffort: cfg.reasoningEffort,
	})
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
