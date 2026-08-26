package settings

import (
	"context"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/config/lifecycle"
	diagnosticcontract "github.com/compozy/compozy/internal/diagnosticcontract"
	"github.com/compozy/compozy/internal/diagnostics"
	skillspkg "github.com/compozy/compozy/internal/skills"
)

func (s *service) persistRuntimeApply(
	ctx context.Context,
	state *activeSnapshot,
	desiredHash string,
	nextActiveHash string,
	nextActiveConfig *compozyconfig.Config,
	configLifecycle lifecycle.Lifecycle,
	noChanges bool,
	writeTarget WriteTargetKind,
	writePath string,
) (ApplyRecord, runtimeApplyPlan, error) {
	plan := newRuntimeApplyPlan(state, nextActiveHash, configLifecycle, noChanges)
	pending, err := s.createPendingApplyRecord(ctx, applyRecordInput{
		desiredHash: desiredHash,
		activeHash:  state.hash,
		generation:  state.generation,
		writeTarget: writeTarget,
		writePath:   writePath,
		lifecycle:   configLifecycle,
	})
	if err != nil {
		return ApplyRecord{}, runtimeApplyPlan{}, err
	}
	if plan.applied && !noChanges {
		plan.partialFailures = s.reconcileRuntimeConfig(
			ctx, nextActiveConfig, configLifecycle, plan.generation,
		)
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
		diagnostics.NewItem(diagnostics.ItemSpec{
			ID:            "config.apply.restart_required",
			Code:          diagnosticcontract.CodeConfigRestartRequired,
			Category:      diagnosticcontract.CategoryConfig,
			Title:         "Daemon restart required",
			Message:       "Desired config was written, but the active generation cannot advance until the daemon restarts.",
			Severity:      diagnosticcontract.SeverityWarn,
			DataFreshness: diagnosticcontract.FreshnessLive,
		},
			diagnostics.WithSuggestedCommand("compozy daemon restart"),
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
	desired *compozyconfig.Config,
	configLifecycle lifecycle.Lifecycle,
	generation int64,
) []ApplyFailure {
	if desired == nil || s.runtimeApplier == nil || !requiresRuntimeReconcile(configLifecycle) {
		return nil
	}
	snapshot := cloneActiveConfig(desired)
	return s.runtimeApplier.ApplyActiveConfig(skillspkg.WithConfigGeneration(ctx, generation), &snapshot)
}

func requiresRuntimeReconcile(configLifecycle lifecycle.Lifecycle) bool {
	switch configLifecycle {
	case lifecycle.Live, lifecycle.LiveAdd, lifecycle.LiveRemoveIfUnused, lifecycle.SessionRebind:
		return true
	default:
		return false
	}
}
