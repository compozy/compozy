package session

import (
	"context"

	"fmt"

	"strings"

	"github.com/compozy/agh/internal/acp"
	aghconfig "github.com/compozy/agh/internal/config"
	hookspkg "github.com/compozy/agh/internal/hooks"
)

func (m *Manager) dispatchAgentPreStart(
	ctx context.Context,
	session *Session,
	resolved aghconfig.ResolvedAgent,
	opts acp.StartOpts,
) (acp.StartOpts, error) {
	if m == nil {
		return opts, nil
	}
	ctx = hookDispatchContext(ctx, m, session)

	command, args := splitCommand(opts.Command)
	payload, err := m.hooks.agent().DispatchAgentPreStart(ctx, hookspkg.AgentPreStartPayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookAgentPreStart,
			Timestamp: m.now(),
		},
		SessionContext: hookSessionContext(session),
		Command:        command,
		Args:           args,
		Cwd:            strings.TrimSpace(opts.Cwd),
		Provider:       strings.TrimSpace(resolved.Provider),
		Model:          strings.TrimSpace(resolved.Model),
	})
	if err != nil {
		return acp.StartOpts{}, fmt.Errorf("session: dispatch agent.pre_start: %w", err)
	}

	next := opts
	next.Command = joinCommand(payload.Command, payload.Args)
	next.Cwd = strings.TrimSpace(payload.Cwd)
	return next, nil
}

func (m *Manager) dispatchAgentSpawned(
	ctx context.Context,
	session *Session,
	proc *AgentProcess,
	resolved aghconfig.ResolvedAgent,
) {
	m.dispatchAgentObservation(ctx, session, proc, resolved, nil, hookspkg.HookAgentSpawned)
}

func (m *Manager) dispatchAgentCrashed(ctx context.Context, session *Session, proc *AgentProcess, waitErr error) {
	m.dispatchAgentObservation(ctx, session, proc, aghconfig.ResolvedAgent{}, waitErr, hookspkg.HookAgentCrashed)
}

func (m *Manager) dispatchAgentStopped(ctx context.Context, session *Session, proc *AgentProcess, waitErr error) {
	m.dispatchAgentObservation(ctx, session, proc, aghconfig.ResolvedAgent{}, waitErr, hookspkg.HookAgentStopped)
}

func (m *Manager) dispatchAgentObservation(
	ctx context.Context,
	session *Session,
	proc *AgentProcess,
	resolved aghconfig.ResolvedAgent,
	waitErr error,
	event hookspkg.HookEvent,
) {
	if m == nil {
		return
	}
	ctx = hookDispatchContext(ctx, m, session)

	command, args := agentCommandAndArgs(proc)
	payload := hookspkg.AgentLifecyclePayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     event,
			Timestamp: m.now(),
		},
		SessionContext: hookSessionContext(session),
		Command:        command,
		Args:           args,
		Cwd:            agentCwd(proc),
		PID:            agentPID(proc),
		Provider:       strings.TrimSpace(resolved.Provider),
		Model:          strings.TrimSpace(resolved.Model),
	}
	if waitErr != nil {
		payload.Error = waitErr.Error()
	}

	var err error
	agentHooks := m.hooks.agent()
	switch event {
	case hookspkg.HookAgentSpawned:
		_, err = agentHooks.DispatchAgentSpawned(ctx, payload)
	case hookspkg.HookAgentCrashed:
		_, err = agentHooks.DispatchAgentCrashed(ctx, payload)
	case hookspkg.HookAgentStopped:
		_, err = agentHooks.DispatchAgentStopped(ctx, payload)
	default:
		return
	}
	if err != nil {
		m.warnHookDispatch(ctx, session, event, err)
	}
}
