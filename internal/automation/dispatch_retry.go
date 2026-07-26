package automation

import (
	"context"
	"errors"
	"fmt"

	"strings"

	"text/template"
	"time"

	"github.com/compozy/agh/internal/acp"

	"github.com/compozy/agh/internal/session"
	taskpkg "github.com/compozy/agh/internal/task"
)

func renderTriggerPrompt(raw string, envelope *ActivationEnvelope) (string, error) {
	if envelope == nil {
		return "", errors.New("automation: activation envelope is required")
	}
	if !strings.Contains(raw, "{{") && !strings.Contains(raw, "}}") {
		return strings.TrimSpace(raw), nil
	}

	tmpl, err := ParseTriggerPromptTemplate(raw)
	if err != nil {
		return "", err
	}

	var builder strings.Builder
	if execErr := executePromptTemplate(&builder, tmpl, envelope); execErr != nil {
		return "", execErr
	}
	return strings.TrimSpace(builder.String()), nil
}

func executePromptTemplate(builder *strings.Builder, tmpl *template.Template, envelope *ActivationEnvelope) error {
	if builder == nil {
		return errors.New("automation: prompt builder is required")
	}
	if tmpl == nil {
		return errors.New("automation: prompt template is required")
	}
	if envelope == nil {
		return errors.New("automation: activation envelope is required")
	}

	if err := tmpl.Execute(builder, envelope); err != nil {
		return fmt.Errorf("automation: execute trigger prompt template: %w", err)
	}
	return nil
}

func directTaskActorContext(job *Job, runID string) (taskpkg.ActorContext, error) {
	if job == nil {
		return taskpkg.ActorContext{}, errors.New("automation: task-backed dispatch job is required")
	}
	return taskpkg.DeriveAutomationActorContext(strings.TrimSpace(job.ID), automationTaskOriginRef(runID))
}

func automationSessionTaskActorContext(
	sessionID string,
	workspaceID string,
	run *Run,
) (taskpkg.ActorContext, error) {
	if run == nil {
		return taskpkg.ActorContext{}, errors.New("automation: run is required for session task actor context")
	}
	return taskpkg.DeriveAutomationLinkedAgentSessionActorContext(
		strings.TrimSpace(sessionID),
		strings.TrimSpace(workspaceID),
		automationTaskOriginRef(run.ID),
	)
}

func automationTaskOriginRef(runID string) string {
	return "run:" + strings.TrimSpace(runID)
}

func automationTaskRunIdempotencyKey(runID string) string {
	return "automation-run:" + strings.TrimSpace(runID)
}

func classifyDispatchError(err error) RunStatus {
	if err == nil {
		return RunCompleted
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return RunCancelled
	}
	return RunFailed
}

func dispatchStopCause(status RunStatus, runErr error) (session.StopCause, string) {
	switch status {
	case RunCompleted:
		return session.CauseCompleted, ""
	case RunCancelled:
		return session.CauseUserRequested, strings.TrimSpace(errorText(runErr))
	default:
		return session.CauseFailed, strings.TrimSpace(errorText(runErr))
	}
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func shouldRetry(cfg RetryConfig, run *Run, attempt int, dispatchErr error) bool {
	if dispatchErr == nil || run == nil || run.Status != RunFailed {
		return false
	}
	if cfg.Strategy != RetryStrategyBackoff {
		return false
	}
	return attempt <= cfg.MaxRetries
}

func retryDelay(cfg RetryConfig, attempt int) (time.Duration, error) {
	if cfg.Strategy != RetryStrategyBackoff {
		return 0, nil
	}

	baseDelay, err := time.ParseDuration(strings.TrimSpace(cfg.BaseDelay))
	if err != nil {
		return 0, fmt.Errorf("automation: parse retry base delay: %w", err)
	}
	if attempt <= 1 {
		return baseDelay, nil
	}
	return baseDelay * time.Duration(1<<(attempt-1)), nil
}

func collectPromptError(ctx context.Context, events <-chan acp.AgentEvent) error {
	if events == nil {
		return errors.New("automation: prompt event stream is required")
	}

	var errs []error
	for {
		select {
		case <-ctx.Done():
			if len(errs) > 0 {
				errs = append(errs, ctx.Err())
				return errors.Join(errs...)
			}
			return ctx.Err()
		case event, ok := <-events:
			if !ok {
				if len(errs) > 0 {
					return errors.Join(errs...)
				}
				if err := ctx.Err(); err != nil {
					return err
				}
				return nil
			}
			if trimmed := strings.TrimSpace(event.Error); trimmed != "" {
				errs = append(errs, errors.New(trimmed))
			}
		}
	}
}

func (d *Dispatcher) logHookDispatchError(message string, err error, attrs ...any) {
	if d == nil || err == nil || d.logger == nil {
		return
	}
	fields := append([]any{}, attrs...)
	fields = append(fields, "error", err)
	d.logger.Warn(message, fields...)
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
