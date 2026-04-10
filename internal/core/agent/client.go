package agent

import (
	"context"
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
}

// SessionRequest contains the parameters for creating a new ACP session.
type SessionRequest struct {
	Prompt     []byte
	WorkingDir string
	Model      string
	MCPServers []model.MCPServer
	ExtraEnv   map[string]string
}

// ResumeSessionRequest contains the parameters for loading and continuing an existing ACP session.
type ResumeSessionRequest struct {
	SessionID  string
	Prompt     []byte
	WorkingDir string
	Model      string
	MCPServers []model.MCPServer
	ExtraEnv   map[string]string
}

// SessionError wraps JSON-RPC/ACP request errors without leaking SDK types.
type SessionError struct {
	Code    int
	Message string
	Data    json.RawMessage
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

type clientImpl struct {
	spec            Spec
	cfg             ClientConfig
	logger          *slog.Logger
	shutdownTimeout time.Duration

	mu            sync.Mutex
	process       *subprocess.Process
	conn          *acp.ClientSideConnection
	started       bool
	closed        bool
	startModel    string
	sessions      map[string]*sessionImpl
	loadSupported bool

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
		spec:            spec,
		cfg:             cfg,
		logger:          cfg.Logger,
		shutdownTimeout: shutdownTimeout,
		sessions:        make(map[string]*sessionImpl),
	}, nil
}

// CreateSession starts a new ACP session and streams updates until the prompt turn completes.
func (c *clientImpl) CreateSession(ctx context.Context, req SessionRequest) (Session, error) {
	workingDir, err := resolveWorkingDir(req.WorkingDir)
	if err != nil {
		return nil, err
	}

	if err := c.ensureStarted(ctx, req); err != nil {
		return nil, err
	}

	requestedModel := resolveModel(c.spec, firstNonEmpty(req.Model, c.cfg.Model))
	mcpServers, err := toACPMCPServers(req.MCPServers)
	if err != nil {
		return nil, fmt.Errorf("prepare ACP MCP servers for new session: %w", err)
	}
	sessionResp, err := c.conn.NewSession(ctx, acp.NewSessionRequest{
		Cwd:        workingDir,
		McpServers: mcpServers,
	})
	if err != nil {
		return nil, wrapSessionSetupError(SessionSetupStageNewSession, wrapACPError(err))
	}

	allowedRoots, err := resolveSessionAllowedRoots(workingDir, c.cfg.AddDirs)
	if err != nil {
		return nil, err
	}

	session := newSessionWithAccess(string(sessionResp.SessionId), workingDir, allowedRoots)
	session.setAgentSessionID(extractAgentSessionID(sessionResp.Meta))
	c.storeSession(session)

	if requestedModel != c.startModel {
		if _, err := c.conn.SetSessionModel(ctx, acp.SetSessionModelRequest{
			SessionId: sessionResp.SessionId,
			ModelId:   acp.ModelId(requestedModel),
		}); err != nil {
			c.removeSession(session.id)
			return nil, wrapSessionSetupError(SessionSetupStageSetModel, wrapACPError(err))
		}
	}
	if modeID := c.spec.sessionModeForAccess(c.cfg.AccessMode); modeID != "" {
		if _, err := c.conn.SetSessionMode(ctx, acp.SetSessionModeRequest{
			SessionId: sessionResp.SessionId,
			ModeId:    acp.SessionModeId(modeID),
		}); err != nil {
			c.removeSession(session.id)
			return nil, wrapSessionSetupError(SessionSetupStageSetMode, wrapACPError(err))
		}
	}

	c.wg.Add(1)
	go c.runPrompt(ctx, session, acp.PromptRequest{
		SessionId: sessionResp.SessionId,
		Prompt:    []acp.ContentBlock{acp.TextBlock(string(req.Prompt))},
	})

	return session, nil
}

// ResumeSession loads an existing ACP session, suppresses replayed updates, and sends a new prompt turn.
func (c *clientImpl) ResumeSession(ctx context.Context, req ResumeSessionRequest) (Session, error) {
	workingDir, err := resolveWorkingDir(req.WorkingDir)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.SessionID) == "" {
		return nil, errors.New("resume ACP session: missing session id")
	}

	sessionReq := SessionRequest{
		Prompt:     req.Prompt,
		WorkingDir: workingDir,
		Model:      req.Model,
		MCPServers: model.CloneMCPServers(req.MCPServers),
		ExtraEnv:   req.ExtraEnv,
	}
	if err := c.ensureStarted(ctx, sessionReq); err != nil {
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
	c.storeSession(session)

	mcpServers, err := toACPMCPServers(req.MCPServers)
	if err != nil {
		c.removeSession(session.id)
		return nil, fmt.Errorf("prepare ACP MCP servers for load session: %w", err)
	}
	loadResp, err := c.conn.LoadSession(ctx, acp.LoadSessionRequest{
		SessionId:  acp.SessionId(sessionID),
		Cwd:        workingDir,
		McpServers: mcpServers,
	})
	if err != nil {
		c.removeSession(session.id)
		return nil, wrapSessionSetupError(SessionSetupStageLoadSession, wrapACPError(err))
	}
	session.setAgentSessionID(extractAgentSessionID(loadResp.Meta))
	session.waitForIdle(ctx, 15*time.Millisecond)
	session.resumeUpdates()

	c.wg.Add(1)
	go c.runPrompt(ctx, session, acp.PromptRequest{
		SessionId: acp.SessionId(sessionID),
		Prompt:    []acp.ContentBlock{acp.TextBlock(string(req.Prompt))},
	})

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

	process := c.processRef()
	if process == nil {
		return nil
	}

	processErr := process.Shutdown(c.shutdownTimeout)
	return c.awaitBackgroundShutdown(processErr)
}

// Kill force-terminates the agent subprocess and waits for background goroutines to exit.
func (c *clientImpl) Kill() error {
	c.markClosed()

	process := c.processRef()
	if process == nil {
		return nil
	}

	processErr := process.Kill()
	return c.awaitBackgroundShutdown(processErr)
}

// ReadTextFile handles ACP file read requests from the agent.
func (c *clientImpl) ReadTextFile(_ context.Context, params acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	path, err := c.resolveSessionFilePath(params.SessionId, params.Path)
	if err != nil {
		return acp.ReadTextFileResponse{}, err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return acp.ReadTextFileResponse{}, err
	}
	return acp.ReadTextFileResponse{Content: string(content)}, nil
}

// WriteTextFile handles ACP file write requests from the agent.
func (c *clientImpl) WriteTextFile(
	_ context.Context,
	params acp.WriteTextFileRequest,
) (acp.WriteTextFileResponse, error) {
	path, err := c.resolveSessionFilePath(params.SessionId, params.Path)
	if err != nil {
		return acp.WriteTextFileResponse{}, err
	}

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
}

// RequestPermission auto-approves the first offered option to match the current non-interactive runtime.
func (c *clientImpl) RequestPermission(
	_ context.Context,
	params acp.RequestPermissionRequest,
) (acp.RequestPermissionResponse, error) {
	if len(params.Options) == 0 {
		return acp.RequestPermissionResponse{
			Outcome: acp.NewRequestPermissionOutcomeCancelled(),
		}, nil
	}
	return acp.RequestPermissionResponse{
		Outcome: acp.NewRequestPermissionOutcomeSelected(params.Options[0].OptionId),
	}, nil
}

// SessionUpdate routes streamed ACP notifications to the correct Compozy session.
func (c *clientImpl) SessionUpdate(ctx context.Context, params acp.SessionNotification) error {
	session := c.lookupSession(string(params.SessionId))
	if session == nil {
		return fmt.Errorf("received update for unknown session %q", params.SessionId)
	}

	update, err := convertACPUpdate(c.spec.ID, params.Update)
	if err != nil {
		return err
	}
	session.publish(ctx, update)
	return nil
}

// CreateTerminal rejects terminal integration because the current ACP wrapper does not expose terminal handles.
func (c *clientImpl) CreateTerminal(
	context.Context,
	acp.CreateTerminalRequest,
) (acp.CreateTerminalResponse, error) {
	return acp.CreateTerminalResponse{}, errors.New("terminal integration is not supported")
}

// KillTerminalCommand rejects terminal integration because the current ACP wrapper does not expose terminal handles.
func (c *clientImpl) KillTerminalCommand(
	context.Context,
	acp.KillTerminalCommandRequest,
) (acp.KillTerminalCommandResponse, error) {
	return acp.KillTerminalCommandResponse{}, errors.New("terminal integration is not supported")
}

// TerminalOutput rejects terminal integration because the current ACP wrapper does not expose terminal handles.
func (c *clientImpl) TerminalOutput(
	context.Context,
	acp.TerminalOutputRequest,
) (acp.TerminalOutputResponse, error) {
	return acp.TerminalOutputResponse{}, errors.New("terminal integration is not supported")
}

// ReleaseTerminal rejects terminal integration because the current ACP wrapper does not expose terminal handles.
func (c *clientImpl) ReleaseTerminal(
	context.Context,
	acp.ReleaseTerminalRequest,
) (acp.ReleaseTerminalResponse, error) {
	return acp.ReleaseTerminalResponse{}, errors.New("terminal integration is not supported")
}

// WaitForTerminalExit rejects terminal integration because the current ACP wrapper does not expose terminal handles.
func (c *clientImpl) WaitForTerminalExit(
	context.Context,
	acp.WaitForTerminalExitRequest,
) (acp.WaitForTerminalExitResponse, error) {
	return acp.WaitForTerminalExitResponse{}, errors.New("terminal integration is not supported")
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

	process, err := subprocess.Launch(context.Background(), subprocess.LaunchConfig{
		Command:         command,
		Env:             subprocess.MergeEnvironment(c.spec.EnvVars, req.ExtraEnv),
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
	c.wg.Add(1)
	go c.waitForProcess()
	c.mu.Unlock()

	initResp, err := conn.Initialize(ctx, acp.InitializeRequest{
		ProtocolVersion: acp.ProtocolVersionNumber,
		ClientCapabilities: acp.ClientCapabilities{
			Fs: acp.FileSystemCapability{
				ReadTextFile:  true,
				WriteTextFile: true,
			},
			Terminal: false,
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
				wrapACPError(err),
			),
		)
	}

	c.mu.Lock()
	c.loadSupported = initResp.AgentCapabilities.LoadSession
	c.mu.Unlock()

	return nil
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
	return startModel, command, nil
}

func (c *clientImpl) waitForProcess() {
	defer c.wg.Done()

	process := c.processRef()
	if process == nil {
		return
	}
	err := process.Wait()

	if err == nil {
		c.failOpenSessions(errors.New("ACP agent process exited before all sessions completed"))
		return
	}
	if process.Forced() {
		c.failOpenSessions(context.Canceled)
		return
	}
	c.failOpenSessions(err)
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
		wrappedErr := wrapACPError(err)
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
			cancelErr = context.Canceled
		}
		session.finish(model.StatusFailed, cancelErr)
		return
	}

	session.waitForIdle(ctx, 15*time.Millisecond)
	session.finish(model.StatusCompleted, nil)
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

func (c *clientImpl) storeSession(session *sessionImpl) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sessions[session.id] = session
}

func (c *clientImpl) removeSession(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.sessions, id)
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
		return sessionErr
	}
	return err
}

func wrapACPLaunchError(spec Spec, command []string, stderr, stage string, err error) error {
	if err == nil {
		return nil
	}

	parts := []string{
		fmt.Sprintf("%s while running %s", stage, formatShellCommand(command)),
		err.Error(),
	}
	if trimmed := strings.TrimSpace(stderr); trimmed != "" {
		parts = append(parts, "adapter stderr: "+trimmed)
	}
	if trimmed := strings.TrimSpace(spec.InstallHint); trimmed != "" {
		parts = append(parts, trimmed)
	}
	if trimmed := strings.TrimSpace(spec.DocsURL); trimmed != "" {
		parts = append(parts, "docs: "+trimmed)
	}
	return errors.New(strings.Join(parts, ". "))
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
