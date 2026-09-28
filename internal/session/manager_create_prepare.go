package session

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/store"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func (m *Manager) prepareCreateStart(ctx context.Context, opts CreateOpts) (sessionStartSpec, error) {
	opts, err := m.dispatchSessionPreCreate(ctx, opts)
	if err != nil {
		return sessionStartSpec{}, err
	}
	sessionType := normalizeSessionType(opts.Type)
	if err := validateCoordinatorExecutionTarget(opts, sessionType); err != nil {
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
	lineage, err := m.normalizeCreateOptsLineage(ctx, sessionID, sessionType, location.workspace.ID, opts)
	if err != nil {
		return sessionStartSpec{}, err
	}
	requestedSpeed, acpOptions, err := normalizeCreateRuntimeOptions(opts)
	if err != nil {
		return sessionStartSpec{}, err
	}
	return sessionStartSpec{
		sessionID:               sessionID,
		profileID:               normalizeCreateProfileID(opts.ProfileID),
		sessionName:             strings.TrimSpace(opts.Name),
		agentName:               strings.TrimSpace(agentName),
		provider:                strings.TrimSpace(opts.Provider),
		model:                   strings.TrimSpace(opts.Model),
		command:                 strings.TrimSpace(opts.Command),
		chainOwner:              opts.ChainOwner,
		reasoningEffort:         strings.TrimSpace(opts.ReasoningEffort),
		speed:                   requestedSpeed,
		acpOptions:              acpOptions,
		permissions:             opts.Permissions,
		workspace:               location.workspace,
		worktreeID:              location.worktreeID,
		worktreeRoot:            location.worktreeRoot,
		cwd:                     location.cwd,
		promptOverlay:           strings.TrimSpace(opts.PromptOverlay),
		contractOverlay:         strings.TrimSpace(opts.ContractOverlay),
		runtimeMode:             strings.TrimSpace(opts.RuntimeMode),
		sessionType:             sessionType,
		lineage:                 lineage,
		allowedToolsOverride:    append([]string(nil), opts.AllowedToolsOverride...),
		deniedToolsOverride:     append([]string(nil), opts.DeniedToolsOverride...),
		creationProfile:         cloneCreationProfile(opts.CreationProfile),
		creationIdentity:        cloneCreationIdentity(opts.CreationIdentity),
		creationIdentityPinned:  opts.CreationProfile != nil || opts.CreationIdentity != nil,
		creationIdentityEnabled: true,
		discardStartFailure:     opts.DiscardStartFailure,
		parentSoulDigest:        strings.TrimSpace(opts.ParentSoulDigest),
		postEvent:               hookspkg.HookSessionPostCreate,
		startAction:             sessionStartActionCreate,
		cleanupSessionDir:       true,
	}, nil
}

func normalizeCreateProfileID(profileID string) string {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return store.DefaultProfileID
	}
	return profileID
}

type createLocation struct {
	workspace    workspacepkg.ResolvedWorkspace
	worktreeID   string
	worktreeRoot string
	cwd          string
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
		cwd: cwd,
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
	if sessionID == "." || sessionID == ".." ||
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
