package daemon

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
)

const taskRolePromptCleanupTimeout = 5 * time.Second

type taskRoleSessionStart func(
	context.Context,
	taskRoleActivation,
) (*session.Info, error)

type taskRoleActivationResult struct {
	info    *session.Info
	created bool
}

func (r *taskRoleRuntime) activateRoleSession(
	ctx context.Context,
	activation taskRoleActivation,
	reason string,
	start taskRoleSessionStart,
) (taskRoleActivationResult, error) {
	existing, err := r.activeRoleSession(ctx, activation)
	if err != nil {
		return taskRoleActivationResult{}, err
	}
	info := existing
	created := false
	if info == nil {
		if start == nil {
			return taskRoleActivationResult{}, errors.New("daemon: task role session starter is required")
		}
		info, err = start(ctx, activation)
		if err != nil {
			return taskRoleActivationResult{}, err
		}
		created = true
	}

	if err := r.promptTaskRoleSession(ctx, info, activation, reason); err != nil {
		if !created {
			return taskRoleActivationResult{}, err
		}
		cleanupErr := r.cleanupCreatedRoleSession(ctx, info, activation.RunID)
		return taskRoleActivationResult{}, errors.Join(err, cleanupErr)
	}
	return taskRoleActivationResult{info: info, created: created}, nil
}

func (r *taskRoleRuntime) promptTaskRoleSession(
	ctx context.Context,
	info *session.Info,
	activation taskRoleActivation,
	reason string,
) error {
	if ctx == nil {
		return errors.New("daemon: task role prompt context is required")
	}
	if info == nil {
		return errors.New("daemon: task role prompt requires session info")
	}
	sessionID := strings.TrimSpace(info.ID)
	if sessionID == "" {
		return errors.New("daemon: task role prompt requires session id")
	}
	key := taskRolePromptInFlightKey(sessionID, activation.RunID)
	if _, ok := r.promptInFlight[key]; ok {
		return nil
	}
	turnID := taskRolePromptTurnID(sessionID, activation.RunID)
	completed, err := r.taskRolePromptCompleted(ctx, sessionID, turnID)
	if err != nil {
		return err
	}
	if completed {
		return nil
	}

	events, err := r.sessions.PromptSynthetic(ctx, sessionID, session.SyntheticPromptOpts{
		Message: taskRoleActivationMessage(activation),
		Metadata: acp.PromptSyntheticMeta{
			TaskID:    strings.TrimSpace(activation.TaskID),
			TaskRunID: strings.TrimSpace(activation.RunID),
			Reason:    strings.TrimSpace(reason),
			Summary:   taskRoleActivationSummary(activation),
		},
		TurnID: turnID,
	})
	if err != nil {
		return fmt.Errorf("daemon: prompt task role session %q: %w", sessionID, err)
	}
	if events == nil {
		return fmt.Errorf("daemon: prompt task role session %q returned nil event stream", sessionID)
	}
	r.promptInFlight[key] = struct{}{}
	r.drainTaskRolePromptEvents(key, sessionID, activation, events)
	return nil
}

func (r *taskRoleRuntime) taskRolePromptCompleted(
	ctx context.Context,
	sessionID string,
	turnID string,
) (bool, error) {
	events, err := r.sessions.Events(ctx, sessionID, store.EventQuery{TurnID: turnID})
	if err != nil {
		return false, fmt.Errorf("daemon: read task role activation turn %q: %w", turnID, err)
	}
	for _, event := range events {
		if event.Type == acp.EventTypeDone || event.Type == acp.EventTypeError {
			return true, nil
		}
	}
	return false, nil
}

func (r *taskRoleRuntime) cleanupCreatedRoleSession(
	ctx context.Context,
	info *session.Info,
	runID string,
) error {
	if r == nil || info == nil {
		return nil
	}
	sessionID := strings.TrimSpace(info.ID)
	if sessionID == "" {
		return nil
	}
	detail := fmt.Sprintf(
		"initial task-role prompt dispatch failed for run %q",
		strings.TrimSpace(runID),
	)
	stopCtx, cancel := detachedDaemonOperationContext(ctx, taskRolePromptCleanupTimeout)
	defer cancel()
	if err := r.sessions.StopWithCause(stopCtx, sessionID, session.CauseFailed, detail); err != nil {
		return fmt.Errorf("daemon: stop unprompted task role session %q: %w", sessionID, err)
	}
	return nil
}

func (r *taskRoleRuntime) drainTaskRolePromptEvents(
	key string,
	sessionID string,
	activation taskRoleActivation,
	events <-chan acp.AgentEvent,
) {
	if r == nil || events == nil {
		return
	}
	r.wg.Go(func() {
		defer r.finishTaskRolePrompt(key)
		for {
			select {
			case <-r.drainCtx.Done():
				return
			case event, ok := <-events:
				if !ok {
					return
				}
				if event.Type == acp.EventTypeError {
					r.logTaskRolePromptError(sessionID, activation)
				}
			}
		}
	})
}

func (r *taskRoleRuntime) finishTaskRolePrompt(key string) {
	if r == nil || key == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.promptInFlight, key)
}

func (r *taskRoleRuntime) logTaskRolePromptError(sessionID string, activation taskRoleActivation) {
	if r == nil || r.logger == nil {
		return
	}
	r.logger.Warn(
		"daemon: task role prompt returned agent error",
		"session_id", strings.TrimSpace(sessionID),
		taskRoleRuntimeTaskIDKey, strings.TrimSpace(activation.TaskID),
		daemonLogRunIDKey, strings.TrimSpace(activation.RunID),
	)
}

func (r *taskRoleRuntime) shutdown(ctx context.Context) error {
	if r == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("daemon: task role runtime shutdown context is required")
	}
	r.lifecycleMu.Lock()
	r.stopping = true
	r.lifecycleMu.Unlock()
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
		return fmt.Errorf("daemon: shutdown task role runtime: %w", ctx.Err())
	}
}

func taskRolePromptInFlightKey(sessionID string, runID string) string {
	return strings.TrimSpace(sessionID) + "\x00" + strings.TrimSpace(runID)
}

func taskRolePromptTurnID(sessionID string, runID string) string {
	digest := sha256.Sum256([]byte(taskRolePromptInFlightKey(sessionID, runID)))
	return fmt.Sprintf("task-role-activation-%x", digest[:16])
}

func taskRoleActivationMessage(activation taskRoleActivation) string {
	return fmt.Sprintf(
		"A queued task run is ready for this task-role session.\n\n"+
			"Task: %s\nRun: %s\n\n"+
			"Follow the task claim instructions in the system context before doing any work. "+
			"This notification does not assign ownership.",
		firstNonEmpty(activation.Title, activation.TaskID),
		strings.TrimSpace(activation.RunID),
	)
}

func taskRoleActivationSummary(activation taskRoleActivation) string {
	return fmt.Sprintf(
		"Pending task run %s for task %s",
		strings.TrimSpace(activation.RunID),
		firstNonEmpty(activation.Title, activation.TaskID),
	)
}
