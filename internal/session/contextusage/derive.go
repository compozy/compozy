package contextusage

import (
	"cmp"
	"slices"
)

func Derive(in Input) ContextUsage {
	if !in.Available {
		return ContextUsage{State: StateUnavailable}
	}
	result := ContextUsage{State: StateUnknown}
	observations := contextObservations(in.UsageEvents)
	result.Injected = deriveInjected(in.Deliveries, observations)
	if len(observations) == 0 {
		return result
	}
	latest := observations[len(observations)-1]
	result.State = StateReported
	result.Used = latest.Usage.ContextUsed
	result.Sequence = new(latest.Sequence)
	result.ReportedTurnID = latest.TurnID
	result.ReportedAt = new(latest.At)
	result.Stale = new(in.Settled != nil && latest.TurnID != in.Settled.TurnID && latest.Sequence < in.Settled.Sequence)
	if latest.Usage.ContextSize != nil && *latest.Usage.ContextSize > 0 {
		result.Size = latest.Usage.ContextSize
		result.SizeSource = "agent"
		if in.Threshold != nil && *in.Threshold > 0 {
			result.PressureThreshold = in.Threshold
		}
	} else if in.CatalogWindow != nil && *in.CatalogWindow > 0 {
		result.State = StateEstimatedSize
		result.Size = in.CatalogWindow
		result.SizeSource = "catalog"
	}
	if result.Size != nil {
		result.Ratio = new(float64(*result.Used) / float64(*result.Size))
	}
	return result
}

func contextObservations(events []UsageEvent) []UsageEvent {
	observations := make([]UsageEvent, 0, len(events))
	for _, event := range events {
		if event.Usage.ContextUsed != nil && *event.Usage.ContextUsed >= 0 {
			observations = append(observations, event)
		}
	}
	slices.SortStableFunc(observations, func(a, b UsageEvent) int { return cmp.Compare(a.Sequence, b.Sequence) })
	return observations
}
