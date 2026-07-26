package acp

import (
	"context"
	"errors"

	"strings"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"

	"github.com/compozy/agh/internal/store"
)

func (d *Driver) runPrompt(ctx context.Context, proc *AgentProcess, active *activePromptState, req PromptRequest) {
	defer func() {
		if active != nil && active.cancel != nil {
			active.cancel()
		}
		proc.endPrompt(active)
	}()

	stopReporter := startPromptActivityReporter(ctx, req)
	defer stopReporter()

	stopCancellationNotifier := d.startPromptCancellationNotifier(ctx, proc, active)
	defer stopCancellationNotifier()

	promptRequest, err := buildWirePromptRequest(proc, req)
	if err != nil {
		emitPromptBuildError(proc, req, err)
		return
	}
	response, err := acpsdk.SendRequest[wirePromptResponse](
		proc.conn,
		ctx,
		acpsdk.AgentMethodSessionPrompt,
		promptRequest,
	)

	if err != nil {
		if proc.stopWasRequested() && shouldSuppressPromptErrorOnStop(err) {
			return
		}
		failure, _ := FailureFromError(err, store.FailurePrompt)
		event := AgentEvent{
			Type:      EventTypeError,
			SessionID: proc.SessionID,
			TurnID:    req.TurnID,
			Timestamp: timeNowUTC(),
			Error:     firstNonEmptyFailureText(failureSummary(failure), err.Error()),
			Failure:   failure,
			Raw:       requestErrorRaw(err),
		}
		proc.emitPromptEvent(event)
		return
	}

	usage := proc.mergePromptUsage(tokenUsageFromPromptResponse(req.TurnID, response.Usage))
	doneEvent := AgentEvent{
		Type:       EventTypeDone,
		SessionID:  proc.SessionID,
		TurnID:     req.TurnID,
		Timestamp:  timeNowUTC(),
		StopReason: string(response.StopReason), PromptStopReason: PromptStopReason(response.StopReason),
	}
	if !usage.IsZero() {
		doneEvent.Usage = &usage
	}
	d.waitForPromptQuiescence(active)
	proc.emitPromptEvent(doneEvent)
}

func (d *Driver) startPromptCancellationNotifier(
	ctx context.Context,
	proc *AgentProcess,
	active *activePromptState,
) func() {
	if ctx.Err() != nil {
		d.sendPromptCancellationNotification(ctx, proc, active)
		return func() {}
	}

	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			d.sendPromptCancellationNotification(ctx, proc, active)
		case <-done:
		}
	}()
	return func() {
		close(done)
	}
}

func (d *Driver) sendPromptCancellationNotification(
	ctx context.Context,
	proc *AgentProcess,
	active *activePromptState,
) {
	if strings.TrimSpace(proc.SessionID) == "" {
		return
	}
	notifyCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
	defer cancel()
	err := proc.conn.SendNotification(
		notifyCtx,
		acpsdk.AgentMethodSessionCancel,
		acpsdk.CancelNotification{SessionId: acpsdk.SessionId(proc.SessionID)},
	)
	if err != nil && !errors.Is(err, context.Canceled) {
		d.logger.WarnContext(
			notifyCtx,
			"acp: send session cancel notification",
			"session_id",
			proc.SessionID,
			"turn_id",
			active.turnID,
			"error",
			err,
		)
	}
}

func buildWirePromptRequest(proc *AgentProcess, req PromptRequest) (acpsdk.PromptRequest, error) {
	promptText, includedSystemPrompt, promptDelivery := proc.nextPromptText(req.Message)
	promptRequest := acpsdk.PromptRequest{
		SessionId: acpsdk.SessionId(proc.SessionID),
		Prompt:    []acpsdk.ContentBlock{textBlockWithPromptCacheControl(promptText, proc.promptCacheControl)},
	}
	meta := req.Meta.Normalize()
	if includedSystemPrompt {
		if promptDelivery == "" {
			promptDelivery = SystemPromptDeliveryFirstTurnPrefix
		}
		meta.System = &PromptSystemMeta{
			PromptDelivery: string(promptDelivery),
		}
	}
	if !meta.IsZero() {
		if err := meta.Validate(); err != nil {
			return acpsdk.PromptRequest{}, err
		}
		metaMap, err := meta.ToMap()
		if err != nil {
			return acpsdk.PromptRequest{}, err
		}
		promptRequest.Meta = metaMap
	}
	return promptRequest, nil
}

func emitPromptBuildError(proc *AgentProcess, req PromptRequest, err error) {
	proc.emitPromptEvent(AgentEvent{
		Type:      EventTypeError,
		SessionID: proc.SessionID,
		TurnID:    req.TurnID,
		Timestamp: timeNowUTC(),
		Error:     err.Error(),
	})
}

func shouldSuppressPromptErrorOnStop(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if reqErr, ok := errors.AsType[*acpsdk.RequestError](err); ok {
		text := strings.ToLower(strings.TrimSpace(requestErrorDiagnosticText(reqErr)))
		if strings.Contains(text, "context canceled") ||
			strings.Contains(text, "context deadline exceeded") ||
			strings.Contains(text, "peer disconnected before response") {
			return true
		}
	}
	failure, ok := FailureFromError(err, store.FailurePrompt)
	return ok && failure != nil && failure.Kind == store.FailureCanceled
}

func startPromptActivityReporter(ctx context.Context, req PromptRequest) func() {
	if req.ActivityReporter == nil {
		return func() {}
	}
	interval := req.ActivityHeartbeatInterval
	if interval <= 0 {
		interval = 30 * time.Second
	}

	done := make(chan struct{})
	report := func(ts time.Time) {
		req.ActivityReporter(PromptActivityReport{
			Timestamp: ts,
			Kind:      clientAgentWaitingKey,
			Detail:    "waiting for session/prompt response",
		})
	}
	report(timeNowUTC())

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			case ts := <-ticker.C:
				report(ts.UTC())
			}
		}
	}()

	return func() {
		close(done)
	}
}
