package acpshared

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/run/internal/runshared"
	"github.com/compozy/compozy/internal/core/run/internal/runtimeevents"
	"github.com/compozy/compozy/internal/core/run/journal"
	"github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

var newAgentClient = agent.NewClient

func SwapNewAgentClientForTest(
	fn func(context.Context, agent.ClientConfig) (agent.Client, error),
) func() {
	previous := newAgentClient
	newAgentClient = fn
	return func() {
		newAgentClient = previous
	}
}

type SessionExecution struct {
	Client        agent.Client
	ReleaseClient func()
	Session       agent.Session
	Handler       *SessionUpdateHandler
	OutFile       *os.File
	ErrFile       *os.File
	Logger        *slog.Logger
}

func (s *SessionExecution) Close() {
	if s.ReleaseClient != nil {
		defer s.ReleaseClient()
	}
	if s.OutFile != nil {
		_ = s.OutFile.Close()
	}
	if s.ErrFile != nil {
		_ = s.ErrFile.Close()
	}
	if s.Client != nil {
		if err := s.Client.Close(); err != nil {
			s.Logger.Warn("failed to close ACP client cleanly", "error", err)
		}
	}
}

func NotifyJobStart(
	emitHuman bool,
	job *job,
	ide string,
	model string,
	addDirs []string,
	reasoningEffort string,
	accessMode string,
) {
	if !emitHuman {
		return
	}

	shellCmd := agent.BuildShellCommandString(ide, model, addDirs, reasoningEffort, accessMode)
	ideName := agent.DisplayName(ide)
	totalIssues := runshared.CountTotalIssues(job)
	codeFileLabel := job.CodeFileLabel()
	if len(job.CodeFiles) > 1 {
		codeFileLabel = fmt.Sprintf("%d files: %s", len(job.CodeFiles), codeFileLabel)
	}
	fmt.Printf(
		"\n=== Running %s (non-interactive) for batch: %s (%d issues)\n$ %s\n",
		ideName,
		codeFileLabel,
		totalIssues,
		shellCmd,
	)
}

func SetupSessionExecution(
	ctx context.Context,
	cfg *config,
	job *job,
	cwd string,
	useUI bool,
	streamHumanOutput bool,
	index int,
	runJournal *journal.Journal,
	aggregateUsage *model.Usage,
	aggregateMu *sync.Mutex,
	activity *activityMonitor,
	logger *slog.Logger,
	trackClient func(agent.Client) func(),
) (*SessionExecution, error) {
	logger = resolveSessionLogger(logger)

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

	session, err := createACPSession(ctx, client, cfg, job, cwd)
	if err != nil {
		_ = outFile.Close()
		_ = errFile.Close()
		_ = client.Close()
		releaseClient()
		return nil, fmt.Errorf("create ACP session: %w", err)
	}

	execution := buildSessionExecution(
		ctx,
		cfg,
		job,
		useUI,
		streamHumanOutput,
		index,
		runJournal,
		aggregateUsage,
		aggregateMu,
		activity,
		logger,
		client,
		releaseClient,
		session,
		outFile,
		errFile,
	)
	if err := emitSessionStartedEvent(ctx, runJournal, cfg.RunArtifacts.RunID, index, session.Identity()); err != nil {
		execution.Close()
		return nil, err
	}

	return execution, nil
}

func buildSessionExecution(
	ctx context.Context,
	cfg *config,
	job *job,
	useUI bool,
	streamHumanOutput bool,
	index int,
	runJournal *journal.Journal,
	aggregateUsage *model.Usage,
	aggregateMu *sync.Mutex,
	activity *activityMonitor,
	logger *slog.Logger,
	client agent.Client,
	releaseClient func(),
	session agent.Session,
	outFile *os.File,
	errFile *os.File,
) *SessionExecution {
	outWriter, errWriter := createLogWriters(outFile, errFile, useUI, streamHumanOutput)
	handler := NewSessionUpdateHandler(
		ctx,
		index,
		cfg.IDE,
		session.ID(),
		logger.With("component", "acp.session", "agent_id", cfg.IDE, "session_id", session.ID()),
		cfg.RunArtifacts.RunID,
		outWriter,
		errWriter,
		runJournal,
		&job.Usage,
		aggregateUsage,
		aggregateMu,
		activity,
	)
	logger.Info(
		"acp session created",
		"agent_id",
		cfg.IDE,
		"session_id",
		session.ID(),
		"job_index",
		index,
	)
	return &SessionExecution{
		Client:        client,
		ReleaseClient: releaseClient,
		Session:       session,
		Handler:       handler,
		OutFile:       outFile,
		ErrFile:       errFile,
		Logger:        logger,
	}
}

func emitSessionStartedEvent(
	ctx context.Context,
	runJournal *journal.Journal,
	runID string,
	index int,
	identity agent.SessionIdentity,
) error {
	if runJournal == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	event, err := runtimeevents.NewRuntimeEvent(
		runID,
		events.EventKindSessionStarted,
		kinds.SessionStartedPayload{
			Index:          index,
			ACPSessionID:   identity.ACPSessionID,
			AgentSessionID: identity.AgentSessionID,
			Resumed:        identity.Resumed,
		},
	)
	if err != nil {
		return err
	}
	if err := runJournal.Submit(ctx, event); err != nil {
		return fmt.Errorf("submit session started event: %w", err)
	}
	return nil
}

func resolveSessionLogger(logger *slog.Logger) *slog.Logger {
	if logger != nil {
		return logger
	}
	return runtimeLogger(false)
}

func createACPClient(ctx context.Context, cfg *config, logger *slog.Logger) (agent.Client, error) {
	client, err := newAgentClient(ctx, agent.ClientConfig{
		IDE:             cfg.IDE,
		Model:           cfg.Model,
		AddDirs:         append([]string(nil), cfg.AddDirs...),
		ReasoningEffort: cfg.ReasoningEffort,
		AccessMode:      cfg.AccessMode,
		Logger:          logger.With("component", "acp.client", "agent_id", cfg.IDE),
		ShutdownTimeout: runshared.ProcessTerminationGracePeriod,
	})
	if err != nil {
		return nil, fmt.Errorf("create ACP client: %w", err)
	}
	return client, nil
}

func createACPSession(
	ctx context.Context,
	client agent.Client,
	cfg *config,
	job *job,
	cwd string,
) (agent.Session, error) {
	prompt := composeSessionPrompt(job.Prompt, job.SystemPrompt)
	if strings.TrimSpace(job.ResumeSession) == "" {
		return client.CreateSession(ctx, agent.SessionRequest{
			Prompt:     prompt,
			WorkingDir: cwd,
			Model:      cfg.Model,
			ExtraEnv:   buildSessionEnvironment(),
		})
	}
	return client.ResumeSession(ctx, agent.ResumeSessionRequest{
		SessionID:  job.ResumeSession,
		Prompt:     prompt,
		WorkingDir: cwd,
		Model:      cfg.Model,
		ExtraEnv:   buildSessionEnvironment(),
	})
}

func createSessionLogFiles(job *job) (*os.File, *os.File, error) {
	outFile, err := CreateLogFile(job.OutLog)
	if err != nil {
		return nil, nil, fmt.Errorf("create out log: %w", err)
	}
	errFile, err := CreateLogFile(job.ErrLog)
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
