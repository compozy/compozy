package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	acp "github.com/coder/acp-go-sdk"

	"github.com/compozy/compozy/internal/core/model"
)

// Client manages an ACP agent subprocess and creates sessions.
type Client interface {
	// CreateSession starts a new ACP session with the given prompt.
	CreateSession(ctx context.Context, req SessionRequest) (Session, error)
	// Close terminates the agent subprocess.
	Close() error
}

// ClientConfig describes how to bootstrap an ACP agent process.
type ClientConfig struct {
	IDE             string
	Model           string
	AddDirs         []string
	ReasoningEffort string
	Logger          *slog.Logger
	ShutdownTimeout time.Duration
}

// SessionRequest contains the parameters for creating a new ACP session.
type SessionRequest struct {
	Prompt     []byte
	WorkingDir string
	Model      string
	ExtraEnv   map[string]string
}

// SessionError wraps JSON-RPC/ACP request errors without leaking SDK types.
type SessionError struct {
	Code    int
	Message string
	Data    json.RawMessage
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

type clientImpl struct {
	spec            Spec
	cfg             ClientConfig
	logger          *slog.Logger
	shutdownTimeout time.Duration

	mu            sync.Mutex
	processCancel context.CancelFunc
	cmd           *exec.Cmd
	stdin         io.WriteCloser
	conn          *acp.ClientSideConnection
	started       bool
	closed        bool
	startModel    string
	waitDone      chan struct{}
	waitErr       error
	sessions      map[string]*sessionImpl

	wg        sync.WaitGroup
	closeOnce sync.Once
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
		waitDone:        make(chan struct{}),
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

	sessionResp, err := c.conn.NewSession(ctx, acp.NewSessionRequest{
		Cwd:        workingDir,
		McpServers: []acp.McpServer{},
	})
	if err != nil {
		return nil, wrapACPError(err)
	}

	session := newSession(string(sessionResp.SessionId))
	c.storeSession(session)

	if requestedModel := resolveModel(c.spec, req.Model); requestedModel != c.startModel {
		if _, err := c.conn.SetSessionModel(ctx, acp.SetSessionModelRequest{
			SessionId: sessionResp.SessionId,
			ModelId:   acp.ModelId(requestedModel),
		}); err != nil {
			c.removeSession(session.id)
			return nil, wrapACPError(err)
		}
	}

	c.wg.Add(1)
	go c.runPrompt(ctx, session, acp.PromptRequest{
		SessionId: sessionResp.SessionId,
		Prompt:    []acp.ContentBlock{acp.TextBlock(string(req.Prompt))},
	})

	return session, nil
}

// Close terminates the agent subprocess and waits for background goroutines to exit.
func (c *clientImpl) Close() error {
	var closeErr error
	c.closeOnce.Do(func() {
		c.mu.Lock()
		c.closed = true
		started := c.started
		stdin := c.stdin
		processCancel := c.processCancel
		waitDone := c.waitDone
		c.mu.Unlock()

		if !started {
			return
		}

		if stdin != nil {
			_ = stdin.Close()
		}

		timer := time.NewTimer(c.shutdownTimeout)
		defer timer.Stop()

		select {
		case <-waitDone:
		case <-timer.C:
			if processCancel != nil {
				processCancel()
			}
			<-waitDone
		}

		c.wg.Wait()

		c.mu.Lock()
		c.sessions = make(map[string]*sessionImpl)
		defer c.mu.Unlock()
		if c.waitErr != nil {
			closeErr = c.waitErr
		}
	})
	return closeErr
}

// ReadTextFile handles ACP file read requests from the agent.
func (c *clientImpl) ReadTextFile(_ context.Context, params acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	content, err := os.ReadFile(params.Path)
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
	if err := os.WriteFile(params.Path, []byte(params.Content), 0o600); err != nil {
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
func (c *clientImpl) SessionUpdate(_ context.Context, params acp.SessionNotification) error {
	session := c.lookupSession(string(params.SessionId))
	if session == nil {
		return fmt.Errorf("received update for unknown session %q", params.SessionId)
	}

	update, err := convertACPUpdate(params.Update)
	if err != nil {
		return err
	}
	session.publish(update)
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

	startModel := resolveModel(c.spec, firstNonEmpty(req.Model, c.cfg.Model))
	processCtx, processCancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(
		processCtx,
		c.spec.Binary,
		c.spec.BootstrapArgs(startModel, c.cfg.ReasoningEffort, c.cfg.AddDirs)...,
	)
	cmd.Env = mergeEnvironment(c.spec.EnvVars, req.ExtraEnv)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		c.mu.Unlock()
		processCancel()
		return fmt.Errorf("create ACP stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		c.mu.Unlock()
		_ = stdin.Close()
		processCancel()
		return fmt.Errorf("create ACP stdout pipe: %w", err)
	}

	stderr := &lockedBuffer{}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		c.mu.Unlock()
		_ = stdin.Close()
		processCancel()
		return fmt.Errorf("start ACP agent process: %w", err)
	}

	conn := acp.NewClientSideConnection(c, stdin, stdout)
	if c.logger != nil {
		conn.SetLogger(c.logger)
	}

	c.processCancel = processCancel
	c.cmd = cmd
	c.stdin = stdin
	c.conn = conn
	c.started = true
	c.startModel = startModel
	c.wg.Add(1)
	go c.waitForProcess()
	c.mu.Unlock()

	if _, err := conn.Initialize(ctx, acp.InitializeRequest{
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
	}); err != nil {
		_ = c.Close()
		return wrapACPError(err)
	}

	return nil
}

func (c *clientImpl) waitForProcess() {
	defer c.wg.Done()

	err := c.cmd.Wait()

	c.mu.Lock()
	c.waitErr = normalizeProcessWaitError(err)
	close(c.waitDone)
	c.mu.Unlock()

	if err == nil {
		c.failOpenSessions(errors.New("ACP agent process exited before all sessions completed"))
		return
	}
	c.failOpenSessions(c.waitErr)
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
		session.finish(model.StatusFailed, wrapACPError(err))
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

func (c *clientImpl) lookupSession(id string) *sessionImpl {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sessions[id]
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

func mergeEnvironment(base map[string]string, extra map[string]string) []string {
	env := append([]string(nil), os.Environ()...)
	for key, value := range base {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}
	for key, value := range extra {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}
	return env
}

func normalizeProcessWaitError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("wait for ACP agent process: %w", err)
}

func resolveWorkingDir(dir string) (string, error) {
	trimmed := filepath.Clean(dir)
	if trimmed == "." || trimmed == "" {
		return "", errors.New("session working directory must not be empty")
	}
	if filepath.IsAbs(trimmed) {
		return trimmed, nil
	}
	abs, err := filepath.Abs(trimmed)
	if err != nil {
		return "", fmt.Errorf("resolve session working directory: %w", err)
	}
	return abs, nil
}

func wrapACPError(err error) error {
	if err == nil {
		return nil
	}

	var requestErr *acp.RequestError
	if errors.As(err, &requestErr) {
		return &SessionError{
			Code:    requestErr.Code,
			Message: requestErr.Message,
			Data:    marshalRawJSON(requestErr.Data),
		}
	}
	return err
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

type lockedBuffer struct {
	mu  sync.Mutex
	buf []byte
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = append(b.buf, p...)
	return len(p), nil
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.buf)
}
