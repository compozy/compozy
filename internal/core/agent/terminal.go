package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	acp "github.com/coder/acp-go-sdk"
)

const terminalIDPrefix = "term-"

type terminalProcess struct {
	id        string
	sessionID string
	cancel    context.CancelFunc
	cmd       *exec.Cmd
	output    *terminalOutputBuffer
	done      chan struct{}

	mu       sync.Mutex
	exitCode *int
	signal   *string
}

type terminalOutputBuffer struct {
	mu        sync.Mutex
	data      []byte
	limit     int
	truncated bool
}

func (c *clientImpl) createTerminal(
	ctx context.Context,
	params acp.CreateTerminalRequest,
) (acp.CreateTerminalResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	session, err := c.terminalSession(params.SessionId)
	if err != nil {
		return acp.CreateTerminalResponse{}, err
	}
	cwd, err := c.resolveTerminalCWD(session, params.Cwd)
	if err != nil {
		return acp.CreateTerminalResponse{}, err
	}
	if err := ctx.Err(); err != nil {
		return acp.CreateTerminalResponse{}, err
	}

	terminalCtx, cancel := context.WithCancel(context.Background())
	// #nosec G204 -- ACP terminal execution is the requested session-scoped command runner.
	cmd := exec.CommandContext(terminalCtx, params.Command, params.Args...)
	cmd.Dir = cwd
	cmd.Env = terminalEnvironment(params.Env)
	output := newTerminalOutputBuffer(params.OutputByteLimit)
	cmd.Stdout = output
	cmd.Stderr = output
	if err := cmd.Start(); err != nil {
		cancel()
		return acp.CreateTerminalResponse{}, fmt.Errorf("start terminal command %q: %w", params.Command, err)
	}

	terminal := &terminalProcess{
		id:        c.nextTerminalID(),
		sessionID: string(params.SessionId),
		cancel:    cancel,
		cmd:       cmd,
		output:    output,
		done:      make(chan struct{}),
	}
	c.storeTerminal(terminal)
	go terminal.wait()
	return acp.CreateTerminalResponse{TerminalId: terminal.id}, nil
}

func (c *clientImpl) killTerminalCommand(
	_ context.Context,
	params acp.KillTerminalCommandRequest,
) (acp.KillTerminalCommandResponse, error) {
	terminal, err := c.lookupTerminal(params.SessionId, params.TerminalId)
	if err != nil {
		return acp.KillTerminalCommandResponse{}, err
	}
	terminal.kill()
	return acp.KillTerminalCommandResponse{}, nil
}

func (c *clientImpl) terminalOutput(
	_ context.Context,
	params acp.TerminalOutputRequest,
) (acp.TerminalOutputResponse, error) {
	terminal, err := c.lookupTerminal(params.SessionId, params.TerminalId)
	if err != nil {
		return acp.TerminalOutputResponse{}, err
	}
	output, truncated := terminal.output.snapshot()
	response := acp.TerminalOutputResponse{
		Output:    output,
		Truncated: truncated,
	}
	if status := terminal.exitStatus(); status != nil {
		response.ExitStatus = status
	}
	return response, nil
}

func (c *clientImpl) releaseTerminal(
	ctx context.Context,
	params acp.ReleaseTerminalRequest,
) (acp.ReleaseTerminalResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	terminal, err := c.removeTerminal(params.SessionId, params.TerminalId)
	if err != nil {
		return acp.ReleaseTerminalResponse{}, err
	}
	terminal.kill()
	if err := terminal.waitFor(ctx); err != nil {
		return acp.ReleaseTerminalResponse{}, err
	}
	return acp.ReleaseTerminalResponse{}, nil
}

func (c *clientImpl) waitForTerminalExit(
	ctx context.Context,
	params acp.WaitForTerminalExitRequest,
) (acp.WaitForTerminalExitResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	terminal, err := c.lookupTerminal(params.SessionId, params.TerminalId)
	if err != nil {
		return acp.WaitForTerminalExitResponse{}, err
	}
	if err := terminal.waitFor(ctx); err != nil {
		return acp.WaitForTerminalExitResponse{}, err
	}
	exitCode, signal := terminal.exitResult()
	return acp.WaitForTerminalExitResponse{
		ExitCode: exitCode,
		Signal:   signal,
	}, nil
}

func (c *clientImpl) terminalSession(sessionID acp.SessionId) (*sessionImpl, error) {
	session := c.lookupSession(string(sessionID))
	if session == nil {
		return nil, fmt.Errorf("received terminal request for unknown session %q", sessionID)
	}
	return session, nil
}

func (c *clientImpl) resolveTerminalCWD(session *sessionImpl, rawCWD *string) (string, error) {
	if session == nil {
		return "", errors.New("terminal session is required")
	}
	cwd := session.workingDir
	if rawCWD != nil && strings.TrimSpace(*rawCWD) != "" {
		resolved, err := resolveSessionPath(session.workingDir, *rawCWD)
		if err != nil {
			return "", err
		}
		cwd = resolved
	}
	if !pathWithinRoots(cwd, session.allowedRoots) {
		return "", fmt.Errorf("terminal cwd %q is outside allowed session roots", cwd)
	}
	return cwd, nil
}

func (c *clientImpl) nextTerminalID() string {
	c.terminalMu.Lock()
	defer c.terminalMu.Unlock()
	c.terminalNext++
	return terminalIDPrefix + strconv.Itoa(c.terminalNext)
}

func (c *clientImpl) storeTerminal(terminal *terminalProcess) {
	c.terminalMu.Lock()
	defer c.terminalMu.Unlock()
	if c.terminals == nil {
		c.terminals = make(map[string]*terminalProcess)
	}
	c.terminals[terminal.id] = terminal
}

func (c *clientImpl) lookupTerminal(sessionID acp.SessionId, terminalID string) (*terminalProcess, error) {
	c.terminalMu.Lock()
	defer c.terminalMu.Unlock()
	terminal := c.terminals[terminalID]
	if terminal == nil {
		return nil, fmt.Errorf("unknown terminal %q", terminalID)
	}
	if terminal.sessionID != string(sessionID) {
		return nil, fmt.Errorf("terminal %q does not belong to session %q", terminalID, sessionID)
	}
	return terminal, nil
}

func (c *clientImpl) removeTerminal(sessionID acp.SessionId, terminalID string) (*terminalProcess, error) {
	c.terminalMu.Lock()
	defer c.terminalMu.Unlock()
	terminal := c.terminals[terminalID]
	if terminal == nil {
		return nil, fmt.Errorf("unknown terminal %q", terminalID)
	}
	if terminal.sessionID != string(sessionID) {
		return nil, fmt.Errorf("terminal %q does not belong to session %q", terminalID, sessionID)
	}
	delete(c.terminals, terminalID)
	return terminal, nil
}

func (c *clientImpl) closeTerminals() error {
	terminals := c.drainTerminals()
	if len(terminals) == 0 {
		return nil
	}
	timeout := c.shutdownTimeout
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var result error
	for _, terminal := range terminals {
		terminal.kill()
		if err := terminal.waitFor(ctx); err != nil {
			result = errors.Join(result, fmt.Errorf("wait for terminal %s during close: %w", terminal.id, err))
		}
	}
	return result
}

func (c *clientImpl) drainTerminals() []*terminalProcess {
	c.terminalMu.Lock()
	defer c.terminalMu.Unlock()
	terminals := make([]*terminalProcess, 0, len(c.terminals))
	for id, terminal := range c.terminals {
		terminals = append(terminals, terminal)
		delete(c.terminals, id)
	}
	return terminals
}

func (t *terminalProcess) wait() {
	waitErr := t.cmd.Wait()
	t.cancel()
	var exitCode *int
	var signal *string
	if t.cmd.ProcessState != nil {
		code := t.cmd.ProcessState.ExitCode()
		if code >= 0 {
			exitCode = &code
		}
	}
	if exitCode == nil && waitErr != nil {
		message := waitErr.Error()
		signal = &message
	}
	t.mu.Lock()
	t.exitCode = exitCode
	t.signal = signal
	close(t.done)
	t.mu.Unlock()
}

func (t *terminalProcess) kill() {
	if t == nil || t.cancel == nil {
		return
	}
	t.cancel()
}

func (t *terminalProcess) waitFor(ctx context.Context) error {
	if t == nil {
		return errors.New("terminal process is required")
	}
	select {
	case <-t.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (t *terminalProcess) exitResult() (*int, *string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return cloneIntPtr(t.exitCode), cloneStringPtr(t.signal)
}

func (t *terminalProcess) exitStatus() *acp.TerminalExitStatus {
	select {
	case <-t.done:
	default:
		return nil
	}
	exitCode, signal := t.exitResult()
	return &acp.TerminalExitStatus{
		ExitCode: exitCode,
		Signal:   signal,
	}
}

func cloneIntPtr(src *int) *int {
	if src == nil {
		return nil
	}
	value := *src
	return &value
}

func cloneStringPtr(src *string) *string {
	if src == nil {
		return nil
	}
	value := *src
	return &value
}

func newTerminalOutputBuffer(limit *int) *terminalOutputBuffer {
	resolvedLimit := 0
	if limit != nil && *limit > 0 {
		resolvedLimit = *limit
	}
	return &terminalOutputBuffer{limit: resolvedLimit}
}

func (b *terminalOutputBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.data = append(b.data, p...)
	if b.limit > 0 && len(b.data) > b.limit {
		b.data = trimUTF8Suffix(b.data, b.limit)
		b.truncated = true
	}
	return len(p), nil
}

func (b *terminalOutputBuffer) snapshot() (string, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(append([]byte(nil), b.data...)), b.truncated
}

func trimUTF8Suffix(data []byte, limit int) []byte {
	if limit <= 0 || len(data) <= limit {
		return data
	}
	start := len(data) - limit
	for start < len(data) && !utf8.RuneStart(data[start]) {
		start++
	}
	return append([]byte(nil), data[start:]...)
}

func terminalEnvironment(env []acp.EnvVariable) []string {
	merged := os.Environ()
	for _, item := range env {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		merged = append(merged, name+"="+item.Value)
	}
	return merged
}
