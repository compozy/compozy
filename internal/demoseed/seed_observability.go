package demoseed

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
)

const usageCostCurrency = "USD"

// observabilityCounts reports what one observability seeding pass persisted.
type observabilityCounts struct {
	tokenUsageDays int
	eventSummaries int
}

func seedObservability(
	ctx context.Context,
	db *globaldb.GlobalDB,
	state *scenario,
) (observabilityCounts, error) {
	days, err := seedTokenUsage(ctx, db, state)
	if err != nil {
		return observabilityCounts{}, err
	}
	summaries, err := seedEventSummaries(ctx, db, state)
	if err != nil {
		return observabilityCounts{}, err
	}
	return observabilityCounts{tokenUsageDays: days, eventSummaries: summaries}, nil
}

func seedTokenUsage(ctx context.Context, db *globaldb.GlobalDB, state *scenario) (int, error) {
	currency := usageCostCurrency
	stories := scenarioTokenUsage()
	for _, story := range stories {
		record, err := state.recordFor(story.WorkspaceKey)
		if err != nil {
			return 0, err
		}
		input, output, cost := story.Input, story.Output, story.CostUSD
		total := input + output
		if err := db.UpsertTokenUsageDaily(ctx, store.TokenUsageDailyUpdate{
			Day: state.clock.dayKey(story.DaysBack), WorkspaceID: record.ID, AgentName: story.AgentName,
			InputTokens: &input, OutputTokens: &output, TotalTokens: &total,
			CostAmount: &cost, CostCurrency: &currency,
			CostStatus: store.TokenCostStatusEstimated, CostSource: store.TokenCostSourceCatalogConfig,
		}); err != nil {
			return 0, fmt.Errorf("demo seed: record token usage for %q: %w", story.AgentName, err)
		}
	}
	return len(stories), nil
}

func seedEventSummaries(ctx context.Context, db *globaldb.GlobalDB, state *scenario) (int, error) {
	stories := scenarioEventSummaries(state.clock)
	summaries := make([]store.EventSummary, 0, len(stories))
	for _, story := range stories {
		summary := store.EventSummary{
			ID: story.ID, SessionID: story.SessionID, Type: story.Type,
			AgentName: story.AgentName, Outcome: story.Outcome,
			Summary: story.Summary, Timestamp: story.At,
		}
		summary.HookEvent = story.HookEvent
		summary.HookName = story.HookName
		summaries = append(summaries, summary)
	}
	if err := db.WriteEventSummaries(ctx, summaries); err != nil {
		return 0, fmt.Errorf("demo seed: write event summaries: %w", err)
	}
	return len(summaries), nil
}
