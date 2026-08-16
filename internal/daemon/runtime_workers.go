package daemon

import (
	"context"

	attachmentspkg "github.com/compozy/compozy/internal/attachments"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type daemonRuntimeWorkers struct {
	autoTitle             *autoTitleRuntime
	runtimeMemory         *runtimeMemoryMonitor
	toolArtifacts         *toolspkg.ToolArtifactSweeper
	sessionAttachments    *attachmentspkg.Sweeper
	authoredHeartbeatWake *apiHeartbeatWakePrompter
}

func (w daemonRuntimeWorkers) shutdown(ctx context.Context, errs *[]error) {
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
