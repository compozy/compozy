package daemon

import (
	"context"
	"errors"
	"fmt"

	"strings"

	"github.com/compozy/agh/internal/coordinator"

	"github.com/compozy/agh/internal/session"
)

func (r *coordinatorRuntime) beginCoordinatorWakeLocked(info *session.Info, decision coordinator.Decision) bool {
	if r == nil {
		return false
	}
	key := coordinatorWakeInFlightKey(info, decision)
	if key == "" {
		return true
	}
	if r.wakeInFlight == nil {
		r.wakeInFlight = make(map[string]struct{})
	}
	if _, ok := r.wakeInFlight[key]; ok {
		return false
	}
	r.wakeInFlight[key] = struct{}{}
	return true
}

func (r *coordinatorRuntime) finishCoordinatorWake(info *session.Info, decision coordinator.Decision) {
	if r == nil {
		return
	}
	key := coordinatorWakeInFlightKey(info, decision)
	if key == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.wakeInFlight, key)
}

func coordinatorWakeInFlightKey(info *session.Info, decision coordinator.Decision) string {
	if info == nil {
		return ""
	}
	sessionID := strings.TrimSpace(info.ID)
	runID := strings.TrimSpace(decision.RunID)
	if sessionID == "" || runID == "" {
		return ""
	}
	return sessionID + "\x00" + runID
}

func (r *coordinatorRuntime) shutdown(ctx context.Context) error {
	if r == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("daemon: coordinator runtime shutdown context is required")
	}
	if r.cancel != nil {
		r.cancel()
	}
	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("daemon: shutdown coordinator runtime: %w", ctx.Err())
	}
}

func coordinatorWakeMessage(decision coordinator.Decision) string {
	taskID := strings.TrimSpace(decision.TaskID)
	runID := strings.TrimSpace(decision.RunID)
	return fmt.Sprintf(
		"A task run is queued for this coordinator.\n\n"+
			"Task: %s\nRun: %s\n\n"+
			"Claim the run through the AGH task claim path by running `agh task next -o json` once without long-polling, then route from durable receipts. "+
			"If the receipts require human input, park the run with the AGH task block path.",
		taskID,
		runID,
	)
}

func coordinatorWakeSummary(decision coordinator.Decision) string {
	return fmt.Sprintf(
		"Coordinator wake for task %s run %s",
		strings.TrimSpace(decision.TaskID),
		strings.TrimSpace(decision.RunID),
	)
}
