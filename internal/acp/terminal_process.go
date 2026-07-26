package acp

import (
	"context"

	"fmt"
	"log/slog"

	"time"

	acpsdk "github.com/coder/acp-go-sdk"

	"github.com/compozy/agh/internal/toolruntime"
)

func (m *terminalManager) kill(id string) error {
	term, err := m.lookup(id)
	if err != nil {
		return err
	}
	term.checkpointInterrupting(context.Background(), "terminal killed")
	if err := killManagedProcess(term.cmd); err != nil {
		return fmt.Errorf("acp: kill terminal %q: %w", id, err)
	}
	return nil
}

func (m *terminalManager) output(id string) (string, bool, *acpsdk.TerminalExitStatus, error) {
	term, err := m.lookup(id)
	if err != nil {
		return "", false, nil, err
	}
	output, truncated, exitStatus := term.snapshot()
	return output, truncated, exitStatus, nil
}

func (m *terminalManager) wait(ctx context.Context, id string) (*acpsdk.TerminalExitStatus, error) {
	term, err := m.lookup(id)
	if err != nil {
		return nil, err
	}
	select {
	case <-term.done:
		_, _, exitStatus := term.snapshot()
		return exitStatus, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (m *terminalManager) release(id string) error {
	term, err := m.lookup(id)
	if err != nil {
		return err
	}
	term.checkpointInterrupting(context.Background(), "terminal released")
	if killErr := killManagedProcess(term.cmd); killErr != nil {
		m.logTerminalKillError(id, "release", killErr)
	}
	m.mu.Lock()
	delete(m.terminals, id)
	m.mu.Unlock()
	return nil
}

func (m *terminalManager) closeAll() {
	m.mu.RLock()
	terminals := make([]*managedTerminal, 0, len(m.terminals))
	for _, terminal := range m.terminals {
		terminals = append(terminals, terminal)
	}
	m.mu.RUnlock()

	for _, terminal := range terminals {
		terminal.checkpointInterrupting(context.Background(), "terminal manager closing")
		if err := killManagedProcess(terminal.cmd); err != nil {
			m.logTerminalKillError(terminal.id, "close_all", err)
		}
	}
}

func (m *terminalManager) logTerminalKillError(id string, reason string, err error) {
	if err == nil || m.logger == nil {
		return
	}
	m.logger.Warn("acp: kill terminal", "terminal_id", id, "reason", reason, "error", err)
}

func (m *terminalManager) lookup(id string) (*managedTerminal, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	term, ok := m.terminals[id]
	if !ok {
		return nil, fmt.Errorf("acp: terminal %q not found", id)
	}
	return term, nil
}

func (w *terminalOutputWriter) Write(p []byte) (int, error) {
	w.terminal.appendOutput(p)
	return len(p), nil
}

func (t *managedTerminal) appendOutput(p []byte) {
	t.mu.Lock()
	defer t.mu.Unlock()
	var truncated bool
	t.output, truncated = appendTerminalOutputWindow(t.output, p, t.outputLimit)
	if truncated {
		t.truncated = true
	}
}

func withoutCancelPreservingDeadline(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		return context.Background(), func() {}
	}

	detached := context.WithoutCancel(ctx)
	deadline, ok := ctx.Deadline()
	if !ok {
		return detached, func() {}
	}
	return context.WithDeadline(detached, deadline)
}

func (t *managedTerminal) wait(ctx context.Context) {
	err := t.cmd.Wait()
	groupWaitErr := forceManagedProcessGroupExit(t.cmd, 250*time.Millisecond)
	exitStatus := &acpsdk.TerminalExitStatus{}
	if t.cmd.ProcessState != nil {
		exitCode := t.cmd.ProcessState.ExitCode()
		if exitCode >= 0 {
			exitStatus.ExitCode = new(exitCode)
		}
	}
	if err != nil && exitStatus.ExitCode == nil {
		signalText := err.Error()
		exitStatus.Signal = &signalText
	}
	if groupWaitErr != nil && exitStatus.Signal == nil {
		signalText := groupWaitErr.Error()
		exitStatus.Signal = &signalText
	}

	t.mu.Lock()
	t.exitStatus = exitStatus
	t.mu.Unlock()
	t.completeProcess(ctx, exitStatus, err, groupWaitErr)
	close(t.done)
}

func (t *managedTerminal) checkpointInterrupting(ctx context.Context, reason string) {
	if t == nil || t.processRecord == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := t.processRecord.Checkpoint(ctx, toolruntime.ProcessCheckpoint{
		State: toolruntime.ProcessStateInterrupting,
		Error: reason,
	}); err != nil {
		slog.Default().Warn("acp: checkpoint terminal process record", "terminal_id", t.id, "error", err)
	}
}

func (t *managedTerminal) completeProcess(
	ctx context.Context,
	exitStatus *acpsdk.TerminalExitStatus,
	waitErr error,
	groupWaitErr error,
) {
	if t == nil || t.processRecord == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	completion := toolruntime.ProcessCompletion{}
	if exitStatus != nil && exitStatus.ExitCode != nil {
		completion.ExitCode = exitStatus.ExitCode
	}
	if waitErr != nil {
		completion.Err = waitErr
	} else if groupWaitErr != nil {
		completion.Err = groupWaitErr
	}
	if err := t.processRecord.Complete(ctx, completion); err != nil {
		slog.Default().Warn("acp: complete terminal process record", "terminal_id", t.id, "error", err)
	}
}

func (t *managedTerminal) snapshot() (string, bool, *acpsdk.TerminalExitStatus) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	output := string(append([]byte(nil), t.output...))
	var exitStatus *acpsdk.TerminalExitStatus
	if t.exitStatus != nil {
		copyStatus := *t.exitStatus
		exitStatus = &copyStatus
	}
	return output, t.truncated, exitStatus
}
