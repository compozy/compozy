package core

import (
	"context"
	"errors"
	"net/http"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/gin-gonic/gin"
)

// AgentInstanceSkillAutoApproveStore is the optional store capability that persists the per-instance
// skill auto-approve toggle.
type AgentInstanceSkillAutoApproveStore interface {
	SetSkillAutoApprove(context.Context, string, bool) (agentinstance.Instance, error)
}

var errSkillAutoApproveUnavailable = errors.New("instance skill auto-approve settings unavailable")

// SetSkillAutoApprove persists the toggle. Callers apply the operator-only on-guard first: the HTTP/UDS
// handler below and the native tool, which does not go through UpdateAgentInstance.
func (api *AgentInstanceAPI) SetSkillAutoApprove(
	ctx context.Context,
	id string,
	enabled bool,
) (agentinstance.Instance, error) {
	settings, ok := api.Store.(AgentInstanceSkillAutoApproveStore)
	if !ok {
		return agentinstance.Instance{}, errSkillAutoApproveUnavailable
	}
	return settings.SetSkillAutoApprove(ctx, id, enabled)
}

// updateAgentInstanceSkillAutoApprove resolves the target with the only agent verb,
// skill_auto_approve_off, then guards the on-transition with two distinct checks: an agent caller
// gets skill_auto_approve_operator_only; an operator-classified app token gets the daily_budget
// path's agent_instance_forbidden. Turning it off stays allowed for both.
func (h *BaseHandlers) updateAgentInstanceSkillAutoApprove(ctx context.Context, c *gin.Context, enabled bool) {
	instance, ok := h.instanceTarget(c, "skill_auto_approve_off")
	if !ok {
		return
	}
	if enabled {
		actor, ok := h.instanceActor(c)
		if !ok {
			return
		}
		if !actor.Operator {
			h.instanceError(c, agentinstance.ErrSkillAutoApproveOperatorOnly)
			return
		}
		if _, app := AppTokenFromContext(ctx); app {
			h.instanceError(c, agentinstance.ErrForbidden)
			return
		}
	}
	updated, err := h.AgentInstances.SetSkillAutoApprove(ctx, instance.ID, enabled)
	if errors.Is(err, errSkillAutoApproveUnavailable) {
		h.respondError(c, http.StatusServiceUnavailable, err)
		return
	}
	if err != nil {
		h.instanceError(c, err)
		return
	}
	payload, err := h.AgentInstances.Payload(ctx, updated)
	if err != nil {
		h.instanceError(c, err)
		return
	}
	c.JSON(http.StatusOK, contract.AgentInstanceResponse{Instance: payload})
}

// setFields counts the request fields present.
func setFields(present ...bool) int {
	count := 0
	for _, set := range present {
		if set {
			count++
		}
	}
	return count
}
