package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
	"github.com/compozy/agh/internal/subprocess"
)

func allowGChatDirectMessage(cfg *resolvedInstanceConfig, user gchatUserIdentity, direct bool) bool {
	if cfg == nil {
		return false
	}
	if !direct {
		return true
	}
	switch cfg.dmPolicy.Normalize() {
	case "", bridgepkg.BridgeDMPolicyOpen:
		return true
	case bridgepkg.BridgeDMPolicyAllowlist:
		return gchatIdentityAllowed(cfg.allowUserIDs, cfg.allowUsernames, user)
	case bridgepkg.BridgeDMPolicyPairing:
		if gchatIdentityAllowed(cfg.pairedUserIDs, cfg.pairedUsernames, user) {
			return true
		}
		return gchatIdentityAllowed(cfg.allowUserIDs, cfg.allowUsernames, user)
	default:
		return false
	}
}

func gchatIdentityAllowed(ids map[string]struct{}, usernames map[string]struct{}, user gchatUserIdentity) bool {
	if len(ids) == 0 && len(usernames) == 0 {
		return false
	}
	if _, ok := ids[normalizeUsername(user.ID)]; ok {
		return true
	}
	if _, ok := usernames[normalizeUsername(firstNonEmpty(user.Username, user.DisplayName))]; ok {
		return true
	}
	return false
}

func mapDirectMessageEvent(
	event gchatEvent,
	managed subprocess.InitializeBridgeManagedInstance,
	receivedAt time.Time,
	parents *bridgesdk.ParentMessageCache,
) (gchatMappedInbound, bool, error) {
	if event.Chat == nil || event.Chat.MessagePayload == nil {
		return gchatMappedInbound{}, false, nil
	}
	message := event.Chat.MessagePayload.Message
	space := event.Chat.MessagePayload.Space
	if isBotUser(message.Sender) {
		return gchatMappedInbound{}, false, nil
	}
	item, err := mapGChatMessage(
		message,
		space,
		managed,
		receivedAt,
		"direct:"+strings.TrimSpace(message.Name),
		"direct_webhook",
		parents,
	)
	return item, true, err
}

func mapDirectActionEvent(
	event gchatEvent,
	managed subprocess.InitializeBridgeManagedInstance,
	receivedAt time.Time,
) (gchatMappedInbound, bool, error) {
	actionCtx, ok := buildDirectActionContext(event)
	if !ok {
		return gchatMappedInbound{}, false, nil
	}
	if isBotUser(actionCtx.user) {
		return gchatMappedInbound{}, false, nil
	}
	item, err := buildDirectActionItem(event, managed, receivedAt, actionCtx)
	return item, true, err
}

type gchatDirectActionContext struct {
	space           gchatSpace
	message         gchatMessage
	user            gchatUser
	actionID        string
	invokedFunction string
	parameters      map[string]string
}

func buildDirectActionContext(event gchatEvent) (gchatDirectActionContext, bool) {
	if event.Chat == nil {
		return gchatDirectActionContext{}, false
	}
	button := event.Chat.ButtonClickedPayload
	invokedFunction := ""
	parameters := map[string]string(nil)
	if event.CommonEventObject != nil {
		invokedFunction = strings.TrimSpace(event.CommonEventObject.InvokedFunction)
		parameters = event.CommonEventObject.Parameters
	}
	if button == nil && invokedFunction == "" {
		return gchatDirectActionContext{}, false
	}

	ctx := gchatDirectActionContext{
		invokedFunction: invokedFunction,
		parameters:      parameters,
	}
	if button != nil {
		ctx.space = button.Space
		ctx.message = button.Message
		ctx.user = button.User
	}
	if ctx.user.Name == "" && event.Chat.User != nil {
		ctx.user = *event.Chat.User
	}
	ctx.actionID = firstNonEmpty(paramValue(parameters, "actionId"), invokedFunction)
	if ctx.actionID == "" {
		return gchatDirectActionContext{}, false
	}
	return ctx, true
}

func buildDirectActionItem(
	event gchatEvent,
	managed subprocess.InitializeBridgeManagedInstance,
	receivedAt time.Time,
	actionCtx gchatDirectActionContext,
) (gchatMappedInbound, error) {
	direct := isDirectSpace(actionCtx.space)
	threadName := firstNonEmpty(
		threadNameForMessage(actionCtx.message, direct),
		strings.TrimSpace(actionCtx.message.Name),
	)
	envelope := bridgepkg.InboundMessageEnvelope{
		BridgeInstanceID: managed.Instance.ID,
		Scope:            managed.Instance.Scope,
		WorkspaceID:      managed.Instance.WorkspaceID,
		ReceivedAt:       normalizeReceivedAt(receivedAt, event.Chat.EventTime),
		Sender:           gchatSender(actionCtx.user),
		EventFamily:      bridgepkg.InboundEventFamilyAction,
		Action: &bridgepkg.InboundAction{
			ActionID:  actionCtx.actionID,
			MessageID: strings.TrimSpace(actionCtx.message.Name),
			Value:     paramValue(actionCtx.parameters, "value"),
			TriggerID: firstNonEmpty(paramValue(actionCtx.parameters, "triggerId"), actionCtx.invokedFunction),
		},
		IdempotencyKey: fmt.Sprintf(
			"gchat:%s:action:%s:%s",
			managed.Instance.ID,
			actionCtx.actionID,
			strings.TrimSpace(actionCtx.message.Name),
		),
	}
	assignGChatRoute(&envelope, strings.TrimSpace(actionCtx.space.Name), threadName, direct)
	if metadata, err := json.Marshal(map[string]any{
		providerSourceKey:     "direct_webhook",
		providerSpaceNameKey:  strings.TrimSpace(actionCtx.space.Name),
		providerThreadNameKey: threadName,
		"message_name":        strings.TrimSpace(actionCtx.message.Name),
	}); err == nil {
		envelope.ProviderMetadata = metadata
	}
	item := gchatMappedInbound{
		Envelope: envelope,
		Direct:   direct,
		User: gchatUserIdentity{
			ID:          envelope.Sender.ID,
			Username:    envelope.Sender.Username,
			DisplayName: envelope.Sender.DisplayName,
		},
	}
	return item, item.Envelope.Validate()
}

func mapPubSubMessageEvent(
	notification gchatWorkspaceEventNotification,
	managed subprocess.InitializeBridgeManagedInstance,
	receivedAt time.Time,
	parents *bridgesdk.ParentMessageCache,
) (gchatMappedInbound, error) {
	message := notification.Message
	if message == nil {
		return gchatMappedInbound{}, errors.New("gchat: pubsub notification missing message")
	}
	space := gchatSpace{}
	if message.Space != nil {
		space = *message.Space
	}
	if space.Name == "" {
		space.Name = strings.TrimPrefix(strings.TrimSpace(notification.TargetResource), "//chat.googleapis.com/")
	}
	return mapGChatMessage(
		*message,
		space,
		managed,
		normalizeReceivedAt(receivedAt, notification.EventTime),
		"pubsub:"+firstNonEmpty(notification.EventType, strings.TrimSpace(message.Name)),
		"pubsub_workspace_events",
		parents,
	)
}

func mapPubSubReactionEvent(
	ctx context.Context,
	api gchatAPI,
	notification gchatWorkspaceEventNotification,
	managed subprocess.InitializeBridgeManagedInstance,
	receivedAt time.Time,
) (gchatMappedInbound, error) {
	if notification.Reaction == nil {
		return gchatMappedInbound{}, errors.New("gchat: pubsub notification missing reaction")
	}
	route, err := resolvePubSubReactionRoute(ctx, api, notification)
	if err != nil {
		return gchatMappedInbound{}, err
	}
	item := buildPubSubReactionItem(notification, managed, receivedAt, route)
	return item, item.Envelope.Validate()
}

type gchatPubSubReactionRoute struct {
	messageName string
	spaceName   string
	threadName  string
	direct      bool
	reaction    *gchatReaction
}

func resolvePubSubReactionRoute(
	ctx context.Context,
	api gchatAPI,
	notification gchatWorkspaceEventNotification,
) (gchatPubSubReactionRoute, error) {
	reaction := notification.Reaction
	messageName := extractReactionMessageName(reaction.Name)
	if messageName == "" {
		return gchatPubSubReactionRoute{}, errors.New("gchat: reaction resource omitted message reference")
	}

	spaceName := strings.TrimPrefix(strings.TrimSpace(notification.TargetResource), "//chat.googleapis.com/")
	threadName := messageName
	direct := false
	if api != nil {
		if message, err := api.GetMessage(ctx, messageName); err == nil && message != nil {
			if message.Space != nil && strings.TrimSpace(message.Space.Name) != "" {
				spaceName = strings.TrimSpace(message.Space.Name)
				direct = isDirectSpace(*message.Space)
			}
			threadName = threadNameForMessage(*message, direct)
			if threadName == "" {
				threadName = strings.TrimSpace(message.Name)
			}
		}
	}
	if spaceName == "" {
		parts := strings.Split(messageName, "/")
		if len(parts) >= 2 {
			spaceName = parts[0] + "/" + parts[1]
		}
	}
	if threadName == "" {
		threadName = messageName
	}
	return gchatPubSubReactionRoute{
		messageName: messageName,
		spaceName:   spaceName,
		threadName:  threadName,
		direct:      direct,
		reaction:    reaction,
	}, nil
}

func assignGChatRoute(
	envelope *bridgepkg.InboundMessageEnvelope,
	spaceName string,
	threadName string,
	direct bool,
) {
	if envelope == nil {
		return
	}
	if direct {
		envelope.PeerID = strings.TrimSpace(spaceName)
	} else {
		envelope.GroupID = strings.TrimSpace(spaceName)
	}
	if direct || strings.TrimSpace(threadName) != "" {
		envelope.ThreadID = encodeGChatThreadID(gchatThreadRef{
			SpaceName:  strings.TrimSpace(spaceName),
			ThreadName: strings.TrimSpace(threadName),
			IsDM:       direct,
		})
	}
}

func buildPubSubReactionItem(
	notification gchatWorkspaceEventNotification,
	managed subprocess.InitializeBridgeManagedInstance,
	receivedAt time.Time,
	route gchatPubSubReactionRoute,
) gchatMappedInbound {
	sender := gchatSender(derefUser(route.reaction.User))
	envelope := bridgepkg.InboundMessageEnvelope{
		BridgeInstanceID: managed.Instance.ID,
		Scope:            managed.Instance.Scope,
		WorkspaceID:      managed.Instance.WorkspaceID,
		ReceivedAt:       normalizeReceivedAt(receivedAt, notification.EventTime),
		Sender:           sender,
		EventFamily:      bridgepkg.InboundEventFamilyReaction,
		Reaction: &bridgepkg.InboundReaction{
			MessageID: route.messageName,
			Emoji:     normalizeGChatEmoji(reactionEmoji(route.reaction)),
			RawEmoji:  reactionEmoji(route.reaction),
			Added:     strings.Contains(strings.ToLower(strings.TrimSpace(notification.EventType)), "created"),
		},
		IdempotencyKey: fmt.Sprintf(
			"gchat:%s:reaction:%s:%s",
			managed.Instance.ID,
			strings.TrimSpace(notification.EventType),
			strings.TrimSpace(route.reaction.Name),
		),
	}
	assignGChatRoute(&envelope, route.spaceName, route.threadName, route.direct)
	if metadata, err := json.Marshal(map[string]any{
		providerSourceKey:     "pubsub_workspace_events",
		"event_type":          strings.TrimSpace(notification.EventType),
		providerSpaceNameKey:  route.spaceName,
		providerThreadNameKey: route.threadName,
		"reaction_name":       strings.TrimSpace(route.reaction.Name),
	}); err == nil {
		envelope.ProviderMetadata = metadata
	}

	return gchatMappedInbound{
		Envelope: envelope,
		Direct:   route.direct,
		User:     gchatUserIdentity{ID: sender.ID, Username: sender.Username, DisplayName: sender.DisplayName},
	}
}
