package modelcatalog

import "strings"

// StartBlockedReason names why a merged model cannot start a session right now.
// The daemon owns the cause; presentation surfaces own the wording.
type StartBlockedReason string

const (
	// StartBlockedUnavailable means an availability authority reported the model unavailable.
	StartBlockedUnavailable StartBlockedReason = "unavailable"
	// StartBlockedLiveDiscoveryUnavailable means the provider's live source never answered.
	StartBlockedLiveDiscoveryUnavailable StartBlockedReason = "live_discovery_unavailable"
	// StartBlockedLiveDiscoveryStale means the provider's live source is serving stale rows.
	StartBlockedLiveDiscoveryStale StartBlockedReason = "live_discovery_stale"
	// StartBlockedNotAdvertised means a fresh live source did not advertise the model.
	StartBlockedNotAdvertised StartBlockedReason = "not_advertised"
)

// ProviderLiveSourceRef returns the provider's own live source reference, when it participated.
func ProviderLiveSourceRef(model Model) (SourceRef, bool) {
	wanted := SourceKindProviderLiveID(model.ProviderID)
	for _, source := range model.Sources {
		if source.SourceID == wanted {
			return source, true
		}
	}
	return SourceRef{}, false
}

// ModelStartability reports whether a session may launch against the merged model, and
// why not when it may not. It is derived on every read so no caller can observe a verdict
// that has gone stale against the model's own sources.
func ModelStartability(model Model) (bool, StartBlockedReason) {
	if model.Available != nil && !*model.Available {
		return false, StartBlockedUnavailable
	}
	// Some providers' offline (builtin/config/models_dev) rows are placeholders that must
	// never outrank what the live agent, once reachable, actually offers.
	if !ProviderRequiresLiveConfirmation(model.ProviderID) {
		return true, ""
	}
	source, found := ProviderLiveSourceRef(model)
	switch {
	case !found:
		return false, StartBlockedLiveDiscoveryUnavailable
	case source.Stale:
		return false, StartBlockedLiveDiscoveryStale
	}
	// Beyond live confirmation, a provider addressed by a CompozyOS logical id also needs a
	// transport binding the live agent advertises, since the logical id itself never reaches
	// the agent. Providers without that split (their model id IS the transport id) need
	// nothing more than the fresh source above.
	if ProviderRequiresLiveBinding(model.ProviderID) && !modelHasTransportBinding(model) {
		return false, StartBlockedNotAdvertised
	}
	return true, ""
}

func modelHasTransportBinding(model Model) bool {
	for _, binding := range model.TransportBindings {
		if strings.TrimSpace(binding.TransportModelID) != "" {
			return true
		}
	}
	return false
}
