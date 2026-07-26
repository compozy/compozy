package session

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/network/participation"
)

func (m *Manager) prepareCreateStart(ctx context.Context, opts CreateOpts) (sessionStartSpec, error) {
	opts, err := m.dispatchSessionPreCreate(ctx, opts)
	if err != nil {
		return sessionStartSpec{}, err
	}

	resolvedWorkspace, err := m.resolveCreateWorkspace(ctx, opts)
	if err != nil {
		return sessionStartSpec{}, err
	}
	sandboxDisabled, err := applyCreateSandboxOverride(&resolvedWorkspace, opts)
	if err != nil {
		return sessionStartSpec{}, err
	}
	cwd, err := ResolveSessionCWD(resolvedWorkspace.RootDir, opts.CWD)
	if err != nil {
		return sessionStartSpec{}, err
	}

	agentName, err := aghconfig.ResolveAgentName(opts.AgentName, resolvedWorkspace.Config.Defaults)
	if err != nil {
		return sessionStartSpec{}, fmt.Errorf("session: resolve agent name: %w", err)
	}
	sessionType := normalizeSessionType(opts.Type)
	sessionID, err := m.createSessionID(opts.DesiredSessionID)
	if err != nil {
		return sessionStartSpec{}, err
	}
	sandboxID := strings.TrimSpace(m.newSandboxID())
	if sandboxID == "" {
		return sessionStartSpec{}, errors.New("session: sandbox id generator returned empty id")
	}
	lineage, err := m.normalizeCreateLineage(ctx, sessionID, sessionType, opts.Lineage)
	if err != nil {
		return sessionStartSpec{}, err
	}
	networkParticipation, networkOwnerKey, participationObservation, err := m.prepareCreateParticipation(
		ctx, opts, resolvedWorkspace.ID, sessionID,
	)
	if err != nil {
		return sessionStartSpec{}, err
	}

	return sessionStartSpec{
		sessionID:                sessionID,
		sandboxID:                sandboxID,
		sessionName:              strings.TrimSpace(opts.Name),
		agentName:                strings.TrimSpace(agentName),
		provider:                 strings.TrimSpace(opts.Provider),
		model:                    strings.TrimSpace(opts.Model),
		reasoningEffort:          strings.TrimSpace(opts.ReasoningEffort),
		permissions:              opts.Permissions,
		sandboxDisabled:          sandboxDisabled,
		workspace:                resolvedWorkspace,
		cwd:                      cwd,
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
		parentSoulDigest:         strings.TrimSpace(opts.ParentSoulDigest),
		postEvent:                hookspkg.HookSessionPostCreate,
		startAction:              sessionStartActionCreate,
		cleanupSessionDir:        true,
	}, nil
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
		sessionID = strings.TrimSpace(m.newSessionID())
		if sessionID == "" {
			return "", errors.New("session: session id generator returned empty id")
		}
	}
	if len(sessionID) > 128 || sessionID == "." || sessionID == ".." || filepath.Base(sessionID) != sessionID ||
		strings.ContainsAny(sessionID, `/\\`) {
		return "", fmt.Errorf("%w: invalid preallocated session id %q", ErrValidation, sessionID)
	}
	return sessionID, nil
}

// ResolveSessionCWD normalizes a creation CWD and rejects workspace escape.
func ResolveSessionCWD(root string, requested string) (string, error) {
	target, err := resolveContainedDirectory(root, requested)
	if err != nil {
		return "", fmt.Errorf(
			"%w: session cwd must remain within the workspace: %w",
			ErrValidation,
			err,
		)
	}
	return target, nil
}
