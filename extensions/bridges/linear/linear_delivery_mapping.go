package main

import (
	"errors"
	"fmt"

	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
)

func normalizeDeliveryEventType(value bridgepkg.DeliveryEventType) bridgepkg.DeliveryEventType {
	return bridgepkg.DeliveryEventType(strings.ToLower(strings.TrimSpace(string(value))))
}

func resolveLinearRemoteMessageID(
	reference *bridgepkg.DeliveryMessageReference,
	state deliveryState,
	snapshot *bridgepkg.DeliverySnapshot,
) string {
	if reference != nil && strings.TrimSpace(reference.RemoteMessageID) != "" {
		return strings.TrimSpace(reference.RemoteMessageID)
	}
	if strings.TrimSpace(state.RemoteMessageID) != "" {
		return strings.TrimSpace(state.RemoteMessageID)
	}
	if snapshot != nil {
		return strings.TrimSpace(snapshot.RemoteMessageID)
	}
	return ""
}

func computeLinearAppendDelta(previous string, current string) string {
	if previous == "" {
		return current
	}
	if strings.HasPrefix(current, previous) {
		return current[len(previous):]
	}
	return current
}

func shouldSkipLinearCommentDelivery(event bridgepkg.DeliveryEvent, _ string, body string) bool {
	return normalizeDeliveryEventType(event.EventType) == bridgepkg.DeliveryEventTypeResume &&
		strings.TrimSpace(body) == ""
}

func validateLinearAgentSessionDelivery(event bridgepkg.DeliveryEvent) error {
	if event.Operation.Normalize() == bridgepkg.DeliveryOperationDelete ||
		normalizeDeliveryEventType(event.EventType) == bridgepkg.DeliveryEventTypeDelete {
		return errors.New("linear: agent session activities are append-only and cannot be deleted")
	}
	if event.Operation.Normalize() == bridgepkg.DeliveryOperationEdit {
		return errors.New("linear: agent session activities are append-only and cannot be edited")
	}
	return nil
}

func decodeLinearAgentSessionThread(event bridgepkg.DeliveryEvent) (linearThreadRef, error) {
	thread, err := decodeLinearThreadID(firstNonEmpty(event.DeliveryTarget.ThreadID, event.RoutingKey.ThreadID))
	if err != nil {
		return linearThreadRef{}, err
	}
	if strings.TrimSpace(thread.AgentSessionID) == "" {
		return linearThreadRef{}, errors.New("linear: agent_sessions mode requires an agent session thread id")
	}
	return thread, nil
}

func resumeLinearAgentSessionDelivery(
	event bridgepkg.DeliveryEvent,
	state deliveryState,
	remoteID string,
) (bridgepkg.DeliveryAck, deliveryState, bool) {
	if normalizeDeliveryEventType(event.EventType) != bridgepkg.DeliveryEventTypeResume ||
		strings.TrimSpace(remoteID) == "" {
		return bridgepkg.DeliveryAck{}, state, false
	}

	next := state
	next.LastSeq = event.Seq
	next.LastContent = event.Content.Text
	next.RemoteMessageID = remoteID
	ack := bridgepkg.DeliveryAck{
		DeliveryID:      event.DeliveryID,
		Seq:             event.Seq,
		RemoteMessageID: remoteID,
	}
	return ack, next, true
}

func shouldProcessLinearAgentSessionAction(action string) bool {
	switch action {
	case providerCreatedKey, providerPromptedKey:
		return true
	default:
		return false
	}
}

func isUnexpectedLinearBotUser(botUserID string, payload linearAgentSessionWebhookPayload) bool {
	appUserID := strings.TrimSpace(firstNonEmpty(payload.AgentSession.AppUserID, payload.AppUserID))
	if strings.TrimSpace(botUserID) == "" || appUserID == "" {
		return false
	}
	return appUserID != strings.TrimSpace(botUserID)
}

func mapLinearAgentSessionMessage(
	action string,
	payload linearAgentSessionWebhookPayload,
) (string, string, bridgepkg.MessageSender, error) {
	switch action {
	case providerCreatedKey:
		if payload.AgentSession.Comment == nil {
			return "", "", bridgepkg.MessageSender{}, errors.New(
				"linear: created agent session event is missing comment payload",
			)
		}
		return strings.TrimSpace(payload.AgentSession.Comment.ID),
			strings.TrimSpace(payload.AgentSession.Comment.Body),
			bridgepkg.MessageSender{
				ID:          firstNonEmpty(actorID(payload.AgentSession.Creator), payload.Actor.ID),
				Username:    linearUserName(actorURL(payload.AgentSession.Creator)),
				DisplayName: firstNonEmpty(actorName(payload.AgentSession.Creator), payload.Actor.Name),
			},
			nil
	case providerPromptedKey:
		if payload.AgentActivity == nil {
			return "", "", bridgepkg.MessageSender{}, errors.New(
				"linear: prompted agent session event is missing agentActivity",
			)
		}
		return strings.TrimSpace(firstNonEmpty(payload.AgentSession.SourceCommentID, payload.AgentSession.CommentID)),
			strings.TrimSpace(firstNonEmpty(payload.AgentActivity.Content.Body, payload.AgentActivity.Body)),
			bridgepkg.MessageSender{
				ID:          strings.TrimSpace(payload.Actor.ID),
				Username:    linearUserName(payload.Actor.URL),
				DisplayName: strings.TrimSpace(payload.Actor.Name),
			},
			nil
	default:
		return "", "", bridgepkg.MessageSender{}, errors.New("linear: unsupported agent session action")
	}
}

func encodeLinearThreadID(ref linearThreadRef) string {
	issueID := strings.TrimSpace(ref.IssueID)
	if strings.TrimSpace(ref.AgentSessionID) != "" {
		if strings.TrimSpace(ref.RootCommentID) != "" {
			return "linear:" + issueID + ":c:" + strings.TrimSpace(
				ref.RootCommentID,
			) + ":s:" + strings.TrimSpace(
				ref.AgentSessionID,
			)
		}
		return "linear:" + issueID + ":s:" + strings.TrimSpace(ref.AgentSessionID)
	}
	if strings.TrimSpace(ref.RootCommentID) != "" {
		return "linear:" + issueID + ":c:" + strings.TrimSpace(ref.RootCommentID)
	}
	return "linear:" + issueID
}

func decodeLinearThreadID(threadID string) (linearThreadRef, error) {
	trimmed := strings.TrimSpace(threadID)
	if trimmed == "" {
		return linearThreadRef{}, errors.New("linear: thread id is required")
	}

	if matches := linearCommentSessionThreadPattern.FindStringSubmatch(trimmed); len(matches) == 4 {
		return linearThreadRef{
			IssueID:        matches[1],
			RootCommentID:  matches[2],
			AgentSessionID: matches[3],
		}, nil
	}
	if matches := linearIssueSessionThreadPattern.FindStringSubmatch(trimmed); len(matches) == 3 {
		return linearThreadRef{
			IssueID:        matches[1],
			AgentSessionID: matches[2],
		}, nil
	}
	if matches := linearCommentThreadPattern.FindStringSubmatch(trimmed); len(matches) == 3 {
		return linearThreadRef{
			IssueID:       matches[1],
			RootCommentID: matches[2],
		}, nil
	}
	if matches := linearIssueThreadPattern.FindStringSubmatch(trimmed); len(matches) == 2 {
		return linearThreadRef{IssueID: matches[1]}, nil
	}
	return linearThreadRef{}, fmt.Errorf("linear: invalid thread id %q", trimmed)
}

func issueThreadIDFromGroup(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return "linear:" + trimmed
		}
	}
	return ""
}

func linearUserName(profileURL string) string {
	url := strings.TrimSpace(profileURL)
	if url == "" {
		return ""
	}
	parts := strings.Split(url, "/profiles/")
	if len(parts) != 2 {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func actorID(actor *linearActor) string {
	if actor == nil {
		return ""
	}
	return strings.TrimSpace(actor.ID)
}

func actorName(actor *linearActor) string {
	if actor == nil {
		return ""
	}
	return strings.TrimSpace(actor.Name)
}

func actorURL(actor *linearActor) string {
	if actor == nil {
		return ""
	}
	return strings.TrimSpace(actor.URL)
}
