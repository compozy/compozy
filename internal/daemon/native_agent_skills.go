package daemon

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/api/contract"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

func (n *daemonNativeTools) agentSkillBindings() map[toolspkg.ToolID]nativeToolBinding {
	available := n.dependencyAvailability(func() bool {
		return n.deps.AgentInstances != nil && n.deps.AgentInstances() != nil &&
			n.deps.AgentInstances().SkillProposals != nil
	})
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDAgentSkillPropose: {call: n.agentSkillPropose, availability: available},
		toolspkg.ToolIDAgentSkillReviews: {call: n.agentSkillReviews, availability: available},
	}
}

func (n *daemonNativeTools) ownSkillInstance(
	ctx context.Context,
	scope toolspkg.Scope,
) (agentinstance.Instance, workspaceaccess.ActorRef, error) {
	api := n.deps.AgentInstances()
	if api == nil || api.Authority.Callers == nil {
		return agentinstance.Instance{}, workspaceaccess.ActorRef{}, errors.New("skill proposals unavailable")
	}
	actor := workspaceaccess.ActorRef{Kind: workspaceaccess.ActorAgentSession, SessionID: scope.SessionID,
		AgentName: scope.AgentName, WorkspaceID: scope.WorkspaceID}
	caller, err := api.Authority.Callers.InstanceCaller(ctx, scope.SessionID)
	if err != nil {
		return agentinstance.Instance{}, actor, err
	}
	if caller.InstanceID == "" || caller.ProfileID != scope.ProfileID {
		return agentinstance.Instance{}, actor, agentinstance.ErrForbidden
	}
	instance, err := api.Store.Get(ctx, caller.InstanceID)
	if err != nil {
		return instance, actor, err
	}
	return instance, actor, api.Authority.Authorize(ctx, actor, instance, "skill_reviews")
}

func (n *daemonNativeTools) agentSkillPropose(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (result toolspkg.ToolResult, callErr error) {
	defer func() { callErr = nativeSkillReviewError(req.ToolID, callErr) }()
	var input contract.AgentSkillProposeRequest
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	instance, actor, err := n.ownSkillInstance(ctx, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	response, err := n.deps.AgentInstances().ProposeSkill(ctx, actor, instance, input)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(response, "Skill proposed for operator review")
}

func (n *daemonNativeTools) agentSkillReviews(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (result toolspkg.ToolResult, callErr error) {
	defer func() { callErr = nativeSkillReviewError(req.ToolID, callErr) }()
	var input struct {
		State  string `json:"state"`
		Cursor string `json:"cursor"`
		Limit  int    `json:"limit"`
	}
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	instance, actor, err := n.ownSkillInstance(ctx, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	if input.Limit == 0 {
		input.Limit = 50
	}
	response, err := n.deps.AgentInstances().SkillReviewsPage(ctx, agentinstance.SkillReviewQuery{
		ProfileID:   instance.ProfileID,
		WorkspaceID: instance.WorkspaceID,
		InstanceID:  instance.ID,
		State:       input.State,
		Limit:       input.Limit,
	}, actor, input.Cursor)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(response, "Skill reviews")
}

func nativeSkillReviewError(id toolspkg.ToolID, err error) error {
	for _, domain := range []error{agentinstance.ErrSkillReviewLimit, agentinstance.ErrSkillProposeUnverified,
		agentinstance.ErrSkillReviewNoTarget, agentinstance.ErrSkillReviewStale, agentinstance.ErrSkillReviewNotFound} {
		if errors.Is(err, domain) {
			return toolspkg.NewToolError(toolspkg.ErrorCode(domain.Error()), id, err.Error(), err)
		}
	}
	if errors.Is(err, agentinstance.ErrForbidden) {
		return nativeHTTPStatusToolError(id, err, 403)
	}
	return err
}
