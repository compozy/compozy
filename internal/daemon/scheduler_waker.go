package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/compozy/agh/internal/acp"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/heartbeat"
	schedulerpkg "github.com/compozy/agh/internal/scheduler"
	"github.com/compozy/agh/internal/session"
)

type schedulerPromptState interface {
	IsPrompting(sessionID string) bool
}

type schedulerSyntheticPrompter interface {
	PromptSynthetic(ctx context.Context, id string, opts session.SyntheticPromptOpts) (<-chan acp.AgentEvent, error)
}

type schedulerSessionWaker struct {
	ctx           context.Context
	cancel        context.CancelFunc
	sessions      SessionManager
	heartbeatWake heartbeat.WakeService
	logger        *slog.Logger
	wg            sync.WaitGroup
}

func newSchedulerSessionWaker(
	ctx context.Context,
	sessions SessionManager,
	logger *slog.Logger,
) *schedulerSessionWaker {
	if ctx == nil {
		ctx = context.Background()
	}
	if logger == nil {
		logger = slog.Default()
	}
	wakeCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	return &schedulerSessionWaker{
		ctx:      wakeCtx,
		cancel:   cancel,
		sessions: sessions,
		logger:   logger,
	}
}

func (w *schedulerSessionWaker) Wake(ctx context.Context, target *schedulerpkg.WakeTarget) error {
	if w == nil || w.sessions == nil {
		return errors.New("daemon: scheduler waker requires session manager")
	}
	return w.wakeOne(ctx, target)
}

func (w *schedulerSessionWaker) WakeMany(
	ctx context.Context,
	targets []schedulerpkg.WakeTarget,
) []error {
	errs := make([]error, len(targets))
	if w == nil || w.sessions == nil {
		err := errors.New("daemon: scheduler waker requires session manager")
		for idx := range errs {
			errs[idx] = err
		}
		return errs
	}
	if w.heartbeatWake == nil {
		for idx := range targets {
			errs[idx] = w.wakeOne(ctx, &targets[idx])
		}
		return errs
	}

	requests, indexes := w.prepareHeartbeatWakeBatch(ctx, targets, errs)
	if len(requests) == 0 {
		return errs
	}
	decisions, err := w.heartbeatWake.WakeMany(ctx, requests)
	for decisionIdx, decision := range decisions {
		if decisionIdx >= len(indexes) {
			break
		}
		idx := indexes[decisionIdx]
		sessionID := strings.TrimSpace(targets[idx].Session.ID)
		errs[idx] = w.handleHeartbeatDecision(ctx, &targets[idx], sessionID, decision)
	}
	missingStart := min(len(decisions), len(indexes))
	if err == nil && missingStart < len(indexes) {
		err = errors.New("daemon: heartbeat wake batch returned fewer decisions than requests")
	}
	for _, idx := range indexes[missingStart:] {
		errs[idx] = err
	}
	return errs
}

func (w *schedulerSessionWaker) wakeOne(ctx context.Context, target *schedulerpkg.WakeTarget) error {
	sessionID, err := schedulerWakeSessionID(target)
	if err != nil {
		return err
	}
	if req, ok := schedulerHeartbeatWakeRequest(target, sessionID); ok && w.heartbeatWake != nil {
		decision, wakeErr := w.heartbeatWake.Wake(ctx, req)
		if wakeErr != nil {
			return wakeErr
		}
		return w.handleHeartbeatDecision(ctx, target, sessionID, decision)
	}
	return w.wakePendingTaskRun(ctx, target, sessionID)
}

func (w *schedulerSessionWaker) prepareHeartbeatWakeBatch(
	ctx context.Context,
	targets []schedulerpkg.WakeTarget,
	errs []error,
) ([]heartbeat.WakeRequest, []int) {
	requests := make([]heartbeat.WakeRequest, 0, len(targets))
	indexes := make([]int, 0, len(targets))
	for idx := range targets {
		target := &targets[idx]
		sessionID, err := schedulerWakeSessionID(target)
		if err != nil {
			errs[idx] = err
			continue
		}
		req, ok := schedulerHeartbeatWakeRequest(target, sessionID)
		if !ok {
			errs[idx] = w.wakePendingTaskRun(ctx, target, sessionID)
			continue
		}
		requests = append(requests, req)
		indexes = append(indexes, idx)
	}
	return requests, indexes
}

func (w *schedulerSessionWaker) handleHeartbeatDecision(
	ctx context.Context,
	target *schedulerpkg.WakeTarget,
	sessionID string,
	decision heartbeat.WakeDecision,
) error {
	switch decision.Result {
	case heartbeat.WakeResultSent:
		return nil
	case heartbeat.WakeResultSkipped:
		if decision.Reason == heartbeat.WakeReasonHeartbeatNoPolicy {
			return w.wakePendingTaskRun(ctx, target, sessionID)
		}
		return nil
	case heartbeat.WakeResultCoalesced, heartbeat.WakeResultRateLimited:
		return nil
	case heartbeat.WakeResultFailed:
		return fmt.Errorf("daemon: heartbeat wake failed: %s", decision.Reason)
	default:
		return fmt.Errorf("daemon: unexpected heartbeat wake result %q", decision.Result)
	}
}

func (w *schedulerSessionWaker) wakePendingTaskRun(
	ctx context.Context,
	target *schedulerpkg.WakeTarget,
	sessionID string,
) error {
	message := schedulerWakeMessage(target)
	if synthetic, ok := w.sessions.(schedulerSyntheticPrompter); ok {
		metadata := acp.PromptSyntheticMeta{
			TaskID:         strings.TrimSpace(target.Work.Task.ID),
			TaskRunID:      strings.TrimSpace(target.Work.Run.ID),
			ClaimTokenHash: strings.TrimSpace(target.Work.Run.ClaimTokenHash),
			Reason:         strings.TrimSpace(target.Reason),
			Summary:        schedulerWakeSummary(target),
		}
		if strings.TrimSpace(target.Session.Type) == string(session.SessionTypeCoordinator) {
			metadata.CoordinatorSessionID = strings.TrimSpace(sessionID)
		}
		events, err := synthetic.PromptSynthetic(ctx, sessionID, session.SyntheticPromptOpts{
			Message:  message,
			Metadata: metadata,
		})
		if err != nil {
			return err
		}
		w.drainEvents(sessionID, target.Work.Run.ID, events)
		return nil
	}

	events, err := w.sessions.Prompt(ctx, sessionID, message)
	if err != nil {
		return err
	}
	w.drainEvents(sessionID, target.Work.Run.ID, events)
	return nil
}

func schedulerWakeSessionID(target *schedulerpkg.WakeTarget) (string, error) {
	if target == nil {
		return "", errors.New("daemon: scheduler wake target is required")
	}
	sessionID := strings.TrimSpace(target.Session.ID)
	if sessionID == "" {
		return "", errors.New("daemon: scheduler wake session id is required")
	}
	return sessionID, nil
}

func schedulerHeartbeatWakeRequest(
	target *schedulerpkg.WakeTarget,
	sessionID string,
) (heartbeat.WakeRequest, bool) {
	if target == nil {
		return heartbeat.WakeRequest{}, false
	}
	workspaceID := strings.TrimSpace(target.Session.WorkspaceID)
	agentName := strings.TrimSpace(target.Session.AgentName)
	if workspaceID == "" || agentName == "" {
		return heartbeat.WakeRequest{}, false
	}
	return heartbeat.WakeRequest{
		WorkspaceID: workspaceID,
		AgentName:   agentName,
		SessionID:   strings.TrimSpace(sessionID),
		Source:      heartbeat.WakeSourceScheduler,
	}, true
}

func (w *schedulerSessionWaker) configureHeartbeatWake(
	store any,
	sessions SessionManager,
	config aghconfig.HeartbeatConfig,
) error {
	if w == nil || !config.Enabled {
		return nil
	}
	wakeStore, ok := store.(heartbeat.WakeStore)
	if !ok {
		return nil
	}
	healthReader, ok := sessions.(heartbeat.SessionHealthReader)
	if !ok {
		return nil
	}
	service, err := heartbeat.NewManagedWakeService(wakeStore, healthReader, w, config)
	if err != nil {
		return fmt.Errorf("daemon: create scheduler heartbeat wake service: %w", err)
	}
	w.heartbeatWake = service
	return nil
}

func (w *schedulerSessionWaker) PromptHeartbeatWake(
	ctx context.Context,
	req heartbeat.SyntheticWakePromptRequest,
) (heartbeat.SyntheticWakePromptResult, error) {
	if w == nil || w.sessions == nil {
		return heartbeat.SyntheticWakePromptResult{}, errors.New(
			"daemon: scheduler heartbeat prompter requires sessions",
		)
	}
	synthetic, ok := w.sessions.(schedulerSyntheticPrompter)
	if !ok {
		return heartbeat.SyntheticWakePromptResult{}, errors.New(
			"daemon: scheduler heartbeat prompter requires synthetic prompt support",
		)
	}
	events, err := synthetic.PromptSynthetic(ctx, req.SessionID, session.SyntheticPromptOpts{
		Message: req.Message,
		TurnID:  req.TurnID,
		Metadata: acp.PromptSyntheticMeta{
			TaskID:               req.SyntheticCorrelation.TaskID,
			TaskRunID:            req.SyntheticCorrelation.TaskRunID,
			WorkflowID:           req.SyntheticCorrelation.WorkflowID,
			ClaimTokenHash:       req.SyntheticCorrelation.ClaimTokenHash,
			CoordinatorSessionID: req.SyntheticCorrelation.CoordinatorSessionID,
			Reason:               heartbeat.SyntheticReasonHeartbeatWake,
			Summary:              req.Summary,
			WakeEventID:          req.WakeEventID,
			PolicySnapshotID:     req.PolicySnapshotID,
			PolicyDigest:         req.PolicyDigest,
			ConfigDigest:         req.ConfigDigest,
		},
		SkipIfBusy: true,
	})
	if err != nil {
		if errors.Is(err, session.ErrPromptInProgress) {
			return heartbeat.SyntheticWakePromptResult{}, heartbeat.ErrSyntheticPromptBusy
		}
		return heartbeat.SyntheticWakePromptResult{}, err
	}
	w.drainEvents(req.SessionID, req.WakeEventID, events)
	return heartbeat.SyntheticWakePromptResult{SyntheticPromptID: req.TurnID}, nil
}

func (w *schedulerSessionWaker) drainEvents(sessionID string, runID string, events <-chan acp.AgentEvent) {
	if events == nil {
		return
	}
	w.wg.Go(func() {
		for {
			select {
			case <-w.ctx.Done():
				return
			case event, ok := <-events:
				if !ok {
					return
				}
				if event.Type == acp.EventTypeError && w.logger != nil {
					w.logger.Warn(
						"scheduler.wake.agent_error",
						"session_id", sessionID,
						"run_id", runID,
					)
				}
			}
		}
	})
}

func (w *schedulerSessionWaker) shutdown(ctx context.Context) error {
	if w == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("daemon: scheduler waker shutdown context is required")
	}
	if w.cancel != nil {
		w.cancel()
	}
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("daemon: shutdown scheduler waker: %w", ctx.Err())
	}
}

func schedulerWakeMessage(target *schedulerpkg.WakeTarget) string {
	if target == nil {
		return ""
	}
	taskTitle := firstSchedulerWakeValue(
		target.Work.Task.Title,
		target.Work.Task.Identifier,
		target.Work.Task.ID,
	)
	return fmt.Sprintf(
		"A task run is queued and may be claimable by this session.\n\n"+
			"Task: %s\nRun: %s\n\n"+
			"Use the existing task claim path before doing any work. "+
			"This notification does not assign ownership.",
		taskTitle,
		strings.TrimSpace(target.Work.Run.ID),
	)
}

func schedulerWakeSummary(target *schedulerpkg.WakeTarget) string {
	if target == nil {
		return ""
	}
	return fmt.Sprintf(
		"Pending task run %s for task %s",
		strings.TrimSpace(target.Work.Run.ID),
		firstSchedulerWakeValue(target.Work.Task.Title, target.Work.Task.ID),
	)
}

func firstSchedulerWakeValue(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return schedulerRuntimeTaskKey
}
