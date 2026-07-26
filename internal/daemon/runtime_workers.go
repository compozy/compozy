package daemon

import (
	"context"

	toolspkg "github.com/compozy/agh/internal/tools"
)

type daemonRuntimeWorkers struct {
	autoTitle     *autoTitleRuntime
	runtimeMemory *runtimeMemoryMonitor
	toolArtifacts *toolspkg.ToolArtifactSweeper
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
}
