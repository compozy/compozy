package core

import (
	"maps"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/observe"
)

// ObserveOverviewPayloadFromView converts the observe read model into the shared payload.
func ObserveOverviewPayloadFromView(view *observe.OverviewView) contract.ObserveOverviewPayload {
	if view == nil {
		return contract.ObserveOverviewPayload{SchemaVersion: contract.ObserveOverviewSchemaVersion}
	}

	return contract.ObserveOverviewPayload{
		SchemaVersion: contract.ObserveOverviewSchemaVersion,
		GeneratedAt:   view.GeneratedAt,
		Attention:     overviewAttentionPayload(view.Attention),
		Today: contract.OverviewTodayPayload{
			RunsCompleted: view.Today.RunsCompleted,
			RunsFailed:    view.Today.RunsFailed,
			TasksClosed:   view.Today.TasksClosed,
		},
		Outcomes: overviewOutcomesPayload(view.Outcomes),
		Usage:    overviewUsagePayload(view.Usage),
		Pulse:    overviewPulsePayload(view.Pulse),
		Network:  contract.OverviewNetworkPayload{MessagesToday: view.Network.MessagesToday},
		System: contract.OverviewSystemPayload{
			HookRunsToday:     view.System.HookRunsToday,
			HookFailuresToday: view.System.HookFailuresToday,
			RetentionDays:     view.System.RetentionDays,
		},
		Freshness: taskDashboardFreshnessPayload(view.Freshness),
	}
}

func overviewAttentionPayload(attention observe.OverviewAttention) contract.OverviewAttentionPayload {
	payload := contract.OverviewAttentionPayload{
		Total:  attention.Total,
		ByKind: maps.Clone(attention.ByKind),
		Items:  make([]contract.OverviewAttentionItemPayload, 0, len(attention.Items)),
	}
	if payload.ByKind == nil {
		payload.ByKind = map[string]int{}
	}
	for _, item := range attention.Items {
		payload.Items = append(payload.Items, contract.OverviewAttentionItemPayload{
			Kind:       item.Kind,
			Title:      item.Title,
			Detail:     item.Detail,
			TaskID:     item.TaskID,
			RunID:      item.RunID,
			SessionID:  item.SessionID,
			OccurredAt: item.OccurredAt,
			Actions:    append([]string(nil), item.Actions...),
		})
	}
	return payload
}

func overviewOutcomesPayload(outcomes observe.OverviewOutcomes) contract.OverviewOutcomesPayload {
	payload := contract.OverviewOutcomesPayload{
		WindowDays: outcomes.WindowDays,
		Days:       make([]contract.OverviewOutcomeDayPayload, 0, len(outcomes.Days)),
		Completed:  outcomes.Completed,
		Failed:     outcomes.Failed,
		Canceled:   outcomes.Canceled,
		SuccessPct: outcomes.SuccessPct,
	}
	for _, day := range outcomes.Days {
		payload.Days = append(payload.Days, contract.OverviewOutcomeDayPayload{
			Date:      day.Day,
			Completed: day.Completed,
			Failed:    day.Failed,
			Canceled:  day.Canceled,
		})
	}
	return payload
}

func overviewUsagePayload(usage observe.OverviewUsage) contract.OverviewUsagePayload {
	payload := contract.OverviewUsagePayload{
		WindowDays:    usage.WindowDays,
		RetentionDays: usage.RetentionDays,
		Truncated:     usage.Truncated,
		TotalTokens:   usage.TotalTokens,
		EstimatedCost: cloneFloat64Ptr(usage.EstimatedCost),
		CostCurrency:  usage.CostCurrency,
		CostStatus:    usage.CostStatus,
		Days:          make([]contract.OverviewUsageDayPayload, 0, len(usage.Days)),
		AgentShare:    make([]contract.OverviewAgentSharePayload, 0, len(usage.AgentShare)),
	}
	for _, day := range usage.Days {
		payload.Days = append(payload.Days, contract.OverviewUsageDayPayload{
			Date:   day.Day,
			Tokens: day.TotalTokens,
		})
	}
	for _, share := range usage.AgentShare {
		payload.AgentShare = append(payload.AgentShare, contract.OverviewAgentSharePayload{
			AgentName: share.AgentName,
			Tokens:    share.Tokens,
			Fraction:  share.Fraction,
		})
	}
	return payload
}

func overviewPulsePayload(pulse observe.OverviewPulse) contract.OverviewPulsePayload {
	payload := contract.OverviewPulsePayload{
		WindowDays: pulse.WindowDays,
		Buckets:    make([]contract.OverviewPulseBucketPayload, 0, len(pulse.Buckets)),
	}
	for _, bucket := range pulse.Buckets {
		payload.Buckets = append(payload.Buckets, contract.OverviewPulseBucketPayload{
			Weekday: bucket.Weekday,
			Hour:    bucket.Hour,
			Events:  bucket.Events,
		})
	}
	if pulse.Busiest != nil {
		payload.Busiest = &contract.OverviewPulseBucketPayload{
			Weekday: pulse.Busiest.Weekday,
			Hour:    pulse.Busiest.Hour,
			Events:  pulse.Busiest.Events,
		}
	}
	if pulse.LongestSession != nil {
		payload.LongestSession = &contract.OverviewLongestSessionPayload{
			SessionID:       pulse.LongestSession.SessionID,
			AgentName:       pulse.LongestSession.AgentName,
			DurationSeconds: pulse.LongestSession.DurationSeconds,
			Date:            pulse.LongestSession.Date,
		}
	}
	return payload
}
