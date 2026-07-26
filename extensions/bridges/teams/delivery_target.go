package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
)

type teamsRemoteMessageRef struct {
	ConversationID string
	ServiceURL     string
	ActivityID     string
}

func resolveTeamsDeliveryTarget(
	cfg resolvedInstanceConfig,
	event bridgepkg.DeliveryEvent,
	userContextLookup func(string, string) (teamsUserContext, bool),
) (teamsResolvedTarget, error) {
	if thread := firstNonEmpty(
		strings.TrimSpace(event.DeliveryTarget.ThreadID),
		strings.TrimSpace(event.RoutingKey.ThreadID),
	); thread != "" {
		if decoded, err := decodeTeamsThreadID(thread); err == nil {
			baseConversationID, replyToID := splitTeamsConversationTarget(decoded.ConversationID)
			return teamsResolvedTarget{
				ConversationID: baseConversationID,
				ServiceURL:     firstNonEmpty(decoded.ServiceURL, cfg.serviceURL),
				ReplyToID:      replyToID,
			}, nil
		}
	}

	targetID := firstNonEmpty(
		strings.TrimSpace(event.DeliveryTarget.PeerID),
		strings.TrimSpace(event.DeliveryTarget.GroupID),
		strings.TrimSpace(event.RoutingKey.PeerID),
		strings.TrimSpace(event.RoutingKey.GroupID),
	)
	if targetID == "" {
		return teamsResolvedTarget{}, errors.New(
			"teams: delivery target requires peer_id or group_id",
		)
	}

	if looksLikeTeamsUserID(targetID) {
		ctx, ok := userContextLookup(cfg.instanceID, targetID)
		serviceURL := cfg.serviceURL
		tenantID := cfg.appTenantID
		if ok {
			serviceURL = firstNonEmpty(ctx.ServiceURL, serviceURL)
			tenantID = firstNonEmpty(ctx.TenantID, tenantID)
		}
		if tenantID == "" {
			return teamsResolvedTarget{}, &bridgesdk.PermanentError{
				Err: errors.New("teams: tenant ID not found for proactive DM target"),
			}
		}
		if serviceURL == "" {
			return teamsResolvedTarget{}, &bridgesdk.PermanentError{
				Err: errors.New("teams: service URL not found for proactive DM target"),
			}
		}
		return teamsResolvedTarget{
			ServiceURL: serviceURL,
			UserID:     normalizeTeamsID(targetID),
			TenantID:   tenantID,
		}, nil
	}

	baseConversationID, replyToID := splitTeamsConversationTarget(targetID)
	return teamsResolvedTarget{
		ConversationID: baseConversationID,
		ServiceURL:     cfg.serviceURL,
		ReplyToID:      replyToID,
	}, nil
}

func decodeTeamsMessageAction(raw json.RawMessage) (teamsActionPayload, bool) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return teamsActionPayload{}, false
	}
	var payload teamsActionPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return teamsActionPayload{}, false
	}
	if strings.TrimSpace(payload.ActionID) == "" {
		return teamsActionPayload{}, false
	}
	return payload, true
}

func decodeTeamsInvokeAction(raw json.RawMessage) (teamsActionPayload, bool) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return teamsActionPayload{}, false
	}
	var wrapper teamsActionValue
	if err := json.Unmarshal(raw, &wrapper); err != nil || wrapper.Action == nil {
		return teamsActionPayload{}, false
	}
	var payload teamsActionPayload
	if err := json.Unmarshal(wrapper.Action.Data, &payload); err != nil {
		return teamsActionPayload{}, false
	}
	if strings.TrimSpace(payload.ActionID) == "" {
		return teamsActionPayload{}, false
	}
	return payload, true
}

func normalizeTeamsAttachments(items []teamsAttachment) []bridgepkg.MessageAttachment {
	attachments := make([]bridgepkg.MessageAttachment, 0, len(items))
	for _, item := range items {
		contentType := strings.TrimSpace(item.ContentType)
		if contentType == "application/vnd.microsoft.card.adaptive" ||
			(contentType == "text/html" && strings.TrimSpace(item.ContentURL) == "") {
			continue
		}
		attachment := bridgepkg.MessageAttachment{
			Name:     strings.TrimSpace(item.Name),
			MIMEType: contentType,
			URL:      strings.TrimSpace(item.ContentURL),
		}
		if attachment.Name == "" && attachment.URL == "" && attachment.MIMEType == "" {
			continue
		}
		attachments = append(attachments, attachment)
	}
	if len(attachments) == 0 {
		return nil
	}
	return attachments
}

func normalizeTeamsText(text string) string {
	return strings.TrimSpace(text)
}

func normalizeTeamsEmoji(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeTeamsID(value string) string {
	return strings.TrimSpace(value)
}

func normalizeTeamsUsername(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func isTeamsDirectConversation(conversation teamsConversation) bool {
	conversationID := strings.TrimSpace(conversation.ID)
	if conversationID == "" {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(conversation.ConversationType), "channel") {
		return false
	}
	return !strings.HasPrefix(conversationID, "19:")
}

func isTeamsMessageFromSelf(activity teamsActivity, cfg resolvedInstanceConfig) bool {
	sender := normalizeTeamsID(activity.From.ID)
	recipient := normalizeTeamsID(activity.Recipient.ID)
	return sender != "" && recipient != "" && sender == recipient ||
		recipient == strings.TrimSpace(cfg.appID)
}

func extractTeamsTenantID(activity teamsActivity) string {
	if tenantID := strings.TrimSpace(activity.Conversation.TenantID); tenantID != "" {
		return tenantID
	}
	if activity.ChannelData.Tenant != nil {
		return strings.TrimSpace(activity.ChannelData.Tenant.ID)
	}
	return ""
}

func messageIDFromConversationID(conversationID string) string {
	parts := strings.SplitN(conversationID, ";messageid=", 2)
	if len(parts) != 2 {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func baseTeamsConversationID(conversationID string) string {
	return strings.TrimSpace(
		messageIDStripPattern.ReplaceAllString(strings.TrimSpace(conversationID), ""),
	)
}

func splitTeamsConversationTarget(conversationID string) (string, string) {
	return baseTeamsConversationID(conversationID), messageIDFromConversationID(conversationID)
}

func encodeTeamsThreadID(ref teamsThreadRef) string {
	encodedConversationID := base64.RawURLEncoding.EncodeToString(
		[]byte(strings.TrimSpace(ref.ConversationID)),
	)
	encodedServiceURL := base64.RawURLEncoding.EncodeToString([]byte(normalizeURL(ref.ServiceURL)))
	return "teams:" + encodedConversationID + ":" + encodedServiceURL
}

func decodeTeamsThreadID(value string) (teamsThreadRef, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 3 || parts[0] != providerTeamsKey {
		return teamsThreadRef{}, errors.New("teams: invalid thread id")
	}
	conversationID, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return teamsThreadRef{}, err
	}
	serviceURL, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return teamsThreadRef{}, err
	}
	return teamsThreadRef{
		ConversationID: string(conversationID),
		ServiceURL:     string(serviceURL),
	}, nil
}

func encodeRemoteMessageID(ref teamsRemoteMessageRef) string {
	conversationID := base64.RawURLEncoding.EncodeToString(
		[]byte(strings.TrimSpace(ref.ConversationID)),
	)
	serviceURL := base64.RawURLEncoding.EncodeToString([]byte(normalizeURL(ref.ServiceURL)))
	activityID := base64.RawURLEncoding.EncodeToString([]byte(strings.TrimSpace(ref.ActivityID)))
	return strings.Join([]string{"teamsmsg", conversationID, serviceURL, activityID}, ":")
}

func decodeRemoteMessageID(value string) (teamsRemoteMessageRef, error) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 4 || parts[0] != "teamsmsg" {
		return teamsRemoteMessageRef{}, errors.New("teams: invalid remote message id")
	}
	conversationID, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return teamsRemoteMessageRef{}, err
	}
	serviceURL, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return teamsRemoteMessageRef{}, err
	}
	activityID, err := base64.RawURLEncoding.DecodeString(parts[3])
	if err != nil {
		return teamsRemoteMessageRef{}, err
	}
	if strings.TrimSpace(string(conversationID)) == "" ||
		strings.TrimSpace(string(serviceURL)) == "" ||
		strings.TrimSpace(string(activityID)) == "" {
		return teamsRemoteMessageRef{}, errors.New("teams: remote message id is incomplete")
	}
	return teamsRemoteMessageRef{
		ConversationID: strings.TrimSpace(string(conversationID)),
		ServiceURL:     normalizeURL(string(serviceURL)),
		ActivityID:     strings.TrimSpace(string(activityID)),
	}, nil
}
