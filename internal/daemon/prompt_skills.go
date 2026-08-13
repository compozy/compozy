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

type skillsCatalogAugmenter struct {
	registry          promptSkillsRegistry
	agentResolver     func() session.AgentResolver
	workspaceResolver func() promptSkillsWorkspaceResolver
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
) session.PromptInputAugmenter {
	if registry == nil {
		return nil
	}

	augmenter := &skillsCatalogAugmenter{
		registry:          registry,
		agentResolver:     agentResolver,
		workspaceResolver: workspaceResolver,
		states:            make(map[string]skillsCatalogSessionState),
	}
	return augmenter.Augment
}

func (a *skillsCatalogAugmenter) Augment(ctx context.Context, sess *session.Session, message string) (string, error) {
	if a == nil || a.registry == nil || sess == nil {
		return message, nil
	}

	info := sess.Info()
	if info == nil {
		return message, nil
	}

	workspace, err := resolvePromptSkillsWorkspace(ctx, a.workspaceResolver, info.WorkspaceID, info.Workspace)
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

	catalog := skillspkg.BuildCurrentCatalog(skills)
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
	workspaceID string,
	workspaceRoot string,
) (*workspacepkg.ResolvedWorkspace, error) {
	target := firstTrimmed(workspaceID, workspaceRoot)
	var resolver promptSkillsWorkspaceResolver
	if resolverGetter != nil {
		resolver = resolverGetter()
	}
	if resolver != nil && target != "" {
		resolved, err := resolver.Resolve(ctx, target)
		if err == nil {
			return &resolved, nil
		}
		if isContextError(err) {
			return nil, err
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
