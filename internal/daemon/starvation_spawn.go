package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/admission"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

// An unresolvable worker leaves the remaining escalation tiers available.
var errStarvationSpawnUnresolvable = errors.New("daemon: starvation has no eligible replacement worker")

// starvationSpawner resolves a deterministic capability-matched agent for a starved run, reusing
// the same workspace + agent resolution stack the review router uses.
type starvationSpawner struct {
	workspaces reviewRouterWorkspaceResolver
	agents     reviewRouterAgentResolver
}

func (r *taskRoleRuntime) resumeStarvationOwner(
	ctx context.Context,
	taskRecord taskpkg.Task,
	run taskpkg.Run,
	spawner starvationSpawner,
) error {
	ownerID := strings.TrimSpace(taskRecord.Owner.Ref)
	if ownerID == "" {
		return errStarvationSpawnUnresolvable
	}
	owner, err := storedSessionStatus(ctx, r.sessions, ownerID)
	if errors.Is(err, session.ErrSessionNotFound) {
		return errStarvationSpawnUnresolvable
	}
	if err != nil {
		return fmt.Errorf("daemon: read starvation owner %q: %w", ownerID, err)
	}
	if owner == nil || owner.ID != ownerID || !starvationOwnerResumable(taskRecord, owner) {
		return errStarvationSpawnUnresolvable
	}
	if required := trimmedNonEmptyStrings(run.RequiredCapabilities); len(required) > 0 {
		resolved, err := resolveWorkspaceForSpawn(ctx, spawner.workspaces, owner.WorkspaceID)
		if err != nil {
			return fmt.Errorf("daemon: resolve starvation owner workspace: %w", err)
		}
		covered, err := agentCoversCapabilities(spawner.agents, resolved, owner.AgentName, required)
		if errors.Is(err, workspacepkg.ErrAgentNotAvailable) || (err == nil && !covered) {
			return errStarvationSpawnUnresolvable
		}
		if err != nil {
			return fmt.Errorf("daemon: resolve starvation owner capabilities: %w", err)
		}
	}
	resumer, ok := r.sessions.(interface {
		ResumeWaitingSession(context.Context, string) error
	})
	if !ok {
		return errStarvationSpawnUnresolvable
	}
	// Restart recovery validates the durable binding. The next scheduler wake dispatches the run.
	err = resumer.ResumeWaitingSession(admission.WithBackgroundWork(ctx), ownerID)
	if err != nil {
		if starvationOwnerResumeRefused(err) {
			return errStarvationSpawnUnresolvable
		}
		return fmt.Errorf("daemon: resume starvation owner %q: %w", ownerID, err)
	}
	r.logger.InfoContext(ctx, "daemon: starvation owner resumed",
		"session_id", ownerID, taskRoleRuntimeTaskIDKey, taskRecord.ID,
		daemonLogRunIDKey, run.ID, "agent_name", owner.AgentName,
	)
	return nil
}

func starvationOwnerResumeRefused(err error) bool {
	// A joined error can include a failure stopping a resumed session, which must surface.
	_, ineligible := err.(*session.MonitorOwnerIneligibleError)
	return ineligible || errors.Is(err, session.ErrMonitorOwnerStopped) ||
		errors.Is(err, session.ErrSessionNotFound) || errors.Is(err, store.ErrSessionNotAttachable) ||
		errors.Is(err, session.ErrSessionInLater) || errors.Is(err, session.ErrSessionArchived) ||
		errors.Is(err, session.ErrPromptInProgress)
}

func starvationOwnerResumable(taskRecord taskpkg.Task, owner *session.Info) bool {
	if owner.State != session.StateStopped || owner.Type != session.SessionTypeUser ||
		owner.StopReason != store.StopShutdown ||
		owner.ArchivedAt != nil || owner.Later != nil || owner.StopVerificationFailed ||
		(owner.Lineage != nil && (owner.Lineage.AutoStopOnParent || owner.Lineage.TTLExpiresAt != nil)) {
		return false
	}
	if firstNonEmpty(
		owner.ProfileID,
		store.DefaultProfileID,
	) != firstNonEmpty(
		taskRecord.ProfileID,
		store.DefaultProfileID,
	) {
		return false
	}
	workspaceID := strings.TrimSpace(owner.WorkspaceID)
	switch taskRecord.Scope.Normalize() {
	case taskpkg.ScopeWorkspace:
		return workspaceID != "" && workspaceID == strings.TrimSpace(taskRecord.WorkspaceID)
	case taskpkg.ScopeGlobal:
		return workspaceID == ""
	default:
		return false
	}
}

// resolveAgent returns the lexicographically-first agent whose capabilities cover required. ok is
// false (with nil error) when nothing covers the set, which the caller treats as unresolvable.
func (s starvationSpawner) resolveAgent(
	ctx context.Context,
	workspaceID string,
	required []string,
) (string, bool, error) {
	required = trimmedNonEmptyStrings(required)
	resolved, err := resolveWorkspaceForSpawn(ctx, s.workspaces, workspaceID)
	if err != nil {
		return "", false, err
	}
	for _, agent := range sortedResolvedAgents(resolved) {
		ok, err := agentCoversCapabilities(s.agents, resolved, agent.Name, required)
		if err != nil {
			if errors.Is(err, workspacepkg.ErrAgentNotAvailable) {
				continue
			}
			return "", false, err
		}
		if ok {
			return strings.TrimSpace(agent.Name), true, nil
		}
	}
	return "", false, nil
}

func resolveWorkspaceForSpawn(
	ctx context.Context,
	resolver reviewRouterWorkspaceResolver,
	workspaceID string,
) (*workspacepkg.ResolvedWorkspace, error) {
	trimmed := strings.TrimSpace(workspaceID)
	if trimmed == "" || resolver == nil {
		return nil, nil
	}
	resolved, err := resolver.Resolve(ctx, trimmed)
	if err != nil {
		return nil, err
	}
	return &resolved, nil
}

func agentCoversCapabilities(
	resolver reviewRouterAgentResolver,
	resolved *workspacepkg.ResolvedWorkspace,
	agentName string,
	required []string,
) (bool, error) {
	if len(required) == 0 {
		return true, nil
	}
	agentName = strings.TrimSpace(agentName)
	if agentName == "" {
		return false, nil
	}
	agent, err := resolveAgentDef(resolver, agentName, resolved)
	if err != nil {
		return false, err
	}
	available := make(map[string]struct{})
	if agent.Capabilities != nil {
		for _, capability := range agent.Capabilities.Capabilities {
			id := strings.TrimSpace(capability.ID)
			if id != "" {
				available[id] = struct{}{}
			}
		}
	}
	for _, capability := range required {
		if _, ok := available[strings.TrimSpace(capability)]; !ok {
			return false, nil
		}
	}
	return true, nil
}

func resolveAgentDef(
	resolver reviewRouterAgentResolver,
	agentName string,
	resolved *workspacepkg.ResolvedWorkspace,
) (compozyconfig.AgentDef, error) {
	if resolver != nil {
		return resolver.ResolveAgent(agentName, resolved)
	}
	if resolved != nil {
		for _, agent := range resolved.Agents {
			if strings.TrimSpace(agent.Name) == strings.TrimSpace(agentName) {
				return agent, nil
			}
		}
	}
	return compozyconfig.AgentDef{}, fmt.Errorf("%w: %s", workspacepkg.ErrAgentNotAvailable, agentName)
}

func trimmedNonEmptyStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
