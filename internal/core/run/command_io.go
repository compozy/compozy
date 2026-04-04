package run

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/model"
)

var newAgentClient = agent.NewClient

type sessionExecution struct {
	client        agent.Client
	releaseClient func()
	session       agent.Session
	handler       *sessionUpdateHandler
	outFile       *os.File
	errFile       *os.File
	logger        *slog.Logger
}

func (s *sessionExecution) close() {
	if s.releaseClient != nil {
		defer s.releaseClient()
	}
	if s.outFile != nil {
		_ = s.outFile.Close()
	}
	if s.errFile != nil {
		_ = s.errFile.Close()
	}
	if s.client != nil {
		if err := s.client.Close(); err != nil {
			s.logger.Warn("failed to close ACP client cleanly", "error", err)
		}
	}
}

func notifyJobStart(
	useUI bool,
	uiCh chan uiMsg,
	index int,
	attempt int,
	maxAttempts int,
	job *job,
	ide string,
	model string,
	addDirs []string,
	reasoningEffort string,
	accessMode string,
) {
	if useUI {
		uiCh <- jobStartedMsg{Index: index, Attempt: attempt, MaxAttempts: maxAttempts}
		return
	}

	shellCmd := agent.BuildShellCommandString(ide, model, addDirs, reasoningEffort, accessMode)
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

func setupSessionExecution(
	ctx context.Context,
	cfg *config,
	job *job,
	cwd string,
	useUI bool,
	uiCh chan uiMsg,
	index int,
	aggregateUsage *model.Usage,
	aggregateMu *sync.Mutex,
	activity *activityMonitor,
	logger *slog.Logger,
	trackClient func(agent.Client) func(),
) (*sessionExecution, error) {
	logger = resolveSessionLogger(logger, useUI)

	client, err := createACPClient(ctx, cfg, logger)
	if err != nil {
		return nil, err
	}
	releaseClient := func() {}
	if trackClient != nil {
		releaseClient = trackClient(client)
	}

	outFile, errFile, err := createSessionLogFiles(job)
	if err != nil {
		_ = client.Close()
		releaseClient()
		return nil, err
	}

	session, err := client.CreateSession(ctx, agent.SessionRequest{
		Prompt:     composeSessionPrompt(job.prompt, job.systemPrompt),
		WorkingDir: cwd,
		Model:      cfg.model,
		ExtraEnv:   buildSessionEnvironment(),
	})
	if err != nil {
		_ = outFile.Close()
		_ = errFile.Close()
		_ = client.Close()
		releaseClient()
		return nil, fmt.Errorf("create ACP session: %w", err)
	}

	outWriter, errWriter := createLogWriters(outFile, errFile, useUI)
	handler := newSessionUpdateHandler(
		index,
		cfg.ide,
		session.ID(),
		logger.With("component", "acp.session", "agent_id", cfg.ide, "session_id", session.ID()),
		outWriter,
		errWriter,
		uiCh,
		aggregateUsage,
		aggregateMu,
		activity,
	)
	logger.Info(
		"acp session created",
		"agent_id",
		cfg.ide,
		"session_id",
		session.ID(),
		"job_index",
		index,
	)

	return &sessionExecution{
		client:        client,
		releaseClient: releaseClient,
		session:       session,
		handler:       handler,
		outFile:       outFile,
		errFile:       errFile,
		logger:        logger,
	}, nil
}

func resolveSessionLogger(logger *slog.Logger, useUI bool) *slog.Logger {
	if logger != nil {
		return logger
	}
	return runtimeLogger(useUI)
}

func createACPClient(ctx context.Context, cfg *config, logger *slog.Logger) (agent.Client, error) {
	client, err := newAgentClient(ctx, agent.ClientConfig{
		IDE:             cfg.ide,
		Model:           cfg.model,
		AddDirs:         append([]string(nil), cfg.addDirs...),
		ReasoningEffort: cfg.reasoningEffort,
		AccessMode:      cfg.accessMode,
		Logger:          logger.With("component", "acp.client", "agent_id", cfg.ide),
		ShutdownTimeout: processTerminationGracePeriod,
	})
	if err != nil {
		return nil, fmt.Errorf("create ACP client: %w", err)
	}
	return client, nil
}

func createSessionLogFiles(job *job) (*os.File, *os.File, error) {
	outFile, err := createLogFile(job.outLog)
	if err != nil {
		return nil, nil, fmt.Errorf("create out log: %w", err)
	}
	errFile, err := createLogFile(job.errLog)
	if err != nil {
		_ = outFile.Close()
		return nil, nil, fmt.Errorf("create err log: %w", err)
	}
	return outFile, errFile, nil
}

func buildSessionEnvironment() map[string]string {
	return map[string]string{
		"FORCE_COLOR":    "1",
		"CLICOLOR_FORCE": "1",
		"TERM":           "xterm-256color",
	}
}

func composeSessionPrompt(prompt []byte, systemPrompt string) []byte {
	basePrompt := append([]byte(nil), prompt...)
	if strings.TrimSpace(systemPrompt) == "" {
		return basePrompt
	}

	combined := strings.TrimSpace(systemPrompt) + "\n\n" + string(basePrompt)
	return []byte(combined)
}
