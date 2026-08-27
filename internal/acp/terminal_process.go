package acp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	acpsdk "github.com/coder/acp-go-sdk"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
)

func (m *terminalManager) kill(ctx context.Context, id string) error {
	if err := terminalRequestContextError(ctx, "kill"); err != nil {
		return err
	}
	managed, err := m.lookup(id)
	if err != nil {
		return err
	}
	if err := managed.handle.Signal(
		ctx,
		managed.actor,
		terminalpkg.SignalHUP,
	); err != nil &&
		!errors.Is(err, terminalpkg.ErrExited) {
		return fmt.Errorf("acp: kill terminal %q: %w", id, err)
	}
	return nil
}

func (m *terminalManager) output(
	ctx context.Context,
	id string,
) (string, bool, *acpsdk.TerminalExitStatus, error) {
	if err := terminalRequestContextError(ctx, "output"); err != nil {
		return "", false, nil, err
	}
	managed, err := m.lookup(id)
	if err != nil {
		return "", false, nil, err
	}
	read, err := managed.handle.Screen(ctx, terminalpkg.ReadOptions{
		View: "tail", MaxBytes: managed.outputLimit,
	})
	if err != nil {
		return "", false, nil, err
	}
	if managed.outputLimit <= 0 {
		return "", read.Seq > 0 || read.Truncated, terminalExitStatus(managed.handle.Info().Exit), nil
	}
	return read.Content, read.Truncated, terminalExitStatus(managed.handle.Info().Exit), nil
}

func (m *terminalManager) wait(ctx context.Context, id string) (*acpsdk.TerminalExitStatus, error) {
	managed, err := m.lookup(id)
	if err != nil {
		return nil, err
	}
	result, err := managed.handle.Wait(ctx, terminalpkg.WaitCondition{Until: "exit"})
	if err != nil {
		return nil, err
	}
	info := managed.handle.Info()
	if info.Exit != nil {
		return terminalExitStatus(info.Exit), nil
	}
	return &acpsdk.TerminalExitStatus{ExitCode: result.ExitCode}, nil
}

func (m *terminalManager) releaseWithContext(ctx context.Context, id string) error {
	if err := terminalRequestContextError(ctx, "release"); err != nil {
		return err
	}
	managed, err := m.lookup(id)
	if err != nil {
		return err
	}
	info := managed.handle.Info()
	if err := m.core.Release(
		ctx,
		info.WS,
		info.ProfileID,
		info.ID,
		managed.actor,
	); err != nil &&
		!errors.Is(err, terminalpkg.ErrExited) {
		return err
	}
	m.mu.Lock()
	delete(m.terminals, id)
	m.mu.Unlock()
	return nil
}

func (m *terminalManager) closeAll() {
	m.mu.RLock()
	ids := make([]string, 0, len(m.terminals))
	for id := range m.terminals {
		ids = append(ids, id)
	}
	m.mu.RUnlock()
	for _, id := range ids {
		cleanupCtx, cancelCleanup := m.cleanupContext()
		if err := m.releaseWithContext(cleanupCtx, id); err != nil && m.logger != nil {
			m.logger.Warn("acp: release terminal during close", "terminal_id", id, "error", err)
		}
		cancelCleanup()
	}
	if m.ownedCore != nil {
		shutdownCtx, cancelShutdown := m.cleanupContext()
		if err := m.ownedCore.Shutdown(shutdownCtx); err != nil && m.logger != nil {
			m.logger.Warn("acp: shutdown local terminal core", "error", err)
		}
		cancelShutdown()
	}
}

func (m *terminalManager) cleanupContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(m.lifecycle), defaultStopTimeout)
}

func terminalRequestContextError(ctx context.Context, operation string) error {
	if ctx == nil {
		return fmt.Errorf("acp: terminal %s context is required", operation)
	}
	if ctx.Err() != nil {
		return context.Cause(ctx)
	}
	return nil
}

func (m *terminalManager) lookup(id string) (*managedTerminal, error) {
	m.mu.RLock()
	managed := m.terminals[id]
	m.mu.RUnlock()
	if managed == nil {
		return nil, fmt.Errorf("acp: terminal %q not found", id)
	}
	return managed, nil
}

func terminalExitStatus(exit *terminalpkg.Exit) *acpsdk.TerminalExitStatus {
	if exit == nil {
		return nil
	}
	status := &acpsdk.TerminalExitStatus{ExitCode: exit.Code}
	if exit.Signal != nil {
		signal := strings.TrimSpace(*exit.Signal)
		if signal != "" {
			status.Signal = &signal
		}
	}
	return status
}
