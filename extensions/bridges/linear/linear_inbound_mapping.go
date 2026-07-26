package main

import (
	"encoding/json"
	"errors"

	"strings"

	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"

	"github.com/compozy/agh/internal/subprocess"
)

type linearMappedInbound struct {
	Envelope bridgepkg.InboundMessageEnvelope
}

func mapLinearCommentCreated(
	payload linearCommentWebhookPayload,
	managed subprocess.InitializeBridgeManagedInstance,
	receivedAt time.Time,
) (linearMappedInbound, bool, error) {
	comment := payload.Data
	if strings.TrimSpace(comment.IssueID) == "" {
		return linearMappedInbound{}, true, nil
	}
	commentID := strings.TrimSpace(comment.ID)
	if commentID == "" {
		return linearMappedInbound{}, false, errors.New("linear: comment webhook data.id is required")
	}
	rootCommentID := firstNonEmpty(comment.ParentID, commentID)
	threadID := encodeLinearThreadID(linearThreadRef{
		IssueID:       strings.TrimSpace(comment.IssueID),
		RootCommentID: strings.TrimSpace(rootCommentID),
	})
	providerMetadata, err := json.Marshal(map[string]any{
		providerOrganizationIDKey: payload.OrganizationID,
		"issue_id":                strings.TrimSpace(comment.IssueID),
		"comment_id":              commentID,
		"root_comment_id":         strings.TrimSpace(rootCommentID),
		providerModeKey:           linearModeComments,
		providerURLKey:            strings.TrimSpace(payload.URL),
	})
	if err != nil {
		return linearMappedInbound{}, false, err
	}

	envelope := bridgepkg.InboundMessageEnvelope{
		BridgeInstanceID:  managed.Instance.ID,
		Scope:             managed.Instance.Scope,
		WorkspaceID:       managed.Instance.WorkspaceID,
		GroupID:           strings.TrimSpace(comment.IssueID),
		ThreadID:          threadID,
		PlatformMessageID: commentID,
		ReceivedAt:        receivedAt,
		Sender: bridgepkg.MessageSender{
			ID:          firstNonEmpty(comment.User.ID, comment.UserID, payload.Actor.ID),
			Username:    linearUserName(firstNonEmpty(comment.User.URL, payload.Actor.URL)),
			DisplayName: firstNonEmpty(comment.User.Name, payload.Actor.Name),
		},
		Content: bridgepkg.MessageContent{
			Text: strings.TrimSpace(comment.Body),
		},
		EventFamily:      bridgepkg.InboundEventFamilyMessage,
		ProviderMetadata: providerMetadata,
		IdempotencyKey:   firstNonEmpty(payload.WebhookID, commentID),
	}
	return linearMappedInbound{Envelope: envelope}, false, envelope.Validate()
}

func mapLinearAgentSessionEvent(
	payload linearAgentSessionWebhookPayload,
	managed *subprocess.InitializeBridgeManagedInstance,
	receivedAt time.Time,
	botUserID string,
) (linearMappedInbound, bool, error) {
	if managed == nil {
		return linearMappedInbound{}, false, errors.New("linear: managed bridge instance is required")
	}
	action := strings.TrimSpace(payload.Action)
	if !shouldProcessLinearAgentSessionAction(action) {
		return linearMappedInbound{}, true, nil
	}

	sessionID := strings.TrimSpace(payload.AgentSession.ID)
	issueID := strings.TrimSpace(payload.AgentSession.IssueID)
	rootCommentID := strings.TrimSpace(
		firstNonEmpty(payload.AgentSession.CommentID, payload.AgentSession.SourceCommentID),
	)
	if issueID == "" || sessionID == "" || rootCommentID == "" {
		return linearMappedInbound{}, false, errors.New(
			"linear: agent session webhook is missing issue, session, or root comment identity",
		)
	}
	if isUnexpectedLinearBotUser(botUserID, payload) {
		return linearMappedInbound{}, true, nil
	}

	messageID, text, sender, err := mapLinearAgentSessionMessage(action, payload)
	if err != nil {
		return linearMappedInbound{}, false, err
	}
	if messageID == "" {
		return linearMappedInbound{}, false, errors.New("linear: agent session webhook message id is required")
	}

	threadID := encodeLinearThreadID(linearThreadRef{
		IssueID:        issueID,
		RootCommentID:  rootCommentID,
		AgentSessionID: sessionID,
	})
	providerMetadata, err := json.Marshal(map[string]any{
		providerOrganizationIDKey: payload.OrganizationID,
		"issue_id":                issueID,
		"root_comment_id":         rootCommentID,
		"agent_session_id":        sessionID,
		"prompt_context":          strings.TrimSpace(payload.PromptContext),
		providerModeKey:           linearModeAgentSessions,
		providerActionKey:         action,
	})
	if err != nil {
		return linearMappedInbound{}, false, err
	}

	envelope := bridgepkg.InboundMessageEnvelope{
		BridgeInstanceID:  managed.Instance.ID,
		Scope:             managed.Instance.Scope,
		WorkspaceID:       managed.Instance.WorkspaceID,
		GroupID:           issueID,
		ThreadID:          threadID,
		PlatformMessageID: messageID,
		ReceivedAt:        receivedAt,
		Sender:            sender,
		Content:           bridgepkg.MessageContent{Text: text},
		EventFamily:       bridgepkg.InboundEventFamilyMessage,
		ProviderMetadata:  providerMetadata,
		IdempotencyKey:    firstNonEmpty(payload.WebhookID, sessionID+":"+action+":"+messageID),
	}
	return linearMappedInbound{Envelope: envelope}, false, envelope.Validate()
}
