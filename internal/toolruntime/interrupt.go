package toolruntime

import (
	"context"
	"errors"
	"fmt"
	"syscall"
	"time"

	"github.com/compozy/agh/internal/procutil"
)

type defaultInterrupter struct{}

func (defaultInterrupter) InterruptProcess(ctx context.Context, record ProcessRecord) error {
	if ctx == nil {
		return errors.New("toolruntime: interrupt context is required")
	}
	if record.PID <= 0 || record.StartedAt.IsZero() {
		return fmt.Errorf("%w: missing pid/start time for process %q", ErrOwnershipValidationFailed, record.ID)
	}
	if !procutil.MatchesStartTime(record.PID, record.StartedAt) {
		return fmt.Errorf(
			"%w: pid %d no longer matches process %q",
			ErrOwnershipValidationFailed,
			record.PID,
			record.ID,
		)
	}

	if err := signalRecord(record, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	if waitForRecordExit(ctx, record, defaultInterruptGrace) {
		return nil
	}
	if record.ProcessGroupID <= 0 && !procutil.MatchesStartTime(record.PID, record.StartedAt) {
		return nil
	}
	if err := signalRecord(record, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	if waitForRecordExit(ctx, record, defaultKillGrace) {
		return nil
	}
	if record.ProcessGroupID > 0 {
		return fmt.Errorf(
			"toolruntime: process group %d for process %q did not exit after interrupt",
			record.ProcessGroupID,
			record.ID,
		)
	}
	if procutil.MatchesStartTime(record.PID, record.StartedAt) {
		return fmt.Errorf("toolruntime: process %q did not exit after interrupt", record.ID)
	}
	return nil
}

func signalRecord(record ProcessRecord, sig syscall.Signal) error {
	if record.ProcessGroupID > 0 {
		return procutil.SignalProcessGroupID(record.ProcessGroupID, sig)
	}
	return procutil.Signal(record.PID, sig)
}

func waitForRecordExit(ctx context.Context, record ProcessRecord, timeout time.Duration) bool {
	if timeout <= 0 {
		timeout = time.Millisecond
	}
	if record.ProcessGroupID > 0 {
		return waitForProcessGroupExit(ctx, record.ProcessGroupID, timeout)
	}
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		if !procutil.MatchesStartTime(record.PID, record.StartedAt) {
			return true
		}
		select {
		case <-waitCtx.Done():
			return false
		case <-ticker.C:
		}
	}
}

func waitForProcessGroupExit(ctx context.Context, pgid int, timeout time.Duration) bool {
	if err := ctx.Err(); err != nil {
		return false
	}
	return procutil.WaitForProcessGroupIDExit(pgid, timeout) == nil
}
