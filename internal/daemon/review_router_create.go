package daemon

import (
	"context"
	"errors"

	"sort"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/session"
	taskpkg "github.com/compozy/agh/internal/task"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func (r *reviewRouter) createRoute(
	ctx context.Context,
	taskRecord taskpkg.Task,
	run taskpkg.Run,
	review *taskpkg.ReviewProfile,
	original originalWorkerIdentity,
	requester originalWorkerIdentity,
	resolved *workspacepkg.ResolvedWorkspace,
) (*session.CreateOpts, string, error) {
	if len(review.AllowedPeerIDs) > 0 {
		return nil, "review profile allows only explicit peers and no active eligible peer is available", nil
	}
	if len(review.AllowedChannelIDs) > 0 {
		return nil, "review profile allows only explicit channels and no active eligible reviewer is available", nil
	}
	agentName, diagnostic, err := r.selectCreateAgent(ctx, review, original, requester, resolved)
	if err != nil || diagnostic != "" {
		return nil, diagnostic, err
	}
	promptOverlay := reviewRouterPromptOverlay(taskRecord.ID, run.ID)
	if r.contextOverlay != nil {
		taskOverlay, err := r.contextOverlay.TaskRunPromptOverlay(ctx, taskRecord, run, nil)
		if err != nil {
			return nil, "reviewer task context render failed: " + err.Error(), err
		}
		promptOverlay = joinPromptOverlays(taskOverlay, promptOverlay)
	}
	return &session.CreateOpts{
		Name:          reviewSessionName(taskRecord.ID),
		AgentName:     agentName,
		Provider:      review.Provider,
		Model:         review.Model,
		Workspace:     taskRecord.WorkspaceID,
		Type:          session.SessionTypeSystem,
		PromptOverlay: promptOverlay,
	}, "", nil
}

func (r *reviewRouter) selectCreateAgent(
	ctx context.Context,
	review *taskpkg.ReviewProfile,
	original originalWorkerIdentity,
	requester originalWorkerIdentity,
	resolved *workspacepkg.ResolvedWorkspace,
) (string, string, error) {
	candidates := reviewCreateAgentCandidates(review, resolved)
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if strings.TrimSpace(original.agentName) != "" && candidate == strings.TrimSpace(original.agentName) {
			continue
		}
		if strings.TrimSpace(requester.agentName) != "" && candidate == strings.TrimSpace(requester.agentName) {
			continue
		}
		ok, err := r.agentHasCapabilities(ctx, resolved, candidate, review.RequiredCapabilities)
		if err != nil {
			if errors.Is(err, workspacepkg.ErrAgentNotAvailable) {
				continue
			}
			return "", "", err
		}
		if ok {
			return strings.TrimSpace(candidate), "", nil
		}
	}
	if len(review.RequiredCapabilities) > 0 {
		return "", "no reviewer agent satisfies required capabilities", nil
	}
	if strings.TrimSpace(review.AgentName) != "" || len(review.AllowedAgentNames) > 0 {
		return "", "review profile agent selectors exclude all eligible reviewer agents", nil
	}
	if resolved != nil {
		for _, agent := range sortedResolvedAgents(resolved) {
			agentName := strings.TrimSpace(agent.Name)
			if agentName == "" {
				continue
			}
			if strings.TrimSpace(original.agentName) != "" && agentName == strings.TrimSpace(original.agentName) {
				continue
			}
			if strings.TrimSpace(requester.agentName) != "" && agentName == strings.TrimSpace(requester.agentName) {
				continue
			}
			return agentName, "", nil
		}
	}
	return "", "", nil
}

func reviewCreateAgentCandidates(
	review *taskpkg.ReviewProfile,
	resolved *workspacepkg.ResolvedWorkspace,
) []string {
	values := make([]string, 0)
	if strings.TrimSpace(review.AgentName) != "" {
		values = append(values, strings.TrimSpace(review.AgentName))
	}
	values = append(values, review.PreferredAgentNames...)
	values = append(values, review.AllowedAgentNames...)
	if len(values) == 0 && len(review.RequiredCapabilities) > 0 && resolved != nil {
		for _, agent := range sortedResolvedAgents(resolved) {
			values = append(values, agent.Name)
		}
	}
	return uniqueTrimmedStrings(values)
}

func sortedResolvedAgents(resolved *workspacepkg.ResolvedWorkspace) []aghconfig.AgentDef {
	if resolved == nil {
		return nil
	}
	agents := append([]aghconfig.AgentDef(nil), resolved.Agents...)
	sort.SliceStable(agents, func(i int, j int) bool {
		return strings.TrimSpace(agents[i].Name) < strings.TrimSpace(agents[j].Name)
	})
	return agents
}

func (r *reviewRouter) agentHasCapabilities(
	_ context.Context,
	resolved *workspacepkg.ResolvedWorkspace,
	agentName string,
	required []string,
) (bool, error) {
	return agentCoversCapabilities(r.agents, resolved, agentName, required)
}

func (r *reviewRouter) resolveWorkspace(
	ctx context.Context,
	workspaceID string,
) (*workspacepkg.ResolvedWorkspace, error) {
	trimmed := strings.TrimSpace(workspaceID)
	if trimmed == "" || r.workspaces == nil {
		return nil, nil
	}
	resolved, err := r.workspaces.Resolve(ctx, trimmed)
	if err != nil {
		return nil, err
	}
	return &resolved, nil
}
