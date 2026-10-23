package modelcatalog

import (
	"context"
	"fmt"
	"strings"
)

func (s *CatalogService) applyBindingAvailability(ctx context.Context, models []Model, opts ListOptions) error {
	required := map[string]bool{"claude": true, "cursor": true}
	for _, source := range s.sourcesSnapshot() {
		if live, ok := source.(*LiveProviderSource); ok {
			provider := live.providerSnapshot()
			runtime := provider.RuntimeProviderName(live.providerID)
			required[live.providerID] = runtime == "claude" || runtime == "cursor"
		}
	}
	var blocked []*Model
	for i := range models {
		model := &models[i]
		if !required[model.ProviderID] || hasFreshTransportBinding(*model) ||
			model.AvailabilityState == AvailabilityStateUnavailableLive ||
			model.AvailabilityState == AvailabilityStateUnavailableStale {
			continue
		}
		model.Available = new(false)
		model.AvailabilityState = AvailabilityStateUnknown
		model.LastError = "live catalog unavailable: no fresh transport binding"
		blocked = append(blocked, model)
	}
	if len(blocked) == 0 {
		return nil
	}
	statuses, err := s.ListSourceStatus(ctx, StatusOptions{
		ProviderID: opts.ProviderID, ExecutionContext: opts.ExecutionContext,
	})
	if err != nil {
		return fmt.Errorf("model catalog: read binding availability: %w", err)
	}
	for _, model := range blocked {
		sourceID := SourceKindProviderLiveID(model.ProviderID)
		for _, status := range statuses {
			if status.ProviderID == model.ProviderID && status.SourceID == sourceID &&
				strings.TrimSpace(status.LastError) != "" {
				model.LastError = "live catalog unavailable: " + RedactString(status.LastError)
				break
			}
		}
	}
	return nil
}

func hasFreshTransportBinding(model Model) bool {
	if model.AvailabilityState != AvailabilityStateAvailableLive {
		return false
	}
	if _, ok := PreferredTransportBinding(model.TransportBindings); !ok {
		return false
	}
	for _, source := range model.Sources {
		if source.SourceID == SourceKindProviderLiveID(model.ProviderID) && !source.Stale {
			return true
		}
	}
	return false
}
