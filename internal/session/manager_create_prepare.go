package session

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/network/identifier"
	"github.com/compozy/compozy/internal/network/participation"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func (m *Manager) prepareCreateStart(ctx context.Context, opts CreateOpts) (sessionStartSpec, error) {
	opts, sessionType, err := m.prepareCreateOptions(ctx, opts)
	if err != nil {
		return sessionStartSpec{}, err
	}

	location, err := m.resolveCreateLocation(ctx, opts)
	if err != nil {
		return sessionStartSpec{}, err
	}

	agentName, err := compozyconfig.ResolveAgentName(opts.AgentName, location.workspace.Config.Defaults)
	if err != nil {
		return sessionStartSpec{}, fmt.Errorf("session: resolve agent name: %w", err)
	}
	sessionID, err := m.createSessionID(opts.DesiredSessionID)
	if err != nil {
		return sessionStartSpec{}, err
	}
	sandboxID, err := m.prepareCreateSandboxID(location.sandboxDisabled)
	if err != nil {
		return sessionStartSpec{}, err
	}
	lineage, err := m.normalizeCreateOptsLineage(ctx, sessionID, sessionType, location.workspace.ID, opts)
	if err != nil {
		return sessionStartSpec{}, err
	}
	networkParticipation, networkOwnerKey, participationObservation, err := m.prepareCreateParticipation(
		ctx, opts, location.workspace.ID, sessionID,
	)
	if err != nil {
		return sessionStartSpec{}, err
	}
	if err := validateCreateNetworkParticipant(agentName, sessionID, networkParticipation.Mode); err != nil {
		return sessionStartSpec{}, err
	}
	requestedSpeed, acpOptions, err := normalizeCreateRuntimeOptions(opts)
	if err != nil {
		return sessionStartSpec{}, err
	}
	return sessionStartSpec{
		sessionID:                sessionID,
		profileID:                normalizeCreateProfileID(opts.ProfileID),
		sandboxID:                sandboxID,
		sessionName:              strings.TrimSpace(opts.Name),
		agentName:                strings.TrimSpace(agentName),
		provider:                 strings.TrimSpace(opts.Provider),
		model:                    strings.TrimSpace(opts.Model),
		reasoningEffort:          strings.TrimSpace(opts.ReasoningEffort),
		speed:                    requestedSpeed,
		acpOptions:               acpOptions,
		permissions:              opts.Permissions,
		sandboxDisabled:          location.sandboxDisabled,
		scope:                    createSessionScope(opts.Global),
		workspace:                location.workspace,
		worktreeID:               location.worktreeID,
		worktreeRoot:             location.worktreeRoot,
		cwd:                      location.cwd,
		networkParticipation:     networkParticipation,
		networkOwnerKey:          networkOwnerKey,
		participationObservation: participationObservation,
		promptOverlay:            strings.TrimSpace(opts.PromptOverlay),
		contractOverlay:          strings.TrimSpace(opts.ContractOverlay),
		runtimeMode:              strings.TrimSpace(opts.RuntimeMode),
		sessionType:              sessionType,
		lineage:                  lineage,
		allowedToolsOverride:     append([]string(nil), opts.AllowedToolsOverride...),
		deniedToolsOverride:      append([]string(nil), opts.DeniedToolsOverride...),
		creationProfile:          cloneCreationProfile(opts.CreationProfile),
		creationIdentity:         cloneCreationIdentity(opts.CreationIdentity),
		creationIdentityPinned:   opts.CreationProfile != nil || opts.CreationIdentity != nil,
		creationIdentityEnabled:  true,
		discardStartFailure:      opts.DiscardStartFailure,
		parentSoulDigest:         strings.TrimSpace(opts.ParentSoulDigest),
		postEvent:                hookspkg.HookSessionPostCreate,
		startAction:              sessionStartActionCreate,
		cleanupSessionDir:        true,
	}, nil
}

func (m *Manager) prepareCreateOptions(ctx context.Context, opts CreateOpts) (CreateOpts, Type, error) {
	prepared, err := m.dispatchSessionPreCreate(ctx, opts)
	if err != nil {
		return CreateOpts{}, "", err
	}
	sessionType := normalizeSessionType(prepared.Type)
	if err := validateCoordinatorExecutionTarget(prepared, sessionType); err != nil {
		return CreateOpts{}, "", err
	}
	return prepared, sessionType, nil
}

func normalizeCreateSpeed(raw speedpkg.Speed) (speed speedpkg.Speed, err error) {
	speed, err = normalizeRequestedSpeed(raw)
	if err != nil {
		return speed, fmt.Errorf("%w: %w", ErrInvalidRuntimeOverride, err)
	}
	return speed, nil
}

func createSessionScope(global bool) store.SessionScope {
	if global {
		return store.SessionScopeGlobal
	}
	return store.SessionScopeWorkspace
}

func normalizeCreateProfileID(profileID string) string {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return store.DefaultProfileID
	}
	return profileID
}

func validateCreateNetworkParticipant(agentName string, sessionID string, mode participation.Mode) error {
	if mode != participation.ModeLive {
		return nil
	}
	return validateLiveNetworkPeerID(agentName, sessionID)
}

type createLocation struct {
	workspace       workspacepkg.ResolvedWorkspace
	worktreeID      string
	worktreeRoot    string
	cwd             string
	sandboxDisabled bool
}

func (m *Manager) resolveCreateLocation(ctx context.Context, opts CreateOpts) (createLocation, error) {
	resolved, err := m.resolveCreateWorkspace(ctx, opts)
	if err != nil {
		return createLocation{}, err
	}
	worktreeID, worktreeRoot, err := m.resolveSessionWorktree(ctx, resolved.ID, opts.Worktree)
	if err != nil {
		return createLocation{}, err
	}
	disabled, err := applyCreateSandboxOverride(&resolved, opts)
	if err != nil {
		return createLocation{}, err
	}
	executionRoot := resolved.RootDir
	if worktreeRoot != "" {
		executionRoot = worktreeRoot
	}
	cwd, err := ResolveSessionCWD(executionRoot, opts.CWD)
	if err != nil {
		return createLocation{}, err
	}
	return createLocation{
		workspace: resolved, worktreeID: worktreeID, worktreeRoot: worktreeRoot,
		cwd: cwd, sandboxDisabled: disabled,
	}, nil
}

func validateCoordinatorExecutionTarget(opts CreateOpts, sessionType Type) error {
	if sessionType != SessionTypeCoordinator {
		return nil
	}
	if strings.TrimSpace(opts.Worktree) != "" {
		return fmt.Errorf("%w: coordinator sessions cannot bind a worktree", ErrValidation)
	}
	if strings.TrimSpace(opts.CWD) != "" {
		return fmt.Errorf("%w: coordinator sessions cannot set cwd", ErrValidation)
	}
	return nil
}

func (m *Manager) prepareCreateSandboxID(disabled bool) (string, error) {
	if disabled {
		return "", nil
	}
	sandboxID, err := m.newSandboxID()
	if err != nil {
		return "", fmt.Errorf("session: generate sandbox id: %w", err)
	}
	sandboxID = strings.TrimSpace(sandboxID)
	if sandboxID == "" {
		return "", errors.New("session: sandbox id generator returned empty id")
	}
	return sandboxID, nil
}

func (m *Manager) prepareCreateParticipation(
	ctx context.Context,
	opts CreateOpts,
	workspaceID string,
	sessionID string,
) (participation.Spec, string, *participation.ResolvedObservation, error) {
	spec, observation, err := m.resolveCreateParticipation(
		ctx,
		workspaceID,
		sessionID,
		opts.NetworkParticipation,
		opts.ResolvedNetworkParticipation,
		opts.NetworkAuthority,
	)
	if err != nil {
		return participation.Spec{}, "", nil, err
	}
	ownerKey := strings.TrimSpace(opts.NetworkOwnerKey)
	if ownerKey == "" {
		ownerKey = participation.OwnerKey(participation.OwnerRef{
			Kind: participation.OwnerKindSession,
			ID:   sessionID,
		})
	}
	if err := participation.ValidateOwnerKey(ownerKey); err != nil {
		return participation.Spec{}, "", nil, fmt.Errorf("session: validate network owner key: %w", err)
	}
	return spec, ownerKey, observation, nil
}

func (m *Manager) createSessionID(desired string) (string, error) {
	sessionID := strings.TrimSpace(desired)
	if sessionID == "" {
		generated, err := m.newSessionID()
		if err != nil {
			return "", fmt.Errorf("session: generate session id: %w", err)
		}
		sessionID = strings.TrimSpace(generated)
		if sessionID == "" {
			return "", errors.New("session: session id generator returned empty id")
		}
	}
	if len(sessionID) > identifier.PeerIDMaxLength || sessionID == "." || sessionID == ".." ||
		filepath.Base(sessionID) != sessionID ||
		strings.ContainsAny(sessionID, `/\\`) {
		return "", fmt.Errorf("%w: invalid preallocated session id %q", ErrValidation, sessionID)
	}
	return sessionID, nil
}

// ResolveSessionCWD normalizes a creation CWD and rejects execution-root escape.
func ResolveSessionCWD(root string, requested string) (string, error) {
	target, err := resolveContainedDirectory(root, requested)
	if err != nil {
		return "", fmt.Errorf(
			"%w: session cwd escapes root: %w",
			ErrValidation,
			err,
		)
	}
	return target, nil
}
