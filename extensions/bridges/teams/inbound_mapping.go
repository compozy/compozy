package main

import (
	"errors"
	"strings"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
)

type mappedTeamsInbound struct {
	Envelope bridgepkg.InboundMessageEnvelope
	Direct   bool
	User     teamsUserIdentity
}

type teamsUserIdentity struct {
	ID          string
	Username    string
	DisplayName string
}

type teamsInboundContext struct {
	receivedAt         time.Time
	serviceURL         string
	conversationID     string
	direct             bool
	baseConversationID string
	threadID           string
	user               teamsUserIdentity
}

func mapTeamsActivity(
	activity teamsActivity,
	cfg resolvedInstanceConfig,
	receivedAt time.Time,
) ([]mappedTeamsInbound, error) {
	switch strings.TrimSpace(activity.Type) {
	case providerMessageKey:
		if payload, ok := decodeTeamsMessageAction(activity.Value); ok {
			item, err := mapTeamsActionActivity(activity, cfg, payload, receivedAt, providerMessageKey)
			if err != nil {
				return nil, err
			}
			if item.Envelope.BridgeInstanceID == "" {
				return nil, nil
			}
			return []mappedTeamsInbound{item}, nil
		}
		item, ignored, err := mapTeamsMessageActivity(activity, cfg, receivedAt)
		if err != nil {
			return nil, err
		}
		if ignored {
			return nil, nil
		}
		return []mappedTeamsInbound{item}, nil
	case "invoke":
		payload, ok := decodeTeamsInvokeAction(activity.Value)
		if !ok {
			return nil, nil
		}
		item, err := mapTeamsActionActivity(activity, cfg, payload, receivedAt, "invoke")
		if err != nil {
			return nil, err
		}
		return []mappedTeamsInbound{item}, nil
	case "messageReaction":
		return mapTeamsReactionActivity(activity, cfg, receivedAt)
	case "conversationUpdate", "installationUpdate":
		return nil, nil
	default:
		return nil, nil
	}
}

func mapTeamsMessageActivity(
	activity teamsActivity,
	cfg resolvedInstanceConfig,
	receivedAt time.Time,
) (mappedTeamsInbound, bool, error) {
	conversationID := strings.TrimSpace(activity.Conversation.ID)
	if conversationID == "" || strings.TrimSpace(activity.ID) == "" {
		return mappedTeamsInbound{}, false, errors.New(
			"teams: message activity requires conversation.id and id",
		)
	}
	if isTeamsMessageFromSelf(activity, cfg) {
		return mappedTeamsInbound{}, true, nil
	}
	inboundContext, err := resolveTeamsInboundContext(activity, cfg, receivedAt, "message activity")
	if err != nil {
		return mappedTeamsInbound{}, false, err
	}
	envelope := buildTeamsMessageEnvelope(cfg, inboundContext, activity)
	if err := envelope.Validate(); err != nil {
		return mappedTeamsInbound{}, false, err
	}
	return mappedTeamsInbound{Envelope: envelope, Direct: inboundContext.direct, User: inboundContext.user}, false, nil
}

func mapTeamsActionActivity(
	activity teamsActivity,
	cfg resolvedInstanceConfig,
	payload teamsActionPayload,
	receivedAt time.Time,
	source string,
) (mappedTeamsInbound, error) {
	if strings.TrimSpace(payload.ActionID) == "" {
		return mappedTeamsInbound{}, errors.New("teams: action activity requires actionId")
	}
	inboundContext, err := resolveTeamsInboundContext(activity, cfg, receivedAt, "action activity")
	if err != nil {
		return mappedTeamsInbound{}, err
	}
	messageID := firstNonEmpty(
		strings.TrimSpace(activity.ReplyToID),
		messageIDFromConversationID(inboundContext.conversationID),
		strings.TrimSpace(activity.ID),
	)
	envelope := buildTeamsActionEnvelope(cfg, inboundContext, activity, payload, source, messageID)
	if err := envelope.Validate(); err != nil {
		return mappedTeamsInbound{}, err
	}
	return mappedTeamsInbound{
		Envelope: envelope,
		Direct:   inboundContext.direct,
		User:     inboundContext.user,
	}, nil
}

func mapTeamsReactionActivity(
	activity teamsActivity,
	cfg resolvedInstanceConfig,
	receivedAt time.Time,
) ([]mappedTeamsInbound, error) {
	inboundContext, err := resolveTeamsInboundContext(
		activity,
		cfg,
		receivedAt,
		"reaction activity",
	)
	if err != nil {
		return nil, err
	}
	messageID := firstNonEmpty(
		messageIDFromConversationID(inboundContext.conversationID),
		strings.TrimSpace(activity.ReplyToID),
		strings.TrimSpace(activity.ID),
	)
	if messageID == "" {
		return nil, errors.New("teams: reaction activity requires a message identifier")
	}
	items := make(
		[]mappedTeamsInbound,
		0,
		len(activity.ReactionsAdded)+len(activity.ReactionsRemoved),
	)
	for _, reaction := range activity.ReactionsAdded {
		item, ok, err := mapTeamsReactionItem(
			cfg,
			inboundContext,
			activity,
			messageID,
			reaction,
			true,
		)
		if err != nil {
			return nil, err
		}
		if ok {
			items = append(items, item)
		}
	}
	for _, reaction := range activity.ReactionsRemoved {
		item, ok, err := mapTeamsReactionItem(
			cfg,
			inboundContext,
			activity,
			messageID,
			reaction,
			false,
		)
		if err != nil {
			return nil, err
		}
		if ok {
			items = append(items, item)
		}
	}
	return items, nil
}
