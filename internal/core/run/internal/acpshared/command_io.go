package acpshared

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/run/internal/runshared"
	"github.com/compozy/compozy/internal/core/run/internal/runtimeevents"
	"github.com/compozy/compozy/internal/core/run/journal"
	"github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

var newAgentClient = agent.NewClient

type runtimeEventSubmitter interface {
	Submit(context.Context, events.Event) error
}

func hasRuntimeEventSubmitter(submitter runtimeEventSubmitter) bool {
	switch typed := submitter.(type) {
	case nil:
		return false
	case *journal.Journal:
		return typed != nil
	default:
		return true
	}
}

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
	Activity      *activityMonitor
}

type SessionSetupRequest struct {
	Context           context.Context
	Config            *config
	Job               *job
	CWD               string
	UseUI             bool
	StreamHumanOutput bool
	Index             int
	RunJournal        runtimeEventSubmitter
	AggregateUsage    *model.Usage
	AggregateMu       *sync.Mutex
	Activity          *activityMonitor
	InitTimeout       time.Duration
	Logger            *slog.Logger
	TrackClient       func(agent.Client) func()
}

func (s *SessionExecution) Close() {
	s.emitProgressSignalDiagnostics()
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

// emitProgressSignalDiagnostics logs this execution's drop/backpressure signals
// once at close, reading the journal drop counters when the submitter exposes
// them.
func (s *SessionExecution) emitProgressSignalDiagnostics() {
	if s.Session == nil {
		return
	}
	logger := s.Logger
	if logger == nil {
		logger = silentLogger()
	}
	var jrnl stallDiagnosticsJournal
	if s.Handler != nil {
		if typed, ok := s.Handler.journal.(stallDiagnosticsJournal); ok {
			jrnl = typed
		}
	}
	logProgressSignalDiagnostics(logger, s.Session.ID(), s.Session, jrnl)
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

func SetupSessionExecution(req SessionSetupRequest) (*SessionExecution, error) {
	if req.Context == nil {
		req.Context = context.Background()
	}
	logger := resolveSessionLogger(req.Logger)
	setupCtx, cancelSetup := withACPInitDeadline(req.Context, req.InitTimeout)
	defer cancelSetup()

	outFile, errFile, err := createSessionLogFiles(req.Job)
	if err != nil {
		return nil, err
	}
	client, err := createACPClient(setupCtx, req.Config, req.Job, logger)
	if err != nil {
		err = withSetupContextCause(setupCtx, err)
		setupErr := setupFailureForUser(req.Config, req.Job, err)
		writeErr := writeSetupFailureToErrLog(errFile, setupErr)
		closeSetupLogFiles(outFile, errFile)
		return nil, joinSetupFailure(setupErr, writeErr)
	}
	releaseClient := func() {}
	if req.TrackClient != nil {
		releaseClient = req.TrackClient(client)
	}
	if err := emitReusableAgentSetupLifecycle(
		req.Context,
		req.RunJournal,
		req.Config.RunArtifacts.RunID,
		req.Job,
	); err != nil {
		logger.Warn("failed to emit reusable agent setup lifecycle; continuing", "error", err)
	}

	session, err := createACPSession(req.Context, setupCtx, client, req.Config, req.Job, req.CWD)
	if err != nil {
		err = withSetupContextCause(setupCtx, err)
		setupErr := setupFailureForUser(req.Config, req.Job, fmt.Errorf("create ACP session: %w", err))
		writeErr := writeSetupFailureToErrLog(errFile, setupErr)
		closeSetupLogFiles(outFile, errFile)
		_ = client.Close()
		releaseClient()
		return nil, joinSetupFailure(setupErr, writeErr)
	}

	execution := buildSessionExecution(
		req,
		sessionExecutionResources{
			client:        client,
			releaseClient: releaseClient,
			session:       session,
			outFile:       outFile,
			errFile:       errFile,
			logger:        logger,
		},
	)
	if err := emitSessionStartedEvent(
		req.Context,
		req.RunJournal,
		req.Config.RunArtifacts.RunID,
		req.Index,
		session.Identity(),
	); err != nil {
		writeErr := writeSetupFailureToErrLog(execution.ErrFile, err)
		execution.Close()
		return nil, joinSetupFailure(err, writeErr)
	}

	return execution, nil
}

func withACPInitDeadline(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeoutCause(ctx, timeout, NewInitTimeoutError(timeout))
}

func withSetupContextCause(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	cause := context.Cause(ctx)
	if cause == nil || errors.Is(err, cause) {
		return err
	}
	return errors.Join(cause, err)
}

type sessionExecutionResources struct {
	client        agent.Client
	releaseClient func()
	session       agent.Session
	outFile       *os.File
	errFile       *os.File
	logger        *slog.Logger
}

func buildSessionExecution(req SessionSetupRequest, resources sessionExecutionResources) *SessionExecution {
	outWriter, errWriter := createLogWriters(
		resources.outFile,
		resources.errFile,
		req.UseUI,
		req.StreamHumanOutput,
	)
	handler := NewSessionUpdateHandler(SessionUpdateHandlerConfig{
		Context:   req.Context,
		Index:     req.Index,
		AgentID:   jobIDE(req.Config, req.Job),
		JobID:     safeJobID(req.Job),
		SessionID: resources.session.ID(),
		Logger: resources.logger.With(
			"component",
			"acp.session",
			"agent_id",
			jobIDE(req.Config, req.Job),
			"session_id",
			resources.session.ID(),
		),
		RunID:          req.Config.RunArtifacts.RunID,
		OutWriter:      outWriter,
		ErrWriter:      errWriter,
		RunJournal:     req.RunJournal,
		RunManager:     req.Config.RuntimeManager,
		JobUsage:       &req.Job.Usage,
		AggregateUsage: req.AggregateUsage,
		AggregateMu:    req.AggregateMu,
		Activity:       req.Activity,
		ReusableAgent:  req.Job.ReusableAgent,
	})
	resources.logger.Info(
		"acp session created",
		"agent_id",
		jobIDE(req.Config, req.Job),
		"session_id",
		resources.session.ID(),
		"job_index",
		req.Index,
	)
	return &SessionExecution{
		Client:        resources.client,
		ReleaseClient: resources.releaseClient,
		Session:       resources.session,
		Handler:       handler,
		OutFile:       resources.outFile,
		ErrFile:       resources.errFile,
		Logger:        resources.logger,
		Activity:      req.Activity,
	}
}

func emitSessionStartedEvent(
	ctx context.Context,
	runJournal runtimeEventSubmitter,
	runID string,
	index int,
	identity agent.SessionIdentity,
) error {
	if !hasRuntimeEventSubmitter(runJournal) {
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

func createACPClient(ctx context.Context, cfg *config, job *job, logger *slog.Logger) (agent.Client, error) {
	ide := jobIDE(cfg, job)
	client, err := newAgentClient(ctx, agent.ClientConfig{
		IDE:                    ide,
		Model:                  jobModel(cfg, job),
		AddDirs:                append([]string(nil), cfg.AddDirs...),
		ReasoningEffort:        jobReasoningEffort(cfg, job),
		AccessMode:             cfg.AccessMode,
		Logger:                 logger.With("component", "acp.client", "agent_id", ide),
		ShutdownTimeout:        runshared.ProcessTerminationGracePeriod,
		TerminalCommandTimeout: cfg.Stall.TerminalCap,
	})
	if err != nil {
		return nil, fmt.Errorf("create ACP client: %w", err)
	}
	return client, nil
}

func setupFailureForUser(cfg *config, job *job, err error) error {
	if err == nil || !agent.IsAuthenticationRequired(err) {
		return err
	}
	runtimeID := strings.TrimSpace(jobIDE(cfg, job))
	command := firstNonEmpty(agent.RuntimeCommandName(runtimeID), runtimeID, "ACP runtime")
	return fmt.Errorf("%s is not authenticated. Run '%s login' and retry: %w", command, command, err)
}

func writeSetupFailureToErrLog(errFile *os.File, err error) error {
	if errFile == nil || err == nil {
		return nil
	}
	if _, writeErr := fmt.Fprintf(errFile, "ACP session setup error: %s\n", err.Error()); writeErr != nil {
		return fmt.Errorf("write ACP session setup error: %w", writeErr)
	}
	return nil
}

func closeSetupLogFiles(outFile *os.File, errFile *os.File) {
	if outFile != nil {
		_ = outFile.Close()
	}
	if errFile != nil {
		_ = errFile.Close()
	}
}

func joinSetupFailure(setupErr error, writeErr error) error {
	if writeErr == nil {
		return setupErr
	}
	return errors.Join(setupErr, writeErr)
}

func createACPSession(
	ctx context.Context,
	setupCtx context.Context,
	client agent.Client,
	cfg *config,
	job *job,
	cwd string,
) (agent.Session, error) {
	prompt := composeSessionPrompt(job.Prompt, job.SystemPrompt)
	modelName := jobModel(cfg, job)
	if strings.TrimSpace(job.ResumeSession) == "" {
		session, err := client.CreateSession(ctx, agent.SessionRequest{
			Prompt:       prompt,
			WorkingDir:   cwd,
			Model:        modelName,
			MCPServers:   model.CloneMCPServers(job.MCPServers),
			ExtraEnv:     buildSessionEnvironment(),
			SetupContext: setupCtx,
			RunID:        cfg.RunArtifacts.RunID,
			JobID:        safeJobID(job),
			RuntimeMgr:   cfg.RuntimeManager,
		})
		if err != nil {
			return nil, err
		}
		return session, nil
	}
	return client.ResumeSession(ctx, agent.ResumeSessionRequest{
		SessionID:    job.ResumeSession,
		Prompt:       prompt,
		WorkingDir:   cwd,
		Model:        modelName,
		MCPServers:   model.CloneMCPServers(job.MCPServers),
		ExtraEnv:     buildSessionEnvironment(),
		SetupContext: setupCtx,
		RunID:        cfg.RunArtifacts.RunID,
		JobID:        safeJobID(job),
		RuntimeMgr:   cfg.RuntimeManager,
	})
}

func jobIDE(cfg *config, job *job) string {
	if job != nil && strings.TrimSpace(job.IDE) != "" {
		return job.IDE
	}
	if cfg == nil {
		return ""
	}
	return cfg.IDE
}

func jobModel(cfg *config, job *job) string {
	if job != nil && strings.TrimSpace(job.Model) != "" {
		return job.Model
	}
	if cfg == nil {
		return ""
	}
	return cfg.Model
}

func jobReasoningEffort(cfg *config, job *job) string {
	if job != nil && strings.TrimSpace(job.ReasoningEffort) != "" {
		return job.ReasoningEffort
	}
	if cfg == nil {
		return ""
	}
	return cfg.ReasoningEffort
}

func createSessionLogFiles(job *job) (*os.File, *os.File, error) {
	outFile, err := CreateLogFile(job.OutLog)
	if err != nil {
		return nil, nil, fmt.Errorf("create out log: %w", err)
	}
	errFile, err := CreateLogFile(job.ErrLog)
	if err != nil {
		if outFile != nil {
			_ = outFile.Close()
		}
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

func safeJobID(job *job) string {
	if job == nil {
		return ""
	}
	return strings.TrimSpace(job.SafeName)
}
