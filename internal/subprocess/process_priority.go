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

// StartWithLaunchNiceness applies Linux priority before exec, without changing executable identity.
func StartWithLaunchNiceness(logger *slog.Logger, niceness int, start func() error) error {
	priorityErr, startErr := procutil.StartWithNiceness(niceness, start)
	if priorityErr != nil {
		logNicenessFailure(logger, 0, niceness, priorityErr)
	}
	return startErr
}

// ApplyLaunchNiceness lowers the scheduling priority of the launched process
// group after start. Linux launches already inherit priority before exec; other
// Unix platforms retain the in-flight fork limitation of SetProcessGroupNiceness.
// Failure never aborts the launch. A failed call can still have lowered some
// group members, and those changes are not restored; the remaining members keep
// the daemon's priority.
func ApplyLaunchNiceness(logger *slog.Logger, pid int, niceness int) {
	if niceness <= 0 || pid <= 0 {
		return
	}
	err := setProcessGroupNiceness(pid, niceness)
	if err == nil {
		return
	}
	logNicenessFailure(logger, pid, niceness, err)
}

func logNicenessFailure(logger *slog.Logger, pid, niceness int, err error) {
	if nicenessFailureWarned.CompareAndSwap(false, true) {
		logger.Warn("subprocess: lower process group priority failed; continuing at default priority",
			"pid", pid, "niceness", niceness, "error", err)
		return
	}
	logger.Debug("subprocess: lower process group priority failed",
		"pid", pid, "niceness", niceness, "error", err)
}
