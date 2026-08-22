package observe

import (
	"context"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/modelcatalog"
	"github.com/compozy/compozy/internal/store"
)

func (o *Observer) overviewUsage(
	ctx context.Context,
	overviewStore OverviewStore,
	query OverviewQuery,
	now time.Time,
) (OverviewUsage, error) {
	windowQuery := store.OverviewDayQuery{
		ReadScope:   query.ReadScope,
		WorkspaceID: query.WorkspaceID,
		SinceDay:    store.LocalDay(store.LocalDayStart(now, query.UsageWindowDays-1)),
	}

	days, err := overviewStore.ListTokenUsageByDay(ctx, windowQuery)
	if err != nil {
		return OverviewUsage{}, fmt.Errorf("observe: query usage by day: %w", err)
	}
	costGroups, err := overviewStore.SumTokenUsageCost(ctx, windowQuery)
	if err != nil {
		return OverviewUsage{}, fmt.Errorf("observe: query usage cost: %w", err)
	}
	agentTotals, err := overviewStore.ListTokenUsageByAgent(ctx, store.OverviewDayQuery{
		ReadScope:   query.ReadScope,
		WorkspaceID: query.WorkspaceID,
		SinceDay:    store.LocalDay(store.LocalDayStart(now, overviewShareWindowDays-1)),
	})
	if err != nil {
		return OverviewUsage{}, fmt.Errorf("observe: query usage by agent: %w", err)
	}
	profileTotals, err := overviewStore.ListTokenUsageByProfile(ctx, windowQuery)
	if err != nil {
		return OverviewUsage{}, fmt.Errorf("observe: query usage by profile: %w", err)
	}

	usage := OverviewUsage{
		WindowDays:    query.UsageWindowDays,
		RetentionDays: o.retention.RetentionDays,
		Truncated:     o.retention.RetentionDays > 0 && query.UsageWindowDays > o.retention.RetentionDays,
		Days:          days,
		AgentShare:    overviewAgentShare(agentTotals),
		Profiles:      overviewProfileUsage(profileTotals),
	}
	for _, day := range days {
		usage.TotalTokens += day.TotalTokens
	}
	usage.EstimatedCost, usage.CostCurrency, usage.CostStatus = overviewUsageCost(costGroups)
	return usage, nil
}

func overviewProfileUsage(totals []store.TokenUsageProfileTotal) []OverviewProfileUsage {
	profiles := make([]OverviewProfileUsage, 0, len(totals))
	for _, total := range totals {
		profiles = append(profiles, OverviewProfileUsage{
			ProfileID: total.ProfileID, ProfileName: total.ProfileName, Color: total.Color,
			Icon: total.Icon, Emoji: total.Emoji, Archived: total.Archived, Tokens: total.TotalTokens,
		})
	}
	return profiles
}

// overviewUsageCost keeps the token_stats provenance rule: a truthful cost only
// exists when every contributing bucket shares one provenance and currency.
func overviewUsageCost(groups []store.TokenUsageCostGroup) (*float64, string, string) {
	unknown := string(modelcatalog.CostStatusUnknown)
	populated := make([]store.TokenUsageCostGroup, 0, len(groups))
	for _, group := range groups {
		if group.RowsTotal > 0 {
			populated = append(populated, group)
		}
	}
	if len(populated) != 1 {
		return nil, "", unknown
	}

	group := populated[0]
	switch group.CostStatus {
	case string(modelcatalog.CostStatusActual), string(modelcatalog.CostStatusEstimated):
		if group.RowsWithoutCost > 0 {
			return nil, "", unknown
		}
		total := group.TotalCost
		return &total, group.CostCurrency, group.CostStatus
	case string(modelcatalog.CostStatusIncluded):
		return nil, "", group.CostStatus
	default:
		return nil, "", unknown
	}
}

func overviewAgentShare(totals []store.TokenUsageAgentTotal) []OverviewAgentShare {
	var windowTotal int64
	for _, total := range totals {
		windowTotal += total.TotalTokens
	}
	if windowTotal <= 0 {
		return nil
	}

	shares := make([]OverviewAgentShare, 0, overviewShareMaxAgents)
	if len(totals) <= overviewShareMaxAgents {
		for _, total := range totals {
			shares = append(shares, agentShareEntry(total.AgentName, total.TotalTokens, windowTotal))
		}
		return shares
	}

	var otherTokens int64
	for index, total := range totals {
		if index < overviewShareMaxAgents-1 {
			shares = append(shares, agentShareEntry(total.AgentName, total.TotalTokens, windowTotal))
			continue
		}
		otherTokens += total.TotalTokens
	}
	shares = append(shares, agentShareEntry("other", otherTokens, windowTotal))
	return shares
}

func agentShareEntry(name string, tokens int64, windowTotal int64) OverviewAgentShare {
	return OverviewAgentShare{
		AgentName: name,
		Tokens:    tokens,
		Fraction:  float64(tokens) / float64(windowTotal),
	}
}
