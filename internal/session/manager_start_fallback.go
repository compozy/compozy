package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
)

// launchAcceptedSessionAttempts launches an accepted start. Session-owned creates run the
// agent's fallback_chain inside this one acceptance lifetime: a refused attempt cleans up
// only its process (recorder, catalog row, and session directory survive) and the next
// route starts; exhaustion returns one error the caller settles as one failed start.
func (m *Manager) launchAcceptedSessionAttempts(
	accepted *acceptedSessionStart,
	runtime *sessionStartRuntime,
) (acp.StartOpts, error) {
	spec := accepted.spec
	routes := sessionOwnedStartRoutes(spec, *runtime)
	if len(routes) == 0 {
		return m.launchAcceptedSessionAttempt(accepted, runtime)
	}
	ctx := accepted.run.ctx
	session := accepted.session
	primarySpec := *spec
	primaryRuntime := *runtime
	restorePrimary := func() {
		*spec = primarySpec
		*runtime = primaryRuntime
		session.applyStartAttemptRoute(spec, *runtime)
	}
	primary := FallbackRoute{
		Provider:        runtime.agent.Provider,
		Model:           runtime.agent.Model,
		ReasoningEffort: spec.reasoningEffort,
		Speed:           spec.speed,
		ACPOptions:      acp.CloneSessionConfigOptionSelections(spec.acpOptions),
		Command:         spec.command,
	}
	agentDef := compozyconfig.CloneAgentDef(runtime.agentDef)
	seq := fallbackSequence{
		session: session,
		agent:   spec.agentName,
		phase:   fallbackPhaseCreate,
		ledger:  m.eventLedger,
		logger:  spec.startLogger(m),
		effectiveCommand: func(route FallbackRoute) string {
			return m.resolveFallbackRouteCommand(ctx, primarySpec, agentDef, route)
		},
	}
	var startOpts acp.StartOpts
	_, _, err := bindWithFallback(ctx, m, seq, primary, routes,
		func(ctx context.Context, attempt int, route FallbackRoute) (struct{}, error) {
			if attempt > 0 {
				*spec = primarySpec
				applyFallbackRouteToSpec(spec, route)
				attemptRuntime, err := m.prepareFallbackStartRuntime(ctx, spec)
				if err != nil {
					if attempt < len(routes) {
						restorePrimary()
					}
					return struct{}{}, err
				}
				*runtime = attemptRuntime
				session.applyStartAttemptRoute(spec, *runtime)
			}
			spec.fallbackAttempt = attempt
			opts, err := m.launchAcceptedSessionAttempt(accepted, runtime)
			if StartAccepted(err) {
				startOpts = opts
				return struct{}{}, err
			}
			if cleanupErr := m.cleanupFailedStart("", nil, accepted.proc); cleanupErr != nil {
				spec.startLogger(m).Warn("session.fallback.cleanup_failed",
					"phase", fallbackPhaseCreate, "attempt", attempt, "error", cleanupErr)
				err = errors.Join(err, cleanupErr)
			}
			accepted.proc = nil
			if attempt < len(routes) {
				// The last refused route stays applied so the one failed start (and its
				// provider_failure marker) is attributed to the route that failed.
				restorePrimary()
			}
			return struct{}{}, err
		})
	if err != nil && !StartAccepted(err) {
		return acp.StartOpts{}, fmt.Errorf("session: start %s session %q: %w", spec.startAction, spec.sessionID, err)
	}
	return startOpts, err
}

// launchAcceptedSessionAttempt prepares and starts one route of an accepted start.
func (m *Manager) launchAcceptedSessionAttempt(
	accepted *acceptedSessionStart,
	runtime *sessionStartRuntime,
) (acp.StartOpts, error) {
	ctx := accepted.run.ctx
	spec := accepted.spec
	startOpts, err := m.prepareSessionLaunch(ctx, spec, accepted.session, runtime, accepted.run)
	if err != nil {
		return acp.StartOpts{}, fmt.Errorf(
			"session: prepare %s launch for %q: %w",
			spec.startAction,
			spec.sessionID,
			err,
		)
	}
	accepted.proc, err = m.startAgentProcess(ctx, spec, startOpts)
	if err != nil {
		return acp.StartOpts{}, fmt.Errorf(
			"session: start %s agent process for %q: %w", spec.startAction, spec.sessionID, err,
		)
	}
	return startOpts, nil
}

// sessionOwnedStartRoutes returns the agent chain only for session-owned creates. Role
// launches (ChainOwnerCaller), resumes, and creates whose pinned creation identity fixes
// the provider never run the chain.
func sessionOwnedStartRoutes(spec *sessionStartSpec, runtime sessionStartRuntime) []FallbackRoute {
	if spec == nil || spec.chainOwner != ChainOwnerSession || spec.startAction != sessionStartActionCreate ||
		spec.creationIdentityPinned {
		return nil
	}
	return fallbackRoutesForAgent(runtime.agentDef)
}

func applyFallbackRouteToSpec(spec *sessionStartSpec, route FallbackRoute) {
	selection := route.selection()
	spec.provider = selection.Provider
	spec.model = selection.Model
	spec.transportModel = ""
	spec.reasoningEffort = selection.ReasoningEffort
	spec.speed = selection.Speed
	spec.acpOptions = selection.ACPOptions
	spec.command = route.Command
}

// prepareFallbackStartRuntime re-resolves an accepted start for a fallback route and
// rebuilds its provider-aware startup definition and MCP servers. The soul and the
// creation identity prepared at acceptance are reused: the creation identity records the
// requested creation intent, while the accepted route is recorded in accepted_route.
func (m *Manager) prepareFallbackStartRuntime(
	ctx context.Context,
	spec *sessionStartSpec,
) (sessionStartRuntime, error) {
	runtime, err := m.resolveSessionStartRuntime(ctx, spec, true)
	if err != nil {
		return sessionStartRuntime{}, startupFailure("session fallback route resolution failed", err)
	}
	if !spec.deferRuntimeValidation {
		if err := m.validateExplicitStartModel(ctx, &runtime, spec); err != nil {
			return sessionStartRuntime{}, err
		}
	}
	if err := m.assembleAcceptedStartupDefinition(ctx, spec, &runtime, m.now()); err != nil {
		return sessionStartRuntime{}, startupFailure("session runtime preparation failed", err)
	}
	runtime.mcpServers, err = m.sessionMCPServers(ctx, spec, runtime.agent, runtime.agentDef)
	if err != nil {
		return sessionStartRuntime{}, startupFailure(
			"session runtime preparation failed",
			fmt.Errorf("session: resolve MCP servers for %q: %w", spec.sessionID, err),
		)
	}
	return runtime, nil
}
