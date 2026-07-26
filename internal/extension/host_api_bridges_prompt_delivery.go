package extensionpkg

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/acp"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
)

const (
	hostAPIBusyRetryAttempts         = 3
	hostAPIBridgePromptRetryInterval = 10 * time.Millisecond
	hostAPIBridgePromptRetryWindow   = 5 * time.Second
)

func (h *HostAPIHandler) submitBridgePrompt(
	ctx context.Context,
	instance bridgepkg.BridgeInstance,
	routingKey bridgepkg.RoutingKey,
	sessionID string,
	envelope bridgepkg.InboundMessageEnvelope,
	message string,
) (hostAPIPromptSubmission, error) {
	if h.sessions == nil {
		return hostAPIPromptSubmission{}, errors.New("extension: session manager is not configured")
	}

	lastSequence, err := h.latestSessionSequence(ctx, sessionID)
	if err != nil {
		return hostAPIPromptSubmission{}, err
	}

	promptCtx := context.WithoutCancel(ctx)
	preparedSubmission := hostAPIPromptSubmission{}
	eventsCh, prepared, err := h.promptBridgeSession(
		promptCtx,
		sessionID,
		message,
		bridgePromptNetworkMeta(envelope),
		func(deliveryCtx context.Context, delivery session.PromptDelivery) error {
			submission, err := h.preparedPromptSubmission(deliveryCtx, delivery)
			if err != nil {
				return err
			}
			if err := h.registerPromptDelivery(
				deliveryCtx,
				instance,
				routingKey,
				delivery.SessionID,
				submission,
			); err != nil {
				return err
			}
			preparedSubmission = submission
			return nil
		},
	)
	if err != nil {
		return hostAPIPromptSubmission{}, err
	}
	go func() {
		drainAgentEvents(eventsCh)
	}()
	if prepared {
		if !preparedSubmission.DeliveryRegistered {
			return hostAPIPromptSubmission{}, errors.New("extension: prompt delivery preparation did not register")
		}
		return preparedSubmission, nil
	}

	events, err := h.sessions.Events(promptCtx, sessionID, store.EventQuery{
		AfterSequence: lastSequence,
	})
	if err != nil {
		return hostAPIPromptSubmission{}, err
	}

	return promptSubmissionFromStoredEvents(events)
}

func (h *HostAPIHandler) retryBusyBridgePrompt(
	ctx context.Context,
	sessionID string,
	submit func() (<-chan acp.AgentEvent, error),
) (<-chan acp.AgentEvent, error) {
	if submit == nil {
		return nil, errors.New("extension: bridge prompt submitter is required")
	}

	var lastErr error
	for attempt := range hostAPIBusyRetryAttempts {
		eventsCh, err := submit()
		if !errors.Is(err, session.ErrPromptInProgress) {
			return eventsCh, err
		}
		lastErr = err
		if attempt == hostAPIBusyRetryAttempts-1 {
			break
		}

		waited, waitErr := h.waitForBridgePromptAvailability(ctx, sessionID)
		if waitErr != nil {
			return nil, waitErr
		}
		if !waited {
			break
		}
	}

	return nil, lastErr
}

func (h *HostAPIHandler) waitForBridgePromptAvailability(ctx context.Context, sessionID string) (bool, error) {
	promptingSessions, ok := h.sessions.(hostAPIPromptingSessionManager)
	if !ok {
		return false, nil
	}

	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return false, errors.New("extension: bridge prompt session id is required")
	}

	deadline := time.Now().Add(hostAPIBridgePromptRetryWindow)
	if ctxDeadline, ok := ctx.Deadline(); ok {
		deadline = ctxDeadline
	}
	for {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		if !promptingSessions.IsPrompting(sessionID) {
			return true, nil
		}
		if time.Now().After(deadline) {
			return false, nil
		}

		timer := time.NewTimer(hostAPIBridgePromptRetryInterval)
		select {
		case <-timer.C:
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return false, ctx.Err()
		}
	}
}

func bridgePromptNetworkMeta(envelope bridgepkg.InboundMessageEnvelope) acp.PromptNetworkMeta {
	family := envelope.EventFamily.Normalize()
	if family == "" {
		family = bridgepkg.InboundEventFamilyMessage
	}
	messageID := strings.TrimSpace(envelope.PlatformMessageID)
	if family == bridgepkg.InboundEventFamilyEdit && envelope.Edit != nil {
		messageID = strings.TrimSpace(envelope.Edit.MessageID)
	}

	meta := acp.PromptNetworkMeta{
		MessageID: messageID,
		Kind:      string(family),
		From:      strings.TrimSpace(envelope.PeerID),
	}
	if ref, ok, err := envelope.NetworkConversationRef(); err == nil && ok {
		meta.Channel = ref.Channel
		meta.Surface = string(ref.Surface)
		meta.ThreadID = ref.ThreadID
		meta.DirectID = ref.DirectID
		meta.WorkID = ref.WorkID
		meta.ReplyTo = ref.ReplyTo
		meta.TraceID = ref.TraceID
		meta.CausationID = ref.CausationID
	}
	return meta.Normalize()
}

func (h *HostAPIHandler) registerPromptDelivery(
	ctx context.Context,
	instance bridgepkg.BridgeInstance,
	routingKey bridgepkg.RoutingKey,
	sessionID string,
	submission hostAPIPromptSubmission,
) error {
	if h.deliveryBroker == nil {
		return nil
	}

	target, err := bridgepkg.BuildDeliveryTarget(instance, bridgepkg.ResolveDeliveryTargetRequest{
		BridgeInstanceID: routingKey.BridgeInstanceID,
		PeerID:           routingKey.PeerID,
		ThreadID:         routingKey.ThreadID,
		GroupID:          routingKey.GroupID,
		Mode:             bridgepkg.DeliveryModeReply,
	})
	if err != nil {
		return fmt.Errorf("extension: resolve prompt delivery target: %w", err)
	}

	if _, err := h.deliveryBroker.RegisterPromptDelivery(ctx, bridgepkg.PromptDeliveryRegistration{
		SessionID:      strings.TrimSpace(sessionID),
		TurnID:         strings.TrimSpace(submission.TurnID),
		ExtensionName:  strings.TrimSpace(instance.ExtensionName),
		RoutingKey:     routingKey,
		DeliveryTarget: target,
		Progress:       bridgepkg.ResolveProgressConfig(&instance, instance.Platform),
		SeedEvents:     submission.SeedEvents,
	}); err != nil {
		return fmt.Errorf("extension: register prompt delivery: %w", err)
	}
	return nil
}

func (h *HostAPIHandler) registerPromptDeliveryAfterSubmission(
	ctx context.Context,
	instance bridgepkg.BridgeInstance,
	routingKey bridgepkg.RoutingKey,
	sessionID string,
	submission hostAPIPromptSubmission,
) error {
	if submission.DeliveryRegistered {
		return nil
	}
	if err := h.registerPromptDelivery(ctx, instance, routingKey, sessionID, submission); err != nil {
		return err
	}
	if err := h.replayPromptDeliveryEvents(ctx, sessionID, submission.TurnID); err != nil {
		return fmt.Errorf("extension: replay prompt delivery events: %w", err)
	}
	return nil
}

func (h *HostAPIHandler) replayPromptDeliveryEvents(ctx context.Context, sessionID string, turnID string) error {
	if h == nil || h.deliveryBroker == nil || h.sessions == nil {
		return nil
	}

	sessionID = strings.TrimSpace(sessionID)
	turnID = strings.TrimSpace(turnID)
	if sessionID == "" || turnID == "" {
		return nil
	}

	const (
		replayPollInterval = 5 * time.Millisecond
		replayPollWindow   = 30 * time.Millisecond
	)

	deadline := h.now().Add(replayPollWindow)
	for {
		events, err := h.sessions.Events(ctx, sessionID, store.EventQuery{TurnID: turnID})
		if err != nil {
			return err
		}

		hasTerminal := false
		for _, storedEvent := range events {
			projected, err := promptProjectionEventFromStoredEvent(storedEvent)
			if err != nil {
				return err
			}
			if err := h.deliveryBroker.ProjectEvent(ctx, sessionID, projected); err != nil {
				return err
			}
			if projected.Type == acp.EventTypeDone || projected.Type == acp.EventTypeError {
				hasTerminal = true
			}
		}

		if hasTerminal || !h.now().Before(deadline) {
			return nil
		}

		timer := time.NewTimer(replayPollInterval)
		select {
		case <-timer.C:
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return ctx.Err()
		}
	}
}
