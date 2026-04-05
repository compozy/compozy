package run

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/model"
)

const execRunSchemaVersion = 1

type execJSONStreamMode uint8

const (
	execJSONStreamDisabled execJSONStreamMode = iota
	execJSONStreamLean
	execJSONStreamRaw
)

// PersistedExecRun is the persisted run contract for resumable exec sessions.
type PersistedExecRun struct {
	Version              int         `json:"version"`
	Mode                 string      `json:"mode"`
	RunID                string      `json:"run_id"`
	Status               string      `json:"status"`
	WorkspaceRoot        string      `json:"workspace_root"`
	IDE                  string      `json:"ide"`
	Model                string      `json:"model"`
	ReasoningEffort      string      `json:"reasoning_effort"`
	AccessMode           string      `json:"access_mode"`
	AddDirs              []string    `json:"add_dirs,omitempty"`
	CreatedAt            time.Time   `json:"created_at"`
	UpdatedAt            time.Time   `json:"updated_at"`
	TurnCount            int         `json:"turn_count"`
	ACPSessionID         string      `json:"acp_session_id,omitempty"`
	AgentSessionID       string      `json:"agent_session_id,omitempty"`
	LoadSessionSupported bool        `json:"load_session_supported,omitempty"`
	Usage                model.Usage `json:"usage,omitempty"`
	LastError            string      `json:"last_error,omitempty"`
	EventsPath           string      `json:"events_path,omitempty"`
	TurnsDir             string      `json:"turns_dir,omitempty"`
}

type persistedExecTurn struct {
	Turn           int                 `json:"turn"`
	Status         string              `json:"status"`
	PromptPath     string              `json:"prompt_path,omitempty"`
	ResponsePath   string              `json:"response_path,omitempty"`
	ResultPath     string              `json:"result_path,omitempty"`
	StdoutLogPath  string              `json:"stdout_log_path,omitempty"`
	StderrLogPath  string              `json:"stderr_log_path,omitempty"`
	Usage          model.Usage         `json:"usage,omitempty"`
	Resumed        bool                `json:"resumed,omitempty"`
	ACPSessionID   string              `json:"acp_session_id,omitempty"`
	AgentSessionID string              `json:"agent_session_id,omitempty"`
	Error          string              `json:"error,omitempty"`
	DryRun         bool                `json:"dry_run,omitempty"`
	StartedAt      time.Time           `json:"started_at"`
	CompletedAt    time.Time           `json:"completed_at"`
	FinalSnapshot  SessionViewSnapshot `json:"final_snapshot,omitempty"`
}

type execEvent struct {
	Type    string               `json:"type"`
	RunID   string               `json:"run_id,omitempty"`
	Turn    int                  `json:"turn,omitempty"`
	Time    time.Time            `json:"time"`
	Status  string               `json:"status,omitempty"`
	DryRun  bool                 `json:"dry_run,omitempty"`
	Session *execEventSession    `json:"session,omitempty"`
	Update  *model.SessionUpdate `json:"update,omitempty"`
	Usage   model.Usage          `json:"usage,omitempty"`
	Output  string               `json:"output,omitempty"`
	Error   string               `json:"error,omitempty"`
}

type execEventSession struct {
	ACPSessionID   string `json:"acp_session_id"`
	AgentSessionID string `json:"agent_session_id,omitempty"`
	Resumed        bool   `json:"resumed,omitempty"`
}

type execTurnPaths struct {
	promptPath   string
	responsePath string
	resultPath   string
	stdoutLog    string
	stderrLog    string
}

type execRunState struct {
	record       PersistedExecRun
	runArtifacts model.RunArtifacts
	events       *execEventEmitter
	turn         int
	turnDir      string
	turnPaths    execTurnPaths
	emitText     bool
	cleanupDir   string
}

type execEventWriter struct {
	mu     sync.Mutex
	file   *os.File
	output io.Writer
	closed bool
}

type execEventEmitter struct {
	rawWriter    *execEventWriter
	stdoutWriter *execEventWriter
	stdoutMode   execJSONStreamMode
}

type execExecutionResult struct {
	status   string
	usage    model.Usage
	output   string
	dryRun   bool
	snapshot SessionViewSnapshot
	identity agent.SessionIdentity
	err      error
}

type execSetupErrorPayload struct {
	Type  string    `json:"type"`
	Time  time.Time `json:"time"`
	RunID string    `json:"run_id,omitempty"`
	Error string    `json:"error"`
}

type execReportedError struct {
	err error
}

func (e *execReportedError) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *execReportedError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

// LoadPersistedExecRun reads one persisted exec run from .compozy/runs/<run-id>/run.json.
func LoadPersistedExecRun(workspaceRoot, runID string) (PersistedExecRun, error) {
	runArtifacts := model.NewRunArtifacts(workspaceRoot, runID)
	payload, err := os.ReadFile(runArtifacts.RunMetaPath)
	if err != nil {
		return PersistedExecRun{}, fmt.Errorf("read persisted exec run: %w", err)
	}
	var record PersistedExecRun
	if err := json.Unmarshal(payload, &record); err != nil {
		return PersistedExecRun{}, fmt.Errorf("decode persisted exec run: %w", err)
	}
	if record.Mode != model.ModeExec {
		return PersistedExecRun{}, fmt.Errorf("run %q is not an exec run", runID)
	}
	if strings.TrimSpace(record.RunID) == "" {
		record.RunID = model.NewRunArtifacts(workspaceRoot, runID).RunID
	}
	if record.EventsPath == "" {
		record.EventsPath = runArtifacts.EventsPath
	}
	if record.TurnsDir == "" {
		record.TurnsDir = runArtifacts.TurnsDir
	}
	return record, nil
}

// WriteExecJSONFailure emits a single JSON failure object to stdout/stderr-neutral writers.
func WriteExecJSONFailure(dst io.Writer, runID string, err error) error {
	if dst == nil || err == nil {
		return nil
	}
	payload := execSetupErrorPayload{
		Type:  "run.failed",
		Time:  time.Now().UTC(),
		RunID: strings.TrimSpace(runID),
		Error: err.Error(),
	}
	return json.NewEncoder(dst).Encode(payload)
}

// IsExecErrorReported returns true when a failed exec already emitted its JSON failure payload.
func IsExecErrorReported(err error) bool {
	var reported *execReportedError
	return errors.As(err, &reported)
}

// ExecuteExec runs one headless-or-TUI exec turn with optional persistence and ACP resume.
func ExecuteExec(ctx context.Context, cfg *model.RuntimeConfig) error {
	promptText, state, internalCfg, execJob, err := prepareExecExecution(cfg)
	if err != nil {
		return err
	}
	defer state.close()
	if cfg.DryRun {
		return state.completeDryRun(promptText)
	}

	useUI := cfg.TUI
	ui, uiCh := setupExecUI(ctx, internalCfg, useUI, execJob)
	result := executeExecJob(ctx, internalCfg, &execJob, cfg.WorkspaceRoot, useUI, uiCh, state)
	if waitErr := waitExecUI(ui); waitErr != nil && result.err == nil {
		result.status = runStatusFailed
		result.err = waitErr
	}
	return finalizeExecResult(state, result)
}

func prepareExecExecution(cfg *model.RuntimeConfig) (string, *execRunState, *config, job, error) {
	promptText, err := resolveExecPromptText(cfg)
	if err != nil {
		return "", nil, nil, job{}, err
	}
	if err := agent.EnsureAvailable(cfg); err != nil {
		return "", nil, nil, job{}, err
	}
	state, err := prepareExecRunState(cfg)
	if err != nil {
		return "", nil, nil, job{}, err
	}
	if err := validateExecResumeCompatibility(cfg, state.record); err != nil {
		state.close()
		return "", nil, nil, job{}, err
	}
	if err := state.writeStarted(cfg); err != nil {
		state.close()
		return "", nil, nil, job{}, err
	}
	internalCfg := newConfig(cfg, state.runArtifacts)
	execJob, err := newExecRuntimeJob(promptText, state)
	if err != nil {
		state.close()
		return "", nil, nil, job{}, err
	}
	return promptText, state, internalCfg, execJob, nil
}

func setupExecUI(ctx context.Context, cfg *config, enabled bool, execJob job) (uiSession, chan uiMsg) {
	if !enabled {
		return nil, nil
	}
	ui := setupUI(ctx, []job{execJob}, cfg, true)
	if ui == nil {
		return nil, nil
	}
	return ui, ui.events()
}

func waitExecUI(ui uiSession) error {
	if ui == nil {
		return nil
	}
	ui.closeEvents()
	return ui.wait()
}

func finalizeExecResult(state *execRunState, result execExecutionResult) error {
	if result.err != nil {
		if emitErr := state.completeTurn(result); emitErr != nil && !errors.Is(emitErr, result.err) {
			return &execReportedError{err: errors.Join(result.err, emitErr)}
		}
		return &execReportedError{err: result.err}
	}
	return state.completeTurn(result)
}

func executeExecJob(
	ctx context.Context,
	cfg *config,
	j *job,
	cwd string,
	useUI bool,
	uiCh chan uiMsg,
	state *execRunState,
) execExecutionResult {
	notifyJobStart(
		useUI,
		false,
		uiCh,
		0,
		1,
		atLeastOne(cfg.maxRetries+1),
		j,
		cfg.ide,
		cfg.model,
		cfg.addDirs,
		cfg.reasoningEffort,
		cfg.accessMode,
	)

	attemptTimeout := cfg.timeout
	for attempt := 1; ; attempt++ {
		result := runSingleExecAttempt(ctx, cfg, j, cwd, useUI, uiCh, attemptTimeout, state)
		if result.err == nil {
			publishExecFinish(useUI, uiCh, true, 0)
			return result
		}

		if !shouldRetryExecAttempt(result.err, attempt, cfg.maxRetries, j) {
			publishExecFinish(useUI, uiCh, false, sessionErrorCode(result.err))
			return result
		}

		publishExecRetry(useUI, uiCh, attempt+1, cfg.maxRetries+1, result.err)
		attemptTimeout = nextRetryTimeout(attemptTimeout, cfg.retryBackoffMultiplier)
	}
}

func publishExecFinish(useUI bool, uiCh chan uiMsg, success bool, exitCode int) {
	if !useUI || uiCh == nil {
		return
	}
	uiCh <- jobFinishedMsg{Index: 0, Success: success, ExitCode: exitCode}
}

func publishExecRetry(useUI bool, uiCh chan uiMsg, attempt int, maxAttempts int, err error) {
	if !useUI || uiCh == nil || err == nil {
		return
	}
	uiCh <- jobRetryMsg{
		Index:       0,
		Attempt:     attempt,
		MaxAttempts: maxAttempts,
		Reason:      err.Error(),
	}
}

func shouldRetryExecAttempt(err error, attempt int, maxRetries int, j *job) bool {
	if j != nil && strings.TrimSpace(j.resumeSession) != "" {
		return false
	}
	return isExecRetryableError(err) && attempt <= maxRetries
}

func runSingleExecAttempt(
	ctx context.Context,
	cfg *config,
	j *job,
	cwd string,
	useUI bool,
	uiCh chan uiMsg,
	timeout time.Duration,
	state *execRunState,
) execExecutionResult {
	attemptCtx := ctx
	cancel := func(error) {}
	stopActivityWatchdog := func() {}
	var activity *activityMonitor
	if timeout > 0 {
		activity = newActivityMonitor()
		attemptCtx, cancel = context.WithCancelCause(ctx)
		stopActivityWatchdog = startACPActivityWatchdog(attemptCtx, activity, timeout, cancel)
	}
	defer func() {
		stopActivityWatchdog()
		cancel(nil)
	}()

	execution, err := setupSessionExecution(
		attemptCtx,
		cfg,
		j,
		cwd,
		useUI,
		false,
		uiCh,
		0,
		nil,
		nil,
		activity,
		runtimeLoggerFor(cfg, useUI),
		nil,
	)
	if err != nil {
		return execExecutionResult{status: runStatusFailed, err: err}
	}
	defer execution.close()
	if state != nil {
		state.record.LoadSessionSupported = execution.client.SupportsLoadSession()
	}

	identity := execution.session.Identity()
	if state != nil {
		if emitErr := state.emitSessionAttached(identity); emitErr != nil {
			return execExecutionResult{status: runStatusFailed, err: emitErr}
		}
	}
	streamErrCh := streamExecSession(execution, state)

	select {
	case <-execution.session.Done():
		return completeFinishedExecAttempt(execution, j, streamErrCh)
	case <-attemptCtx.Done():
		cancelErr := context.Cause(attemptCtx)
		if cancelErr == nil {
			cancelErr = attemptCtx.Err()
		}
		return failExecAttempt(execution, cancelErr)
	}
}

func streamExecSession(execution *sessionExecution, state *execRunState) <-chan error {
	streamErrCh := make(chan error, 1)
	go func() {
		for update := range execution.session.Updates() {
			if err := execution.handler.HandleUpdate(update); err != nil {
				streamErrCh <- err
				return
			}
			if state != nil {
				if err := state.emitSessionUpdate(update); err != nil {
					streamErrCh <- err
					return
				}
			}
		}
		streamErrCh <- nil
	}()
	return streamErrCh
}

func completeFinishedExecAttempt(
	execution *sessionExecution,
	j *job,
	streamErrCh <-chan error,
) execExecutionResult {
	streamErr := <-streamErrCh
	if streamErr != nil {
		return failExecAttempt(execution, streamErr)
	}
	sessionErr := execution.session.Err()
	if sessionErr != nil {
		return failExecAttempt(execution, sessionErr)
	}
	snapshot := execution.handler.Snapshot()
	if completionErr := execution.handler.HandleCompletion(nil); completionErr != nil {
		return failExecAttempt(execution, completionErr)
	}
	return execExecutionResult{
		status:   runStatusSucceeded,
		usage:    j.usage,
		output:   renderAssistantOutput(snapshot),
		snapshot: snapshot,
		identity: execution.session.Identity(),
	}
}

func failExecAttempt(execution *sessionExecution, err error) execExecutionResult {
	if execution == nil || execution.handler == nil || execution.session == nil {
		return execExecutionResult{status: runStatusFailed, err: err}
	}
	if completionErr := execution.handler.HandleCompletion(
		err,
	); completionErr != nil &&
		!errors.Is(completionErr, err) {
		err = errors.Join(err, completionErr)
	}
	return execExecutionResult{
		status:   runStatusFailed,
		snapshot: execution.handler.Snapshot(),
		identity: execution.session.Identity(),
		err:      err,
	}
}

func prepareExecRunState(cfg *model.RuntimeConfig) (*execRunState, error) {
	record, runID, err := resolvePersistedExecRecord(cfg)
	if err != nil {
		return nil, err
	}
	resolvedModel, err := agent.ResolveRuntimeModel(cfg.IDE, cfg.Model)
	if err != nil {
		return nil, err
	}
	state := &execRunState{
		record:   record,
		turn:     atLeastOne(record.TurnCount + 1),
		emitText: cfg.OutputFormat == model.OutputFormatText && !cfg.TUI,
	}
	if !cfg.Persist {
		return prepareEphemeralExecRunState(state, cfg, runID)
	}
	if err := preparePersistentExecRunState(state, cfg, runID, resolvedModel); err != nil {
		return nil, err
	}
	return state, nil
}

func resolvePersistedExecRecord(cfg *model.RuntimeConfig) (PersistedExecRun, string, error) {
	runID := strings.TrimSpace(cfg.RunID)
	if runID == "" {
		return PersistedExecRun{}, buildExecRunID(), nil
	}
	record, err := LoadPersistedExecRun(cfg.WorkspaceRoot, runID)
	if err != nil {
		return PersistedExecRun{}, "", err
	}
	cfg.Persist = true
	return record, runID, nil
}

func prepareEphemeralExecRunState(
	state *execRunState,
	cfg *model.RuntimeConfig,
	runID string,
) (*execRunState, error) {
	tempDir, err := os.MkdirTemp("", "compozy-exec-*")
	if err != nil {
		return nil, fmt.Errorf("create exec temp dir: %w", err)
	}
	state.cleanupDir = tempDir
	state.runArtifacts = model.NewRunArtifacts(tempDir, runID)
	state.events = newExecEventEmitter(nil, execJSONStdoutMode(cfg.OutputFormat), os.Stdout)
	return state, nil
}

func preparePersistentExecRunState(
	state *execRunState,
	cfg *model.RuntimeConfig,
	runID string,
	resolvedModel string,
) error {
	state.runArtifacts = model.NewRunArtifacts(cfg.WorkspaceRoot, runID)
	if err := ensureExecRunDirectories(state); err != nil {
		return err
	}
	eventFile, err := os.OpenFile(state.runArtifacts.EventsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open exec events: %w", err)
	}
	state.events = newExecEventEmitter(eventFile, execJSONStdoutMode(cfg.OutputFormat), os.Stdout)
	if strings.TrimSpace(state.record.RunID) == "" {
		state.record = newPersistedExecRunRecord(cfg, state.runArtifacts, runID, resolvedModel)
	}
	return nil
}

func ensureExecRunDirectories(state *execRunState) error {
	if err := os.MkdirAll(state.runArtifacts.RunDir, 0o755); err != nil {
		return fmt.Errorf("mkdir exec run dir: %w", err)
	}
	if err := os.MkdirAll(state.runArtifacts.TurnsDir, 0o755); err != nil {
		return fmt.Errorf("mkdir exec turns dir: %w", err)
	}
	turnDir := filepath.Join(state.runArtifacts.TurnsDir, fmt.Sprintf("%04d", state.turn))
	if err := os.MkdirAll(turnDir, 0o755); err != nil {
		return fmt.Errorf("mkdir exec turn dir: %w", err)
	}
	state.turnDir = turnDir
	state.turnPaths = execTurnPaths{
		promptPath:   filepath.Join(turnDir, "prompt.md"),
		responsePath: filepath.Join(turnDir, "response.txt"),
		resultPath:   filepath.Join(turnDir, "result.json"),
		stdoutLog:    filepath.Join(turnDir, "stdout.log"),
		stderrLog:    filepath.Join(turnDir, "stderr.log"),
	}
	return nil
}

func newPersistedExecRunRecord(
	cfg *model.RuntimeConfig,
	runArtifacts model.RunArtifacts,
	runID string,
	resolvedModel string,
) PersistedExecRun {
	return PersistedExecRun{
		Version:         execRunSchemaVersion,
		Mode:            model.ModeExec,
		RunID:           runID,
		WorkspaceRoot:   cfg.WorkspaceRoot,
		IDE:             cfg.IDE,
		Model:           resolvedModel,
		ReasoningEffort: cfg.ReasoningEffort,
		AccessMode:      cfg.AccessMode,
		AddDirs:         append([]string(nil), cfg.AddDirs...),
		CreatedAt:       time.Now().UTC(),
		EventsPath:      runArtifacts.EventsPath,
		TurnsDir:        runArtifacts.TurnsDir,
	}
}

func (s *execRunState) close() {
	if s == nil {
		return
	}
	if s.events != nil {
		_ = s.events.Close()
	}
	if strings.TrimSpace(s.cleanupDir) != "" {
		_ = os.RemoveAll(s.cleanupDir)
	}
}

func (s *execRunState) writeStarted(cfg *model.RuntimeConfig) error {
	if s == nil {
		return nil
	}
	s.record.UpdatedAt = time.Now().UTC()
	if cfg.Persist {
		s.record.Status = "running"
		if err := s.writeRecord(); err != nil {
			return err
		}
	}
	return s.emit(execEvent{
		Type:   "run.started",
		RunID:  s.runArtifacts.RunID,
		Turn:   s.turn,
		Time:   time.Now().UTC(),
		Status: "running",
		DryRun: cfg.DryRun,
	})
}

func (s *execRunState) completeDryRun(promptText string) error {
	if strings.TrimSpace(s.turnPaths.promptPath) != "" {
		if err := os.WriteFile(s.turnPaths.promptPath, []byte(promptText), 0o600); err != nil {
			return fmt.Errorf("write exec prompt: %w", err)
		}
	}
	result := execExecutionResult{
		status: runStatusSucceeded,
		output: promptText,
		dryRun: true,
	}
	if err := s.completeTurn(result); err != nil {
		return err
	}
	return nil
}

func (s *execRunState) completeTurn(result execExecutionResult) error {
	if s == nil {
		return nil
	}
	now := time.Now().UTC()
	turnRecord := s.buildTurnRecord(result, now)
	if err := s.writeTurnArtifacts(turnRecord, result.output); err != nil {
		return err
	}
	s.applyTurnResult(result, now)
	if err := s.persistTurnResult(); err != nil {
		return err
	}
	if err := s.emitTurnResult(result, now); err != nil {
		return err
	}
	if err := s.writeTextOutput(result); err != nil {
		return err
	}
	return result.err
}

func (s *execRunState) buildTurnRecord(result execExecutionResult, completedAt time.Time) persistedExecTurn {
	record := persistedExecTurn{
		Turn:           s.turn,
		Status:         result.status,
		PromptPath:     s.turnPaths.promptPath,
		ResponsePath:   s.turnPaths.responsePath,
		ResultPath:     s.turnPaths.resultPath,
		StdoutLogPath:  s.turnPaths.stdoutLog,
		StderrLogPath:  s.turnPaths.stderrLog,
		Usage:          result.usage,
		Resumed:        result.identity.Resumed,
		ACPSessionID:   result.identity.ACPSessionID,
		AgentSessionID: result.identity.AgentSessionID,
		Error:          errorString(result.err),
		DryRun:         result.dryRun,
		StartedAt:      s.record.UpdatedAt,
		CompletedAt:    completedAt,
		FinalSnapshot:  result.snapshot,
	}
	return record
}

func (s *execRunState) writeTurnArtifacts(turnRecord persistedExecTurn, output string) error {
	if strings.TrimSpace(s.turnPaths.responsePath) != "" {
		if err := os.WriteFile(s.turnPaths.responsePath, []byte(output), 0o600); err != nil {
			return fmt.Errorf("write exec response: %w", err)
		}
	}
	if strings.TrimSpace(s.turnPaths.resultPath) == "" {
		return nil
	}
	payload, err := json.MarshalIndent(turnRecord, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal exec turn result: %w", err)
	}
	if err := os.WriteFile(s.turnPaths.resultPath, payload, 0o600); err != nil {
		return fmt.Errorf("write exec turn result: %w", err)
	}
	return nil
}

func (s *execRunState) applyTurnResult(result execExecutionResult, completedAt time.Time) {
	s.record.Status = result.status
	s.record.TurnCount = s.turn
	s.record.UpdatedAt = completedAt
	s.record.ACPSessionID = result.identity.ACPSessionID
	if strings.TrimSpace(result.identity.AgentSessionID) != "" {
		s.record.AgentSessionID = result.identity.AgentSessionID
	}
	s.record.Usage.Add(result.usage)
	s.record.LastError = errorString(result.err)
}

func (s *execRunState) persistTurnResult() error {
	if strings.TrimSpace(s.turnPaths.promptPath) == "" || strings.TrimSpace(s.runArtifacts.RunDir) == "" {
		return nil
	}
	return s.writeRecord()
}

func (s *execRunState) emitTurnResult(result execExecutionResult, completedAt time.Time) error {
	return s.emit(execEvent{
		Type:   "run." + result.status,
		RunID:  s.runArtifacts.RunID,
		Turn:   s.turn,
		Time:   completedAt,
		Status: result.status,
		Usage:  result.usage,
		Output: result.output,
		Error:  errorString(result.err),
	})
}

func (s *execRunState) writeTextOutput(result execExecutionResult) error {
	if result.err != nil || !s.emitText || strings.TrimSpace(result.output) == "" {
		return nil
	}
	if _, err := fmt.Fprintln(os.Stdout, result.output); err != nil {
		return fmt.Errorf("write exec stdout: %w", err)
	}
	return nil
}

func (s *execRunState) emitSessionAttached(identity agent.SessionIdentity) error {
	if s == nil {
		return nil
	}
	s.record.ACPSessionID = identity.ACPSessionID
	if strings.TrimSpace(identity.AgentSessionID) != "" {
		s.record.AgentSessionID = identity.AgentSessionID
	}
	if s.turnPaths.promptPath != "" {
		if err := s.writeRecord(); err != nil {
			return err
		}
	}
	return s.emit(execEvent{
		Type:  "session.attached",
		RunID: s.runArtifacts.RunID,
		Turn:  s.turn,
		Time:  time.Now().UTC(),
		Session: &execEventSession{
			ACPSessionID:   identity.ACPSessionID,
			AgentSessionID: identity.AgentSessionID,
			Resumed:        identity.Resumed,
		},
	})
}

func (s *execRunState) emitSessionUpdate(update model.SessionUpdate) error {
	return s.emit(execEvent{
		Type:   "session.update",
		RunID:  s.runArtifacts.RunID,
		Turn:   s.turn,
		Time:   time.Now().UTC(),
		Update: &update,
		Usage:  update.Usage,
	})
}

func (s *execRunState) emit(event execEvent) error {
	if s == nil || s.events == nil {
		if strings.TrimSpace(event.Output) == "" || event.Type != "run.failed" {
			return nil
		}
		return nil
	}
	return s.events.Write(event)
}

func (s *execRunState) writeRecord() error {
	if s == nil || strings.TrimSpace(s.runArtifacts.RunMetaPath) == "" {
		return nil
	}
	payload, err := json.MarshalIndent(s.record, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal exec run record: %w", err)
	}
	if err := os.WriteFile(s.runArtifacts.RunMetaPath, payload, 0o600); err != nil {
		return fmt.Errorf("write exec run record: %w", err)
	}
	return nil
}

func (w *execEventWriter) Write(event execEvent) error {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal exec event: %w", err)
	}
	payload = append(payload, '\n')
	if w.file != nil {
		if _, err := w.file.Write(payload); err != nil {
			return fmt.Errorf("write exec events file: %w", err)
		}
	}
	if w.output != nil {
		if _, err := w.output.Write(payload); err != nil {
			return fmt.Errorf("write exec stdout event: %w", err)
		}
	}
	return nil
}

func (w *execEventWriter) Close() error {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed = true
	if w.file != nil {
		return w.file.Close()
	}
	return nil
}

func newExecEventEmitter(eventFile *os.File, stdoutMode execJSONStreamMode, stdout io.Writer) *execEventEmitter {
	emitter := &execEventEmitter{stdoutMode: stdoutMode}
	if eventFile != nil {
		emitter.rawWriter = &execEventWriter{file: eventFile}
	}
	if stdoutMode != execJSONStreamDisabled && stdout != nil {
		emitter.stdoutWriter = &execEventWriter{output: stdout}
	}
	if emitter.rawWriter == nil && emitter.stdoutWriter == nil {
		return nil
	}
	return emitter
}

func (e *execEventEmitter) Write(event execEvent) error {
	if e == nil {
		return nil
	}
	if e.rawWriter != nil {
		if err := e.rawWriter.Write(event); err != nil {
			return err
		}
	}
	if e.stdoutWriter == nil || !shouldEmitExecStdoutEvent(e.stdoutMode, event) {
		return nil
	}
	return e.stdoutWriter.Write(event)
}

func (e *execEventEmitter) Close() error {
	if e == nil {
		return nil
	}
	var err error
	if e.rawWriter != nil {
		err = errors.Join(err, e.rawWriter.Close())
	}
	if e.stdoutWriter != nil {
		err = errors.Join(err, e.stdoutWriter.Close())
	}
	return err
}

func execJSONStdoutMode(format model.OutputFormat) execJSONStreamMode {
	switch format {
	case model.OutputFormatJSON:
		return execJSONStreamLean
	case model.OutputFormatRawJSON:
		return execJSONStreamRaw
	default:
		return execJSONStreamDisabled
	}
}

func shouldEmitExecStdoutEvent(mode execJSONStreamMode, event execEvent) bool {
	switch mode {
	case execJSONStreamRaw:
		return true
	case execJSONStreamLean:
		return shouldEmitLeanExecEvent(event)
	default:
		return false
	}
}

func shouldEmitLeanExecEvent(event execEvent) bool {
	switch event.Type {
	case "run.started", "session.attached", "run.succeeded", "run.failed":
		return true
	case "session.update":
		return shouldEmitLeanSessionUpdate(event.Update)
	default:
		return false
	}
}

func shouldEmitLeanSessionUpdate(update *model.SessionUpdate) bool {
	if update == nil {
		return false
	}
	switch update.Kind {
	case model.UpdateKindUserMessageChunk,
		model.UpdateKindAgentMessageChunk,
		model.UpdateKindToolCallStarted,
		model.UpdateKindToolCallUpdated:
		return true
	case model.UpdateKindUnknown:
		return update.Status == model.StatusCompleted || update.Status == model.StatusFailed
	default:
		return false
	}
}

func newExecRuntimeJob(promptText string, state *execRunState) (job, error) {
	jb := job{
		codeFiles: []string{"exec"},
		groups: map[string][]model.IssueEntry{
			"exec": {{
				Name:     "exec",
				Content:  promptText,
				CodeFile: "exec",
			}},
		},
		safeName:  "exec",
		prompt:    []byte(promptText),
		outBuffer: newLineBuffer(0),
		errBuffer: newLineBuffer(0),
	}
	if state == nil {
		return jb, nil
	}
	jb.outPromptPath = state.turnPaths.promptPath
	jb.outLog = state.turnPaths.stdoutLog
	jb.errLog = state.turnPaths.stderrLog
	jb.resumeRunID = state.record.RunID
	jb.resumeSession = state.record.ACPSessionID
	if strings.TrimSpace(state.turnPaths.promptPath) != "" {
		if err := os.WriteFile(state.turnPaths.promptPath, []byte(promptText), 0o600); err != nil {
			return job{}, fmt.Errorf("write exec prompt: %w", err)
		}
	}
	return jb, nil
}

func resolveExecPromptText(cfg *model.RuntimeConfig) (string, error) {
	switch {
	case strings.TrimSpace(cfg.ResolvedPromptText) != "":
		return cfg.ResolvedPromptText, nil
	case strings.TrimSpace(cfg.PromptText) != "":
		return cfg.PromptText, nil
	case strings.TrimSpace(cfg.PromptFile) != "":
		content, err := os.ReadFile(cfg.PromptFile)
		if err != nil {
			return "", fmt.Errorf("read prompt file %s: %w", cfg.PromptFile, err)
		}
		if strings.TrimSpace(string(content)) == "" {
			return "", fmt.Errorf("prompt file %s is empty", cfg.PromptFile)
		}
		return string(content), nil
	default:
		return "", errors.New("exec prompt is empty")
	}
}

func resolvedExecModel(cfg *model.RuntimeConfig) string {
	modelName, err := agent.ResolveRuntimeModel(cfg.IDE, cfg.Model)
	if err != nil {
		return cfg.Model
	}
	return modelName
}

func validateExecResumeCompatibility(cfg *model.RuntimeConfig, record PersistedExecRun) error {
	if strings.TrimSpace(cfg.RunID) == "" {
		return nil
	}
	if record.Mode != model.ModeExec {
		return fmt.Errorf("run %q is not an exec run", record.RunID)
	}
	if cfg.WorkspaceRoot != "" && record.WorkspaceRoot != "" && cfg.WorkspaceRoot != record.WorkspaceRoot {
		return fmt.Errorf(
			"run-id %q belongs to workspace %q, not %q",
			record.RunID,
			record.WorkspaceRoot,
			cfg.WorkspaceRoot,
		)
	}
	if cfg.IDE != record.IDE ||
		resolvedExecModel(cfg) != record.Model ||
		cfg.ReasoningEffort != record.ReasoningEffort ||
		cfg.AccessMode != record.AccessMode ||
		!equalStringSlices(cfg.AddDirs, record.AddDirs) {
		return fmt.Errorf("run-id %q must continue with the persisted exec runtime configuration", record.RunID)
	}
	if strings.TrimSpace(record.ACPSessionID) == "" {
		return fmt.Errorf("run-id %q cannot be resumed because it has no persisted ACP session id", record.RunID)
	}
	return nil
}

func renderAssistantOutput(snapshot SessionViewSnapshot) string {
	if len(snapshot.Entries) == 0 {
		return ""
	}
	sections := make([]string, 0, len(snapshot.Entries))
	for _, entry := range snapshot.Entries {
		if entry.Kind != transcriptEntryAssistantMessage {
			continue
		}
		outLines, _ := renderContentBlocks(entry.Blocks)
		section := strings.TrimSpace(strings.Join(outLines, "\n"))
		if section == "" {
			continue
		}
		sections = append(sections, section)
	}
	return strings.Join(sections, "\n\n")
}

func buildExecRunID() string {
	return fmt.Sprintf("exec-%s", time.Now().UTC().Format("20060102-150405-000000000"))
}

func isExecRetryableError(err error) bool {
	if err == nil {
		return false
	}
	if isActivityTimeout(err) {
		return true
	}
	var setupErr *agent.SessionSetupError
	return errors.As(err, &setupErr)
}

func nextRetryTimeout(current time.Duration, multiplier float64) time.Duration {
	if current <= 0 {
		return current
	}
	next := time.Duration(float64(current) * multiplier)
	const maxTimeout = 30 * time.Minute
	if next > maxTimeout {
		return maxTimeout
	}
	return next
}

func equalStringSlices(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for idx := range left {
		if left[idx] != right[idx] {
			return false
		}
	}
	return true
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
