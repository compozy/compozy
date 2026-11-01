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

func (d *Daemon) prepareShutdownSessions(ctx context.Context, sessions SessionManager) error {
	d.mu.Lock()
	restart, grace := d.restartRequested, d.config.Daemon.ShutdownGrace
	d.mu.Unlock()
	if restart && grace > 0 {
		waitSessionDrain(ctx, sessions, grace)
	}
	if recoverer, ok := sessions.(restartSessionManager); ok {
		return recoverer.PrepareDaemonShutdown(ctx, restart)
	}
	return nil
}

func waitSessionDrain(ctx context.Context, sessions SessionManager, grace time.Duration) {
	reader, ok := sessions.(sessionPromptStateReader)
	if !ok {
		return
	}
	drainCtx, cancel := context.WithTimeout(ctx, grace)
	defer cancel()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		busy := false
		for _, info := range sessions.List() {
			if info != nil && reader.IsPrompting(info.ID) {
				busy = true
				break
			}
		}
		if !busy {
			return
		}
		select {
		case <-drainCtx.Done():
			return
		case <-ticker.C:
		}
	}
}

// restartInProgress reports that the daemon is shutting down to relaunch itself, so session stops are
// interruptions rather than decisions.
func (d *Daemon) restartInProgress() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.restartRequested
}
