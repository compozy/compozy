package daemon

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/session"
	skillspkg "github.com/compozy/compozy/internal/skills"
	"github.com/compozy/compozy/internal/store"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

const promptSkillsCatalogCacheMaxSessions = 2048

type promptSkillsRegistry interface {
	ForWorkspace(ctx context.Context, resolved *workspacepkg.ResolvedWorkspace) ([]*skillspkg.Skill, error)
	ForAgentSession(
		ctx context.Context,
		resolved *workspacepkg.ResolvedWorkspace,
		agentName string,
		sessionID string,
	) ([]*skillspkg.Skill, error)
	ForAgentDefSession(
		ctx context.Context,
		resolved *workspacepkg.ResolvedWorkspace,
		agent compozyconfig.AgentDef,
		sessionID string,
	) ([]*skillspkg.Skill, error)
}

type promptSkillsWorkspaceResolver interface {
	Resolve(ctx context.Context, target string) (workspacepkg.ResolvedWorkspace, error)
}

type promptSkillsProfileWorkspaceResolver interface {
	ResolveForProfile(
		ctx context.Context,
		target string,
		profileName string,
	) (workspacepkg.ResolvedWorkspace, error)
}

type skillsCatalogAugmenter struct {
	registry          promptSkillsRegistry
	agentResolver     func() session.AgentResolver
	workspaceResolver func() promptSkillsWorkspaceResolver
	profileNames      session.ProfileNameResolver
	sequence          atomic.Uint64

	mu     sync.Mutex
	states map[string]skillsCatalogSessionState
}

type skillsCatalogSessionState struct {
	acpSessionID string
	signature    [sha256.Size]byte
	lastUsed     uint64
}

func newSkillsCatalogAugmenter(
	registry promptSkillsRegistry,
	agentResolver func() session.AgentResolver,
	workspaceResolver func() promptSkillsWorkspaceResolver,
	profileNames session.ProfileNameResolver,
) session.PromptInputAugmenter {
	augmenter := newSkillsCatalogAugmenterState(registry, agentResolver, workspaceResolver, profileNames)
	if augmenter == nil {
		return nil
	}
	return augmenter.Augment
}

func newSkillsCatalogAugmenterState(
	registry promptSkillsRegistry,
	agentResolver func() session.AgentResolver,
	workspaceResolver func() promptSkillsWorkspaceResolver,
	profileNames session.ProfileNameResolver,
) *skillsCatalogAugmenter {
	if registry == nil {
		return nil
	}

	augmenter := &skillsCatalogAugmenter{
		registry:          registry,
		agentResolver:     agentResolver,
		workspaceResolver: workspaceResolver,
		profileNames:      profileNames,
		states:            make(map[string]skillsCatalogSessionState),
	}
	return augmenter
}

func (a *skillsCatalogAugmenter) Augment(ctx context.Context, sess *session.Session, message string) (string, error) {
	return a.augment(ctx, sess, message, nil)
}

func (a *skillsCatalogAugmenter) AugmentWithPolicy(
	ctx context.Context,
	sess *session.Session,
	message string,
	resolved ResolvedHarnessContext,
) (string, error) {
	return a.augment(ctx, sess, message, resolved.Policy.SkillInjectionFilter)
}

func (a *skillsCatalogAugmenter) augment(
	ctx context.Context,
	sess *session.Session,
	message string,
	filter SkillInjectionFilter,
) (string, error) {
	if a == nil || a.registry == nil || sess == nil {
		return message, nil
	}

	info := sess.Info()
	if info == nil {
		return message, nil
	}

	workspace, err := resolvePromptSkillsWorkspace(
		ctx, a.workspaceResolver, a.profileNames, info.ProfileID, info.WorkspaceID, info.Workspace,
	)
	if err != nil {
		return "", fmt.Errorf("daemon: resolve prompt skills workspace: %w", err)
	}

	var skills []*skillspkg.Skill
	agent := sess.AgentDefinition()
	if strings.TrimSpace(agent.Name) != "" {
		skills, err = a.skillsForSessionAgent(ctx, workspace, agent, info.ID)
	} else {
		skills, err = a.registry.ForWorkspace(ctx, workspace)
	}
	if err != nil {
		return "", fmt.Errorf("daemon: load current skills catalog for session %q: %w", info.ID, err)
	}

	filtered := make([]*skillspkg.Skill, 0, len(skills))
	for _, skill := range skills {
		if skill != nil && (filter == nil || filter(skill)) {
			filtered = append(filtered, skill)
		}
	}
	catalog := skillspkg.BuildCurrentCatalog(filtered)
	if strings.TrimSpace(catalog) == "" {
		a.forgetSession(info.ID)
		return message, nil
	}
	if a.catalogUnchanged(info, catalog) {
		catalog = skillspkg.BuildCurrentCatalogUnchanged()
	}
	if strings.TrimSpace(message) == "" {
		return catalog, nil
	}
	return catalog + "\n\n" + message, nil
}

func (a *skillsCatalogAugmenter) skillsForSessionAgent(
	ctx context.Context,
	workspace *workspacepkg.ResolvedWorkspace,
	agent compozyconfig.AgentDef,
	sessionID string,
) ([]*skillspkg.Skill, error) {
	return resolveSessionAgentProjection(sessionAgentProjection[[]*skillspkg.Skill]{
		agent:                 agent,
		hasConcreteDefinition: true,
		workspace:             workspace,
		agentResolver:         a.agentResolver,
		byName: func(agentName string) ([]*skillspkg.Skill, error) {
			return a.registry.ForAgentSession(ctx, workspace, agentName, sessionID)
		},
		byDefinition: func(resolvedAgent compozyconfig.AgentDef) ([]*skillspkg.Skill, error) {
			return a.registry.ForAgentDefSession(ctx, workspace, resolvedAgent, sessionID)
		},
	})
}

func (a *skillsCatalogAugmenter) catalogUnchanged(info *session.Info, catalog string) bool {
	if info == nil {
		return false
	}

	key := strings.TrimSpace(info.ID)
	if key == "" {
		return false
	}

	acpSessionID := strings.TrimSpace(info.ACPSessionID)
	signature := sha256.Sum256([]byte(catalog))
	sequence := a.sequence.Add(1)

	a.mu.Lock()
	defer a.mu.Unlock()

	state, ok := a.states[key]
	unchanged := ok && state.acpSessionID == acpSessionID && state.signature == signature
	a.states[key] = skillsCatalogSessionState{
		acpSessionID: acpSessionID,
		signature:    signature,
		lastUsed:     sequence,
	}
	a.evictOldestLocked()
	return unchanged
}

func (a *skillsCatalogAugmenter) forgetSession(sessionID string) {
	key := strings.TrimSpace(sessionID)
	if key == "" {
		return
	}

	a.mu.Lock()
	delete(a.states, key)
	a.mu.Unlock()
}

func (a *skillsCatalogAugmenter) evictOldestLocked() {
	if len(a.states) <= promptSkillsCatalogCacheMaxSessions {
		return
	}

	var oldestKey string
	var oldestSequence uint64
	for key, state := range a.states {
		if oldestKey == "" || state.lastUsed < oldestSequence {
			oldestKey = key
			oldestSequence = state.lastUsed
		}
	}
	if oldestKey != "" {
		delete(a.states, oldestKey)
	}
}

func resolvePromptSkillsWorkspace(
	ctx context.Context,
	resolverGetter func() promptSkillsWorkspaceResolver,
	profileNames session.ProfileNameResolver,
	profileID string,
	workspaceID string,
	workspaceRoot string,
) (*workspacepkg.ResolvedWorkspace, error) {
	target := firstTrimmed(workspaceID, workspaceRoot)
	var resolver promptSkillsWorkspaceResolver
	if resolverGetter != nil {
		resolver = resolverGetter()
	}
	if resolver != nil && target != "" {
		trimmedProfileID := strings.TrimSpace(profileID)
		var resolved workspacepkg.ResolvedWorkspace
		var err error
		if trimmedProfileID == "" || trimmedProfileID == store.DefaultProfileID {
			resolved, err = resolver.Resolve(ctx, target)
		} else {
			if profileNames == nil {
				return nil, errors.New("daemon: profile name resolver is required for session skill resolution")
			}
			profileName, profileErr := profileNames.ProfileName(ctx, trimmedProfileID)
			if profileErr != nil {
				return nil, fmt.Errorf("daemon: resolve session profile %q: %w", trimmedProfileID, profileErr)
			}
			profileResolver, ok := resolver.(promptSkillsProfileWorkspaceResolver)
			if !ok {
				return nil, errors.New("daemon: workspace resolver does not support profile layers")
			}
			resolved, err = profileResolver.ResolveForProfile(ctx, target, profileName)
		}
		if err == nil {
			if trimmedProfileID != "" {
				resolved.ProfileID = trimmedProfileID
			}
			return &resolved, nil
		}
		if isContextError(err) {
			return nil, err
		}
		if trimmedProfileID != "" && trimmedProfileID != store.DefaultProfileID {
			return nil, fmt.Errorf("daemon: resolve session profile workspace: %w", err)
		}
	}

	if target == "" {
		return nil, nil
	}
	return &workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{
			ID:      strings.TrimSpace(workspaceID),
			RootDir: strings.TrimSpace(workspaceRoot),
		},
		ProfileID: strings.TrimSpace(profileID),
	}, nil
}

func firstTrimmed(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func isContextError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
