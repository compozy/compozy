package acp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	acpsdk "github.com/coder/acp-go-sdk"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
)

func (m *terminalManager) kill(id string) error {
	managed, err := m.lookup(id)
	if err != nil {
		return err
	}
	if err := managed.handle.Signal(context.Background(), managed.actor, terminalpkg.SignalHUP); err != nil && !errors.Is(err, terminalpkg.ErrExited) {
		return fmt.Errorf("acp: kill terminal %q: %w", id, err)
	}
	return nil
}

func (m *terminalManager) output(id string) (string, bool, *acpsdk.TerminalExitStatus, error) {
	managed, err := m.lookup(id)
	if err != nil {
		return "", false, nil, err
	}
	read, err := managed.handle.Screen(context.Background(), terminalpkg.ReadOptions{
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
	if result.Reason == "timeout" && ctx.Err() != nil {
		return nil, ctx.Err()
	}
	info := managed.handle.Info()
	if info.Exit != nil {
		return terminalExitStatus(info.Exit), nil
	}
	return &acpsdk.TerminalExitStatus{ExitCode: result.ExitCode}, nil
}

func (m *terminalManager) release(id string) error {
	managed, err := m.lookup(id)
	if err != nil {
		return err
	}
	info := managed.handle.Info()
	if err := m.core.Release(context.Background(), info.WS, info.ProfileID, info.ID, managed.actor); err != nil && !errors.Is(err, terminalpkg.ErrExited) {
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
		if err := m.release(id); err != nil && m.logger != nil {
			m.logger.Warn("acp: release terminal during close", "terminal_id", id, "error", err)
		}
	}
	if m.ownedCore != nil {
		if err := m.ownedCore.Shutdown(context.Background()); err != nil && m.logger != nil {
			m.logger.Warn("acp: shutdown local terminal core", "error", err)
		}
	}
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
