package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/store"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func (m *Manager) startupPrompt(
	ctx context.Context,
	sessionCtx hookspkg.SessionContext,
	startupCtx StartupPromptContext,
	agent aghconfig.AgentDef,
	workspace *workspacepkg.ResolvedWorkspace,
) (string, error) {
	prompt := strings.TrimSpace(agent.Prompt)
	if m.assembler == nil {
		return m.dispatchPromptPostAssemble(ctx, sessionCtx, prompt)
	}

	assembledPrompt, err := assembleStartupPrompt(ctx, m.assembler, startupCtx, agent, workspace)
	if err != nil {
		return "", fmt.Errorf("session: assemble prompt for %q: %w", agent.Name, err)
	}
	if strings.TrimSpace(assembledPrompt) == "" {
		assembledPrompt = prompt
	}

	return m.dispatchPromptPostAssemble(ctx, sessionCtx, strings.TrimSpace(assembledPrompt))
}

func assembleStartupPrompt(
	ctx context.Context,
	assembler PromptAssembler,
	startupCtx StartupPromptContext,
	agent aghconfig.AgentDef,
	workspace *workspacepkg.ResolvedWorkspace,
) (string, error) {
	if startupAssembler, ok := assembler.(StartupPromptAssembler); ok {
		return startupAssembler.AssembleStartup(ctx, startupCtx, agent, workspace)
	}
	return assembler.Assemble(ctx, agent, workspace)
}

func (m *Manager) startPermissions(sessionType Type, configured string) aghconfig.PermissionMode {
	if normalizeSessionType(sessionType) == SessionTypeDream {
		return aghconfig.PermissionModeApproveAll
	}

	mode := aghconfig.PermissionMode(strings.TrimSpace(configured))
	if mode == "" {
		return aghconfig.PermissionModeApproveReads
	}
	return mode
}

func (m *Manager) writeMeta(session *Session) error {
	if session == nil {
		return errors.New("session: session is required")
	}
	if err := store.WriteSessionMeta(session.MetaPath(), session.meta()); err != nil {
		return fmt.Errorf("session: write meta for %q: %w", session.ID, err)
	}
	return nil
}

func (m *Manager) activateAndWatch(
	ctx context.Context,
	session *Session,
	proc *AgentProcess,
	adoptCurrentModel bool,
	resolved aghconfig.ResolvedAgent,
	networkCapabilities []NetworkPeerCapability,
	postEvent hookspkg.HookEvent,
	preserveStopReason bool,
) error {
	now := m.now()
	if err := m.activate(session); err != nil {
		return err
	}
	if err := session.activateWithProcess(proc, now, adoptCurrentModel, preserveStopReason); err != nil {
		return err
	}
	if err := m.persistSessionLifecycleState(ctx, session, true); err != nil {
		rollbackErr := m.rollbackActivation(session, proc, now)
		return errors.Join(err, rollbackErr)
	}
	if err := m.joinNetworkPeer(ctx, session, networkCapabilities); err != nil {
		rollbackErr := m.rollbackActivation(session, proc, now)
		return errors.Join(
			fmt.Errorf("session: join network channel for %q: %w", session.ID, err),
			rollbackErr,
		)
	}

	m.dispatchAgentSpawned(ctx, session, proc, resolved)
	switch postEvent {
	case hookspkg.HookSessionPostCreate:
		m.dispatchSessionPostCreate(ctx, session)
	case hookspkg.HookSessionPostResume:
		m.dispatchSessionPostResume(ctx, session)
	}
	if m.notifier != nil {
		m.notifier.OnSessionCreated(ctx, session)
	}
	if _, err := m.persistSessionPresence(ctx, session, now); err != nil {
		m.sessionLogger(session).Warn("session: persist health presence failed", "error", err)
	}
	m.watchProcess(m.lifecycleCtx, session)
	return nil
}

func (m *Manager) joinNetworkPeer(ctx context.Context, session *Session, capabilities []NetworkPeerCapability) error {
	if ctx == nil {
		return errors.New("session: join network peer context is required")
	}
	if session == nil {
		return nil
	}

	info := session.Info()
	if info == nil || info.NetworkParticipation.Mode != participation.ModeLive {
		return nil
	}
	channelID := strings.TrimSpace(info.NetworkParticipation.ChannelID)

	lifecycle := m.currentNetworkPeerLifecycle()
	if lifecycle == nil {
		return nil
	}

	return lifecycle.JoinChannel(
		ctx,
		newNetworkPeerJoin(
			info.ID,
			networkPeerID(info.AgentName, info.ID),
			info.WorkspaceID,
			firstNonEmpty(strings.TrimSpace(info.Name), strings.TrimSpace(info.AgentName)),
			channelID,
			info.NetworkOwnerKey,
			info.NetworkParticipation,
			capabilities,
		),
	)
}

func (m *Manager) leaveNetworkPeer(ctx context.Context, session *Session) error {
	if ctx == nil {
		return errors.New("session: leave network peer context is required")
	}
	if session == nil {
		return nil
	}

	info := session.Info()
	if info == nil || info.NetworkParticipation.Mode != participation.ModeLive {
		return nil
	}

	lifecycle := m.currentNetworkPeerLifecycle()
	if lifecycle == nil {
		return nil
	}

	return lifecycle.LeaveChannel(ctx, info.ID)
}

func (m *Manager) rollbackActivation(session *Session, proc *AgentProcess, now time.Time) error {
	if session == nil {
		return nil
	}

	m.remove(session.ID)
	session.rollbackActivation(now)

	if proc == nil {
		return nil
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), defaultLifecycleTimeout)
	defer cancel()
	return m.driver.Stop(stopCtx, proc)
}

func (m *Manager) sessionLogger(session *Session) *slog.Logger {
	logger := m.logger
	if logger == nil {
		logger = slog.Default()
	}
	if session == nil {
		return logger
	}

	info := session.Info()
	return logger.With("session_id", info.ID, "agent_name", info.AgentName, "provider", info.Provider)
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func networkPeerID(agentName string, sessionID string) string {
	return strings.ToLower(strings.TrimSpace(agentName)) + "." + strings.TrimSpace(sessionID)
}

func isProcessDone(proc *AgentProcess) bool {
	if proc == nil {
		return true
	}
	select {
	case <-proc.Done():
		return true
	default:
		return false
	}
}

func waitForPromptSetup(ctx context.Context, session *Session, promptSetupDone <-chan struct{}) error {
	if promptSetupDone == nil {
		return nil
	}
	select {
	case <-promptSetupDone:
		return nil
	case <-ctx.Done():
		sessionID := ""
		if session != nil {
			sessionID = session.ID
		}
		return fmt.Errorf("session: wait for in-flight prompt setup for %q: %w", sessionID, ctx.Err())
	}
}

func newID(prefix string) string {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		now := time.Now().UTC().UnixNano()
		if strings.TrimSpace(prefix) == "" {
			return fmt.Sprintf("%d", now)
		}
		return fmt.Sprintf("%s-%d", prefix, now)
	}

	if strings.TrimSpace(prefix) == "" {
		return hex.EncodeToString(random[:])
	}
	return fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(random[:]))
}
