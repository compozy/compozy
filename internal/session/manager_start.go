package session

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/workref"
)

type sessionStartRuntime struct {
	agent               compozyconfig.ResolvedAgent
	agentDef            compozyconfig.AgentDef
	mcpServers          []compozyconfig.MCPServer
	networkCapabilities []NetworkPeerCapability
}

type sessionStartStorage struct {
	sessionDir string
	metaPath   string
	dbPath     string
	recorder   EventRecorder
}

const (
	sessionStartActionCreate = "create"
	sessionStartActionResume = "resume"
)

func (m *Manager) prepareResumeStart(ctx context.Context, meta store.SessionMeta) (sessionStartSpec, error) {
	meta, err := m.dispatchSessionPreResume(ctx, meta)
	if err != nil {
		return sessionStartSpec{}, fmt.Errorf("session: dispatch pre-resume for %q: %w", meta.ID, err)
	}

	resolvedWorkspace, err := m.resolveResumeWorkspace(ctx, meta)
	if err != nil {
		return sessionStartSpec{}, fmt.Errorf("session: resolve resume workspace for %q: %w", meta.ID, err)
	}
	if err := validateSessionParticipationWorkspace(meta.NetworkSpecSnapshot(), resolvedWorkspace.ID); err != nil {
		return sessionStartSpec{}, fmt.Errorf("session: validate resume participation for %q: %w", meta.ID, err)
	}
	worktreeID, worktreeRoot, err := m.resolveSessionWorktree(ctx, resolvedWorkspace.ID, meta.WorktreeIDValue())
	if err != nil {
		return sessionStartSpec{}, fmt.Errorf("session: resolve resume worktree for %q: %w", meta.ID, err)
	}
	executionRoot := resolvedWorkspace.RootDir
	if worktreeRoot != "" {
		executionRoot = worktreeRoot
	}
	cwd, err := resumeSessionCWD(meta, executionRoot)
	if err != nil {
		return sessionStartSpec{}, fmt.Errorf("session: validate resume cwd for %q: %w", meta.ID, err)
	}
	spec, err := sessionStartSpecFromMeta(meta, &resolvedWorkspace, cwd)
	if err != nil {
		return sessionStartSpec{}, fmt.Errorf("session: build resume start spec for %q: %w", meta.ID, err)
	}
	spec.worktreeID = worktreeID
	spec.worktreeRoot = worktreeRoot
	spec.postEvent = hookspkg.HookSessionPostResume
	spec.startAction = sessionStartActionResume
	spec.includePromptUpdatedAt = true
	spec.preserveStopReason = sessionMetaStopReason(meta) == store.StopAgentCrashed
	spec.acpSessionID = derefString(meta.ACPSessionID)
	spec.stopReason = sessionMetaStopReason(meta)
	spec.stopDetail = strings.TrimSpace(meta.StopDetail)
	spec.failure = store.CloneSessionFailure(meta.Failure)
	return spec, nil
}

func resumeSessionCWD(meta store.SessionMeta, executionRoot string) (string, error) {
	requested := executionRoot
	if meta.CreationProfile != nil {
		requested = strings.TrimSpace(meta.CreationProfile.CWD)
	} else if cwd := strings.TrimSpace(meta.CWD); cwd != "" {
		requested = cwd
	}
	return ResolveSessionCWD(executionRoot, requested)
}

func (m *Manager) startSession(ctx context.Context, spec *sessionStartSpec) (_ *Session, err error) {
	accepted, err := m.acceptSessionStart(ctx, ctx, spec)
	if err != nil {
		return nil, err
	}
	if err := m.runAcceptedSessionStartAndDispatch(accepted); err != nil {
		return nil, err
	}
	return accepted.session, nil
}

func (m *Manager) startAgentProcess(
	ctx context.Context,
	spec *sessionStartSpec,
	session *Session,
	startOpts acp.StartOpts,
) (*AgentProcess, error) {
	transportStarted := time.Now()
	proc, err := m.driver.Start(ctx, startOpts)
	if err != nil {
		m.sessionLogger(session).Warn("session.start.driver_start_failed", "phase", spec.startAction, "error", err)
		m.logSandboxTransport(session, sandboxEventTransportError, err, time.Since(transportStarted))
		return proc, startupFailure(
			"agent runtime startup failed",
			fmt.Errorf("session: %s agent for %q: %w", spec.startAction, spec.sessionID, err),
		)
	}
	m.logSandboxTransport(session, sandboxEventTransportConnect, nil, time.Since(transportStarted))
	proc.configureRuntime(session.CurrentTurnSource)
	return proc, nil
}

func (s *sessionStartSpec) startupSessionContext(updatedAt time.Time) hookspkg.SessionContext {
	ref := workref.NewRoot(s.workspace.ID, s.workspace.RootDir)
	ctx := hookspkg.SessionContext{
		ProfileID:             strings.TrimSpace(s.profileID),
		SessionID:             strings.TrimSpace(s.sessionID),
		SessionName:           strings.TrimSpace(s.sessionName),
		SessionType:           string(normalizeSessionType(s.sessionType)),
		AgentName:             strings.TrimSpace(s.agentName),
		WorkspaceID:           ref.WorkspaceID,
		Workspace:             ref.Workspace,
		SessionRuntimeContext: hookspkg.NewSessionRuntimeContext(s.worktreeID, s.acpSessionID),
		State:                 string(StateStarting),
		SessionSoulContext: hookSessionSoulContext(
			s.soulSnapshotID,
			s.soulDigest,
		),
		CreatedAt: s.createdAt,
	}
	if s.includePromptUpdatedAt {
		ctx.UpdatedAt = updatedAt
	}
	return ctx
}

func (s *sessionStartSpec) startupPromptContext(updatedAt time.Time) StartupPromptContext {
	ref := workref.NewRoot(s.workspace.ID, s.workspace.RootDir)
	return StartupPromptContext{
		SessionID:            strings.TrimSpace(s.sessionID),
		SessionName:          strings.TrimSpace(s.sessionName),
		AgentName:            strings.TrimSpace(s.agentName),
		Provider:             strings.TrimSpace(s.provider),
		WorkspaceID:          ref.WorkspaceID,
		Workspace:            ref.Workspace,
		WorktreeID:           strings.TrimSpace(s.worktreeID),
		NetworkParticipation: s.networkParticipation,
		SessionType:          normalizeSessionType(s.sessionType),
		SoulSnapshot:         cloneSoulSnapshotPointer(s.soulSnapshot),
		CreatedAt:            s.createdAt,
		UpdatedAt:            updatedAt,
	}
}
