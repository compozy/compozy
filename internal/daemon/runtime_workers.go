package daemon

import (
	"context"

	"github.com/compozy/compozy/internal/heartbeat"

	"github.com/compozy/compozy/internal/admission"
	"github.com/compozy/compozy/internal/retention"

	attachmentspkg "github.com/compozy/compozy/internal/attachments"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type daemonRuntimeWorkers struct {
	proactiveChecks       *heartbeat.CheckService
	instanceMaintenance   *retention.PeriodicWorker
	supervision           *retention.PeriodicWorker
	autoTitle             *autoTitleRuntime
	runtimeMemory         *runtimeMemoryMonitor
	hostLoad              *admission.LoadMonitor
	toolArtifacts         *toolspkg.ToolArtifactSweeper
	sessionAttachments    *attachmentspkg.Sweeper
	authoredHeartbeatWake *apiHeartbeatWakePrompter
	loopReconciler        *loopReconcilerRuntime
	managedDeliveries     *ownedWorkerGroup
}

func (w daemonRuntimeWorkers) shutdown(ctx context.Context, errs *[]error) {
	if w.proactiveChecks != nil {
		appendWrappedError(errs, "daemon: shutdown proactive checks", w.proactiveChecks.Shutdown(ctx))
	}
	appendWrappedError(errs, "daemon: stop managed delivery recovery",
		stopManagedDeliveryRecovery(ctx, w.managedDeliveries))
	if w.instanceMaintenance != nil {
		appendWrappedError(errs, "daemon: shutdown instance maintenance", w.instanceMaintenance.Shutdown(ctx))
	}
	if w.supervision != nil {
		appendWrappedError(errs, "daemon: shutdown supervision", w.supervision.Shutdown(ctx))
	}
	if w.loopReconciler != nil {
		appendWrappedError(errs, "daemon: shutdown Loop reconciler", w.loopReconciler.Shutdown(ctx))
	}
	if w.autoTitle != nil {
		appendWrappedError(
			errs,
			"daemon: shutdown automatic title runtime",
			w.autoTitle.Shutdown(ctx),
		)
	}
	if w.runtimeMemory != nil {
		appendWrappedError(
			errs,
			"daemon: shutdown runtime memory monitor",
			w.runtimeMemory.Shutdown(ctx),
		)
	}
	if w.hostLoad != nil {
		appendWrappedError(errs, "daemon: shutdown host load monitor", w.hostLoad.Shutdown(ctx))
	}
	if w.toolArtifacts != nil {
		appendWrappedError(
			errs,
			"daemon: shutdown tool artifact retention",
			w.toolArtifacts.Shutdown(ctx),
		)
	}
	if w.sessionAttachments != nil {
		appendWrappedError(
			errs,
			"daemon: shutdown session attachment retention",
			w.sessionAttachments.Shutdown(ctx),
		)
	}
	if w.authoredHeartbeatWake != nil {
		appendWrappedError(
			errs,
			"daemon: shutdown API heartbeat wake prompter",
			w.authoredHeartbeatWake.shutdown(ctx),
		)
	}
}
