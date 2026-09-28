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
	selectedRuntime, selectionRevision := store.SessionRuntimeSelectionStateValues(meta.RuntimeSelectionValue())
	acpOptions := ACPOptionSelectionsFromStore(meta.ACPOptionsValue())
	spec := sessionStartSpec{
		sessionID:                meta.ID,
		profileID:                strings.TrimSpace(meta.ProfileID),
		sessionName:              meta.Name,
		agentName:                meta.AgentName,
		provider:                 strings.TrimSpace(meta.Provider),
		model:                    strings.TrimSpace(meta.Model),
		reasoningEffort:          strings.TrimSpace(meta.ReasoningEffort),
		speed:                    requestedSpeed,
		acpOptions:               acpOptions,
		selectedRuntime:          runtimeSelectionFromSessionStore(selectedRuntime),
		runtimeSelectionRevision: selectionRevision,
		permissions:              compozyconfig.PermissionMode(strings.TrimSpace(meta.EffectivePermissionsValue())),
		workspace:                *workspace,
		worktreeID:               strings.TrimSpace(meta.WorktreeIDValue()),
		cwd:                      cwd,
		sessionType:              normalizeSessionType(Type(meta.SessionType)),
		lineage:                  store.NormalizeSessionLineage(meta.ID, meta.Lineage),
		createdAt:                meta.CreatedAt,
		soulSnapshotID:           strings.TrimSpace(meta.SoulSnapshotID),
		soulDigest:               strings.TrimSpace(meta.SoulDigest),
		parentSoulDigest:         strings.TrimSpace(meta.ParentSoulDigest),
		creationProfile:          cloneCreationProfile(meta.CreationProfile),
		creationOptions:          cloneCreationOptions(meta.CreationOptions),
		creationIdentity:         creationIdentityFromMeta(meta),
		creationIdentityPinned:   meta.CreationProfile != nil,
		creationIdentityEnabled:  meta.CreationProfile != nil,
		advertisedCommands:       store.CloneSessionAdvertisedCommands(meta.AdvertisedCommandsValue()),
	}
	if spec.creationProfile != nil {
		spec.runtimeMode = spec.creationProfile.RuntimeMode
		spec.allowedToolsOverride = append([]string(nil), spec.creationProfile.AllowedTools...)
		spec.deniedToolsOverride = append([]string(nil), spec.creationProfile.DeniedTools...)
	}
	return spec, nil
}
