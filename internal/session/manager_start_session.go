package session

import (
	"strings"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
)

func (s *sessionStartSpec) newStartingSession(
	resolved compozyconfig.ResolvedAgent,
	agentDef compozyconfig.AgentDef,
	storage sessionStartStorage,
	now time.Time,
) *Session {
	createdAt := s.createdAt
	if createdAt.IsZero() {
		createdAt = now
	}

	return &Session{
		ID: s.sessionID, Name: s.sessionName, AgentName: resolved.Name,
		Provider: strings.TrimSpace(resolved.Provider), Model: strings.TrimSpace(resolved.Model),
		ReasoningEffort: strings.TrimSpace(s.reasoningEffort), Speed: s.speed, WorkspaceID: s.workspace.ID,
		RuntimeStatus:            RuntimeStatusBinding,
		SelectedRuntime:          cloneRuntimeSelection(s.selectedRuntime),
		RuntimeSelectionRevision: s.runtimeSelectionRevision,
		Workspace:                s.workspace.RootDir, WorktreeID: s.worktreeID, CWD: s.cwd,
		NetworkParticipation: s.networkParticipation,
		NetworkOwnerKey:      s.networkOwnerKey,
		Type:                 normalizeSessionType(s.sessionType),
		Lineage:              store.CloneSessionLineage(s.lineage), State: StateStarting,
		stopReason: s.stopReason, stopDetail: s.stopDetail, failure: store.CloneSessionFailure(s.failure),
		ACPSessionID: s.acpSessionID, Sandbox: cloneSessionSandboxMeta(s.sandbox),
		SoulSnapshotID: s.soulSnapshotID, SoulDigest: s.soulDigest, ParentSoulDigest: s.parentSoulDigest,
		AdvertisedCommands: store.CloneSessionAdvertisedCommands(s.advertisedCommands),
		CreatedAt:          createdAt, UpdatedAt: now, creationProfile: cloneCreationProfile(s.creationProfile),
		creationOptions:  cloneCreationOptions(s.creationOptions),
		creationIdentity: cloneCreationIdentity(s.creationIdentity), sessionDir: storage.sessionDir,
		metaPath: storage.metaPath, dbPath: storage.dbPath, recorder: storage.recorder,
		agentDef:             compozyconfig.CloneAgentDef(agentDef),
		sandboxDestroyOnStop: !s.sandboxDisabled && s.workspace.Sandbox.DestroyOnStop,
	}
}
