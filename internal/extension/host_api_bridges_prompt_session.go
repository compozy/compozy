package extensionpkg

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
)

func (h *HostAPIHandler) preparedPromptSubmission(
	ctx context.Context,
	delivery session.PromptDelivery,
) (hostAPIPromptSubmission, error) {
	events, err := h.sessions.Events(ctx, delivery.SessionID, store.EventQuery{TurnID: delivery.TurnID})
	if err != nil {
		return hostAPIPromptSubmission{}, fmt.Errorf("extension: load prepared prompt seed events: %w", err)
	}
	submission, err := promptSubmissionFromStoredEvents(events)
	if err != nil {
		return hostAPIPromptSubmission{}, fmt.Errorf("extension: prepare prompt delivery seed: %w", err)
	}
	if strings.TrimSpace(submission.TurnID) != strings.TrimSpace(delivery.TurnID) {
		return hostAPIPromptSubmission{}, fmt.Errorf(
			"extension: prepared prompt turn %q does not match %q",
			submission.TurnID,
			delivery.TurnID,
		)
	}
	submission.DeliveryRegistered = true
	return submission, nil
}

func (h *HostAPIHandler) promptBridgeSession(
	ctx context.Context,
	sessionID string,
	message string,
	meta acp.PromptNetworkMeta,
	prepareDelivery session.PromptDeliveryPreparer,
) (<-chan acp.AgentEvent, bool, error) {
	if promptSessions, ok := h.sessions.(hostAPIBridgePromptSessionManager); ok {
		events, err := h.retryBusyBridgePrompt(ctx, sessionID, func() (<-chan acp.AgentEvent, error) {
			return promptSessions.PromptWithOpts(ctx, sessionID, session.PromptOpts{
				Message:         message,
				TurnSource:      session.TurnSourceNetwork,
				PromptMeta:      acp.PromptMeta{Network: &meta},
				PrepareDelivery: prepareDelivery,
			})
		})
		return events, true, err
	}

	events, err := h.retryBusyBridgePrompt(ctx, sessionID, func() (<-chan acp.AgentEvent, error) {
		return h.sessions.Prompt(ctx, sessionID, message)
	})
	return events, false, err
}
