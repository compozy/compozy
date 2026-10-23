package modelcatalog

import (
	"context"
	"errors"
	"strings"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

func (s *CatalogService) applyBindingAvailability(ctx context.Context, models []Model, opts ListOptions) error {
	runtimes := map[string]string{
		liveSourcesClaudeKey: liveSourcesClaudeKey,
		liveSourcesCursorKey: liveSourcesCursorKey,
	}
	for _, source := range s.sourcesSnapshot() {
		if live, ok := source.(*LiveProviderSource); ok {
			provider := live.providerSnapshot()
			runtime := provider.RuntimeProviderName(live.providerID)
			runtimes[live.providerID] = runtime
		}
	}
	var blocked []*Model
	for i := range models {
		model := &models[i]
		if !RequiresLiveTransportBinding(runtimes[model.ProviderID], model.ModelID) ||
			HasFreshTransportBinding(*model, opts.Now) ||
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
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		for _, model := range blocked {
			model.LastError += "; source status unavailable: " + RedactString(err.Error())
		}
		return nil
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

// RequiresLiveTransportBinding keeps catalog selection aligned with session admission.
func RequiresLiveTransportBinding(runtimeProvider, modelID string) bool {
	if runtimeProvider == liveSourcesCursorKey {
		return true
	}
	if runtimeProvider != liveSourcesClaudeKey {
		return false
	}
	for _, model := range compozyconfig.BuiltinProviders()[liveSourcesClaudeKey].Models.Curated {
		if strings.TrimSpace(model.ID) == strings.TrimSpace(modelID) {
			return true
		}
	}
	return false
}

// HasFreshTransportBinding applies the admission cache's ten-minute reuse window.
func HasFreshTransportBinding(model Model, now time.Time) bool {
	if model.AvailabilityState != AvailabilityStateAvailableLive {
		return false
	}
	if _, ok := PreferredTransportBinding(model.TransportBindings); !ok {
		return false
	}
	for _, source := range model.Sources {
		age := now.Sub(source.RefreshedAt)
		if source.SourceID == SourceKindProviderLiveID(model.ProviderID) && !source.Stale &&
			!source.RefreshedAt.IsZero() && age >= 0 && age < 10*time.Minute {
			return true
		}
	}
	return false
}
