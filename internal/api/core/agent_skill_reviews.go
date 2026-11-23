package core

import (
	"context"
	"errors"
	"log/slog"
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
		Kind:        c.Query("kind"),
		DecidedVia:  c.Query("decided_via"),
		Limit:       list.Limit,
	}, actor, c.Query("cursor"))
	if err != nil {
		h.skillReviewError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *BaseHandlers) GetAgentSkillReview(c *gin.Context) {
	review, instance, ok := h.skillReviewTarget(c)
	if !ok {
		return
	}
	payload, err := skillReviewPayload(review, true)
	if err != nil {
		h.skillReviewError(c, err)
		return
	}
	payload.Revert = h.AgentInstances.skillRevertEligibility(c.Request.Context(), review, instance)
	c.JSON(http.StatusOK, contract.AgentSkillReviewResponse{Review: payload})
}

// skillRevertEligibility is the detail's revert field: the same rule Revert evaluates under the lock.
func (api *AgentInstanceAPI) skillRevertEligibility(
	ctx context.Context,
	review agentinstance.SkillReview,
	instance agentinstance.Instance,
) *contract.AgentSkillRevertEligibility {
	target := ""
	if api.SkillTarget != nil {
		if resolved, _, err := api.SkillTarget(ctx, instance); err == nil {
			target = resolved
		}
	}
	eligibility, err := api.SkillProposals.RevertEligibility(ctx, review, target)
	if err != nil {
		slog.WarnContext(ctx, "skill.review.revert_eligibility_failed", "review_id", review.ID, "error", err)
		eligibility = agentinstance.RevertEligibility{Reason: agentinstance.SkillRevertTargetUnavailable}
	}
	return &contract.AgentSkillRevertEligibility{Allowed: eligibility.Allowed, Reason: eligibility.Reason}
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

// Skill review decisions on POST /agent-skill-reviews/:id/<decision>.
const (
	skillReviewApprove = "approve"
	skillReviewReject  = "reject"
	skillReviewRevert  = "revert"
)

func (h *BaseHandlers) ApproveAgentSkillReview(c *gin.Context) {
	h.decideAgentSkillReview(c, skillReviewApprove)
}
func (h *BaseHandlers) RejectAgentSkillReview(c *gin.Context) {
	h.decideAgentSkillReview(c, skillReviewReject)
}
func (h *BaseHandlers) RevertAgentSkillReview(c *gin.Context) {
	h.decideAgentSkillReview(c, skillReviewRevert)
}

func (h *BaseHandlers) decideAgentSkillReview(c *gin.Context, decision string) {
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
	if api.SkillTarget == nil {
		h.respondError(c, 503, errors.New("skill installation target unavailable"))
		return
	}
	ctx := c.Request.Context()
	target, _, targetErr := api.SkillTarget(ctx, instance)
	switch decision {
	case skillReviewApprove:
		if targetErr != nil {
			h.skillReviewError(c, targetErr)
			return
		}
		// A post-commit projection or publication warning is logged by the service and retried by
		// Reconcile; the skill is installed either way.
		_, err = api.SkillProposals.Approve(ctx, actor, instance, review.ID, input.Digest, operator, target)
	case skillReviewRevert:
		// Without a resolvable target the revert is refused as target_unavailable by the service.
		if targetErr != nil {
			target = ""
		}
		_, err = api.SkillProposals.Revert(ctx, actor, instance, review.ID, input.Digest, operator, target)
	default:
		// An instance without an installation target never installs anything, so its reviews reject
		// without the skill lock. Any other target failure refuses the rejection: without the install
		// log, a committed approval whose projection is still pending cannot be told apart.
		if targetErr != nil && !errors.Is(targetErr, agentinstance.ErrSkillReviewNoTarget) {
			h.skillReviewError(c, targetErr)
			return
		}
		if targetErr != nil {
			target = ""
		}
		err = api.SkillProposals.Reject(ctx, actor, instance, review.ID, input.Digest, operator, target)
	}
	if err != nil {
		h.skillReviewError(c, err)
		return
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
		Name:        input.Name,
		Description: input.Description,
		Body:        input.Body,
		Mode:        agentinstance.SkillProposalMode(input.Mode),
		RemovePaths: input.RemovePaths,
		Lesson: agentinstance.SkillLesson{
			Cue:        input.Lesson.Cue,
			Lesson:     input.Lesson.Lesson,
			Evidence:   input.Lesson.Evidence,
			Confidence: input.Lesson.Confidence,
		},
		EvidenceSessionIDs: input.EvidenceSessionIDs,
	}
	for _, file := range input.Files {
		proposal.Files = append(proposal.Files, agentinstance.SkillDraftFile{Path: file.Path, Content: file.Content})
	}
	service := *api.SkillProposals
	service.ScanRoots = roots
	result, err := service.Propose(ctx, actor, instance, target, proposal)
	if err != nil {
		return contract.AgentSkillReviewResponse{}, err
	}
	payload, err := skillReviewPayload(result.Review, false)
	return contract.AgentSkillReviewResponse{Review: payload, DuplicateOfPending: result.Duplicate}, err
}

// skillReviewErrorStatuses is the one code-to-status table for skill review domain errors, shared by
// the HTTP/UDS handlers and the native skill tools.
var skillReviewErrorStatuses = []struct {
	reason error
	status int
}{
	{agentinstance.ErrSkillReviewOperatorOnly, 403}, {agentinstance.ErrForbidden, 403},
	{agentinstance.ErrSkillReviewNotFound, 404}, {agentinstance.ErrSkillReviewStale, 409},
	{agentinstance.ErrSkillReviewNoTarget, 409}, {agentinstance.ErrSkillReviewLimit, 409},
	{agentinstance.ErrSkillProposeUnverified, 422}, {agentinstance.ErrSkillReviewInvalid, 422},
	{agentinstance.ErrSkillProposeLessonRequired, 422}, {agentinstance.ErrSkillProposeLessonTooLong, 422},
	{agentinstance.ErrSkillProposeEvidenceRequired, 422}, {agentinstance.ErrSkillProposeEvidenceNotFound, 422},
	{agentinstance.ErrSkillProposeLessonInvalid, 422},
	{agentinstance.ErrSkillProposePreviouslyRejected, 409}, {agentinstance.ErrSkillProposeNameTaken, 409},
	{ErrSkillReviewInvalidCursor, 422},
	{agentinstance.ErrSkillReviewTargetExists, 409}, {agentinstance.ErrSkillInstallBusy, 409},
	{agentinstance.ErrSkillInstallConflict, 409}, {agentinstance.ErrSkillInstallConflictNotFound, 404},
	{agentinstance.ErrSkillInstallConflictResolving, 409},
	{agentinstance.ErrSkillReviewBaseStale, 409}, {agentinstance.ErrSkillReviewBaseMissing, 409},
	{agentinstance.ErrSkillReviewBaseUnsupported, 422}, {agentinstance.ErrSkillProposeNoChange, 409},
	{agentinstance.ErrSkillProposeInvalidRemove, 422}, {agentinstance.ErrSkillReviewRevertConflict, 409},
}

// SkillReviewErrorStatus returns the stable code and HTTP status of a skill review domain error.
func SkillReviewErrorStatus(err error) (string, int, bool) {
	for _, item := range skillReviewErrorStatuses {
		if errors.Is(err, item.reason) {
			return item.reason.Error(), item.status, true
		}
	}
	return "", 0, false
}

func (h *BaseHandlers) skillReviewError(c *gin.Context, err error) {
	if code, status, ok := SkillReviewErrorStatus(err); ok {
		if reason := agentinstance.SkillRevertReason(err); reason != "" {
			c.JSON(status, contract.AgentSkillRevertConflictPayload{Code: code, Reason: reason, Error: err.Error()})
			return
		}
		c.JSON(status, contract.ErrorPayload{Code: code, Error: err.Error()})
		return
	}
	if errors.Is(err, errSkillReviewQuery) {
		h.respondError(c, 422, err)
		return
	}
	h.instanceError(c, err)
}
