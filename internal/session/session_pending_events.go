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
		Kind  string `json:"kind"`
	} `json:"tool_call"`
}

const pendingInteractionActorOperator = "operator"

func (m *Manager) applyAttentionAgentEvent(ctx context.Context, target *Session, event acp.AgentEvent) (bool, error) {
	if target == nil || event.Type != acp.EventTypePermission {
		return false, nil
	}
	target.interactionLifecycleMu.Lock()
	commit, handled, err := m.commitApplyAttentionAgentEvent(ctx, target, event)
	target.interactionLifecycleMu.Unlock()
	if err == nil && commit.Interaction != nil {
		m.publishAttentionCommit(ctx, target.ID, commit)
	}
	return handled, err
}

func (m *Manager) commitApplyAttentionAgentEvent(
	ctx context.Context,
	target *Session,
	event acp.AgentEvent,
) (store.SessionAttentionCommit, bool, error) {
	if target == nil || event.Type != acp.EventTypePermission {
		return store.SessionAttentionCommit{}, false, nil
	}
	requestID := strings.TrimSpace(event.RequestIDValue())
	if requestID == "" {
		return store.SessionAttentionCommit{}, false, errors.New("session: permission event request id is required")
	}
	if strings.TrimSpace(event.Decision) == "" {
		commit, err := m.applyPendingPermissionEvent(ctx, target, event, requestID)
		return commit, err == nil, err
	}
	interaction := pendingInteractionByProviderRequest(
		target.Info(),
		store.PendingInteractionKindPermission,
		requestID,
	)
	if interaction == nil || interaction.TurnID != event.TurnID ||
		(!event.Timestamp.IsZero() && event.Timestamp.Before(interaction.CreatedAt)) {
		return store.SessionAttentionCommit{}, false, nil
	}
	resolvedBy := strings.TrimSpace(event.ResolvedByValue())
	if resolvedBy == "" {
		resolvedBy = runtimeChangedEvidenceProviderKey
	}
	resolution := event.Decision
	status := store.PendingInteractionStatusResolved
	if resolvedBy == runtimeActivityKindTimeout {
		status = store.PendingInteractionStatusTimedOut
	}
	if event.Decision == string(acp.PermissionDecisionCanceled) {
		status = store.PendingInteractionStatusCanceled
		if resolvedBy == "timeout" {
			status = store.PendingInteractionStatusTimedOut
		}
		if resolvedBy == "provider" && target.Info().State != StateStopping && target.Info().State != StateStopped {
			status = store.PendingInteractionStatusTimedOut
			resolution = "expired-by-provider"
		}
	}
	commit, err := m.commitTransitionPendingInteraction(ctx, store.PendingInteractionTransition{
		InteractionID: interaction.InteractionID,
		Status:        status,
		Resolution:    resolution,
		ResolvedBy:    resolvedBy,
		At:            event.Timestamp,
	})
	return commit, err == nil, err
}

func permissionInteractionPayload(event acp.AgentEvent) (store.PendingInteractionPayload, string) {
	payload := store.PendingInteractionPayload{Action: event.Action, Resource: event.Resource}
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
	payload.ToolKind = strings.TrimSpace(raw.ToolCall.Kind)
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
	target.interactionLifecycleMu.Lock()
	commit, handled, err := m.commitApplyClarifyAttentionEvent(ctx, target, event)
	target.interactionLifecycleMu.Unlock()
	if err == nil && commit.Interaction != nil {
		m.publishAttentionCommit(ctx, target.ID, commit)
	}
	return handled, err
}

func (m *Manager) commitApplyClarifyAttentionEvent(
	ctx context.Context,
	target *Session,
	event toolspkg.ClarifyEvent,
) (store.SessionAttentionCommit, bool, error) {
	requestID := strings.TrimSpace(event.Request.RequestID)
	if event.Status == toolspkg.ClarifyStatusPending {
		info := target.Info()
		if info.State == StateStopping || info.State == StateStopped {
			return store.SessionAttentionCommit{}, true, nil
		}
		if pendingInteractionByProviderRequest(info, store.PendingInteractionKindClarify, requestID) != nil {
			return store.SessionAttentionCommit{}, true, nil
		}
		commit, err := m.commitCreatePendingInteraction(ctx, store.PendingInteractionCreate{
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
			return store.SessionAttentionCommit{}, true, m.refreshPendingInteractions(ctx, target.ID)
		}
		return commit, err == nil, err
	}
	interaction := pendingInteractionByProviderRequest(
		target.Info(),
		store.PendingInteractionKindClarify,
		requestID,
	)
	if interaction == nil {
		return store.SessionAttentionCommit{}, false, nil
	}
	status, err := clarifyInteractionStatus(event.Status)
	if err != nil {
		return store.SessionAttentionCommit{}, false, err
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
	commit, err := m.commitTransitionPendingInteraction(ctx, transition)
	return commit, err == nil, err
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
			return new(*interaction)
		}
	}
	return nil
}
