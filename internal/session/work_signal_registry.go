package session

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/diagnostics"
)

// WorkSignalSource binds a known kind to its authoritative inspection adapter.
type WorkSignalSource struct {
	Kind   WorkSignalKind
	Source SignalSource
}

// SignalSourceFunc adapts a subsystem's existing read boundary.
type SignalSourceFunc func(context.Context, string) ([]WorkSignal, error)

const signalSourceUnknown = "unknown"

func (f SignalSourceFunc) Signals(ctx context.Context, id string) ([]WorkSignal, error) {
	return f(ctx, id)
}

// WorkSignalRegistry collects fresh evidence without owning the evidence's lifecycle.
type WorkSignalRegistry struct {
	mu      sync.RWMutex
	sources map[WorkSignalKind]SignalSource
}

func NewWorkSignalRegistry(sources ...WorkSignalSource) *WorkSignalRegistry {
	registry := &WorkSignalRegistry{sources: make(map[WorkSignalKind]SignalSource)}
	registry.Register(sources...)
	return registry
}

func (r *WorkSignalRegistry) Register(sources ...WorkSignalSource) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, source := range sources {
		r.sources[source.Kind] = source.Source
	}
}

// Inspect reports unknown sources explicitly; no failed inspection renews a signal.
func (r *WorkSignalRegistry) Inspect(ctx context.Context, id string, now time.Time) *SupervisionState {
	r.mu.RLock()
	sources := maps.Clone(r.sources)
	r.mu.RUnlock()
	state := CloneSupervisionState(nil)
	for _, name := range WorkSignalKindValues() {
		kind := WorkSignalKind(name)
		source := sources[kind]
		if source == nil {
			state.Sources = append(state.Sources, SignalSourceState{
				Kind: kind, State: signalSourceUnknown, Error: "source is not configured",
			})
			continue
		}
		inspectionCtx, cancel := context.WithTimeout(ctx, defaultLifecycleTimeout)
		signals, err := source.Signals(inspectionCtx, id)
		cancel()
		if err != nil {
			state.Sources = append(state.Sources, SignalSourceState{
				Kind: kind, State: signalSourceUnknown, Error: diagnostics.RedactAndBound(err.Error(), 512),
			})
			continue
		}
		appendInspectedSignals(state, kind, signals, now)
	}
	slices.SortFunc(state.WorkSignals, func(a, b WorkSignal) int {
		if n := cmp.Compare(a.Kind, b.Kind); n != 0 {
			return n
		}
		return cmp.Compare(a.Ref, b.Ref)
	})
	slices.SortFunc(state.Sources, func(a, b SignalSourceState) int {
		if n := cmp.Compare(a.Kind, b.Kind); n != 0 {
			return n
		}
		if n := cmp.Compare(a.Ref, b.Ref); n != 0 {
			return n
		}
		return cmp.Compare(a.State, b.State)
	})
	return state
}

func appendInspectedSignals(state *SupervisionState, kind WorkSignalKind, signals []WorkSignal, now time.Time) {
	status := SignalSourceState{Kind: kind, State: "absent"}
	for _, signal := range signals {
		if signal.AttentionReason != "" {
			state.Sources = append(state.Sources, SignalSourceState{
				Kind: kind, State: "attention", Ref: signal.Ref,
				Error: diagnostics.RedactAndBound(signal.AttentionReason, 512),
			})
		}
		if signal.Kind != kind || signal.Since.IsZero() || signal.ValidUntil.IsZero() {
			state.Sources = append(state.Sources, SignalSourceState{
				Kind:  kind,
				State: signalSourceUnknown,
				Ref:   signal.Ref,
				Error: "source returned incomplete freshness evidence",
			})
			continue
		}
		if signal.ValidUntil.Before(now) ||
			(signal.ValidUntil.Equal(now) && (kind == WorkSignalTaskLease || kind == WorkSignalScheduledWait)) {
			if signal.StaleAttention {
				state.Sources = append(state.Sources, SignalSourceState{
					Kind: kind, State: "stale", Ref: signal.Ref, Error: fmt.Sprintf("%s evidence is stale", kind),
				})
			}
			continue
		}
		signal.Ref = strings.TrimSpace(signal.Ref)
		state.WorkSignals = append(state.WorkSignals, signal)
		status.State = "present"
	}
	state.Sources = append(state.Sources, status)
}

func supervisionNeedsAttention(state *SupervisionState) bool {
	return state != nil && slices.ContainsFunc(state.Sources, func(source SignalSourceState) bool {
		return source.State == signalSourceUnknown || source.State == "stale" || source.State == "attention"
	})
}
