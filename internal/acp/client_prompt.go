package acp

import (
	"context"
	"errors"

	"strings"
	"sync"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"

	"github.com/compozy/compozy/internal/store"
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

	promptRequest, err := buildWirePromptRequest(proc, req)
	if err != nil {
		emitPromptBuildError(proc, req, err)
		return
	}
	// The SDK turns context cancellation into request-scoped $/cancel_request.
	response, err := acpsdk.SendRequest[wirePromptResponse](
		proc.conn,
		ctx,
		acpsdk.AgentMethodSessionPrompt,
		promptRequest,
	)
	// Drain admitted steering before closing its owning turn's event stream.
	active.steerMu.Lock()
	active.finishing = true
	active.steerMu.Unlock()
	active.steerWG.Wait()

	if err != nil {
		if proc.stopWasRequested() && shouldSuppressPromptErrorOnStop(err) {
			return
		}
		proc.emitPromptEvent(proc.promptErrorEvent(req, err, timeNowUTC()))
		return
	}
	if _, included := promptRequest.Meta["system"]; included {
		proc.markSystemPromptSent()
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

func buildWirePromptRequest(proc *AgentProcess, req PromptRequest) (acpsdk.PromptRequest, error) {
	attachmentBlocks, err := attachmentContentBlocks(req.Attachments, proc.CapsSnapshot())
	if err != nil {
		return acpsdk.PromptRequest{}, err
	}
	promptText, includedSystemPrompt, promptDelivery := proc.nextPromptText(req.Message)
	prompt := make([]acpsdk.ContentBlock, 0, 1+len(req.Attachments))
	if promptText != "" {
		prompt = append(prompt, textBlockWithPromptCacheControl(promptText, proc.promptCacheControl))
	}
	prompt = append(prompt, attachmentBlocks...)
	promptRequest := acpsdk.PromptRequest{
		SessionId: acpsdk.SessionId(proc.SessionID),
		Prompt:    prompt,
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
	stopped := make(chan struct{})
	report := func(ts time.Time) {
		req.ActivityReporter(PromptActivityReport{
			Timestamp: ts,
			Kind:      clientAgentWaitingKey,
			Detail:    "waiting for session/prompt response",
		})
	}
	report(timeNowUTC())

	go func() {
		defer close(stopped)
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

	var stopOnce sync.Once
	return func() {
		stopOnce.Do(func() {
			close(done)
		})
		<-stopped
	}
}
