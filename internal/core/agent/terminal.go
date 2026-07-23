package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	acp "github.com/coder/acp-go-sdk"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/subprocess"
)

const (
	terminalIDPrefix       = "term-"
	defaultOutputByteLimit = 10 * 1024 * 1024
	// terminalKillGracePeriod is how long the graceful kill ladder waits after
	// SIGTERM before escalating to SIGKILL on the process group.
	terminalKillGracePeriod = 2 * time.Second
)

// errTerminalCommandCap marks a terminal command terminated because it exceeded
// its absolute wall-clock cap rather than being canceled by the run.
var errTerminalCommandCap = errors.New("terminal command exceeded wall-clock cap")

type terminalProcess struct {
	id        string
	sessionID string
	ctx       context.Context
	cancel    context.CancelFunc
	cmd       *exec.Cmd
	output    *terminalOutputBuffer
	done      chan struct{}
	grace     time.Duration
	killOnce  sync.Once
	onDone    func()

	mu       sync.Mutex
	exitCode *int
	signal   *string
}

type terminalOutputBuffer struct {
	mu        sync.Mutex
	data      []byte
	limit     int
	truncated bool
	// recordActivity records a stall-watchdog heartbeat on each write so a
	// terminal command that keeps producing output is never misread as stalled,
	// while one that goes silent still trips the idle window.
	recordActivity func()
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

	// Parent the command to the session's run/attempt context so cancellation
	// and shutdown propagate, and bound it with an absolute wall-clock cap as a
	// last-resort backstop for a truly runaway command.
	baseCtx := terminalBaseContext(ctx, session)
	terminalCtx, cancel := context.WithTimeoutCause(baseCtx, c.resolveTerminalCap(), errTerminalCommandCap)
	// #nosec G204 -- ACP terminal execution is the requested session-scoped command runner.
	cmd := exec.CommandContext(terminalCtx, params.Command, params.Args...)
	cmd.Dir = cwd
	cmd.Env = terminalEnvironment(params.Env)
	output := newTerminalOutputBuffer(params.OutputByteLimit, c.recordActivity)
	cmd.Stdout = output
	cmd.Stderr = output

	terminal := &terminalProcess{
		id:        c.nextTerminalID(),
		sessionID: string(params.SessionId),
		ctx:       terminalCtx,
		cancel:    cancel,
		cmd:       cmd,
		output:    output,
		done:      make(chan struct{}),
		grace:     terminalKillGracePeriod,
		onDone:    c.terminalActivityFinished,
	}
	// On context cancellation (attempt cancel or cap expiry) os/exec invokes
	// cmd.Cancel. It must not block, so it force-kills the process group as the
	// last-resort reaper; the graceful ladder lives in kill().
	cmd.Cancel = terminal.forceTerminate

	if err := subprocess.ConfigureCommandProcessGroup(cmd); err != nil {
		cancel()
		return acp.CreateTerminalResponse{}, fmt.Errorf("configure terminal command %q: %w", params.Command, err)
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return acp.CreateTerminalResponse{}, fmt.Errorf("start terminal command %q: %w", params.Command, err)
	}

	c.storeTerminal(terminal)
	if c.terminalActivityStarted != nil {
		c.terminalActivityStarted()
	}
	go terminal.wait()
	return acp.CreateTerminalResponse{TerminalId: terminal.id}, nil
}

// terminalBaseContext returns the parent context for a terminal command,
// preferring the session's run/attempt context so cancellation propagates.
func terminalBaseContext(ctx context.Context, session *sessionImpl) context.Context {
	if session != nil {
		if runCtx := session.runContext(); runCtx != nil {
			return runCtx
		}
	}
	if ctx != nil {
		return ctx
	}
	return context.Background()
}

// resolveTerminalCap returns the configured absolute per-command wall-clock cap,
// falling back to the resolved stall-policy default when unset.
func (c *clientImpl) resolveTerminalCap() time.Duration {
	if c.terminalCommandTimeout > 0 {
		return c.terminalCommandTimeout
	}
	return model.DefaultStallTerminalCap
}

func (c *clientImpl) killTerminal(
	_ context.Context,
	params acp.KillTerminalRequest,
) (acp.KillTerminalResponse, error) {
	terminal, err := c.lookupTerminal(params.SessionId, params.TerminalId)
	if err != nil {
		return acp.KillTerminalResponse{}, err
	}
	terminal.kill()
	return acp.KillTerminalResponse{}, nil
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
	terminal, err := c.lookupTerminal(params.SessionId, params.TerminalId)
	if err != nil {
		return acp.ReleaseTerminalResponse{}, err
	}
	terminal.kill()
	if err := terminal.waitFor(ctx); err != nil {
		return acp.ReleaseTerminalResponse{}, err
	}
	if _, err := c.removeTerminal(params.SessionId, params.TerminalId); err != nil {
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

// terminalReaper is the capability the stall watchdog asserts on the client to
// reap a session's terminal commands on fire; *clientImpl satisfies it.
type terminalReaper interface {
	CloseTerminals() error
}

var _ terminalReaper = (*clientImpl)(nil)

// CloseTerminals gracefully tears down every terminal command opened during this
// client's sessions, running the SIGTERM/SIGKILL process-group kill ladder on
// each. It is exported so the stall watchdog can reap a hung command's process
// tree on fire without adding a parallel kill path.
func (c *clientImpl) CloseTerminals() error {
	return c.closeTerminals()
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
	if t.onDone != nil {
		defer t.onDone()
	}
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
		// Surface the cap as the cause when the command was terminated for
		// exceeding its absolute wall-clock backstop, so a stalled command
		// resolves as a clearly-attributed failure rather than a bare signal.
		if cause := t.capCause(); cause != nil {
			message = cause.Error()
		}
		signal = &message
	}
	t.mu.Lock()
	t.exitCode = exitCode
	t.signal = signal
	close(t.done)
	t.mu.Unlock()
}

// capCause reports the absolute-cap cause when the terminal context expired
// because the command outran its wall-clock backstop.
func (t *terminalProcess) capCause() error {
	if t == nil || t.ctx == nil {
		return nil
	}
	if cause := context.Cause(t.ctx); errors.Is(cause, errTerminalCommandCap) {
		return cause
	}
	return nil
}

// kill runs the graceful process-group kill ladder: cooperative SIGTERM, a
// grace period, then SIGKILL, and finally cancels the command context so the
// process tree is reaped and no orphaned subprocess survives.
func (t *terminalProcess) kill() {
	if t == nil {
		return
	}
	t.killOnce.Do(func() {
		if err := subprocess.TerminateCommandProcessTree(t.cmd); err == nil {
			if t.waitWithin(t.grace) {
				t.cancelCommand()
				return
			}
		}
		if err := subprocess.ForceTerminateCommandProcessTree(t.cmd); err != nil {
			slog.Warn(
				"failed to force-terminate terminal process group",
				"terminal_id", t.id,
				"session_id", t.sessionID,
				"error", err,
			)
		}
		t.cancelCommand()
	})
}

// forceTerminate SIGKILLs the process group without blocking. It is wired to
// cmd.Cancel so context cancellation always reaps the tree even when the
// graceful ladder is not driving termination.
func (t *terminalProcess) forceTerminate() error {
	if t == nil {
		return nil
	}
	return subprocess.ForceTerminateCommandProcessTree(t.cmd)
}

func (t *terminalProcess) cancelCommand() {
	if t.cancel != nil {
		t.cancel()
	}
}

// waitWithin reports whether the command exited within d. A non-positive d
// performs a single non-blocking check.
func (t *terminalProcess) waitWithin(d time.Duration) bool {
	if d <= 0 {
		select {
		case <-t.done:
			return true
		default:
			return false
		}
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-t.done:
		return true
	case <-timer.C:
		return false
	}
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

func newTerminalOutputBuffer(limit *int, recordActivity func()) *terminalOutputBuffer {
	resolvedLimit := defaultOutputByteLimit
	if limit != nil && *limit > 0 {
		resolvedLimit = *limit
	}
	return &terminalOutputBuffer{limit: resolvedLimit, recordActivity: recordActivity}
}

func (b *terminalOutputBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	b.data = append(b.data, p...)
	if b.limit > 0 && len(b.data) > b.limit {
		b.data = trimUTF8Suffix(b.data, b.limit)
		b.truncated = true
	}
	b.mu.Unlock()
	// Record the heartbeat outside b.mu so we never nest the buffer lock inside
	// the activity monitor's lock.
	if b.recordActivity != nil {
		b.recordActivity()
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
