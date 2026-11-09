package session

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
)

// bindPromptRuntimeWithFallback runs the first bind of a logical session through the
// agent's fallback_chain. The caller holds the prompt's exclusive setup reservation for
// the whole sequence, so no concurrent prompt can start a second sequence; each refused
// attempt is restored to the unbound snapshot by replacePromptRuntime before the next.
func (m *Manager) bindPromptRuntimeWithFallback(
	ctx context.Context,
	session *Session,
	snapshot *runtimeBindingSnapshot,
	plan *promptRuntimePlan,
	routes []FallbackRoute,
) (*AgentProcess, error) {
	primary := FallbackRoute{
		Provider:        plan.selection.Provider,
		Model:           plan.selection.Model,
		ReasoningEffort: plan.selection.ReasoningEffort,
		Speed:           plan.selection.Speed,
		ACPOptions:      acp.CloneSessionConfigOptionSelections(plan.selection.ACPOptions),
		Command:         plan.spec.command,
		SessionEnv:      compozyconfig.MergeSessionEnv(nil, plan.spec.sessionEnv),
	}
	baseSpec := plan.spec
	agentDef := compozyconfig.CloneAgentDef(plan.runtime.agentDef)
	seq := fallbackSequence{
		session: session,
		agent:   session.Info().AgentName,
		phase:   fallbackPhaseBind,
		ledger:  m.eventLedger,
		logger:  m.sessionLogger(session),
		effectiveCommand: func(route FallbackRoute) string {
			return m.resolveFallbackRouteCommand(ctx, &baseSpec, agentDef, route)
		},
	}
	proc, _, err := m.bindWithFallback(ctx, seq, primary, routes,
		func(ctx context.Context, attempt int, route FallbackRoute) (*AgentProcess, error) {
			attemptPlan := plan
			if attempt > 0 {
				routePlan, err := m.preparePromptRuntimePlanForRoute(
					ctx,
					session,
					route.selection(),
					route.Command,
					route.SessionEnv,
				)
				if err != nil {
					return nil, err
				}
				if err := m.validateExplicitStartModel(ctx, &routePlan.runtime, &routePlan.spec); err != nil {
					return nil, err
				}
				attemptPlan = routePlan
			}
			attemptPlan.attempt = attempt
			attemptPlan.spec.fallbackAttempt = attempt
			return m.replacePromptRuntime(ctx, session, snapshot, attemptPlan)
		})
	if err != nil {
		return nil, fmt.Errorf("session: bind runtime for %q: %w", session.ID, err)
	}
	return proc, nil
}

// resolveFallbackRouteCommand resolves the command a route launches with, applying the
// same provider-aware and spawn-inheritance rules as the launch itself.
func (m *Manager) resolveFallbackRouteCommand(
	ctx context.Context,
	base *sessionStartSpec,
	agentDef compozyconfig.AgentDef,
	route FallbackRoute,
) string {
	if base == nil {
		return ""
	}
	spec := *base
	spec.provider = strings.TrimSpace(route.Provider)
	spec.model = strings.TrimSpace(route.Model)
	spec.reasoningEffort = strings.TrimSpace(route.ReasoningEffort)
	spec.command = strings.TrimSpace(route.Command)
	resolved, err := spec.workspace.Config.ResolveSessionAgentWithRuntime(agentDef, compozyconfig.RuntimeOverrides{
		Provider: spec.provider, Model: spec.model, Reasoning: spec.reasoningEffort, Command: spec.command,
		SessionEnv: route.SessionEnv,
	})
	if err != nil {
		return ""
	}
	resolved, err = m.resolveSpawnProviderCommand(ctx, &spec, agentDef, resolved, map[string]bool{spec.sessionID: true})
	if err != nil {
		return ""
	}
	return resolved.Command
}

// bindUnboundPromptRuntime owns the binds that differ for a session without a process:
// a released idle runtime reloads its ACP session, a derived child binds its pending
// route, and a logical session's first bind walks the agent's fallback_chain (ADR-004).
func (m *Manager) bindUnboundPromptRuntime(
	ctx context.Context,
	session *Session,
	snapshot *runtimeBindingSnapshot,
	plan *promptRuntimePlan,
) (*AgentProcess, bool, error) {
	if snapshot.runtimeRelease != nil {
		proc, err := m.rebindReleasedRuntime(ctx, session, snapshot, plan)
		return proc, true, err
	}
	if pending := session.pendingRouteSnapshot(); pending != nil {
		proc, handled, err := m.bindPendingRoute(ctx, session, snapshot, plan, *pending)
		if handled {
			return proc, true, err
		}
	}
	// A continuation's explicit provider selection must fail rather than run another route.
	if derived := plan.spec.derivation; derived != nil && derived.Kind == store.LineageKindContinue &&
		plan.spec.selectedRuntime != nil {
		return nil, false, nil
	}
	if routes := fallbackRoutesForAgent(plan.runtime.agentDef); len(routes) > 0 {
		proc, err := m.bindPromptRuntimeWithFallback(ctx, session, snapshot, plan, routes)
		return proc, true, err
	}
	return nil, false, nil
}
