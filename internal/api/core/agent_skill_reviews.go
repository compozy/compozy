package core

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/workspaceaccess"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) skillReviewsReady(c *gin.Context) bool {
	if h.AgentInstances == nil || h.AgentInstances.SkillProposals == nil {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("skill reviews unavailable"))
		return false
	}
	return true
}

func (h *BaseHandlers) ListAgentSkillReviews(c *gin.Context) {
	if !h.skillReviewsReady(c) {
		return
	}
	actor, ok := h.instanceActor(c)
	if !ok {
		return
	}
	profile, err := h.resolveProfileReadScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	if profile.AllProfiles {
		h.respondProfileReadScopeError(c, singleProfileReadError("skill reviews belong to one profile"))
		return
	}
	list, err := h.agentInstanceListQuery(c, profile, false)
	if err != nil {
		h.respondError(c, 422, err)
		return
	}
	instanceID := c.Query("instance_id")
	if !actor.Operator && instanceID == "" {
		caller, err := h.AgentInstances.Authority.Callers.InstanceCaller(c.Request.Context(), actor.SessionID)
		if err != nil {
			h.skillReviewError(c, err)
			return
		}
		instanceID = caller.InstanceID
	}
	if instanceID != "" {
		instance, ok := h.instanceByID(c, instanceID, "skill_reviews")
		if !ok {
			return
		}
		if c.Query("workspace") != "" && list.WorkspaceID != instance.WorkspaceID {
			h.skillReviewError(c, agentinstance.ErrForbidden)
			return
		}
		list.WorkspaceID = instance.WorkspaceID
	} else if !actor.Operator {
		h.skillReviewError(c, agentinstance.ErrForbidden)
		return
	}
	result, err := h.AgentInstances.SkillReviewsPage(c.Request.Context(), agentinstance.SkillReviewQuery{
		ProfileID:   profile.ProfileID,
		WorkspaceID: list.WorkspaceID,
		InstanceID:  instanceID,
		State:       list.State,
		Limit:       list.Limit,
	}, actor, c.Query("cursor"))
	if err != nil {
		h.skillReviewError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *BaseHandlers) GetAgentSkillReview(c *gin.Context) {
	review, _, ok := h.skillReviewTarget(c)
	if !ok {
		return
	}
	payload, err := skillReviewPayload(review, true)
	if err != nil {
		h.skillReviewError(c, err)
		return
	}
	c.JSON(http.StatusOK, contract.AgentSkillReviewResponse{Review: payload})
}

func (h *BaseHandlers) skillReviewTarget(c *gin.Context) (agentinstance.SkillReview, agentinstance.Instance, bool) {
	if !h.skillReviewsReady(c) {
		return agentinstance.SkillReview{}, agentinstance.Instance{}, false
	}
	review, err := h.AgentInstances.SkillProposals.Store.GetReview(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.skillReviewError(c, err)
		return review, agentinstance.Instance{}, false
	}
	instance, ok := h.instanceByID(c, review.InstanceID, "skill_reviews")
	return review, instance, ok
}

func (h *BaseHandlers) ApproveAgentSkillReview(c *gin.Context) { h.decideAgentSkillReview(c, true) }
func (h *BaseHandlers) RejectAgentSkillReview(c *gin.Context)  { h.decideAgentSkillReview(c, false) }

func (h *BaseHandlers) decideAgentSkillReview(c *gin.Context, approve bool) {
	actor, ok := h.instanceActor(c)
	if !ok {
		return
	}
	if !actor.Operator {
		h.skillReviewError(c, agentinstance.ErrSkillReviewOperatorOnly)
		return
	}
	scope, err := h.resolveProfileMutationScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	review, instance, ok := h.skillReviewTarget(c)
	if !ok {
		return
	}
	if instance.ProfileID != scope.ProfileID {
		h.skillReviewError(c, agentinstance.ErrSkillReviewNotFound)
		return
	}
	var input contract.AgentSkillReviewDecisionRequest
	if err := decodeAuthoredJSONBody(c, &input, false); err != nil {
		h.respondError(c, 422, err)
		return
	}
	if strings.TrimSpace(input.Digest) == "" {
		h.respondError(c, 422, errors.New("digest is required"))
		return
	}
	api := h.AgentInstances
	operator := "operator:" + instance.ProfileID
	if approve {
		if api.SkillTarget == nil {
			h.respondError(c, 503, errors.New("skill installation target unavailable"))
			return
		}
		var target string
		target, _, err = api.SkillTarget(c.Request.Context(), instance)
		if err == nil {
			err = api.SkillProposals.Approve(
				c.Request.Context(),
				actor,
				instance,
				review.ID,
				input.Digest,
				operator,
				target,
			)
		}
	} else {
		err = api.SkillProposals.Reject(c.Request.Context(), actor, instance, review.ID, input.Digest, operator)
	}
	if err != nil {
		h.skillReviewError(c, err)
		return
	}
	if approve && api.SkillPublished != nil {
		if err := api.SkillPublished(c.Request.Context()); err != nil {
			h.skillReviewError(c, err)
			return
		}
	}
	h.GetAgentSkillReview(c)
}

func (api *AgentInstanceAPI) ProposeSkill(
	ctx context.Context,
	actor workspaceaccess.ActorRef,
	instance agentinstance.Instance,
	input contract.AgentSkillProposeRequest,
) (contract.AgentSkillReviewResponse, error) {
	if api.SkillProposals == nil || api.SkillTarget == nil {
		return contract.AgentSkillReviewResponse{}, errors.New("skill proposals unavailable")
	}
	target, roots, err := api.SkillTarget(ctx, instance)
	if err != nil {
		return contract.AgentSkillReviewResponse{}, err
	}
	proposal := agentinstance.SkillProposalInput{
		Name:               input.Name,
		Description:        input.Description,
		Body:               input.Body,
		EvidenceSessionIDs: input.EvidenceSessionIDs,
	}
	for _, file := range input.Files {
		proposal.Files = append(proposal.Files, agentinstance.SkillDraftFile{Path: file.Path, Content: file.Content})
	}
	service := *api.SkillProposals
	service.ScanRoots = roots
	review, err := service.Propose(ctx, actor, instance, target, proposal)
	if err != nil {
		return contract.AgentSkillReviewResponse{}, err
	}
	payload, err := skillReviewPayload(review, false)
	return contract.AgentSkillReviewResponse{Review: payload}, err
}

func (h *BaseHandlers) skillReviewError(c *gin.Context, err error) {
	for _, item := range []struct {
		reason error
		status int
	}{
		{agentinstance.ErrSkillReviewOperatorOnly, 403}, {agentinstance.ErrForbidden, 403},
		{agentinstance.ErrSkillReviewNotFound, 404}, {agentinstance.ErrSkillReviewStale, 409},
		{agentinstance.ErrSkillReviewNoTarget, 409}, {agentinstance.ErrSkillReviewLimit, 409},
		{agentinstance.ErrSkillProposeUnverified, 422}, {agentinstance.ErrSkillReviewInvalid, 422},
	} {
		if errors.Is(err, item.reason) {
			c.JSON(item.status, contract.ErrorPayload{Code: item.reason.Error(), Error: err.Error()})
			return
		}
	}
	if errors.Is(err, errSkillReviewQuery) {
		h.respondError(c, 422, err)
		return
	}
	h.instanceError(c, err)
}
