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
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

type createWorkspaceContext struct {
	workspace       workspacepkg.ResolvedWorkspace
	worktreeID      string
	worktreeRoot    string
	sandboxDisabled bool
	cwd             string
	agentName       string
}

func (m *Manager) prepareCreateStart(ctx context.Context, opts CreateOpts) (sessionStartSpec, error) {
	opts, err := m.dispatchSessionPreCreate(ctx, opts)
	if err != nil {
		return sessionStartSpec{}, err
	}
	sessionType := normalizeSessionType(opts.Type)
	if err := validateCoordinatorExecutionTarget(opts, sessionType); err != nil {
		return sessionStartSpec{}, err
	}

	workspaceContext, err := m.prepareCreateWorkspaceContext(ctx, opts)
	if err != nil {
		return sessionStartSpec{}, err
	}
	sessionID, err := m.createSessionID(opts.DesiredSessionID)
	if err != nil {
		return sessionStartSpec{}, err
	}
	sandboxID, err := m.prepareCreateSandboxID(workspaceContext.sandboxDisabled)
	if err != nil {
		return sessionStartSpec{}, err
	}
	lineage, err := m.normalizeCreateLineage(ctx, sessionID, sessionType, workspaceContext.workspace.ID, opts.Lineage)
	if err != nil {
		return sessionStartSpec{}, err
	}
	networkParticipation, networkOwnerKey, participationObservation, err := m.prepareCreateParticipation(
		ctx, opts, workspaceContext.workspace.ID, sessionID,
	)
	if err != nil {
		return sessionStartSpec{}, err
	}
	if networkParticipation.Mode == participation.ModeLive {
		if err := validateLiveNetworkPeerID(workspaceContext.agentName, sessionID); err != nil {
			return sessionStartSpec{}, err
		}
	}
	requestedSpeed, err := normalizeRequestedSpeed(opts.Speed)
	if err != nil {
		return sessionStartSpec{}, fmt.Errorf("%w: %w", ErrInvalidRuntimeOverride, err)
	}

	return sessionStartSpec{
		sessionID:                sessionID,
		sandboxID:                sandboxID,
		sessionName:              strings.TrimSpace(opts.Name),
		agentName:                strings.TrimSpace(workspaceContext.agentName),
		provider:                 strings.TrimSpace(opts.Provider),
		model:                    strings.TrimSpace(opts.Model),
		reasoningEffort:          strings.TrimSpace(opts.ReasoningEffort),
		speed:                    requestedSpeed,
		permissions:              opts.Permissions,
		sandboxDisabled:          workspaceContext.sandboxDisabled,
		workspace:                workspaceContext.workspace,
		worktreeID:               workspaceContext.worktreeID,
		worktreeRoot:             workspaceContext.worktreeRoot,
		cwd:                      workspaceContext.cwd,
		networkParticipation:     networkParticipation,
		networkOwnerKey:          networkOwnerKey,
		participationObservation: participationObservation,
		promptOverlay:            strings.TrimSpace(opts.PromptOverlay),
		contractOverlay:          strings.TrimSpace(opts.ContractOverlay),
		runtimeMode:              strings.TrimSpace(opts.RuntimeMode),
		sessionType:              sessionType,
		lineage:                  lineage,
		allowedToolsOverride:     append([]string(nil), opts.AllowedToolsOverride...),
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

func (m *Manager) prepareCreateWorkspaceContext(
	ctx context.Context,
	opts CreateOpts,
) (createWorkspaceContext, error) {
	resolvedWorkspace, err := m.resolveCreateWorkspace(ctx, opts)
	if err != nil {
		return createWorkspaceContext{}, err
	}
	worktreeID, worktreeRoot, err := m.resolveSessionWorktree(ctx, resolvedWorkspace.ID, opts.Worktree)
	if err != nil {
		return createWorkspaceContext{}, err
	}
	sandboxDisabled, err := applyCreateSandboxOverride(&resolvedWorkspace, opts)
	if err != nil {
		return createWorkspaceContext{}, err
	}
	executionRoot := resolvedWorkspace.RootDir
	if worktreeRoot != "" {
		executionRoot = worktreeRoot
	}
	cwd, err := ResolveSessionCWD(executionRoot, opts.CWD)
	if err != nil {
		return createWorkspaceContext{}, err
	}
	agentName, err := compozyconfig.ResolveAgentName(opts.AgentName, resolvedWorkspace.Config.Defaults)
	if err != nil {
		return createWorkspaceContext{}, fmt.Errorf("session: resolve agent name: %w", err)
	}
	return createWorkspaceContext{
		workspace:       resolvedWorkspace,
		worktreeID:      worktreeID,
		worktreeRoot:    worktreeRoot,
		sandboxDisabled: sandboxDisabled,
		cwd:             cwd,
		agentName:       agentName,
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
