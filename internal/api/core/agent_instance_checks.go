package core

import (
	"context"
	"errors"
	"net/http"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/gin-gonic/gin"
)

type AgentInstanceChecker interface {
	Check(context.Context, string, bool) (heartbeat.CheckResult, error)
}

func (h *BaseHandlers) CheckAgentInstance(c *gin.Context) {
	var request contract.AgentInstanceCheckRequest
	if err := decodeAuthoredJSONBody(c, &request, false); err != nil {
		h.respondError(c, 422, err)
		return
	}
	verb := "check"
	if request.DryRun {
		verb = "check_dry_run"
	}
	instance, ok := h.instanceTarget(c, verb)
	if !ok {
		return
	}
	actor, ok := h.instanceActor(c)
	if !ok {
		return
	}
	if !request.DryRun && !actor.Operator {
		h.instanceError(c, agentinstance.ErrForbidden)
		return
	}
	if h.AgentInstances.Checks == nil {
		h.respondError(c, 503, errors.New("instance checks unavailable"))
		return
	}
	result, err := h.AgentInstances.Checks.Check(c.Request.Context(), instance.ID, request.DryRun)
	if err != nil {
		h.instanceError(c, err)
		return
	}
	response := contract.AgentInstanceCheckResponse{Gate: result.Gate, OccurrenceID: result.OccurrenceID}
	if result.Digest != nil {
		response.Digest = &contract.AgentInstanceCheckDigest{ItemKeys: []string{}, Fingerprint: result.Digest.Fingerprint, Diagnostics: result.Digest.Diagnostics}
		for _, item := range result.Digest.Items {
			response.Digest.ItemKeys = append(response.Digest.ItemKeys, item.Key)
		}
	}
	status := http.StatusOK
	if result.OccurrenceID != "" {
		status = http.StatusAccepted
	}
	c.JSON(status, response)
}

type AgentInstanceCheckSettings interface {
	SetChecks(context.Context, string, bool) (agentinstance.Instance, error)
}

func (h *BaseHandlers) UpdateAgentInstance(c *gin.Context) {
	var request contract.AgentInstanceUpdateRequest
	if err := decodeAuthoredJSONBody(c, &request, false); err != nil {
		h.respondError(c, 422, err)
		return
	}
	if request.ChecksEnabled == nil {
		h.respondError(c, 422, errors.New("checks_enabled is required"))
		return
	}
	verb := "checks_off"
	if *request.ChecksEnabled {
		verb = "checks_on"
	}
	instance, ok := h.instanceTarget(c, verb)
	if !ok {
		return
	}
	if h.AgentInstances.CheckSettings == nil {
		h.respondError(c, 503, errors.New("instance check settings unavailable"))
		return
	}
	updated, err := h.AgentInstances.CheckSettings.SetChecks(c.Request.Context(), instance.ID, *request.ChecksEnabled)
	if err != nil {
		h.instanceError(c, err)
		return
	}
	payload, err := h.AgentInstances.Payload(c.Request.Context(), updated)
	if err != nil {
		h.instanceError(c, err)
		return
	}
	c.JSON(http.StatusOK, contract.AgentInstanceResponse{Instance: payload})
}

type AgentInstanceOccurrenceReader interface {
	Occurrence(context.Context, string, string) (agentinstance.Occurrence, error)
}

func (h *BaseHandlers) GetAgentInstanceOccurrence(c *gin.Context) {
	instance, ok := h.instanceTarget(c, "occurrence")
	if !ok {
		return
	}
	reader, ok := h.AgentInstances.Store.(AgentInstanceOccurrenceReader)
	if !ok {
		h.respondError(c, 503, errors.New("instance occurrences unavailable"))
		return
	}
	result, err := reader.Occurrence(c.Request.Context(), instance.ID, c.Param("occurrence"))
	if err != nil {
		h.instanceError(c, err)
		return
	}
	c.JSON(http.StatusOK, contract.AgentInstanceOccurrenceResponse{State: result.State, Result: result.Result, Reason: result.Reason, ReportID: result.ReportID})
}
