package session

import (
	"context"
	"errors"
	"strings"
)

// AgentExtensionNotifier is the optional driver surface for fire-and-forget
// agent extension delivery over a live process connection.
type AgentExtensionNotifier interface {
	NotifyExtension(ctx context.Context, proc *AgentProcess, method string, params any) error
}

var _ AgentExtensionNotifier = (*ACPDriverAdapter)(nil)

// NotifyAgentExtension delivers one fire-and-forget extension notification to the live agent process.
func (m *Manager) NotifyAgentExtension(
	ctx context.Context,
	sessionID, method string,
	params any,
) error {
	if ctx == nil {
		return errors.New("session: extension notification context is required")
	}
	target := strings.TrimSpace(sessionID)
	if target == "" {
		return errors.New("session: extension notification session ID is required")
	}
	if strings.TrimSpace(method) == "" {
		return errors.New("session: extension notification method is required")
	}
	if m == nil {
		return nil
	}
	session, ok := m.Get(target)
	if !ok || session == nil {
		return nil
	}
	proc := session.processHandle()
	if proc == nil {
		return nil
	}
	notifier, ok := m.driver.(AgentExtensionNotifier)
	if !ok || notifier == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return notifier.NotifyExtension(ctx, proc, method, params)
}
