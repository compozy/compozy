package core

import (
	"context"
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/diagnostics"
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
	response := AgentInstanceCheckResponse(result)
	status := http.StatusOK
	if result.OccurrenceID != "" {
		status = http.StatusAccepted
	}
	c.JSON(status, response)
}

type AgentInstanceCheckSettings interface {
	SetChecks(context.Context, string, bool) (agentinstance.Instance, error)
}

func decodeAgentInstanceUpdate(c *gin.Context, request *contract.AgentInstanceUpdateRequest) error {
	var raw json.RawMessage
	if err := decodeAuthoredJSONBody(c, &raw, false); err != nil {
		return err
	}
	if err := json.Unmarshal(raw, request); err != nil {
		return err
	}
	if request.DailyBudget != nil {
		return jsonv2.Unmarshal(raw, request, jsonv2.RejectUnknownMembers(true))
	}
	return nil
}

func (h *BaseHandlers) UpdateAgentInstance(c *gin.Context) {
	var request contract.AgentInstanceUpdateRequest
	if err := decodeAgentInstanceUpdate(c, &request); err != nil {
		h.respondError(c, 422, err)
		return
	}
	if (request.ChecksEnabled == nil) == (request.DailyBudget == nil) {
		h.respondError(c, 422, errors.New("exactly one of checks_enabled or daily_budget is required"))
		return
	}
	if request.DailyBudget != nil {
		h.updateAgentInstanceDailyBudget(c, request.DailyBudget)
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

func (h *BaseHandlers) updateAgentInstanceDailyBudget(c *gin.Context, input *contract.AgentInstanceDailyBudgetInput) {
	limits := input.Limits()
	if err := limits.Validate(); err != nil {
		h.respondError(c, 422, err)
		return
	}
	instance, ok := h.instanceTarget(c, "budget_update")
	if !ok {
		return
	}
	actor, ok := h.instanceActor(c)
	if !ok {
		return
	}
	if _, app := AppTokenFromContext(c.Request.Context()); !actor.Operator || app {
		h.instanceError(c, agentinstance.ErrForbidden)
		return
	}
	budget, ok := h.AgentInstances.Store.(AgentInstanceDailyBudgetStore)
	if !ok {
		h.respondError(c, 503, errors.New("instance daily budget settings unavailable"))
		return
	}
	commit := func(ctx context.Context) error {
		return budget.SetDailyBudgetLimits(ctx, instance.ID, limits)
	}
	var updateErr error
	if h.AgentInstances.WithDailyBudgetUpdate != nil {
		configured := limits.Tokens != nil || limits.USD != nil
		updateErr = h.AgentInstances.WithDailyBudgetUpdate(c.Request.Context(), instance.ID, configured, commit)
	} else {
		updateErr = commit(c.Request.Context())
	}
	if updateErr != nil {
		h.instanceError(c, updateErr)
		return
	}
	payload, err := h.AgentInstances.Payload(c.Request.Context(), instance)
	if err != nil {
		slog.WarnContext(c.Request.Context(), "api: committed daily budget update without refreshed status",
			"instance_id", instance.ID, "error", err)
		payload = AgentInstancePayload(instance)
		payload.Sources = []heartbeat.HeartbeatSourceStatus{}
		payload.ResolvedAppearance = map[string]any{}
		if limits.Tokens != nil || limits.USD != nil {
			payload.DailyBudget = &contract.AgentInstanceDailyBudgetLimits{Tokens: limits.Tokens, USD: limits.USD}
		}
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
	c.JSON(
		http.StatusOK,
		contract.AgentInstanceOccurrenceResponse{
			State:    result.State,
			Result:   result.Result,
			Reason:   result.Reason,
			ReportID: result.ReportID,
		},
	)
}

// AgentInstanceCheckResponse projects the public check result for every transport.
func AgentInstanceCheckResponse(result heartbeat.CheckResult) contract.AgentInstanceCheckResponse {
	response := contract.AgentInstanceCheckResponse{
		Sources:      result.Sources,
		Gate:         result.Gate,
		OccurrenceID: result.OccurrenceID,
		Digest:       []contract.AgentInstanceCheckDigest{},
	}
	if result.Digest != nil {
		response.Digest = make([]contract.AgentInstanceCheckDigest, 0, len(result.Digest.Items))
		for _, item := range result.Digest.Items {
			response.Digest = append(
				response.Digest,
				contract.AgentInstanceCheckDigest{
					Key:     item.Key,
					Summary: agentDigestItemSummary(item),
					Source:  agentDigestItemSource(item),
				},
			)
		}
	}
	return response
}

func agentCheckDigestSummary(key string) string {
	family, rest, _ := strings.Cut(key, ":")
	label := map[string]string{
		"task": "Blocked task", "run": "Queued run", "health": "Session health",
		queryFilterIdleValue: "Idle session", "auto": "Failed automation", "monitor": "Fired monitor",
	}[family]
	return diagnostics.RedactAndBound(label+" "+strings.ReplaceAll(rest, ":", " · "), 200)
}

func agentDigestItemSummary(item heartbeat.DigestItem) string {
	if strings.HasPrefix(item.Key, "source:") {
		return item.Summary
	}
	return agentCheckDigestSummary(item.Key)
}
func agentDigestItemSource(item heartbeat.DigestItem) *contract.AgentInstanceProposalSource {
	if item.Source == nil {
		return nil
	}
	s := item.Source
	return &contract.AgentInstanceProposalSource{
		Kind:       s.Kind,
		SourceID:   s.SourceID,
		SourceRef:  s.SourceRef,
		ItemKey:    s.ItemKey,
		URL:        s.URL,
		EventID:    s.EventID,
		TriggerID:  s.TriggerID,
		RunID:      s.RunID,
		ReceivedAt: s.ReceivedAt,
	}
}
