package acpshared

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"strings"
	"sync"

	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/run/internal/runshared"
	"github.com/compozy/compozy/internal/core/run/internal/runtimeevents"
	"github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

var newAgentClient = agent.NewClient

type runtimeEventSubmitter interface {
	Submit(context.Context, events.Event) error
}

func hasRuntimeEventSubmitter(submitter runtimeEventSubmitter) bool {
	if submitter == nil {
		return false
	}
	value := reflect.ValueOf(submitter)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return !value.IsNil()
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
	Logger            *slog.Logger
	TrackClient       func(agent.Client) func()
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

func SetupSessionExecution(req SessionSetupRequest) (*SessionExecution, error) {
	if req.Context == nil {
		req.Context = context.Background()
	}
	logger := resolveSessionLogger(req.Logger)

	client, err := createACPClient(req.Context, req.Config, req.Job, logger)
	if err != nil {
		return nil, err
	}
	releaseClient := func() {}
	if req.TrackClient != nil {
		releaseClient = req.TrackClient(client)
	}

	outFile, errFile, err := createSessionLogFiles(req.Job)
	if err != nil {
		_ = client.Close()
		releaseClient()
		return nil, err
	}
	if err := emitReusableAgentSetupLifecycle(
		req.Context,
		req.RunJournal,
		req.Config.RunArtifacts.RunID,
		req.Job,
	); err != nil {
		logger.Warn("failed to emit reusable agent setup lifecycle; continuing", "error", err)
	}

	session, err := createACPSession(req.Context, client, req.Config, req.Job, req.CWD)
	if err != nil {
		_ = outFile.Close()
		_ = errFile.Close()
		_ = client.Close()
		releaseClient()
		return nil, fmt.Errorf("create ACP session: %w", err)
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
		execution.Close()
		return nil, err
	}

	return execution, nil
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
		IDE:             ide,
		Model:           jobModel(cfg, job),
		AddDirs:         append([]string(nil), cfg.AddDirs...),
		ReasoningEffort: jobReasoningEffort(cfg, job),
		AccessMode:      cfg.AccessMode,
		Logger:          logger.With("component", "acp.client", "agent_id", ide),
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
			Model:      jobModel(cfg, job),
			MCPServers: model.CloneMCPServers(job.MCPServers),
			ExtraEnv:   buildSessionEnvironment(),
			RunID:      cfg.RunArtifacts.RunID,
			JobID:      safeJobID(job),
			RuntimeMgr: cfg.RuntimeManager,
		})
	}
	return client.ResumeSession(ctx, agent.ResumeSessionRequest{
		SessionID:  job.ResumeSession,
		Prompt:     prompt,
		WorkingDir: cwd,
		Model:      jobModel(cfg, job),
		MCPServers: model.CloneMCPServers(job.MCPServers),
		ExtraEnv:   buildSessionEnvironment(),
		RunID:      cfg.RunArtifacts.RunID,
		JobID:      safeJobID(job),
		RuntimeMgr: cfg.RuntimeManager,
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

func safeJobID(job *job) string {
	if job == nil {
		return ""
	}
	return strings.TrimSpace(job.SafeName)
}
