package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/gin-gonic/gin"
)

// AgentInstanceDailyBudgetStore is an optional capability for operator-owned
// budget mutations and public budget status, without widening InstanceStore.
type AgentInstanceDailyBudgetStore interface {
	SetDailyBudgetLimits(context.Context, string, agentinstance.DailyBudgetLimits) error
	DailyBudgetLimits(context.Context, string) (agentinstance.DailyBudgetLimits, error)
	ReadDailyBudgetUsage(context.Context, string, time.Time) (agentinstance.DailyBudgetUsage, error)
}

type agentInstanceUpdateBody contract.AgentInstanceUpdateRequest

// UnmarshalJSON rejects misplaced budget fields before an empty budget can clear caps.
func (request *agentInstanceUpdateBody) UnmarshalJSON(raw []byte) error {
	var update contract.AgentInstanceUpdateRequest
	if err := json.Unmarshal(raw, &update); err != nil {
		return err
	}
	if update.DailyBudget != nil {
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&update); err != nil {
			return err
		}
	}
	*request = agentInstanceUpdateBody(update)
	return nil
}

func (h *BaseHandlers) updateAgentInstanceDailyBudget(
	ctx context.Context, c *gin.Context, input contract.AgentInstanceDailyBudgetInput,
) {
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
	if _, app := AppTokenFromContext(ctx); !actor.Operator || app {
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
	configured := limits.Tokens != nil || limits.USD != nil
	var updateErr error
	if h.AgentInstances.WithDailyBudgetUpdate != nil {
		updateErr = h.AgentInstances.WithDailyBudgetUpdate(ctx, instance.ID, configured, commit)
	} else {
		updateErr = commit(ctx)
	}
	if updateErr != nil {
		h.instanceError(c, updateErr)
		return
	}
	payload, err := h.AgentInstances.Payload(ctx, instance)
	if err != nil {
		slog.WarnContext(ctx, "api: committed daily budget update without refreshed status",
			"instance_id", instance.ID, "error", err)
		payload = AgentInstancePayload(instance)
		payload.Sources = []heartbeat.HeartbeatSourceStatus{}
		payload.ResolvedAppearance = map[string]any{}
		if configured {
			payload.DailyBudget = &contract.AgentInstanceDailyBudgetLimits{Tokens: limits.Tokens, USD: limits.USD}
		}
	}
	c.JSON(http.StatusOK, contract.AgentInstanceResponse{Instance: payload})
}

// decimalBudgetUSD preserves every decimal place of exact canonical receipts.
// Receipt amounts are finite base-ten decimals, so their reduced denominators
// contain only factors two and five.
func decimalBudgetUSD(value *big.Rat) string {
	if value == nil || value.Sign() == 0 {
		return "0"
	}
	denominator := new(big.Int).Set(value.Denom())
	var twoCount, fiveCount int
	for new(big.Int).Mod(denominator, big.NewInt(2)).Sign() == 0 {
		denominator.Quo(denominator, big.NewInt(2))
		twoCount++
	}
	for new(big.Int).Mod(denominator, big.NewInt(5)).Sign() == 0 {
		denominator.Quo(denominator, big.NewInt(5))
		fiveCount++
	}
	if denominator.Cmp(big.NewInt(1)) != 0 {
		return value.RatString()
	}
	digits := max(twoCount, fiveCount)
	decimal := value.FloatString(digits)
	if digits > 0 {
		decimal = strings.TrimRight(strings.TrimRight(decimal, "0"), ".")
	}
	return decimal
}
