package session

import (
	"strings"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/store"
)

func (s *sessionStartSpec) newStartingSession(
	resolved aghconfig.ResolvedAgent,
	agentDef aghconfig.AgentDef,
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
		ReasoningEffort: strings.TrimSpace(s.reasoningEffort), WorkspaceID: s.workspace.ID,
		Workspace: s.workspace.RootDir, CWD: s.cwd, NetworkParticipation: s.networkParticipation,
		NetworkOwnerKey: s.networkOwnerKey,
		Type:            normalizeSessionType(s.sessionType),
		Lineage:         store.CloneSessionLineage(s.lineage), State: StateStarting,
		stopReason: s.stopReason, stopDetail: s.stopDetail, failure: store.CloneSessionFailure(s.failure),
		ACPSessionID: s.acpSessionID, Sandbox: cloneSessionSandboxMeta(s.sandbox),
		SoulSnapshotID: s.soulSnapshotID, SoulDigest: s.soulDigest, ParentSoulDigest: s.parentSoulDigest,
		AdvertisedCommands: store.CloneSessionAdvertisedCommands(s.advertisedCommands),
		CreatedAt:          createdAt, UpdatedAt: now, creationProfile: cloneCreationProfile(s.creationProfile),
		creationOptions:  cloneCreationOptions(s.creationOptions),
		creationIdentity: cloneCreationIdentity(s.creationIdentity), sessionDir: storage.sessionDir,
		metaPath: storage.metaPath, dbPath: storage.dbPath, recorder: storage.recorder,
		agentDef:             aghconfig.CloneAgentDef(agentDef),
		sandboxDestroyOnStop: !s.sandboxDisabled && s.workspace.Sandbox.DestroyOnStop,
	}
}
