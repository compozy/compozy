package session

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

func (m *Manager) recordPromptEventWithCost(
	ctx context.Context,
	state *promptTurnDispatchState,
	loop *promptPumpLoopState,
	event *acp.AgentEvent,
	runtimeEvent bool,
) error {
	if err := normalizePromptCost(ctx, state, event); err != nil {
		return err
	}
	return m.observeRecordAndNotifyPromptEvent(ctx, state.session, state, loop, *event, runtimeEvent)
}

// normalizePromptCost runs before canonical persistence and every observer. The
// baseline comes from the ledger, including archived turns, so runtime release,
// daemon restart and transcript rewind cannot charge a cumulative total twice.
func normalizePromptCost(ctx context.Context, state *promptTurnDispatchState, event *acp.AgentEvent) error {
	if event.Usage == nil || event.Usage.CostAmount == nil || event.Usage.ReportedCostAmount != nil {
		return nil
	}
	usage := *event.Usage
	event.Usage = &usage
	usage.ReportedCostAmount = usage.CostAmount
	if usage.CostCurrency == nil || strings.TrimSpace(*usage.CostCurrency) == "" ||
		math.IsNaN(*usage.CostAmount) || math.IsInf(*usage.CostAmount, 0) || *usage.CostAmount < 0 {
		return nil // The observer classifies invalid reports as unknown.
	}
	currency := strings.TrimSpace(*usage.CostCurrency)
	if state.costBaselines == nil {
		state.costBaselines = make(map[string]float64)
	}
	previous, loaded := state.costBaselines[currency]
	if !loaded {
		var err error
		previous, err = previousReportedCost(ctx, state.session.recorderHandle(), state.turnID, currency)
		if err != nil {
			return err
		}
		state.costBaselines[currency] = previous
	}
	semantics := compozyconfig.ProviderCostReportingFor(state.session.Info().Provider)
	usage.CostAmount = new(turnCost(*usage.ReportedCostAmount, previous, semantics))
	return nil
}

func turnCost(amount, previous float64, semantics compozyconfig.ProviderCostReporting) float64 {
	if semantics == compozyconfig.ProviderCostPerTurn || amount < previous {
		return amount
	}
	return amount - previous
}

func previousReportedCost(ctx context.Context, recorder EventRecorder, turnID, currency string) (float64, error) {
	// Read bounded ledger pages instead of trusting the independently rebuildable
	// token_usage projection. A failed projection write must not lose the baseline.
	query := store.EventQuery{Types: []string{acp.EventTypeUsage, acp.EventTypeDone}, Limit: 128}
	for {
		rows, err := recorder.Query(ctx, query)
		if err != nil {
			return 0, fmt.Errorf("session: read reported cost baseline: %w", err)
		}
		for _, row := range slices.Backward(rows) {
			if row.TurnID == turnID {
				continue
			}
			event, decodeErr := transcript.UnmarshalAgentEvent(row.Content)
			if decodeErr != nil {
				return 0, fmt.Errorf("session: decode reported cost baseline: %w", decodeErr)
			}
			usage := event.Usage
			if usage == nil || usage.CostCurrency == nil || strings.TrimSpace(*usage.CostCurrency) != currency {
				continue
			}
			amount := usage.ReportedCostAmount
			if amount == nil {
				amount = usage.CostAmount
			}
			if amount != nil && *amount >= 0 && !math.IsNaN(*amount) && !math.IsInf(*amount, 0) {
				return *amount, nil
			}
		}
		if len(rows) < query.Limit {
			return 0, nil
		}
		query.BeforeSequence = rows[0].Sequence
	}
}
