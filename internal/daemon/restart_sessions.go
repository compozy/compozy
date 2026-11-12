package daemon

import (
	"context"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/session"
)

type restartSessionManager interface {
	CaptureRestartInterruptions(context.Context, []*session.Info) error
	PrepareDaemonShutdown(context.Context, bool) error
	ResumeInterruptedSessions(context.Context) error
}

func recoverBootPendingStops(ctx context.Context, sessions SessionManager) error {
	if recoverer, ok := sessions.(sessionPendingStopRecoverer); ok {
		if err := recoverer.RecoverPendingStops(ctx); err != nil {
			return fmt.Errorf("daemon: recover pending session stops: %w", err)
		}
	}
	return nil
}

func captureBootRestartInterruptions(ctx context.Context, sessions SessionManager, infos []*session.Info) error {
	if recoverer, ok := sessions.(restartSessionManager); ok {
		if err := recoverer.CaptureRestartInterruptions(ctx, infos); err != nil {
			return fmt.Errorf("daemon: capture interrupted sessions: %w", err)
		}
	}
	return nil
}

func resumeBootInterruptedSessions(ctx context.Context, sessions SessionManager) error {
	if recoverer, ok := sessions.(restartSessionManager); ok {
		return recoverer.ResumeInterruptedSessions(ctx)
	}
	return nil
}

// Recovery can submit provider prompts, so tools and transports must already be listening.
func recoverBootDelegatedSessions(ctx context.Context, state *bootState) error {
	if state.subagents != nil {
		if err := state.subagents.Recover(ctx); err != nil {
			return fmt.Errorf("daemon: recover subagents: %w", err)
		}
	}
	if manager, ok := state.sessions.(*session.Manager); ok {
		if service := manager.ReplyWatches(); service != nil {
			if err := service.Recover(ctx); err != nil {
				return fmt.Errorf("daemon: recover reply watches: %w", err)
			}
		}
	}
	return nil
}

// daemonShutdownBeginner closes agent-runtime admission before the shutdown drains.
type daemonShutdownBeginner interface {
	BeginDaemonShutdown(restart bool)
}

// beginSessionShutdown tells the session manager that shutdown has begun. It is distinct from the
// operator drain, which only closes new-work admission and can be reopened.
func (d *Daemon) beginSessionShutdown(sessions SessionManager) {
	if beginner, ok := sessions.(daemonShutdownBeginner); ok {
		beginner.BeginDaemonShutdown(d.restartInProgress())
	}
}

func (d *Daemon) prepareShutdownSessions(ctx context.Context, sessions SessionManager) error {
	d.mu.Lock()
	restart, grace := d.restartRequested, d.config.Daemon.ShutdownGrace
	d.mu.Unlock()
	if restart && grace > 0 {
		wait := restartDrainWait(grace)
		if busy := waitSessionDrain(ctx, sessions, wait); busy > 0 {
			d.runtimeLogger().Info("daemon: restart drain ended with turns still running",
				"running_turns", busy,
				"drain_ms", wait.Milliseconds(),
			)
		}
	}
	if recoverer, ok := sessions.(restartSessionManager); ok {
		return recoverer.PrepareDaemonShutdown(ctx, restart)
	}
	return nil
}

// waitSessionDrain gives running turns up to wait to finish and returns how many were still running.
// It returns as soon as none is left. Turns blocked on the operator are never waited for.
func waitSessionDrain(ctx context.Context, sessions SessionManager, wait time.Duration) int {
	reader, ok := sessions.(sessionPromptStateReader)
	if !ok {
		return 0
	}
	drainCtx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		busy := 0
		for _, info := range sessions.List() {
			if info != nil && reader.IsPrompting(info.ID) && !sessionWaitsOnOperator(info) {
				busy++
			}
		}
		if busy == 0 {
			return 0
		}
		select {
		case <-drainCtx.Done():
			return busy
		case <-ticker.C:
		}
	}
}

// sessionWaitsOnOperator reports a turn blocked on an approval, a clarification or another pending
// interaction. Only the operator can release it, so no drain wait helps it finish.
func sessionWaitsOnOperator(info *session.Info) bool {
	return len(info.PendingInteractions) > 0 || info.PendingPermissionCount > 0 || info.PendingClarifyCount > 0
}

// restartInProgress reports that the daemon is shutting down to relaunch itself, so session stops are
// interruptions rather than decisions.
func (d *Daemon) restartInProgress() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.restartRequested
}
