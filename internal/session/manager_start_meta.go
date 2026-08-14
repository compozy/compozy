package session

import (
	"fmt"
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
	selectedRuntime, selectionRevision := store.SessionRuntimeSelectionStateValues(meta.RuntimeSelection)
	spec := sessionStartSpec{
		sessionID:                meta.ID,
		sandboxID:                sessionSandboxID(meta.Sandbox),
		sandbox:                  cloneSessionSandboxMeta(meta.Sandbox),
		sandboxDisabled:          meta.Sandbox == nil,
		sessionName:              meta.Name,
		agentName:                meta.AgentName,
		provider:                 strings.TrimSpace(meta.Provider),
		model:                    strings.TrimSpace(meta.Model),
		reasoningEffort:          strings.TrimSpace(meta.ReasoningEffort),
		speed:                    requestedSpeed,
		selectedRuntime:          runtimeSelectionFromSessionStore(selectedRuntime),
		runtimeSelectionRevision: selectionRevision,
		permissions:              compozyconfig.PermissionMode(strings.TrimSpace(meta.EffectivePermissions)),
		workspace:                *workspace,
		worktreeID:               strings.TrimSpace(meta.WorktreeIDValue()),
		networkParticipation:     meta.NetworkSpecSnapshot(),
		networkOwnerKey:          meta.NetworkOwnerKeySnapshot(),
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
		if err := applyCreationProfileSandbox(&spec); err != nil {
			return sessionStartSpec{}, err
		}
	}
	return spec, nil
}

func applyCreationProfileSandbox(spec *sessionStartSpec) error {
	profile := store.NormalizeSessionCreationProfile(*spec.creationProfile)
	switch profile.SandboxMode {
	case store.SessionCreationSandboxNone:
		spec.sandboxDisabled = true
		spec.workspace.SandboxRef = ""
	case store.SessionCreationSandboxRef:
		resolved, err := spec.workspace.Config.ResolveSandbox(profile.SandboxRef)
		if err != nil {
			return fmt.Errorf(
				"session: resolve creation profile sandbox ref %q: %w",
				profile.SandboxRef,
				err,
			)
		}
		spec.sandboxDisabled = false
		spec.workspace.SandboxRef = profile.SandboxRef
		spec.workspace.Sandbox = resolved
	default:
		return fmt.Errorf("session: invalid creation profile sandbox mode %q", profile.SandboxMode)
	}
	return nil
}
