package session

import (
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func sessionStartSpecFromMeta(
	meta store.SessionMeta,
	workspace *workspacepkg.ResolvedWorkspace,
	cwd string,
) (sessionStartSpec, error) {
	requestedSpeed, err := normalizeRequestedSpeed(meta.Speed)
	if err != nil {
		return sessionStartSpec{}, err
	}
	spec := sessionStartSpec{
		sessionID:               meta.ID,
		sandboxID:               sessionSandboxID(meta.Sandbox),
		sandbox:                 cloneSessionSandboxMeta(meta.Sandbox),
		sandboxDisabled:         meta.Sandbox == nil,
		sessionName:             meta.Name,
		agentName:               meta.AgentName,
		provider:                strings.TrimSpace(meta.Provider),
		model:                   strings.TrimSpace(meta.Model),
		reasoningEffort:         strings.TrimSpace(meta.ReasoningEffort),
		speed:                   requestedSpeed,
		permissions:             compozyconfig.PermissionMode(strings.TrimSpace(meta.EffectivePermissions)),
		workspace:               *workspace,
		networkParticipation:    meta.NetworkSpecSnapshot(),
		networkOwnerKey:         meta.NetworkOwnerKeySnapshot(),
		cwd:                     cwd,
		sessionType:             normalizeSessionType(Type(meta.SessionType)),
		lineage:                 store.NormalizeSessionLineage(meta.ID, meta.Lineage),
		createdAt:               meta.CreatedAt,
		soulSnapshotID:          strings.TrimSpace(meta.SoulSnapshotID),
		soulDigest:              strings.TrimSpace(meta.SoulDigest),
		parentSoulDigest:        strings.TrimSpace(meta.ParentSoulDigest),
		creationProfile:         cloneCreationProfile(meta.CreationProfile),
		creationOptions:         cloneCreationOptions(meta.CreationOptions),
		creationIdentity:        creationIdentityFromMeta(meta),
		creationIdentityPinned:  meta.CreationProfile != nil,
		creationIdentityEnabled: meta.CreationProfile != nil,
		advertisedCommands:      store.CloneSessionAdvertisedCommands(meta.AdvertisedCommands),
	}
	if spec.creationProfile != nil {
		spec.runtimeMode = spec.creationProfile.RuntimeMode
		spec.allowedToolsOverride = append([]string(nil), spec.creationProfile.AllowedTools...)
	}
	return spec, nil
}
