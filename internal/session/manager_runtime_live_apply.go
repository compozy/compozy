package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/transcript"
)

const (
	// liveRuntimeApplyTimeout bounds one live set_config_option exchange, including a model switch.
	liveRuntimeApplyTimeout = 30 * time.Second
	// runtimeChangedEvidenceProviderKey names the applied provider in a runtime changed marker.
	runtimeChangedEvidenceProviderKey = "provider"
)

// applyRuntimeSelectionLive sends the stored selection to a live session_config runtime
// instead of waiting for the next prompt. It never replaces the process or flips the
// runtime status, so an in-flight turn keeps running under unchanged supervision. A
// refusal or transport failure is recorded on the runtime; the stored selection stays
// and the next prompt's admission retries it through ensurePromptRuntime.
func (m *Manager) applyRuntimeSelectionLive(session *Session) {
	if m == nil || session == nil {
		return
	}
	configurator, ok := m.driver.(RuntimeConfigurator)
	if !ok {
		return
	}
	session.runtimeApplyMu.Lock()
	defer session.runtimeApplyMu.Unlock()

	snapshot := session.runtimeBindingSnapshot()
	if !liveRuntimeApplyEligible(&snapshot) {
		return
	}
	selection, err := normalizePromptRuntimeSelection(session, nil)
	if err != nil || runtimeSelectionsEqual(snapshot.selection, *selection) && !runtimeControlsDrifted(&snapshot) {
		return
	}

	ctx, cancel := context.WithTimeout(m.lifecycleCtx, liveRuntimeApplyTimeout)
	defer cancel()
	plan, err := m.prepareLiveRuntimePlan(ctx, session, &snapshot, *selection)
	if err != nil || plan == nil {
		m.recordLiveRuntimeFailure(ctx, session, &snapshot, err)
		return
	}
	requested := runtimeConfigForSelection(plan.selection, plan.spec.transportModel)
	rollback := runtimeConfigForSelection(snapshot.selection, currentTransportModel(snapshot.process.CapsSnapshot()))
	if err := configurator.ConfigureRuntime(ctx, snapshot.process, requested); err != nil {
		cause := fmt.Errorf("session: apply runtime selection live: %w", err)
		m.recordLiveRuntimeFailure(
			ctx,
			session,
			&snapshot,
			m.rollbackLiveRuntime(configurator, &snapshot, rollback, cause),
		)
		return
	}
	if !session.commitLiveRuntimeSelection(snapshot.process, plan.selection, m.now()) {
		return
	}
	if err := m.persistSessionLifecycleState(ctx, session, false); err != nil {
		cause := m.rollbackLiveRuntime(configurator, &snapshot, rollback, err)
		session.commitLiveRuntimeSelection(snapshot.process, snapshot.selection, m.now())
		m.recordLiveRuntimeFailure(ctx, session, &snapshot, cause)
		return
	}
	if plan.runtime.startupRebuilt {
		session.setRuntimeDefinition(&plan.runtime)
		if replacer, ok := m.driver.(StartupPromptReplacer); ok {
			replacer.ReplacePendingStartupPrompt(
				snapshot.process,
				plan.runtime.agent.Prompt,
				plan.runtime.startupManifest,
			)
		}
	}
	m.recordRuntimeChangedMarker(ctx, session, &snapshot, plan.selection)
}

// liveRuntimeApplyEligible admits only a ready process whose provider honors session config;
// binds, recoveries, and released runtimes keep their owner and apply at the next prompt.
func liveRuntimeApplyEligible(snapshot *runtimeBindingSnapshot) bool {
	return snapshot.process != nil && !isProcessDone(snapshot.process) &&
		snapshot.runtimeStrategy == acp.RuntimeApplicationSessionConfig &&
		snapshot.status == RuntimeStatusReady && snapshot.runtimeRelease == nil
}

// prepareLiveRuntimePlan returns nil without error when the selection needs a new process.
func (m *Manager) prepareLiveRuntimePlan(
	ctx context.Context,
	session *Session,
	snapshot *runtimeBindingSnapshot,
	selection RuntimeSelection,
) (*promptRuntimePlan, error) {
	if strings.TrimSpace(snapshot.selection.Provider) != strings.TrimSpace(selection.Provider) {
		return nil, nil
	}
	plan, err := m.preparePromptRuntimePlan(ctx, session, selection)
	if err != nil {
		return nil, err
	}
	runtimeProvider := strings.TrimSpace(plan.runtime.agent.RuntimeProvider)
	if runtimeProvider == "" {
		runtimeProvider = strings.TrimSpace(plan.runtime.agent.Provider)
	}
	if strings.TrimSpace(snapshot.selection.Provider) != strings.TrimSpace(plan.selection.Provider) ||
		RuntimeStrategyForProvider(runtimeProvider) != acp.RuntimeApplicationSessionConfig {
		return nil, nil
	}
	if err := m.validateExplicitStartModel(ctx, &plan.runtime, &plan.spec); err != nil {
		return nil, err
	}
	return plan, nil
}

// rollbackLiveRuntime restores the previous provider config after a partial apply. A failed
// rollback keeps the process bound: the effective runtime still differs from the stored
// selection, so the next prompt reconfigures or replaces it before dispatch.
func (m *Manager) rollbackLiveRuntime(
	configurator RuntimeConfigurator,
	snapshot *runtimeBindingSnapshot,
	rollback acp.RuntimeConfig,
	cause error,
) error {
	cleanupCtx, cancel := m.lifecycleCleanupContext()
	defer cancel()
	return errors.Join(cause, configurator.ConfigureRuntime(cleanupCtx, snapshot.process, rollback))
}

func (m *Manager) recordLiveRuntimeFailure(
	ctx context.Context,
	session *Session,
	snapshot *runtimeBindingSnapshot,
	cause error,
) {
	if cause == nil {
		return
	}
	m.sessionLogger(session).Warn("session: live runtime apply failed; next prompt retries", "error", cause)
	if !session.recordLiveRuntimeFailure(snapshot.process, cause.Error(), m.now()) {
		return
	}
	if err := m.persistSessionLifecycleState(ctx, session, false); err != nil {
		m.sessionLogger(session).Warn("session: persist live runtime failure", "error", err)
	}
}

// recordRuntimeChangedMarker makes a turn with mixed runtimes explainable: events recorded
// after the marker carry the new effective runtime.
func (m *Manager) recordRuntimeChangedMarker(
	ctx context.Context,
	session *Session,
	snapshot *runtimeBindingSnapshot,
	applied RuntimeSelection,
) {
	turnID := session.CurrentTurnID()
	duringTurn := turnID != ""
	if !duringTurn {
		var err error
		if turnID, err = m.newPromptTurnID(); err != nil {
			m.sessionLogger(session).Warn("session: runtime changed marker turn id", "error", err)
			return
		}
	}
	runtimeProvider := strings.TrimSpace(session.providerRoutingSnapshot().RuntimeProvider)
	if runtimeProvider == "" {
		runtimeProvider = strings.TrimSpace(applied.Provider)
	}
	m.emitTranscriptMarker(ctx, session, turnID, transcript.MarkerRuntimeChanged,
		runtimeChangedSummary(snapshot.selection, applied),
		map[string]any{
			runtimeChangedEvidenceProviderKey: applied.Provider,
			"previous_model":                  snapshot.selection.Model,
			sessionModelConfigKey:             applied.Model,
			"previous_reasoning_effort":       snapshot.selection.ReasoningEffort,
			"reasoning_effort":                applied.ReasoningEffort,
			"previous_speed":                  string(snapshot.selection.Speed),
			"speed":                           string(applied.Speed),
			"during_turn":                     duringTurn,
			"live_apply":                      string(RuntimeLiveApplyForProvider(runtimeProvider)),
		})
}

func runtimeChangedSummary(previous RuntimeSelection, applied RuntimeSelection) string {
	changes := make([]string, 0, 3)
	if previous.Model != applied.Model && applied.Model != "" {
		changes = append(changes, "model "+applied.Model)
	}
	if previous.ReasoningEffort != applied.ReasoningEffort && applied.ReasoningEffort != "" {
		changes = append(changes, "reasoning "+applied.ReasoningEffort)
	}
	if previous.Speed != applied.Speed && applied.Speed != "" {
		changes = append(changes, "speed "+string(applied.Speed))
	}
	if len(changes) == 0 {
		return "Runtime settings applied"
	}
	return "Runtime changed to " + strings.Join(changes, ", ")
}
