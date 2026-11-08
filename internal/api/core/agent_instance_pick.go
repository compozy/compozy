package core

import (
	"context"
	"errors"
	"net/http"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/store"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) PickAgentInstanceProposal(c *gin.Context) {
	if hasAgentCallerIdentityCredentials(agentCallerCredentialsFromRequest(c)) {
		c.JSON(
			http.StatusForbidden,
			contract.ErrorPayload{Code: "proposal_pick_operator_only", Error: "Only the operator can pick a proposal."},
		)
		return
	}
	instance, ok := h.instanceTarget(c, "reports")
	if !ok {
		return
	}
	var request contract.AgentInstancePickRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.respondError(c, 422, err)
		return
	}
	if request.Proposal == nil {
		c.JSON(
			422,
			contract.ErrorPayload{
				Code:  agentinstance.ErrProposalRange.Error(),
				Error: agentinstance.ErrProposalRange.Error(),
			},
		)
		return
	}
	storage, ok := h.AgentInstances.Store.(agentinstance.PickStore)
	if !ok {
		h.respondError(c, 503, errors.New("proposal picks unavailable"))
		return
	}
	var actions []agentinstance.ActionSnapshot
	if h.AgentInstances.PickActions != nil {
		var err error
		actions, err = h.AgentInstances.PickActions(c.Request.Context(), instance.ID)
		if err != nil {
			h.respondError(c, 500, err)
			return
		}
	}
	pick, err := storage.PickProposal(
		c.Request.Context(),
		agentinstance.PickRequest{
			Actions: actions, InstanceID: instance.ID,
			ReportID:      c.Param("report_id"),
			ProposalIndex: *request.Proposal,
			Actor:         "operator",
			QueueCap:      h.AgentInstances.QueueCap,
		},
	)
	if err != nil {
		for _, mapping := range []struct {
			cause  error
			status int
			code   string
		}{
			{agentinstance.ErrReportNotFound, 404, "report_not_found"},
			{agentinstance.ErrProposalRange, 422, "proposal_out_of_range"},
			{agentinstance.ErrReportNotProposal, 422, "report_not_proposal"},
			{agentinstance.ErrAlreadyPicked, 409, "proposal_already_picked"},
			{agentinstance.ErrInstanceNotActive, 409, "instance_not_active"},
			{agentinstance.ErrNoMainSession, 409, "instance_no_main_session"},
			{store.ErrSessionInputQueueFull, 409, "session_input_queue_full"},
		} {
			if errors.Is(err, mapping.cause) {
				c.JSON(mapping.status, contract.ErrorPayload{Code: mapping.code, Error: mapping.code})
				return
			}
		}
		h.respondError(c, 500, err)
		return
	}
	if h.AgentInstances.AfterPicks != nil {
		if err := h.AgentInstances.AfterPicks(
			context.WithoutCancel(c.Request.Context()),
			instance.ID,
			[]agentinstance.Pick{pick},
		); err != nil {
			h.respondError(c, 500, err)
			return
		}
	}
	c.JSON(200, contract.AgentInstancePickResponse{Pick: pick})
}
