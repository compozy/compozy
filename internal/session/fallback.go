package session

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
)

// ChainOwner declares who runs the fallback chain for one launch, so no launch ever has
// two chain owners (ADR-004).
type ChainOwner uint8

const (
	// ChainOwnerSession is the default: the session layer owns the agent's fallback_chain
	// for ordinary work sessions (HTTP/CLI/tool Create, first binds, agent-requested Spawn).
	ChainOwnerSession ChainOwner = iota
	// ChainOwnerCaller means a caller (a background role) owns its chain; the session layer
	// performs exactly the requested route and reports the structured acceptance outcome.
	ChainOwnerCaller
)

const (
	fallbackPhaseBind   = "bind"
	fallbackPhaseCreate = "create"
)

// StartAccepted reports whether a start/bind attempt reached ACP acceptance: a nil error,
// or an error carrying acp.AcceptedStartError anywhere in its chain. It is the only
// acceptance derivation chain owners use; a nil or non-nil process never decides it.
func StartAccepted(err error) bool {
	if err == nil {
		return true
	}
	_, accepted := acp.AcceptedSessionID(err)
	return accepted
}

// FallbackRoute is one ordered pre-acceptance route for a work session.
type FallbackRoute struct {
	Provider        string
	Model           string
	ReasoningEffort string
	Speed           speedpkg.Speed
	ACPOptions      []acp.SessionConfigOptionSelection
	// Command is the route's explicit launch command (its account); empty inherits the
	// provider-aware resolution. It never leaves the resolver boundary as text.
	Command string
}

// label names the route for humans without its command: "<provider>/<model>".
func (r FallbackRoute) label() string {
	provider := strings.TrimSpace(r.Provider)
	model := strings.TrimSpace(r.Model)
	if model == "" {
		return provider
	}
	return provider + "/" + model
}

func (r FallbackRoute) selection() RuntimeSelection {
	return RuntimeSelection{
		Provider:        strings.TrimSpace(r.Provider),
		Model:           strings.TrimSpace(r.Model),
		ReasoningEffort: strings.TrimSpace(r.ReasoningEffort),
		Speed:           r.Speed,
		ACPOptions:      acp.CloneSessionConfigOptionSelections(r.ACPOptions),
	}
}

// fallbackRoutesForAgent maps AgentDef.FallbackChain to FallbackRoute values in order.
func fallbackRoutesForAgent(def compozyconfig.AgentDef) []FallbackRoute {
	if len(def.FallbackChain) == 0 {
		return nil
	}
	routes := make([]FallbackRoute, 0, len(def.FallbackChain))
	for _, route := range def.FallbackChain {
		routes = append(routes, FallbackRoute{
			Provider:        strings.TrimSpace(route.Provider),
			Model:           strings.TrimSpace(route.Model),
			ReasoningEffort: strings.TrimSpace(route.ReasoningEffort),
			Speed:           route.Speed,
			ACPOptions:      ACPOptionSelectionsFromConfig(route.ACPOptions),
			Command:         strings.TrimSpace(route.Command),
		})
	}
	return routes
}

// fallbackAttempt performs one launch for the given route and returns its value and
// error; acceptance is derived by the helper from the error (StartAccepted).
type fallbackAttempt[T any] func(ctx context.Context, attempt int, route FallbackRoute) (T, error)

// fallbackSequence names the invocation-owned identity and sinks that survive every
// attempt: the logical session, the attempt-event writer, and the marker sink.
type fallbackSequence struct {
	session *Session
	agent   string
	phase   string                  // "bind" | "create"
	ledger  store.EventSummaryStore // daemon ledger (GET /api/logs); nil in tests without a ledger
	logger  *slog.Logger
	// effectiveCommand resolves the command a route launches with (explicit, agent,
	// provider, or inherited) so events and markers carry its fingerprint.
	effectiveCommand func(FallbackRoute) string
}

func (s fallbackSequence) commandFor(route FallbackRoute) string {
	if command := strings.TrimSpace(route.Command); command != "" {
		return command
	}
	if s.effectiveCommand == nil {
		return ""
	}
	return s.effectiveCommand(route)
}

func (s fallbackSequence) log() *slog.Logger {
	if s.logger != nil {
		return s.logger
	}
	return slog.Default()
}

// bindWithFallback tries primary, then every route in order, stopping at the first
// accepted attempt (StartAccepted), including an accepted attempt that then failed.
// Before each fallback attempt it commits session.fallback.used to the daemon ledger
// (write failure aborts the sequence) and, for every refused attempt the chain moves
// past, records a provider_failure marker attributed to the refused route. It returns
// the accepted value, the last attempt index, and either nil, the accepted attempt's
// error, or a deterministic exhaustion error naming every attempt.
func (m *Manager) bindWithFallback[T any](
	ctx context.Context,
	seq fallbackSequence,
	primary FallbackRoute,
	routes []FallbackRoute,
	attempt fallbackAttempt[T],
) (T, int, error) {
	var zero T
	if attempt == nil {
		return zero, 0, errors.New("session: fallback attempt callback is required")
	}
	all := make([]FallbackRoute, 0, len(routes)+1)
	all = append(all, primary)
	all = append(all, routes...)
	causes := make([]error, 0, len(all))
	for index, route := range all {
		if index > 0 {
			if err := m.recordSessionFallbackEvent(ctx, seq, index, route); err != nil {
				return zero, index, errors.Join(append(causes, err)...)
			}
		}
		value, err := attempt(ctx, index, route)
		if StartAccepted(err) {
			if err != nil && len(routes) > 0 {
				seq.log().Warn("session.fallback.accepted_with_error",
					"phase", seq.phase, "attempt", index,
					"provider_command_fingerprint", providerCommandFingerprint(seq.commandFor(route)),
					"error", err)
			}
			return value, index, err
		}
		if len(routes) == 0 {
			return zero, index, err
		}
		causes = append(causes, err)
		if context.Cause(ctx) != nil {
			// Cancellation abandons the sequence: no marker, no further route.
			return zero, index, err
		}
		if index+1 >= len(all) {
			break
		}
		next := all[index+1]
		m.recordRefusedAttemptMarker(ctx, seq, index, route, &next, err)
	}
	seq.log().Warn("session.fallback.exhausted", "phase", seq.phase, "attempts", len(all))
	return zero, len(all) - 1, fallbackExhaustedError(all, causes)
}

// fallbackExhaustedError lists every attempt in order and wraps the last cause.
func fallbackExhaustedError(routes []FallbackRoute, causes []error) error {
	if len(causes) == 0 {
		return errors.New("fallback chain exhausted without attempts")
	}
	var prior strings.Builder
	last := len(causes) - 1
	for index, cause := range causes[:last] {
		fmt.Fprintf(&prior, "attempt %d (%s) refused before acceptance: %v; ",
			index+1, routes[index].label(), causeOrUnknown(cause))
	}
	return fmt.Errorf(
		"fallback chain exhausted after %d attempt(s): %sattempt %d (%s) refused before acceptance: %w",
		len(causes), prior.String(), last+1, routes[last].label(), causeOrUnknown(causes[last]),
	)
}

func causeOrUnknown(err error) error {
	if err == nil {
		return errors.New("attempt returned without acceptance")
	}
	return err
}
