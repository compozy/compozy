package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/session"
	taskpkg "github.com/compozy/agh/internal/task"
)

type taskWakeSessionManager interface {
	PromptSynthetic(ctx context.Context, id string, opts session.SyntheticPromptOpts) (<-chan acp.AgentEvent, error)
}

type taskWakeBridge struct {
	// drainCtx anchors detached synthetic wake event drainers to the daemon lifetime.
	drainCtx context.Context
	cancel   context.CancelFunc
	sessions taskWakeSessionManager
	logger   *slog.Logger
	wg       sync.WaitGroup
}

var _ taskpkg.WakeNotifier = (*taskWakeBridge)(nil)

func newTaskWakeBridge(
	ctx context.Context,
	sessions taskWakeSessionManager,
	logger *slog.Logger,
) (*taskWakeBridge, error) {
	if sessions == nil {
		return nil, errors.New("daemon: task wake bridge requires sessions")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if logger == nil {
		logger = slog.Default()
	}
	drainCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	return &taskWakeBridge{
		drainCtx: drainCtx,
		cancel:   cancel,
		sessions: sessions,
		logger:   logger,
	}, nil
}

func (b *taskWakeBridge) WakeCreator(ctx context.Context, creatorSessionID string, ev taskpkg.WakeEvent) error {
	if b == nil || b.sessions == nil {
		return errors.New("daemon: task wake bridge is not initialized")
	}
	if ctx == nil {
		return errors.New("daemon: task wake context is required")
	}
	sessionID := strings.TrimSpace(creatorSessionID)
	if sessionID == "" {
		return taskpkg.ErrSessionNotLive
	}
	event := ev.Normalize()
	if err := event.Validate(); err != nil {
		return err
	}
	events, err := b.sessions.PromptSynthetic(ctx, sessionID, session.SyntheticPromptOpts{
		Message: taskWakePromptMessage(event),
		Metadata: acp.PromptSyntheticMeta{
			TaskID:      event.TaskID,
			TaskRunID:   event.RunID,
			Reason:      string(event.Reason),
			Summary:     event.Summary,
			WakeEventID: event.WakeEventID,
		},
		SkipIfBusy:              false,
		InterruptIfAgentWaiting: false,
	})
	if err != nil {
		if errors.Is(err, session.ErrSessionNotFound) || errors.Is(err, session.ErrSessionNotActive) {
			return fmt.Errorf("%w: %s", taskpkg.ErrSessionNotLive, sessionID)
		}
		return err
	}
	b.drainWakeEvents(sessionID, event, events)
	return nil
}

func taskWakePromptMessage(ev taskpkg.WakeEvent) string {
	summary := taskpkg.RedactClaimTokens(strings.TrimSpace(ev.Summary))
	lines := []string{
		"Task wake: " + summary,
		"Task ID: " + strings.TrimSpace(ev.TaskID),
	}
	if runID := strings.TrimSpace(ev.RunID); runID != "" {
		lines = append(lines, "Run ID: "+runID)
	}
	lines = append(lines,
		"Reason: "+string(ev.Reason.Normalize()),
		"Wake event ID: "+strings.TrimSpace(ev.WakeEventID),
	)
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func (b *taskWakeBridge) drainWakeEvents(sessionID string, ev taskpkg.WakeEvent, events <-chan acp.AgentEvent) {
	if b == nil {
		return
	}
	if events == nil {
		return
	}
	drainCtx := context.Background()
	if b != nil && b.drainCtx != nil {
		drainCtx = b.drainCtx
	}
	b.wg.Go(func() {
		for {
			select {
			case <-drainCtx.Done():
				return
			case event, ok := <-events:
				if !ok {
					return
				}
				if event.Type == acp.EventTypeError && b != nil && b.logger != nil {
					b.logger.Warn(
						"task.wake.agent_error",
						"session_id", strings.TrimSpace(sessionID),
						"task_id", strings.TrimSpace(ev.TaskID),
						"run_id", strings.TrimSpace(ev.RunID),
						"wake_event_id", strings.TrimSpace(ev.WakeEventID),
						"reason", ev.Reason.Normalize(),
					)
				}
			}
		}
	})
}

func (b *taskWakeBridge) shutdown(ctx context.Context) error {
	if b == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("daemon: task wake bridge shutdown context is required")
	}
	if b.cancel != nil {
		b.cancel()
	}
	done := make(chan struct{})
	go func() {
		b.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("daemon: shutdown task wake bridge: %w", ctx.Err())
	}
}
