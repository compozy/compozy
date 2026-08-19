package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/worktree"
)

func (b *loopActionSessionBinder) resolveEffectiveCreationProfile(
	ctx context.Context,
	req looppkg.ActionSessionBindRequest,
	pinnedProfileRef string,
) (store.SessionCreationProfile, session.CreateOpts, *worktree.Worktree, error) {
	agent := strings.TrimSpace(req.Agent)
	var opts session.CreateOpts
	var materialized *worktree.Worktree
	if strings.TrimSpace(pinnedProfileRef) != "" {
		profile, err := b.creationStore.GetSessionCreationProfile(ctx, pinnedProfileRef)
		if err != nil {
			return store.SessionCreationProfile{}, session.CreateOpts{}, nil, err
		}
		if err := validatePinnedRuntime(req.Runtime, profile); err != nil {
			return store.SessionCreationProfile{}, session.CreateOpts{}, nil, err
		}
		agent = profile.AgentName
		opts = createOptionsFromProfile(req, profile)
		pinnedCWD := opts.CWD
		pinnedWorktree := opts.Worktree
		opts.CWD = ""
		opts.Worktree = ""
		resolution, err := b.policyGate.applyResolved(ctx, &opts, agent, opts.AllowedToolsOverride)
		if err != nil {
			return store.SessionCreationProfile{}, session.CreateOpts{}, nil, err
		}
		opts.CWD = pinnedCWD
		opts.Worktree = pinnedWorktree
		return b.validateLoopCreationProfile(ctx, req, pinnedProfileRef, opts, &resolution, nil)
	}
	if agent == "" {
		return store.SessionCreationProfile{}, session.CreateOpts{}, nil, fmt.Errorf(
			"%w: managed Goal agent is required",
			looppkg.ErrValidation,
		)
	}
	opts = b.baseCreateOptions(req, agent, loopManagedGoalKind)
	resolution, err := b.policyGate.applyResolved(ctx, &opts, agent, opts.AllowedToolsOverride)
	if err != nil {
		return store.SessionCreationProfile{}, session.CreateOpts{}, nil, err
	}
	materialized, err = b.applyLoopActionEnvironment(ctx, &opts, req)
	if err != nil {
		return store.SessionCreationProfile{}, session.CreateOpts{}, nil, err
	}
	return b.validateLoopCreationProfile(ctx, req, pinnedProfileRef, opts, &resolution, materialized)
}

func (b *loopActionSessionBinder) validateLoopCreationProfile(
	ctx context.Context,
	req looppkg.ActionSessionBindRequest,
	pinnedProfileRef string,
	opts session.CreateOpts,
	resolution *loopSessionPolicyResolution,
	materialized *worktree.Worktree,
) (store.SessionCreationProfile, session.CreateOpts, *worktree.Worktree, error) {
	profile := profileFromPolicyResolution(opts, resolution)
	profileRef, err := profile.Ref()
	if err != nil {
		return store.SessionCreationProfile{}, session.CreateOpts{}, nil,
			b.rollbackLoopActionEnvironment(ctx, req, materialized, err)
	}
	if pinned := strings.TrimSpace(pinnedProfileRef); pinned != "" && profileRef != pinned {
		return store.SessionCreationProfile{}, session.CreateOpts{}, nil, bindingMismatch("creation profile drifted")
	}
	policyDigest, err := profile.PolicySpecDigest()
	if err != nil {
		return store.SessionCreationProfile{}, session.CreateOpts{}, nil,
			b.rollbackLoopActionEnvironment(ctx, req, materialized, err)
	}
	if staticDigest := strings.TrimSpace(req.StaticPolicySpecDigest); staticDigest != "" &&
		staticDigest != policyDigest {
		return store.SessionCreationProfile{}, session.CreateOpts{}, nil,
			b.rollbackLoopActionEnvironment(
				ctx,
				req,
				materialized,
				bindingMismatch("static policy digest drifted"),
			)
	}
	return profile, opts, materialized, nil
}

func (b *loopActionSessionBinder) baseCreateOptions(
	req looppkg.ActionSessionBindRequest,
	agent string,
	kind string,
) session.CreateOpts {
	opts := session.CreateOpts{
		AgentName:                    strings.TrimSpace(agent),
		Provider:                     strings.TrimSpace(req.Runtime.Provider),
		Model:                        strings.TrimSpace(req.Runtime.Model),
		ReasoningEffort:              strings.TrimSpace(req.Runtime.Reasoning),
		Name:                         loopRuntimeSessionName(kind, agent, req.Handle),
		ResolvedNetworkParticipation: req.NetworkParticipation,
		NetworkOwnerKey: participation.OwnerKey(participation.OwnerRef{
			Kind: participation.OwnerKindLoopRun,
			ID:   string(req.LoopRunID),
		}),
		PromptOverlay:       strings.TrimSpace(req.ContractBlock),
		ContractOverlay:     strings.TrimSpace(req.ContractBlock),
		Type:                session.SessionTypeSystem,
		DeniedToolsOverride: loopActionTerminalTools(),
	}
	applyLoopDirectoryBeforePolicy(&opts, req.EnvironmentValue())
	if workspaceID := strings.TrimSpace(string(req.WorkspaceID)); workspaceID != "" {
		opts.Workspace = workspaceID
	} else {
		opts.WorkspacePath = strings.TrimSpace(b.globalWorkspacePath)
	}
	return opts
}

func validatePinnedRuntime(runtime looppkg.RuntimeSpec, profile store.SessionCreationProfile) error {
	if provider := strings.TrimSpace(runtime.Provider); provider != "" &&
		compozyconfig.CanonicalProviderName(provider) != compozyconfig.CanonicalProviderName(profile.Provider) {
		return bindingMismatch("requested runtime provider differs from pinned creation profile")
	}
	if model := strings.TrimSpace(runtime.Model); model != "" && model != strings.TrimSpace(profile.Model) {
		return bindingMismatch("requested runtime model differs from pinned creation profile")
	}
	if reasoning := strings.TrimSpace(runtime.Reasoning); reasoning != "" &&
		reasoning != strings.TrimSpace(profile.ReasoningEffort) {
		return bindingMismatch("requested runtime reasoning differs from pinned creation profile")
	}
	return nil
}

func appliedRuntimeFromCreateOptions(opts session.CreateOpts) looppkg.RuntimeSpec {
	return looppkg.RuntimeSpec{
		Provider:  strings.TrimSpace(opts.Provider),
		Model:     strings.TrimSpace(opts.Model),
		Reasoning: strings.TrimSpace(opts.ReasoningEffort),
	}
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
		Worktree:                     profile.WorktreeRef,
		SandboxRef:                   profile.SandboxRef,
		DisableSandbox:               profile.SandboxMode == store.SessionCreationSandboxNone,
		Permissions:                  compozyconfig.PermissionMode(profile.Permissions),
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
		DeniedToolsOverride:  mergeLoopActionDeniedTools(profile.DeniedTools, loopActionTerminalTools()),
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
		WorktreeRef:     opts.Worktree,
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
