package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/compozy/agh/internal/acp"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/memory"
	memcontract "github.com/compozy/agh/internal/memory/contract"
	"github.com/compozy/agh/internal/session"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func failureCandidateMetadata(content string) (sessionID string, workspaceID string, agentName string) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			continue
		}
		var candidate memcontract.Candidate
		if err := json.Unmarshal([]byte(raw), &candidate); err != nil {
			continue
		}
		if candidate.Metadata != nil {
			sessionID = firstNonEmptyString(sessionID, candidate.Metadata["session_id"])
		}
		workspaceID = firstNonEmptyString(workspaceID, candidate.WorkspaceID)
		agentName = firstNonEmptyString(agentName, candidate.AgentName)
		if sessionID != "" || workspaceID != "" || agentName != "" {
			return sessionID, workspaceID, agentName
		}
	}
	return "", "", ""
}

type daemonMemoryProposalSink struct {
	base              *memory.Store
	workspaceResolver workspacepkg.RuntimeResolver
}

func (s *daemonMemoryProposalSink) ProposeCandidate(
	ctx context.Context,
	candidate memcontract.Candidate,
) (memcontract.Decision, error) {
	if s == nil || s.base == nil {
		return memcontract.Decision{}, errors.New("daemon: memory store is not configured")
	}
	target, candidate, err := s.targetStore(ctx, candidate)
	if err != nil {
		return memcontract.Decision{}, err
	}
	return target.ProposeCandidate(ctx, candidate)
}

func (s *daemonMemoryProposalSink) targetStore(
	ctx context.Context,
	candidate memcontract.Candidate,
) (*memory.Store, memcontract.Candidate, error) {
	scope := candidate.Scope.Normalize()
	if scope == "" {
		scope = candidate.Frontmatter.Scope.Normalize()
	}
	switch scope {
	case "", memcontract.ScopeGlobal:
		candidate.Scope = memcontract.ScopeGlobal
		candidate.Frontmatter.Scope = memcontract.ScopeGlobal
		return s.base, candidate, nil
	case memcontract.ScopeWorkspace, memcontract.ScopeAgent:
		workspaceRoot, workspaceID, err := s.resolveWorkspace(ctx, candidate)
		if err != nil {
			return nil, candidate, err
		}
		candidate.WorkspaceID = workspaceID
		candidate.Scope = scope
		candidate.Frontmatter.Scope = scope
		store := s.base.ForWorkspace(workspaceRoot)
		if scope == memcontract.ScopeAgent {
			tier := candidate.AgentTier.Normalize()
			if tier == "" {
				tier = memcontract.AgentTierWorkspace
			}
			candidate.AgentTier = tier
			candidate.Frontmatter.AgentTier = tier
			store = store.ForAgent(workspaceID, candidate.AgentName, tier)
		}
		return store, candidate, nil
	default:
		return nil, candidate, fmt.Errorf("daemon: unsupported memory scope %q", candidate.Scope)
	}
}

func (s *daemonMemoryProposalSink) resolveWorkspace(
	ctx context.Context,
	candidate memcontract.Candidate,
) (root string, workspaceID string, err error) {
	if candidate.Metadata != nil {
		root = strings.TrimSpace(candidate.Metadata["workspace_root"])
	}
	workspaceID = strings.TrimSpace(candidate.WorkspaceID)
	if root != "" {
		identity, identityErr := workspacepkg.EnsureIdentity(ctx, root)
		if identityErr != nil {
			return "", "", fmt.Errorf("daemon: resolve memory candidate workspace identity: %w", identityErr)
		}
		return root, identity.WorkspaceID, nil
	}
	if workspaceID == "" {
		return "", "", errors.New("daemon: workspace memory candidate requires workspace id")
	}
	if s.workspaceResolver == nil {
		return "", "", errors.New("daemon: workspace resolver is not configured")
	}
	resolved, err := s.workspaceResolver.Resolve(ctx, workspaceID)
	if err != nil {
		return "", "", fmt.Errorf("daemon: resolve memory candidate workspace %q: %w", workspaceID, err)
	}
	return resolved.RootDir, firstNonEmptyString(resolved.WorkspaceID, resolved.ID, workspaceID), nil
}

type forkedMemoryExtractor struct {
	sessions       memoryExtractorSessionManager
	roles          RoleResolver
	deadline       time.Duration
	logger         *slog.Logger
	now            func() time.Time
	workspaceRoots *sync.Map
}

func (e *forkedMemoryExtractor) Extract(
	ctx context.Context,
	turn memcontract.TurnRecord,
) ([]memcontract.Candidate, error) {
	if e == nil || e.sessions == nil {
		return nil, errors.New("daemon: memory extractor sessions are not configured")
	}
	if e.roles == nil {
		return nil, errors.New("daemon: memory extractor role resolver is not configured")
	}
	correlation := roleInvocationCorrelation{
		WorkspaceID:     strings.TrimSpace(turn.WorkspaceID),
		SessionID:       strings.TrimSpace(turn.SessionID),
		AgentName:       strings.TrimSpace(turn.AgentID),
		ParentSessionID: strings.TrimSpace(turn.ParentSessionID),
		RootSessionID:   strings.TrimSpace(turn.RootSessionID),
	}
	roleCtx := withRoleInvocationCorrelation(ctx, correlation)
	role, err := e.roles.Resolve(roleCtx, turn.WorkspaceID, aghconfig.RoleMemoryExtractor)
	if err != nil {
		return nil, fmt.Errorf("daemon: resolve memory extractor role: %w", err)
	}
	if !role.Enabled {
		return nil, nil
	}
	if role.Inherit {
		role.AgentName = strings.TrimSpace(turn.AgentID)
	}
	runCtx := context.WithoutCancel(ctx)
	if e.deadline > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(runCtx, e.deadline)
		defer cancel()
	}
	prompt, err := renderMemoryExtractorPrompt(turn)
	if err != nil {
		return nil, err
	}
	child, err := e.spawnExtractorSession(runCtx, role, correlation, turn)
	if child != nil {
		defer e.stopChild(ctx, child.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("daemon: spawn memory extractor session: %w", err)
	}

	events, err := e.sessions.PromptSynthetic(runCtx, child.ID, session.SyntheticPromptOpts{
		Message: prompt,
		Metadata: acp.PromptSyntheticMeta{
			TaskID:  memoryExtractorSyntheticTaskID,
			Reason:  "memory_extractor",
			Summary: "extract durable Memory v2 candidates",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("daemon: prompt memory extractor session: %w", err)
	}
	output, err := collectMemoryExtractorOutput(runCtx, events)
	if err != nil {
		return nil, err
	}
	candidates, err := parseMemoryExtractorCandidates(output, turn, e.workspaceRoot(turn.SessionID), e.nowUTC())
	if err != nil {
		return nil, err
	}
	return candidates, nil
}

func (e *forkedMemoryExtractor) spawnExtractorSession(
	ctx context.Context,
	role ResolvedRole,
	correlation roleInvocationCorrelation,
	turn memcontract.TurnRecord,
) (*session.Session, error) {
	return invokeRoleWithFallback(ctx, role, correlation, func(
		attemptCtx context.Context,
		route roleAttemptRoute,
	) (*session.Session, bool, error) {
		spawned, err := e.sessions.Spawn(attemptCtx, session.SpawnOpts{
			ParentSessionID:    turn.SessionID,
			AgentName:          route.AgentName,
			Provider:           route.Provider,
			Model:              route.Model,
			ReasoningEffort:    route.ReasoningEffort,
			Name:               "Memory extractor",
			PromptOverlay:      memoryExtractorOverlay(),
			SpawnRole:          session.SpawnRoleMemoryExtractor,
			TTL:                e.extractorTTL(),
			AllowStoppedParent: true,
		})
		return spawned, spawned != nil, err
	})
}

func (e *forkedMemoryExtractor) Drain(context.Context) error {
	return nil
}

func (e *forkedMemoryExtractor) extractorTTL() time.Duration {
	if e.deadline > 0 {
		return e.deadline + memoryExtractorStopTimeout
	}
	return 2 * time.Minute
}

func (e *forkedMemoryExtractor) stopChild(parentCtx context.Context, id string) {
	stopCtx, cancel := context.WithTimeout(context.WithoutCancel(parentCtx), memoryExtractorStopTimeout)
	defer cancel()
	if err := e.sessions.StopWithCause(stopCtx, id, session.CauseCompleted, "memory extractor completed"); err != nil &&
		e.logger != nil {
		e.logger.Warn("daemon: stop memory extractor child failed", "session_id", id, "error", err)
	}
}

func (e *forkedMemoryExtractor) workspaceRoot(sessionID string) string {
	if e == nil || e.workspaceRoots == nil {
		return ""
	}
	defer e.workspaceRoots.Delete(sessionID)
	value, ok := e.workspaceRoots.Load(sessionID)
	if !ok {
		return ""
	}
	root, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(root)
}

func (e *forkedMemoryExtractor) nowUTC() time.Time {
	if e != nil && e.now != nil {
		return e.now().UTC()
	}
	return time.Now().UTC()
}
