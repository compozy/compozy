package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
)

const defaultSandboxExecTimeout = 30 * time.Second

// SandboxExecRequest describes one command execution inside a session sandbox.
type SandboxExecRequest struct {
	SessionID string
	Command   string
	Timeout   time.Duration
}

// SandboxExecResult reports the terminal execution result.
type SandboxExecResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

// ExecSandbox runs a command through the active session's sandbox tool host.
func (m *Manager) ExecSandbox(ctx context.Context, req SandboxExecRequest) (SandboxExecResult, error) {
	if ctx == nil {
		return SandboxExecResult{}, errors.New("session: sandbox exec context is required")
	}
	sessionID := strings.TrimSpace(req.SessionID)
	if sessionID == "" {
		return SandboxExecResult{}, errors.New("session: sandbox exec session id is required")
	}
	command := strings.TrimSpace(req.Command)
	if command == "" {
		return SandboxExecResult{}, errors.New("session: sandbox exec command is required")
	}

	sess, ok := m.Get(sessionID)
	if !ok {
		return SandboxExecResult{}, fmt.Errorf("%w: %s", ErrSessionNotFound, sessionID)
	}
	info := sess.Info()
	if info == nil || info.State != StateActive {
		return SandboxExecResult{}, fmt.Errorf("%w: %s", ErrSessionNotActive, sessionID)
	}
	if info.Sandbox == nil {
		return SandboxExecResult{}, errors.New("session: sandbox is not configured")
	}
	process := sess.processHandle()
	if process == nil {
		return SandboxExecResult{}, errors.New("session: agent process is not available")
	}
	toolHost := process.ToolHost()
	if toolHost == nil {
		return SandboxExecResult{}, errors.New("session: sandbox tool host is not available")
	}

	timeout := req.Timeout
	if timeout <= 0 {
		timeout = defaultSandboxExecTimeout
	}
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cwd := strings.TrimSpace(info.Sandbox.RuntimeRootDir)
	createReq := acpsdk.CreateTerminalRequest{Command: command}
	if cwd != "" {
		createReq.Cwd = &cwd
	}
	terminal, err := toolHost.CreateTerminal(execCtx, createReq)
	if err != nil {
		return SandboxExecResult{}, fmt.Errorf("session: sandbox exec create terminal: %w", err)
	}

	exitCode, waitErr := toolHost.WaitForTerminalExit(execCtx, terminal.TerminalId)
	output, outputErr := toolHost.TerminalOutput(terminal.TerminalId)
	releaseErr := toolHost.ReleaseTerminal(terminal.TerminalId)

	result := SandboxExecResult{ExitCode: exitCode, Stdout: output}
	if waitErr != nil {
		result.Stderr = waitErr.Error()
	}
	if outputErr != nil {
		return result, fmt.Errorf("session: sandbox exec read output: %w", outputErr)
	}
	if releaseErr != nil {
		return result, fmt.Errorf("session: sandbox exec release terminal: %w", releaseErr)
	}
	if waitErr != nil {
		return result, fmt.Errorf("session: sandbox exec wait: %w", waitErr)
	}
	return result, nil
}
