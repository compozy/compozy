package settings

import (
	"context"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/config/lifecycle"
	diagnosticcontract "github.com/compozy/agh/internal/diagnosticcontract"
	"github.com/compozy/agh/internal/diagnostics"
)

func (s *service) persistRuntimeApply(
	ctx context.Context,
	state *activeSnapshot,
	desiredHash string,
	nextActiveHash string,
	nextActiveConfig *aghconfig.Config,
	configLifecycle lifecycle.Lifecycle,
	noChanges bool,
) (ApplyRecord, runtimeApplyPlan, error) {
	plan := newRuntimeApplyPlan(state, nextActiveHash, configLifecycle, noChanges)
	pending, err := s.createPendingApplyRecord(ctx, applyRecordInput{
		desiredHash: desiredHash,
		activeHash:  state.hash,
		generation:  state.generation,
		lifecycle:   configLifecycle,
	})
	if err != nil {
		return ApplyRecord{}, runtimeApplyPlan{}, err
	}
	if plan.applied && !noChanges {
		plan.partialFailures = s.reconcileRuntimeConfig(ctx, nextActiveConfig, configLifecycle)
		if len(plan.partialFailures) > 0 {
			plan.status = lifecycle.StatusFailed
			plan.activeHash = state.hash
			plan.generation = state.generation
			plan.applied = false
			plan.diagnostics = diagnosticsFromApplyFailures(plan.partialFailures)
		}
	}
	if len(plan.diagnostics) == 0 {
		plan.diagnostics = restartRequiredDiagnostics(configLifecycle, plan.status)
	}
	record, err := s.finalizeApplyRecord(ctx, pending, applyRecordInput{
		desiredHash:  desiredHash,
		activeHash:   plan.activeHash,
		generation:   plan.generation,
		lifecycle:    configLifecycle,
		status:       plan.status,
		diagnostics:  plan.diagnostics,
		appliedAtNow: plan.applied && !noChanges,
	})
	if err != nil {
		return ApplyRecord{}, runtimeApplyPlan{}, err
	}
	if plan.applied && !noChanges {
		s.advanceActiveConfig(nextActiveConfig, nextActiveHash, plan.generation)
	}
	return record, plan, nil
}

func restartScopeForLifecycle(configLifecycle lifecycle.Lifecycle) string {
	if configLifecycle == lifecycle.RestartRequired {
		return restartScopeDaemon
	}
	return ""
}

func restartRequiredDiagnostics(
	configLifecycle lifecycle.Lifecycle,
	status lifecycle.Status,
) []diagnosticcontract.DiagnosticItem {
	if configLifecycle != lifecycle.RestartRequired || status != lifecycle.StatusBlocked {
		return nil
	}
	return []diagnosticcontract.DiagnosticItem{
		diagnostics.NewItem(
			"config.apply.restart_required",
			diagnosticcontract.CodeConfigRestartRequired,
			diagnosticcontract.CategoryConfig,
			"Daemon restart required",
			"Desired config was written, but the active generation cannot advance until the daemon restarts.",
			diagnosticcontract.SeverityWarn,
			diagnosticcontract.FreshnessLive,
			diagnostics.WithSuggestedCommand("agh daemon restart"),
		),
	}
}

func diagnosticsFromApplyFailures(
	failures []ApplyFailure,
) []diagnosticcontract.DiagnosticItem {
	if len(failures) == 0 {
		return nil
	}
	items := make([]diagnosticcontract.DiagnosticItem, 0, len(failures))
	for _, failure := range failures {
		items = append(items, failure.Diagnostic)
	}
	return items
}

func (s *service) reconcileRuntimeConfig(
	ctx context.Context,
	desired *aghconfig.Config,
	configLifecycle lifecycle.Lifecycle,
) []ApplyFailure {
	if desired == nil || s.runtimeApplier == nil || !requiresRuntimeReconcile(configLifecycle) {
		return nil
	}
	snapshot := cloneActiveConfig(desired)
	return s.runtimeApplier.ApplyActiveConfig(ctx, &snapshot)
}

func requiresRuntimeReconcile(configLifecycle lifecycle.Lifecycle) bool {
	switch configLifecycle {
	case lifecycle.Live, lifecycle.LiveAdd, lifecycle.LiveRemoveIfUnused, lifecycle.SessionRebind:
		return true
	default:
		return false
	}
}
