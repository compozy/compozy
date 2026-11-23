package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/workspaceaccess"
	"github.com/gin-gonic/gin"
)

// ListAgentSkillInstallConflicts lists the retained install conflicts under the target roots of the
// selected profile and workspace (every workspace when none is given). Operator only.
func (h *BaseHandlers) ListAgentSkillInstallConflicts(c *gin.Context) {
	scope, ok := h.skillInstallConflictScope(c, false)
	if !ok {
		return
	}
	conflicts, err := h.AgentInstances.SkillProposals.InstallConflicts(c.Request.Context(), scope.actor, scope.roots)
	if err != nil {
		h.skillReviewError(c, err)
		return
	}
	result := contract.AgentSkillInstallConflictsResponse{
		Conflicts: make([]contract.AgentSkillInstallConflictPayload, 0, len(conflicts)),
	}
	for _, conflict := range conflicts {
		payload := contract.AgentSkillInstallConflictPayload{
			OpID: conflict.OpID, Skill: conflict.Skill, ReviewID: conflict.ReviewID,
			Action: conflict.Action, State: conflict.State, Keep: conflict.Keep, RetainedCopy: conflict.RetainedCopy,
		}
		if !conflict.Since.IsZero() {
			since := conflict.Since
			payload.Since = &since
		}
		result.Conflicts = append(result.Conflicts, payload)
	}
	c.JSON(http.StatusOK, result)
}

// ResolveAgentSkillInstallConflict keeps the current or previous version of a conflicted skill.
func (h *BaseHandlers) ResolveAgentSkillInstallConflict(c *gin.Context) {
	scope, ok := h.skillInstallConflictScope(c, true)
	if !ok {
		return
	}
	var input contract.AgentSkillInstallConflictResolveRequest
	if err := decodeAuthoredJSONBody(c, &input, false); err != nil {
		h.respondError(c, 422, err)
		return
	}
	if input.Keep != agentinstance.SkillConflictKeepCurrent && input.Keep != agentinstance.SkillConflictKeepPrevious {
		h.respondError(c, 422, errors.New("keep must be current or previous"))
		return
	}
	// A post-commit finalize or publication warning is logged by the service; repeating the resolve
	// publishes the catalog again.
	result, err := h.AgentInstances.SkillProposals.ResolveInstallConflict(c.Request.Context(), scope.actor,
		scope.roots, c.Param("op_id"), input.Keep, "operator:"+scope.profileID)
	if err != nil {
		h.skillReviewError(c, err)
		return
	}
	c.JSON(http.StatusOK, contract.AgentSkillInstallConflictResolveResponse{
		OpID: result.OpID, Skill: result.Skill, Keep: result.Keep, KeptCopy: result.KeptCopy,
		RetainedCopy: result.RetainedCopy, AlreadyResolved: result.AlreadyResolved,
	})
}

type skillInstallConflictScope struct {
	actor     workspaceaccess.ActorRef
	profileID string
	roots     []string
}

// skillInstallConflictScope authorizes an operator caller and resolves the deduplicated skill target
// roots of the instances in the selected profile and workspace (every workspace when none is given).
func (h *BaseHandlers) skillInstallConflictScope(c *gin.Context, mutation bool) (skillInstallConflictScope, bool) {
	var result skillInstallConflictScope
	if !h.skillReviewsReady(c) {
		return result, false
	}
	actor, ok := h.instanceActor(c)
	if !ok {
		return result, false
	}
	if !actor.Operator {
		h.skillReviewError(c, agentinstance.ErrSkillReviewOperatorOnly)
		return result, false
	}
	api := h.AgentInstances
	if api.SkillTarget == nil {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("skill installation target unavailable"))
		return result, false
	}
	resolve := h.resolveProfileReadScope
	if mutation {
		resolve = h.resolveProfileMutationScope
	}
	profile, err := resolve(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return result, false
	}
	if profile.AllProfiles {
		h.respondProfileReadScopeError(c, singleProfileReadError("install conflicts belong to one profile"))
		return result, false
	}
	query, err := h.agentInstanceListQuery(c, profile, c.Query("workspace") == "")
	if err != nil {
		h.respondError(c, 422, err)
		return result, false
	}
	roots, err := api.skillTargetRoots(c.Request.Context(), query)
	if err != nil {
		h.skillReviewError(c, err)
		return result, false
	}
	return skillInstallConflictScope{actor: actor, profileID: profile.ProfileID, roots: roots}, true
}

// skillTargetRoots pages the instances of query, archived ones included, and returns their distinct
// skill target roots, so a conflict stays reachable after the last instance on its root is archived.
// Instances whose root is absent or out of scope are skipped; any other lookup failure fails the
// discovery instead of presenting an incomplete root set as complete.
func (api *AgentInstanceAPI) skillTargetRoots(ctx context.Context, query agentinstance.ListQuery) ([]string, error) {
	query.Limit, query.AfterID, query.IncludeArchived = 100, "", true
	seen := map[string]bool{}
	var roots []string
	for {
		page, err := api.Store.List(ctx, query)
		if err != nil {
			return nil, err
		}
		for _, instance := range page {
			root, err := api.skillRecoveryRoot(ctx, instance)
			if err != nil {
				return nil, fmt.Errorf("locate the skill root of instance %s: %w", instance.ID, err)
			}
			if root == "" || seen[root] {
				continue
			}
			seen[root] = true
			roots = append(roots, root)
		}
		if len(page) < query.Limit {
			return roots, nil
		}
		query.AfterID = page[len(page)-1].ID
	}
}

func (api *AgentInstanceAPI) skillRecoveryRoot(ctx context.Context, instance agentinstance.Instance) (string, error) {
	if api.SkillRecoveryRoot != nil {
		return api.SkillRecoveryRoot(ctx, instance)
	}
	root, _, err := api.SkillTarget(ctx, instance)
	if errors.Is(err, agentinstance.ErrSkillReviewNoTarget) {
		return "", nil
	}
	return root, err
}
