package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
)

func (b *loopActionSessionBinder) resolveEffectiveCreationProfile(
	ctx context.Context,
	req looppkg.ActionSessionBindRequest,
	pinnedProfileRef string,
) (store.SessionCreationProfile, session.CreateOpts, error) {
	agent := strings.TrimSpace(req.Agent)
	var opts session.CreateOpts
	if strings.TrimSpace(pinnedProfileRef) != "" {
		profile, err := b.creationStore.GetSessionCreationProfile(ctx, pinnedProfileRef)
		if err != nil {
			return store.SessionCreationProfile{}, session.CreateOpts{}, err
		}
		agent = profile.AgentName
		opts = createOptionsFromProfile(req, profile)
	} else {
		if agent == "" {
			return store.SessionCreationProfile{}, session.CreateOpts{}, fmt.Errorf(
				"%w: managed Goal agent is required",
				looppkg.ErrValidation,
			)
		}
		opts = b.baseCreateOptions(req, agent, loopManagedGoalKind)
	}
	resolution, err := b.policyGate.applyResolved(ctx, &opts, agent, opts.AllowedToolsOverride)
	if err != nil {
		return store.SessionCreationProfile{}, session.CreateOpts{}, err
	}
	profile := profileFromPolicyResolution(opts, &resolution)
	profileRef, err := profile.Ref()
	if err != nil {
		return store.SessionCreationProfile{}, session.CreateOpts{}, err
	}
	if pinned := strings.TrimSpace(pinnedProfileRef); pinned != "" && profileRef != pinned {
		return store.SessionCreationProfile{}, session.CreateOpts{}, bindingMismatch("creation profile drifted")
	}
	policyDigest, err := profile.PolicySpecDigest()
	if err != nil {
		return store.SessionCreationProfile{}, session.CreateOpts{}, err
	}
	if staticDigest := strings.TrimSpace(req.StaticPolicySpecDigest); staticDigest != "" &&
		staticDigest != policyDigest {
		return store.SessionCreationProfile{}, session.CreateOpts{}, bindingMismatch("static policy digest drifted")
	}
	return profile, opts, nil
}

func (b *loopActionSessionBinder) baseCreateOptions(
	req looppkg.ActionSessionBindRequest,
	agent string,
	kind string,
) session.CreateOpts {
	opts := session.CreateOpts{
		AgentName:                    strings.TrimSpace(agent),
		Model:                        strings.TrimSpace(req.Model),
		CWD:                          strings.TrimSpace(req.CWD),
		Name:                         loopRuntimeSessionName(kind, agent, req.Handle),
		ResolvedNetworkParticipation: req.NetworkParticipation,
		NetworkOwnerKey: participation.OwnerKey(participation.OwnerRef{
			Kind: participation.OwnerKindLoopRun,
			ID:   string(req.LoopRunID),
		}),
		PromptOverlay:   strings.TrimSpace(req.ContractBlock),
		ContractOverlay: strings.TrimSpace(req.ContractBlock),
		Type:            session.SessionTypeSystem,
	}
	if workspaceID := strings.TrimSpace(string(req.WorkspaceID)); workspaceID != "" {
		opts.Workspace = workspaceID
	} else {
		opts.WorkspacePath = strings.TrimSpace(b.globalWorkspacePath)
	}
	return opts
}

func createOptionsFromProfile(
	req looppkg.ActionSessionBindRequest,
	profile store.SessionCreationProfile,
) session.CreateOpts {
	return session.CreateOpts{
		AgentName:                    profile.AgentName,
		Provider:                     profile.Provider,
		Model:                        profile.Model,
		ReasoningEffort:              profile.ReasoningEffort,
		CWD:                          profile.CWD,
		SandboxRef:                   profile.SandboxRef,
		DisableSandbox:               profile.SandboxMode == store.SessionCreationSandboxNone,
		Permissions:                  aghconfig.PermissionMode(profile.Permissions),
		Name:                         loopRuntimeSessionName(loopManagedGoalKind, profile.AgentName, req.Handle),
		Workspace:                    profile.WorkspaceID,
		ResolvedNetworkParticipation: req.NetworkParticipation,
		NetworkOwnerKey: participation.OwnerKey(participation.OwnerRef{
			Kind: participation.OwnerKindLoopRun,
			ID:   string(req.LoopRunID),
		}),
		PromptOverlay:        profile.PromptOverlay,
		ContractOverlay:      profile.ContractOverlay,
		RuntimeMode:          profile.RuntimeMode,
		Type:                 session.SessionTypeSystem,
		AllowedToolsOverride: append([]string(nil), profile.AllowedTools...),
	}
}

func profileFromPolicyResolution(
	opts session.CreateOpts,
	resolution *loopSessionPolicyResolution,
) store.SessionCreationProfile {
	sandboxMode := store.SessionCreationSandboxRef
	sandboxRef := strings.TrimSpace(opts.SandboxRef)
	if opts.DisableSandbox || sandboxRef == "" {
		sandboxMode = store.SessionCreationSandboxNone
		sandboxRef = ""
	}
	permissions := strings.TrimSpace(string(opts.Permissions))
	if permissions == "" {
		permissions = strings.TrimSpace(resolution.agent.Permissions)
	}
	return session.BuildCreationProfile(session.CreationProfileInput{
		AgentName:       resolution.agent.Name,
		Provider:        resolution.agent.Provider,
		Model:           resolution.agent.Model,
		ReasoningEffort: opts.ReasoningEffort,
		WorkspaceID:     resolution.workspace.ID,
		CWD:             opts.CWD,
		SandboxMode:     sandboxMode,
		SandboxRef:      sandboxRef,
		Permissions:     permissions,
		AllowedTools:    opts.AllowedToolsOverride,
		AgentTools:      resolution.agent.Tools,
		AgentToolsets:   resolution.agent.Toolsets,
		DeniedTools:     resolution.agent.DenyTools,
		RuntimeMode:     opts.RuntimeMode,
		PromptOverlay:   opts.PromptOverlay,
		ContractOverlay: opts.ContractOverlay,
	})
}

func bindingCreationIdentity(
	profile store.SessionCreationProfile,
	opts session.CreateOpts,
	sessionID string,
) (store.SessionCreationIdentity, error) {
	profileRef, err := profile.Ref()
	if err != nil {
		return store.SessionCreationIdentity{}, err
	}
	policyDigest, err := profile.PolicySpecDigest()
	if err != nil {
		return store.SessionCreationIdentity{}, err
	}
	networkParticipation := participationSnapshotValue(opts.ResolvedNetworkParticipation)
	creationDigest, err := profile.CreationDigest(store.SessionCreationOptions{
		SessionID:            sessionID,
		Name:                 opts.Name,
		NetworkOwnerKey:      opts.NetworkOwnerKey,
		NetworkParticipation: networkParticipation,
		SessionType:          string(opts.Type),
	})
	if err != nil {
		return store.SessionCreationIdentity{}, err
	}
	return store.SessionCreationIdentity{
		CreationProfileRef: profileRef,
		PolicySpecDigest:   policyDigest,
		CreationDigest:     creationDigest,
	}, nil
}

func bindingAttemptIdentity(req looppkg.ActionSessionBindRequest, epoch int64) (string, string) {
	if epoch == req.TargetBindingEpoch {
		attemptID := strings.TrimSpace(req.BindingAttemptID)
		sessionID := strings.TrimSpace(req.DesiredSessionID)
		if attemptID != "" && sessionID != "" {
			return attemptID, sessionID
		}
	}
	return deterministicBindingValue("bind", req, epoch), deterministicBindingValue("sess", req, epoch)
}

func deterministicBindingValue(prefix string, req looppkg.ActionSessionBindRequest, epoch int64) string {
	material := fmt.Sprintf(
		"%s\x00%s\x00%d\x00%s\x00%d\x00%s\x00%d",
		req.LoopRunID,
		req.WorkspaceID,
		req.Generation,
		req.NodeID,
		req.ItemIndex,
		strings.TrimSpace(req.Handle),
		epoch,
	)
	return prefix + "_" + shortSHA256(material)
}

func shortSHA256(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])[:32]
}

func cloneStoreCreationProfile(profile store.SessionCreationProfile) *store.SessionCreationProfile {
	cloned := store.NormalizeSessionCreationProfile(profile)
	return &cloned
}

func cloneStoreCreationIdentity(identity store.SessionCreationIdentity) *store.SessionCreationIdentity {
	cloned := identity
	return &cloned
}
