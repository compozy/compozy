package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type permissionAttentionPayload struct {
	RequestID string `json:"request_id"`
	ToolID    string `json:"tool_id"`
	Options   []struct {
		Decision string `json:"decision"`
	} `json:"options"`
	ToolCall struct {
		Title string `json:"title"`
	} `json:"tool_call"`
}

const pendingInteractionActorOperator = "operator"

func (m *Manager) applyAttentionAgentEvent(
	ctx context.Context,
	target *Session,
	event acp.AgentEvent,
) (bool, error) {
	if target == nil || event.Type != acp.EventTypePermission {
		return false, nil
	}
	requestID := strings.TrimSpace(event.RequestIDValue())
	if requestID == "" {
		return false, errors.New("session: permission event request id is required")
	}
	if strings.TrimSpace(event.Decision) == "" {
		info := target.Info()
		if pendingInteractionByProviderRequest(info, store.PendingInteractionKindPermission, requestID) != nil {
			return true, nil
		}
		payload, title := permissionInteractionPayload(event)
		_, err := m.createPendingInteraction(ctx, store.PendingInteractionCreate{
			SessionID:         target.ID,
			Kind:              store.PendingInteractionKindPermission,
			ProviderRequestID: requestID,
			TurnID:            event.TurnID,
			Title:             title,
			Payload:           payload,
			CreatedAt:         event.Timestamp,
		})
		if errors.Is(err, store.ErrPendingInteractionConflict) {
			return true, m.refreshPendingInteractions(ctx, target.ID)
		}
		return err == nil, err
	}
	interaction := pendingInteractionByProviderRequest(
		target.Info(),
		store.PendingInteractionKindPermission,
		requestID,
	)
	if interaction == nil {
		return false, nil
	}
	resolvedBy := strings.TrimSpace(event.ResolvedByValue())
	if resolvedBy == "" {
		resolvedBy = "provider"
	}
	_, err := m.transitionPendingInteraction(ctx, store.PendingInteractionTransition{
		InteractionID: interaction.InteractionID,
		Status:        store.PendingInteractionStatusResolved,
		Resolution:    event.Decision,
		ResolvedBy:    resolvedBy,
		At:            event.Timestamp,
	})
	return err == nil, err
}

func permissionInteractionPayload(event acp.AgentEvent) (store.PendingInteractionPayload, string) {
	payload := store.PendingInteractionPayload{}
	title := strings.TrimSpace(event.Title)
	if len(event.Raw) == 0 {
		return payload, title
	}
	var raw permissionAttentionPayload
	if err := json.Unmarshal(event.Raw, &raw); err != nil {
		return payload, title
	}
	if title == "" {
		title = strings.TrimSpace(raw.ToolCall.Title)
	}
	payload.ToolID = strings.TrimSpace(raw.ToolID)
	for _, option := range raw.Options {
		payload.Decisions = append(payload.Decisions, option.Decision)
	}
	return payload, title
}

func (m *Manager) applyClarifyAttentionEvent(
	ctx context.Context,
	target *Session,
	event toolspkg.ClarifyEvent,
) (bool, error) {
	requestID := strings.TrimSpace(event.Request.RequestID)
	if event.Status == toolspkg.ClarifyStatusPending {
		info := target.Info()
		if pendingInteractionByProviderRequest(info, store.PendingInteractionKindClarify, requestID) != nil {
			return true, nil
		}
		_, err := m.createPendingInteraction(ctx, store.PendingInteractionCreate{
			SessionID:         target.ID,
			Kind:              store.PendingInteractionKindClarify,
			ProviderRequestID: requestID,
			TurnID:            "clarify:" + requestID,
			Title:             event.Request.Question,
			Payload: store.PendingInteractionPayload{
				Choices: event.Request.Choices,
			},
			CreatedAt: event.At,
		})
		if errors.Is(err, store.ErrPendingInteractionConflict) {
			return true, m.refreshPendingInteractions(ctx, target.ID)
		}
		return err == nil, err
	}
	interaction := pendingInteractionByProviderRequest(
		target.Info(),
		store.PendingInteractionKindClarify,
		requestID,
	)
	if interaction == nil {
		return false, nil
	}
	status, err := clarifyInteractionStatus(event.Status)
	if err != nil {
		return false, err
	}
	transition := store.PendingInteractionTransition{
		InteractionID: interaction.InteractionID,
		Status:        status,
		At:            event.At,
	}
	if status == store.PendingInteractionStatusResolved {
		question := toolspkg.ClarifyQuestion{
			Question: event.Request.Question,
			Choices:  append([]string(nil), event.Request.Choices...),
		}
		transition.Resolution = event.Answer.Resolution(question)
		transition.ResolvedBy = event.ResolvedBy
	}
	_, err = m.transitionPendingInteraction(ctx, transition)
	return err == nil, err
}

func clarifyInteractionStatus(status toolspkg.ClarifyStatus) (string, error) {
	switch status {
	case toolspkg.ClarifyStatusResolved:
		return store.PendingInteractionStatusResolved, nil
	case toolspkg.ClarifyStatusTimedOut:
		return store.PendingInteractionStatusTimedOut, nil
	case toolspkg.ClarifyStatusCanceled:
		return store.PendingInteractionStatusCanceled, nil
	default:
		return "", fmt.Errorf("session: unsupported clarification attention status %q", status)
	}
}

func pendingInteractionByProviderRequest(info *Info, kind string, requestID string) *store.PendingInteraction {
	if info == nil {
		return nil
	}
	for index := range info.PendingInteractions {
		interaction := &info.PendingInteractions[index]
		if interaction.Kind == kind && interaction.ProviderRequestID == requestID &&
			(interaction.Status == store.PendingInteractionStatusPending ||
				interaction.Status == store.PendingInteractionStatusOrphaned) {
			cloned := *interaction
			return &cloned
		}
	}
	return nil
}
