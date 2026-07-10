package agent

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	acp "github.com/coder/acp-go-sdk"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/subprocess"
)

// Client manages an ACP agent subprocess and creates sessions.
type Client interface {
	// CreateSession starts a new ACP session with the given prompt.
	CreateSession(ctx context.Context, req SessionRequest) (Session, error)
	// ResumeSession attaches to an existing ACP session and sends a new prompt into it.
	ResumeSession(ctx context.Context, req ResumeSessionRequest) (Session, error)
	// CancelSession requests cancellation of the active prompt turn for a session.
	CancelSession(ctx context.Context, sessionID string) error
	// PromptSession sends a new prompt turn into an already active ACP session.
	PromptSession(ctx context.Context, req PromptSessionRequest) (Session, error)
	// SupportsLoadSession reports whether the connected ACP agent advertised session/load support.
	SupportsLoadSession() bool
	// Close terminates the agent subprocess.
	Close() error
	// Kill force-terminates the agent subprocess immediately.
	Kill() error
}

// ClientConfig describes how to bootstrap an ACP agent process.
type ClientConfig struct {
	IDE             string
	Model           string
	AddDirs         []string
	ReasoningEffort string
	AccessMode      string
	Logger          *slog.Logger
	ShutdownTimeout time.Duration
	// TerminalCommandTimeout is the absolute wall-clock cap applied to each
	// terminal command as a last-resort backstop. Zero falls back to the
	// resolved stall-policy default.
	TerminalCommandTimeout time.Duration
}

// SessionRequest contains the parameters for creating a new ACP session.
type SessionRequest struct {
	Prompt       []byte               `json:"prompt,omitempty"`
	WorkingDir   string               `json:"working_dir,omitempty"`
	Model        string               `json:"model,omitempty"`
	MCPServers   []model.MCPServer    `json:"mcp_servers,omitempty"`
	ExtraEnv     map[string]string    `json:"extra_env,omitempty"`
	Context      context.Context      `json:"-"`
	SetupContext context.Context      `json:"-"`
	RunID        string               `json:"-"`
	JobID        string               `json:"-"`
	RuntimeMgr   model.RuntimeManager `json:"-"`
}

// ResumeSessionRequest contains the parameters for loading and continuing an existing ACP session.
type ResumeSessionRequest struct {
	SessionID    string               `json:"session_id,omitempty"`
	Prompt       []byte               `json:"prompt,omitempty"`
	WorkingDir   string               `json:"working_dir,omitempty"`
	Model        string               `json:"model,omitempty"`
	MCPServers   []model.MCPServer    `json:"mcp_servers,omitempty"`
	ExtraEnv     map[string]string    `json:"extra_env,omitempty"`
	Context      context.Context      `json:"-"`
	SetupContext context.Context      `json:"-"`
	RunID        string               `json:"-"`
	JobID        string               `json:"-"`
	RuntimeMgr   model.RuntimeManager `json:"-"`
}

// PromptSessionRequest contains the parameters for a follow-up prompt in an active ACP session.
type PromptSessionRequest struct {
	SessionID string `json:"session_id,omitempty"`
	Prompt    []byte `json:"prompt,omitempty"`
	MessageID string `json:"message_id,omitempty"`
}

type sessionRequestJSON struct {
	Prompt     string            `json:"prompt,omitempty"`
	WorkingDir string            `json:"working_dir,omitempty"`
	Model      string            `json:"model,omitempty"`
	MCPServers []model.MCPServer `json:"mcp_servers,omitempty"`
	ExtraEnv   map[string]string `json:"extra_env,omitempty"`
}

type resumeSessionRequestJSON struct {
	SessionID  string            `json:"session_id,omitempty"`
	Prompt     string            `json:"prompt,omitempty"`
	WorkingDir string            `json:"working_dir,omitempty"`
	Model      string            `json:"model,omitempty"`
	MCPServers []model.MCPServer `json:"mcp_servers,omitempty"`
	ExtraEnv   map[string]string `json:"extra_env,omitempty"`
}

func (r SessionRequest) MarshalJSON() ([]byte, error) {
	return json.Marshal(sessionRequestJSON{
		Prompt:     string(r.Prompt),
		WorkingDir: r.WorkingDir,
		Model:      r.Model,
		MCPServers: r.MCPServers,
		ExtraEnv:   r.ExtraEnv,
	})
}

func (r *SessionRequest) UnmarshalJSON(data []byte) error {
	if r == nil {
		return errors.New("unmarshal session request: nil receiver")
	}

	var payload sessionRequestJSON
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}

	r.Prompt = nil
	if payload.Prompt != "" {
		r.Prompt = []byte(payload.Prompt)
	}
	r.WorkingDir = payload.WorkingDir
	r.Model = payload.Model
	r.MCPServers = payload.MCPServers
	r.ExtraEnv = payload.ExtraEnv
	return nil
}

func (r ResumeSessionRequest) MarshalJSON() ([]byte, error) {
	return json.Marshal(resumeSessionRequestJSON{
		SessionID:  r.SessionID,
		Prompt:     string(r.Prompt),
		WorkingDir: r.WorkingDir,
		Model:      r.Model,
		MCPServers: r.MCPServers,
		ExtraEnv:   r.ExtraEnv,
	})
}

func (r *ResumeSessionRequest) UnmarshalJSON(data []byte) error {
	if r == nil {
		return errors.New("unmarshal resume session request: nil receiver")
	}

	var payload resumeSessionRequestJSON
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}

	r.SessionID = payload.SessionID
	r.Prompt = nil
	if payload.Prompt != "" {
		r.Prompt = []byte(payload.Prompt)
	}
	r.WorkingDir = payload.WorkingDir
	r.Model = payload.Model
	r.MCPServers = payload.MCPServers
	r.ExtraEnv = payload.ExtraEnv
	return nil
}

// SessionError wraps JSON-RPC/ACP request errors without leaking SDK types.
type SessionError struct {
	Code    int
	Message string
	Data    json.RawMessage
}

// AuthenticationRequiredError marks ACP protocol authentication failures.
type AuthenticationRequiredError struct {
	Err error
}

// PromptCancelledError marks an ACP prompt turn canceled while the session stays reusable.
type PromptCancelledError struct {
	SessionID string
}

func (e *PromptCancelledError) Error() string {
	if e == nil || strings.TrimSpace(e.SessionID) == "" {
		return "ACP prompt turn canceled"
	}
	return fmt.Sprintf("ACP prompt turn canceled for session %s", e.SessionID)
}

// IsPromptCancelled reports whether err is a prompt-turn cancellation, not process shutdown.
func IsPromptCancelled(err error) bool {
	var promptErr *PromptCancelledError
	return errors.As(err, &promptErr)
}

// Error implements the error interface.
func (e *AuthenticationRequiredError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err == nil {
		return "authentication required"
	}
	return e.Err.Error()
}

// Unwrap returns the underlying ACP session error.
func (e *AuthenticationRequiredError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// IsAuthenticationRequired reports whether err is an ACP authentication failure.
func IsAuthenticationRequired(err error) bool {
	var authErr *AuthenticationRequiredError
	return errors.As(err, &authErr)
}

// SessionSetupStage identifies which ACP bootstrap or session-configuration step failed.
type SessionSetupStage string

const (
	// SessionSetupStageStartProcess indicates that starting the ACP subprocess failed.
	SessionSetupStageStartProcess SessionSetupStage = "start_process"
	// SessionSetupStageInitialize indicates that ACP protocol initialization failed.
	SessionSetupStageInitialize SessionSetupStage = "initialize"
	// SessionSetupStageNewSession indicates that ACP session creation failed.
	SessionSetupStageNewSession SessionSetupStage = "new_session"
	// SessionSetupStageLoadSession indicates that ACP session loading failed.
	SessionSetupStageLoadSession SessionSetupStage = "load_session"
	// SessionSetupStageSetModel indicates that ACP session model configuration failed.
	SessionSetupStageSetModel SessionSetupStage = "set_model"
	// SessionSetupStageSetReasoning indicates that ACP session reasoning configuration failed.
	SessionSetupStageSetReasoning SessionSetupStage = "set_reasoning"
	// SessionSetupStageSetMode indicates that ACP session mode configuration failed.
	SessionSetupStageSetMode SessionSetupStage = "set_mode"
)

// SessionSetupError wraps an ACP setup failure with its stage for retry classification.
type SessionSetupError struct {
	Stage SessionSetupStage
	Err   error
}

// Error implements the error interface.
func (e *SessionSetupError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err == nil {
		return string(e.Stage)
	}
	return fmt.Sprintf("%s: %v", e.Stage, e.Err)
}

// Unwrap returns the underlying setup failure.
func (e *SessionSetupError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// Error implements the error interface.
func (e *SessionError) Error() string {
	if e == nil {
		return ""
	}
	if len(e.Data) == 0 {
		return fmt.Sprintf("ACP error %d: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("ACP error %d: %s (%s)", e.Code, e.Message, string(e.Data))
}

func (e *SessionError) toolCallID() string {
	if e == nil || len(e.Data) == 0 {
		return ""
	}

	var payload struct {
		ToolCallID string `json:"tool_call_id"`
	}
	if err := json.Unmarshal(e.Data, &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.ToolCallID)
}

// defaultAgentHandlerDeadline bounds the fast agent-facing handlers we own
// (permission and filesystem requests) so a wedged handler returns a structured
// failure instead of hanging the ACP dispatch goroutine forever.
const defaultAgentHandlerDeadline = 30 * time.Second

type clientImpl struct {
	spec            Spec
	cfg             ClientConfig
	logger          *slog.Logger
	shutdownTimeout time.Duration
	// terminalCommandTimeout caps each terminal command's absolute wall-clock
	// runtime. Zero falls back to model.DefaultStallTerminalCap.
	terminalCommandTimeout time.Duration
	// handlerDeadline bounds the permission/filesystem handlers. Zero disables
	// the deadline (used by focused tests that build the struct directly).
	handlerDeadline time.Duration

	mu             sync.Mutex
	process        *subprocess.Process
	conn           *acp.ClientSideConnection
	started        bool
	closed         bool
	startModel     string
	startCommand   []string
	sessions       map[string]*sessionImpl
	pendingCreates int
	pendingUpdates map[string][]model.SessionUpdate
	loadSupported  bool
	terminalMu     sync.Mutex
	terminalNext   int
	terminals      map[string]*terminalProcess

	wg sync.WaitGroup
}

var _ Client = (*clientImpl)(nil)
var _ acp.Client = (*clientImpl)(nil)

// NewClient constructs a Compozy ACP client wrapper for the configured agent runtime.
func NewClient(_ context.Context, cfg ClientConfig) (Client, error) {
	spec, err := lookupAgentSpec(cfg.IDE)
	if err != nil {
		return nil, err
	}

	shutdownTimeout := cfg.ShutdownTimeout
	if shutdownTimeout <= 0 {
		shutdownTimeout = 3 * time.Second
	}

	return &clientImpl{
		spec:                   spec,
		cfg:                    cfg,
		logger:                 cfg.Logger,
		shutdownTimeout:        shutdownTimeout,
		terminalCommandTimeout: cfg.TerminalCommandTimeout,
		handlerDeadline:        defaultAgentHandlerDeadline,
		sessions:               make(map[string]*sessionImpl),
	}, nil
}

// CreateSession starts a new ACP session and streams updates until the prompt turn completes.
func (c *clientImpl) CreateSession(ctx context.Context, req SessionRequest) (Session, error) {
	req, workingDir, err := prepareCreateSessionRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	setupCtx := req.SetupContext
	if setupCtx == nil {
		setupCtx = ctx
	}

	if err := c.ensureStarted(setupCtx, req); err != nil {
		return nil, err
	}

	mcpServers, err := toACPMCPServers(req.MCPServers)
	if err != nil {
		return nil, fmt.Errorf("prepare ACP MCP servers for new session: %w", err)
	}
	c.beginPendingCreate()
	defer c.finishPendingCreate()
	sessionResp, err := c.conn.NewSession(setupCtx, acp.NewSessionRequest{
		Cwd:        workingDir,
		McpServers: mcpServers,
	})
	if err != nil {
		return nil, c.wrapACPSetupErrorWithDiagnostics(setupCtx, SessionSetupStageNewSession, "create ACP session", err)
	}

	allowedRoots, err := resolveSessionAllowedRoots(workingDir, c.cfg.AddDirs)
	if err != nil {
		return nil, err
	}

	session := newSessionWithAccess(string(sessionResp.SessionId), workingDir, allowedRoots)
	session.setAgentSessionID(extractAgentSessionID(sessionResp.Meta))
	c.storeSession(ctx, session)

	if err := c.configureSession(
		setupCtx,
		sessionResp.SessionId,
		req.Model,
		sessionResp.ConfigOptions,
		sessionResp.Modes,
	); err != nil {
		c.removeSession(session.id)
		return nil, err
	}

	model.DispatchObserverHook(
		ctx,
		req.RuntimeMgr,
		"agent.post_session_create",
		sessionCreatedHookPayload{
			RunID:     req.RunID,
			JobID:     req.JobID,
			SessionID: session.id,
			Identity:  session.Identity(),
		},
	)

	c.wg.Add(1)
	go c.runPrompt(ctx, session, acp.PromptRequest{
		SessionId: sessionResp.SessionId,
		Prompt:    []acp.ContentBlock{acp.TextBlock(string(req.Prompt))},
	})

	return session, nil
}

func prepareCreateSessionRequest(ctx context.Context, req SessionRequest) (SessionRequest, string, error) {
	workingDir, err := resolveWorkingDir(req.WorkingDir)
	if err != nil {
		return SessionRequest{}, "", err
	}

	req.Context = ctx
	req.WorkingDir = workingDir
	req, err = req.dispatchPreCreateHook()
	if err != nil {
		return SessionRequest{}, "", err
	}

	workingDir, err = resolveWorkingDir(req.WorkingDir)
	if err != nil {
		return SessionRequest{}, "", err
	}
	req.WorkingDir = workingDir

	return req, workingDir, nil
}

func prepareResumeSessionRequest(ctx context.Context, req ResumeSessionRequest) (ResumeSessionRequest, string, error) {
	workingDir, err := resolveWorkingDir(req.WorkingDir)
	if err != nil {
		return ResumeSessionRequest{}, "", err
	}
	if strings.TrimSpace(req.SessionID) == "" {
		return ResumeSessionRequest{}, "", errors.New("resume ACP session: missing session id")
	}
	req.Context = ctx
	req.WorkingDir = workingDir
	req, err = req.dispatchPreResumeHook()
	if err != nil {
		return ResumeSessionRequest{}, "", err
	}
	workingDir, err = resolveWorkingDir(req.WorkingDir)
	if err != nil {
		return ResumeSessionRequest{}, "", err
	}
	req.WorkingDir = workingDir
	if strings.TrimSpace(req.SessionID) == "" {
		return ResumeSessionRequest{}, "", errors.New("resume ACP session: missing session id")
	}
	return req, workingDir, nil
}

// ResumeSession loads an existing ACP session, suppresses replayed updates, and sends a new prompt turn.
func (c *clientImpl) ResumeSession(ctx context.Context, req ResumeSessionRequest) (Session, error) {
	req, workingDir, err := prepareResumeSessionRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	sessionReq := SessionRequest{
		Prompt:       req.Prompt,
		WorkingDir:   workingDir,
		Model:        req.Model,
		MCPServers:   model.CloneMCPServers(req.MCPServers),
		ExtraEnv:     req.ExtraEnv,
		SetupContext: req.SetupContext,
	}
	setupCtx := req.SetupContext
	if setupCtx == nil {
		setupCtx = ctx
	}
	if err := c.ensureStarted(setupCtx, sessionReq); err != nil {
		return nil, err
	}
	if !c.SupportsLoadSession() {
		return nil, wrapSessionSetupError(
			SessionSetupStageLoadSession,
			errors.New("ACP agent does not support session/load"),
		)
	}

	allowedRoots, err := resolveSessionAllowedRoots(workingDir, c.cfg.AddDirs)
	if err != nil {
		return nil, err
	}

	sessionID := strings.TrimSpace(req.SessionID)
	session := newLoadedSession(sessionID, workingDir, allowedRoots)
	c.storeSession(ctx, session)

	mcpServers, err := toACPMCPServers(req.MCPServers)
	if err != nil {
		c.removeSession(session.id)
		return nil, fmt.Errorf("prepare ACP MCP servers for load session: %w", err)
	}
	loadResp, err := c.conn.LoadSession(setupCtx, acp.LoadSessionRequest{
		SessionId:  acp.SessionId(sessionID),
		Cwd:        workingDir,
		McpServers: mcpServers,
	})
	if err != nil {
		c.removeSession(session.id)
		return nil, c.wrapACPSetupErrorWithDiagnostics(setupCtx, SessionSetupStageLoadSession, "load ACP session", err)
	}
	session.setAgentSessionID(extractAgentSessionID(loadResp.Meta))
	if err := c.configureSession(
		setupCtx,
		acp.SessionId(sessionID),
		req.Model,
		loadResp.ConfigOptions,
		loadResp.Modes,
	); err != nil {
		c.removeSession(session.id)
		return nil, err
	}
	session.waitForIdle(ctx, 15*time.Millisecond)
	session.resumeUpdates()

	c.wg.Add(1)
	go c.runPrompt(ctx, session, acp.PromptRequest{
		SessionId: acp.SessionId(sessionID),
		Prompt:    []acp.ContentBlock{acp.TextBlock(string(req.Prompt))},
	})

	return session, nil
}

// CancelSession requests cancellation of the active prompt turn without closing the ACP session.
func (c *clientImpl) CancelSession(ctx context.Context, sessionID string) error {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return errors.New("cancel ACP session: missing session id")
	}
	c.mu.Lock()
	closed := c.closed
	conn := c.conn
	c.mu.Unlock()
	if closed {
		return errors.New("ACP client is already closed")
	}
	if conn == nil {
		return errors.New("ACP client is not started")
	}
	if err := conn.Cancel(ctx, acp.CancelNotification{SessionId: acp.SessionId(sessionID)}); err != nil {
		return wrapACPError(err)
	}
	return nil
}

// PromptSession sends a follow-up prompt into an already active ACP session.
func (c *clientImpl) PromptSession(ctx context.Context, req PromptSessionRequest) (Session, error) {
	sessionID := strings.TrimSpace(req.SessionID)
	if sessionID == "" {
		return nil, errors.New("prompt ACP session: missing session id")
	}
	if strings.TrimSpace(string(req.Prompt)) == "" {
		return nil, errors.New("prompt ACP session: prompt is required")
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, errors.New("ACP client is already closed")
	}
	if !c.started || c.conn == nil {
		c.mu.Unlock()
		return nil, errors.New("ACP client is not started")
	}
	previous := c.sessions[sessionID]
	if previous == nil {
		c.mu.Unlock()
		return nil, fmt.Errorf("prompt ACP session: unknown session %q", sessionID)
	}
	workingDir := previous.workingDir
	allowedRoots := append([]string(nil), previous.allowedRoots...)
	identity := previous.Identity()
	c.mu.Unlock()

	session := newSessionWithAccess(sessionID, workingDir, allowedRoots)
	session.identity = identity
	c.storeSession(ctx, session)

	promptReq := acp.PromptRequest{
		SessionId: acp.SessionId(sessionID),
		Prompt:    []acp.ContentBlock{acp.TextBlock(string(req.Prompt))},
	}
	if messageID := strings.TrimSpace(req.MessageID); messageID != "" {
		promptReq.MessageId = &messageID
	}
	c.wg.Add(1)
	go c.runPrompt(ctx, session, promptReq)
	return session, nil
}

// SupportsLoadSession reports whether the connected ACP runtime advertised session/load support.
func (c *clientImpl) SupportsLoadSession() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.loadSupported
}

// Close terminates the agent subprocess and waits for background goroutines to exit.
func (c *clientImpl) Close() error {
	c.markClosed()
	terminalErr := c.closeTerminals()

	process := c.processRef()
	if process == nil {
		return terminalErr
	}

	processErr := process.Shutdown(c.shutdownTimeout)
	return errors.Join(terminalErr, c.awaitBackgroundShutdown(processErr))
}

// Kill force-terminates the agent subprocess and waits for background goroutines to exit.
func (c *clientImpl) Kill() error {
	c.markClosed()
	terminalErr := c.closeTerminals()

	process := c.processRef()
	if process == nil {
		return terminalErr
	}

	processErr := process.Kill()
	return errors.Join(terminalErr, c.awaitBackgroundShutdown(processErr))
}

// ReadTextFile handles ACP file read requests from the agent.
func (c *clientImpl) ReadTextFile(
	ctx context.Context,
	params acp.ReadTextFileRequest,
) (acp.ReadTextFileResponse, error) {
	path, err := c.resolveSessionFilePath(params.SessionId, params.Path)
	if err != nil {
		return acp.ReadTextFileResponse{}, err
	}

	return runWithDeadline(ctx, c.handlerDeadline, "read text file", func() (acp.ReadTextFileResponse, error) {
		content, err := os.ReadFile(path)
		if err != nil {
			return acp.ReadTextFileResponse{}, err
		}
		return acp.ReadTextFileResponse{Content: string(content)}, nil
	})
}

// WriteTextFile handles ACP file write requests from the agent.
func (c *clientImpl) WriteTextFile(
	ctx context.Context,
	params acp.WriteTextFileRequest,
) (acp.WriteTextFileResponse, error) {
	path, err := c.resolveSessionFilePath(params.SessionId, params.Path)
	if err != nil {
		return acp.WriteTextFileResponse{}, err
	}

	return runWithDeadline(ctx, c.handlerDeadline, "write text file", func() (acp.WriteTextFileResponse, error) {
		mode := os.FileMode(0o600)
		if info, statErr := os.Stat(path); statErr == nil {
			mode = info.Mode().Perm()
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return acp.WriteTextFileResponse{}, fmt.Errorf("stat session file %q: %w", path, statErr)
		}
		if err := os.WriteFile(path, []byte(params.Content), mode); err != nil {
			return acp.WriteTextFileResponse{}, err
		}
		return acp.WriteTextFileResponse{}, nil
	})
}

// RequestPermission auto-approves the first offered option to match the current non-interactive runtime.
func (c *clientImpl) RequestPermission(
	ctx context.Context,
	params acp.RequestPermissionRequest,
) (acp.RequestPermissionResponse, error) {
	return runWithDeadline(
		ctx,
		c.handlerDeadline,
		"request permission",
		func() (acp.RequestPermissionResponse, error) {
			return defaultPermissionOutcome(params), nil
		},
	)
}

// defaultPermissionOutcome auto-approves the first offered option, or cancels
// when the agent offered no options.
func defaultPermissionOutcome(params acp.RequestPermissionRequest) acp.RequestPermissionResponse {
	if len(params.Options) == 0 {
		return acp.RequestPermissionResponse{
			Outcome: acp.NewRequestPermissionOutcomeCancelled(),
		}
	}
	return acp.RequestPermissionResponse{
		Outcome: acp.NewRequestPermissionOutcomeSelected(params.Options[0].OptionId),
	}
}

// runWithDeadline runs fn under an absolute deadline. When timeout is positive
// and fn does not return within it, a structured failure carrying the deadline
// cause is returned instead of hanging the ACP dispatch goroutine. A zero
// timeout runs fn inline with no deadline.
//
// The worker goroutine that runs fn is not cancellable: on deadline it is
// abandoned to finish (or block) on its own. Callers must therefore pass only
// short-lived, non-cancellable operations (local filesystem I/O, pure
// computation). A caller that hands fn a call which can block indefinitely (a
// hung network filesystem, a blocking network read) would leak one goroutine
// per timed-out invocation for the lifetime of the client.
func runWithDeadline[T any](
	ctx context.Context,
	timeout time.Duration,
	op string,
	fn func() (T, error),
) (T, error) {
	if timeout <= 0 {
		return fn()
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	type outcome struct {
		value T
		err   error
	}
	// Buffered so the worker never blocks on send after we return on deadline.
	resultCh := make(chan outcome, 1)
	go func() {
		value, err := fn()
		resultCh <- outcome{value: value, err: err}
	}()

	select {
	case res := <-resultCh:
		return res.value, res.err
	case <-ctx.Done():
		var zero T
		return zero, fmt.Errorf("%s deadline exceeded after %s: %w", op, timeout, ctx.Err())
	}
}

// SessionUpdate routes streamed ACP notifications to the correct Compozy session.
func (c *clientImpl) SessionUpdate(ctx context.Context, params acp.SessionNotification) error {
	update, err := convertACPUpdate(c.spec.ID, params.Update)
	if err != nil {
		return err
	}

	session, bufferPending := c.lookupSessionForUpdate(string(params.SessionId))
	if session == nil {
		if !bufferPending {
			return fmt.Errorf("received update for unknown session %q", params.SessionId)
		}
		c.bufferPendingUpdate(string(params.SessionId), update)
		return nil
	}

	session.publish(ctx, update)
	return nil
}

func (c *clientImpl) CreateTerminal(
	ctx context.Context,
	params acp.CreateTerminalRequest,
) (acp.CreateTerminalResponse, error) {
	return c.createTerminal(ctx, params)
}

func (c *clientImpl) KillTerminal(
	ctx context.Context,
	params acp.KillTerminalRequest,
) (acp.KillTerminalResponse, error) {
	return c.killTerminal(ctx, params)
}

func (c *clientImpl) TerminalOutput(
	ctx context.Context,
	params acp.TerminalOutputRequest,
) (acp.TerminalOutputResponse, error) {
	return c.terminalOutput(ctx, params)
}

func (c *clientImpl) ReleaseTerminal(
	ctx context.Context,
	params acp.ReleaseTerminalRequest,
) (acp.ReleaseTerminalResponse, error) {
	return c.releaseTerminal(ctx, params)
}

func (c *clientImpl) WaitForTerminalExit(
	ctx context.Context,
	params acp.WaitForTerminalExitRequest,
) (acp.WaitForTerminalExitResponse, error) {
	return c.waitForTerminalExit(ctx, params)
}

func (c *clientImpl) ensureStarted(ctx context.Context, req SessionRequest) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return errors.New("ACP client is already closed")
	}
	if c.started {
		c.mu.Unlock()
		return nil
	}

	startModel, command, err := c.resolveStartCommand(ctx, req)
	if err != nil {
		c.mu.Unlock()
		return err
	}

	process, err := subprocess.Launch(detachedContext(ctx), subprocess.LaunchConfig{
		Command:         command,
		Env:             c.launchEnvironment(req),
		WorkingDir:      req.WorkingDir,
		WaitDelay:       c.shutdownTimeout,
		WaitErrorPrefix: "wait for ACP agent process",
	})
	if err != nil {
		c.mu.Unlock()
		return wrapSessionSetupError(
			SessionSetupStageStartProcess,
			wrapACPLaunchError(c.spec, command, "", "start ACP agent process", err),
		)
	}

	conn := acp.NewClientSideConnection(c, process.Stdin(), process.Stdout())
	if c.logger != nil {
		conn.SetLogger(c.logger)
	}

	c.process = process
	c.conn = conn
	c.started = true
	c.startModel = startModel
	c.startCommand = append([]string(nil), command...)
	c.wg.Add(1)
	go c.waitForProcess()
	c.mu.Unlock()

	initResp, err := conn.Initialize(ctx, acp.InitializeRequest{
		ProtocolVersion: acp.ProtocolVersionNumber,
		ClientCapabilities: acp.ClientCapabilities{
			Fs: acp.FileSystemCapabilities{
				ReadTextFile:  true,
				WriteTextFile: true,
			},
			Terminal: true,
		},
		ClientInfo: &acp.Implementation{
			Name:    "compozy",
			Version: "dev",
		},
	})
	if err != nil {
		_ = c.Close()
		return wrapSessionSetupError(
			SessionSetupStageInitialize,
			wrapACPLaunchError(
				c.spec,
				command,
				process.StderrBuffer().String(),
				"initialize ACP agent",
				wrapACPErrorWithContextCause(ctx, wrapACPError(err)),
			),
		)
	}

	c.mu.Lock()
	c.loadSupported = initResp.AgentCapabilities.LoadSession
	c.mu.Unlock()

	return nil
}

func detachedContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return context.WithoutCancel(ctx)
}

// launchEnvironment merges the spec environment, the launch-time model pin,
// and the per-session extra environment for the agent subprocess.
func (c *clientImpl) launchEnvironment(req SessionRequest) []string {
	modelEnv := launchModelEnv(c.spec, firstNonEmpty(req.Model, c.cfg.Model))
	if len(modelEnv) == 0 {
		return subprocess.MergeEnvironment(c.spec.EnvVars, req.ExtraEnv)
	}

	base := make(map[string]string, len(c.spec.EnvVars)+len(modelEnv))
	for key, value := range c.spec.EnvVars {
		base[key] = value
	}
	for key, value := range modelEnv {
		base[key] = value
	}
	return subprocess.MergeEnvironment(base, req.ExtraEnv)
}

func (c *clientImpl) resolveStartCommand(ctx context.Context, req SessionRequest) (string, []string, error) {
	requestedModel := resolveModel(c.spec, firstNonEmpty(req.Model, c.cfg.Model))
	startModel := c.spec.DefaultModel
	if c.spec.UsesBootstrapModel {
		startModel = requestedModel
	}
	command, err := resolveLaunchCommand(
		ctx,
		c.spec,
		startModel,
		c.cfg.ReasoningEffort,
		c.cfg.AddDirs,
		c.cfg.AccessMode,
		false,
	)
	if err != nil {
		return "", nil, err
	}
	if err := validateRuntimeCompatibility(c.spec, requestedModel, c.cfg.ReasoningEffort, command); err != nil {
		return "", nil, wrapSessionSetupError(SessionSetupStageStartProcess, err)
	}
	return startModel, command, nil
}

func (c *clientImpl) waitForProcess() {
	defer c.wg.Done()

	process := c.processRef()
	if process == nil {
		return
	}
	err := process.Wait()
	if terminalErr := c.closeTerminals(); terminalErr != nil && c.logger != nil {
		c.logger.Warn("failed to close ACP terminals after agent process exit", "error", terminalErr)
	}

	if process.Forced() {
		c.failOpenSessions(context.Canceled)
		return
	}
	if err == nil {
		c.failOpenSessions(c.agentProcessExitError("ACP agent process exited before all sessions completed", nil))
		return
	}
	c.failOpenSessions(c.agentProcessExitError("ACP agent process failed before all sessions completed", err))
}

func (c *clientImpl) agentProcessExitError(message string, err error) error {
	openSessions := c.openSessionCount()
	stderr := ""
	if process := c.processRef(); process != nil && process.StderrBuffer() != nil {
		stderr = strings.TrimSpace(process.StderrBuffer().String())
	}
	if err == nil {
		err = errors.New(message)
	} else {
		err = fmt.Errorf("%s: %w", message, err)
	}
	if diagnostic := acpProcessStderrDiagnostic(stderr); diagnostic != "" {
		err = fmt.Errorf("%w. %s", err, diagnostic)
	}
	if stderr == "" {
		return fmt.Errorf("%w (open_sessions=%d)", err, openSessions)
	}
	return fmt.Errorf("%w (open_sessions=%d, stderr=%q)", err, openSessions, stderr)
}

func acpProcessStderrDiagnostic(stderr string) string {
	switch {
	case strings.Contains(stderr, "Failed to reserve virtual memory for CodeRange"):
		return "adapter stderr indicates the Codex Code Mode runtime crashed while reserving V8 CodeRange memory"
	case strings.Contains(stderr, "failed to initialize code mode runtime"):
		return "adapter stderr indicates the Codex Code Mode runtime failed to initialize"
	default:
		return ""
	}
}

func (c *clientImpl) openSessionCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.sessions)
}

func (c *clientImpl) markClosed() {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
}

func (c *clientImpl) processRef() *subprocess.Process {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.process
}

func (c *clientImpl) startCommandSnapshot() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.startCommand...)
}

func (c *clientImpl) awaitBackgroundShutdown(processErr error) error {
	c.wg.Wait()

	c.mu.Lock()
	defer c.mu.Unlock()
	c.sessions = make(map[string]*sessionImpl)
	return processErr
}

func (c *clientImpl) runPrompt(ctx context.Context, session *sessionImpl, prompt acp.PromptRequest) {
	defer c.wg.Done()

	resp, err := c.conn.Prompt(ctx, prompt)
	if err != nil {
		if ctx.Err() != nil {
			cancelErr := context.Cause(ctx)
			if cancelErr == nil {
				cancelErr = context.Canceled
			}
			session.finish(model.StatusFailed, cancelErr)
			return
		}
		if process := c.processRef(); process != nil && process.Forced() {
			session.finish(model.StatusFailed, context.Canceled)
			return
		}
		wrappedErr := codexModelCompatibilityHint(c.spec, c.startModel, wrapACPError(err))
		session.waitForIdle(ctx, 15*time.Millisecond)
		if shouldDowngradePromptErrorAfterToolFailure(session, wrappedErr) {
			session.finish(model.StatusCompleted, nil)
			return
		}
		session.finish(model.StatusFailed, wrappedErr)
		return
	}

	if resp.StopReason == acp.StopReasonCancelled {
		cancelErr := context.Cause(ctx)
		if cancelErr == nil {
			if ctx.Err() == nil {
				session.finish(model.StatusCompleted, &PromptCancelledError{SessionID: string(prompt.SessionId)})
				return
			}
			cancelErr = context.Canceled
		}
		session.finish(model.StatusFailed, cancelErr)
		return
	}

	if resp.Usage != nil {
		if u := convertACPUsage(*resp.Usage); u != (model.Usage{}) {
			session.publish(ctx, model.SessionUpdate{Usage: u})
		}
	}

	session.waitForIdle(ctx, 15*time.Millisecond)
	session.finish(model.StatusCompleted, nil)
}

// NewPromptMessageID returns an ACP-compatible UUIDv4 message id.
func NewPromptMessageID() (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate ACP message id: %w", err)
	}
	random[6] = (random[6] & 0x0f) | 0x40
	random[8] = (random[8] & 0x3f) | 0x80
	return fmt.Sprintf(
		"%x-%x-%x-%x-%x",
		random[0:4],
		random[4:6],
		random[6:8],
		random[8:10],
		random[10:16],
	), nil
}

func shouldDowngradePromptErrorAfterToolFailure(session *sessionImpl, err error) bool {
	if err == nil {
		return false
	}

	var sessionErr *SessionError
	if !errors.As(err, &sessionErr) {
		return false
	}
	if sessionErr.toolCallID() != "" {
		return true
	}
	return session != nil && session.lastUpdateFailedToolCall()
}

func (c *clientImpl) storeSession(ctx context.Context, session *sessionImpl) {
	// Parent terminal commands created during this session's prompt turn to the
	// cancellable run/attempt context so cancellation and shutdown propagate.
	session.setRunContext(ctx)

	c.mu.Lock()
	c.sessions[session.id] = session
	pending := append([]model.SessionUpdate(nil), c.pendingUpdates[session.id]...)
	delete(c.pendingUpdates, session.id)
	c.mu.Unlock()

	// Replay buffered updates with the request context values intact, but detach
	// cancellation so a caller timeout does not discard already-received session
	// updates during backpressure.
	detached := detachedContext(ctx)
	for i := range pending {
		session.publish(detached, pending[i])
	}
}

func (c *clientImpl) removeSession(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.sessions, id)
}

func (c *clientImpl) beginPendingCreate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pendingCreates++
}

func (c *clientImpl) finishPendingCreate() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.pendingCreates > 0 {
		c.pendingCreates--
	}
	if c.pendingCreates == 0 {
		c.pendingUpdates = nil
	}
}

func extractAgentSessionID(meta any) string {
	record, ok := meta.(map[string]any)
	if !ok {
		return ""
	}
	for _, key := range []string{"agentSessionId", "sessionId"} {
		value, ok := record[key]
		if !ok {
			continue
		}
		text, ok := value.(string)
		if !ok {
			continue
		}
		trimmed := strings.TrimSpace(text)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func (c *clientImpl) lookupSession(id string) *sessionImpl {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sessions[id]
}

func (c *clientImpl) lookupSessionForUpdate(id string) (*sessionImpl, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	session := c.sessions[id]
	if session != nil {
		return session, false
	}
	return nil, c.pendingCreates > 0
}

func (c *clientImpl) bufferPendingUpdate(id string, update model.SessionUpdate) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.pendingUpdates == nil {
		c.pendingUpdates = make(map[string][]model.SessionUpdate)
	}
	c.pendingUpdates[id] = append(c.pendingUpdates[id], update)
}

func toACPMCPServers(src []model.MCPServer) ([]acp.McpServer, error) {
	if len(src) == 0 {
		return []acp.McpServer{}, nil
	}

	servers := make([]acp.McpServer, 0, len(src))
	for idx := range src {
		item := src[idx]
		if item.Stdio == nil {
			return nil, fmt.Errorf("unsupported ACP MCP server transport at index %d: only stdio is supported", idx)
		}
		servers = append(servers, acp.McpServer{
			Stdio: &acp.McpServerStdio{
				Name:    item.Stdio.Name,
				Command: item.Stdio.Command,
				Args:    append([]string(nil), item.Stdio.Args...),
				Env:     toACPEnvVars(item.Stdio.Env),
			},
		})
	}
	if len(servers) == 0 {
		return []acp.McpServer{}, nil
	}
	return servers, nil
}

func toACPEnvVars(src map[string]string) []acp.EnvVariable {
	if len(src) == 0 {
		return nil
	}

	keys := make([]string, 0, len(src))
	for key := range src {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	env := make([]acp.EnvVariable, 0, len(keys))
	for _, key := range keys {
		env = append(env, acp.EnvVariable{
			Name:  key,
			Value: src[key],
		})
	}
	return env
}

func (c *clientImpl) resolveSessionFilePath(sessionID acp.SessionId, rawPath string) (string, error) {
	session := c.lookupSession(string(sessionID))
	if session == nil {
		return "", fmt.Errorf("received file request for unknown session %q", sessionID)
	}

	path, err := resolveSessionPath(session.workingDir, rawPath)
	if err != nil {
		return "", err
	}
	if !pathWithinRoots(path, session.allowedRoots) {
		return "", fmt.Errorf("path %q is outside allowed session roots", rawPath)
	}
	return path, nil
}

func (c *clientImpl) failOpenSessions(err error) {
	c.mu.Lock()
	sessions := make([]*sessionImpl, 0, len(c.sessions))
	for _, session := range c.sessions {
		sessions = append(sessions, session)
	}
	c.mu.Unlock()

	for _, session := range sessions {
		session.finish(model.StatusFailed, err)
	}
}

func resolveWorkingDir(dir string) (string, error) {
	trimmed := filepath.Clean(dir)
	if trimmed == "." || trimmed == "" {
		return "", errors.New("session working directory must not be empty")
	}
	abs, err := resolveAbsolutePath(trimmed)
	if err != nil {
		return "", fmt.Errorf("resolve session working directory: %w", err)
	}
	return abs, nil
}

func resolveSessionAllowedRoots(workingDir string, addDirs []string) ([]string, error) {
	roots := make([]string, 0, len(addDirs)+1)
	roots = append(roots, workingDir)
	for _, dir := range addDirs {
		if strings.TrimSpace(dir) == "" {
			continue
		}
		absDir, err := resolveAbsolutePath(dir)
		if err != nil {
			return nil, fmt.Errorf("resolve add-dir %q: %w", dir, err)
		}
		roots = append(roots, absDir)
	}
	return roots, nil
}

func resolveSessionPath(workingDir string, rawPath string) (string, error) {
	trimmed := strings.TrimSpace(rawPath)
	if trimmed == "" {
		return "", errors.New("session file path must not be empty")
	}

	if filepath.IsAbs(trimmed) {
		return filepath.Clean(trimmed), nil
	}
	if workingDir == "" {
		return "", fmt.Errorf("resolve relative session file path %q: missing working directory", rawPath)
	}
	return filepath.Clean(filepath.Join(workingDir, trimmed)), nil
}

func resolveAbsolutePath(path string) (string, error) {
	cleaned := filepath.Clean(path)
	if filepath.IsAbs(cleaned) {
		return cleaned, nil
	}
	abs, err := filepath.Abs(cleaned)
	if err != nil {
		return "", err
	}
	return abs, nil
}

func pathWithinRoots(path string, roots []string) bool {
	for _, root := range roots {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			continue
		}
		if rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))) {
			return true
		}
	}
	return false
}

const (
	// jsonRPCServerError is the JSON-RPC 2.0 server error code currently used by
	// ACP runtimes for protocol-level authentication failures.
	jsonRPCServerError = -32000
)

const acpAuthenticationRequiredMessage = "Authentication required"

func wrapACPError(err error) error {
	if err == nil {
		return nil
	}

	var requestErr *acp.RequestError
	if errors.As(err, &requestErr) {
		sessionErr := &SessionError{
			Code:    requestErr.Code,
			Message: requestErr.Message,
		}
		data, marshalErr := marshalRawJSON(requestErr.Data)
		if marshalErr != nil {
			return errors.Join(sessionErr, fmt.Errorf("marshal ACP request error data: %w", marshalErr))
		}
		sessionErr.Data = data
		if isAuthenticationRequiredSessionError(sessionErr) {
			return &AuthenticationRequiredError{Err: sessionErr}
		}
		return sessionErr
	}
	return err
}

func (c *clientImpl) wrapACPSetupErrorWithDiagnostics(
	ctx context.Context,
	stage SessionSetupStage,
	operation string,
	err error,
) error {
	wrapped := wrapACPErrorWithContextCause(ctx, wrapACPError(err))
	if command := c.startCommandSnapshot(); len(command) > 0 {
		stderr := ""
		if process := c.processRef(); process != nil && process.StderrBuffer() != nil {
			stderr = process.StderrBuffer().String()
		}
		wrapped = wrapACPLaunchError(c.spec, command, stderr, operation, wrapped)
	}
	return wrapSessionSetupError(stage, wrapped)
}

func wrapACPErrorWithContextCause(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	cause := cancellationCause(ctx)
	if cause == nil || errors.Is(err, cause) {
		return err
	}
	return errors.Join(cause, err)
}

func cancellationCause(ctx context.Context) error {
	if ctx == nil || ctx.Err() == nil {
		return nil
	}
	if cause := context.Cause(ctx); cause != nil {
		return cause
	}
	// Defensive fallback for non-standard contexts with Err but no Cause.
	return ctx.Err()
}

func isAuthenticationRequiredSessionError(err *SessionError) bool {
	if err == nil || err.Code != jsonRPCServerError {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(err.Message), acpAuthenticationRequiredMessage) {
		return true
	}
	return strings.EqualFold(sessionErrorDataMessage(err.Data), acpAuthenticationRequiredMessage)
}

func sessionErrorDataMessage(data json.RawMessage) string {
	if len(data) == 0 {
		return ""
	}
	var payload struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.Message)
}

type acpLaunchError struct {
	spec    Spec
	command []string
	stderr  string
	stage   string
	err     error
}

func (e *acpLaunchError) Error() string {
	if e == nil {
		return ""
	}
	parts := []string{
		fmt.Sprintf("%s while running %s", e.stage, formatShellCommand(e.command)),
		e.err.Error(),
	}
	if trimmed := strings.TrimSpace(e.stderr); trimmed != "" {
		parts = append(parts, "adapter stderr: "+trimmed)
	}
	if includeACPLaunchInstallGuidance(e.stage) {
		if trimmed := strings.TrimSpace(e.spec.InstallHint); trimmed != "" {
			parts = append(parts, trimmed)
		}
		if trimmed := strings.TrimSpace(e.spec.DocsURL); trimmed != "" {
			parts = append(parts, "docs: "+trimmed)
		}
	}
	return strings.Join(parts, ". ")
}

func includeACPLaunchInstallGuidance(stage string) bool {
	switch stage {
	case "start ACP agent process", "initialize ACP agent":
		return true
	default:
		return false
	}
}

func (e *acpLaunchError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func wrapACPLaunchError(spec Spec, command []string, stderr, stage string, err error) error {
	if err == nil {
		return nil
	}

	return &acpLaunchError{
		spec:    spec,
		command: append([]string(nil), command...),
		stderr:  stderr,
		stage:   stage,
		err:     err,
	}
}

func wrapSessionSetupError(stage SessionSetupStage, err error) error {
	if err == nil {
		return nil
	}
	return &SessionSetupError{Stage: stage, Err: err}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
