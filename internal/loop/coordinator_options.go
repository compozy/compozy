package loop

import (
	"time"

	"github.com/compozy/compozy/internal/loop/gate"
	watchpkg "github.com/compozy/compozy/internal/loop/watch"
)

// DefaultWatchSilenceWindow is the fallback inactivity clock for watch-source loops.
const DefaultWatchSilenceWindow = 30 * time.Minute

// WatchPoller is the coordinator seam for daemon -> extension watch/poll calls.
type WatchPoller = watchpkg.Poller

// CoordinatorRunnerOption configures the loop coordinator runner.
type CoordinatorRunnerOption func(*CoordinatorRunner)

// WithCoordinatorHookDispatcher injects loop hook dispatch for coordinator call sites.
func WithCoordinatorHookDispatcher(hooks HookDispatcher) CoordinatorRunnerOption {
	return func(r *CoordinatorRunner) {
		r.hooks = hooks
	}
}

// WithCoordinatorWatchPoller injects the watch-source poll bridge used by source/watch-source nodes.
func WithCoordinatorWatchPoller(poller WatchPoller) CoordinatorRunnerOption {
	return func(r *CoordinatorRunner) {
		r.watchPoller = poller
	}
}

// WithCoordinatorWatchEventsLedger injects the watch-events replay reader.
func WithCoordinatorWatchEventsLedger(ledger WatchEventsLedger) CoordinatorRunnerOption {
	return func(r *CoordinatorRunner) {
		r.watchEventsLedger = ledger
	}
}

// WithCoordinatorGateEvaluator injects runtime evaluation for gate control nodes.
func WithCoordinatorGateEvaluator(evaluator gate.GateEvaluator) CoordinatorRunnerOption {
	return func(r *CoordinatorRunner) {
		r.gateEvaluator = evaluator
	}
}

// WithCoordinatorVerdictReader injects persisted machine-verdict history.
func WithCoordinatorVerdictReader(reader gate.VerdictReader) CoordinatorRunnerOption {
	return func(r *CoordinatorRunner) {
		r.verdicts = reader
	}
}

// WithCoordinatorNodeAttemptReader injects the durable attempt-ledger reader.
func WithCoordinatorNodeAttemptReader(reader NodeAttemptReader) CoordinatorRunnerOption {
	return func(r *CoordinatorRunner) {
		r.attempts = reader
	}
}

// WithCoordinatorRetryRand injects the retry jitter source.
func WithCoordinatorRetryRand(randFloat64 func() float64) CoordinatorRunnerOption {
	return func(r *CoordinatorRunner) {
		if randFloat64 != nil {
			r.retryRand = randFloat64
		}
	}
}

// WithCoordinatorActionRegistry injects runtime action execution for worker node runs.
func WithCoordinatorActionRegistry(registry *ActionRegistry) CoordinatorRunnerOption {
	return func(r *CoordinatorRunner) {
		r.actionRegistry = registry
	}
}

// WithCoordinatorRuntimeCatalog injects pre-bind provider/model validation.
func WithCoordinatorRuntimeCatalog(catalog WorkspaceRuntimeCatalog) CoordinatorRunnerOption {
	return func(r *CoordinatorRunner) {
		r.runtimeCatalog = catalog
	}
}

// WithCoordinatorWatchSilenceWindow overrides the watch-source inactivity window.
func WithCoordinatorWatchSilenceWindow(window time.Duration) CoordinatorRunnerOption {
	return func(r *CoordinatorRunner) {
		if window > 0 {
			r.watchSilenceWindow = window
		}
	}
}
