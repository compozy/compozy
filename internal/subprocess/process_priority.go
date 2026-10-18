package subprocess

import (
	"log/slog"
	"sync/atomic"

	"github.com/compozy/compozy/internal/procutil"
)

// nicenessFailureWarned keeps a persistent setpriority failure (for example a
// sandbox that forbids it) to one warning per daemon; later failures log at debug.
var nicenessFailureWarned atomic.Bool

// setProcessGroupNiceness is replaceable by tests that exercise the failure path.
var setProcessGroupNiceness = procutil.SetProcessGroupNiceness

// applyLaunchNiceness lowers the scheduling priority of the launched process
// group. Failure never aborts the launch: the process keeps running at the
// daemon's priority.
func applyLaunchNiceness(logger *slog.Logger, pid int, niceness int) {
	if niceness <= 0 || pid <= 0 {
		return
	}
	err := setProcessGroupNiceness(pid, niceness)
	if err == nil {
		return
	}
	if nicenessFailureWarned.CompareAndSwap(false, true) {
		logger.Warn("subprocess: lower process group priority failed; continuing at default priority",
			"pid", pid, "niceness", niceness, "error", err)
		return
	}
	logger.Debug("subprocess: lower process group priority failed",
		"pid", pid, "niceness", niceness, "error", err)
}
