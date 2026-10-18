package session

import (
	"context"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
)

// bindPendingRoute performs the first bind of a derived child created with a declared
// route: the same index is resolved again through the fallback-account resolver and its
// command becomes RuntimeOverrides.Command. A missing or changed route fails the bind
// with route_not_found and leaves the session unbound; nothing runs on the default
// account. A prompt that selects a different runtime replaces the pending route.
func (m *Manager) bindPendingRoute(
	ctx context.Context,
	session *Session,
	snapshot *runtimeBindingSnapshot,
	plan *promptRuntimePlan,
	pending store.SessionPendingRoute,
) (*AgentProcess, bool, error) {
	if !pendingRouteMatchesSelection(pending, plan.selection) {
		if session.clearPendingRoute() {
			if err := m.persistSessionMetadataOnly(session); err != nil {
				return nil, true, err
			}
		}
		return nil, false, nil
	}
	agentName := session.Info().AgentName
	routes := fallbackRoutesForAgent(plan.runtime.agentDef)
	if pending.Index > len(routes) {
		return nil, true, m.restorePromptRuntime(
			session, snapshot, newDeriveRouteError(agentName, len(routes), pending.Index),
		)
	}
	route := routes[pending.Index-1]
	if compozyconfig.CommandFingerprint(route.Command) != strings.TrimSpace(pending.CommandFingerprint) {
		return nil, true, m.restorePromptRuntime(session, snapshot, deriveErr(
			ErrDeriveRouteNotFound,
			"route_not_found: agent %q route %d changed since it was chosen", agentName, pending.Index,
		))
	}
	routePlan, err := m.preparePromptRuntimePlanForRoute(ctx, session, plan.selection, route.Command)
	if err != nil {
		return nil, true, err
	}
	if err := m.validateExplicitStartModel(ctx, &routePlan.runtime, &routePlan.spec); err != nil {
		return nil, true, err
	}
	routePlan.attempt = pending.Index
	proc, err := m.replacePromptRuntime(ctx, session, snapshot, routePlan)
	if err != nil {
		return nil, true, err
	}
	if session.clearPendingRoute() {
		if err := m.persistSessionMetadataOnly(session); err != nil {
			m.sessionLogger(session).Warn("session.pending_route.clear_persist_failed", "error", err)
		}
	}
	return proc, true, nil
}

func pendingRouteMatchesSelection(pending store.SessionPendingRoute, selection RuntimeSelection) bool {
	if strings.TrimSpace(selection.Provider) != strings.TrimSpace(pending.Provider) {
		return false
	}
	model := strings.TrimSpace(pending.Model)
	return model == "" || strings.TrimSpace(selection.Model) == model
}
