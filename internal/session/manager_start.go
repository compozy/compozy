package session

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/acp"
	aghconfig "github.com/compozy/agh/internal/config"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/workref"
)

type sessionStartRuntime struct {
	agent               aghconfig.ResolvedAgent
	agentDef            aghconfig.AgentDef
	mcpServers          []aghconfig.MCPServer
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
	cwd, err := resumeSessionCWD(meta, resolvedWorkspace.RootDir)
	if err != nil {
		return sessionStartSpec{}, fmt.Errorf("session: validate resume cwd for %q: %w", meta.ID, err)
	}

	return sessionStartSpec{
		sessionID:               meta.ID,
		sandboxID:               sessionSandboxID(meta.Sandbox),
		sandbox:                 cloneSessionSandboxMeta(meta.Sandbox),
		sandboxDisabled:         meta.Sandbox == nil,
		sessionName:             meta.Name,
		agentName:               meta.AgentName,
		provider:                strings.TrimSpace(meta.Provider),
		model:                   strings.TrimSpace(meta.Model),
		reasoningEffort:         strings.TrimSpace(meta.ReasoningEffort),
		permissions:             aghconfig.PermissionMode(strings.TrimSpace(meta.EffectivePermissions)),
		workspace:               resolvedWorkspace,
		networkParticipation:    meta.NetworkSpecSnapshot(),
		networkOwnerKey:         meta.NetworkOwnerKeySnapshot(),
		cwd:                     cwd,
		sessionType:             normalizeSessionType(Type(meta.SessionType)),
		lineage:                 store.NormalizeSessionLineage(meta.ID, meta.Lineage),
		postEvent:               hookspkg.HookSessionPostResume,
		startAction:             sessionStartActionResume,
		includePromptUpdatedAt:  true,
		preserveStopReason:      sessionMetaStopReason(meta) == store.StopAgentCrashed,
		createdAt:               meta.CreatedAt,
		acpSessionID:            derefString(meta.ACPSessionID),
		stopReason:              sessionMetaStopReason(meta),
		stopDetail:              strings.TrimSpace(meta.StopDetail),
		failure:                 store.CloneSessionFailure(meta.Failure),
		soulSnapshotID:          strings.TrimSpace(meta.SoulSnapshotID),
		soulDigest:              strings.TrimSpace(meta.SoulDigest),
		parentSoulDigest:        strings.TrimSpace(meta.ParentSoulDigest),
		creationProfile:         cloneCreationProfile(meta.CreationProfile),
		creationOptions:         cloneCreationOptions(meta.CreationOptions),
		creationIdentity:        creationIdentityFromMeta(meta),
		creationIdentityPinned:  meta.CreationProfile != nil,
		creationIdentityEnabled: meta.CreationProfile != nil,
		advertisedCommands:      store.CloneSessionAdvertisedCommands(meta.AdvertisedCommands),
	}, nil
}

func resumeSessionCWD(meta store.SessionMeta, workspaceRoot string) (string, error) {
	requested := workspaceRoot
	if meta.CreationProfile != nil {
		requested = strings.TrimSpace(meta.CreationProfile.CWD)
	} else if cwd := strings.TrimSpace(meta.CWD); cwd != "" {
		requested = cwd
	}
	return ResolveSessionCWD(workspaceRoot, requested)
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
		SessionID:    strings.TrimSpace(s.sessionID),
		SessionName:  strings.TrimSpace(s.sessionName),
		SessionType:  string(normalizeSessionType(s.sessionType)),
		AgentName:    strings.TrimSpace(s.agentName),
		WorkspaceID:  ref.WorkspaceID,
		Workspace:    ref.Workspace,
		ACPSessionID: strings.TrimSpace(s.acpSessionID),
		State:        string(StateStarting),
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
		NetworkParticipation: s.networkParticipation,
		SessionType:          normalizeSessionType(s.sessionType),
		SoulSnapshot:         cloneSoulSnapshotPointer(s.soulSnapshot),
		CreatedAt:            s.createdAt,
		UpdatedAt:            updatedAt,
	}
}
