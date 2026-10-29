package session

import (
	"context"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
)

func (m *Manager) preparePiBridgeStartup(
	spec *sessionStartSpec,
	session *Session,
	runtime *sessionStartRuntime,
	opts acp.StartOpts,
) acp.StartOpts {
	if runtime.agent.Harness != compozyconfig.ProviderHarnessPiACP {
		return opts
	}
	var statusPath string
	code := ""
	if len(opts.MCPServers) > 0 {
		next, path, err := preparePiNativeTools(session, opts)
		if err != nil {
			code = "pi_native_tools_materialization_failed"
		} else {
			opts, statusPath = next, path
		}
	}
	opts.FinalizeStartup = func(ctx context.Context) (string, acp.StartupManifest, error) {
		if code == "" && statusPath != "" {
			code = readPiBridgeStatus(ctx, statusPath)
		}
		if err := ctx.Err(); err != nil {
			return "", acp.StartupManifest{}, err
		}
		runtime.nativeToolsUnavailable = code != "" || statusPath == ""
		if code != "" {
			if err := m.recordPiBridgeWarning(ctx, session, runtime.agent.Provider, code); err != nil {
				return "", acp.StartupManifest{}, err
			}
		}
		if err := m.assembleAcceptedStartupDefinition(ctx, spec, runtime, m.now()); err != nil {
			return "", acp.StartupManifest{}, err
		}
		session.setAgentDefinition(runtime.agentDef, runtime.startupManifest)
		session.mu.Lock()
		session.nativeToolsUnavailable = runtime.nativeToolsUnavailable
		session.mu.Unlock()
		return runtime.agent.Prompt, runtime.startupManifest, nil
	}
	return opts
}

func (m *Manager) recordPiBridgeWarning(ctx context.Context, session *Session, provider, code string) error {
	turnID := session.CurrentTurnID()
	if turnID == "" {
		var err error
		turnID, err = m.newPromptTurnID()
		if err != nil {
			return err
		}
	}
	now := m.now()
	guidance := "Pi native tools are unavailable; using CLI guidance."
	return m.recordEvent(ctx, session, acp.AgentEvent{
		Type: acp.EventTypeRuntimeWarning, SessionID: session.ID, TurnID: turnID, Timestamp: now,
		Text: guidance,
		ProviderError: &acp.ProviderErrorDiagnostic{
			Code: code, Provider: provider, NextAction: acp.ProviderFailureActionInspect,
			Guidance: guidance, OccurrenceCount: 1, FirstSeenAt: now, LastSeenAt: now,
		},
	})
}
