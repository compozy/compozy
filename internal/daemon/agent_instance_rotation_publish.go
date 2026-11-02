package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

var _ agentinstance.MainSessionRotator = (*instanceMainRotator)(nil)

type instanceRotationPublisher interface {
	PublishRotation(context.Context, agentinstance.RotationPublication) error
}

type instanceRotationGuard interface {
	PublishInstanceRotation(context.Context, string, func() error) error
}

type instanceMonitorRetirer interface {
	RetireSession(string, func() error) error
}

func (r *instanceMainRotator) Rotate(
	ctx context.Context,
	instance agentinstance.Instance,
	decision agentinstance.RotationDecision,
) error {
	publisher, ok := r.store.(instanceRotationPublisher)
	guard, guardOK := r.sessions.(instanceRotationGuard)
	monitors, monitorOK := r.monitors.(instanceMonitorRetirer)
	if !ok || !guardOK || !monitorOK {
		return errors.New("instance rotation publication unavailable")
	}
	events, err := r.sessions.Events(
		ctx,
		instance.MainSessionID,
		store.EventQuery{Limit: 200, Types: []string{acp.EventTypeUserMessage, acp.EventTypeAgentMessage}},
	)
	if err != nil {
		return err
	}
	var parts []string
	remaining := 16000
	for i := len(events) - 1; i >= 0 && remaining > 0; i-- {
		event, err := transcript.UnmarshalAgentEvent(events[i].Content)
		if err != nil {
			return err
		}
		text := []rune(event.Text)
		if len(text) > remaining {
			text = text[len(text)-remaining:]
		}
		parts = append(parts, event.Type+": "+string(text))
		remaining -= len(text)
	}
	for left, right := 0, len(parts)-1; left < right; left, right = left+1, right-1 {
		parts[left], parts[right] = parts[right], parts[left]
	}
	candidate, err := r.starter.StartSuccessor(ctx, instance, strings.Join(parts, "\n"))
	if err != nil {
		return r.rotationFailed(ctx, instance, candidate, "successor_start_failed", err)
	}
	request := decision.Publication
	request.SuccessorID = candidate
	err = monitors.RetireSession(instance.MainSessionID, func() error {
		return guard.PublishInstanceRotation(
			ctx,
			instance.MainSessionID,
			func() error { return publisher.PublishRotation(ctx, request) },
		)
	})
	if err != nil {
		return r.rotationFailed(ctx, instance, candidate, "publication_failed", err)
	}
	instance.MainSessionID = candidate
	if r.changed != nil {
		r.changed(instance)
	}
	return r.Cleanup(ctx, instance)
}

func (r *instanceMainRotator) rotationFailed(
	ctx context.Context,
	instance agentinstance.Instance,
	candidate, reason string,
	cause error,
) error {
	r.deferRotationRetry(instance.MainSessionID)
	cleanupCtx := context.WithoutCancel(ctx)
	if candidate != "" {
		cause = errors.Join(cause, r.sessions.Stop(cleanupCtx, candidate))
	}
	payload, err := json.Marshal(map[string]string{
		loopActionRuntimeReasonKey: reason,
		"since":                    r.now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return errors.Join(cause, err)
	}
	_, err = r.reports.AppendReport(
		cleanupCtx,
		instance.ID,
		agentinstance.ReportInput{
			SourceKey: "rotation:failed:" + instance.MainSessionID,
			Kind:      "rotation_failed",
			Payload:   payload,
		},
	)
	return errors.Join(cause, err)
}
